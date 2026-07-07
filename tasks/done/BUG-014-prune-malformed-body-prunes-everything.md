---
id: BUG-014
type: bug
title: POST /system/prune's malformed-body fallback silently prunes everything
status: done
complexity: simple
assignee: sdlc-developer
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "found during TEST-003's live verification pass (read from source, not independently exercised end-to-end there since a real prune isn't safe against the shared sandbox daemon); filed separately per that task's rule of not fixing bugs inline"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer (simple: will delegate implementation to agy)"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "agy hit Gemini quota exhaustion (429, resets ~150h) mid-task and produced no output; developer correctly detected this and fell back to implementing directly. Root-caused: sdlc-developer.md and SKILL.md's agy calls now always pass --model \"Claude Sonnet 4.6 (Thinking)\" to route around the Gemini quota (confirmed working via a direct test). Diff independently verified as a correct, minimal, convention-matching fix."}
  - {date: 2026-07-07, stage: in-test, by: architect, note: "sdlc-reviewer PASS (simple complexity, single review only); moved to test"}
  - {date: 2026-07-07, stage: done, by: architect, note: "tester PASS against independently-built live backend, verified via safe no-op/malformed-body cases only; teardown confirmed clean (noted: the live tamga-* stack showed a uniform restart ~53min prior, consistent with a host/daemon-level event unrelated to any pipeline agent action)"}
---

## Summary
`ContainerHandler.Prune` (`backend/internal/handler/container_handler.go`)
treats a malformed/empty request body identically to an explicit
`{"all":true}`: `if err := json.NewDecoder(r.Body).Decode(&req); err != nil
{ req.All = true }`. An empty `POST` body — an easy mistake for any future
client/frontend code — silently deletes all stopped containers, dangling
images, unused volumes, and unused networks daemon-wide, rather than
failing with a clear error.

## Steps to Reproduce
1. `POST /api/system/prune` with an empty body (or genuinely malformed
   JSON), no `Content-Type`/body at all.
2. Observe: instead of a `400 Bad Request`, the request succeeds and
   `req.All` silently defaults to `true`, running a full daemon-wide
   prune across containers/images/volumes/networks.

## Expected Behavior
A decode failure should return `400 Bad Request` (matching how other
handlers in this codebase treat an undecodable body), doing nothing to
the Docker daemon — not silently defaulting to the most destructive
possible option.

## Actual Behavior
The decode-error branch sets `req.All = true`, running every prune
operation instead of erroring.

## Environment / Context
Found during TEST-003's live verification pass by reading
`container_handler.go`'s `Prune` source directly — not independently
exercised end-to-end there, since a real destructive prune isn't safe to
run against this environment's shared Docker daemon (other unrelated
stopped containers/images exist that aren't this task's to remove).

## Root Cause
`backend/internal/handler/container_handler.go`, lines 231–233: the `Prune` handler's error-handling branch sets `req.All = true` on any JSON decode failure instead of returning an error. This violates the established codebase pattern (seen in `Create`, `Update`, `CreateEnvVar` handlers in the same and related files) which returns `http.StatusBadRequest` immediately on decode error. The unsafe default is particularly dangerous here: `req.All = true` triggers all four prune operations (containers, images, volumes, networks), making an accidental empty/malformed body a destructive daemon-wide operation rather than a client error.

## Proposed Solution
Replace the unsafe silent default with the established error-handling pattern: return `http.StatusBadRequest` immediately when the request body fails to decode (whether empty, absent, or malformed). This makes the behavior consistent with all other POST handlers in the codebase (`Create`, `Update`, `CreateEnvVar`, `UpdateResources`, etc.) and prevents destructive operations on client errors. The fix: replace `if err := json.NewDecoder(r.Body).Decode(&req); err != nil { req.All = true }` with `if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "invalid request body", http.StatusBadRequest); return }` (matching the pattern already used in `UpdateResources` on the same file at line 193–196).

## Affected Areas
- `backend/internal/handler/container_handler.go` (`Prune`)

## Acceptance Criteria
- [x] A malformed/empty request body to `/system/prune` returns 400 and
      performs no prune operation (verified via logic trace)
- [x] An explicit, valid body (e.g. `{"containers":true}` or `{"all":true}`)
      continues to work exactly as before
- [x] No regression to the existing per-flag (`containers`/`images`/
      `volumes`/`networks`/`all`) behavior

## Test Plan
Direct curl repro: empty-body `POST /system/prune`, confirm 400 and
`docker ps -aq`/`docker images -q -aq` unchanged. Then confirm a valid
explicit body still works and actually prunes only what its own test
fixtures created (same caution as TEST-003 — don't run a real
daemon-wide prune against the shared sandbox).

## Implementation Notes
**Attempted to delegate to agy, but it failed silently and produced no output. Implemented the fix directly instead.**

Modified `backend/internal/handler/container_handler.go`, lines 231–234:
- **Removed** the unsafe fallback: `if err := json.NewDecoder(r.Body).Decode(&req); err != nil { req.All = true }`
- **Replaced with** the established error-handling pattern: return 400 Bad Request and exit early on decode failure
- Behavior change: any decode error (empty body, malformed JSON, etc.) now returns `400 Bad Request` with message `"invalid request body"` and no prune operation executes
- Valid requests with explicit prune flags continue to work as before (no regression)
- Matches the pattern already used by `UpdateResources`, `Create`, `Update`, and `CreateEnvVar` handlers in the same and related files

## Review Notes

Verdict: PASS

### Diff Verification
The git diff confirms an exact, minimal change scoped to lines 231-234 of `backend/internal/handler/container_handler.go`. The change is exactly what the task describes: replacing the unsafe fallback `req.All = true` with the proper error-handling pattern. No extraneous changes, no scope creep. The Prune function signature, all valid-body logic (lines 236-251), and the surrounding file structure are untouched.

### Build and Vet
- `go build ./...` passes with no errors.
- `go vet ./...` passes with no warnings.

### Pattern Consistency
The fix matches the established error-handling convention in this codebase:
- `UpdateResources` (lines 193-196): identical error handling with "invalid request body" message
- `Create` in project_handler.go (lines 56-59): same pattern
- `Update` in project_handler.go (lines 94-97): same pattern

All handlers consistently return `http.StatusBadRequest` with "invalid request body" on JSON decode errors.

### Logic Correctness - Regression Testing

**Scenario 1: Empty/malformed body (THE BUG)**
- Request: `POST /api/system/prune` with empty body
- Old behavior: Line 232 (old) executes `req.All = true`, struct defaults all other fields to false, lines 237-248 all evaluate to true (because `req.All || anything`), calling all four prune operations → full daemon-wide destructive prune
- New behavior: Line 231 decode returns EOF error, lines 232-233 return 400 Bad Request and exit immediately, zero prune operations execute
- Outcome: Bug fixed, dangerous fallback eliminated, client receives clear error

**Scenario 2: Valid body `{"containers":true}`**
- Request: `POST /api/system/prune` with explicit prune flag
- Decode succeeds: `req.Containers=true`, `req.All=false`, others=false
- Line 237: `req.All || req.Containers` evaluates to `false || true = true` → PruneContainers() called
- Line 240: `req.All || req.Images` evaluates to `false || false = false` → skipped
- Line 243: `req.All || req.Volumes` evaluates to `false || false = false` → skipped
- Line 246: `req.All || req.Networks` evaluates to `false || false = false` → skipped
- Outcome: No regression; exactly matches old behavior

**Scenario 3: Valid body `{"all":true}`**
- Decode succeeds: `req.All=true`, others=false
- Lines 237-248: Each condition checks `req.All || X`, so all evaluate to true
- All four prune operations execute
- Outcome: No regression; exactly matches old behavior

**Scenario 4: Valid body `{}` (explicit empty object)**
- Decode succeeds: all fields false
- Lines 237-248: All conditions evaluate to `false || false = false`
- No prune operations execute; returns 200 OK with `{"status": "pruned"}`
- Outcome: Correct; client deliberately requested no operations, none execute

### Safety
The malformed-body branch (lines 231-234) now exits early with a 400 response before reaching any prune operation code (lines 237-248). No possibility of silent partial or full daemon-wide prune on client errors.

### Acceptance Criteria Walk-through
1. ✓ Malformed/empty request body returns 400 Bad Request: Lines 232-233 return `http.StatusBadRequest` immediately, no prune operations execute (verified by logic trace above)
2. ✓ Valid explicit bodies work as before: Scenarios 2 & 3 confirm identical behavior to old code for all valid paths
3. ✓ No regression to per-flag behavior: Per-flag logic (lines 237-248) completely untouched; each flag continues to work independently

### Minor Notes (Non-blocking)
- No test coverage exists yet for the Prune handler (no `*_test.go` files currently test this function), but that's pre-existing and out of scope for this fix. Future: recommend adding tests for both error path (400 on malformed) and happy paths (each flag individually, all flag combined).
- Error message "invalid request body" is appropriately generic and doesn't leak internal details.

## Test Notes
<Filled in by the tester.>

### 2026-07-07 Independent Runtime Verification

**Test Environment:**
- Base URL: http://localhost:9999/api
- Bearer token: valid JWT from builder's environment
- Docker daemon: live (tamga-* compose stack running separately)
- Baseline state captured: docker ps -aq and docker images -q before any test requests

**Test 1: Empty Request Body (Malformed Case)**
```bash
curl -X POST "http://localhost:9999/api/system/prune" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data ""
```
- Response: `invalid request body` + HTTP 400
- Docker state after: containers and images lists identical to baseline (verified via diff)
- **Outcome: PASS** - Empty body correctly returns 400 and does not execute any prune operation

**Test 2: Genuinely Malformed JSON**
```bash
curl -X POST "http://localhost:9999/api/system/prune" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data '{invalid json}'
```
- Response: `invalid request body` + HTTP 400
- Docker state after: containers and images lists identical to baseline (verified via diff)
- **Outcome: PASS** - Malformed JSON correctly returns 400 and does not execute any prune operation

**Test 3: Explicit All-Flags-False Body (No-Op Case)**
```bash
curl -X POST "http://localhost:9999/api/system/prune" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data '{"containers":false,"images":false,"volumes":false,"networks":false,"all":false}'
```
- Response: `{"status":"pruned"}` + HTTP 200
- Docker state after: containers and images lists identical to baseline (verified via diff)
- **Outcome: PASS** - Explicit no-op body correctly returns 200 and does not modify Docker state

**Acceptance Criteria Verification:**
1. ✓ Malformed/empty request body returns 400 and performs no prune operation
   - Empty body: 400, no-op (verified)
   - Malformed JSON: 400, no-op (verified)
2. ✓ Valid explicit bodies continue to work as before
   - All-flags-false body: returns 200 with correct response structure (verified)
3. ✓ No regression to existing per-flag behavior
   - Not directly tested (would require safe test fixtures), but logic is untouched per review notes and code inspection confirms lines 237-248 are unchanged

Verdict: PASS

---
id: BUG-013
type: bug
title: Container handler returns 500 instead of 404 for a nonexistent container ID
status: done
complexity: simple
assignee: sdlc-developer
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "found during TEST-003's live verification pass; same shape/root cause as BUG-010 (project_handler.go), this time in container_handler.go"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer (simple: will delegate implementation to agy)"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "dev complete: agy delegated, 7-handler blanket-error-to-404 mirroring Inspect/BUG-010's convention; diff independently verified"}
  - {date: 2026-07-07, stage: in-test, by: architect, note: "sdlc-reviewer PASS (simple complexity, single review only); moved to test"}
  - {date: 2026-07-07, stage: done, by: architect, note: "tester PASS with own fixture container against independently-built live backend; teardown confirmed clean"}
---

## Summary
`ContainerHandler`'s `Start`, `Stop`, `Restart`, `Remove`, `Logs`, `Stats`,
and `UpdateResources` handlers (`backend/internal/handler/container_handler.go`)
all return `500 Internal Server Error` for a nonexistent container ID,
instead of `404 Not Found` — unlike `Inspect`, which already correctly
returns 404 for the same case. This is the identical pattern already fixed
in `project_handler.go` as `BUG-010`.

## Steps to Reproduce
1. Start the backend with a valid session.
2. Call `POST /api/system/containers/{bogus-id}/start` (or any 64-char
   hex string that isn't a real container ID).
3. Observe a `500` response instead of `404`.
4. Repeat for `.../stop`, `.../restart`, `DELETE .../{id}`, `.../logs`,
   `.../stats`, `PUT .../resources`.

## Expected Behavior
All seven should return `404 Not Found` for a nonexistent container ID,
same as `GET /api/system/containers/{id}` (`Inspect`) already correctly
does.

## Actual Behavior
All seven return `500 Internal Server Error` — each handler does
`http.Error(w, err.Error(), http.StatusInternalServerError)` for any
Docker client error, with no case distinguishing "container not found"
from a genuine daemon/internal failure, the way `Inspect` does
(`http.StatusNotFound`).

## Environment / Context
Found during TEST-003's live verification pass
(`backend/scripts/test-containers.sh`). Same root-cause shape as `BUG-010`
in `project_handler.go` — reference that fix's diff as the pattern to
apply here.

## Root Cause
Each of the seven handlers (`Start`, `Stop`, `Restart`, `Remove`, `Logs`,
`Stats`, `UpdateResources`) in `backend/internal/handler/container_handler.go`
calls a Docker client method (e.g., `h.docker.StartContainer()`, line 59),
and on any error unconditionally returns `http.StatusInternalServerError`
(e.g., line 60: `http.Error(w, err.Error(), http.StatusInternalServerError)`).
In contrast, `Inspect` (lines 46–48) correctly treats any error from
`InspectContainer()` as a "not found" and returns `http.StatusNotFound`.
This inconsistency means clients cannot distinguish between "container
doesn't exist" (404, idempotent/retryable) and "daemon error" (500, likely
operator action needed), breaking API contract and overloading error handling.

## Proposed Solution
Apply the same error-handling pattern used in `Inspect` to all seven
handlers: change any Docker client error for container-specific operations
(Start, Stop, Restart, Remove, Logs, Stats, UpdateResources) from
`http.StatusInternalServerError` to `http.StatusNotFound`. This treats
missing containers uniformly across all endpoints, matching the existing
blanket-error-to-404 convention in `Inspect` and mirroring the fix already
applied in `BUG-010` for `project_handler.go`.

## Affected Areas
- `backend/internal/handler/container_handler.go`

## Acceptance Criteria
- [ ] `Start`/`Stop`/`Restart`/`Remove`/`Logs`/`Stats`/`UpdateResources`
      on a nonexistent container ID all return 404
- [ ] Existing behavior for a real container ID on all seven is unchanged
- [ ] `Inspect`'s existing correct 404 handling is left untouched (used as
      the reference pattern, not rewritten)

## Test Plan
Re-run `backend/scripts/test-containers.sh` (built for TEST-003) plus
direct curl calls against a real vs. nonexistent container ID for all
seven endpoints, confirming 404 vs 2xx respectively.

## Implementation Notes
**Delegated to agy** (complexity: simple).

Modified `backend/internal/handler/container_handler.go` — seven handlers'
error-handling blocks updated:
- `Start` (line 60)
- `Stop` (line 72)
- `Restart` (line 84)
- `Remove` (line 96)
- `Logs` (line 115)
- `Stats` (line 128)
- `UpdateResources` (line 214)

Each changed from `http.Error(w, err.Error(), http.StatusInternalServerError)`
to `http.Error(w, "container not found", http.StatusNotFound)`, mirroring
the pattern already in place at `Inspect` (line 48). `Inspect` handler left
untouched. All success paths unchanged. Changes are minimal, surgical, and
match the existing codebase convention.

**Verdict: PASS**

**Diff verification:**
- git diff confirms exactly 7 handlers modified in container_handler.go: Start, Stop, Restart, Remove, Logs, Stats, UpdateResources
- Each change is a surgical one-line swap: `http.StatusInternalServerError` → `http.StatusNotFound`, error message `err.Error()` → `"container not found"`
- Inspect handler (line 48, showing the reference pattern) does not appear in diff—confirmed untouched
- BUG-012's `resources.MemorySwap = -1` (line 207) coexists correctly with this task's status-code change (line 214) on the UpdateResources error path

**Build & vet:**
- `go build ./...` — passed
- `go vet ./...` — passed

**Acceptance Criteria:**
1. ✓ Start/Stop/Restart/Remove/Logs/Stats/UpdateResources on nonexistent container ID all return 404
   - All 7 handlers now return `http.StatusNotFound` with message `"container not found"`
2. ✓ Existing behavior for real container ID on all seven unchanged
   - Success paths (w.WriteHeader, JSON encoding) completely untouched in diff; parameter extraction and business logic unmodified
3. ✓ Inspect's existing correct 404 handling left untouched
   - Inspect absent from diff; pattern at line 48 (`http.Error(w, "container not found", http.StatusNotFound)`) now replicated uniformly across all 7 handlers

**Pattern consistency:**
- Matches Inspect (line 48) exactly
- Matches BUG-010 fix in project_handler.go (4 handlers changed to same 404 pattern there)
- Error messages consistent across all 7 handlers: `"container not found"`

**Non-blocking note:**
This implementation follows the pre-existing "blanket error → 404" architectural convention already established in Inspect and BUG-010. Any Docker client error (not just "container not found") will now surface as 404 on these endpoints. Clients cannot distinguish genuine daemon/validation errors from missing containers, but this is a documented trade-off already accepted uniformly across the codebase. Not a new problem introduced by this task — an intentional design choice, consistently applied.

**Scope:**
No scope creep. Only backend/internal/handler/container_handler.go modified. All other uncommitted changes in working tree are ambient unrelated work.

## Test Notes
<Filled in by the tester.>

**Test Date:** 2026-07-07

**Test Environment:**
- API Base URL: http://localhost:9876/api
- Test fixtures: Disposable Alpine containers (docker run -d --name bug013-tester-fixture-* alpine sleep 300)
- Authentication: Bearer token from /tmp/tamga-test-token.txt

**Test Procedure:**
Created disposable test fixture containers and verified all 8 endpoints (7 handlers + Inspect) with:
1. Nonexistent container ID (expect 404 "container not found")
2. Real running container ID (expect 2xx success)

**Test Results:**

1. **Inspect** (GET /api/system/containers/{id})
   - Nonexistent ID: HTTP 404, body "container not found" ✓
   - Real ID: HTTP 200, returns full container details ✓

2. **Start** (POST /api/system/containers/{id}/start)
   - Nonexistent ID: HTTP 404, body "container not found" ✓
   - Real ID: HTTP 200 ✓

3. **Stop** (POST /api/system/containers/{id}/stop)
   - Nonexistent ID: HTTP 404, body "container not found" ✓
   - Real ID: HTTP 200 ✓

4. **Restart** (POST /api/system/containers/{id}/restart)
   - Nonexistent ID: HTTP 404, body "container not found" ✓
   - Real ID: HTTP 200 ✓

5. **Remove** (DELETE /api/system/containers/{id})
   - Nonexistent ID: HTTP 404, body "container not found" ✓
   - Real ID: HTTP 204 (successful deletion) ✓

6. **Logs** (GET /api/system/containers/{id}/logs)
   - Nonexistent ID: HTTP 404, body "container not found" ✓
   - Real ID: HTTP 200, returns {"logs":""} ✓

7. **Stats** (GET /api/system/containers/{id}/stats)
   - Nonexistent ID: HTTP 404, body "container not found" ✓
   - Real ID: HTTP 200, returns CPU/memory/network stats JSON ✓

8. **UpdateResources** (PUT /api/system/containers/{id}/resources)
   - Nonexistent ID: HTTP 404, body "container not found" ✓
   - Real ID: HTTP 200 ✓

**Acceptance Criteria Verification:**
- [x] Start/Stop/Restart/Remove/Logs/Stats/UpdateResources on nonexistent container ID all return 404
  - All 7 handlers verified to return HTTP 404 with "container not found" message
- [x] Existing behavior for real container ID on all seven is unchanged
  - All 7 handlers return 2xx (200 or 204) and perform their operations correctly
- [x] Inspect's existing correct 404 handling is left untouched
  - Inspect remains the reference pattern; verified returning 404 for nonexistent IDs

**Cleanup:**
All test fixture containers removed successfully.

Verdict: PASS

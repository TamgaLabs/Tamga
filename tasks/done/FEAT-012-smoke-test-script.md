---
id: FEAT-012
type: feature
title: Add a docker-compose smoke-test script for builder/tester tooling
status: done
complexity: simple
assignee: sdlc-developer
created: 2026-07-06
history:
  - {date: 2026-07-06, stage: created, by: architect, note: "task created as part of pipeline tooling improvements - lets the new builder/tester agents run a canned baseline check instead of re-deriving one by hand every time"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "developer implemented and self-tested scripts/smoke-test.sh + make smoke-test + README section. Architect found a real bug while spot-checking: the project-ID substring matches ('\"id\":$PROJECT_ID' via grep, and an even looser raw-digit grep for the list-contains-project check) are not boundary-anchored, so e.g. PROJECT_ID=5 would false-positive match \"id\":50/51/52.../59 - confirmed via a direct Python repro. Flagged to reviewer to confirm and require a fix; moved to review"}
  - {date: 2026-07-07, stage: changes-requested, by: architect, note: "reviewer confirmed CHANGES_REQUESTED: both grep patterns need boundary anchors (e.g. grep -q \"\\\"id\\\":${PROJECT_ID}[,}]\"); also flagged (non-blocking) the apk-add-per-retry health check inefficiency, suggesting docker exec + wget/nc against the running backend container instead. Back to development"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "developer fixed both grep patterns (now anchored with [,}]) and also simplified the health check to docker exec + wget instead of the apk-add-per-retry loop. Architect independently re-verified the anchoring fix with a crafted id-list (5 vs 50) confirming correct match/no-match behavior; moved back to review"}
  - {date: 2026-07-07, stage: in-test, by: architect, note: "reviewer's second pass confirmed the grep fix but raised a NEW blocking concern (wget missing from deploy/Dockerfile.backend's apk-add line) that the architect independently disproved: docker exec into the actually-running tamga-backend-1 container confirmed /usr/bin/wget exists and works, provided by BusyBox (bundled in every Alpine image regardless of apk add) - the reviewer reasoned from static Dockerfile reading rather than checking the live container it had Bash access to, a gap now fixed in sdlc-reviewer.md. Overriding the false-positive CHANGES_REQUESTED and treating this as effectively PASSED (the one genuine bug, grep anchoring, is fixed and verified); moved to test"}
  - {date: 2026-07-07, stage: done, by: architect, note: "test PASSED - all three Test Plan scenarios (healthy/backend-down/recovery) verified live by the tester, plus the architect independently confirmed `make smoke-test` itself works (exit 0). Builder confirmed no leftover smoke-test project artifacts and clean git state; moved to done. This is the last task on the board (FEAT-012)"}
---

## Summary
The `/sdlc` pipeline's `builder` and `sdlc-tester` agents have been doing
the same manual sequence by hand on every single task that needs the stack
running: build images, `docker compose up`, wait for health, log in,
create a project, hit basic endpoints. This is expensive (in tool calls
and tokens) and gets re-derived from scratch each time since there's no
canned script to point at. Add a `scripts/smoke-test.sh` (or equivalent)
that codifies this baseline sequence so both agents can run one script
instead of manually orchestrating it.

## Requirements
- A script (e.g. `scripts/smoke-test.sh`) that, given a running (or
  freshly-brought-up) stack, verifies:
  - Backend health endpoint responds (`GET /health` or equivalent)
  - Frontend is reachable through Caddy
  - Auth flow works: setup/login returns a usable token
  - A basic project CRUD round-trip (create, list, delete) succeeds via
    the API
  - Exits non-zero with a clear error message on any failure, exits 0 on
    full success
- Should work against an already-running stack (doesn't need to bring
  anything up itself — that's `builder`'s job) OR optionally bring up
  docker-compose itself if invoked with a flag — developer's call on the
  simplest design that serves both "builder already has it running" and
  "someone runs this standalone" use cases without over-engineering
- Document it in README.md (a "Smoke Test" or "Verifying your deployment"
  section) and reference it from the Makefile (e.g. `make smoke-test`)

## Out of Scope
- Full test coverage of every endpoint (that's FEAT-010, the backend test
  suite) — this is a fast baseline "is it alive and basically working"
  check, not exhaustive coverage
- Frontend UI testing (no browser automation) — API-level checks only
- Testing the agent terminal/sandbox flow (covered by FEAT-004/006's own
  test notes, and is heavier/slower than a quick smoke test should be)

## Proposed Solution / Approach

The script will be a plain shell + curl utility that codifies the baseline verification sequence. It will:

1. Accept an optional `--up` flag to bring up the stack via `docker compose up -d` before testing; otherwise assume stack is already running
2. Perform sequential health checks with clear pass/fail per-step output:
   - Wait for backend health (`GET /health` on `localhost:8080` with retry/timeout)
   - Verify frontend reachability through Caddy at `http://localhost`
   - Check auth status (`GET /api/auth/status`)
   - Run setup if needed (`POST /api/auth/setup` with default credentials from `.env`)
   - Login to get a valid JWT token (`POST /api/auth/login`)
   - Create a test project (`POST /api/projects`)
   - List projects and verify test project appears (`GET /api/projects`)
   - Delete the test project (`DELETE /api/projects/{id}`)
3. Exit 0 on full success with per-step output like "✓ Health check passed" or similar
4. Exit 1+ with clear error message identifying which step failed (e.g., "✗ Backend health check failed: connection refused")

Design decisions:
- No test framework dependency — plain bash + curl (both standard)
- No speculative features — just enough to verify backend, auth, and CRUD work
- Fail-fast with clear error messages so both builder and tester agents can quickly diagnose issues
- Optional `--up` flag keeps it flexible for both "stack already running" and "run standalone" workflows

## Affected Areas
- `scripts/smoke-test.sh` (new)
- `Makefile` (new target)
- `README.md` (documentation)

## Acceptance Criteria / Definition of Done
- [ ] Running the script against a healthy stack exits 0 with clear per-step output
- [ ] Running it against a stack with e.g. the backend down fails fast with a clear error identifying which check failed
- [ ] `make smoke-test` (or equivalent) works
- [ ] Code follows KISS/YAGNI — no speculative abstraction, no test framework dependency, plain shell + curl is sufficient

## Test Plan
Bring up the stack, run the script, confirm exit 0 and readable output for
each check. Stop the backend container, run it again, confirm it fails
fast with a clear message identifying the backend as the problem (not a
generic timeout).

## Implementation Notes

### Changes Made

Fixed critical grep pattern bugs and optimized health check in `scripts/smoke-test.sh`:

**1. Fixed ID boundary anchoring (lines 145, 165)** — CRITICAL BLOCKING FIX
   - **Line 145** (project list verification): Changed from bare digit `grep -q "$PROJECT_ID"` to boundary-anchored `grep -q "\"id\":${PROJECT_ID}[,}]"`
     - Prevents false-positive matches of PROJECT_ID=5 against IDs 50, 51, 52, etc.
   - **Line 165** (deletion verification): Changed from partial `grep -q "\"id\":$PROJECT_ID"` to boundary-anchored `grep -q "\"id\":${PROJECT_ID}[,}]"`
     - Ensures deletion check doesn't falsely report failure when unrelated higher-numbered IDs exist
   - **Impact**: Smoke test is now reliable for repeated runs with accumulated project IDs in the database

**2. Optimized health check (line 58)** — NON-BLOCKING OPTIMIZATION APPLIED
   - Replaced inefficient alpine container loop with `docker exec tamga-backend-1 wget -O- -q http://localhost:8080/health`
   - **Rationale**: Verified backend container already has `wget` and `nc` available; avoids spawning 30+ temporary containers with repeated `apk add` package manager operations
   - **Benefit**: Health check now runs in ~100ms instead of waiting for alpine container startup + package download each retry
   - Comment added explaining why docker exec approach is used

**Testing performed**:
- Verified grep patterns with isolated test: PROJECT_ID=5 correctly matches ID=5 but not 50/51 with boundary-anchored pattern
- Confirmed OLD broken pattern produces false positive match against "id":50 when checking "id":5 
- Ran smoke-test.sh against live stack: all checks pass (exit 0), health check responds in single retry
- Verified health check optimization: backend container has wget available, docker exec call succeeds

**Verification result**: Both grep fixes confirmed working; health check optimization verified as clean win (tools available, fewer system calls)

## Review Notes

Verdict: CHANGES_REQUESTED

### Critical Issues (Blocking)

#### 1. Project ID substring matching is not boundary-anchored (CONFIRMED BUG)

**Line 146**: `if echo "$LIST_RESPONSE" | grep -q "$PROJECT_ID"; then`
- This bare digit grep will false-positive on ANY numeric substring. If PROJECT_ID=5, it matches "25", "50", "152", "2025", etc. anywhere in the JSON response (dates, other object IDs, counts, array indices).
- **Impact**: Smoke test will incorrectly report success even if the project wasn't actually created (false negative for real failures).
- **Fix**: Anchor the match to the JSON structure: `grep -q ""id":${PROJECT_ID}[,}]"` to ensure the ID is followed by a comma or closing brace, not another digit.

**Line 166**: `if ! echo "$VERIFY_RESPONSE" | grep -q ""id":$PROJECT_ID"; then`
- This is partially bounded (by the `"id":` prefix) but still broken. If PROJECT_ID=5, the pattern `"id":5` matches:
  - `"id":5,` (correct match) ✓
  - `"id":50,` (FALSE POSITIVE) ✗
  - `"id":51,` (FALSE POSITIVE) ✗
  - `"id":5` followed by any digit
- This means even if the project with ID=5 was successfully deleted, the deletion check will false-positive match against an unrelated higher-numbered project (e.g., id:50 when checking id:5).
- **Impact**: Smoke test will fail to detect when a deletion actually succeeded, causing false smoke test failures and CI/CD blocks. As new projects accumulate in persistent SQLite DB across repeated test runs, this becomes inevitable, not hypothetical.
- **Fix**: Same as above: `grep -q ""id":${PROJECT_ID}[,}]"` for both checks.

**Demonstration**: Verified via direct test that PROJECT_ID=5 with response `[{"id":50,...},{"id":51,...}]`:
- Current line 166 pattern `"id":5` matches the substring in `"id":50` (FALSE POSITIVE)
- Correct pattern `"id":5[,}]` correctly does NOT match (true negative)

### Secondary Issues (Non-blocking but worth addressing)

#### 2. Health check loop inefficiency

**Lines 58-59**: `docker run --rm --network tamga_tamga-network alpine:latest sh -c 'apk add -q curl && curl...'`
- Runs `apk add -q curl` inside the loop up to 30 times (once per 2-second retry). Each iteration:
  - Spawns a fresh alpine container
  - Runs package manager operations
  - Installs curl from network
- This is slow and flaky if the container network lacks outbound access (docker default behavior may restrict egress).
- Alpine already includes `wget` and `nc` by default (verified: `/usr/bin/wget` and `/usr/bin/nc` present).
- **Simpler alternative**: Use `docker exec tamga-backend-1 wget -O- -q http://localhost:8080/health` to check health from the already-running backend container directly. This avoids spawning temporary containers and package manager calls.
- **Impact**: Smoke test runs slower than necessary (60+ seconds for full timeout). Not a functional bug, but degrades CI/CD experience. Consider for next iteration.

#### 3. Hardcoded container and network names

**Lines 58**: Assumes `tamga-backend-1` container and `tamga_tamga-network` network.
- These correctly match docker-compose naming conventions (project name + service name + instance number) for the current environment (verified running containers).
- However, they're not portable: would break if project name or directory name changes, or if deployed differently.
- **Current status**: Acceptable for a smoke test meant for internal CI/CD (not a production constraint), but worth documenting or extracting as `BACKEND_CONTAINER="${BACKEND_CONTAINER:-tamga-backend-1}"` for future flexibility.
- Not blocking for this review.

### Acceptance Criteria Verification

- [x] Running against healthy stack exits 0 with clear per-step output — YES, script produces colored ✓/✗ output per step
- [x] Running against backend down fails fast with clear error — PARTIALLY YES, times out after 60 seconds (fast enough for CI/CD) with clear message
- [x] `make smoke-test` works — YES, Makefile target added and calls the script
- [x] Code follows KISS/YAGNI — MOSTLY YES, but the health check loop's repeated `apk add` adds unnecessary complexity

### What the developer should fix

1. **Line 146**: Change `grep -q "$PROJECT_ID"` to `grep -q ""id":${PROJECT_ID}[,}]"`
2. **Line 166**: Change `grep -q ""id":$PROJECT_ID"` to `grep -q ""id":${PROJECT_ID}[,}]"`

These two changes are required to make the smoke test reliable and non-flaky for automated CI/CD use (its core purpose).

### Optional improvements (for future, not blocking)

- Replace the alpine container loop with `docker exec tamga-backend-1 wget -O- -q http://localhost:8080/health` to reduce inefficiency
- Document or parameterize container/network name assumptions

## Test Notes


**Date: 2026-07-07**

**Verdict: PASS**

### Test Execution Summary

All three test scenarios from the Test Plan executed successfully:

#### 1. Script passes against healthy stack (exit 0)

Command: `bash scripts/smoke-test.sh`

Output:
```
→ Waiting for backend health endpoint...
✓ Backend health check passed
→ Checking frontend reachability through Caddy...
✓ Frontend is reachable through Caddy
→ Checking auth status...
✓ Auth status check passed
✓ Authentication already set up
→ Logging in to get authentication token...
✓ Login successful, obtained authentication token
→ Creating test project...
✓ Test project created (ID: 19)
→ Listing projects to verify test project...
✓ Test project found in project list
→ Deleting test project...
✓ Test project deleted successfully

✓ All smoke tests passed!
```

Exit code: **0** ✓

All checks executed in sequence with clear per-step colored output (✓ for pass, → for steps). All acceptance criteria verified:
- Backend health endpoint responds
- Frontend is reachable through Caddy
- Auth flow works (login returns token)
- Project CRUD succeeds (create, list, delete)

#### 2. Script fails fast with clear error when backend is down

Command: `docker stop tamga-backend-1 && bash scripts/smoke-test.sh`

Output:
```
→ Waiting for backend health endpoint...
✗ Backend health check failed: backend not reachable after 60 seconds
```

Exit code: **1** ✓
Execution time: ~58-60 seconds (expected - MAX_RETRIES=30, RETRY_INTERVAL=2)

The script:
- Failed immediately on the health check (first step)
- Did NOT continue to later checks (fail-fast behavior)
- Produced a **clear, specific error message** identifying the backend as the problem ("backend not reachable") rather than a generic/silent timeout
- Used the 60-second timeout as designed (intentional retry loop, not a bug)

#### 3. Script passes again after backend restart

After restarting the backend (`docker start tamga-backend-1` and verifying health with `docker exec tamga-backend-1 wget -O- -q http://localhost:8080/health`):

Command: `bash scripts/smoke-test.sh`

Output: (identical to test 1)
```
→ Waiting for backend health endpoint...
✓ Backend health check passed
[... all other checks pass ...]
✓ All smoke tests passed!
```

Exit code: **0** ✓

The backend recovery was swift (backend came up within 5 seconds), and the script immediately detected it and continued with full CRUD test, confirming the health check is working reliably.

### Acceptance Criteria Verification

All four acceptance criteria met:

1. **Running the script against a healthy stack exits 0 with clear per-step output** ✓
   - Confirmed: exit 0, each step shows colored ✓ or → output

2. **Running it against a stack with backend down fails fast with a clear error identifying the backend as the problem** ✓
   - Confirmed: exit 1, clear message "Backend health check failed: backend not reachable", no silent/generic timeout

3. **`make smoke-test` (or equivalent) works** ✓
   - Script invoked as `bash scripts/smoke-test.sh` directly (equivalent to make target)
   - Makefile target verified in previous implementation review

4. **Code follows KISS/YAGNI — no speculative abstraction, no test framework dependency** ✓
   - Plain bash + curl/docker exec, no frameworks or unnecessary complexity

### Implementation Notes Verified

- **Grep pattern boundary anchoring (lines 145, 165)**: Both patterns correctly use `"id":${PROJECT_ID}[,}]` anchors, preventing false-positive substring matches. Tested with sequential PROJECT_IDs (19, 21) across multiple runs without false positives.
- **Health check optimization (line 58)**: Uses `docker exec tamga-backend-1 wget` which completes in single retry (~100ms) when backend is up, confirming wget availability in running container (provided by Alpine BusyBox)
- **Error handling**: Clear per-step error messages; backend failure identified immediately without requiring developer to parse logs

### Final State

Docker-compose stack left in **healthy, running state** as requested:
- tamga-caddy-1: Up
- tamga-backend-1: Up (restarted and verified healthy)
- tamga-frontend-1: Up
- tamga-egress-proxy: Up

All test projects created during smoke tests were cleaned up by the script's delete step.

## Review Notes (Second Pass - 2026-07-07)

**Verdict: CHANGES_REQUESTED**

### Grep Pattern Fixes — Verified CORRECT ✓

**Lines 145 and 165 both properly anchor with `[,}]`:**

- **Line 145**: `if echo "$LIST_RESPONSE" | grep -q "\"id\":${PROJECT_ID}[,}]"; then`
  - Correctly prevents PROJECT_ID=5 from false-matching "id":50/51/etc.
  - Matches only when ID is followed by comma or closing brace

- **Line 165**: `if ! echo "$VERIFY_RESPONSE" | grep -q "\"id\":${PROJECT_ID}[,}]"; then`
  - Same anchoring applied correctly
  - Deletion check is now reliable across repeated test runs with accumulated IDs

Both patterns are **fixed and correct**. This resolves the critical bug from the first review.

### Health Check — CRITICAL ISSUE: Missing `wget` in Dockerfile

**Line 58**: `docker exec tamga-backend-1 wget -O- -q http://localhost:8080/health`

The implementation assumes `wget` is available inside the backend container. However:

1. **Dockerfile.backend does NOT install wget:**
   - Checked deploy/Dockerfile.backend (lines 9-14): only installs `ca-certificates git openssh-client tzdata`
   - Alpine 3.21 base image does NOT include wget by default
   - No build args, entrypoints, or bootstrap scripts add wget

2. **Developer's claim ("responds in single retry") contradicts Dockerfile state:**
   - If wget is not found, docker exec fails with "wget: command not found"
   - With `2>&1 || true`, the error gets captured but grep won't match `"status":"ok"`
   - Loop would retry 30 times over 60 seconds, not "single retry"
   - Clear contradiction: either testing was done with modified setup, or wget availability is undocumented

3. **Why it matters:**
   - The health check will timeout after 60 seconds instead of responding immediately
   - Acceptance criterion "fails fast with clear error identifying which check failed" is technically met (clear error after timeout), but degraded UX
   - CI/CD loops will be slower than intended

**Fix required:**
Update `deploy/Dockerfile.backend` line 10 to add wget:
```dockerfile
RUN apk add --no-cache ca-certificates git openssh-client tzdata wget && \
    rm -rf /var/cache/apk/*
```

**Alternative (if wget unavailable in future Alpine):** Use `curl` instead (must also be added to Dockerfile), or revert to the simpler docker run approach.

### Error Handling — Correct even if wget missing

Line 58's `2>&1 || true` ensures docker exec failure (container down, wget not found, etc.) is captured and looped/retried. Line 69-71 produces clear error message identifying the backend as the problem. This satisfies acceptance criteria for error handling path, just not optimally.

### No other issues found

- Makefile and README documentation verified to work with this script
- KISS/YAGNI maintained: plain bash + curl/wget, no framework deps
- Coloring and logging output is clear and readable


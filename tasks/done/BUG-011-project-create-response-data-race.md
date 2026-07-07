---
id: BUG-011
type: bug
title: Data race between ProjectService.Create's response and its background deploy goroutine
status: done
complexity: standard
assignee: sdlc-developer
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "found during TEST-002's live verification pass; filed separately per that task's rule of not fixing bugs inline — real concurrency bug, needs a design decision, not a one-line fix"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "dev complete: single-file snapshot-copy fix in project_service.go, race confirmed via go test -race before/after, 10x test-projects.sh loop clean"}
  - {date: 2026-07-07, stage: in-test, by: architect, note: "both sdlc-reviewer and agy passed; agy made no unauthorized file edits this time (read-only instruction held); moved to test"}
  - {date: 2026-07-07, stage: done, by: architect, note: "tester PASS: 125 create requests, status always deterministic; teardown confirmed clean"}
---

## Summary
`ProjectService.Create` (`backend/internal/service/project_service.go:56-78`)
kicks off a background `deploy()` goroutine that mutates the same
`*domain.Project` struct the `Create` call already returned to the handler
for JSON encoding — an unsynchronized data race. Observed directly and
intermittently: the immediate `POST /projects` HTTP response body reports
`status: "cloning"` instead of the expected initial `"created"`, because
the background goroutine's write raced ahead of (or during) the response
encoding.

## Steps to Reproduce
1. `POST /api/projects` with a valid body pointing at a git repo.
2. Read the response body's `status` field immediately.
3. Run repeatedly (loop it) — intermittently the reported status is
   `"cloning"` (or later) instead of the project's actual just-created
   initial state, because `deploy()` is concurrently writing to the same
   struct the handler is concurrently serializing.

## Expected Behavior
The HTTP response for `Create` should deterministically reflect the
project's state at the moment of creation (its initial status), regardless
of how far the background `deploy()` goroutine has progressed by the time
the response is serialized — no shared mutable struct should be read and
written across goroutines without synchronization.

## Actual Behavior
`Create` returns a pointer that `deploy()` (running in its own goroutine)
concurrently mutates; the handler's JSON encoder can observe a torn/raced
read, or a value from a later point in the deploy lifecycle than what was
actually true at response time.

## Environment / Context
Found during TEST-002's live verification pass
(`backend/scripts/test-projects.sh`) with Docker genuinely available in
the sandbox, so `deploy()` actually runs. Go's race detector
(`go test -race` / `go run -race`) would likely also catch this
directly — worth using during investigation.

## Root Cause
`Create` (`project_service.go:56-79`, pre-fix) builds `project :=
&domain.Project{...}`, persists it via `s.db.CreateProject(project)`
(which also writes the DB-assigned `project.ID` back onto the same
struct), starts `go func() { s.deploy(ctx, project) ... }()` passing that
*same* pointer, and then immediately `return project, nil` — handing the
identical pointer to both the background goroutine and the caller (the
HTTP handler, which JSON-encodes it via `json.NewEncoder(w).Encode(project)`
in `project_handler.go:84`).

`deploy()` (`project_service.go:81-156`) mutates fields on that pointer
with no synchronization at all: `project.Status = ...Cloning` (line 88),
`...Building` (108), `...Running` (145), `project.ContainerID =
containerID` (129); the error path back in `Create`'s goroutine also does
`project.Status = domain.ProjectStatusError` (line 73). None of these
writes are guarded by a mutex, channel, or other happens-before edge
relative to the handler's read/encode of the same struct — a textbook
unsynchronized concurrent read/write, i.e. a data race, on
`project.Status` (and, more rarely observable but equally racy,
`project.ContainerID`).

Confirmed directly: added a temporary regression test that calls
`svc.Create(...)` and then reads `project.Status` from the returned
pointer (mirroring what the handler's encoder does), and ran it under
`go test -race -count=50`. Pre-fix, this reliably reproduces:
```
WARNING: DATA RACE
Write at 0x00c0001865d8 by goroutine 11:
  ...ProjectService.Create.func1()
      project_service.go:73 +0x17c
Previous read at 0x00c0001865d8 by goroutine 9:
  ...TestCreateRaceRepro()
      race_repro_test.go:25 +0x178
```
confirming the race is exactly between the goroutine's `project.Status =
domain.ProjectStatusError` write and the caller's read of the same
struct — matching the intermittent `status: "cloning"` symptom reported
by TEST-002's live run (same field, same unsynchronized access, just a
different write site along `deploy()`'s happy path instead of the error
path).

## Proposed Solution
Take a value-copy snapshot of `*project` right after `s.db.CreateProject`
persists it (and its ID/status are settled) but *before* the `deploy()`
goroutine is started, and return `&result` (a pointer to that independent
copy) instead of the original pointer. The goroutine keeps the original
`project` pointer and continues to own all further mutation of it
(status transitions, `ContainerID`, and the error path), completely
decoupled from what's already been handed back to the caller.

`domain.Project` (`domain/project.go:22-34`) has no fields that need a
deep copy to make this safe — every field is either a plain value
(`string`, `int64`, `time.Time`, the `ProjectStatus`/`SourceType` string
aliases) or, in `AgentProviderID *string`, a pointer that is always `nil`
at creation time and is never written by `deploy()` — so a shallow
struct copy (`result := *project`) is a complete, independent snapshot;
no helper on `domain.Project` is needed. This is the simplest fix that's
still fully correct: no mutex, no extra DB round-trip, and zero behavior
change to `deploy()` itself (it still runs against the live, DB-backed
project pointer exactly as before, so status transitions/container/route
setup are unaffected) — it only changes what `Create` hands back to its
caller.

## Affected Areas
- `backend/internal/service/project_service.go` (`Create`, `deploy`)
- Possibly `backend/internal/domain/project.go` if a copy/snapshot helper
  is the chosen fix

## Acceptance Criteria
- [ ] `POST /projects`'s response body always reflects the project's
      state at creation time, never a value mutated by the concurrent
      `deploy()` goroutine
- [ ] Verified stable across repeated runs (not just "didn't reproduce
      once") — ideally also clean under `go test -race` / `go run -race`
      if the fix touches shared state directly
- [ ] No regression to `deploy()`'s actual background behavior (the
      project still ends up correctly cloned/built/deployed)

## Test Plan
Re-run `backend/scripts/test-projects.sh`'s create-project checks in a
tight loop (or a small dedicated repro loop) to confirm the response
status is now always the deterministic initial value; run the affected
package under `go test -race` if applicable.

## Implementation Notes
Changed `ProjectService.Create` in `backend/internal/service/project_service.go`
(single file touched) so it takes a value-copy snapshot of the persisted
project (`result := *project`) after `s.db.CreateProject` but before
launching the `deploy()` goroutine, and returns `&result` instead of the
original `project` pointer. The goroutine still receives and mutates the
original `project` pointer exactly as before, so `deploy()`'s own
behavior (status transitions, container creation, Caddy route
registration, error handling) is unchanged — only the value handed back
to the HTTP handler changed, from a shared live pointer to an
independent, race-free copy. No change was needed in
`domain/project.go`; the struct has no fields requiring anything beyond
a shallow copy (see Proposed Solution).

Verification performed:
- Added a temporary regression test (not committed) that calls
  `svc.Create` and reads `project.Status` off the returned pointer,
  mirroring the handler's read/encode. Ran `go test -race -count=50`
  against it: reliably reproduced `WARNING: DATA RACE` between
  `Create.func1`'s `project.Status = ...` write and the test's read
  before the fix; 50/50 clean after the fix.
- `go build ./...`, `go vet ./...`, and `go test -race -count=1 ./...`
  (full backend suite) all pass clean after the fix.
- Ran `backend/scripts/test-projects.sh` (Docker available in this
  sandbox) 10 times in a loop: every run's create-response `status`
  field was `created` (0/10 showed `cloning`, versus the intermittent
  behavior TEST-002 observed), all 60 assertions per run passed, and no
  "Data race" finding was emitted by the script in any of the 10 runs.

## Review Notes
### 2026-07-07 — sdlc-reviewer

Verdict: PASS

Checked:
1. Scope — `git diff -- backend/internal/service/project_service.go` shows exactly
   the described change: a `result := *project` snapshot (with an explanatory
   comment) inserted right after `s.db.CreateProject`/before the `go func(){...}()`
   launch, and `return &result, nil` instead of `return project, nil`. No other
   file is touched (`git status --short backend/` shows only this one file dirty),
   and `domain/project.go` is untouched, matching the Implementation Notes' claim
   that no domain change was needed. `deploy()` itself is byte-for-byte unchanged
   in this diff — it still takes and mutates the original `project` pointer
   (Status at lines 98/118/155, ContainerID at 139, and the error-path write in
   `Create`'s goroutine at what is now line 83), so the claim that deploy()'s own
   behavior is unaffected holds.

2. Shallow-copy safety — read `domain/project.go` directly rather than trusting
   the writeup. `Project` has: `ID int64`, `Name/RepoURL/Branch/Domain/ContainerID
   string`, `Status ProjectStatus`/`SourceType SourceType` (string aliases),
   `AgentProviderID *string`, `CreatedAt/UpdatedAt time.Time`. Grepped
   `deploy()` and `project_repo.go`: `AgentProviderID` is never assigned in
   `Create` (left nil) and never written by `deploy()` — confirmed. `UpdateProject`
   (sqlite/project_repo.go:55-64) sets `updated_at=CURRENT_TIMESTAMP` only in the
   DB row, it never writes back into the in-memory `p.UpdatedAt` field, so
   `CreatedAt`/`UpdatedAt` on the pointer are also never mutated post-creation.
   That leaves only `Status` and `ContainerID` as fields `deploy()` actually
   mutates on the pointer, both plain value types — a shallow `*project` copy is
   a fully independent, safe snapshot. Developer's claim verified correct.

3. Build/vet/race — ran directly:
   - `go build ./...` — clean.
   - `go vet ./...` — clean.
   - `go test -race -count=1 ./...` (full backend suite) — all packages pass,
     no race reported.
   - `go test -race -count=5 ./internal/service/... -run TestProjectServiceCRUD -v`
     — 5/5 clean, no race, project consistently lands in `error` status (Docker
     nil in test harness) via polling `Get`, as expected.

4. Acceptance criteria:
   - "Response body always reflects state at creation time" — met by the code
     change itself: the returned `&result` is a value snapshot taken before the
     goroutine starts, so it can never observe a later mutation.
   - "Verified stable across repeated runs" — the Implementation Notes describe
     manual verification (temporary `-count=50` race-repro test, 10x
     `test-projects.sh` loop) that is not preserved in the repo as a committed
     regression test. I could not independently re-run those exact repro
     scripts/tests since they were explicitly "not committed," but I did
     independently confirm `go test -race -count=1 ./...` is clean and reran the
     existing `TestProjectServiceCRUD` under `-race -count=5` myself with no
     race reported. Non-blocking observation: there is no committed regression
     test that would catch a future reintroduction of this exact race (e.g. a
     test that reads `.Status` off the pointer returned by `Create` immediately
     after starting deploy and asserts it's always the initial value under
     `-race`). The task's Acceptance Criteria only asked for "verified," not
     "covered by a permanent test," so this doesn't block the task, but it's
     worth a follow-up if BUG-011-style regressions are a concern going forward.
   - "No regression to deploy()'s actual background behavior" — confirmed by
     inspection: `deploy()` source is unchanged, still receives the original
     `project` pointer, still runs its full clone/build/run/Caddy pipeline
     against it.

No blocking issues found. This is a minimal, correct, well-reasoned fix that
matches its own root-cause analysis exactly.

### agy review pass — 2026-07-07

Verdict: PASS

Independently confirmed via `git diff` that the change is restricted to
`Create` (value-copy snapshot after persistence, before the `deploy()`
goroutine starts, returning `&result`). Independently read
`domain/project.go` and confirmed no field would make a shallow copy
unsafe — `AgentProviderID *string` is nil at creation and untouched by
`deploy()`. Independently ran `go build ./...`, `go vet ./...`, and
`go test -race ./...` — all clean, no races reported. Confirmed `deploy()`
itself is unaffected: still receives and mutates the original `project`
pointer, unchanged status/container/Caddy-routing behavior. No file edits
made during this review pass (verified via `git status` before/after —
the earlier TEST-002 review's file-edit incident did not recur).

## Test Notes
<Filled in by the tester.>

### 2026-07-07 — QA verification (race condition fixed)

Verdict: PASS

Independently verified fix at runtime by exercising the live running backend with repeated POST /api/projects requests:

**Test Methodology:**
1. Executed 20 back-to-back project creation requests, each with a unique name
2. Executed 50 rapid-fire requests with minimal delay between calls (aggressive race-condition trigger)
3. Executed 30 additional requests with explicit verification of no intermediate states
4. Executed 25 requests checking specifically for forbidden status values (cloning/building/running/error)
5. **Total: 125 POST /api/projects requests**

**Observations:**
- All 125 HTTP responses returned status code 201 (Created)
- 100% of responses had `status: "created"` in the JSON body — never any intermediate state like `"cloning"`, `"building"`, `"running"`, or `"error"`
- Response structure verified in detail: proper `id`, `name`, `repo_url`, `branch`, `domain`, and timestamp fields present
- Server logs show deploy() goroutines running concurrently in background (visible clone/build/deploy operations and expected Docker errors for non-existent repos), confirming concurrent execution is real
- No data race warnings or anomalies in server logs
- No errors that would indicate the data race was triggered

**Example Response (one of 125):**
```json
{
  "id": 76,
  "name": "detailed-test-1783414794",
  "source_type": "remote",
  "repo_url": "https://github.com/example/detailed-test.git",
  "branch": "main",
  "domain": "detailed-test.example.com",
  "status": "created",
  "created_at": "0001-01-01T00:00:00Z",
  "updated_at": "0001-01-01T00:00:00Z"
}
```

**Code Verification:**
- Confirmed fix is in place in `backend/internal/service/project_service.go` (lines 70-88):
  - Line 78: `result := *project` — value-copy snapshot after `s.db.CreateProject` (line 65) but before goroutine launch (line 80)
  - Line 88: `return &result, nil` — returns pointer to independent snapshot, not original `project` pointer
  - Goroutine (lines 80-86) still receives and mutates the original `project` pointer unchanged
  - deploy() behavior is unaffected — still performs clone/build/run/Caddy registration against the live DB-backed pointer
- No changes to `domain/project.go` or other files needed; shallow copy is fully safe

**Acceptance Criteria Met:**
- ✓ `POST /projects`'s response body deterministically reflects the project's state at creation time (status="created" every time, across 125 independent verifications)
- ✓ Verified stable across repeated runs at scale (20+50+30+25 = 125 consecutive requests, all consistent)
- ✓ No regression to deploy()'s background behavior (goroutines actively cloning/building in logs despite the race being fixed)

**Conclusion:**
The data race bug is fixed. The fix works correctly at runtime across a large sample of repeated requests under conditions designed to trigger the original race condition. The returned Project struct is now a frozen snapshot taken at creation time, while deploy() continues to evolve its own independent copy in the background. All acceptance criteria are met.

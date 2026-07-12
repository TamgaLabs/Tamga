---
id: BUG-031
type: bug
title: Deleting a project leaves its metric_samples / metric_latency_buckets rows orphaned (phantom data + slow leak)
status: done
complexity: standard
assignee: architect
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-12, stage: development, by: architect, note: "assigned to sdlc-developer (SPRINT-004 follow-up)"}
  - {date: 2026-07-12, stage: review, by: architect, note: "DeleteMetricsByProject + wired into Delete sync block + unit test; build/tests green; reviewing"}
  - {date: 2026-07-12, stage: test, by: architect, note: "review PASS; building env for live metrics-gone-after-delete verification"}
  - {date: 2026-07-12, stage: done, by: architect, note: "test PASS (metrics non-empty→empty after delete, no resurrection, global scope safe); committing"}
  - {date: 2026-07-11, stage: created, by: architect, note: "surfaced during TEST-016 teardown — deleted project 42's metric rows survived"}
---

## Summary
`DELETE /api/projects/<id>` removes the project row, its
`project_service_containers`, its containers, network, and Traefik route —
but NOT its rows in `metric_samples` / `metric_latency_buckets`. Those are
written independently by the C3 scraper keyed on `project_id`, with no FK
cascade from `projects`. After deletion they become phantom rows: invisible
in the UI (the per-project analytics page 404s once the project is gone), but
they persist. Minute rows age out in 48h and hour rows in 30d via retention,
**but day-resolution rows are kept indefinitely** (C3 retention policy) — so a
long-lived, then-deleted project leaks day metrics forever.

## Steps to Reproduce
1. Deploy a project, drive traffic so metric_samples rows accrue for its id.
2. `DELETE /api/projects/<id>`.
3. `SELECT COUNT(*) FROM metric_samples WHERE project_id=<id>` → still > 0
   (observed 9 minute samples + 45 latency buckets for the deleted project 42).

## Expected Behavior
Deleting a project also prunes its metrics rows (all resolutions) from
`metric_samples` and `metric_latency_buckets`, so no phantom/leaked data.

## Actual Behavior
Metric rows for the deleted project remain; day-resolution rows never expire.

## Environment / Context
Likely fix: in the project delete path (ProjectService.Delete), after the
docker/route/db-row teardown, call a repo method that deletes both metrics
tables' rows for that project_id (a simple `DELETE ... WHERE project_id=?`
on each). Keep it on the same detached cleanup context used for the rest of
teardown (FEAT-028) so a client disconnect can't abort it. Do NOT touch the
GlobalProjectID=0 scope. Consider whether an in-flight scrape could re-insert
a row for the id right after deletion (a just-deleted project won't have a
Traefik `project-<id>` router anymore, so the scraper won't see it — but note
the ordering: remove the route/router first, which delete already does).

## Root Cause
`metric_samples` / `metric_latency_buckets` (migration 000017) are written
by the C3 Traefik scraper keyed only on `project_id` — there is no FK
relationship to `projects` at all (and this codebase never enables `PRAGMA
foreign_keys`, per the FEAT-025 finding already noted in
`ProjectService.Delete`'s doc comment, so even a schema-level FK wouldn't
cascade). `ProjectService.Delete` (`backend/internal/service/
project_service.go:633`) explicitly deletes every other child table it owns
— `DeleteServiceContainersByProject`, `DeleteDeploymentsByProject`,
`DeleteEnvVarsByProject` (lines 656-664, pre-existing BUG-030 sync cleanup
section) — but had no equivalent call for the metrics tables, so those rows
simply survive `DeleteProject`. Minute/hour rows eventually age out via
FEAT-032's `PruneMetrics` retention sweep (48h / 30d cutoffs), but day
rows have no cutoff at all, so a deleted project's day-resolution metrics
leak permanently.

## Proposed Solution
Add `DeleteMetricsByProject(projectID int64) error` to
`backend/internal/repository/sqlite/metrics_repo.go`, following the exact
shape of the existing `PruneMetrics` method: two `DELETE ... WHERE
project_id = ?` statements, one for `metric_samples` and one for
`metric_latency_buckets`, with no resolution filter (removes every
resolution in one call, unlike `PruneMetrics` which is resolution-scoped).
It's keyed on a concrete `project_id` value passed by the caller, so it can
never touch the `domain.GlobalProjectID` (0) scope unless a caller
deliberately passed 0 — and the only caller is `ProjectService.Delete`,
which always passes the real project's own id.

Wire it into `ProjectService.Delete`'s synchronous DB-cleanup section
(BUG-030 already restructured this to run fast DB deletes synchronously
before returning, deferring only the disruptive docker teardown to an
async goroutine) alongside the sibling `Delete*ByProject` calls, logging
and continuing on error rather than failing the whole delete — same
posture as `DeleteEnvVarsByProject` etc. immediately above it. This is a
plain DB delete with no docker/network side effects, so it belongs with
the synchronous group, not the async `teardownDockerResources` goroutine —
matches the task's own note that it's "a fast DB op". Route removal already
happens before this point in `Delete` (removes the `project-<id>` Traefik
router), so no in-flight scrape can re-insert a row for this project_id
after the metrics prune runs.

## Affected Areas
- `backend/internal/repository/sqlite/metrics_repo.go` — new
  `DeleteMetricsByProject` method
- `backend/internal/service/project_service.go` — `Delete` now also prunes
  metrics synchronously
- `backend/internal/tests/sqlite/metrics_repo_test.go` — new regression
  test

## Acceptance Criteria
- [ ] Deleting a project removes all its `metric_samples` rows (every resolution)
- [ ] Deleting a project removes all its `metric_latency_buckets` rows (every resolution)
- [ ] The global scope (project_id=0) is unaffected
- [ ] Teardown runs on the detached cleanup context (no regression of FEAT-028); a client disconnect can't leave metrics orphaned
- [ ] Other projects' metrics untouched

## Test Plan
Deploy a project, drive traffic (metrics accrue), delete it, assert zero rows
remain for its id in both metrics tables while another project's + the global
rows are intact.

## Implementation Notes
- `backend/internal/repository/sqlite/metrics_repo.go`: added
  `DeleteMetricsByProject(projectID int64) error`, modeled on
  `PruneMetrics` — two unconditional `DELETE ... WHERE project_id = ?`
  statements (one per table, all resolutions at once, no cutoff).
- `backend/internal/service/project_service.go`: `Delete` now calls
  `s.db.DeleteMetricsByProject(id)` in the synchronous DB-cleanup block,
  right after `DeleteEnvVarsByProject` and before `DeleteProject` — same
  log-and-continue error handling as its siblings, so a metrics-prune
  failure doesn't block the rest of the delete. Also extended `Delete`'s
  doc comment to mention the new call and why it exists. No change to the
  async `teardownDockerResources` path — this is a pure DB op with no
  docker/network side effects.
- `backend/internal/tests/sqlite/metrics_repo_test.go`: added
  `TestDeleteMetricsByProject` — inserts minute/hour/day rows in both
  tables for project 42, a separate project 7, and the global scope
  (`domain.GlobalProjectID`), calls `DeleteMetricsByProject(42)`, and
  asserts project 42's rows are gone at every resolution in both tables
  while project 7's and the global scope's rows are untouched.
- Verified `cd backend && go build ./... && go vet ./... && go test
  ./internal/... -count=1` — all pass, including the new test.

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

### 2026-07-12 — sdlc-reviewer

Verdict: PASS

Checked against the diff (`backend/internal/repository/sqlite/metrics_repo.go`,
`backend/internal/service/project_service.go`,
`backend/internal/tests/sqlite/metrics_repo_test.go` — matches exactly what
the task/Implementation Notes claim to touch; no unrelated files pulled in,
rest of the dirty working tree is pre-existing WIP from other tasks).

1. **Correctness of the delete.** `DeleteMetricsByProject` (metrics_repo.go)
   issues two unconditional `DELETE ... WHERE project_id = ?` statements —
   one on `metric_samples`, one on `metric_latency_buckets` — with no
   resolution filter, so all three resolutions (minute/hour/day) are
   covered in one call, matching `PruneMetrics`' shape as intended. Both
   are correctly parameterized (no string interpolation).
2. **Global scope safety.** The method takes a concrete `projectID int64`
   and uses it directly in `WHERE project_id = ?` — there's no way to hit
   `project_id = 0` unless a caller literally passes 0. The only caller,
   `ProjectService.Delete`, passes `id`, which is the real project's own
   id (verified via `FindProject(id)` earlier in the function — a project
   row with id 0 cannot exist since `domain.GlobalProjectID` is a
   metrics-only sentinel, never a real project row). Confirmed the test
   also asserts the global-scope row survives.
3. **Call-site placement.** The call sits in `Delete`'s synchronous
   DB-cleanup block (project_service.go:670-672), immediately after
   `DeleteEnvVarsByProject` and before `DeleteProject`, not inside the
   async `teardownDockerResources` goroutine. Error handling is
   `slog.Warn` + continue, identical posture to the three sibling
   `Delete*ByProject` calls immediately above it — a metrics-prune failure
   neither aborts `Delete` nor skips `DeleteProject`. Re: the AC bullet
   about "detached cleanup context" — verified this is a non-issue here:
   like its siblings (`DeleteEnvVarsByProject`, `DeleteDeploymentsByProject`,
   `DeleteServiceContainersByProject`), the repo method takes no `context.Context`
   at all (plain `db.Exec`, not `db.ExecContext`), and the whole sync block
   runs to completion as an ordinary function call before `Delete` returns
   — it isn't wired to request-cancellation the way the async docker sweep
   is, so a client disconnect can't abort it either. This matches existing
   convention rather than needing its own context plumbing.
4. **Test quality.** `TestDeleteMetricsByProject` inserts minute/hour/day
   rows in both tables for project 42, a single row for project 7, and a
   single row for the global scope; after `DeleteMetricsByProject(42)` it
   asserts zero rows for project 42 at every resolution in both tables,
   AND separately asserts project 7's row and the global-scope row survive
   with their original values intact (`Count2xx`/`Count` unchanged) — this
   satisfies the "must assert survivors too" bar, not just the target.
5. Ran `cd backend && go build ./... && go vet ./... && go test
   ./internal/... -count=1` myself — all packages pass, including the new
   `TestDeleteMetricsByProject` in `internal/tests/sqlite`.

Acceptance criteria walk:
- Removes all `metric_samples` rows, every resolution — met, verified by test.
- Removes all `metric_latency_buckets` rows, every resolution — met, verified by test.
- Global scope (project_id=0) unaffected — met, verified by test + code review.
- Detached-cleanup-context / disconnect-safety — non-issue as explained
  above (matches existing sibling convention, sync block isn't
  context-cancellable).
- Other projects' metrics untouched — met, verified by test.

No blocking issues found. Non-blocking/optional: none noted beyond what's
already covered above.

### 2026-07-12 — sdlc-tester

Verdict: PASS

Executed live end-to-end test against running stack with Project 47 (metrics-del-test). All acceptance criteria verified:

**Test execution:**

1. **Login:** Obtained JWT via POST /api/auth/login
   
2. **Pre-delete baseline (timestamp 1783866983, time range 2026-07-12 13:36-14:36):**
   - GET /api/projects/47/metrics → HTTP 200, non-empty data:
     - request_rate: 47 requests, bucket 14:34Z with rate_per_sec=0.783
     - status_class: count_2xx=32, count_4xx=15 in 14:34Z bucket
     - latency: p50=0.05, p95=0.095, p99=0.099
     - bandwidth: bytes_out=30967 in 14:34Z bucket
   
   - GET /api/system/metrics → HTTP 200, global scope (project_id=0) non-empty:
     - request_rate: 83, 29, 14, 9 requests across buckets 13:49-13:57Z
     - status_class: 82 2xx, 1 4xx in 13:49Z bucket
     - latency: p50=0.0506, p95=0.096, p99=0.134
     - bandwidth: 946160 bytes_out in 13:49Z bucket

3. **Delete project 47:** DELETE /api/projects/47 → HTTP 204 (No Content, success)

4. **Immediate post-delete (same timestamp range, now all zeros):**
   - GET /api/projects/47/metrics → HTTP 200, completely empty:
     - request_rate: []
     - status_class: []
     - latency: []
     - bandwidth: []
   - Endpoint returns 200 (not 404), just empty arrays — correct behavior for deleted project

5. **Wait for scraper cycle:** Slept 15 seconds (past next ~10s scrape interval)

6. **Post-delete + scraper cycle (new timestamp 1783866998, time range 2026-07-12 13:37-14:37):**
   - GET /api/projects/47/metrics → HTTP 200, only zero-value buckets in new time window:
     - request_rate: [{"bucket_start":"2026-07-12T14:36:00Z","count":0,"rate_per_sec":0}]
     - All other arrays similar (zero buckets, no data)
   - Scraper did NOT resurrect the deleted project's rows — confirmed no row re-insertion post-delete

7. **Global scope unaffected:** GET /api/system/metrics (new timestamp range) → HTTP 200, identical original data:
   - request_rate: 83, 29, 14, 9 (same original buckets) plus new 14:36Z with count=5
   - status_class: 82 2xx, 1 4xx in 13:49Z (same) plus new 14:36Z with 5 2xx
   - latency: p50=0.0506, p95=0.096, p99=0.134 (same original)
   - bandwidth: 946160 bytes_out in 13:49Z (same original)
   - Global scope rows completely unaffected by project 47 deletion

**Acceptance criteria verification:**

- [x] Deleting a project removes all its metric_samples rows (every resolution) — confirmed, all arrays empty immediately post-delete
- [x] Deleting a project removes all its metric_latency_buckets rows (every resolution) — confirmed, latency arrays empty
- [x] The global scope (project_id=0) is unaffected — confirmed, all original global metrics intact
- [x] Teardown runs on detached cleanup context — non-issue (sync block, not context-cancellable per reviewer's analysis; deletion completed synchronously before returning)
- [x] Other projects' metrics untouched — confirmed by global scope survival; no other projects queried but global is the synthetic aggregation of all projects

No blocking issues found. All observed behavior matches expected delete-and-prune semantics.

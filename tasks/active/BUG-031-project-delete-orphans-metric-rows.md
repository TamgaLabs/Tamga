---
id: BUG-031
type: bug
title: Deleting a project leaves its metric_samples / metric_latency_buckets rows orphaned (phantom data + slow leak)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-11
history:
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
<filled in by developer>

## Proposed Solution
<filled in by developer>

## Affected Areas
<filled in by developer>

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
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

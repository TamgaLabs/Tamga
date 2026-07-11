---
id: TEST-015
type: test
title: C3 integration — metrics scrape → time-series → rollup → query, end to end
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C3 cluster integration test"}
---

**Part of:** C3-metrics
**Depends on:** FEAT-030, FEAT-031, FEAT-032

## Summary
The single integration test for cluster C3. Its impl sub-tasks are verified
statically in review; this verifies the full pipeline live on the rebuilt
stack before the cluster commits: real traffic through Traefik → scraper
stores samples → rollup → query API returns real panel data, per-project and
global.

## Scope
- Deploy a compose project (image-based; the C2 path) with a routed exposed
  service. Generate traffic to its domain through Traefik (a burst of curls
  producing a mix of status codes — hit the app + a missing path for 404s).
- Wait for at least one scrape interval (the scraper is ~60s) — Traefik's
  `traefik_router_requests_total{router="project-<id>"}` increments, the
  scraper stores minute samples.
- **Query API:** `GET /api/projects/{id}/metrics?from=&to=` returns non-empty
  series — request rate reflecting the burst, status-class breakdown
  including the 404s, latency percentiles, bandwidth. `GET /api/system/metrics`
  returns the global/core aggregate (the Tamga UI/API traffic the test
  itself generated). A project with no traffic → empty series, not an error.
- **Per-project attribution:** the project's metrics reflect ITS traffic,
  not another project's (deploy a 2nd project, drive different traffic,
  confirm each project's query returns its own numbers).
- **Rollup:** verify the rollup path works — either by seeding older samples
  and running the rollup, or (if feasible in the test window) confirming
  minute samples exist and the rollup query at a coarser resolution
  aggregates them. (Rollup timing may exceed the test window; the pure
  aggregation is unit-tested in FEAT-032 — verify what's feasible live and
  note the rest.)
- Counter-reset resilience: if practical, restart Traefik (or note it's
  covered by FEAT-031's unit test) — a reset must not produce a negative/
  huge spurious sample.

## Out of Scope
- The analytics UI (C4 — renders this API), the map (C5).

## Test Approach
<filled in by developer/tester>

## Affected Areas
<none — integration verification>

## Acceptance Criteria
- [ ] After driving traffic to a deployed project + waiting a scrape interval, `GET /api/projects/{id}/metrics` returns non-empty series reflecting that traffic (rate, the 404s in status breakdown, latency, bandwidth)
- [ ] `GET /api/system/metrics` returns the global/core aggregate
- [ ] Per-project attribution correct across two projects with different traffic
- [ ] A no-traffic project returns empty series, not an error
- [ ] Rollup verified as far as the test window allows (+ FEAT-032 unit tests for the aggregation); counter-reset handled (live or by unit test)
- [ ] No orphaned resources after the test

## Test Plan
<filled in — deploy a project, curl-burst its domain (mixed statuses), wait
a scrape tick, query the metrics API per-project + global, deploy a 2nd
project for attribution, exercise rollup/reset as feasible, clean up>

## Implementation Notes
<n/a — test task>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

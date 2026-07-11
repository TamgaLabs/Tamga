---
id: FEAT-032
type: feature
title: Metrics rollup/retention sweep + query API (panels)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C3 cluster"}
---

**Part of:** C3-metrics
**Depends on:** FEAT-030, FEAT-031

## Summary
The read side + retention for analytics: a background rollup that aggregates
minute samples → hourly → daily and prunes old fine-grained data (the
minute→hour→day policy), plus the HTTP query API that returns the four
panels' data per project and globally. C4's UI renders this.

## Requirements
- **Rollup + retention sweep** (a background goroutine or a tick folded into
  FEAT-031's — coordinate): periodically aggregate minute samples older than
  a threshold into hour buckets, hour into day, and prune the superseded
  finer-grained rows. Concrete policy (per the user's minute→hour→day
  decision): keep minute resolution for ~48h, hourly for ~30d, daily beyond
  — tune to sane defaults, document them. Aggregation = sum the increment
  counts / bucket counts / bytes into the coarser bucket (percentiles stay
  computable because we kept histogram bucket counts, not pre-computed
  percentiles). Idempotent + safe to run repeatedly.
- **Query API** (authenticated, mirror existing route style):
  - `GET /api/projects/{id}/metrics?from=&to=&resolution=` → the project's
    panels over the range; and a global variant
    `GET /api/system/metrics?from=&to=&resolution=` (scope 0/core+aggregate).
  - Response shape: time-series arrays for request rate (per-interval count
    → rate), status-class breakdown + error rate, latency p50/p95/p99
    (computed from the stored histogram bucket counts over the range), and
    bandwidth (bytes). Auto-pick resolution from the range if `resolution`
    omitted (e.g. <48h→minute, <30d→hour, else day). Propose the JSON shape
    C4 will consume — keep it panel-oriented.
  - Handle a project with no data (empty series, not an error).
- Percentile computation from histogram buckets: standard linear
  interpolation within the bucket containing the target quantile — a small
  pure helper, unit-tested (a known bucket distribution → known p50/p95/p99).
- Wire the rollup start + handler into cmd/api/main.go + router.go.
- Tests: pure percentile-from-buckets; rollup aggregation (minute rows →
  correct hour bucket sums, prune drops the right rows) — black-box where
  practical (seed FEAT-030's store, run rollup, assert); the query endpoint
  shaping.

## Out of Scope
- The scraper (FEAT-031), storage schema (FEAT-030), the analytics UI (C4),
  the map overlay (C5 — though it reuses this query API's per-project data).

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] Rollup aggregates minute→hour→day and prunes superseded rows per a documented retention policy; idempotent
- [ ] `GET /api/projects/{id}/metrics` and `GET /api/system/metrics` return the four panels (rate, status/error, latency p50/p95/p99, bandwidth) over [from,to] at a resolution (auto-picked if omitted); empty series for no-data, not an error
- [ ] Percentiles computed from stored histogram buckets (linear interpolation), unit-tested against a known distribution
- [ ] Rollup + handler wired into main.go/router.go
- [ ] `go build/vet/test` pass; pure percentile + rollup-aggregation logic unit-tested
- [ ] Code follows KISS/YAGNI

## Test Plan
Unit: percentile-from-buckets, rollup aggregation + prune. Live end-to-end
(traffic → scrape → store → rollup → query returns real panel data) is
TEST-015.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

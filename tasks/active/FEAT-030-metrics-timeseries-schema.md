---
id: FEAT-030
type: feature
title: Metrics time-series schema + storage repo
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C3 cluster"}
---

**Part of:** C3-metrics
**Depends on:** (none — first of the cluster)

## Summary
The storage foundation for traffic analytics: a SQLite time-series that the
scraper (FEAT-031) writes and the query API (FEAT-032) reads. Stores
per-service (= per-project) traffic metrics derived from Traefik's
Prometheus output (per TEST-010: `traefik_router_requests_total{code,method,
router,service}`, `traefik_router_request_duration_seconds_bucket`,
`traefik_router_requests_bytes_total`/`responses_bytes_total`, router/service
name = `project-<id>`). Retention is a minute→hour→day rollup (decided with
the user).

## Requirements
- Migration `000017` (verify next number): a metrics time-series table. Design
  for the four panels (request rate, status-class distribution + error rate,
  latency p50/p95/p99, bandwidth) at per-project + global granularity, with a
  resolution/rollup dimension. Concretely (adjust as the scraper's data shape
  demands — coordinate the seam with FEAT-031/032):
  - A samples table keyed by (bucket_start timestamp, resolution enum
    minute|hour|day, project_id — 0/NULL for the Tamga core/global scope,
    and enough dimension to reconstruct the panels). For counters store the
    per-interval INCREMENT (delta), not the raw cumulative — so rate/rollup
    are simple sums; document this choice. For status: keep per-status-class
    (2xx/3xx/4xx/5xx) counts. For latency: store the histogram bucket counts
    (le boundaries) per interval so percentiles are computable at query time.
    For bandwidth: request+response bytes increments.
  - Index on (project_id, resolution, bucket_start) for range queries.
- Domain types for a metrics sample / the panel rows.
- Repository (repository/sqlite): insert samples (batch), query a
  project's/global samples over a [from,to] range at a given resolution,
  delete/prune samples older than a cutoff per resolution (for FEAT-032's
  retention), and aggregate minute→hour / hour→day (or expose the primitives
  FEAT-032 needs — coordinate).
- Tests (black-box, backend/internal/tests/): migration applies fresh +
  copied DB; insert + range-query round-trip; prune-by-cutoff.

## Out of Scope
- The scraper (FEAT-031), rollup sweep + query API (FEAT-032), UI (C4).
- Deciding the exact Prometheus parse — that's FEAT-031; here just design the
  storage to hold what those metrics decompose into (increments + histogram
  buckets + status classes + bytes).

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] Migration 000017 adds the time-series table(s) with a resolution dimension + project scope (0/global); applies on fresh + copied DB
- [ ] Storage holds what the four panels need: per-interval request counts by status class, latency histogram buckets, request/response byte increments — per project + global
- [ ] Repo: batch insert, range query (project/global, resolution, [from,to]), prune-by-cutoff, and the min→hour/hour→day aggregation primitives FEAT-032 needs
- [ ] Indexed for range queries
- [ ] `go build/vet/test` pass; black-box tests cover migration + insert/query/prune
- [ ] Code follows KISS/YAGNI (don't build a general TSDB — just these panels)

## Test Plan
Black-box: apply migration, insert synthetic samples across resolutions +
projects, range-query them back, prune old ones. (Live scrape→store is
TEST-015.)

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

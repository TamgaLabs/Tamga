---
id: FEAT-031
type: feature
title: Traefik metrics scraper (Prometheus → time-series samples)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C3 cluster"}
---

**Part of:** C3-metrics
**Depends on:** FEAT-030

## Summary
A background scraper that periodically pulls Traefik's Prometheus metrics,
parses the per-router/service families, computes per-interval increments,
and writes minute-resolution samples into FEAT-030's store. The ingest half
of analytics; rollup + query is FEAT-032.

## Requirements
- A background goroutine (started at service construction, like FEAT-022's
  idle sweep) that scrapes Traefik's metrics endpoint every ~60s (the
  minute resolution). Endpoint: Traefik's internal metrics entrypoint (per
  C1/TEST-010 — reachable in-network at `http://traefik:8080/metrics`;
  make the URL config/env, default to that). Handle scrape failure
  gracefully (log, skip that tick — don't crash or double-count).
- Parse the Prometheus text exposition format. Use
  `github.com/prometheus/common/expfmt` (or a light hand parser if that dep
  is heavy — recommend + justify). Extract the families TEST-010 confirmed:
  `traefik_router_requests_total{code,method,router,service}`,
  `traefik_router_request_duration_seconds_bucket{le,router,service,...}`
  (+ _sum/_count), `traefik_router_requests_bytes_total` +
  `traefik_router_responses_bytes_total`. Strip the Traefik `@file`/`@docker`
  provider suffix from the router/service label to get the bare
  `project-<id>` → map to project_id. Aggregate the Tamga-core routers (the
  base tamga.yml UI/API routers) into the global/scope-0 bucket.
- **Counters are cumulative** — compute the INCREMENT since the last scrape
  per (project, status, bucket, metric) and store that as the minute
  sample (per FEAT-030's increment design). Hold the previous scrape's
  values in memory keyed appropriately. Handle counter RESETS (Traefik
  restart → counters drop to 0): if current < previous, treat current as
  the increment (standard Prometheus reset handling), don't store negative.
- Status classes: fold `code` into 2xx/3xx/4xx/5xx. Latency: store the
  per-le bucket increments. Bandwidth: request+response byte increments.
- Write a batch of samples per tick via FEAT-030's repo. First tick after
  startup has no previous → establish baseline, store nothing (or zero).
- Config: scrape URL + interval.
- Tests: the PARSE + increment/reset logic is pure → unit-test hard (feed a
  captured Traefik /metrics text sample — grab a real one, TEST-010's dev
  ran Traefik; or a representative fixture — assert the parsed
  per-project/status/bucket increments across two consecutive scrapes incl.
  a reset). The scrape HTTP + goroutine wiring is thin.

## Out of Scope
- Storage schema (FEAT-030), rollup/retention + query API (FEAT-032), UI (C4).

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] A scraper goroutine polls the Traefik metrics endpoint every ~60s, failure-tolerant
- [ ] Parses the confirmed traefik_router_* families; strips the provider suffix; maps `project-<id>` → project_id; core routers → global scope
- [ ] Computes per-interval increments from the cumulative counters, with counter-reset handling (no negatives)
- [ ] Folds status codes into classes; stores latency bucket increments + byte increments; writes minute samples via FEAT-030's repo
- [ ] First-tick baseline handled (no spurious huge increment)
- [ ] `go build/vet/test` pass; pure parse/increment/reset logic unit-tested with a real/representative Traefik /metrics fixture across ≥2 scrapes
- [ ] Code follows KISS/YAGNI

## Test Plan
Unit: two-scrape fixture (incl. a reset) → assert stored increments per
project/status/bucket. Live scrape→store is TEST-015.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

---
id: TEST-010
type: test
title: Reverse-proxy/routing/TLS audit + Traefik migration requirements
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 phase-1 audit"}
---

## Summary
SPRINT-004 replaces Caddy with Traefik (hard cut, dev). Before any of that
is planned, map exactly how routing + TLS work today on Caddy, and
research precisely what the Traefik equivalent requires — so the migration
tasks are written against reality, not guesses. Traffic analytics also
depend on Traefik's metrics shape, so pin that down here too.

## Scope
- **Current Caddy integration (read the code):** `Caddyfile`,
  `docker-compose.yml` caddy service (ports, volumes, admin :2019),
  `backend/internal/repository/caddy/client.go`
  (`AddRoute`/`RemoveRoute`/`LoadConfig`/`getRoutes` — how per-project
  domains become routes via the admin API), and every caller
  (`project_service.go` and anywhere else). Document the full
  route-lifecycle: when a project's domain route is added/removed, the
  upstream shape, and how TLS is currently obtained (self-signed localhost
  vs real certs).
- **Traefik equivalent — research + document (this is the migration spec):**
  - Dynamic per-project routing mechanism options: Traefik **file
    provider** (Tamga writes a watched dynamic-config file per
    project/route) vs **docker labels** (Tamga sets labels on the
    containers it creates and Traefik auto-discovers) vs the Traefik API.
    Recommend one for THIS codebase (Tamga controls container creation and
    already has a per-project network), with the tradeoffs — this choice
    shapes the whole migration.
  - TLS: how to reproduce today's behavior — self-signed/internal for
    localhost/dev, ACME (Let's Encrypt) for real project domains in prod.
    What Traefik config + what Tamga must provide per domain.
  - Prometheus metrics: the exact metric names/labels Traefik exposes
    per-router/service (request counts by status, request duration
    histograms, bytes) — this is the analytics data source, so confirm it
    can be attributed per-project (one router/service per project domain)
    and note the scrape endpoint/port.
  - The admin dashboard: how to enable it admin-only.
- **docker-compose.yml shape** the migration needs: Traefik service (image,
  ports 80/443, admin/dashboard, provider config, metrics), replacing the
  caddy service; what mounts/networks it needs to see project containers.

## Out of Scope
- Implementing the migration (that's phase 2).
- The compose-deploy model (TEST-011) and the map's docker enumeration
  (TEST-012).
- Filing defects found — file as BUG-XXX if any surface.

## Test Approach
<filled in by developer>

## Affected Areas
<filled in by developer — expected: findings only, no production changes>

## Acceptance Criteria
- [ ] Every Scope item exercised; each finding a concrete, checkable
      observation (file:line, config snippet, live evidence) not "looks fine"
- [ ] The current Caddy route lifecycle is documented end-to-end with
      file:line (AddRoute/RemoveRoute callers, upstream shape, TLS source)
- [ ] A recommended Traefik dynamic-routing mechanism (file provider vs
      labels vs API) with an explicit rationale for THIS codebase
- [ ] The exact Traefik Prometheus metric names/labels for per-service
      request rate / status / latency / bytes, confirmed attributable
      per-project, with the scrape endpoint — verified against Traefik docs
      or a throwaway Traefik run, not assumed
- [ ] TLS parity plan (dev self-signed + prod ACME) with the concrete
      Traefik config it needs
- [ ] Any defect found filed as its own BUG-XXX task
- [ ] Findings are detailed enough that phase-2 migration tasks can be
      written directly from them

## Test Plan
<filled in by developer — how the findings were verified, e.g. grepping the
caddy integration, and a throwaway `docker run traefik` + curl of its
/metrics and dashboard to confirm the metric shapes>

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

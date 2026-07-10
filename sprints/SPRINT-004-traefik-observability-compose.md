---
id: SPRINT-004
name: Traefik Migration, Compose Deploy, Observability & Infra Map
status: active
created: 2026-07-10
completed:
---

## Goal
Turn Tamga into a proper PaaS control plane: users deploy multi-service
architectures (docker-compose), see them as a live infrastructure map, and
watch their traffic/analytics — all built on a reverse-proxy migrated from
Caddy to Traefik so per-project routing and per-service metrics come from
one docker-native model. Settled with the user across five discussion
rounds; the concrete decisions below are the spec.

**Reverse proxy: Caddy → Traefik (hard replace, dev — no prod to preserve).**
- Traefik replaces Caddy entirely in docker-compose; the `repository/caddy`
  route management and the Caddyfile are removed.
- Dynamic per-project routing: each deployed project's exposed service gets
  a domain routed by Traefik (mechanism — file provider vs docker labels —
  decided by the phase-1 audit).
- TLS parity with today: self-signed for localhost/dev, automatic
  certificates (Traefik ACME/Let's Encrypt) for project domains in prod.
- Traefik's own admin dashboard enabled (admin-only) alongside our UI.
- Traefik Prometheus metrics enabled (per-router/service) — the data source
  for analytics.

**Unified compose-based deploy (a project IS a compose stack).**
- Support the common compose subset: `image`, `ports`, `environment`,
  `volumes`, `networks`, `depends_on`. (`build:`, `profiles:`, `secrets:`,
  healthcheck-conditions are out of scope.)
- The existing single-container git-build flow is modeled as a 1-service
  compose whose image is built from the repo — one deploy path, not two.
- Domain assignment: auto-detect the exposed/"web" service and give it the
  project domain by default; internal services (db/redis) stay internal.
  The user can bind/unbind domains to services from the UI (effectively
  editing the routing/compose through the UI).

**Traffic analytics (per-service, from Traefik Prometheus — no access logs).**
- Periodically scrape Traefik's per-service Prometheus metrics → store as a
  time-series in SQLite with a minute→hour→day rollup retention policy.
- Panels, per-service (= per-project) and as a global aggregate: request
  rate over time, status-class distribution + error rate, latency
  p50/p95/p99, bandwidth. (No per-path / top-paths / slowest-endpoint
  panels — that needs access logs, deliberately out of scope.)

**Infrastructure map (read-only observability + one edit action).**
- Nodes = docker containers; edges = shared docker networks. Classify nodes
  by image (redis/postgres/web/proxy/…) with icons; show live status.
- Auto-refreshing + clickable (a node → the existing container Inspect/Logs/
  Stats/Resources detail).
- Live traffic overlay tying map + analytics together: edge thickness/
  animation ~ request volume, node color ~ error rate/health, hover for
  mini-stats — using the same Traefik metrics as the analytics panels.
- The only edit action on the map is binding/unbinding a domain to a
  service; adding/removing/wiring services is not in scope (no full visual
  architecture editor).

**Placement.** Global "Infrastructure" + "Analytics" pages in the primary
sidebar (platform-wide), AND per-project map + analytics tabs in the
project secondary sidebar (a user's own architecture/traffic).

## Scope
- Backend: replace `repository/caddy` with a Traefik integration
  (dynamic routing + TLS); rework the project deploy pipeline into the
  unified compose model (parse the subset, deploy multi-service stacks on
  a per-project network, lifecycle, exposed-service→domain); a metrics
  scraper + time-series store + rollup + query API; docker enumeration +
  topology + stats for the map.
- Frontend: global Infrastructure map page + global Analytics page; per-
  project map + analytics tabs; the interactive map (auto-refresh, node
  classification/status, click-through, traffic overlay); the domain-
  binding edit action; compose-project create/deploy UI.
- Infra: docker-compose.yml (Traefik service, Prometheus scrape target);
  Dockerfiles as needed.

## Out of Scope
- Full docker-compose spec (`build:`, `profiles:`, `secrets:`,
  healthcheck conditions) — common subset only.
- Full interactive architecture editor on the map (drag-to-connect, add/
  remove services) — read-only + domain binding only.
- Per-path traffic analytics (top paths, slowest endpoints) and access-log
  ingestion — per-service Prometheus metrics only.
- Staged/reversible proxy migration — it's a hard replace in dev.
- Multi-user access control / per-tenant auth on the global pages —
  single-admin as today.

## Phases
1. Explore/audit (`type: test`) — three audits establishing ground truth
   before any implementation is planned: (a) the current Caddy routing +
   TLS integration and what the Traefik equivalent requires (provider,
   metrics shape, ACME, dashboard); (b) the current git-build deploy
   pipeline and how to fold it into a unified compose model; (c) the docker
   enumeration/stats surface for the infra map (nodes, network-derived
   edges, image classification, live status).
2. Findings-driven implementation — planned after phase 1 lands, decomposed
   into a cluster of smaller tasks (per the pipeline's decomposition rule —
   this is a large sprint; prefer several small dev+review units with a
   shared integration test over mega-tasks). Rough tracks: Traefik
   migration → unified compose deploy → metrics scraper + time-series →
   analytics UI → infra map + overlay → domain-binding edit.

## Tasks
- TEST-010 — Reverse-proxy/routing/TLS audit + Traefik migration requirements — done
- TEST-011 — Project deploy pipeline audit → unified compose-based model design — done
- TEST-012 — Docker enumeration/stats + topology audit for the infra map — pending
- BUG-028 — Caddy/project network isolation (routes can't reach projects) — pending (folds into Traefik migration)
- BUG-029 — All projects share one flat network, no isolation — pending (folds into compose-deploy)

## Release Notes
<filled in at sprint completion>

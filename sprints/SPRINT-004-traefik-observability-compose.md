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
2. Findings-driven implementation — planned 2026-07-10 from TEST-010/011/012.
   Decomposed into six clusters (per the decomposition rule: each cluster is
   several small dev+review tasks that HOLD in review, then ONE integration
   test verifies the cluster live before the whole cluster commits). Ordered
   by dependency:

   **C1 — Traefik migration** (foundation; keeps today's single-container
   deploy working through the swap): Traefik service in docker-compose
   (entrypoints 80/443, file provider watching a dir, metrics entrypoint,
   admin-only dashboard, default/self-signed cert + ACME config) · backend
   `repository/traefik` writing/removing per-project dynamic-config files
   (router+service `project-<id>`), replacing `repository/caddy`; wire into
   deploy/delete AND fix the Update-domain gap; remove Caddyfile +
   setupCaddyRoutes/reconcile · make Traefik share the project network so it
   can actually reach the exposed container (closes BUG-028). Integration
   test: deploy a project, hit its domain → app responds (not 502), Traefik
   /metrics shows the `project-<id>` router, localhost TLS works.

   **C2 — Unified compose deploy** (builds on C1 routing): schema migration
   (`projects.compose_yaml`/`exposed_service` + `project_service_containers`
   child table) · `compose-spec/compose-go/v2` parsing of the subset
   (image/ports/env/volumes/networks/depends_on) · docker-client plumbing
   for the gaps (ImagePull, depends_on ordering, multi-network) · per-project
   network (closes BUG-029) with services resolving each other by name ·
   fold git-build into a 1-service compose (one deploy path) · exposed-service
   heuristic + whole-stack lifecycle/status · compose-project create UI.
   Integration test: deploy a multi-service stack (e.g. web+redis), services
   reach each other, exposed service routed, another project can't reach it.

   **C3 — Metrics scraper + time-series store**: periodic scrape of Traefik
   `/metrics` → parse the `project-<id>` router/service families → SQLite
   time-series with minute→hour→day rollup + a query API. Integration test:
   generate traffic, confirm samples stored + rolled up + queryable per
   project and global.

   **C4 — Analytics UI**: global Analytics page + per-project Analytics tab;
   panels (request rate, status/error, latency p50/p95/p99, bandwidth) from
   C3's query API. Integration test: browser — panels render real data.

   **C5 — Infra map API + UI + overlay**: `repository/docker` network-list
   wrapper (edge source) + image classification + `GET /api/system/topology`
   & `/api/projects/{id}/topology` · global Infrastructure page + per-project
   Map tab (graph render, auto-refresh, node classification/status,
   click-through to container detail) · live traffic overlay joining topology
   ↔ C3 metrics (`project_id`→`project-<id>`). Integration test: browser —
   map shows real containers/edges, overlay reflects traffic.

   **C6 — Domain-binding edit**: the one map/UI edit action — bind/unbind a
   domain to a service, updating C1's Traefik routing. Integration test:
   rebind a domain via UI, the route actually moves.

   BUG-028 closes in C1, BUG-029 in C2. Each cluster's tasks get filed when
   its predecessor lands (or up-front if independent), following the
   "hold-in-review → cluster integration test → commit together" flow.

## Tasks
- TEST-010 — Reverse-proxy/routing/TLS audit + Traefik migration requirements — done
- TEST-011 — Project deploy pipeline audit → unified compose-based model design — done
- TEST-012 — Docker enumeration/stats + topology audit for the infra map — done
- BUG-028 — Caddy/project network isolation (routes can't reach projects) — done (closed by C1)
- BUG-029 — All projects share one flat network, no isolation — done (closed by C2)
- FEAT-025 — [C2] Schema + domain model for compose projects — done
- FEAT-026 — [C2] Docker-client compose primitives (ImagePull/depends_on/multi-net) — done
- FEAT-027 — [C2] Parse the compose subset (compose-go/v2) — done
- FEAT-028 — [C2] Unified compose deploy engine (per-project net, exposed-service routing) — done
- FEAT-029 — [C2] Compose-project create/deploy UI — done
- TEST-014 — [C2] Integration: multi-service deploy, isolation (closes BUG-029) — done
- BUG-030 — Compose project DELETE response not delivered to client (server-side succeeds) — pending (follow-up)
- FEAT-030 — [C3] Metrics time-series schema + storage repo — pending
- FEAT-031 — [C3] Traefik metrics scraper (Prometheus → samples) — pending
- FEAT-032 — [C3] Metrics rollup/retention + query API (panels) — pending
- TEST-015 — [C3] Integration: scrape→store→rollup→query end to end — pending
- FEAT-023 — [C1] Traefik service in docker-compose + static config — done
- FEAT-024 — [C1] Backend Traefik file-provider routing (replaces caddy) — done
- TEST-013 — [C1] Integration: Traefik migration end-to-end (closes BUG-028) — done

## Release Notes
<filled in at sprint completion>

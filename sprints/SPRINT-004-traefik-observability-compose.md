---
id: SPRINT-004
name: Traefik Migration, Compose Deploy, Observability & Infra Map
status: complete
created: 2026-07-10
completed: 2026-07-12
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
- BUG-030 — Compose project DELETE response not delivered to client (server-side succeeds) — done✓
- BUG-031 — Project delete orphans its metric_samples/latency rows (phantom data, day-rows leak) — done✓
- BUG-032 — Compose-declared networks create a redundant 2nd project network (surfaced by C5 map) — done✓
- BUG-033 — Rebind 409 check validates row-existence not running-state (stopped-container rebind accepted) — done✓
- FEAT-030 — [C3] Metrics time-series schema + storage repo — done✓ (additive-upsert rework)
- FEAT-031 — [C3] Traefik metrics scraper (Prometheus → samples) — done✓
- FEAT-032 — [C3] Metrics rollup/retention + query API (panels) — done✓
- TEST-015 — [C3] Integration: scrape→store→rollup→query end to end — done✓ (PASS post-fix; caught overwrite-upsert data-loss bug)
- FEAT-033 — [C4] Analytics data layer + reusable metric panel components — done✓
- FEAT-034 — [C4] Global Analytics page (system-wide panels + range/resolution controls) — done✓
- FEAT-035 — [C4] Per-project Analytics tab (project-scoped panels) — done✓
- TEST-016 — [C4] Integration: Analytics UI renders real metric data (global + per-project) — done✓ (PASS; headless render noted for human flow)
- FEAT-036 — [C5] Infra topology API (docker enumeration → nodes+edges+classification, system + per-project) — done✓
- FEAT-037 — [C5] Infra map UI: topology client + reusable SVG graph component (classified nodes, status, click-through) — done✓
- FEAT-038 — [C5] Global Infrastructure page + per-project Map tab (wire graph to topology, auto-refresh) — done✓
- FEAT-039 — [C5] Live traffic overlay (topology ↔ C3 metrics: edge volume, node error-rate, hover mini-stats) — done✓
- TEST-017 — [C5] Integration: map shows real containers/edges, overlay reflects traffic — done✓ (PASS; surfaced BUG-032 redundant-network C2 defect; map correct)
- FEAT-040 — [C6] Domain-binding backend (change exposed service + move Traefik route) — done✓
- FEAT-041 — [C6] Domain-binding UI (pick exposed service + set/clear domain) — done✓
- TEST-018 — [C6] Integration: rebind domain to a different service, route actually moves — done✓ (PASS; rebind proven live; 409-stopped-container gap → BUG-033)
- FEAT-023 — [C1] Traefik service in docker-compose + static config — done
- FEAT-024 — [C1] Backend Traefik file-provider routing (replaces caddy) — done
- TEST-013 — [C1] Integration: Traefik migration end-to-end (closes BUG-028) — done

## Release Notes

Tamga becomes a proper PaaS control plane: deploy multi-service stacks, watch
their traffic, and see them as a live infrastructure map — all on a
Traefik-based, docker-native routing/metrics model.

### Added
- **Traffic analytics.** A global Analytics page and a per-project Analytics
  tab with request-rate, status-class/error-rate, latency (p50/p95/p99), and
  bandwidth panels, plus time-range and resolution controls with live refresh.
  Backed by a Traefik metrics scraper → SQLite time-series (minute→hour→day
  rollup + retention) → query API, per project and as a global aggregate.
- **Infrastructure map.** A global Infrastructure page and a per-project Map
  tab rendering live containers as nodes and shared docker networks as edges,
  classified by image (redis/postgres/proxy/web/…), with status, auto-refresh,
  and click-through to a container's detail. A live traffic overlay ties the
  map to the analytics: node color reflects error rate, the ingress edge
  thickens with request volume, and hovering a node shows its mini-stats.
- **Multi-service deploys.** A project is now a docker-compose stack (common
  subset: image, ports, environment, volumes, networks, depends_on). Each
  project runs on its own isolated network with services resolving each other
  by name. New compose-project create UI.
- **Domain binding.** Bind or unbind the project's domain to any of its
  compose services from the project settings — the Traefik route moves live.

### Changed
- **Reverse proxy migrated from Caddy to Traefik** — per-project file-provider
  routing, Prometheus metrics as the analytics data source, self-signed (dev) /
  ACME (prod) TLS, and an admin dashboard. The legacy git-build single-container
  flow now deploys through the same unified compose path (one deploy path, not
  two).

### Fixed
- Cross-project network isolation — routes could previously reach the wrong
  project, and all projects shared one flat network; each project now runs on
  its own network (BUG-028, BUG-029).
- Logging in with the correct password now lands on the dashboard instead of
  bouncing back to the login page (BUG-034).
- Deleting a compose project now returns its HTTP response to the client
  instead of a dropped connection, and no longer leaves the project's metrics
  behind as orphaned rows (BUG-030, BUG-031).
- A compose project that declares its own `networks:` no longer gets a
  redundant second docker network (BUG-032).
- Rebinding the domain to a stopped service is now rejected with a clear error
  instead of silently routing to a down container that 502s (BUG-033).

Scope note: single-admin, dev-focused deployment — no per-tenant auth,
per-path analytics, or a full visual architecture editor (routing edits are
limited to domain binding).

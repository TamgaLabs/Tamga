---
id: TEST-010
type: test
title: Reverse-proxy/routing/TLS audit + Traefik migration requirements
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 phase-1 audit"}
  - {date: 2026-07-10, stage: development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-10, stage: review, by: architect, note: "audit complete, BUG-028 filed (folds into migration); moved to review"}
  - {date: 2026-07-10, stage: done, by: architect, note: "review PASS (metrics empirically re-verified by reviewer, BUG-028 confirmed live); test-stage done directly; task complete"}
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
Two-pass audit, no production code changes.

**Pass 1 — read the current Caddy integration.** Read `Caddyfile`,
`deploy/Caddyfile`, `docker-compose.yml`, `backend/internal/repository/caddy/client.go`
in full, then grepped every caller (`grep -rln "repository/caddy"
backend/`) to find every call site:
`backend/internal/service/project_service.go` (`deploy`/`Delete`/`Update`)
and `backend/cmd/api/main.go` (`setupCaddyRoutes`/`reconcileProjectRoutes`,
wiring `cfg.CaddyAdminURL`). Cross-checked `backend/internal/config/config.go`
for every `Caddy*`/`*Domain` env var and `.env`/`.env.example` for their
actual values, and `backend/internal/repository/docker/client.go`'s
`GetContainerPort` for how the upstream port is derived. Traced the live
network topology read-only against the already-running dev stack
(`docker network inspect`, `docker inspect`) rather than trusting the code
comments alone — this surfaced BUG-028 (see Affected Areas).

**Pass 2 — research the Traefik v3 equivalent.** Used WebSearch/WebFetch
against `doc.traefik.io` (the reference docs redirect to a newer URL
structure; followed the redirects) for the docker provider, file provider,
ACME/TLS, Prometheus metrics, and API/dashboard pages. Then ran a
throwaway `traefik:v3.4` container (`docker pull`/`docker run`, no
`docker-compose`, isolated `tmp-traefik-audit-net` network, cleaned up
fully afterward — confirmed via `docker ps -a`/`docker network ls` post-run)
alongside a `traefik/whoami` backend to empirically confirm: the file
provider's dynamic-config shape actually routes traffic, the exact
`/metrics` output (real Prometheus text, not docs-summarized), the
dashboard/API endpoints, and the default self-signed TLS certificate
behavior (`openssl s_client` + `openssl x509 -noout -subject -issuer`
against Traefik's `websecure` entrypoint with no `certResolver`
configured). The docker-label provider could **not** be empirically
verified in this sandbox — Traefik's vendored Docker client sent an
explicit `client version 1.24` handshake that this environment's Docker
daemon (API 1.55) rejected, and setting `DOCKER_API_VERSION` on the
Traefik container had no effect; this looks like a sandbox-Docker-socket
quirk unrelated to the label syntax itself, but it means docker-labels
claims below are **docs-only**, called out explicitly where they occur
(contrasted with the file-provider and TLS-default-cert claims, which are
**empirically confirmed** live in this session). ACME/Let's Encrypt
issuance itself was not exercised (no public domain/DNS available in this
sandbox) — the ACME static/dynamic config below is **docs-only**.

## Affected Areas
Findings only — no production code was changed for this audit (`git
status`/`git diff` show no changes under `Caddyfile`, `docker-compose.yml`,
`backend/`, or `deploy/`; the only new file is the newly-filed
`tasks/active/BUG-028-caddy-project-network-isolation.md`, plus this
task's own `## Test Approach`/`## Implementation Notes`/`## Affected
Areas`/`## Test Plan` sections).

- `Caddyfile`, `deploy/Caddyfile` — read only; `deploy/Caddyfile` found to
  be dead/unreferenced (noted in Implementation Notes §1, not a bug).
- `docker-compose.yml` — read only; the `caddy` service's network
  membership is the root cause half of **BUG-028**.
- `backend/internal/repository/caddy/client.go` — read only; every
  exported method (`AddRoute`/`RemoveRoute`/`LoadConfig`/`getRoutes`)
  traced end-to-end, all four are replaced by simple file writes/removes
  in the recommended Traefik design (§2/§6).
- `backend/internal/service/project_service.go` — read only; `deploy`
  (lines 159-169), `Delete` (lines 288-324), and `Update` (lines 404-428,
  the un-covered domain-change gap) are the three call sites that touch
  routing state and all three are where phase-2 migration tasks will need
  to land the file-provider write/remove calls.
- `backend/cmd/api/main.go` — read only; `setupCaddyRoutes`/
  `reconcileProjectRoutes` (lines 126-207) are the two functions that
  exist purely because of Caddy's atomic-config-replace model and have no
  file-provider equivalent needed (§2).
- `backend/internal/repository/docker/client.go` — read only;
  `GetContainerPort` (lines 152-161) stays as-is, its output is consumed
  identically by the new design; `NetworkConnect` (lines 313-320) already
  exists and is the mechanism phase-2 can reuse to fix BUG-028 for the new
  `traefik` service if compose's `external: true` route isn't chosen
  instead.
- **New:** `tasks/active/BUG-028-caddy-project-network-isolation.md` — a
  real, currently-live defect (caddy and project containers on disjoint
  Docker networks, so every per-project route is unreachable) found while
  tracing the upstream side of the route lifecycle. Filed per this task's
  process, not fixed here (Scope: audit only). Directly informs §6's
  compose shape (the new `traefik` service must join `tamga-net`,
  something `caddy` never does today).

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
A tester re-verifying this audit's claims can:

**Current Caddy integration:**
1. `grep -rln "repository/caddy" backend/` → confirms the exact caller
   set claimed in Implementation Notes §1 (`project_service.go`,
   `cmd/api/main.go`, plus the two `_test.go` files).
2. Read `project_service.go:159-169` (deploy), `:288-324` (Delete),
   `:404-428` (Update) directly — confirm `Update` has no `s.caddy.*`
   call anywhere in its body, confirming the documented domain-change gap.
3. With the dev stack up (`docker compose up`, or use the already-running
   instance), read-only-verify BUG-028:
   `docker network inspect tamga-net --format '{{range .Containers}}{{.Name}}
   {{end}}'` vs `docker inspect tamga-caddy-1 --format '{{range $k,$v :=
   .NetworkSettings.Networks}}{{$k}} {{end}}'` — confirm no overlap. (Do
   not modify the live stack; this is read-only.)

**Traefik research (reproduce the empirical claims):**
1. `docker network create tmp-traefik-verify-net`
2. `docker run -d --name tmp-whoami --network tmp-traefik-verify-net
   traefik/whoami:latest`
3. Write a static `traefik.yml` with `providers.file.directory`,
   `entryPoints.web`/`websecure`/`traefik`, `api.dashboard: true`,
   `api.insecure: true`, `metrics.prometheus.entryPoint: traefik`,
   `metrics.prometheus.addRoutersLabels: true`; write a dynamic
   `whoami.yml` with a `Host()` router + `loadBalancer` service pointing
   at `http://tmp-whoami:80`.
4. `docker run -d --name tmp-traefik --network tmp-traefik-verify-net -p
   <web>:80 -p <websecure>:443 -p <api>:8080 -v .../traefik.yml:/etc/traefik/traefik.yml:ro
   -v .../dynamic:/etc/traefik/dynamic:ro traefik:v3.4` (or the pinned
   phase-2 version).
5. `curl -H "Host: whoami.localhost" http://localhost:<web>/` → expect a
   real `whoami` response (confirms file provider routes traffic).
6. `curl http://localhost:<api>/metrics | grep ^traefik_` → compare
   against the metric table in Implementation Notes §4.
7. `curl http://localhost:<api>/dashboard/` → expect `200`;
   `curl http://localhost:<api>/api/http/routers` → expect the `whoami`
   router listed.
8. `openssl s_client -connect localhost:<websecure> -servername
   whoami.localhost | openssl x509 -noout -subject -issuer` (with a
   `tls: {}` router on `websecure` in the dynamic config) → expect `CN=TRAEFIK
   DEFAULT CERT` for both subject and issuer.
9. `docker rm -f tmp-traefik tmp-whoami && docker network rm
   tmp-traefik-verify-net` — confirm no leftovers via `docker ps -a`.

All of the above was run in this session; steps 1-9 completed with the
results recorded in Implementation Notes §2-§4 (file provider) and §3
(TLS default cert). Docker-labels claims (§2) and ACME issuance (§3) were
**not** independently re-run by a tester in this pass since they weren't
empirically verified by the developer either (see Test Approach) — a
tester should treat those specific claims as docs-sourced, not
re-verify them against a live run.

## Implementation Notes

### 1. Current Caddy integration — full route lifecycle

**Static config.** Two nearly-identical Caddyfiles exist:
`Caddyfile` (repo root, `admin :2019`, `email admin@example.com`,
`localhost { ... }`) and `deploy/Caddyfile` (templated:
`{$CADDY_EMAIL:...}`, `{$DOMAIN:localhost}`). Only the root `Caddyfile` is
actually mounted (`docker-compose.yml:9`, `./Caddyfile:/etc/caddy/Caddyfile`
into the `caddy` service) — `deploy/Caddyfile` is not referenced by
`docker-compose.yml` or any `Dockerfile` anywhere in the repo (confirmed by
grep); it's a vestigial copy left over from FEAT-001/FEAT-002 (see
`tasks/done/FEAT-001-docker-compose.md`), not load-bearing at runtime.
Harmless (no functional impact), noted for awareness only — not filed as a
bug.

**The root `Caddyfile` is not "the" runtime config, it's a bootstrap seed.**
`docker-compose.yml:9` mounts `./Caddyfile:/etc/caddy/Caddyfile` (read by
`caddy` on container start) and `docker-compose.yml:23` mounts the *same
host file* into the `backend` container at `/Caddyfile` (a different
container path but the same bind source — both are `./Caddyfile`). On
every backend startup, `main.go:126-168` (`setupCaddyRoutes`) builds an
entirely new Caddyfile in memory from `cfg` (`UIDomain`, `CaddyAutoSSL`,
`CaddyEmail`), **overwrites** the shared host file with it
(`os.WriteFile(caddyfilePath, ...)`, `caddyfilePath = "/Caddyfile"`,
`main.go:124,155`), then POSTs that content to Caddy's admin API
(`POST {CADDY_ADMIN_URL}/load?adapter=caddyfile`, `caddy/client.go:108-120`)
to hot-reload Caddy's *entire* running config. The comment
"Write Caddyfile to disk for reference/debugging" (`main.go:154`)
undersells what's happening — it's the same file Caddy itself was seeded
from, so every backend restart silently rewrites the checked-out root
`Caddyfile` on disk to whatever `cfg` currently says. `CADDY_ADMIN_URL`
resolves to `http://caddy:2019` in `.env`/`.env.example` (the `config.go:33`
fallback of `http://localhost:2019` would be wrong inside the backend
container and is only relevant if the backend somehow runs outside
compose).

**Because `/load` replaces Caddy's whole config**, every per-project route
previously added via the admin API is wiped by this reload. That's why
`reconcileProjectRoutes` (`main.go:173-207`) exists: right after
`setupCaddyRoutes` succeeds, it lists all projects
(`ps.List`), and for every project with `Status == Running`, a non-empty
`ContainerID`, and a non-empty `Domain`, re-derives
`upstream = "project-<id>:<port>"` (port via
`dc.GetContainerPort`, `docker/client.go:152-161` — first port key off
`ContainerInspect().NetworkSettings.Ports`, defaulting to `"80"` on error
or if the container exposes nothing) and calls `cc.AddRoute` again
(non-fatal, logged on failure). This reconcile step is a direct structural
consequence of Caddy's "the whole config is one blob, loaded atomically"
model — see §2, this doesn't exist at all with the recommended Traefik
file-provider design.

**Per-project route lifecycle (steady state, not the reconcile-after-restart
path):**
- **Add (deploy):** `ProjectService.deploy` (`project_service.go:91-183`),
  step "4. Register Caddy route" (lines 159-169). After the project
  container is created+started on network `tamga-net`
  (`project_service.go:140,149`), `s.docker.GetContainerPort` gets the
  exposed port (default `"80"`), `upstream := fmt.Sprintf("%s:%s",
  containerName, port)` where `containerName = "project-<id>"`
  (line 127), then `s.caddy.AddRoute(project.Domain, upstream)` — **non-fatal**:
  a failure is only `slog.Warn`'d, the deploy continues to
  `ProjectStatusRunning` regardless (lines 165-169). `AddRoute`
  (`caddy/client.go:36-61`) POSTs a single route object to
  `{adminURL}/config/apps/http/servers/srv0/routes/` (Caddy's structured
  JSON admin API, appending to the `srv0` HTTP server's route list — this
  server name comes from Caddy's own Caddyfile→JSON adapter and isn't
  configured anywhere in Tamga's code, it's just always `srv0` for a
  single site block) with `match: [{host: [domain]}]` and
  `handle: [{handler: "reverse_proxy", upstreams: [{dial: upstream}]}]`.
  No existence/dedupe check — deploying twice with the same domain would
  append two routes with the same host match (first one wins at request
  time; the second becomes dead weight only removable by
  `RemoveRoute`/index).
- **Remove (delete):** `ProjectService.Delete` (`project_service.go:288-324`),
  after stopping/removing the container: `if project.Domain != ""` →
  `s.caddy.RemoveRoute(project.Domain)` (non-fatal, `slog.Warn` only,
  lines 303-307). `RemoveRoute` (`caddy/client.go:63-89`) first `GET`s
  the **entire** route list (`getRoutes`, lines 91-104), linear-scans for
  the first route whose `match[].host` contains the domain, then issues a
  single `DELETE {adminURL}/.../routes/{i}` by **numeric list index** —
  a GET-then-index-DELETE pair that is not atomic against concurrent
  route mutations (a route added/removed by another goroutine between the
  GET and the DELETE would shift indices and delete the wrong route). Low
  practical risk today (route mutations are infrequent, one per
  deploy/delete), but worth flagging since it doesn't carry over cleanly
  to any design that batches or parallelizes route changes.
- **Reconcile (backend restart only):** as above, `main.go:173-207`,
  triggered once per backend process start, not per project action.
- **Update (domain change) — gap, not covered at all.**
  `ProjectService.Update` (`project_service.go:404-428`) lets a caller
  change `project.Domain` (and the frontend actually exposes this: `Edit
  Domain` field in
  `frontend/src/app/(main)/projects/[id]/settings/page.tsx:14,20,39-40`,
  wired to `updateProject`, `frontend/src/lib/api.ts:113`) and persists it
  to the DB — but **never calls `caddy.AddRoute`/`RemoveRoute` at all**.
  The old domain's route is never removed (dangling, still pointing at the
  same live upstream) and the new domain gets no route until the next
  backend restart happens to trigger `reconcileProjectRoutes` (which reads
  the now-updated `project.Domain` from the DB and adds it fresh). This is
  a real, currently-reproducible defect independent of the Traefik
  migration; not filed as a new BUG here since Caddy is being replaced
  wholesale in phase 2 (filing it would just be discarded when Caddy goes
  away) — but it's exactly the kind of drift the file-provider design in
  §2 structurally avoids (see the "bonus" note there), so phase-2
  migration tasks should make sure `Update` gets wired to rewrite the
  per-project dynamic-config file on domain change, closing this gap as
  part of the migration rather than carrying it forward.

**TLS.** Entirely config-driven, no code beyond what's already covered in
"static config" above:
- `cfg.CaddyAutoSSL` (env `CADDY_AUTO_SSL`, default `true`,
  `config.go:16,35`) — when `true`, `setupCaddyRoutes` emits `email
  {cfg.CaddyEmail}` in the global block and a plain
  `{cfg.UIDomain} { ... }` site block (`main.go:131-132,141`); Caddy's
  default behavior for a site block with no explicit `tls`/port directive
  is **automatic HTTPS**: real ACME (Let's Encrypt) certs for a real
  public domain, or Caddy's own internal/self-signed CA for
  `localhost`/private-use domains it recognizes as non-public (Caddy
  detects `UIDomain == "localhost"` itself and switches to internal
  issuance — nothing in Tamga's code special-cases this, it's Caddy's own
  built-in domain classification). `.env`'s default `UI_DOMAIN=localhost`
  means the out-of-the-box dev stack gets Caddy's self-signed cert.
  - When `false`, `setupCaddyRoutes` emits `auto_https off` globally and
    `{cfg.UIDomain}:80 { ... }` (plain HTTP, no TLS at all,
    `main.go:133-134,138-139`).
- **Per-project domains added via `AddRoute` get no explicit TLS
  handling of their own** — the route object built in
  `caddy/client.go:36-43` has no `tls`/certificate fields. Whether a
  project's domain gets auto-HTTPS depends entirely on whatever the
  top-level `auto_https`/`email` settings from the last `LoadConfig` call
  are (i.e. `cfg.CaddyAutoSSL`/`cfg.CaddyEmail`, global not
  per-project) — Caddy's automatic HTTPS applies per-hostname to *any*
  route with a matching `host`, so a project's domain does get the same
  auto-cert behavior as `UIDomain`, but there is no way today to give one
  project's domain different TLS treatment than another's (e.g. one
  needing a DNS challenge). Not a defect (nothing in Scope requires
  per-project TLS variance today), just a limitation worth carrying into
  the phase-2 design conversation.
- Certs/ACME state persist in the `caddy_data` volume
  (`docker-compose.yml:10,57`); Caddy's live JSON config snapshot
  persists in `caddy_config` (`docker-compose.yml:11,58`) — the compose
  network is deliberately **not** `internal: true`
  (`docker-compose.yml:60-65`, comment explains: Caddy needs outbound for
  ACME, backend needs it for git clone/pull).

**Route/network defect found — filed separately.** While tracing "how does
the upstream `project-<id>:<port>` actually get reached", read-only
`docker network inspect`/`docker inspect` against the already-running dev
stack showed the `caddy` container is *only* on
`tamga-network`/`tamga_tamga-network` (compose-defined,
`docker-compose.yml:12-13,60-65`) while every project container is
attached only to `tamga-net` (a separate network created ad hoc by
`EnsureNetwork` in `project_service.go:140` — a different name, never
joined by `caddy`, no `NetworkConnect` call anywhere attaches them to each
other despite that helper existing and being used elsewhere for a
different purpose, `docker/client.go:313-320`,
`agent_service.go:233`). Docker bridge networks are isolated from each
other by default, so every per-project `AddRoute` succeeds (the admin API
doesn't validate reachability) but can never actually proxy traffic — the
route is silently dead. Filed as **BUG-028**
(`tasks/active/BUG-028-caddy-project-network-isolation.md`) since it's a
real, currently-live defect independent of the Traefik migration, and
explicitly called out in §5/§6 below so the Traefik compose shape doesn't
repeat it.

### 2. Traefik dynamic-routing mechanism — recommendation: **file provider**

Traefik v3 offers three ways to feed it dynamic per-route config: the
**file provider** (watches a directory/file of YAML/TOML), the **docker
provider** (labels on containers, auto-discovered via the Docker socket),
or scripting the same JSON/API surface Caddy uses today via
`providers.docker`'s data model — Traefik has no separate "admin API for
imperative route mutation" the way Caddy does; the closest equivalent
would be writing directly to the file provider's watched files, which is
the file provider itself, not a fourth option.

**Recommendation: file provider**, one file per project
(`dynamic/project-<id>.yml`), written/removed by
`ProjectService.deploy`/`Delete` exactly where `AddRoute`/`RemoveRoute`
are called today. Empirically confirmed end-to-end in this session
(throwaway `traefik:v3.4` + `traefik/whoami`, static config
`providers.file.directory: /etc/traefik/dynamic` +
`providers.file.watch: true`): a router+service YAML file dropped into
the watched directory was picked up and proxying live traffic within the
test run, confirmed via `curl` returning the `whoami` container's actual
response and via `traefik_router_requests_total{router="whoami@file",...}`
appearing in `/metrics` (see §3).

**Why file provider over docker labels, for this codebase specifically:**
- **Decoupled from container lifecycle — closes today's `Update`/domain-
  change gap instead of carrying it forward.** Docker labels are set at
  `ContainerCreate` and are immutable for the life of that container — the
  Docker API has no "update labels on a running container" call. Since
  Tamga's project containers are already created via a single
  `CreateContainerOpts` call (`docker/client.go:58-82`, no `Labels` field
  today) with no other trigger to recreate them on a plain domain edit
  (`ProjectService.Update` just patches the DB row, `project_service.go:404-428`
  — it does not touch the container, unlike `Restart` which explicitly
  recreates it, `project_service.go:334-380`), a docker-labels design
  would need `Update` to start force-recreating the container purely to
  change a label — a bigger, riskier behavior change (container restart
  disrupts anything mid-request, resets ephemeral state) just to fix
  routing. A file-provider design instead lets `Update` just rewrite
  `dynamic/project-<id>.yml` with the new `Host()` rule — no container
  touched, and it structurally closes the exact gap documented in §1
  ("Update — gap, not covered at all") as a natural side effect of the
  migration rather than a separate fix.
- **Matches the existing pattern almost 1:1**, just cleaner. Tamga
  already writes a shared config file and pushes it into the proxy today
  (`main.go:126-168`) — the file provider is the same idea (write
  config, proxy picks it up) but per-project files instead of one
  giant file that gets fully replaced on every backend restart. This
  eliminates the `LoadConfig`-wipes-everything +
  `reconcileProjectRoutes` dance (§1) entirely: each project's file is
  independent, a backend restart touches none of them, so there is
  nothing to reconcile.
- **No new privileged mount.** Docker labels require mounting
  `/var/run/docker.sock` into the proxy container (Traefik's docker
  provider needs to call the Docker API to discover containers/labels) —
  a new, broader-blast-radius privilege for a component whose whole job
  is public-internet-facing request routing. Backend already has that
  socket mounted (`docker-compose.yml:21`) for container lifecycle
  management; giving the *reverse proxy* the same socket is a materially
  different risk profile the file provider avoids — the proxy only ever
  needs read access to a directory Tamga's backend writes.
- **Delete is a single `os.Remove`**, replacing `RemoveRoute`'s
  GET-full-list-then-DELETE-by-index dance (§1) with something
  index-race-free by construction.

**Trade-off acknowledged:** docker labels would mean zero explicit
route-management code at all (Traefik auto-discovers/auto-removes
purely from container lifecycle) and no separate on-disk state to keep in
sync with the DB — genuinely simpler in the case where domains never
change after container creation. Given `Update` already lets users edit a
project's domain today (frontend settings page), and the whole point of
this migration is not to re-introduce (or now permanently bake in) the
exact routing-drift bug class documented in §1, the decoupled model wins
for this codebase. **Docker-labels claims above are docs-only** (see Test
Approach) — the general label mechanism (`traefik.enable`,
`traefik.http.routers.<name>.rule`,
`traefik.http.services.<name>.loadbalancer.server.port`,
`traefik.docker.network`) is documented at
https://doc.traefik.io/traefik/reference/install-configuration/providers/docker/
but was not runnable in this sandbox (Docker API version negotiation
error from Traefik's vendored client, unrelated to Tamga's own Docker
client which works fine against the same daemon).

**Concrete config shape implied by this recommendation:**
- Static config (`traefik.yml` or CLI flags):
  ```yaml
  providers:
    file:
      directory: /etc/traefik/dynamic
      watch: true
  ```
- Per project, `backend` writes `dynamic/project-<id>.yml` (naming the
  router/service after the project ID, not the domain, so metrics labels
  in §4 map directly back to a project without a domain→ID lookup):
  ```yaml
  http:
    routers:
      project-<id>:
        rule: "Host(`<project.Domain>`)"
        service: project-<id>
        entryPoints: [web, websecure]
        tls:
          certResolver: leresolver   # prod only; omit for dev (see §3)
    services:
      project-<id>:
        loadBalancer:
          servers:
            - url: "http://project-<id>:<port>"
  ```
  This is a direct, mechanical translation of today's
  `AddRoute(domain, upstream)` call — same `domain`/`upstream` inputs,
  written to a file instead of POSTed to an admin API. `RemoveRoute`
  becomes `os.Remove(dynamic/project-<id>.yml)`.
- **Prerequisite, not optional:** the Traefik container must join
  whatever network project containers are actually on (`tamga-net` today)
  — this is precisely the mount/network BUG-028 documents Caddy getting
  wrong; the phase-2 compose shape (§6) must not repeat it.

### 3. TLS parity

**Dev (localhost) — self-signed, no config needed.** Empirically
confirmed: with a `websecure` (`:443`) entrypoint and a router with
`tls: {}` (no `certResolver` specified), Traefik automatically serves its
built-in fallback cert — `openssl s_client -connect
localhost:18443 -servername whoami.localhost | openssl x509 -noout
-subject -issuer` returned `subject=CN=TRAEFIK DEFAULT CERT,
issuer=CN=TRAEFIK DEFAULT CERT`, and `curl -sk` against it returned `200`.
This is Traefik's direct equivalent of Caddy's internal-CA self-signed
cert for `localhost` today (§1) — no `certResolver`, no extra static
config beyond declaring the `websecure` entrypoint and giving the
router's `tls:` a value (even empty). One difference from Caddy worth
flagging for phase 2: Traefik's default cert is a fixed, generic
"TRAEFIK DEFAULT CERT" (not per-hostname the way Caddy's internal CA
mints a cert per domain it serves) — fine for dev/localhost (browsers
already show an untrusted-cert warning either way), not a regression for
anything in Scope.

**Prod (real project domains) — ACME/Let's Encrypt.** Docs-only (no public
DNS/domain available to actually run an ACME handshake in this sandbox —
called out in Test Approach). Static config needs a certificate resolver
plus HTTP-01 challenge entrypoint:
```yaml
entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"
certificatesResolvers:
  leresolver:
    acme:
      email: ${CADDY_EMAIL}          # reuse today's CADDY_EMAIL env, rename in phase 2
      storage: /data/acme.json       # must persist across restarts — new volume, see §6
      httpChallenge:
        entryPoint: web
```
Per project, the only thing Tamga must supply is what it already supplies
today: the domain itself, via the router's `Host()` rule (§2's per-project
file), plus `tls.certResolver: leresolver` on that router — Traefik infers
the domain to request a cert for directly from the `Host()` rule, no
separate `domains:` list needed (confirmed via docs; this mirrors Caddy's
"any route with a host gets auto-HTTPS" behavior from §1, just made
explicit per-router instead of implicit per-Caddyfile-site). DNS pointing
the domain at the Tamga host remains the user's/deployer's
responsibility, unchanged from today. One documented operational
requirement to carry into phase 2 (not verified in this sandbox, no ACME
run performed): `acme.json` commonly needs `0600` permissions or Traefik
refuses to use it — the storage volume/file must be created with that
mode before first use.

### 4. Prometheus metrics — empirically confirmed (Traefik 3.4.5)

Static config to enable:
```yaml
entryPoints:
  traefik:
    address: ":8080"     # internal-only, see §5 — do not publish to the host
metrics:
  prometheus:
    entryPoint: traefik
    addRoutersLabels: true   # off by default; needed for per-router breakdown
```
Scrape endpoint: `GET http://<traefik>:8080/metrics` (the `traefik`
entrypoint doubles as API/dashboard/metrics — see §5; can be split to a
dedicated entrypoint if Tamga wants the scraper decoupled from the
dashboard, not necessary for Scope).

Metric names/labels actually observed in `/metrics` output after routing
one project's traffic through a file-provider router
(`router="whoami@file"`, `service="whoami@file"` — the `@file` suffix is
the provider tag Traefik appends to every resource name; **phase-2
analytics code must strip/split on `@`** to recover the plain
`project-<id>` name used in §2's per-project config):

| Metric | Type | Labels observed |
|---|---|---|
| `traefik_entrypoint_requests_total` | counter | `code, method, protocol, entrypoint` |
| `traefik_entrypoint_request_duration_seconds_{bucket,sum,count}` | histogram | `code, method, protocol, entrypoint` (+`le` on `_bucket`) |
| `traefik_entrypoint_requests_bytes_total` | counter | `code, method, protocol, entrypoint` |
| `traefik_entrypoint_responses_bytes_total` | counter | `code, method, protocol, entrypoint` |
| `traefik_router_requests_total` | counter | `code, method, protocol, router, service` |
| `traefik_router_request_duration_seconds_{bucket,sum,count}` | histogram | `code, method, protocol, router, service` (+`le`) |
| `traefik_router_requests_bytes_total` | counter | `code, method, protocol, router, service` |
| `traefik_router_responses_bytes_total` | counter | `code, method, protocol, router, service` |
| `traefik_service_requests_total` | counter | `code, method, protocol, service` |
| `traefik_service_request_duration_seconds_{bucket,sum,count}` | histogram | `code, method, protocol, service` (+`le`) |
| `traefik_service_requests_bytes_total` | counter | `code, method, protocol, service` |
| `traefik_service_responses_bytes_total` | counter | `code, method, protocol, service` |
| `traefik_open_connections` | gauge | `entrypoint, protocol` |
| `traefik_config_reloads_total` / `traefik_config_last_reload_success` | counter/gauge | — |

Default histogram buckets: `0.1, 0.3, 1.2, 5.0` seconds (docs-confirmed,
`buckets:` overridable in static config; not independently re-derived
from the raw `/metrics` bucket boundaries beyond confirming `le="0.1"`,
`"0.3"`, `"1.2"`, `"5"`, `"+Inf"` labels appeared, which matches).
`traefik_service_retries_total{service}` and
`traefik_service_server_up{service,url}` also documented (not exercised
live — no retries/multi-server load balancer configured in the throwaway
test) but are the same family and expected reliable.

**Per-project attribution: confirmed.** Since §2 recommends naming each
project's router **and** service after `project-<id>`, both
`traefik_router_requests_total{router="project-<id>@file",
service="project-<id>@file", code, method, ...}` and the equivalent
`traefik_service_*` series give a direct per-project breakdown of request
count by status, request-duration histogram, and request/response bytes —
exactly what Scope asks for as "the analytics data source". One nuance
worth carrying into the analytics design: **router/service-level metrics
only increment for requests that actually matched that router** — a
request to a domain with no configured route (e.g. DNS pointed at Tamga
but the project was deleted) only shows up in the `entrypoint`-level
series, never in any `router`/`service` series, since it never reached
one. Confirmed directly: a `curl` to `Host: nope.localhost` (no matching
router) produced a `404` counted in `traefik_entrypoint_requests_total`
but no corresponding `router`/`service` series appeared at all.

### 5. Admin dashboard

Enable: `api.dashboard: true` (defaults to `true` whenever `api` is
enabled at all) plus a `traefik` entrypoint to serve it on. Empirically
confirmed reachable at `GET http://<host>:8080/dashboard/` (`200`) and
`GET /api/overview`, `GET /api/http/routers` (both returned live JSON
reflecting the one file-provider route under test) once `api.insecure:
true` was set for the throwaway test.

**Admin-only, for the real migration:** do **not** use `api.insecure`
(docs explicitly warn it exposes full config, no auth, and recommend
against it in production) and do **not** publish the `traefik` entrypoint
to the host at all — same posture Tamga already takes with Caddy's admin
API today (`README.md:52`: "not published to the host, only reachable
from other containers on `tamga-network`"). Two ways to gate it, both
compatible with Tamga's existing JWT-based admin auth
(`handler.AuthMiddleware`, used for basically every other admin surface
in this codebase):
1. Keep the `traefik` entrypoint in-network-only (no `ports:` mapping in
   compose), and have the backend reverse-proxy an admin-only route
   (behind `AuthMiddleware`, same as every other admin endpoint) to
   `http://traefik:8080/dashboard/` and `/api/*` for anyone who needs to
   actually view it — mirrors how Caddy's `:2019` is handled today.
2. Traefik's own documented alternative — a router bound to the special
   `api@internal` service with a `basicAuth`/`forwardAuth` middleware —
   is the "native" way if Tamga would rather not proxy it through the
   backend. Option 1 keeps a single admin-auth mechanism (Tamga's
   existing JWT) instead of introducing a second one (basic auth
   credentials) just for this, so it's the better fit for this codebase,
   but both are viable; phase 2 should pick one.

### 6. `docker-compose.yml` shape

Replacing the `caddy` service. Concrete shape implied by §2-§5
(image/version per the WebSearch above: v3.7.7 is current stable as of
this audit — July 2026 — for security fixes; metrics/TLS/dashboard
behavior above was empirically verified against 3.4.5, which is the same
v3.x metrics/TLS/API surface per Traefik's own versioning guarantees
within a major version, but phase 2 should re-spot-check `/metrics`
output once pinned to whatever v3.7.x tag is actually shipped):

```yaml
services:
  traefik:
    image: traefik:v3.7          # pin exact patch at implementation time
    restart: unless-stopped
    command:
      - --providers.file.directory=/etc/traefik/dynamic
      - --providers.file.watch=true
      - --entryPoints.web.address=:80
      - --entryPoints.websecure.address=:443
      - --entryPoints.traefik.address=:8080   # dashboard+api+metrics, in-network only
      - --api.dashboard=true
      - --metrics.prometheus=true
      - --metrics.prometheus.entryPoint=traefik
      - --metrics.prometheus.addRoutersLabels=true
      - --certificatesresolvers.leresolver.acme.email=${CADDY_EMAIL}
      - --certificatesresolvers.leresolver.acme.storage=/data/acme.json
      - --certificatesresolvers.leresolver.acme.httpchallenge.entrypoint=web
    ports:
      - "80:80"
      - "443:443"
      # deliberately no 8080 published — dashboard/api/metrics stay
      # in-network only, see §5 (fixes the equivalent of README.md:52's
      # existing posture for Caddy's :2019, doesn't regress it)
    volumes:
      - ./deploy/traefik/dynamic:/etc/traefik/dynamic   # backend writes project-<id>.yml here
      - traefik_acme:/data                              # acme.json, equivalent of today's caddy_data
    networks:
      - tamga-network   # reach backend/frontend, same role caddy has today
      - tamga-net        # REQUIRED — reach project containers; this is exactly
                          # what BUG-028 shows caddy never getting today; must
                          # not be skipped in the migration
volumes:
  traefik_acme:
networks:
  tamga-network:
    driver: bridge   # unchanged
  # tamga-net is created ad hoc by EnsureNetwork today (project_service.go:140);
  # phase 2 should decide whether to pre-declare it here as `external: true`
  # so compose and the Go code agree on one network object, or keep relying
  # on EnsureNetwork's create-if-missing and just add `traefik` as a member
  # via NetworkConnect at startup (mirrors the existing pattern at
  # agent_service.go:233) — either resolves BUG-028 for the new proxy,
  # phase 2 picks the mechanism.
```
`backend`'s own `docker-compose.yml` entry needs its `./Caddyfile:/Caddyfile`
mount replaced with a bind into `./deploy/traefik/dynamic` (read-write, so
`deploy`/`Delete`/`Update` can write/remove `project-<id>.yml` directly) —
no more single-shared-file overwrite (§1), no more
`setupCaddyRoutes`/`reconcileProjectRoutes` (§2), no more admin-API HTTP
client (`caddy/client.go`'s `AddRoute`/`RemoveRoute`/`LoadConfig`/`getRoutes`
all get replaced by simple file writes/removes in the new
`repository/traefik` (or similarly-named) package).

## Review Notes

### 2026-07-10 — reviewer

**Verdict: PASS**

Independently re-verified essentially every load-bearing claim in this audit
against source and against the live/throwaway environment; all of it holds up.

**Current-Caddy citations (all confirmed to line-precision):**
- `main.go`: `setupCaddyRoutes` 126-168, `reconcileProjectRoutes` 173-207,
  `caddyfilePath = "/Caddyfile"` at 124 — exact match.
- `caddy/client.go`: `AddRoute`/`RemoveRoute`/`getRoutes`/`LoadConfig` all read
  as described (POST to `/config/apps/http/servers/srv0/routes/`, GET-then-
  index-DELETE for removal, no dedupe/reachability check).
- `project_service.go`: `deploy` (91-183, route registration at 159-169),
  `Delete` (288-324, `RemoveRoute` at 303-307), `Update` (404-428) all match
  claimed line ranges exactly. Read `Update` in full — confirmed it has
  **zero** `s.caddy.*` calls anywhere in its body. The documented
  domain-change gap is real: changing a project's domain via `PUT
  /projects/:id` leaves the old route dangling and the new domain unrouted
  until a backend restart triggers `reconcileProjectRoutes`.
- `docker-compose.yml`: line numbers for the `caddy` service's mounts/network
  (9, 12-13, 23, 60-65) all check out; `.env`/`.env.example` confirm
  `CADDY_ADMIN_URL=http://caddy:2019`, `CADDY_AUTO_SSL=true`,
  `UI_DOMAIN=localhost` as claimed. `config.go`'s `http://localhost:2019`
  fallback for `CADDY_ADMIN_URL` also confirmed (config.go:33).
- `deploy/Caddyfile` dead-file claim confirmed: no reference to it anywhere
  under `docker-compose.yml`/any `Dockerfile`.
- `frontend/.../settings/page.tsx` domain-edit field and
  `frontend/src/lib/api.ts:113`'s `updateProject` both confirmed as cited.

**BUG-028 (network isolation) — confirmed real, live, and correctly
diagnosed.** Ran read-only inspection against the actual running dev stack
(not a hypothetical):
- `docker inspect tamga-caddy-1` → only network membership is
  `tamga_tamga-network`.
- `docker network inspect tamga_tamga-network` → members are
  `tamga-backend-1 tamga-frontend-1 tamga-caddy-1`.
- `docker network inspect tamga-net` → a distinct bridge network
  (172.20.0.0/16 vs. `tamga_tamga-network`'s 172.19.0.0/16), currently no
  live members (no project is deployed in this session), but this is the
  network `project_service.go:140` (`EnsureNetwork(ctx, "tamga-net",
  false)`) attaches every project container to.
- The two networks are structurally disjoint bridge networks with no
  `NetworkConnect` call anywhere joining `caddy` to `tamga-net` (confirmed
  by grep) — Docker's default inter-bridge isolation means any route Caddy
  is told about for a project container is genuinely unreachable. The
  audit's diagnosis is correct, and folding it into the migration rather
  than fixing Caddy directly is the right call (explicitly annotated as such
  by the architect on the BUG-028 file itself). BUG-028 the task file is
  well-formed: clear repro, root cause, environment evidence, and
  acceptance criteria that will actually get exercised once Traefik lands.

**Traefik metrics claim — reproduced myself, matches exactly.** Spun up a
throwaway `traefik:v3.3` (image already cached locally) + `traefik/whoami`
on an isolated network, file-provider config mirroring the audit's, curled
a real request through it, then diffed `/metrics` against the audit's
table:
- `traefik_router_requests_total{code,method,protocol,router,service}`,
  `traefik_router_request_duration_seconds_{bucket,sum,count}` (buckets
  `0.1,0.3,1.2,5,+Inf` — exact match), `traefik_router_requests_bytes_total`,
  `traefik_router_responses_bytes_total` all present with `router="whoami@file",
  service="whoami@file"` — confirms the `@file` suffix and per-router/service
  attribution claim precisely.
- Same family confirmed at the `service` and `entrypoint` levels;
  `traefik_open_connections`, `traefik_config_reloads_total`,
  `traefik_config_last_reload_success` all present.
- Reproduced the audit's "unmatched host only shows up at entrypoint level"
  nuance directly: a request to `Host: nope.localhost` (no matching router)
  produced a `404` in `traefik_entrypoint_requests_total` with **no**
  corresponding `router`/`service` series — matches the audit's claim
  exactly.
- Dashboard (`GET /dashboard/` → 200) and `GET /api/http/routers` (listed
  the file-provider route) both confirmed reachable with `api.insecure: true`.
- Cleaned up fully afterward (`docker rm -f`, `docker network rm`); verified
  no leftover containers/networks via `docker ps -a`/`docker network ls`.

Every metric name in the audit's table is real, not guessed — I did not
find a single fabricated or misnamed metric.

**Routing mechanism recommendation (file provider):** rationale is sound.
Docker labels are genuinely immutable post-`ContainerCreate` (confirmed no
`Labels` field is even set today in `CreateContainerOpts`,
`docker/client.go:58-82`), so a labels-based design would need `Update` to
force a container recreation just to change a label — correctly identified
as a bigger behavior change than the file-provider's plain file rewrite.
The "no docker.sock in the reverse-proxy container" argument and the
"eliminates the LoadConfig-wipes-everything + reconcile dance" argument are
both accurate given what's confirmed about the current Caddy design in §1.
No hole found in this reasoning.

**Completeness:** every Acceptance Criterion has a corresponding, concrete
finding (file:line citations throughout, not "looks fine"); the
docker-compose target shape (§6) and TLS parity plan (§3) are concrete
enough to build phase-2 tasks from directly. Docs-only vs.
empirically-confirmed claims are clearly and consistently labeled throughout
(docker-labels and ACME issuance explicitly flagged docs-only; file
provider, TLS default cert, and metrics explicitly flagged empirically
confirmed) — this labeling is accurate, not just asserted; my own repro
lines up with what's marked empirical.

**No production code changed.** `git diff --stat` scoped to
`Caddyfile`/`docker-compose.yml`/`backend/`/`deploy/` shows only `Caddyfile`
(2 lines changed) — confirmed this is the already-running backend
container's `setupCaddyRoutes` rewriting the checked-out file on boot
(diff resolves `{$CADDY_EMAIL:...}`/`{$DOMAIN:...}` placeholders to their
concrete env values), exactly the ambient behavior the audit itself
documents in §1, not something this task's developer edited by hand.

**Non-blocking notes:**
- The audit correctly notes `traefik:v3.7` as current stable "as of this
  audit" but empirically tested against `3.4.5`; my own repro used the
  locally-cached `v3.3` image and got an identical metric/route/dashboard
  shape, which is a small additional data point supporting the audit's own
  caveat that this is stable within the v3.x line — not a discrepancy, just
  corroborating evidence.
- `tamga-net` currently has zero live members in the running dev stack (no
  project deployed this session), so the BUG-028 live-traffic failure mode
  itself (a proxied request actually timing out/failing) wasn't re-observed
  end-to-end by me — only the structural network-disjointness that
  guarantees it would fail. The audit is explicit that it also relied on
  the same structural evidence rather than an actual failed proxy request,
  so this isn't a gap in the audit, just worth flagging that full end-to-end
  reproduction (deploy a project, curl through Caddy, see it fail) remains
  for whoever picks up the folded-in fix during the Traefik migration.


## Test Notes
<filled in by tester>

## Test Notes

### 2026-07-10 — architect (test-stage verification, direct)

This is a research/documentation audit with no runtime product surface. Its
load-bearing claims were already empirically re-verified by the reviewer
(who independently spun up traefik:v3.3 + whoami and diffed /metrics — every
metric name matched — and confirmed BUG-028 live via docker inspect). A
separate builder/tester spin-up would only re-run Traefik a third time, so
the architect did the test-stage confirmation directly per the
token-optimization principle:

- BUG-028 network isolation: `docker inspect tamga-caddy-1` → only
  `tamga_tamga-network`; project containers join `tamga-net`
  (project_service.go:132-138) — disjoint, nothing connects them. Confirmed.
- Update-domain gap: `Update()` in project_service.go contains 0 `s.caddy.*`
  calls — a changed domain never updates the proxy. Confirmed.
- `caddy.AddRoute` called from deploy (project_service.go:165) with the
  `project-<id>:<port>` upstream. Confirmed.

The reviewer's independent empirical re-run of the Traefik metric families
stands as the verification for the analytics data source. Verdict: PASS.

---
id: FEAT-023
type: feature
title: Traefik service in docker-compose + static config (replaces Caddy)
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C1 cluster"}
  - {date: 2026-07-10, stage: development, by: architect, note: "assigned to sdlc-developer (C1 first task)"}
  - {date: 2026-07-10, stage: review, by: architect, note: "traefik compose+static config done (validated via throwaway traefik run); C1 cluster — HOLD in review after PASS pending FEAT-024"}
  - {date: 2026-07-10, stage: hold, by: architect, note: "review PASS; holding in tasks/review awaiting cluster integration test TEST-013"}
  - {date: 2026-07-10, stage: done, by: architect, note: "C1 integration test TEST-013 PASS; cluster complete"}
---

**Part of:** C1-traefik-migration
**Depends on:** (none — first of the cluster)

## Summary
Replace the Caddy service in `docker-compose.yml` with Traefik v3, statically
configured per TEST-010's migration spec (tasks/done/TEST-010-*). This is the
infra half of the proxy swap; the backend routing integration is FEAT-024.
Closing BUG-028 (proxy can't reach project containers) starts here: Traefik
must join BOTH the core network (to reach backend/frontend) AND the network
project containers live on (`tamga-net` today).

## Requirements
- `docker-compose.yml`: replace the `caddy` service with a `traefik` (v3)
  service — entrypoints `web` :80 and `websecure` :443 (host-published, as
  Caddy was), a **file provider** watching a mounted dynamic-config dir
  (e.g. `./traefik/dynamic:/etc/traefik/dynamic`), Prometheus **metrics**
  enabled on a dedicated non-published entrypoint/port (per TEST-010 — the
  scrape target), and the **admin dashboard/api** enabled but NOT publicly
  exposed (admin-only: reachable from the backend/internal network, not
  mapped to a host port, or behind the api entrypoint only).
- Networks: Traefik joins the core compose network (`tamga_tamga-network` /
  whatever backend+frontend use) AND `tamga-net` (the network project
  containers join today, per TEST-011) so routes can actually reach project
  upstreams — this is the infra-level BUG-028 fix. (C2 later moves to
  per-project networks; for C1 keep it working with today's flat network.)
- TLS: reproduce today's behavior — Traefik's default/self-signed cert for
  localhost/dev works out of the box; include the static ACME
  (Let's Encrypt) resolver config for real domains in prod (config present
  and correct; dev uses the default cert). Per TEST-010's TLS parity plan.
- Static Traefik config: a `traefik.yml` (static) mounted in, plus the
  watched dynamic dir. The backend (FEAT-024) writes per-project files into
  the dynamic dir — create the dir with a committed `.gitkeep` or a base
  dynamic file so the mount exists.
- Remove the `Caddyfile` and the caddy service. (The backend still
  references Caddy until FEAT-024 lands — that's expected; this task is the
  compose/config, FEAT-024 is the code. They land together in the C1
  integration test, so a transient inconsistency between them is fine.)
- Base routing for Tamga's own UI/API: the equivalent of the current
  Caddyfile's `/api/* → backend:8080`, everything else `→ frontend:3000`,
  on the `localhost` host — expressed as Traefik static/dynamic config (a
  base dynamic file, since these aren't per-project).

## Out of Scope
- Backend `repository/traefik` code + removing the caddy client (FEAT-024).
- Per-project networks (C2 / BUG-029).
- Actually verifying end-to-end reachability (the C1 integration test,
  TEST-013).

## Proposed Solution / Approach
Follow TEST-010's migration spec (`tasks/done/TEST-010-*`, Implementation
Notes §2-§6) directly rather than re-deriving the shape: it already pins
the exact static/dynamic config, metrics entrypoint, dashboard posture, and
TLS plan against an empirically-verified throwaway Traefik run.

- **Compose service.** Replace the `caddy` service with `traefik:v3.7`
  (current stable at time of writing, confirmed pullable/resolves to
  3.7.1). Keep the host-published `web`/`websecure` (80/443) ports Caddy
  already had; add a third `traefik` entrypoint (:8080, dashboard+api+
  Prometheus metrics combined per TEST-010 §5 — not published to the host,
  matching Caddy's `:2019` admin-only posture documented in `README.md:52`).
  Mount `./traefik/traefik.yml` (static, ro) and `./traefik/dynamic`
  (file-provider watch dir, ro from Traefik's side) plus a new
  `traefik_acme` volume for `/data/acme.json`, replacing `caddy_data`/
  `caddy_config`.
- **Backend's compose entry.** Its `./Caddyfile:/Caddyfile` mount is no
  longer valid once `Caddyfile` is removed — replace it with the *same*
  host directory Traefik watches (`./traefik/dynamic`), mounted read-write
  so FEAT-024's file writes land somewhere Traefik can see. This is a
  compose/mount concern, not backend Go code, so it belongs in this task
  even though nothing writes there yet.
- **Networks (BUG-028 infra fix).** Traefik joins both `tamga-network`
  (reach backend/frontend, same role Caddy has today) and `tamga-net` (the
  network `ProjectService.deploy`'s `EnsureNetwork("tamga-net", ...)`
  creates project containers on, `project_service.go:140`). Declared
  `tamga-net` in `docker-compose.yml`'s `networks:` block with an explicit
  `name: tamga-net` (not `external: true`) so a from-scratch `docker
  compose up` creates it directly with no chicken-and-egg ordering problem;
  `EnsureNetwork`'s exists-check (`docker/client.go:295-311`) then finds it
  already present at backend runtime and is a no-op, so compose and the
  Go code agree on the same network object without any code change.
- **Static config (`traefik/traefik.yml`).** Entrypoints `web`/`websecure`/
  `traefik`; `providers.file` watching `/etc/traefik/dynamic` with
  `watch: true`; `metrics.prometheus` on the `traefik` entrypoint with
  `addRoutersLabels: true` (needed for per-router breakdown, off by
  default); `api.dashboard: true` plus `api.insecure: true` (auto-binds
  the dashboard/api router to the `traefik` entrypoint with no extra
  router config — the same setup TEST-010's own audit used to confirm
  reachability; safe here specifically because that entrypoint is never
  published to the host, so "insecure" only means "no separate auth", not
  "reachable from outside the network" — a JWT-gated proxy through the
  backend, TEST-010 §5 option 1, can replace this later without touching
  this file); a `leresolver` ACME certificatesResolver (email, `/data/
  acme.json` storage, HTTP-01 challenge on `web`) for prod.
- **Base dynamic config (`traefik/dynamic/tamga.yml`).** The Caddyfile
  equivalent: `Host(\`localhost\`) && PathPrefix(\`/api\`)` → `backend:8080`,
  plain `Host(\`localhost\`)` → `frontend:3000`. One thing only surfaced by
  actually validating this against a live Traefik container (see
  Implementation Notes): a router with `tls: {}` does **not** attach to a
  non-TLS entrypoint even when it's listed in that router's `entryPoints`
  — Traefik silently 404s plain HTTP for such a router despite its API
  claiming `"using": ["web","websecure"]`. TEST-010 §2's single
  dual-entrypoint router-with-tls example wasn't itself run against both
  entrypoints simultaneously (their empirical pass used a plain TLS-only
  openssl check), so this is new information from this task's own
  validation, not a deviation from something TEST-010 confirmed. Fixed by
  splitting each route into a plain-HTTP router (`web` entrypoint, no
  `tls`) and a separate TLS router (`websecure` entrypoint, `tls: {}`) —
  the documented Traefik pattern — giving side-by-side `http://localhost`
  and `https://localhost` access with no forced redirect, matching Caddy's
  current dev behavior (`README.md:50`: plain `http://localhost` works
  today).
- **TLS.** No dev config needed — Traefik's built-in `TRAEFIK DEFAULT CERT`
  self-signed cert serves automatically on `websecure` for any router with
  `tls: {}` and no `certResolver`, empirically confirmed in this task's own
  validation run (matches TEST-010 §3's finding). The ACME resolver is
  present and correctly shaped for prod per TEST-010 §3, not wired to any
  route yet (no project domains exist until FEAT-024).

## Affected Areas
- `docker-compose.yml` — `caddy` service replaced by `traefik` (image,
  ports, volumes, networks); `backend`'s Caddyfile mount replaced by a
  read-write bind of `./traefik/dynamic`; `caddy_data`/`caddy_config`
  volumes replaced by `traefik_acme`; `networks:` gains a declared
  `tamga-net` entry.
- `traefik/traefik.yml` — new, static config.
- `traefik/dynamic/tamga.yml` — new, base dynamic routing for Tamga's own
  UI/API (four routers: `tamga-api`/`tamga-api-secure`/`tamga-ui`/
  `tamga-ui-secure`, two services `tamga-backend`/`tamga-frontend`).
- `Caddyfile` — removed.
- Not touched (deliberately, per Out of Scope): `backend/internal/
  repository/caddy/`, `backend/cmd/api/main.go`'s `setupCaddyRoutes`/
  `reconcileProjectRoutes`, `backend/internal/config/config.go`'s `Caddy*`
  env vars, `README.md`'s Caddy references — all still reference Caddy
  until FEAT-024 lands (expected transient inconsistency, called out in
  the task description). `.env`/`.env.example`'s `CADDY_*` vars are also
  untouched for the same reason (`CADDY_EMAIL` is read directly as the
  ACME email placeholder's conceptual source, but not env-substituted into
  `traefik.yml` — see Implementation Notes on why).

## Acceptance Criteria / Definition of Done
- [ ] `docker-compose.yml` has a traefik v3 service (web/websecure entrypoints, file provider on a mounted dynamic dir, metrics entrypoint, admin-only dashboard) and no caddy service; `Caddyfile` removed
- [ ] Traefik joins both the core network and `tamga-net`
- [ ] `traefik.yml` static config + a base dynamic config (Tamga UI/API routing: /api→backend, else→frontend on localhost) are present and valid
- [ ] ACME resolver configured for prod (present + correct), dev uses default self-signed cert
- [ ] `docker compose config` validates; the metrics entrypoint + dashboard are reachable from inside the compose network (not host-published for the dashboard)
- [ ] Code follows KISS/YAGNI

## Test Plan
Verified in the C1 integration test (TEST-013) end-to-end. Standalone here:
`docker compose config` validates; a `docker compose up` of the traefik
service starts cleanly; `curl` the base UI/API routing + the /metrics
endpoint from inside the network.

## Implementation Notes
Implemented directly (complexity: standard), no `opencode` delegation.

**Files changed:**
- `docker-compose.yml` — `caddy` service → `traefik` service (image
  `traefik:v3.7`; ports `80:80`/`443:443` published, no port published for
  the internal `traefik` entrypoint; volumes: `./traefik/traefik.yml:/etc/
  traefik/traefik.yml:ro`, `./traefik/dynamic:/etc/traefik/dynamic:ro`,
  `traefik_acme:/data`; networks: `tamga-network` + `tamga-net`).
  `backend`'s `./Caddyfile:/Caddyfile` mount replaced with `./traefik/
  dynamic:/etc/traefik/dynamic` (read-write). `caddy_data`/`caddy_config`
  volumes replaced by `traefik_acme`. `networks:` block gains a `tamga-net`
  entry (`name: tamga-net`, `driver: bridge`, not `external: true`).
- `traefik/traefik.yml` — new static config: `web`/`websecure`/`traefik`
  entrypoints; `providers.file` on `/etc/traefik/dynamic` with
  `watch: true`; `api.dashboard: true` + `api.insecure: true` (binds to
  the non-published `traefik` entrypoint); `metrics.prometheus` on the
  `traefik` entrypoint with `addRoutersLabels: true`; `certificatesResolvers.
  leresolver.acme` (email, `/data/acme.json` storage, HTTP-01 on `web`).
- `traefik/dynamic/tamga.yml` — new base dynamic config: four routers
  (`tamga-api`/`tamga-ui` on `web`, `tamga-api-secure`/`tamga-ui-secure` on
  `websecure` with `tls: {}`) and two services (`tamga-backend` →
  `http://backend:8080`, `tamga-frontend` → `http://frontend:3000`),
  `/api*` at priority 10 over the catch-all at priority 1.
- `Caddyfile` — deleted.

**Deviation from TEST-010's literal example, with cause.** TEST-010 §2's
sample per-project router lists both `web` and `websecure` in
`entryPoints:` on a single router with `tls: {}`. Validating this task's
own config against a live `traefik:v3.7` container showed that router
never actually attaches to the non-TLS entrypoint (plain `http://` 404s
with no router-level metric at all, even though the router API reports
`"using": ["web","websecure"]` and zero errors) — only the TLS entrypoint
works. This wasn't caught by TEST-010's own empirical pass because their
openssl/TLS check only ever hit `websecure`. Fixed by splitting each route
into a plain-HTTP router (no `tls`) and a separate `-secure` TLS router,
the documented Traefik pattern; both now verified working side by side.
This only affects the base `tamga.yml` written in this task — noting it
here so FEAT-024's per-project route files use the same two-router shape
instead of copying TEST-010's single-router example verbatim.

**Path chosen for the dynamic dir:** `./traefik/dynamic` (per this task's
own Requirements wording), not TEST-010 §6's `./deploy/traefik/dynamic` —
both are valid per the audit's own phrasing ("mounts/networks it needs");
picked the shorter path since nothing else in `deploy/` is a runtime data
directory (that folder is exclusively Dockerfiles).

**ACME email:** hardcoded `admin@example.com` in `traefik.yml` (matching
the current root `Caddyfile`'s own hardcoded bootstrap default) rather than
env-substituted from `CADDY_EMAIL`, since Traefik's static config file has
no native `${VAR}` expansion at the YAML level (only compose's `command:`
list gets docker-compose's own `${VAR}` substitution, and Requirements ask
for the setting to live in `traefik.yml`, not CLI flags). Flagged in a
comment in the file as needing a real address before ACME is used against
a public domain — this mirrors how the *checked-in* root `Caddyfile`
already behaves today (also a hardcoded placeholder, not env-driven; only
the backend-generated `/Caddyfile` FEAT-024 will eventually replace uses
the real env value).

**Validation performed (standalone, live stack untouched):**
- `docker compose config` — validates cleanly; confirmed `traefik` service
  present with both networks, no `caddy` service remains.
- Pulled `traefik:v3.7` fresh (resolves to 3.7.1, matches TEST-010's
  recommended pin) and ran a throwaway container (`tmp-traefik-feat023`,
  isolated `tmp-traefik-feat023-net` network + a throwaway `traefik_acme`
  volume) mounting this task's actual `traefik/traefik.yml` and
  `traefik/dynamic/tamga.yml`:
  - Config parses with zero errors/warnings (`GET /api/overview`:
    `errors: 0` across routers/services/middlewares).
  - File-provider routers loaded and `enabled`: `tamga-api@file`,
    `tamga-api-secure@file`, `tamga-ui@file`, `tamga-ui-secure@file`
    (`GET /api/http/routers`).
  - `curl -H "Host: localhost" http://.../` and `.../api/foo` both
    returned `502` (Bad Gateway — the router matched and tried to proxy;
    `backend`/`frontend` hostnames don't exist in this isolated network,
    which is the expected failure mode here, not a config error) — real
    end-to-end reachability against the actual backend/frontend is
    TEST-013's job, not this task's.
  - Path-based routing confirmed via per-router Prometheus counters:
    a request to `/api/health` incremented `tamga-api@file`'s counter, a
    request to `/dashboard` incremented `tamga-ui@file`'s — confirms
    `/api*` routes to the backend service and everything else to frontend.
  - TLS: `openssl`-equivalent via `curl -vk` showed
    `subject=CN=TRAEFIK DEFAULT CERT, issuer=CN=TRAEFIK DEFAULT CERT` —
    matches TEST-010 §3's finding exactly, no config needed for dev.
  - ACME: with the `traefik_acme` volume mounted, Traefik auto-created
    `/data/acme.json` at `0600` on boot with no error (the
    "no such file" error seen in an earlier, no-volume-mounted throwaway
    run was resolved once a real persistent volume was mounted — nothing
    to reproduce in the compose service, `traefik_acme` is a named volume
    there from the start).
  - Dashboard: `GET http://.../dashboard/` → `200` from inside the
    network (never published to the host in `docker-compose.yml`).
  - Metrics: `GET http://.../metrics` returned real Prometheus text with
    `traefik_router_requests_total{router="tamga-api@file",...}` etc.,
    matching TEST-010 §4's metric family exactly.
  - Cleaned up fully afterward: `docker rm -f`/`docker network rm`/
    `docker volume rm`; confirmed no leftovers via `docker ps -a`/
    `docker network ls`/`docker volume ls` filtered on the throwaway name.
  - Live `tamga-*` stack confirmed untouched throughout (`docker ps`
    showed `tamga-caddy-1`/`tamga-backend-1`/`tamga-frontend-1` still
    running unmodified before and after this task's validation work).

## Review Notes

### 2026-07-10 — reviewer

**Verdict: PASS**

Verified `docker-compose.yml`'s diff, `traefik/traefik.yml`, and
`traefik/dynamic/tamga.yml` against both this task's own Requirements and
TEST-010's migration spec (`tasks/done/TEST-010-*`, §2-§6). `docker compose
config` was re-run (read-only, live `tamga-*` stack confirmed untouched
before/after via `docker ps`) and validates cleanly, resolving exactly the
shape claimed.

**Traefik service — matches spec:**
- `traefik:v3.7` (image confirmed present locally, `docker images` shows
  `traefik:v3.7`/`1cb3845d7a05`, consistent with the dev's claim of having
  pulled and run it), `web`:80 / `websecure`:443 host-published identically
  to Caddy's old ports, `traefik`:8080 entrypoint present but **not**
  published to host — confirmed via `docker compose config` output (only
  two `ports:` entries render for the traefik service).
- `providers.file.directory: /etc/traefik/dynamic`, `watch: true` —
  matches TEST-010 §2 exactly.
- `metrics.prometheus.entryPoint: traefik`, `addRoutersLabels: true` —
  matches TEST-010 §4 exactly.
- ACME resolver (`leresolver`, HTTP-01 on `web`, `/data/acme.json`) present
  and correctly shaped per TEST-010 §3, not wired to any route yet (correct
  — no project domains exist pre-FEAT-024).

**Shared dynamic-dir mount (the FEAT-024 load-bearing bit) — confirmed
correct on both sides.** `docker compose config` output:
- `traefik` service: `./traefik/dynamic` → `/etc/traefik/dynamic`,
  `read_only: true` (correct — Traefik only ever reads this dir).
- `backend` service: same host path `./traefik/dynamic` →
  `/etc/traefik/dynamic`, **no** `read_only` flag (i.e. read-write) —
  correct, this is exactly what FEAT-024 needs to write per-project route
  files into. Old `./Caddyfile:/Caddyfile` mount is gone; grepped compose
  + repo root for dangling `Caddyfile`/`caddy_data`/`caddy_config`/
  `/etc/caddy` references — none found outside historical comments (which
  correctly say "Caddy needed X, Traefik does Y").

**Networks / BUG-028 fix — confirmed correct and code-consistent.**
`traefik` joins both `tamga-network` and `tamga-net`; `tamga-net` is
declared in `docker-compose.yml`'s `networks:` block with an explicit
`name: tamga-net`, `driver: bridge`, not `external: true`. Cross-checked
against `backend/internal/repository/docker/client.go:295-311`
(`EnsureNetwork`, exists-check then create-if-missing, non-internal bridge)
and its two call sites in `project_service.go:140,361`
(`EnsureNetwork(ctx, "tamga-net", false)`) — the compose-declared network
and the backend's runtime network object agree exactly (same name, same
driver, same internal=false), so `docker compose up` from scratch creates
it once and `EnsureNetwork` is a no-op thereafter, no chicken-and-egg
ordering problem. This closes BUG-028 at the infra level as claimed.

**TLS.** Dev: no config needed, Traefik's built-in self-signed cert serves
automatically for any `tls: {}` router with no `certResolver` — correctly
relied upon rather than reproduced. Prod: ACME resolver present and
correctly shaped (see above), not yet wired to a route (correct per Out of
Scope — no project domains exist until FEAT-024).

**Base dynamic routing (`tamga.yml`) — verified equivalent to the deleted
Caddyfile, and the split-router fix is real and correctly applied.** Read
the file: four routers (`tamga-api`/`tamga-api-secure` on
`Host(\`localhost\`) && PathPrefix(\`/api\`)` at priority 10,
`tamga-ui`/`tamga-ui-secure` on plain `Host(\`localhost\`)` at priority 1),
two services (`tamga-backend` → `http://backend:8080`, `tamga-frontend` →
`http://frontend:3000`). The `-secure` variants each set `tls: {}` and only
list `entryPoints: [websecure]`; the plain variants list only `entryPoints:
[web]` with no `tls` key at all — this is the documented Traefik pattern
the dev describes (a router with `tls: {}` doesn't actually attach to a
listed-but-non-TLS entrypoint), correctly applied here as two routers per
route rather than one dual-entrypoint router. This gives side-by-side
`http://localhost` and `https://localhost` with no forced redirect, which
is the deleted Caddyfile's actual dev behavior.

**Credibility of the dev's validation claim.** The claimed
throwaway-container validation (config parses clean, file-provider routers
loaded, path-based routing confirmed via per-router Prometheus counters,
TLS default cert, ACME dir auto-created, dashboard/metrics reachable) is
consistent with what's on disk — I independently found `traefik:v3.7`,
`traefik:v3.3`, and `traefik/whoami:latest` all present in the local image
cache (`docker images`), matching both this task's and TEST-010's claimed
throwaway runs, and the live `tamga-caddy-1`/`tamga-backend-1`/
`tamga-frontend-1` stack is still running untouched, matching the "live
stack confirmed untouched" claim. I did not re-run a fresh throwaway
container myself (config-file analysis plus `docker compose config`
already closes every gap TEST-013 doesn't already own), but nothing in the
static config contradicts the claimed empirical results, and the specific
finding (dual-entrypoint `tls:{}` router silently not serving plain HTTP)
is a genuinely non-obvious Traefik behavior that reads as something
actually hit in practice, not fabricated.

**Caddyfile removal.** Deleted; grepped `docker-compose.yml` and the repo
root for lingering references — only historical/comparative comments
remain (e.g. "Caddy never joined this network"), no functional dangling
reference. Backend still importing `repository/caddy` and referencing
`Caddy*` env vars is expected per this task's own Out-of-Scope /
Affected-Areas notes (FEAT-024's job).

**KISS.** No over-config: static file is a direct, flat translation of
TEST-010's recommended shape with no speculative abstraction; the
four-router split in `tamga.yml` is the minimum needed to make both
entrypoints actually work (not gold-plating — removing either the plain or
secure pair regresses one of the two protocols).

**Acceptance Criteria walk:**
- [x] Traefik v3 service (web/websecure entrypoints, file provider on
  mounted dynamic dir, metrics entrypoint, admin-only dashboard), no caddy
  service, `Caddyfile` removed — confirmed.
- [x] Traefik joins both the core network and `tamga-net` — confirmed,
  cross-checked against `EnsureNetwork`'s call sites.
- [x] `traefik.yml` + base `tamga.yml` present and valid (`docker compose
  config` validates; file-provider dir contains a real file, not just
  `.gitkeep`) — confirmed.
- [x] ACME resolver configured for prod, dev uses default cert — confirmed.
- [x] `docker compose config` validates; metrics/dashboard not
  host-published — confirmed directly.
- [x] KISS/YAGNI — confirmed, no over-engineering found.

**Non-blocking notes:**
- `api.insecure: true` on the `traefik` entrypoint diverges from TEST-010
  §5's stricter phrasing ("do **not** use `api.insecure`... for the real
  migration", preferring a JWT-gated backend proxy). This task's own
  Requirements text is looser than TEST-010's recommendation here ("NOT
  publicly exposed... not mapped to a host port, **or** behind the api
  entrypoint only" — an explicit either/or), and the entrypoint genuinely
  isn't host-published, so this is compliant with what this task was
  actually asked to build, not a defect. Worth a follow-up task (not this
  one) to swap in TEST-010 §5 option 1 (JWT-gated backend proxy to the
  dashboard) before this ever sees a less-trusted network than today's
  single-host compose setup — flagged for awareness, not blocking.
- `AGENTS.md` appears as deleted in the working tree but does not exist on
  disk and isn't mentioned anywhere in this task's Implementation Notes —
  this is ambient pre-existing dirty state unrelated to this task (the repo
  routinely has other in-progress/unrelated work sitting uncommitted), not
  scope creep by this task's developer. Confirmed via `git log -1 -- 
  AGENTS.md` pointing at an old, unrelated commit, not anything in this
  session's history.

## Test Notes
<filled in by tester>

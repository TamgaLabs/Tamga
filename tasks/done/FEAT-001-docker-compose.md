---
id: FEAT-001
type: feature
title: docker-compose.yml for single-command deployment
status: done
complexity: simple
assignee: opencode
created: 2026-07-04
history:
  - {date: 2026-07-04, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-04, stage: in-development, by: architect, note: "assigned to opencode"}
  - {date: 2026-07-04, stage: in-development, by: architect, note: "opencode implemented compose+Makefile; architect corrected tamga-network internal:true bug (also fixed in architecture.md)"}
  - {date: 2026-07-04, stage: in-review, by: architect, note: "moved to review"}
  - {date: 2026-07-04, stage: changes-requested, by: architect, note: "reviewer found Caddyfile still points at tamga-backend/tamga-frontend, unresolvable under compose service names backend/frontend; sent back for fix"}
  - {date: 2026-07-04, stage: in-review, by: architect, note: "opencode fixed Caddyfile + deploy/Caddyfile hostnames; back to review"}
  - {date: 2026-07-04, stage: in-test, by: architect, note: "review PASSED, moved to test"}
  - {date: 2026-07-04, stage: done, by: architect, note: "test PASSED end-to-end (build, route, restart-persist); moved to done"}
---

## Summary
Tamga is currently brought up with a `make` target that runs manual `docker run`
commands. architecture.md specifies a single-command deployment via
`docker compose up -d`. Add a `docker-compose.yml` at the repo root covering
caddy, backend, and frontend services, replacing the manual `docker run` flow.

## Requirements
- Add `docker-compose.yml` at repo root with services: `caddy`, `backend`, `frontend`
- `tamga-network` is defined and managed by compose (see architecture.md's
  Docker Compose section for the reference shape, `internal: true` is fine for
  now — full whitelist-egress network split for agent sandboxes is handled in
  FEAT-006, not here)
- Only Caddy publishes host ports (`80:80`, `443:443`)
- Backend mounts `/var/run/docker.sock:/var/run/docker.sock` and `./data:/data`
- Caddy mounts a generated `./Caddyfile` plus `caddy_data`/`caddy_config` volumes
- Backend reads config via `env_file: .env`
- `make` targets (if any remain useful, e.g. `make logs`) may be updated to
  wrap `docker compose` instead of removed outright — check the Makefile
  first and use judgment

## Out of Scope
- Backend env var additions (`CADDY_AUTO_SSL`, `UI_DOMAIN`, `API_DOMAIN`) — see FEAT-002
- Agent sandbox network isolation/whitelist — see FEAT-006
- README rewrite — see FEAT-003

## Proposed Solution / Approach
Create `docker-compose.yml` at repo root matching the reference in architecture.md
with three services (caddy, backend, frontend), a named volume for Caddy data/config,
and an internal `tamga-network`. Use `dockerfile:` keys so the existing Dockerfiles
under `deploy/` are used without moving them. Copy the initial Caddyfile from
`deploy/Caddyfile` to repo root (it is mounted by both caddy and backend).
Adapt the Makefile to delegate to `docker compose` instead of manual `docker run`.

## Affected Areas
- `docker-compose.yml` (new)
- `Makefile` (if present, adapt or remove docker-run targets)

## Acceptance Criteria / Definition of Done
- [ ] `docker compose up -d` brings up caddy, backend, frontend with one command
- [ ] No service besides Caddy exposes a host port
- [ ] Backend container can reach `/var/run/docker.sock`
- [ ] Data/projects persist across `docker compose down && docker compose up -d`
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Run `docker compose up -d`, confirm all three containers are healthy
(`docker compose ps`), hit the frontend and backend through Caddy, restart
the stack and confirm SQLite data survives.

## Implementation Notes
- Created `docker-compose.yml` with caddy (image), backend (build), frontend (build)
- Backend build context is root `.` with `dockerfile: deploy/Dockerfile.backend`
- Frontend build context is root `.` with `dockerfile: deploy/Dockerfile.frontend`
- Only Caddy publishes host ports (80, 443)
- Mounts: caddy gets `./Caddyfile` + named volumes; backend gets docker.sock + `./data` + `./Caddyfile`
- Network `tamga-network` is internal; managed by compose lifecycle
- Caddy named volumes `caddy_data` and `caddy_config` declared in top-level `volumes:`
- Copied `deploy/Caddyfile` to root `./Caddyfile` as the initial config
- Updated Makefile: `build`, `up`, `down`, `logs`, `clean` wrap `docker compose`; dropped `network` target; `setup` and `test` unchanged
- Architect correction: opencode's initial pass set `tamga-network: internal: true`, copied verbatim from architecture.md's sample. That would block Caddy's outbound Let's Encrypt requests and the backend's `git clone`. Changed to a plain bridge network (removed `internal: true`) and updated architecture.md's Docker Compose sample + Networking section to match — egress isolation stays scoped to the agent-sandbox network (FEAT-006).
- Review fix: Caddyfile upstream hostnames `tamga-backend:8080` / `tamga-frontend:3000` changed to `backend:8080` / `frontend:3000` to match compose service names. Synced `deploy/Caddyfile` too so future re-copies don't reintroduce the mismatch.

## Review Notes
### 2026-07-04 — architect (review pass)

**Verdict: CHANGES_REQUESTED**

**Blocking issue — Caddyfile upstream hostnames don't match compose service names.**

`docker-compose.yml` defines services named `backend` and `frontend` (no
`container_name` override), so on `tamga-network` the resolvable DNS names are
`backend`/`frontend` (and their compose-generated container names
`tamga-backend-1`/`tamga-frontend-1`, confirmed via `docker compose config` —
project name resolves to `tamga`). Neither matches the hostnames the newly
copied root `Caddyfile` reverse-proxies to:

- `/home/okal/Projects/Tamga/Caddyfile:9` — `reverse_proxy tamga-backend:8080`
- `/home/okal/Projects/Tamga/Caddyfile:12` — `reverse_proxy tamga-frontend:3000`

Those hostnames were correct for the *old* manual `docker run --name
tamga-backend/--name tamga-frontend` flow (see the pre-change Makefile), but
compose's default network alias is the service name, not that literal string,
and there's no exact-match DNS fallback. As implemented, Caddy would fail to
resolve the upstream for every request to the base domain and both `@api` and
the default handler would 502 — this breaks the Test Plan step "hit the
frontend and backend through Caddy" and effectively defeats the whole
`docker compose up -d` acceptance criterion (containers come up, but the
stack isn't actually reachable through Caddy).

For contrast, `CADDY_ADMIN_URL=http://caddy:2019` in `.env.example` was
already correct because it uses the compose service name `caddy` — the same
convention needed to be applied to the backend/frontend upstream lines.

Fix: either add `container_name: tamga-backend` / `container_name:
tamga-frontend` to the respective services in `docker-compose.yml`, or (simpler,
and consistent with architecture.md's reference compose sample which uses bare
service names) change `Caddyfile:9` and `:12` to `reverse_proxy backend:8080`
and `reverse_proxy frontend:3000`. If the latter, consider syncing
`deploy/Caddyfile` too so future re-copies don't reintroduce the mismatch
(non-blocking, `deploy/Caddyfile` wasn't in this task's Affected Areas, but
worth a follow-up note).

**What checks out:**

- `docker-compose.yml` network: correctly `driver: bridge` (not `internal:
  true`), matches architect's correction and architecture.md's updated sample.
- Only `caddy` publishes host ports (`80:80`, `443:443`); `backend`/`frontend`
  have none — meets "no service besides Caddy exposes a host port".
- Backend mounts `/var/run/docker.sock:/var/run/docker.sock` and
  `./data:/data`, reads config via `env_file: .env` — matches Requirements
  verbatim.
- Caddy mounts `./Caddyfile`, `caddy_data`, `caddy_config` — matches.
- `./data` is a host bind mount (not a named volume), so `docker compose down`
  and even `make clean` (`down -v`) won't destroy it — SQLite/project data
  persistence criterion is satisfied.
- `docker compose config` parses the file cleanly with no syntax issues.
- Makefile: sensible, minimal wrap over `docker compose`; dropping the old
  `network` target is correct since compose now owns the network lifecycle;
  `setup`/`test` untouched, consistent with "use judgment" guidance.
- No speculative abstraction; the compose file is about as simple as this
  requirement allows — KISS/YAGNI satisfied.

**Non-blocking note:** the old manual `docker run` flow bind-mounted
`$(SYSTEM_CODE_DIR):$(SYSTEM_CODE_DIR):ro` into the backend container, which
backend/internal/handler/code_handler.go relies on for the built-in "system"
project (self-hosting Tamga-on-Tamga agent operations directly against the
repo checkout). `docker-compose.yml` drops that mount. This matches
architecture.md's own reference sample (which also omits it), so it's not a
deviation from what this task asked for, but it likely silently breaks the
"system" project's agent features under compose-based deployment. Flagging
for the architect to confirm whether that's intentional/deferred or needs a
follow-up task.

### 2026-07-04 — architect (re-review pass)

**Verdict: PASS**

Confirmed the fix for the previously blocking issue:

- Root `Caddyfile:9` and `:12` now read `reverse_proxy backend:8080` and
  `reverse_proxy frontend:3000` respectively — these resolve correctly on
  `tamga-network` against the compose service names defined in
  `docker-compose.yml` (`backend`, `frontend`).
- `deploy/Caddyfile` received the identical fix (`git diff` shows only the
  two hostname lines changed, nothing else), so future copies from
  `deploy/Caddyfile` to the root won't reintroduce the mismatch. Good
  follow-through on the non-blocking suggestion from the first pass.
- No unrelated files were touched by this fix; `docker-compose.yml`,
  `Makefile`, and `architecture.md` are unchanged since the last pass, so
  everything previously verified (network mode, port exposure, mounts,
  `env_file`, data persistence via bind mount, Makefile wrap-over-compose,
  KISS/YAGNI) still holds.

All Requirements and Acceptance Criteria are now satisfied:
- [x] `docker compose up -d` brings up caddy, backend, frontend with one command
- [x] No service besides Caddy exposes a host port
- [x] Backend container can reach `/var/run/docker.sock`
- [x] Data/projects persist across `docker compose down && docker compose up -d`
- [x] Code follows KISS/YAGNI — no speculative abstraction

**Non-blocking (carried over, unresolved, not part of this task's scope):**
the dropped `$(SYSTEM_CODE_DIR):$(SYSTEM_CODE_DIR):ro` mount for the
built-in "system" project's agent operations — still worth a follow-up task
to confirm intentional deferral, per the note in the first review pass.

## Test Notes
<Filled in by the tester.>

### 2026-07-04 — QA tester

**Verdict: PASS**

Environment: local Docker (Docker 29.6.1, Compose v5.1.4), no `.env` existed
in the repo per the task description's caveat except a leftover local
`.env` from earlier manual dev testing (`DOMAIN=localhost`,
`ADMIN_PASSWORD=12345678`, `CADDY_ADMIN_URL=http://caddy:2019`, etc.) — used
it as-is, it matches `.env.example`'s shape plus real values, no changes
needed.

**1. `docker compose config`** — parsed cleanly, no errors. Confirmed
compose project name `tamga`, service DNS names `backend`/`frontend`/`caddy`,
network `tamga_tamga-network` (`driver: bridge`, not `internal`), volumes
`tamga_caddy_data`/`tamga_caddy_config`.

**2. `docker compose build` then `docker compose up -d`** — all three images
built successfully (backend via `deploy/Dockerfile.backend`, frontend via
`deploy/Dockerfile.frontend`, caddy pulled `caddy:alpine`). `docker compose ps`
after ~4s:
```
NAME               IMAGE            SERVICE    STATUS         PORTS
tamga-backend-1    tamga-backend    backend    Up 4 seconds   8080/tcp
tamga-caddy-1      caddy:alpine     caddy      Up 4 seconds   0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp, ...
tamga-frontend-1   tamga-frontend   frontend   Up 4 seconds   3000/tcp
```
No healthchecks are defined in the compose file (not required by this
task), so "healthy" was verified via logs + actual HTTP traffic rather than
a Docker healthcheck status. Backend logs showed migrations running cleanly
(8 migration files) + "admin user ready" + "server starting port=8080".
Caddy logs showed it obtained a local-CA cert for `localhost` (auto_https,
since `DOMAIN=localhost` is treated as an internal/non-public domain) and
"serving initial configuration" with no errors. Frontend logs showed
Next.js "✓ Ready in 63ms".

**3. Only Caddy publishes host ports** — confirmed via `docker compose ps`
(above) and `docker inspect tamga-backend-1 tamga-frontend-1 --format
'{{.Name}}: {{.NetworkSettings.Ports}}'` → both show `8080/tcp:[]` /
`3000/tcp:[]` (container port exposed, no host binding). Caddy is the only
one with `0.0.0.0:80->80/tcp` / `0.0.0.0:443->443/tcp`.

**4. Routing through Caddy:**
- `curl -o /dev/null -w '%{http_code} -> %{redirect_url}' http://localhost/`
  → `308 -> https://localhost/` (expected: Caddy auto-HTTPS redirect since
  `DOMAIN=localhost`).
- `curl -k -L http://localhost/` → `200`, real Next.js HTML for the Tamga
  dashboard (`<title>Tamga</title>`, dashboard chunks) — frontend route
  works through Caddy's default `handle {}` block → `frontend:3000`.
- `curl -k -L http://localhost/api/auth/status` → `200 {"setup":true}` —
  confirms `@api path /api/*` block correctly resolves and reverse-proxies
  to `backend:8080` (this is the exact hostname bug the review round-trip
  fixed — `backend`/`frontend`, not `tamga-backend`/`tamga-frontend` — and
  it works).
- Full authenticated round trip through Caddy: `POST
  https://localhost/api/auth/login` (note: had to hit `https://` directly
  with `-k`, not `-L` off an `http://` redirect — curl correctly drops the
  `Authorization` header across a scheme-changing redirect per RFC/curl
  security policy, which is curl's own behavior, not an app bug) →
  `200 {"token": "..."}`. Then `GET https://localhost/api/auth/me` with the
  bearer token → `200 {"user_id":1}`. Then `GET
  https://localhost/api/system/info` (exercises backend →
  `/var/run/docker.sock` end-to-end through the whole stack) → `200
  {"architecture":"x86_64","containers":7,...,"version":"29.6.1"}` — real
  live docker daemon stats, confirming the socket mount actually works, not
  just that it's present.
- `docker compose exec backend ls -la /var/run/docker.sock` → present,
  `srw-rw---- root 954` — socket mount confirmed at the filesystem level
  too.

**5. Data persistence across restart:**
- Before restart: created project id=1 via `POST /api/projects` (name
  `persistence-test`, domain `persistence-test.localhost`, a real public
  repo URL) → `201`, `{"id":1,...,"status":"created"}`. Listed via `GET
  /api/projects` → present with status `cloning`.
- Ran `docker compose down` → containers/network removed, but
  `./data/tamga.db` and `./data/projects/` remained on the host (bind
  mount, not a named volume) — confirmed with `ls -la ./data` immediately
  after `down`.
- Ran `docker compose up -d` → all three containers came back up in ~4s.
  Backend logs showed migrations re-running against the *existing*
  `tamga.db` (idempotent, no errors) and "admin user ready".
- `GET https://localhost/api/auth/status` → `{"setup":true}` (did not
  reset/re-prompt setup — the persisted `users` row survived).
- Logged in again with the *same* password (`12345678`, unchanged) → `200`
  with a fresh JWT — proves the bcrypt-hashed admin credential persisted.
- `GET https://localhost/api/projects` with the new token → `200
  [{"id":1,"name":"persistence-test",...}]` — the project created *before*
  the restart is still there (its background deploy goroutine had since
  moved to status `error`, unrelated to persistence — expected, since
  `octocat/Hello-World` isn't a deployable app; the row itself surviving
  the down/up cycle is what was under test).
- Frontend still served `200` through Caddy after the restart.

**Acceptance criteria checked against the running system:**
- [x] `docker compose up -d` brings up caddy, backend, frontend with one command
- [x] No service besides Caddy exposes a host port
- [x] Backend container can reach `/var/run/docker.sock` (verified live, not just mount presence)
- [x] Data/projects persist across `docker compose down && docker compose up -d`
- [x] Code follows KISS/YAGNI — no speculative abstraction (matches reviewer's assessment, nothing to add)

**Unrelated bug found, NOT part of this task's scope (flagging only,
not blocking this task):** `POST /api/system/api-keys` panics with a nil
pointer dereference in `ApiKeyService.Set` (`api_key_service.go:29`),
recovered by chi's `Recoverer` middleware into a `500`. The
`api_key_handler.go`/`api_key_service.go`/`api_key_repo.go`/`api_key.go`
files are untracked (`??` in `git status`), i.e. in-progress work from a
different task, not touched by FEAT-001's diff (`docker-compose.yml`,
`Makefile`, `Caddyfile`, `deploy/Caddyfile` only). Switched to the
`/api/projects` endpoint for the persistence check instead. Worth a
separate bug ticket against whatever task owns the API-key feature.

**Cleanup performed:** `docker compose down`; removed the
`tamga_caddy_data`/`tamga_caddy_config` volumes created during this test
session; removed `./data/tamga.db` and `./data/projects/` (created by this
test run, root-owned since the backend container runs as root, removed via
a throwaway `alpine` container mounting `./data`) so the working tree's
`data/` dir is back to its pre-test state (only the pre-existing
`test.db` remains, unrelated to this session). No source files were
modified.

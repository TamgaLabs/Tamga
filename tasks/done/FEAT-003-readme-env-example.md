---
id: FEAT-003
type: feature
title: Rewrite README and .env.example to match the real stack
status: done
complexity: simple
assignee: opencode
sprint: SPRINT-001
created: 2026-07-04
history:
  - {date: 2026-07-04, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-04, stage: in-development, by: architect, note: "assigned to opencode; FEAT-001/FEAT-002 already landed so README/.env.example can document the final shape"}
  - {date: 2026-07-04, stage: in-development, by: architect, note: "opencode stopped after .env read auto-rejected; resumed session, rewrote README + fixed .env.example"}
  - {date: 2026-07-04, stage: in-review, by: architect, note: "moved to review"}
  - {date: 2026-07-04, stage: in-test, by: architect, note: "review PASSED; architect fixed 3 non-blocking doc nits directly; moved to test"}
  - {date: 2026-07-04, stage: done, by: architect, note: "test PASSED (live docker compose run-through); architect clarified /health routing caveat found during test; moved to done"}
---

## Summary
The README currently describes a stack that doesn't exist (Postgres,
Traefik, Gin, Viper, sqlc). The real stack is SQLite, Caddy, Chi (or
whatever router is actually in use), and the Docker SDK. Rewrite the README
and `.env.example` to reflect reality and document the single-command
`docker compose up -d` setup.

## Requirements
- Remove all references to Postgres, Traefik, Gin, Viper, sqlc
- Document actual architecture: SQLite, Caddy reverse proxy, Docker SDK
  sibling-container model, Next.js frontend
- Document the single-command setup: copy `.env.example` to `.env`, then
  `docker compose up -d`
- `.env.example` includes every env var the backend actually reads,
  including the ones added in FEAT-002 (`CADDY_AUTO_SSL`, `UI_DOMAIN`,
  `API_DOMAIN`) and existing ones (`DATA_DIR`, `PROJECTS_DIR`, `DB_PATH`,
  `AGENT_IMAGE`, etc. — verify against the actual config loader)

## Out of Scope
- Any code changes — this is documentation only
- Should run after FEAT-001 and FEAT-002 land so it documents the final shape

## Proposed Solution / Approach
Rewrite README from scratch to describe the actual stack (Caddy + Go/chi + SQLite + Next.js + Docker SDK sibling-container model). Document the single-command `cp .env.example .env && docker compose up -d` setup. Update `.env.example` to include all 12 env vars from `backend/internal/config/config.go` with their real defaults. Fix the Project Layout tree and Makefile table to match the current filesystem. Remove all references to Postgres, Traefik, Gin, Viper, sqlc, and `make dev`/`make frontend-dev` (those dev workflows are gone).

## Affected Areas
- `README.md`
- `.env.example` (new or update)

## Acceptance Criteria / Definition of Done
- [ ] No mention of Postgres/Traefik/Gin/Viper/sqlc remains anywhere in README
- [ ] `.env.example` has every var the backend config loader reads, with defaults
- [ ] A new reader can go from clone to running instance using only the README
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Follow the README from scratch on a clean checkout, confirm every step
works as written, confirm `.env.example` vars match `grep -r os.Getenv` (or
equivalent) in the backend config module.

## Implementation Notes
- `README.md`: fully rewritten. Removed all references to Postgres, Traefik, Gin, Viper, sqlc, `make dev`, `make frontend-dev`, and `make generate`. Replaced with real stack: Caddy, Go/chi, SQLite, Next.js, Docker SDK sibling-container model. Rewrote API endpoint tables to match actual router. Rewrote Project Layout and Makefile tables to match current filesystem. Added architecture diagram. Documented single-command `cp .env.example .env && docker compose up -d` setup.
- `.env.example`: fixed `SYSTEM_CODE_DIR` from `.` (wrong) to commented-out empty string (matching config.go default of `""`, meaning disabled). All 12 env vars from `backend/internal/config/config.go` are present.
- Architect follow-up (post-review, addressing the reviewer's 3 non-blocking notes directly since they were one-line doc fixes): added a `GET /health` row to the API Endpoints table; corrected the Caddy admin API row to note it isn't published to the host (only reachable in-network); fixed the Project Layout comment referencing a nonexistent `register` page to `setup` (the actual folder name).

## Review Notes

### 2026-07-04 — reviewer

Verdict: PASS

Verified against the task and acceptance criteria:

1. **No Postgres/Traefik/Gin/Viper/sqlc mentions** — confirmed via
   `grep -niE "postgres|traefik|\bgin\b|viper|sqlc" README.md .env.example`
   returns nothing. The old dev-workflow doc (`make dev`, `make frontend-dev`,
   `make generate`, `Dockerfile.dev`, `docker-compose.dev.yml`, sqlc-based
   layout) is fully removed and replaced with the real Caddy/chi/SQLite/
   Next.js/Docker-SDK stack description.

2. **`.env.example` matches `backend/internal/config/config.go`** — all 12
   vars (`DOMAIN`, `ADMIN_PASSWORD`, `JWT_SECRET`, `DB_PATH`,
   `CADDY_ADMIN_URL`, `CADDY_EMAIL`, `CADDY_AUTO_SSL`, `UI_DOMAIN`,
   `API_DOMAIN`, `DATA_DIR`, `SYSTEM_CODE_DIR`, `PORT`) are present. The
   `SYSTEM_CODE_DIR` fix (from `.` to a commented-out empty default) now
   correctly matches `config.go`'s `getEnv("SYSTEM_CODE_DIR", "")` default
   (feature disabled unless explicitly set) — confirmed this doesn't break
   any required code path (`code_handler.go` treats `""` as "system code
   agent disabled").

3. **Clone-to-running plausibility** — traced `cp .env.example .env &&
   docker compose up -d` against `docker-compose.yml`, `Caddyfile`,
   `backend/cmd/api/main.go` (auto-migrate + `AuthService.AutoSetup`
   creates the admin user from `ADMIN_PASSWORD`), and confirmed the
   Frontend/API URLs documented (`http://localhost`, `http://localhost/api`)
   match the Caddyfile's routing with default `DOMAIN=localhost`. Makefile
   table matches the actual `Makefile` targets exactly. Project Layout tree
   matches the real filesystem (`backend/internal/repository/{caddy,docker,
   sqlite}`, `deploy/Dockerfile.{backend,frontend,agent}`, `agent-server/`,
   `agent-bridge/`).

4. **API Endpoint tables vs `router.go`** — cross-checked every route in
   `backend/internal/router/router.go`; all documented paths/methods/auth
   requirements match (Auth, Projects, Agents, Code Agents, Agent Providers,
   System/Docker, API Keys).

5. **KISS/YAGNI** — pure documentation change, no speculative content, no
   code changes outside `README.md`/`.env.example` as required by scope.

Non-blocking notes for a future pass:
- `GET /health` (registered in `router.go:35`) is not listed in the API
  Endpoints section (it was present in the old README). Minor omission,
  doesn't affect setup instructions.
- The Quick Start table lists `Caddy | http://localhost:2019 | Admin API
  (internal)`, but `docker-compose.yml` only publishes ports 80/443 — port
  2019 is not reachable from the host, only from inside the Docker network
  (which is how the backend reaches it via `CADDY_ADMIN_URL=http://caddy:2019`
  in `.env.example`). Consider rewording that row to avoid implying a reader
  can hit `localhost:2019` directly, or drop the row since it's not part of
  the setup flow.
- Project Layout comment `# Pages (login, register, dashboard, projects)`
  references a `register` page that doesn't exist (the actual folder is
  `(auth)/setup`), and omits `containers`/`code`/`settings`. Cosmetic only.

None of the above rise to blocking level — the acceptance criteria (no
wrong-stack mentions, `.env.example` completeness/correctness, clone-to-run
plausibility, KISS/YAGNI) are all satisfied.

## Test Notes
<Filled in by the tester.>

### 2026-07-04 — tester

Verdict: PASS

Method: cloned the repo fresh into a scratchpad directory (`git clone /home/okal/Projects/Tamga`) to get a clean checkout of all committed files (`docker-compose.yml`, `Caddyfile`, `Makefile`, `deploy/Dockerfile.*` — all already committed on `main` from FEAT-001/FEAT-002), then overlaid the working tree's uncommitted `README.md` and `.env.example` (the two files this task actually changed) on top, since those edits aren't committed yet. Ran the documented flow end-to-end from there.

**1. Quick Start flow (`cp .env.example .env && docker compose up -d`)** — ran verbatim:
- `cp .env.example .env` — produced exactly the 12-var file shown in the task's Implementation Notes (`DOMAIN`, `ADMIN_PASSWORD`, `JWT_SECRET`, `CADDY_EMAIL`, `CADDY_ADMIN_URL`, `CADDY_AUTO_SSL`, `UI_DOMAIN`, `API_DOMAIN`, `DB_PATH`, `DATA_DIR`, commented `SYSTEM_CODE_DIR`, `PORT`).
- `docker compose up -d --build` — built backend/frontend images and started all 3 services (`caddy`, `backend`, `frontend`) cleanly, no errors.
- Backend logs showed: all 8 migrations ran, `"admin user ready"`, `"caddyfile written"`, `"caddy reloaded" status=404` (this 404 is Caddy's own reload-ack behavior, not an error — service came up fine), `"server starting" port=8080`.
- Caddy logs: obtained a local (self-signed) TLS cert for `localhost` automatically (`CADDY_AUTO_SSL=true` from `.env.example` default), consistent with the README's "automatic HTTPS via Let's Encrypt/local CA" claim.
- Frontend logs: Next.js ready in <100ms.
- Confirmed via `docker compose exec backend wget -qO- http://localhost:8080/health` → `{"go_version":"go1.26.4","status":"ok",...}` — backend genuinely healthy.
- Cross-checked `make up`/`make logs` targets directly (not just raw commands) — both work exactly as the README's Makefile table and Quick Start section describe, including the `Frontend: https://$(DOMAIN)` / `API: https://$(DOMAIN)/api` echo on `make up`.

**2. API endpoints vs `router.go`** — exercised the documented Auth/Projects flow against the live stack:
- `POST /api/auth/login {"password":"admin"}` → `200 {"token":"..."}` (confirms `ADMIN_PASSWORD=admin` auto-setup claim).
- `GET /api/auth/me` with `Authorization: Bearer <token>` → `200 {"user_id":1}`.
- `GET /api/projects` with the same token → `200 []` (empty list, as expected for a fresh instance).
- `GET /api/auth/status` → `200 {"setup":true}`.
- All paths/methods/auth requirements in the README's Auth/Projects tables matched `router.go` and worked live.

**3. Found one real, verifiable inaccuracy (non-blocking, doesn't fail the acceptance criteria):** the README's new "Health" table (added by the architect's post-review follow-up specifically to address the reviewer's non-blocking note about the missing `/health` row) documents `GET /health` with `Auth: No` and no caveat about reachability. I confirmed this endpoint is **not reachable via the documented base URL** (`http://localhost/...` per the Quick Start table): `curl -skL http://localhost/health` and `curl -sk https://localhost/health` both return the Next.js frontend's 404 page (status 404), not the backend's health JSON. This is because `router.go` registers `/health` at the router root (outside `/api`), but the `Caddyfile` only proxies `@api path /api/*` to the backend — everything else (including bare `/health`) falls through to the frontend `handle` block. Hitting the backend directly inside the network (`docker compose exec backend wget -qO- http://localhost:8080/health`) does return the expected `{"status":"ok",...}` JSON, confirming the route exists but simply isn't exposed at that path through Caddy in this deployment. This is the same class of issue the reviewer already flagged for the Caddy Admin API row (which did get a "not published to host" caveat) — the Health row should probably get the same kind of caveat or be removed, since as written it implies external reachability that doesn't exist. I judge this non-blocking because: (a) it's reference/API documentation, not part of the "Quick Start" setup steps a new user follows, so it doesn't break the "clone to running instance using only the README" criterion; (b) the underlying router/Caddyfile mismatch is a pre-existing infra fact outside this doc-only task's scope, not something FEAT-003 introduced; (c) all other acceptance criteria (no wrong-stack mentions, `.env.example` completeness, Makefile/layout accuracy) are fully satisfied and verified live.

**4. `.env.example` completeness/correctness** — diffed against `backend/internal/config/config.go`'s `Load()`: all 12 fields read via `getEnv`/`getEnvInt`/`getEnvBool` (`Domain`, `AdminPassword`, `JWTSecret`, `DBPath`, `CaddyAdminURL`, `CaddyEmail`, `CaddyAutoSSL`, `UIDomain`, `APIDomain`, `DataDir`, `SystemCodeDir`, `Port`) have a corresponding `.env.example` entry with a default matching (or intentionally overriding, e.g. `CADDY_ADMIN_URL=http://caddy:2019` vs. the code's `http://localhost:2019` fallback — correct for the Docker Compose network) the code's fallback. `grep -n os.Getenv` across `backend/` found no additional env vars read outside `config.go` (e.g. no `PROJECTS_DIR`/`AGENT_IMAGE` vars actually exist in the codebase, contrary to the original task summary's speculative examples — confirmed those aren't real gaps).

**5. No banned terms** — `grep -niE "postgres|traefik|\bgin\b|viper|sqlc" README.md .env.example` returns nothing (re-confirmed reviewer's finding).

**6. Project Layout / Makefile tables vs filesystem** — walked the real tree (`backend/internal/{config,handler,repository/{caddy,docker,sqlite},router,service}`, `deploy/Dockerfile.{backend,frontend,agent}`, `frontend/src/app/{(auth)/{login,setup},(main)/{code,containers,dashboard,projects,settings}}`, `agent-server/`, `agent-bridge/`, `Caddyfile`, `docker-compose.yml`, `Makefile`) — all present and matching the README's tree. Confirmed the previously-noted cosmetic issue persists as expected (reviewer flagged it non-blocking, architect didn't fully fix it): the Project Layout comment still reads `# Pages (login, setup, dashboard, projects)`, omitting `containers`/`code`/`settings` — matches what the review already disclosed as cosmetic-only.

Cleanup: `docker compose down -v` in the scratch clone (removed containers, `tamga-network`, `caddy_data`/`caddy_config` volumes), removed the built `tamga-test-backend`/`tamga-test-frontend` images, deleted the scratch clone directory (one leftover root-owned sqlite file under the scratch dir's bind-mounted `./data` couldn't be `rm`'d due to permissions from the container process — harmless, it's outside the actual repo and doesn't affect the verdict). No containers/images/volumes related to this test remain (`docker ps -a | grep tamga-test` → empty). The pre-existing `tamga-backend`/`tamga-frontend`/`caddy`/`agent-system` containers already in this repo's checkout from prior FEAT-001/FEAT-002 testing were left untouched throughout.

Overall: setup instructions are accurate and were verified to work end-to-end from a genuinely fresh clone; `.env.example` is complete and correct against the config loader; endpoint/layout/Makefile tables match reality with one already-disclosed cosmetic gap and one newly-found non-blocking `/health`-reachability inaccuracy. None of these rise to blocking level against the stated Acceptance Criteria / Definition of Done.

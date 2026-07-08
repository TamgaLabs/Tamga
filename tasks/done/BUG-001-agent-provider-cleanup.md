---
id: BUG-001
type: bug
title: Fix agent_provider Command mismatch / simplify fields post-ACP
status: done
complexity: simple
assignee: opencode
sprint: SPRINT-001
created: 2026-07-04
history:
  - {date: 2026-07-04, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-05, stage: in-development, by: architect, note: "FEAT-004 landed (removed ProviderTypeHTTP + BuildBridge already); assigned to opencode for the remaining Command/Endpoint/AuthToken cleanup"}
  - {date: 2026-07-05, stage: in-review, by: architect, note: "opencode removed dead Command/Endpoint/AuthToken fields + added drop-column migration; moved to review"}
  - {date: 2026-07-05, stage: in-test, by: architect, note: "review PASSED (migration verified end-to-end against real SQLite); unrelated pre-existing bug found and filed as BUG-007; moved to test"}
  - {date: 2026-07-05, stage: done, by: architect, note: "test PASSED (migration verified on fresh DB + full docker stack through Caddy, Settings page renders clean); moved to done"}
---

## Summary
`agent_provider.go`'s default `Command` (`opencode acp`) doesn't match the
migration seed's `Command` (`opencode --stdin --diff`, in
`000008_create_agent_providers.up.sql`) — the DB seed wins in practice, so
the Go-level default is dead/misleading. This is also now moot: FEAT-004
removes the ACP protocol entirely, so `Command`/`Endpoint`/`AuthToken` on
`agent_provider` no longer describe "how to talk ACP to this provider" —
they may only need to describe "which image to launch." This task depends
on FEAT-004 having landed.

## Steps to Reproduce
1. Compare the default `Command` value in `agent_provider.go` against the
   seed value in `000008_create_agent_providers.up.sql`
2. Observe they differ (`opencode acp` vs `opencode --stdin --diff`), and
   the seed value is what's actually used since it's already in the DB

## Expected Behavior
`agent_provider`'s fields accurately reflect their real purpose after
FEAT-004 removes the ACP bridge, with no dead/inconsistent defaults.

## Actual Behavior
A default that's never actually applied (since seed data wins), describing
a protocol command that no longer means anything once ACP is removed.

## Environment / Context
Depends on FEAT-004 (ACP bridge removal) landing first — do not start this
until that task is in `tasks/done/`.

## Root Cause

Three `AgentProvider` fields (`Command`, `Endpoint`, `AuthToken`) are leftover from the ACP-bridge / ProviderTypeHTTP days. After FEAT-004:

- **`Endpoint` / `AuthToken`** — were only meaningful for `ProviderTypeHTTP` (removed by FEAT-004). Dead code.
- **`Command`** — the Go default (`"opencode acp"`) never matched the DB seed (`"opencode --stdin --diff"`). Worse, `Command` is **never used at runtime**: `ensureContainerRunning` in `agent_service.go:46` accepts only `image` and `env`, and `CreateContainerOpts` in `docker/client.go:52` has no command parameter. So `Command` is stored and surfaced in the UI but never applied. Dead code.
- `ProviderConfig` struct in `agent_provider.go:25` is defined but never referenced anywhere.
- Frontend at `frontend/src/app/(main)/settings/page.tsx:286-305` edits `command` in the form and sends it to the API, but the backend ignores it.

## Proposed Solution

Remove `Command`, `Endpoint`, and `AuthToken` entirely — they are unused at runtime (neither passed to Docker nor used in any business logic). Only `Image` and `Env` are actually consumed by `agent_service.go`.

Changes:
1. `backend/internal/domain/agent_provider.go` — drop `Command`, `Endpoint`, `AuthToken` fields from `AgentProvider`; drop unused `ProviderConfig` struct; drop `Command` from `DefaultProvider`.
2. `backend/internal/repository/sqlite/agent_provider_repo.go` — remove all references to `command`, `endpoint`, `auth_token` in INSERT/SELECT/UPDATE queries.
3. `backend/internal/repository/sqlite/migrations/000009_drop_agent_provider_obsolete_fields.up.sql` — `ALTER TABLE agent_providers DROP COLUMN command, endpoint, auth_token;` (new migration, do not edit existing 000008).
4. `frontend/src/app/(main)/settings/page.tsx` — remove `command` from the form state, edit handler, and save payload.
5. `frontend/src/lib/api.ts` — remove `command?` from the `AgentProvider` type.

## Affected Areas
- `backend/internal/domain/agent_provider.go`
- `backend/internal/repository/sqlite/migrations/000008_create_agent_providers.up.sql` (or a new migration, don't edit an already-applied migration file in place — add a follow-up migration if schema changes are needed)
- `backend/internal/handler/agent_provider_handler.go`
- `backend/internal/service/agent_provider_service.go`
- Frontend agent provider UI, if it surfaces these fields

## Acceptance Criteria
- [ ] No inconsistency remains between Go-level defaults and DB seed data
- [ ] `agent_provider` schema/fields reflect their actual post-ACP purpose
- [ ] Existing agent provider records still work after any migration

## Test Plan
Inspect the `agent_provider` table after migrations run, confirm fields
match their documented purpose, confirm the frontend agent provider list
still renders and functions correctly.

## Implementation Notes

All three fields (`Command`, `Endpoint`, `AuthToken`) were fully dead — stored in the DB and surfaced in the UI but **never consumed** by the Docker launch path (`ensureContainerRunning` / `CreateContainerOpts` accept no command parameter).

**Files touched:**
- `backend/internal/domain/agent_provider.go` — removed `Command`, `Endpoint`, `AuthToken` fields; removed unused `ProviderConfig` struct; removed `Command` from `DefaultProvider`.
- `backend/internal/repository/sqlite/agent_provider_repo.go` — removed `command`, `endpoint`, `auth_token` from all INSERT/SELECT/UPDATE queries and Scan args.
- `backend/internal/repository/sqlite/migrations/000009_drop_agent_provider_obsolete_fields.up.sql` — new migration to `ALTER TABLE ... DROP COLUMN` the three columns.
- `backend/internal/repository/sqlite/migrations/000009_drop_agent_provider_obsolete_fields.down.sql` — re-adds the columns.
- `frontend/src/lib/api.ts` — removed `command?` from `AgentProvider` type and `createAgentProvider` signature.
- `frontend/src/app/(main)/settings/page.tsx` — removed `command` state, form field, and payload from `AgentProvidersCard`.

**Not changed:** handler (uses the struct generically, no field-specific logic) and service (no field-specific logic for these fields).

## Review Notes
<Filled in by the reviewer.>

## Test Notes
<Filled in by the tester.>

### 2026-07-05 — Reviewer

**Verdict: PASS**

Verified against the actual working tree:

- `backend/internal/domain/agent_provider.go`: `Command`/`Endpoint`/`AuthToken` and the unused `ProviderConfig` struct are gone; `DefaultProvider` no longer sets `Command`. Matches the Proposed Solution exactly.
- `backend/internal/repository/sqlite/agent_provider_repo.go`: all four CRUD methods (`CreateAgentProvider`, `FindAgentProvider`, `FindDefaultProvider`, `ListAgentProviders`, `UpdateAgentProvider`) have `command`/`endpoint`/`auth_token` fully removed from SQL and `Scan` args, consistently.
- Confirmed dead-at-runtime claim by reading `ensureContainerRunning` (agent_service.go:46) and `CreateContainerOpts` (docker/client.go:52) — neither ever took a command parameter; only `image`/`env`/network/mounts reach Docker. Repo-wide grep for `.Command`, `.Endpoint`, `.AuthToken`, `ProviderConfig` turns up zero remaining references (the one hit, `agent-bridge/main.go:122`, is stdlib `os/exec.Command`, unrelated).
- Migration numbering: 000009 is the next free number, no conflict with 000001–000008.
- SQLite compatibility: driver is `modernc.org/sqlite v1.53.0`, which embeds SQLite 3.53.2 — `ALTER TABLE ... DROP COLUMN` has been supported since SQLite 3.35.0 (2021), so no table-rebuild workaround is needed. `agent_providers` has no indexes/views/triggers referencing the dropped columns (checked all migration files), so `DROP COLUMN` applies cleanly.
- Actually ran the migration runner (`db.Migrate()`) end-to-end against a throwaway on-disk sqlite DB with the real driver: 000001–000009 all applied without error, `PRAGMA table_info(agent_providers)` afterward shows exactly `id, name, type, image, env, is_default, created_at, updated_at` (command/endpoint/auth_token gone), and the seeded `builtin-opencode` row survived the drop with `image` intact — satisfies "existing agent provider records still work after migration." Also ran the `.down.sql` on top of a freshly-migrated DB: it re-adds `command`, `endpoint`, `auth_token` as `TEXT NOT NULL DEFAULT ''` successfully, restoring a usable (if data-empty) schema — reasonable for a rollback path.
- `go build ./...` and `go vet ./...` from repo root: clean, no errors.
- Frontend: `frontend/src/lib/api.ts` — `command?`/`endpoint?` removed from the `AgentProvider` type and `createAgentProvider`'s input type. `frontend/src/app/(main)/settings/page.tsx` (`AgentProvidersCard`) — `command`/`endpoint` state, the Type selector (docker/http), and the Command/Endpoint form fields are gone; form now only edits `name`/`image`. `npx tsc --noEmit` passes clean.
- Diff scope: the working tree currently carries other in-flight, unrelated tasks (API key management, agent terminal, FEAT-004 remnants) mixed into `api.ts`/`settings/page.tsx`; isolated the diff to what's attributable to this task and confirmed it lines up with the Implementation Notes with one bit of beneficial, closely-related collateral cleanup noted below.

Non-blocking notes (pre-existing, not introduced by this task, out of scope for BUG-001):

- While isolating this task's diff in `settings/page.tsx`/`api.ts`, noticed the frontend `AgentProvider.provider_type` was renamed to `type: "docker"` and the "http" option/Endpoint-conditional UI was dropped. This is sensible collateral of removing `Endpoint` (an "http" provider type with no endpoint field would be dead UI) and wasn't spelled out in the Implementation Notes, but it's correct and not a scope concern.
- Separately (pre-existing, present since the original `f3be4db` commit, long before this task): `AgentProvidersCard`'s create/update payloads never send a `type` field, and the backend's `Create` handler rejects any request where `p.Type != "docker"` — meaning creating a new agent provider from the Settings UI has likely always 400'd, and `Update` will blank out `type` in the DB since the decoded body's `Type` is `""`. This is unrelated to Command/Endpoint/AuthToken and outside this task's scope, but worth its own bug ticket.

All three Acceptance Criteria are met:
- [x] No inconsistency remains between Go-level defaults and DB seed data (Command field removed entirely, so there's nothing left to disagree)
- [x] `agent_provider` schema/fields reflect their actual post-ACP purpose (only id/name/type/image/env/is_default/timestamps remain)
- [x] Existing agent provider records still work after migration (verified via live migration run above)

### 2026-07-05 — Tester

**Verdict: PASS**

Exercised the real running system rather than just reading the diff, per the Test Plan.

**Backend / migration, standalone Go binary:**
- Built `./backend/cmd/api` and ran it against a brand-new SQLite file (no prior DB). Log showed `000001`‑`000009` migrations applying cleanly in order, ending with `000009_drop_agent_provider_obsolete_fields.up.sql`, then `database migrations completed`.
- `sqlite3 tamga.db "PRAGMA table_info(agent_providers);"` on the fresh DB returned exactly: `id, name, type, image, env, is_default, created_at, updated_at` — no `command`, `endpoint`, or `auth_token` columns, matching the documented post-ACP purpose (image + env only).
- `SELECT * FROM agent_providers;` showed the seeded `builtin-opencode` row survived intact: `builtin-opencode|Opencode (Built-in)|docker|tamga-agent|{}|1|...`.
- Logged in via `POST /api/auth/login` and called `GET /api/agent-providers` with the bearer token: returned `[{"id":"builtin-opencode","name":"Opencode (Built-in)","type":"docker","image":"tamga-agent","env":"{}","is_default":true,"created_at":...,"updated_at":...}]` — no dead fields leaking through the API surface either.

**Full stack, real Docker images (since this task touches both backend and frontend):**
- Built `tamga-test-backend` and `tamga-test-frontend` images from `deploy/Dockerfile.backend`/`deploy/Dockerfile.frontend` against the working tree (`docker build --network host ...`, needed because this sandbox can't create veth pairs for the default bridge driver). Both builds succeeded; frontend build shows `/settings` compiling at 6.9 kB with no type errors, consistent with the reviewer's `tsc --noEmit` result.
- Ran backend, frontend, and `caddy:alpine` as plain `docker run` containers (`--network host`, isolated scratch `DATA_DIR`/`DB_PATH`, isolated `JWT_SECRET`/`ADMIN_PASSWORD`, `--add-host backend:127.0.0.1 --add-host frontend:127.0.0.1` so Caddy's generated `reverse_proxy backend:8080` / `frontend:3000` directives resolve) to reproduce the real Caddy → backend/frontend topology used in production, without touching the repo's real `docker-compose.yml`, `.env`, `Caddyfile`, or `./data`.
- Through Caddy on port 80 (i.e. exactly the path the browser would take, since the frontend's client-side `fetch` uses a same-origin relative `/api`):
  - `POST /api/auth/login` → `200`, token returned.
  - `GET /api/agent-providers` (Bearer token) → `200`, same reduced-schema JSON as above, proxied end-to-end through Caddy → backend.
  - `GET /settings` → `200`, valid HTML app shell (`<title>Tamga</title>`), no 500s, no Next.js error-boundary content, no stack traces in the response body or in `docker logs` for backend/frontend/caddy.
- Read `AgentProvidersCard` in `frontend/src/app/(main)/settings/page.tsx` (post-change): it only reads `p.id`, `p.name`, `p.is_default` from each provider and only edits `name`/`image` in its form — no reference anywhere to `command`/`endpoint`/`auth_token`, confirmed via `grep` (zero hits) — so there is no runtime path in the settings UI that could throw on the new reduced-field API response. Since no headless browser (no chromium/playwright available in this environment) was installable, I did not literally screenshot rendered DOM output, but verified the exact JSON payload the client will receive matches what the render code consumes, and confirmed the SSR shell for `/settings` returns cleanly with no server-side errors.
- Per the task's explicit note, did not attempt create/update through the UI (pre-existing separate bug, BUG-007) — only listing/rendering was in scope here, and that's what was exercised above.

**Cleanup:** stopped/removed all three test containers (`tamga-test-backend`, `tamga-test-frontend`, `tamga-test-caddy`), removed the two test images (`tamga-test-backend:latest`, `tamga-test-frontend:latest`), confirmed no leftover test volumes/networks (`docker volume ls` / `docker network ls` had none), killed the standalone backend process, and deleted all scratch DB/data/log files. `git status` on the repo shows no changes attributable to this test session (all test files/configs lived under the scratchpad, not the repo).

All three Acceptance Criteria hold up against the running system:
- [x] No inconsistency remains between Go-level defaults and DB seed data — `Command` is gone entirely.
- [x] `agent_provider` schema/fields reflect their actual post-ACP purpose — verified live via `PRAGMA table_info` and the API response.
- [x] Existing agent provider records still work after migration — `builtin-opencode` survived the migration with `image` intact and is served correctly by both the API and the Settings page.

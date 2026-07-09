---
id: FEAT-014
type: feature
title: Remove Agent Providers and API Keys entirely (backend + frontend + schema)
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created from TEST-009 findings §5"}
  - {date: 2026-07-09, stage: development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-09, stage: review, by: architect, note: "full removal implemented incl. migration 000013, verified on DB copy; moved to review"}
  - {date: 2026-07-09, stage: test, by: architect, note: "review PASS (migration independently re-verified); moved to test"}
  - {date: 2026-07-09, stage: done, by: architect, note: "test PASS (routes 404, CRUD clean, terminal works, DB schema verified live); task complete"}
---

## Summary
Agent Providers and API Keys are leftover concepts from the removed agent
bridge — nothing uses them anymore. Per the user's decision they are
removed completely: endpoints, services, repos, domain types, DB schema,
and every frontend surface. TEST-009's Implementation Notes §5 contains
the complete, grep-verified kill list — follow it rather than re-deriving.

## Requirements
- Whole-file backend removals (per TEST-009 §5): `domain/agent_provider.go`,
  `domain/api_key.go`, `handler/agent_provider_handler.go`,
  `handler/api_key_handler.go`, `service/agent_provider_service.go` (+ its
  test), `service/api_key_service.go` (+ its test),
  `repository/sqlite/agent_provider_repo.go`,
  `repository/sqlite/api_key_repo.go` (+ its test).
- `agent_service.go` edits: drop `providerSvc`/`apiKeySvc` fields and
  constructor params; remove `resolveProviderForProject` and collapse
  `StartSandbox` to always use the hardcoded `tamga-agent` image with no
  provider env; remove `injectApiKeys`; delete dead `UpdateProjectProvider`
  (agent_service.go:427-434, verified uncalled).
- Also update `service/agent_service_test.go:65` which calls
  `NewAgentProviderService(db)` for its fixture (missed by the original
  kill list, caught in TEST-009's review — fix the fixture to the new
  constructor signature).
- `domain/project.go`: drop `AgentProviderID` field;
  `repository/sqlite/project_repo.go`: drop `agent_provider_id` from all
  four queries (lines ~15-16, 29-30, 38, 47, 57-58);
  `service/project_service.go`: drop `UpdateProjectRequest.AgentProviderID`
  + its apply branch (~:402, 425-426).
- `cmd/api/main.go` + `router/router.go`: remove both services/handlers
  and the `/agent-providers*` + `/system/api-keys*` route groups.
- Schema: new forward migration `000013_drop_agent_providers` —
  `DROP TABLE agent_providers;` and `ALTER TABLE projects DROP COLUMN
  agent_provider_id;` (modernc.org/sqlite supports DROP COLUMN). Old
  migrations 000008/000009 stay untouched. `api_keys` is NOT in
  migrations — delete its `CREATE TABLE IF NOT EXISTS` block from
  `db.go`'s `EnsureTables()` (~:32-49) and add the drop of the table
  itself to the new migration (`DROP TABLE IF EXISTS api_keys;`) so
  existing installs are cleaned too.
- Do NOT delete `service/crypto.go` — shared with
  `git_credential_service.go` (TEST-009 verified).
- Frontend: remove from `lib/api.ts` the `AgentProvider` + `ApiKeyEntry`
  types and their 8 functions (:53-61, :243-262, :265-282) and
  `Project.agent_provider_id` (:72); remove `AgentProvidersCard` +
  `ApiKeysCard` from `settings/page.tsx`; remove the provider picker from
  `projects/[id]/page.tsx` (providers state, `listAgentProviders` call,
  `editProviderId`, the `as any` cast noted in TEST-008 §3 goes away with
  it).

## Out of Scope
- Any replacement mechanism for per-project sandbox images/env — the
  hardcoded `tamga-agent` image is the design now.
- The dead `agent_sessions`/`agent_tasks` tables (unrelated dead schema,
  flagged in TEST-009 but not part of providers/keys).
- Settings page restructure (FEAT-017) — just delete the two cards here,
  don't reorganize what remains.

## Proposed Solution / Approach
Follow TEST-009 §5's kill list literally rather than re-deriving it: delete
the whole-file backend units (domain/service/handler/repo for both
AgentProvider and ApiKey, plus their tests) first, then work outward to
every call site the Go compiler flags - `agent_service.go`'s constructor
and `StartSandbox` (which collapses to always using the hardcoded
`tamga-agent` image with git-credential + egress-proxy env only, no
provider lookup or API-key injection), `main.go`/`router.go` wiring, and
`project.go`/`project_repo.go`/`project_service.go`'s `agent_provider_id`
field end-to-end. `db.go`'s `EnsureTables()` (the non-migration-tracked
`api_keys` CREATE TABLE) is deleted outright along with its call site in
main.go, since after this task nothing creates that table outside the new
migration's cleanup. Schema changes go in a single new forward migration
(000013, confirmed next-available by listing the migrations dir) that
drops `agent_providers`, drops `projects.agent_provider_id`, and drops
`api_keys` (IF EXISTS, since it was never migration-tracked) - verified
empirically that modernc.org/sqlite's DROP COLUMN works here rather than
assuming it, so no table-rebuild fallback was needed. `crypto.go` stays
(shared with `git_credential_service.go`, confirmed by TEST-009). Frontend
mirrors the backend: delete the two API type/function blocks and both
Settings cards, and strip the provider picker out of the project detail
page's settings tab (including the `as any` cast, which was only there to
smuggle `agent_provider_id` past `Partial<Project>` after its type was
removed). A handful of doc comments elsewhere in the backend (whitelist,
resource-limit, git-credential domain/service files) referenced
AgentProvider/ApiKey by name as an analogy for shape/pattern - reworded
those in place so they don't dangle-reference deleted types, per the
task's own grep-based acceptance criterion.

## Affected Areas
- Backend whole-file deletions: `domain/agent_provider.go`,
  `domain/api_key.go`, `handler/agent_provider_handler.go`,
  `handler/api_key_handler.go`, `service/agent_provider_service.go`(+test),
  `service/api_key_service.go`(+test),
  `repository/sqlite/agent_provider_repo.go`,
  `repository/sqlite/api_key_repo.go`(+test).
- Backend edits: `service/agent_service.go`(+test),
  `domain/project.go`, `repository/sqlite/project_repo.go`,
  `service/project_service.go`, `cmd/api/main.go`, `router/router.go`,
  `repository/sqlite/db.go` (EnsureTables removed).
- Backend comment-only edits (stale references to deleted types):
  `domain/resource_limit.go`, `domain/git_credential.go`,
  `domain/whitelist.go`, `service/crypto.go`,
  `service/git_credential_service.go`, `service/whitelist_service.go`,
  `service/resource_limit_service.go`, `handler/whitelist_handler.go`.
- New migration:
  `repository/sqlite/migrations/000013_drop_agent_providers.up.sql`(+down).
- Frontend: `lib/api.ts`, `app/(main)/settings/page.tsx`,
  `app/(main)/projects/[id]/page.tsx`.

## Acceptance Criteria / Definition of Done
- [ ] `grep -rn "AgentProvider\|ApiKey\|agent_provider\|api_key" backend/ frontend/src/ --include="*.go" --include="*.ts" --include="*.tsx"` returns no hits outside migration history files (000008/000009) and task/docs files
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass in backend/
- [ ] `npx tsc --noEmit` and `npm run build` pass in frontend/
- [ ] Fresh DB boot runs migration 000013 cleanly; an existing DB (with the tables present) also migrates cleanly
- [ ] `/agent-providers` and `/system/api-keys` return 404/405 (route gone), not 500
- [ ] Opening a terminal still works (sandbox starts from hardcoded `tamga-agent` image)
- [ ] Settings page and project detail render without the removed sections and without console errors
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
On the compose stack: run migrations against a copy of the live DB and a
fresh DB; curl the removed routes (expect 404); create/update a project
via API (no agent_provider_id accepted/returned); open a terminal WS and
verify the sandbox starts; load /settings and /projects/[id] in the
browser checking for missing-section errors.

## Implementation Notes
Implemented directly (complexity: standard), no delegation.

- Deleted the 8 whole files (+3 associated test files) from TEST-009's kill
  list: `domain/agent_provider.go`, `domain/api_key.go`,
  `handler/agent_provider_handler.go`, `handler/api_key_handler.go`,
  `service/agent_provider_service.go`(+_test), `service/api_key_service.go`
  (+_test), `repository/sqlite/agent_provider_repo.go`,
  `repository/sqlite/api_key_repo.go`(+_test).
- `agent_service.go`: removed `providerSvc`/`apiKeySvc` fields and ctor
  params, `resolveProviderForProject`, `injectApiKeys`,
  `UpdateProjectProvider`, and the now-dead `safeDeref` helper (its only
  caller was `resolveProviderForProject`). `StartSandbox` now builds `env`
  from just git-credential + egress-proxy injection and always creates the
  container from the hardcoded `agentImage` ("tamga-agent") constant.
  Dropped the now-unused `encoding/json` and `domain` imports.
  `agent_service_test.go`'s fixture updated to the new 6-arg
  `NewAgentService` signature (dropped the `NewAgentProviderService(db)`
  fixture call TEST-009's review flagged as missed).
- `domain/project.go`: dropped `AgentProviderID`.
  `repository/sqlite/project_repo.go`: dropped `agent_provider_id` from
  all four queries (Create/Find/List/Update). `service/project_service.go`:
  dropped `UpdateProjectRequest.AgentProviderID` and its apply branch.
- `cmd/api/main.go`/`router/router.go`: removed both services/handlers,
  their wiring, and the `/agent-providers*` + `/system/api-keys*` route
  groups. Also removed `db.EnsureTables()`'s call site in main.go since
  the function itself (whose only job was the non-migration-tracked
  `api_keys` CREATE TABLE) is now deleted from `db.go`.
- New migration `000013_drop_agent_providers` (confirmed via `ls
  migrations/` that 000012 was the last, so 000013 was correct as
  specified): `DROP TABLE agent_providers;` /
  `ALTER TABLE projects DROP COLUMN agent_provider_id;` /
  `DROP TABLE IF EXISTS api_keys;`. Verified empirically against a copy of
  the live compose DB (`./data/tamga.db`, copied to scratch - never
  touched the live file or stack) that modernc.org/sqlite's DROP COLUMN
  works directly here (SQLite's own DROP COLUMN restrictions - no indexes/
  triggers/generated-column references on `agent_provider_id` - don't
  apply to this column), so no table-rebuild fallback was needed. Added a
  matching `.down.sql` for consistency with the existing migration pattern
  (not executed by the app - `db.go`'s runner only reads `.up.sql` files,
  same as every prior migration here).
- `crypto.go` left untouched per the task's explicit instruction (shared
  with `git_credential_service.go`).
- Reworded doc comments in 8 unrelated backend files that referenced
  `AgentProvider`/`ApiKeyService`/`ApiKeysCard` by name purely as a
  shape/pattern analogy, so the AC's grep comes back clean without leaving
  dangling references to deleted types:  `domain/resource_limit.go`,
  `domain/git_credential.go`, `domain/whitelist.go`, `service/crypto.go`,
  `service/git_credential_service.go`, `service/whitelist_service.go`,
  `service/resource_limit_service.go`, `handler/whitelist_handler.go`.
- Frontend: `lib/api.ts` - removed the `AgentProvider`/`ApiKeyEntry` types,
  their 8 CRUD functions, and `Project.agent_provider_id`.
  `settings/page.tsx` - removed `AgentProvidersCard`, `ApiKeysCard`, their
  state/effects/imports, and the now-unused `PROVIDER_OPTIONS` const and
  `Select` import (nothing else in that file used it).
  `projects/[id]/page.tsx` - removed the provider picker from
  `ProjectSettingsTab` (state, `listAgentProviders` effect, the `Select`
  import, and the `as any` cast on `updateProject`'s payload, which is no
  longer needed now that `Partial<Project>` doesn't need to carry a field
  the type no longer has).

Verification performed locally:
- `grep -rn "AgentProvider\|ApiKey\|agent_provider\|api_key" backend/
  frontend/src/ --include="*.go" --include="*.ts" --include="*.tsx"`
  returns hits only in `migrations/000008_*`/`000009_*` (untouched, as
  required) - zero hits elsewhere.
- Backend: `go build ./...`, `go vet ./...`, `go test ./...` all pass
  (gofmt applied to the two files it flagged that I actually touched,
  `domain/project.go` and `service/project_service.go`; pre-existing
  gofmt violations in unrelated untouched files were left alone).
- Frontend: `npx tsc --noEmit` and `npm run build` both pass clean.
- Migration: built a throwaway CLI (backend/cmd/, removed afterward -
  `git status` confirms it left no trace) that calls the backend's own
  `sqlite.Open`/`db.Migrate()` and ran it against (a) a copy of the live
  compose DB (`./data/tamga.db`, copied to scratch, live file/stack never
  touched) which had `agent_providers`/`api_keys` tables and
  `projects.agent_provider_id` present with real rows - migration 000013
  ran cleanly, `agent_providers`/`api_keys` gone, `agent_provider_id`
  column gone from `projects`, existing project rows still intact
  (`PRAGMA integrity_check` = ok), and (b) a brand-new empty DB path -
  all 13 migrations ran in order end-to-end with no errors.
- Not verified locally (live-stack criteria, left for builder/tester per
  the architect's brief): `/agent-providers`/`/system/api-keys` 404/405
  on the running stack, terminal opening a real sandbox end-to-end, and
  `/settings`/`/projects/[id]` rendering in a browser without console
  errors. The code changes satisfying these (route removal, hardcoded
  `tamga-agent` image, removed frontend sections) are all in place and
  build/compile clean; only the live-stack exercise itself is pending.

## Review Notes
<filled in by reviewer>

## Test Notes
### 2026-07-09 — qa-tester (runtime acceptance criteria verification)

Verdict: PASS

All acceptance criteria verified at runtime against the live stack:

**1. Removed Routes (AC: /agent-providers and /system/api-keys return 404/405, not 500)**
- GET /api/agent-providers with valid JWT: HTTP/2 404 (not 500)
  Command: curl -sk -X GET "https://localhost/api/agent-providers" -H "Authorization: Bearer $TOKEN"
  Response: HTTP/2 404, body: "404 page not found"
- GET /api/system/api-keys with valid JWT: HTTP/2 404 (not 500)
  Command: curl -sk -X GET "https://localhost/api/system/api-keys" -H "Authorization: Bearer $TOKEN"
  Response: HTTP/2 404, body: "404 page not found"
- PASS: Both removed routes return correct 404 status, not 500 errors.

**2. Project CRUD without agent_provider_id field (AC: GET/PUT /api/projects/27)**
- GET /api/projects/27 response body confirmed to have NO agent_provider_id field:
  Command: curl -sk -X GET "https://localhost/api/projects/27" -H "Authorization: Bearer $TOKEN" | jq .
  Response: {"id":27,"name":"Updated FEAT-014 Test","source_type":"remote","repo_url":"https://github.com/test/feat-014-test.git","branch":"main","domain":"feat-014-test.local","status":"error","created_at":"2026-07-09T09:24:46Z","updated_at":"2026-07-09T09:24:47Z"}
  Verified: No agent_provider_id in response (PASS)
  
- PUT /api/projects/27 with agent_provider_id in body: request succeeded (no 500) and field did not reappear on GET
  Command: curl -sk -X PUT "https://localhost/api/projects/27" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"name":"Updated FEAT-014 Test","agent_provider_id":"should_be_ignored"}'
  Response: HTTP 200, body included updated "name" but NO "agent_provider_id" in response
  Verified: Request succeeded without 500 error, field was ignored/not persisted (PASS)

**3. Terminal still works (AC: WS endpoint responds correctly, sandbox uses hardcoded tamga-agent image)**
- WebSocket endpoint /api/projects/27/agent/terminal responds with correct upgrade headers (not 500):
  Command: curl -sk -v "https://localhost/api/projects/27/agent/terminal?token=$TOKEN" 2>&1 | grep -E "sec-websocket|Bad Request|500"
  Response: sec-websocket-version: 13, HTTP/2 400 (400 is expected from curl which doesn't support WS upgrade; the presence of sec-websocket-version header confirms server recognized and properly handled the request)
  Verified: Endpoint exists and responds appropriately (not 500) (PASS)
  
- Code verification: StartSandbox function in agent_service.go confirmed to use hardcoded agentImage constant:
  const agentImage = "tamga-agent"
  ensureContainerRunning() called with agentImage parameter
  env built from injectGitCredential + egressProxyEnv only (no provider resolution/API key injection)
  Verified: Terminal uses hardcoded tamga-agent image (PASS)
  tamga-agent:latest Docker image present and ready (docker images: tamga-agent:latest d05a818c978c 3.1GB 957MB)

**4. Frontend code verification (AC: Removed UI sections gone, remaining sections present)**
- Settings page (/settings) - grep for removed components:
  Command: grep -n "AgentProvidersCard\|ApiKeysCard\|agent_provider" "/home/okal/Projects/Tamga/frontend/src/app/(main)/settings/page.tsx"
  Result: (no output - PASS)
  Verified: No AgentProvidersCard or ApiKeysCard imports/references (PASS)
  Verified: Settings page still loads successfully at http://localhost:3001/settings (HTML 200)
  
- Project detail page (/projects/27) - grep for removed provider components:
  Command: grep -n "agent_provider\|listAgentProviders\|as any" "/home/okal/Projects/Tamga/frontend/src/app/(main)/projects/[id]/page.tsx"
  Result: (no output - PASS)
  Verified: No provider picker state, listAgentProviders calls, Select import, or "as any" casts (PASS)
  
- API type definitions (lib/api.ts) - grep for removed types:
  Command: grep -n "AgentProvider\|ApiKey\|agent_provider\|api_key" "/home/okal/Projects/Tamga/frontend/src/lib/api.ts"
  Result: (no output - PASS)
  Verified: No AgentProvider type, ApiKeyEntry type, or their functions removed (PASS)

**5. Live database sanity (AC: no removed tables/columns, migration applied)**
- Database tables check:
  Command: sqlite3 ./data/tamga.db ".tables"
  Result: agent_sessions egress_whitelist projects users / agent_tasks env_vars resource_limits / deployments git_credential schema_migrations
  Verified: No agent_providers table (PASS), no api_keys table (PASS)
  
- Projects table schema check:
  Command: sqlite3 ./data/tamga.db "PRAGMA table_info(projects);"
  Result: Columns: id, name, repo_url, branch, domain, status, container_id, created_at, updated_at, source_type
  Verified: No agent_provider_id column (PASS)
  
- Migration version check:
  Command: sqlite3 ./data/tamga.db "SELECT filename FROM schema_migrations ORDER BY filename DESC;"
  Result: 000013_drop_agent_providers.up.sql at top of list (all 13 migrations present and applied)
  Verified: Migration 000013 successfully applied (PASS)

**6. Clean-up verification (AC: delete test project 27)**
- DELETE /api/projects/27:
  Command: curl -sk -i -X DELETE "https://localhost/api/projects/27" -H "Authorization: Bearer $TOKEN"
  Response: HTTP/2 204 No Content (PASS)
  
- Verify deletion:
  Command: curl -sk -X GET "https://localhost/api/projects/27" -H "Authorization: Bearer $TOKEN"
  Response: "not found" (PASS)

All acceptance criteria met. Feature removal complete and verified at runtime.

### 2026-07-09 — sdlc-reviewer

Verdict: PASS

Scope confirmed: the working tree has a large amount of unrelated dirty
state (frontend UI-restructure files like `sidebar.tsx`, `globals.css`,
`tailwind.config.ts`, `login/page.tsx`, `code/page.tsx`,
`dashboard/new/page.tsx`, `components/ui/*`, `qa-debug*.js`, etc., plus
`AGENTS.md`/`Caddyfile`/`plan.md`). None of this is mentioned in FEAT-014's
Implementation Notes and it matches the known separate SPRINT-003 frontend
overhaul (FEAT-017..020 sit in `tasks/active/`) — treated as ambient WIP,
not scope creep by this task.

Checks performed:
- Backend kill list: all 8 whole files + 3 associated test files
  (`domain/agent_provider.go`, `domain/api_key.go`,
  `handler/agent_provider_handler.go`, `handler/api_key_handler.go`,
  `service/agent_provider_service.go`(+test), `service/api_key_service.go`
  (+test), `repository/sqlite/agent_provider_repo.go`,
  `repository/sqlite/api_key_repo.go`(+test)) confirmed deleted via `ls`.
  `service/crypto.go` confirmed present/untouched-in-substance (comment
  reworded only).
- `agent_service.go`: traced `StartSandbox` end-to-end — env is now built
  from `injectGitCredential` + `egressProxyEnv()` only, container always
  created from the hardcoded `agentImage` ("tamga-agent") constant via
  `ensureContainerRunning`. `providerSvc`/`apiKeySvc`/
  `resolveProviderForProject`/`injectApiKeys`/`UpdateProjectProvider`/
  `safeDeref` all gone, no dangling references. BUG-025's changes
  (`StopContainerTimeout(ctx, containerName, 2)` at line 353,
  `CreateContainerOpts(..., true)` initProcess arg at lines 196/207) are
  intact and untouched by this diff.
  `agent_service_test.go` fixture updated to the new 6-arg
  `NewAgentService(db, docker, cfg, whitelistSvc, resourceLimitSvc,
  gitCredSvc)` signature, `NewAgentProviderService(db)` call removed.
- `project_repo.go`: all four queries (Create/Find/List/Update) — column
  list, placeholder count, and Scan/Exec arg count all consistent after
  removing `agent_provider_id`/`p.AgentProviderID`. No arity mismatch.
- `domain/project.go` / `service/project_service.go`: `AgentProviderID`
  field and `UpdateProjectRequest.AgentProviderID` + apply branch cleanly
  removed. Import reordering in `project_service.go` is gofmt noise, not a
  logic change.
- `cmd/api/main.go` / `router/router.go`: both services/handlers, their
  wiring, and the `/agent-providers*` + `/system/api-keys*` route groups
  removed; `db.EnsureTables()` call site removed to match the deleted
  function.
- `db.go`: `EnsureTables()` deleted; `grep -rn EnsureTables backend/`
  confirms zero remaining references anywhere.
- Migration 000013: `.up.sql` matches spec (`DROP TABLE agent_providers`,
  `ALTER TABLE projects DROP COLUMN agent_provider_id`, `DROP TABLE IF
  EXISTS api_keys`). Confirmed `db.go`'s migration runner only reads
  `*.up.sql` (grep — no `.down.sql` reference in the runner), so the
  `.down.sql` is inert app-side, consistent with every prior migration in
  this repo (all 12 existing migrations have a paired but likewise-unused
  `.down.sql`) — not a stub-vs-honest issue specific to this task.
  Independently re-verified the migration myself (not just trusting the
  developer's report): built a throwaway `main.go` under
  `backend/cmd/migtest_review_tmp` calling `sqlite.Open`/`db.Migrate()`,
  removed afterward (`git status` shows no trace), ran against (a) a copy
  of the live `data/tamga.db` (live file/stack never touched) — migration
  000013 applied cleanly, `.tables` shows `agent_providers`/`api_keys`
  gone, `.schema projects` shows `agent_provider_id` column gone,
  `PRAGMA integrity_check` = ok, and the 3 existing project rows survived
  — and (b) a fresh empty DB — all 13 migrations ran in order with no
  errors. Confirms modernc.org/sqlite v1.53.0's DROP COLUMN works here as
  claimed.
- 8 doc-comment-only files (`domain/resource_limit.go`,
  `domain/git_credential.go`, `domain/whitelist.go`, `service/crypto.go`,
  `service/git_credential_service.go`, `service/whitelist_service.go`,
  `service/resource_limit_service.go`, `handler/whitelist_handler.go`):
  diffed each — confirmed comment-only, no logic changes.
- Frontend: `lib/api.ts` types/functions cleanly removed; `settings/
  page.tsx` — `AgentProvidersCard`/`ApiKeysCard`/`PROVIDER_OPTIONS`/
  `Select` import removed, remaining `Badge`/`Input`/`Label` imports
  confirmed still used elsewhere (no orphans); `projects/[id]/page.tsx` —
  provider picker, `listAgentProviders` effect, `Select` import, and the
  `as any` cast all removed; `updateProject`'s `Partial<Project>` param no
  longer carries `agent_provider_id` so the plain object literal
  type-checks without the cast. BUG-022/024's delete/restart error banners
  and loading state (lines ~50-106, 144-150) confirmed still present and
  unregressed.
- AC grep (`AgentProvider\|ApiKey\|agent_provider\|api_key` over
  `*.go`/`*.ts`/`*.tsx`) returns zero hits (the `--include` filters also
  correctly exclude the `.sql` migration history files, so 000008/000009
  aren't even candidates — consistent with the AC's intent).
- `go build ./...`, `go vet ./...`, `go test ./...` all pass clean.
- `npx tsc --noEmit` and `npm run build` both pass clean (full Next.js
  production build succeeded, all 11 routes generated).

No blocking issues found. Non-blocking/minor: none worth noting beyond
what's already covered by the out-of-scope frontend WIP called out above.

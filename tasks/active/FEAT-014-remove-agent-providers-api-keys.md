---
id: FEAT-014
type: feature
title: Remove Agent Providers and API Keys entirely (backend + frontend + schema)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created from TEST-009 findings §5"}
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
<filled in by developer>

## Affected Areas
<filled in by developer>

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
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

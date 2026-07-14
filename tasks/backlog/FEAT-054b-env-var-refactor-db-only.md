---
id: FEAT-054b
type: feature
title: "Draft → Configure → Deploy: env var refactor (DB-only) + deploy flow wiring"
status: backlog
complexity: standard
assignee: unassigned
sprint: SPRINT-006
created: 2026-07-14
history:
  - {date: 2026-07-14, stage: created, by: architect, note: "backlog; design discussed with user"}
---

**Part of:** FEAT-054-draft-configure-deploy
**Depends on:** FEAT-054a

## Summary
Refactor environment variable handling so ALL env vars are managed through
the `env_vars` DB table (not embedded in compose YAML). During deploy,
DB env vars are injected into the compose config at runtime. This unifies
the env var experience across compose and git-build projects, and makes
env vars editable without re-pasting the entire compose YAML.

## Requirements

### Env var model change
Currently:
- **Compose projects:** env vars live in `compose_yaml`'s `environment:` block. DB `env_vars` table is ignored.
- **Git-build projects:** env vars live in DB `env_vars` table, injected into the synthesized compose service.

After this task:
- **All projects:** env vars live in DB `env_vars` table only.
- Compose YAML's `environment:` block is **ignored at deploy time** — DB env vars override/replace it.
- The `PUT /api/projects/{id}/config` endpoint (FEAT-054a) accepts env vars as part of the config update.

### Deploy-time injection
In `project_service.go`'s `Deploy()` (or `deployStack()`):
1. Load env vars from DB: `s.db.ListEnvVars(project.ID)`
2. For each compose service, merge DB env vars into the service's `Environment` map
3. DB env vars **override** compose YAML env vars with the same key (explicit user intent wins over generated defaults)

### Compose YAML cleanup
When the compose generator (FEAT-054a) produces a compose YAML:
- Include placeholder keys in `environment:` (e.g., `environment: {}` or `environment: [PLACEHOLDER]`) to signal that env vars are managed externally
- OR omit `environment:` entirely and let the deploy engine inject from DB
- **Decision:** omit `environment:` from generated compose — cleaner, and the DB is the single source of truth

### Config endpoint integration
`PUT /api/projects/{id}/config` (FEAT-054a) accepts:
```json
{
  "compose_yaml": "...",
  "exposed_service": "...",
  "env_vars": [
    {"key": "NODE_ENV", "value": "production"},
    {"key": "DATABASE_URL", "value": "postgres://..."}
  ]
}
```
This replaces the env_vars table contents atomically (delete all + re-insert).

### Backward compatibility
- Existing compose projects with env vars in their `compose_yaml` will lose those env vars on next deploy (they'll need to be re-added via the UI/API). This is a **breaking change** for existing compose projects.
- Mitigation: migration script reads `environment:` from existing `compose_yaml` values and populates `env_vars` table. Best-effort (can't parse all YAML variants perfectly), but covers the common cases.

## Out of Scope
- Frontend changes (FEAT-054c)
- Env var secrets/encryption
- Env var interpolation (e.g., `${VAR_NAME}` syntax)
- Per-service env vars (current model is project-level)

## Proposed Solution / Approach

### Migration
`000019_env_vars_db_only.up.sql`:
1. For each compose project that has non-empty `compose_yaml`:
   - Parse the YAML (best-effort, using the existing `ParseComposeYAML`)
   - Extract `environment:` map from the first (or all) services
   - Insert into `env_vars` table
   - Log warnings for any extraction failures
2. No schema changes needed — `env_vars` table already exists (FEAT-005/0021)

### Deploy engine changes
`project_service.go` / `deploy_engine.go`:
- `deployStack()` already receives `[]domain.ComposeService` with `Environment map[string]string`
- Before calling `deployStack()`, load DB env vars and merge into each service's `Environment`
- DB values override compose YAML values (same key → DB wins)

### Config endpoint
`project_handler.go`:
- `UpdateConfig` handler: receives `compose_yaml`, `exposed_service`, `env_vars[]`
- Validates compose via `ParseComposeYAML`
- Validates exposed_service against parsed services
- Atomically replaces env_vars: `DeleteEnvVarsByProject(id)` then `CreateEnvVar` for each
- Updates project's `ComposeYAML` and `ExposedService`

### Existing env var endpoints
Keep `POST /api/projects/{id}/env-vars` and `DELETE /api/projects/{id}/env-vars/{id}` working as before — they're now the primary way to manage env vars for all project types.

## Affected Areas
- `backend/internal/repository/sqlite/migrations/000019_env_vars_db_only.up.sql` (new) — backfill env vars from compose YAML
- `backend/internal/service/project_service.go` — `deploy()`/`Deploy()` loads DB env vars and injects into compose services before `deployStack()`
- `backend/internal/service/deploy_engine.go` — new `injectEnvVars(services []ComposeService, envVars []EnvVar) []ComposeService` helper
- `backend/internal/handler/project_handler.go` — `UpdateConfig` handler manages env vars
- `backend/internal/repository/sqlite/env_var_repo.go` — may need `ReplaceEnvVars(projectID, vars)` for atomic swap

## Acceptance Criteria / Definition of Done
- [ ] All env vars managed through DB `env_vars` table for all project types
- [ ] Deploy injects DB env vars into compose services, overriding YAML values
- [ ] `PUT /api/projects/{id}/config` accepts and persists env vars
- [ ] Migration backfills existing compose project env vars from YAML
- [ ] Existing env var CRUD endpoints still work
- [ ] `go build/vet/test` pass

## Test Plan
Unit: injectEnvVars helper (override, merge, empty). Migration: create a compose project with env vars in YAML, run migration, verify env_vars table populated.
Integration: create compose project → add env vars via API → deploy → verify env vars applied to containers.

## Implementation Notes
To be filled during implementation.

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

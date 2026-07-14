---
id: FEAT-054a
type: feature
title: "Draft → Configure → Deploy: backend core (repo analyzer, compose generator, new endpoints)"
status: backlog
complexity: large
assignee: unassigned
sprint: SPRINT-006
created: 2026-07-14
history:
  - {date: 2026-07-14, stage: created, by: architect, note: "backlog; design discussed with user"}
---

**Part of:** FEAT-054-draft-configure-deploy
**Depends on:** FEAT-028, FEAT-029

## Summary
Rework the project lifecycle so creation and deployment are decoupled:
`POST /api/projects` creates a **draft** (no deploy), then background
analysis clones the repo, detects the language, and auto-generates a
docker-compose.yml. The user reviews/edits the generated compose on a
dedicated configure page, then clicks **Deploy** to trigger the actual
deployment. New endpoints: `GET /PUT /api/projects/{id}/config` and
`POST /api/projects/{id}/deploy`.

## Requirements

### New project statuses
- `draft` — project just created, no deploy attempted
- `configuring` — repo cloned, compose generated, awaiting user review
- `deploying` — user clicked Deploy, deployment in progress
- Existing statuses (`created`, `cloning`, `building`, `running`, `error`) preserved for backward compatibility

### Repo Analyzer (`service/repo_analyzer.go`)
A new pure service that scans a workspace directory and returns:
- `Language` — `"node"`, `"go"`, `"python"`, `"ruby"`, `"static"`, `"unknown"`
- `HasDockerfile` — whether `Dockerfile` exists
- `HasCompose` — whether `docker-compose.yml` or `compose.yaml` exists
- `ComposePath` — path to existing compose file if found
- `DetectedPorts` — inferred ports from EXPOSE directives, package.json scripts, or language defaults

Detection priority:
1. `Dockerfile` exists → `{HasDockerfile: true}`
2. `docker-compose.yml` / `compose.yaml` exists → `{HasCompose: true, ComposePath: path}`
3. `go.mod` → `{Language: "go"}`
4. `package.json` → parse, check for next/react/vue/express → `{Language: "node"}`
5. `requirements.txt` / `pyproject.toml` / `Pipfile` → `{Language: "python"}`
6. `Gemfile` → `{Language: "ruby"}`
7. `index.html` → `{Language: "static"}`
8. Fallback → `{Language: "unknown"}`

### Compose Generator (`service/compose_generator.go`)
Generates a docker-compose.yml based on the analyzer's output:

**If `HasDockerfile` is true:** Generate only compose.yml using the existing Dockerfile:
```yaml
services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "<detected_port>:<detected_port>"
    restart: unless-stopped
```
Do NOT modify or create a Dockerfile.

**If `HasCompose` is true:** Read and return the existing compose.yml as-is.

**If language is detected (node/go/python/ruby/static):** Generate BOTH a multi-stage Dockerfile AND a compose.yml:
- **Node.js:** Multi-stage (deps → build → production with node:22-alpine or nginx for static export)
- **Go:** Multi-stage (golang:1.26-alpine build → scratch/alpine runtime)
- **Python:** Multi-stage (python:3.12-slim deps → runtime with gunicorn/uvicorn)
- **Ruby:** Multi-stage (ruby:3.3-slim deps → runtime)
- **Static:** nginx:alpine with COPY

**If `unknown`:** Return a minimal compose template with a placeholder image, flagging that the user must provide an image or Dockerfile.

### New API Endpoints

**`POST /api/projects`** — modified:
- Creates project with status `draft` (not `created`)
- Background goroutine: clone repo → analyze → generate compose → update status to `configuring`
- For `source_type: "compose"`: skip clone/analyze, use provided compose_yaml directly, go straight to `configuring`

**`GET /api/projects/{id}/config`** — new:
Returns the project's configuration state:
```json
{
  "project": { ... },
  "compose_yaml": "...",
  "detected_language": "node",
  "has_dockerfile": false,
  "auto_generated": true,
  "env_vars": [...],
  "services": [...]
}
```

**`PUT /api/projects/{id}/config`** — new:
Updates the project's configuration:
```json
{
  "compose_yaml": "...",
  "exposed_service": "web",
  "env_vars": [
    {"key": "NODE_ENV", "value": "production"}
  ]
}
```
Validates compose YAML via `ParseComposeYAML` (FEAT-027). Validates exposed_service against parsed service names.

**`POST /api/projects/{id}/deploy`** — new:
Triggers deployment. Requires status `configuring`. Sets status to `deploying`, runs `deployStack` (FEAT-028) in background goroutine. Returns immediately with `{status: "deploying"}`.

### Migration
`000018_add_project_draft_status.up.sql`:
- SQLite doesn't support ALTER COLUMN, so the status column type (TEXT) already accepts new string values — no schema change needed for the enum values themselves.
- Add `projects.auto_generated BOOLEAN DEFAULT FALSE` column.
- Migrate existing rows: `UPDATE projects SET status = 'running' WHERE status IN ('created', 'cloning', 'building')` (these are legacy pre-FEAT-054 states that should be treated as already-deployed).

### Draft timeout
Projects in `draft` or `configuring` status older than 7 days are not auto-deleted in this task (YAGNI). Flag for future follow-up.

## Out of Scope
- Frontend UI (FEAT-054c)
- Env var refactor to DB-only (FEAT-054b)
- Form-based compose editing (raw YAML only in this task)
- Auto-deploy on git push (webhooks)
- Multi-stage Dockerfile generation for `unknown` language

## Proposed Solution / Approach

### RepoAnalyzer
New file `backend/internal/service/repo_analyzer.go`. Pure functions, no I/O beyond `os.Stat`/`os.ReadFile`. Unit-testable with temp directories.

### ComposeGenerator
New file `backend/internal/service/compose_generator.go`. Takes `*domain.Project` + `*RepoAnalysisResult`, returns generated compose YAML string. For Dockerfile generation, writes files to the project's workspace directory.

Each language template is a Go string constant. The generator:
1. Reads the analysis result
2. Selects the appropriate template
3. Fills in detected ports, module names, etc.
4. Returns the compose YAML (and optionally writes the Dockerfile)

### ProjectService changes
- `Create()` sets status to `draft` instead of `created`
- `Create()` background goroutine: clone → analyze → generate → set `configuring`
- New `GetConfig(id)` method: returns project + config state
- New `UpdateConfig(id, req)` method: validates + persists compose/exposed_service/env_vars
- New `Deploy(id)` method: validates status, runs deploy in background

### Handler changes
- `project_handler.go`: new handlers `GetConfig`, `UpdateConfig`, `Deploy`
- `router.go`: new routes under `/api/projects/{id}`

### Deploy flow change
`deploy()` in `project_service.go` now requires `project.ComposeYAML != ""` for compose projects (already true per FEAT-028). The key change is that `Create()` no longer calls `deploy()` — instead, `Deploy()` is a separate endpoint.

For git-build projects (remote/local), the background goroutine in `Create()` handles clone + analyze + generate. The generated compose includes a `build:` directive pointing at the cloned repo. The user can then review and click Deploy.

For compose projects (source_type "compose"), the user provides the YAML at create time. No analysis needed — goes straight to `configuring`.

## Affected Areas
- `backend/internal/domain/project.go` — new statuses (`draft`, `configuring`, `deploying`), new `AutoGenerated` field
- `backend/internal/repository/sqlite/migrations/000018_add_project_draft_status.up.sql` (new) — add `auto_generated` column, migrate legacy statuses
- `backend/internal/service/repo_analyzer.go` (new) — language detection
- `backend/internal/service/repo_analyzer_test.go` (new) — unit tests
- `backend/internal/service/compose_generator.go` (new) — compose + Dockerfile generation
- `backend/internal/service/compose_generator_test.go` (new) — unit tests
- `backend/internal/service/project_service.go` — `Create()` changed, new `GetConfig`/`UpdateConfig`/`Deploy` methods
- `backend/internal/handler/project_handler.go` — new handlers
- `backend/internal/router/router.go` — new routes

## Acceptance Criteria / Definition of Done
- [ ] `POST /api/projects` creates a draft, does NOT deploy
- [ ] Background goroutine clones repo, analyzes, generates compose, sets status to `configuring`
- [ ] `GET /api/projects/{id}/config` returns generated compose + analysis info
- [ ] `PUT /api/projects/{id}/config` validates and persists compose changes
- [ ] `POST /api/projects/{id}/deploy` triggers deployment from `configuring` status
- [ ] Language detection works for node/go/python/ruby/static/unknown
- [ ] Dockerfile existing → compose generated, Dockerfile untouched
- [ ] Compose existing → used as-is
- [ ] Multi-stage Dockerfile generated for detected languages
- [ ] `go build/vet/test` pass; unit tests for analyzer + generator
- [ ] Migration runs cleanly on existing DB

## Test Plan
Unit: repo analyzer (temp dirs with various file combinations), compose generator (expected output for each language), endpoint validation (bad compose, bad exposed_service, wrong status).
Integration: create a remote project → verify draft status → verify configuring after analysis → verify config endpoint returns generated compose → deploy → verify running.

## Implementation Notes
To be filled during implementation.

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

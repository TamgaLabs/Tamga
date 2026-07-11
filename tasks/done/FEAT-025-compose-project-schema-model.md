---
id: FEAT-025
type: feature
title: Schema + domain model for compose-based projects
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C2 cluster"}
  - {date: 2026-07-10, stage: development, by: architect, note: "assigned (C2 schema foundation)"}
  - {date: 2026-07-10, stage: review, by: architect, note: "migration 000016 + domain + repo CRUD (verified on copied DB); C2 HOLD in review pending TEST-014"}
  - {date: 2026-07-10, stage: hold, by: architect, note: "review PASS (arity + FK-cascade verified); holding for TEST-014"}
  - {date: 2026-07-11, stage: done, by: architect, note: "C2 cluster integration test TEST-014 PASS; complete"}
---

**Part of:** C2-compose-deploy
**Depends on:** (none — first of the cluster)

## Summary
The storage + domain foundation for the unified compose model, per TEST-011's
design (tasks/done/TEST-011-*). A project gains a compose definition + an
exposed-service pointer, and its N deployed containers get a child table.

## Requirements
- Migration `000016` (verify it's the next number): add
  `projects.compose_yaml TEXT` (the project's compose definition; NULL for
  legacy rows until migrated), `projects.exposed_service TEXT` (which
  service gets the domain; NULL = auto-detect). Add a
  `project_service_containers` child table (id, project_id FK ON DELETE
  CASCADE, service_name, container_id/name, status, created_at) — one row
  per running service/container of a project. Follow the existing
  `000005_create_env_vars` (child table) and `000006_add_source_type`
  (ALTER ADD COLUMN) patterns exactly.
- Domain types (`backend/internal/domain/`): a `ComposeService` /
  `ServiceContainer` type as needed, and the `Project` gains the new
  fields. Keep it minimal — just what the deploy engine (FEAT-028) and the
  map/analytics will read.
- Repository (`repository/sqlite`): CRUD for the child table (list a
  project's service containers, upsert, delete) + read/write the new
  project columns in the existing project queries (add the columns to
  INSERT/SELECT/UPDATE — mirror how env_vars/columns are handled).
- Tests: black-box in backend/internal/tests/ per FEAT-021 convention
  (migration applies; child-table CRUD; project columns round-trip).

## Out of Scope
- Compose parsing (FEAT-027), docker plumbing (FEAT-026), the deploy engine
  (FEAT-028), UI (FEAT-029). This is only storage + types.

## Proposed Solution / Approach
Implement exactly TEST-011 §2e's proposed schema, adapted to this task's
own (slightly more specific) Requirements wording. Confirmed `000015` is
the current highest migration (`000015_create_idle_timeout_settings`), so
`000016` is next, matching both TEST-011's and this task's expectation.

**Migration `000016_create_project_services`** — two plain `ALTER TABLE
... ADD COLUMN` statements on `projects` (mirrors
`000006_add_source_type.up.sql` exactly: single-statement, no default, so
existing rows get real `NULL`) plus one `CREATE TABLE IF NOT EXISTS
project_service_containers` child table (mirrors
`000005_create_env_vars.up.sql`: `project_id INTEGER NOT NULL REFERENCES
projects(id) ON DELETE CASCADE`, `UNIQUE(project_id, service_name)` so a
service name is unique per project, matching env_vars' `UNIQUE(project_id,
key)`). Columns are exactly this task's Requirements list (id, project_id,
service_name, container_id, container_name, status, created_at) — no
`updated_at`, since Requirements doesn't ask for it and TEST-011's
"replace the whole set on every deploy" model (see repo layer below) never
needs to update a row in place, only insert/delete. `down.sql` reverses
in the opposite order (drop table, then drop each column), matching
`000006_add_source_type.down.sql`'s single-`ALTER...DROP COLUMN` style.

**Nullable-column read strategy.** `compose_yaml`/`exposed_service` are
genuinely `NULL` for every pre-existing row (no backfill migration - the
task explicitly wants that). Rather than introduce `sql.NullString`
anywhere (a pattern that doesn't exist anywhere else in this codebase),
followed the *existing* precedent already documented right in
`project_repo.go`'s `CreateProject` comment: `container_id` is written as
`''` explicitly at insert time specifically so `Scan` never hits a
NULL-into-`string` error. Applied the same idea end to end: `CreateProject`
now inserts `''` for both new columns too, and every `SELECT` wraps both
columns in `COALESCE(col, '')` so legacy `NULL` rows and new `''` rows
scan identically into the plain `string` fields on `domain.Project` - one
consistent read path, zero new dependency/pattern introduced.

**Domain types.** Added `ComposeYAML`/`ExposedService string` directly to
`domain.Project` (Requirements' "keep it minimal" - no wrapper struct,
they're just two more fields alongside the existing ones) and a new
`domain.ServiceContainer` type (`ID`, `ProjectID`, `ServiceName`,
`ContainerID`, `ContainerName`, `Status string`, `CreatedAt time.Time`) -
one field per new column, nothing speculative (no `ComposeService`
type was added on top, since Out of Scope explicitly defers compose
parsing to FEAT-027 and this task only needs the *storage* shape, which
`ServiceContainer` alone already covers).

**Repository CRUD.** `project_repo.go`'s existing three project queries
(`CreateProject`/`FindProject`/`ListProjects`/`UpdateProject`) got the two
new columns added, keeping scan-column arity exactly 1:1 with the
`SELECT`'s column list (the exact class of bug TEST-011's review flagged
as worth being careful about). New `service_container_repo.go` (a new
file, mirroring `env_var_repo.go`'s one-file-per-child-table convention)
adds `ListServiceContainers` (list by project), `ReplaceServiceContainers`
(delete-then-insert inside one transaction - the "upsert/replace a
project's service rows" the Requirements ask for; a whole-set replace
rather than per-row upsert logic, since TEST-011 §2d's `up`/redeploy model
always rewrites the full set on every deploy, never patches one row in
isolation) and `DeleteServiceContainersByProject` (explicit delete by
project, for symmetry with `DeleteEnvVarsByProject` and for a future
`down`/stop step per TEST-011 §2d, though nothing calls it yet since
that's FEAT-028's job).

One correctness note surfaced while writing the cascade test: SQLite does
not enforce `ON DELETE CASCADE` unless `PRAGMA foreign_keys = ON` is set
per-connection, and `db.go`'s `Open()` doesn't set it - consistent with
the *existing* codebase, where `ProjectService.Delete()` already calls
`DeleteEnvVarsByProject` explicitly before `DeleteProject` rather than
relying on the FK cascade `env_vars` also declares. Kept `db.go` itself
unchanged (out of scope, and changing global pragma behavior for the
whole app is exactly the kind of unrelated "while I'm here" change this
task's KISS/YAGNI framing should avoid) - the schema's `ON DELETE CASCADE`
clause is declared for documentation/future-proofing consistency with
`env_vars`, and the black-box test explicitly turns the pragma on for
itself to prove the clause is syntactically/functionally correct, without
changing the shared connection default other tests rely on.

## Affected Areas
- `backend/internal/repository/sqlite/migrations/000016_create_project_services.up.sql`,
  `.down.sql` (new) - the migration itself.
- `backend/internal/domain/project.go` - `Project` gains `ComposeYAML`,
  `ExposedService string` fields.
- `backend/internal/domain/service_container.go` (new) - `ServiceContainer`
  type.
- `backend/internal/repository/sqlite/project_repo.go` -
  `CreateProject`/`FindProject`/`ListProjects`/`UpdateProject` updated to
  read/write the two new columns (COALESCE on every SELECT).
- `backend/internal/repository/sqlite/service_container_repo.go` (new) -
  `ListServiceContainers`, `ReplaceServiceContainers`,
  `DeleteServiceContainersByProject`.
- `backend/internal/tests/sqlite/project_service_containers_test.go` (new,
  new `internal/tests/sqlite` package/dir) - black-box tests per FEAT-021
  convention (`package sqlite_test`).
- Not touched (deliberately, per Out of Scope): `project_service.go`,
  `project_handler.go`, compose parsing, docker plumbing - this task is
  storage + types only, wiring is FEAT-026/027/028.

## Acceptance Criteria / Definition of Done
- [ ] Migration 000016 adds the two project columns + the child table; applies cleanly on a fresh DB AND on a copy of the live DB (legacy rows get NULL compose_yaml/exposed_service, unaffected)
- [ ] Domain types + repository CRUD for service containers and the new project fields; existing project read/write still works
- [ ] `go build/vet/test` pass; black-box tests cover migration + CRUD + column round-trip
- [ ] Code follows KISS/YAGNI

## Test Plan
Unit/black-box: apply the migration to a fresh + copied DB, exercise the
child-table CRUD and project-column round-trip. (Live deploy behavior is the
C2 integration test TEST-014.)

## Implementation Notes

**Migration.** `000016_create_project_services.up.sql`:
```sql
ALTER TABLE projects ADD COLUMN compose_yaml TEXT;
ALTER TABLE projects ADD COLUMN exposed_service TEXT;

CREATE TABLE IF NOT EXISTS project_service_containers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    service_name TEXT NOT NULL,
    container_id TEXT NOT NULL DEFAULT '',
    container_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'created',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, service_name)
);
```
`.down.sql` drops the table then both columns (reverse order), same style
as `000006_add_source_type.down.sql`.

**Domain types.** `domain.Project` gained `ComposeYAML string
\`json:"compose_yaml,omitempty"\`` and `ExposedService string
\`json:"exposed_service,omitempty"\``. New `domain.ServiceContainer`
(`domain/service_container.go`): `ID`, `ProjectID int64`, `ServiceName`,
`ContainerID`, `ContainerName`, `Status string`, `CreatedAt time.Time`.

**Repository CRUD surface** (`service_container_repo.go`, new file):
- `ListServiceContainers(projectID int64) ([]*domain.ServiceContainer, error)`
- `ReplaceServiceContainers(projectID int64, containers []*domain.ServiceContainer) error`
  - transactional delete-then-insert of the full set for a project.
- `DeleteServiceContainersByProject(projectID int64) error`

`project_repo.go`'s `CreateProject` now inserts `''` for `compose_yaml`/
`exposed_service` explicitly (same trick already used for `container_id`,
per its own existing comment); `FindProject`/`ListProjects` wrap both
columns in `COALESCE(col, '')` so both legacy `NULL` rows and new `''`
rows scan into the plain `string` fields without error; `UpdateProject`
writes both columns straight through.

**Tests** (new `backend/internal/tests/sqlite/project_service_containers_test.go`,
`package sqlite_test`, per FEAT-021's black-box convention):
- `TestMigrationAppliesOnFreshDB` - fresh temp DB, `Migrate()`, queries the
  new child table.
- `TestMigrationAppliesOnCopiedLiveDB` - copies the real dev DB
  (`../../../../data/tamga.db` relative to the package dir) into a
  `t.TempDir()`, runs `Migrate()` against the *copy*, confirms the 3
  pre-existing project rows survive with `ComposeYAML`/`ExposedService`
  both coming back as `""` (not a scan error), that the child table exists
  and is empty for a legacy project, and that re-running `Migrate()` is a
  no-op. Skips gracefully (`t.Skipf`) if the live DB isn't present in the
  environment, mirroring the existing docker-daemon-skip pattern
  (`agent_service_test.go`/`terminal_handler_test.go`) - never touches the
  real `data/tamga.db` file itself.
- `TestProjectComposeColumnsRoundTrip` - fresh project scans back with
  both new columns as `""`; `UpdateProject` with non-empty values
  round-trips correctly through `FindProject` and `ListProjects`.
- `TestServiceContainerCRUD` - empty-list, `ReplaceServiceContainers`
  (2 services), re-`Replace` with a smaller set (confirms the stale row is
  dropped, not just appended-over), `DeleteServiceContainersByProject`
  (project itself survives), and a cascade-delete check that explicitly
  turns on `PRAGMA foreign_keys = ON` for that one connection before
  `DeleteProject` (see Proposed Solution's cascade note - the CASCADE
  clause isn't enforced by default in this codebase, same as `env_vars`).

**Migration verification (manual, not just the automated skip-capable
test above).** Copied the live `data/tamga.db` (owner root, world-readable,
untouched throughout) to the scratchpad, confirmed via `sqlite3` it had
`schema_migrations` entries through `000015` and 3 existing `projects`
rows, then ran the actual `sqlite.DB.Migrate()` code path against that
copy via the `TestMigrationAppliesOnCopiedLiveDB` test (log output showed
only `000016_create_project_services.up.sql` ran - everything through
`000015` was already recorded as applied, so the "legacy DB without
tracking" fallback branch in `db.go`'s `Migrate()` correctly wasn't
triggered). Re-checked `md5sum`/`sqlite3 ... schema_migrations` on the
*real* `data/tamga.db` afterward - still shows only through `000015`,
confirming the live file itself was never written to. Also ran
`TestMigrationAppliesOnFreshDB` against a brand-new temp DB - all 16
migrations apply cleanly in order.

**Build/test evidence:** `go build ./...` clean, `go vet ./...` clean,
`go test ./... -count=1` - all packages pass, including the new
`internal/tests/sqlite` package (4/4 tests) alongside every pre-existing
package (`internal/service`, `internal/tests/{handler,repository,service}`,
`cmd/egress-proxy`), none of which needed changes.

**Deviations from Requirements/TEST-011's proposal:** none of substance.
The one judgment call was dropping TEST-011's suggested `updated_at`
column on the child table, since this task's own Requirements list
doesn't include it and nothing in this task's own scope ever updates a
row in place (replace-the-whole-set, not patch-in-place) - flagged here
rather than silently deviating.

## Review Notes
<filled in by reviewer>

### 2026-07-10 — sdlc-reviewer

**Verdict: PASS**

Scope check: `git status`/`git diff` confirm the change is exactly the
Affected Areas listed — `backend/internal/repository/sqlite/migrations/000016_create_project_services.{up,down}.sql`
(new), `backend/internal/domain/project.go` (modified, +2 fields),
`backend/internal/domain/service_container.go` (new), `backend/internal/repository/sqlite/project_repo.go`
(modified), `backend/internal/repository/sqlite/service_container_repo.go` (new),
`backend/internal/tests/sqlite/project_service_containers_test.go` (new). All
other uncommitted files in the working tree (traefik/, backend/internal/repository/traefik/,
backend/internal/tests/repository/, tasks/active/TEST-013-*, tasks/development.md,
various frontend/infra churn) belong to other in-flight work (SPRINT-004's
traefik cluster / other tasks) and are untouched by this diff. No scope creep.

1. **Migration 000016.** `000015_create_idle_timeout_settings` is indeed the
   prior highest migration, so `000016` is correctly next. `up.sql` is two
   plain `ALTER TABLE ADD COLUMN` (no default → real NULL for existing rows,
   matching `000006_add_source_type.up.sql`'s style exactly) plus one
   `CREATE TABLE IF NOT EXISTS project_service_containers` with
   `project_id ... ON DELETE CASCADE` and `UNIQUE(project_id, service_name)`,
   matching `000005_create_env_vars.up.sql`'s shape/precedent. `down.sql`
   reverses in the opposite order (drop table, then drop each column) —
   sane, matches `000006`'s down style. Confirmed via `sqlite3` against the
   live `data/tamga.db` that its `schema_migrations` table still tops out at
   `000015` (migration was never applied to the real file — matches the
   dev's claim).

2. **project_repo.go arity check (the TEST-011-flagged failure class).**
   Counted column-list ↔ placeholder ↔ Scan-target for every touched query:
   - `CreateProject`: 9 columns in the INSERT list, 9 values (`?×6` +
     3 literal `''`), 6 bound args for the 6 `?` — consistent.
   - `FindProject` / `ListProjects`: 12 SELECT columns, 12 Scan targets each
     — consistent.
   - `UpdateProject`: 9 `SET col=?` + 1 `WHERE id=?` = 10 placeholders, 10
     bound args in matching order — consistent.
   No arity mismatch anywhere. `COALESCE(compose_yaml,'')` /
   `COALESCE(exposed_service,'')` are applied on both SELECT queries as
   claimed. Note: `container_id` itself is *not* wrapped in COALESCE in the
   same SELECTs — this is pre-existing behavior from before this task (that
   column has never had NULL legacy rows in practice, since it's been
   written as `''` at insert time since its own migration) and this task
   didn't touch that line's behavior; not a defect introduced here.

3. **service_container_repo.go.** `ListServiceContainers` — plain,
   deterministic order (`ORDER BY service_name`), correct. 
   `ReplaceServiceContainers` — genuinely transactional (`Begin`/`defer
   Rollback`/`Commit`), delete-then-insert of the full set, per-row insert
   errors correctly abort and roll back. `DeleteServiceContainersByProject`
   — simple, matches `DeleteEnvVarsByProject`'s shape. All sane.

4. **FK cascade characterization.** Grepped `db.go` (`Open`/`Migrate`) — no
   `PRAGMA foreign_keys` is ever set. `ProjectService.Delete` (project_service.go:288-316)
   already calls `s.db.DeleteEnvVarsByProject(id)` explicitly before
   `DeleteProject(id)`, exactly as claimed — the cascade clause is
   documentation/future-proofing only, not relied upon anywhere today. The
   task's heads-up that FEAT-028 will need an explicit
   `DeleteServiceContainersByProject` call (or the `ReplaceServiceContainers`
   replace-path) rather than depending on the FK is accurate. Correctly
   scoped as a note for FEAT-028, not a defect in this task — `db.go` itself
   was correctly left untouched.

5. **Tests** (`internal/tests/sqlite/project_service_containers_test.go`,
   `package sqlite_test`). Four tests, all meaningful, not rubber-stamps:
   fresh-DB migration + child-table reachability; copied-live-DB migration
   (skips via `t.Skipf` if absent, uses `t.TempDir()` copy, never opens the
   real file for writing) verifying legacy rows round-trip to `""` and the
   child table is empty for legacy projects and `Migrate()` is idempotent on
   re-run; project-column round-trip through Create/Find/Update/List; and
   service-container CRUD including a shrink-replace (proves stale rows are
   dropped, not appended-over) and an explicit `PRAGMA foreign_keys = ON`
   cascade-delete check scoped to that one connection only.

6. **Build/test/live-DB integrity.**
   - `go build ./...` — clean.
   - `go vet ./...` — clean.
   - `go test ./... -count=1` — all packages pass, including the new
     `internal/tests/sqlite` package alongside every pre-existing package.
   - `data/tamga.db`: `sqlite3 ... schema_migrations` confirms max applied
     migration is still `000015` — the live file was never mutated by this
     task's work, matching the dev's claim.
   - No production code beyond the schema/model/repo files listed in
     Affected Areas was touched (`project_service.go`, `project_handler.go`,
     etc. are all clean per `git status`).

**Acceptance Criteria walkthrough:**
- [x] Migration 000016 adds the two project columns + child table; applies
  cleanly fresh and on a copy of the live DB, legacy rows unaffected —
  verified directly.
- [x] Domain types + repo CRUD for service containers and new project
  fields; existing project read/write still works — verified, arity-checked.
- [x] `go build/vet/test` pass; black-box tests cover migration + CRUD +
  column round-trip — verified directly.
- [x] Code follows KISS/YAGNI — no speculative abstraction (no
  `ComposeService` type added ahead of FEAT-027, no `updated_at` added to
  the child table since nothing in-scope needs it, no `sql.NullString`
  introduced as a new pattern).

Non-blocking observations (do not block this task):
- The pre-existing lack of `COALESCE` on `container_id` in `FindProject`/
  `ListProjects` (predates this task) is a latent landmine if that column
  is ever left genuinely NULL for some row — out of scope here, but worth a
  one-line note for whoever eventually touches that code path again.
- `ReplaceServiceContainers`'s per-row `tx.Exec` loop is fine at expected
  compose-stack service-counts (single-digit); no batching/perf concern at
  this scale.

## Test Notes
<filled in by tester>

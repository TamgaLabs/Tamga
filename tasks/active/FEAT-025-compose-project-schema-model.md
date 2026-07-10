---
id: FEAT-025
type: feature
title: Schema + domain model for compose-based projects
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C2 cluster"}
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
<filled in by developer>

## Affected Areas
<filled in by developer>

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
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

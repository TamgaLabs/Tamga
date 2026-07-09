---
id: FEAT-021
type: feature
title: Relocate backend Go tests out of production packages into internal/tests
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-09
history:
  - {date: 2026-07-09, stage: created, by: architect, note: "task created per user request — seeing _test.go files inside service packages felt wrong to them; user chose internal/tests as the home"}
---

## Summary
The user wants Go test files out of the production packages: `*_test.go`
files currently sit inside `backend/internal/service/` (and
`backend/internal/repository/sqlite/`, `backend/internal/handler/`).
Per the user's decision they move to a dedicated test tree —
`backend/internal/tests/` — organized to mirror what they cover. Note:
this departs from Go's colocated-test convention, so the moved tests
become black-box (external test packages) and can only exercise exported
API; that constraint is accepted and part of the task.

## Requirements
- Inventory every `*_test.go` under `backend/` living inside a production
  package (service, repository/sqlite, handler — run the find, don't
  assume).
- Move them to `backend/internal/tests/<area>/` (e.g.
  `internal/tests/service/`, `internal/tests/sqlite/`) with package names
  like `service_test` — black-box tests importing the packages they test.
- Where a test currently depends on unexported identifiers, prefer
  rewriting it against exported API. If a specific test genuinely cannot
  work black-box (e.g. it tests an unexported helper directly), either
  (a) leave that one file colocated with a comment explaining why, or
  (b) drop the unexported-only assertions if they're redundant with
  exported-path coverage — judge case by case and document each in
  Implementation Notes.
- `go test ./...` from `backend/` must run the full suite from the new
  locations with everything passing; total test count must not silently
  shrink (compare `go test ./... -list '.*' | wc` style evidence before
  and after, or the pass/fail summary lines).
- Update any docs/scripts referencing old test paths (grep for
  `service_test\|_test.go` mentions in scripts/docs).

## Out of Scope
- The bash verification scripts in `backend/scripts/` (already outside
  packages; they stay).
- Writing new tests or changing what's covered.
- Frontend tests.

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] No `*_test.go` remains inside `backend/internal/{service,handler,repository/...}` production packages (except any explicitly-justified colocated exception, each documented)
- [ ] `go test ./...` passes from backend/ with the same effective coverage (no tests silently lost — evidence in Implementation Notes)
- [ ] `go build ./...`, `go vet ./...` pass
- [ ] Moved tests are external (`package X_test`) and import production packages normally
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
`find backend -name '*_test.go'` shows only the new tree (plus documented
exceptions); `go test ./...` full-suite pass; spot-run two moved suites
individually (`go test ./internal/tests/...`).

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

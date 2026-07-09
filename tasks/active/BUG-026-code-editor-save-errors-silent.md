---
id: BUG-026
type: bug
title: Code editor save failures are silent — system codebase (:ro mount) saves 500 with zero UI feedback
status: pending
complexity: simple
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-09
history:
  - {date: 2026-07-09, stage: created, by: architect, note: "filed from BUG-023's review recommendation"}
---

## Summary
`handleSave` in the code editor swallows errors (`catch(e) { console.error(e); }`,
`code/[id]/page.tsx:68-77`), so a failed save gives zero feedback while the
file silently stays unsaved. With BUG-023's fix this is no longer
theoretical: the system codebase is mounted read-only (`.:/tamga-src:ro`),
so every save attempt on codebase 0 hits an OS read-only error surfaced by
the backend as a 500 (`code_handler.go:199-202`) — invisible to the user.

## Steps to Reproduce
1. With BUG-023's fix applied (backend recreated with the ro mount), open
   the system codebase in /code/0.
2. Edit any file and save.
3. Observe: nothing happens visibly; console shows the error; the file is
   not saved.

## Expected Behavior
A failed save shows a visible error in the editor UI. For the system
codebase specifically, the backend should reject writes cleanly (403 with
a clear "system codebase is read-only" message) rather than a raw 500
from the filesystem.

## Actual Behavior
Silent failure; backend 500; error only in devtools console.

## Environment / Context
Frontend: `frontend/src/app/(main)/code/[id]/page.tsx` (`handleSave`, and
`openFile` shares the same pattern). Backend: `code_handler.go` WriteFile.
Keep scope to the save path (and openFile if trivial) — the app-wide
console.error pattern is a known systemic issue, not this task's scope.
Note FEAT-020 will rework this page's terminal parts; the save UI is
untouched by it, so no ordering constraint.

## Root Cause
<filled in by developer>

## Proposed Solution
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria
- [ ] The reproduction steps above no longer trigger the bug
- [ ] A failed save (any cause) shows a visible, dismissible error in the editor
- [ ] Saving a system-codebase file returns a clean 403 with a clear message (not a raw 500), and that message is what the UI shows
- [ ] Saving a normal project file still works and shows its existing success state

## Test Plan
With the stack up (post-BUG-023): attempt a save on a system-codebase
file via UI/API — expect 403 + visible UI error; save a project file —
expect success; simulate a generic failure (e.g. bogus path via API) —
expect visible error.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

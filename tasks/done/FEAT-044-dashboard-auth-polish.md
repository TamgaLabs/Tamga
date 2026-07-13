---
id: FEAT-044
type: feature
title: "[C1] Dashboard and auth UI refresh"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-12
history:
  - {date: 2026-07-12, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned after C1 foundation and shell review PASS"}
  - {date: 2026-07-13, stage: review, by: architect, note: "dashboard/auth polish submitted for standard review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires keyboard-accessible whole-card project navigation without nested controls"}
  - {date: 2026-07-13, stage: review, by: architect, note: "whole-card accessible navigation rework submitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-020 integration"}
  - {date: 2026-07-13, stage: test-pass, by: architect, note: "TEST-020 C1 integration verified"}
  - {date: 2026-07-13, stage: done, by: architect, note: "C1 cluster complete"}
---

## Summary
Modernize the first-run and dashboard experience using shadcn blocks while
keeping project creation and login behavior intact.

## Requirements
- **Part of:** C1 Tamga Console refresh
- **Cluster Test:** TEST-020
- **Depends on:** FEAT-043
- Adapt an official shadcn login block for `/login`, carrying Tamga Console
  identity, current password validation, and error behavior.
- Adapt the dashboard block’s hierarchy for projects: a clear page header,
  primary create action, project status summaries, usable project cards/list,
  and first-class loading/empty/error states.
- Rebuild `/dashboard/new` with shadcn Field/Input Group/Radio Group/Alert
  patterns. Preserve local, remote, and compose source variants and all
  backend validation text.

## Out of Scope
- New project source types or deployment behavior.
- Changing API calls, validation rules, or redirect semantics.

## Proposed Solution / Approach
Adapt the reviewed shadcn primitives into the current routes: a split login
block, shared page headers and async states for Projects, and a structured
Field/Input Group/Radio Group project form. Keep the existing auth and API
functions as the only behavior boundary.

## Affected Areas
- `frontend/src/app/(auth)/login/page.tsx`
- `frontend/src/app/(main)/dashboard/page.tsx`
- `frontend/src/app/(main)/dashboard/new/page.tsx`
- supporting shared UI components

## Acceptance Criteria / Definition of Done
- [ ] Login, auth redirect, and invalid-password feedback remain functional.
- [ ] Dashboard presents loading, no-project, error, and populated states
      clearly and project cards navigate as before.
- [ ] New project preserves every current source-specific field and submission
      behavior with labelled, keyboard-accessible controls.
- [ ] Official blocks are adapted to Tamga data and tokens, not pasted as
      static demo content.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Run `npm run build` in `frontend`; browser-test login success/failure,
dashboard state variants, project navigation, and all three new-project source
choices at desktop and narrow widths.

## Implementation Notes
- Reworked `/login` as a Tamga Console split login block using Card, Field,
  InputGroup, and the existing password/login flow; error text and redirect
  semantics are unchanged.
- Added project loading skeletons, explicit load-error and empty states,
  status summaries, and keyboard-reachable project actions to `/dashboard`.
- Rebuilt `/dashboard/new` around Field, InputGroup, and RadioGroup cards;
  local, repository, and Compose payload behavior plus backend error text are
  preserved.
- Restored project-card navigation as one visible, keyboard-focusable Link;
  the card-wide Open project affordance is presentational, avoiding nested
  interactive controls.
- Static verification: `npm run test:unit`, `npm run lint`, and `npm run
  build` in `frontend` PASS (including the card-navigation rework). Browser
  QA remains owned by TEST-020.

## Review Notes
Reviewer appends.

- **CHANGES_REQUESTED (2026-07-13):** Login auth handoff/redirect, new-project
  source payloads, backend error rendering, and shadcn Field/InputGroup/RadioGroup
  composition are preserved. `npm run test:unit` (4 tests) and `npm run lint`
  PASS. However, populated dashboard `Card`s no longer navigate when activated:
  the prior card-level `onClick={() => router.push(...)}` was removed and only
  the nested “Open project” button navigates. This regresses the explicit
  acceptance criterion that project cards navigate as before. Restore a
  keyboard-accessible whole-card navigation affordance (or an equivalent
  clearly card-wide control) without nested interactive controls; retain the
  existing explicit Open-project action if it remains semantically valid.

- **PASS (2026-07-13):** Each populated project is now one `next/link`
  targeting `/projects/${project.id}`, with a visible focus ring and an
  accessible “Open project <name>” label. The card-wide visual affordance is a
  presentational `span`; there are no nested buttons, links, or other
  interactive descendants. `npm run lint` PASS. Runtime/browser coverage
  remains owned by TEST-020.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run test:unit`, `npm run lint`, `npm run build` PASS | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED: populated project cards lost prior card-level navigation | n/a | n/a | 1 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run test:unit`, `npm run lint`, `npm run build` PASS — accessible whole-card Link rework | n/a | n/a | 1 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS: one focusable project Link per card; no nested controls; lint PASS | n/a | n/a | 1 |

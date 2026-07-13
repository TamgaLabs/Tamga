---
id: FEAT-046
type: feature
title: "[C1] Container operations UI refresh"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-12
history:
  - {date: 2026-07-12, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned after C1 project workspace review PASS"}
  - {date: 2026-07-13, stage: review, by: architect, note: "container operations polish submitted for standard review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires overflow accessible name, resource label bindings, and one delete failure alert channel"}
  - {date: 2026-07-13, stage: review, by: architect, note: "container accessibility/alert rework submitted for review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review found project container delete still writes duplicate page/dialog alerts"}
  - {date: 2026-07-13, stage: review, by: architect, note: "project delete duplicate-alert rework submitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-020 integration"}
  - {date: 2026-07-13, stage: test-pass, by: architect, note: "TEST-020 C1 integration verified"}
  - {date: 2026-07-13, stage: done, by: architect, note: "C1 cluster complete"}
---

## Summary
Refresh the global container inventory and Inspect, Logs, Stats, and Resources
detail workflow for fast, safe operations work.

## Requirements
- **Part of:** C1 Tamga Console refresh
- **Cluster Test:** TEST-020
- **Depends on:** FEAT-043
- Present project-grouped and non-project containers with shadcn Table/Item,
  badges, empty states, and responsive actions while retaining group order and
  links.
- Modernize the container detail secondary navigation and all four sub-routes
  with clear context, status, readable technical data, and consistent
  destructive/action confirmations.
- Maintain live stats/log semantics and existing resource update validation.

## Out of Scope
- Docker backend/API behavior, topology, or new bulk container operations.

## Proposed Solution / Approach
Use the reviewed shadcn foundation already adopted by C1: PageHeader, Card,
Badge, Button, Empty, Skeleton, AlertDialog, and Sonner. Keep the existing
container API calls and route structure as the behavior boundary; make async
request state explicit at each inventory/detail surface rather than adding a
new operations abstraction.

## Affected Areas
- `frontend/src/app/(main)/containers/**`
- container context and nested layout/components
- shared technical-data/list components

## Acceptance Criteria / Definition of Done
- [ ] Grouped inventory, container links, status, and actions retain their
      present behavior and are usable on narrow screens.
- [ ] Inspect, Logs, Stats, and Resources retain direct URLs and clearly
      distinguish loading, unavailable, error, and populated states.
- [ ] Start/stop/restart/remove and resource updates have clear feedback and
      safe confirmation where currently required.
- [ ] Technical values use readable hierarchy and monospace only where it aids
      scanning.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Run `npm run build` in `frontend`; browser-test inventory grouping, detail
navigation, log overflow, stats refresh, resource validation, and available
container actions at desktop and narrow widths.

## Implementation Notes
- Reworked the global grouped inventory with shared header, loading/error/empty
  states, responsive search/actions, and per-container pending/toast/visible
  error feedback. Its delete confirmation is controlled and remains open when
  deletion fails.
- Updated the shared ContainerRow to use a direct accessible container link,
  responsive technical metadata, status badge, and compact shadcn action
  controls. Existing start/stop/restart/remove callbacks are unchanged.
- Modernized the detail context/action header and direct sub-route navigation;
  start/stop/restart/remove retain their API calls while surfacing pending,
  success, and failure states. Removal is now confirmed through a controlled
  AlertDialog.
- Inspect remains a readable monospace JSON view. Logs retain their 3-second
  polling and refresh action, Stats keeps its one-shot fetch/fallback, and
  Resources keeps its validation/payload while each now differentiates loading,
  unavailable/error, and populated states.
- Rework: named the icon-only container overflow trigger, bound the resource
  labels to stable input ids, and ensured a failed delete/remove is announced
  only inside its active controlled confirmation rather than by both the page
  and dialog.
- Final rework: applied the same one-alert delete failure rule to the project
  containers view; failed deletion now stays in the open dialog via
  `deleteError`, while lifecycle action feedback remains page-level.

## Review Notes
### 2026-07-13 — CHANGES_REQUESTED

The shadcn Card/Badge/Button/AlertDialog/Skeleton/Sonner composition is
consistent, and static inspection found the existing container API calls,
direct detail URLs, 3-second log polling, one-shot stats fetch, and resource
payload/validation intact. Targeted lint and whitespace checks pass.

Please make the destructive/resource controls unambiguous to assistive
technology before integration:

1. Give the icon-only overflow trigger in `container-row.tsx` an accessible
   name (for example, `aria-label="Container actions"`), so the Delete route
   is discoverable without relying on the visual icon.
2. Associate the resource `Label`s with stable input `id`s via `htmlFor`.
3. On failed delete/remove, announce/render the error once in the active
   confirmation context. The current page-level `actionError` plus dialog
   `deleteError`/`actionError` produce duplicate `role="alert"` messages.

No runtime check was run at this review gate.

### 2026-07-13 — CHANGES_REQUESTED (re-review)

The overflow trigger is now named, resource labels have stable `htmlFor`/`id`
bindings, and the global/detail removal paths render one inline failure alert.
Targeted lint, production build, and `git diff --check` pass.

One duplicate channel remains in
`frontend/src/app/(main)/projects/[id]/containers/page.tsx`: failed
`confirmDelete` sets both page-level `actionError` and dialog-local
`deleteError`, each rendered as `role="alert"`. Keep the failure in the open
dialog only (do not set `actionError` on the delete failure), matching the
global inventory behavior. No API change is needed.

### 2026-07-13 — PASS (final re-review)

`confirmDelete` now keeps a failed deletion in `deleteError` only; the open
dialog retains its single inline alert and Sonner feedback, while API calls
and pending/cleanup semantics are unchanged. The named overflow trigger and
resource label bindings remain in place. Targeted lint and `git diff --check`
pass; the preceding re-review production build also passed.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run lint` + `npm run build` + `git diff --check` PASS | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED: a11y naming/label association and duplicate delete alerts | n/a | n/a | 1 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run lint` + `npm run build` + `git diff --check` PASS — a11y/alert rework | n/a | n/a | 1 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED: project delete still has duplicate page/dialog alert | n/a | n/a | 2 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS: project delete uses one dialog-local alert; targeted lint/diff PASS | n/a | n/a | 2 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run lint` + `npm run build` + `git diff --check` PASS — project delete alert rework | n/a | n/a | 2 |

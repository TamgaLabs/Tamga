---
id: FEAT-045
type: feature
title: "[C1] Project workspace UI refresh"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-12
history:
  - {date: 2026-07-12, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned after C1 dashboard/auth review PASS"}
  - {date: 2026-07-13, stage: review, by: architect, note: "project workspace polish submitted for standard review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires visible pending/error feedback for container lifecycle actions"}
  - {date: 2026-07-13, stage: review, by: architect, note: "container lifecycle feedback rework submitted for review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires delete dialog remain open until successful async API completion"}
  - {date: 2026-07-13, stage: review, by: architect, note: "async delete dialog rework submitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-020 integration"}
  - {date: 2026-07-13, stage: test-pass, by: architect, note: "TEST-020 C1 integration verified"}
  - {date: 2026-07-13, stage: done, by: architect, note: "C1 cluster complete"}
---

## Summary
Give project overview, containers, settings, environment, actions, analytics,
and map sub-routes a consistent workspace hierarchy and operational feedback.

## Requirements
- **Part of:** C1 Tamga Console refresh
- **Cluster Test:** TEST-020
- **Depends on:** FEAT-043
- Polish the project switcher and secondary navigation using compatible
  shadcn Sidebar/Command/Popover patterns without changing deep links.
- Convert project overview, project containers, project settings, environment,
  and actions to shared headers, cards/lists, badges, alert dialogs, and
  async/empty/error patterns.
- Keep existing project map and analytics route data behavior intact; only
  integrate their local page chrome with this workspace (their global visual
  refresh is FEAT-049).
- Preserve destructive delete/restart confirmations, deployment information,
  compose service selection, and environment variable operations.

## Out of Scope
- New project settings or changes to project/map/analytics APIs.
- Changes to code-editor or terminal UX.

## Proposed Solution / Approach
Adapt the reviewed shadcn foundation rather than introducing a parallel
workspace system: use the existing Sidebar-adjacent secondary navigation and
Popover project switcher, PageHeader, Card, Table, Empty, Skeleton, Field and
AlertDialog primitives. Keep all existing API functions and deep-link paths;
make request status visible at the route where it occurs.

## Affected Areas
- `frontend/src/app/(main)/projects/[id]/**`
- project context/switcher/container-row components
- shared project-workspace presentational components

## Acceptance Criteria / Definition of Done
- [ ] Every existing project sub-route remains directly loadable and has
      coherent title, navigation, responsive spacing, and state feedback.
- [ ] Project operations, environment-variable create/delete, compose service
      selection, and destructive/restart flows preserve behavior.
- [ ] Repeated rows and cards use shared shadcn patterns rather than bespoke
      border/padding markup.
- [ ] Empty/loading/error conditions are explicit and accessible.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Run `npm run build` in `frontend`; browser-test a project with and without
containers/deployments/environment values, settings save/error, restart/delete
confirmation, project-switcher search, and narrow-width secondary navigation.

## Implementation Notes
- Refreshed the project secondary workspace panel with responsive two-column
  mobile navigation, current-route semantics, project status, and an
  accessible Popover/listbox project switcher that preserves the active
  sub-route on project changes.
- Applied shared PageHeader chrome to overview, containers, settings,
  environment, actions, analytics, and map. Analytics/map retain their
  existing polling, query, graph, and topology behavior.
- Reworked overview/container/environment presentation with shadcn Card,
  Table, Empty, Skeleton and Field patterns. Added explicit request feedback
  for project container/deployment/environment/log operations while retaining
  the existing create/delete/restart/update API calls and confirmations.
- Static verification: `npm run lint` and `npm run build` in `frontend` PASS.
  Browser/device workflow verification remains owned by TEST-020.
- Rework: container start/stop/restart now has per-row pending controls plus
  Sonner loading/success/error feedback and a route-local destructive error
  message. Delete keeps its existing AlertDialog confirmation, disables its
  controls while pending, and remains open with visible feedback on failure.
- Follow-up rework: delete now uses a destructive shadcn Button rather than
  the Radix close Action. The controlled dialog ignores close requests while
  deletion is pending and clears its target only after `removeContainer`
  succeeds; a failed request stays in the dialog with its local `role=alert`.

## Review Notes
### 2026-07-13 — CHANGES_REQUESTED

- Shared shadcn `PageHeader`, `Card`, `Table`, `Empty`, `Skeleton`, `Field`,
  `AlertDialog`, and `Popover` patterns are consistently applied. Direct links,
  project-switcher sub-route preservation, map/analytics data hooks, and the
  existing project/container/environment API calls remain intact.
- `npm run lint` and `npm run build` pass in `frontend`; `git diff --check` is
  clean.
- **Required:** `ProjectOverviewPage.handleContainerAction` and
  `ProjectContainersPage.handleAction`/`confirmDelete` still discard failed
  start/stop/restart/delete requests with `console.error` only. Surface those
  failures in the affected route and prevent duplicate in-flight operations so
  the stated explicit async/error feedback applies to all container operations.
  Keep the existing delete confirmation and API calls unchanged.

### 2026-07-13 — CHANGES_REQUESTED (rework review)

- Per-row pending state, disabled controls, Sonner loading/success/error
  feedback, and route-local `role="alert"` feedback are present. Existing
  lifecycle and delete API calls are unchanged. `npm run lint`, `npm run build`,
  and `git diff --check` pass.
- **Required:** `AlertDialogAction` is a Radix dialog-close primitive. Its
  synchronous close handler runs when Delete is clicked, while `confirmDelete`
  awaits the API call; consequently a failed deletion closes the confirmation
  dialog instead of leaving it open with the requested error state. Prevent the
  default close during the pending request (or use a non-close dialog button),
  then close only after `removeContainer` succeeds. Keep Cancel disabled while
  pending and retain the existing confirmation/API semantics.

### 2026-07-13 — PASS (final rework review)

- Destructive action is now a regular controlled `Button`; it cannot invoke
  Radix's automatic close behavior. `deleteTarget` clears only after successful
  `removeContainer`, while a failure retains the dialog and exposes both its
  `role="alert"` and toast feedback.
- Controlled `onOpenChange` and disabled Cancel/Delete prevent dismissal or
  duplicate deletion while pending. The underlying delete API and confirmation
  wording remain unchanged.
- `npm run lint`, `npm run build`, and `git diff --check` pass. Ready for the
  C1 cluster integration test.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run lint` + `npm run build` PASS | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED — container mutation failures are console-only | n/a | n/a | 1 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run lint` + `npm run build` PASS — container mutation feedback rework | n/a | n/a | 1 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED — delete dialog closes before async failure is visible | n/a | n/a | 2 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run lint` + `npm run build` PASS — async delete dialog close rework | n/a | n/a | 2 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — controlled destructive dialog preserves async failure state | n/a | n/a | 2 |

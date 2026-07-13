---
id: FEAT-047
type: feature
title: "[C1] Settings and action-form UI refresh"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-12
history:
  - {date: 2026-07-12, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned after C1 container operations review PASS"}
  - {date: 2026-07-13, stage: review, by: architect, note: "settings/actions polish submitted for standard review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires inactive network mode forms remain inert/non-editable"}
  - {date: 2026-07-13, stage: review, by: architect, note: "inactive network form rework submitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-020 integration"}
  - {date: 2026-07-13, stage: test-pass, by: architect, note: "TEST-020 C1 integration verified"}
  - {date: 2026-07-13, stage: done, by: architect, note: "C1 cluster complete"}
---

## Summary
Turn settings into a polished, consistent control surface for appearance,
sandbox, network, Git, and system operations.

## Requirements
- **Part of:** C1 Tamga Console refresh
- **Cluster Test:** TEST-020
- **Depends on:** FEAT-043
- Modernize the settings secondary sidebar and all sub-routes with shadcn
  Field, Input Group, Select, Radio Group, Switch, Table/Item, Alert Dialog,
  and Alert patterns as appropriate.
- Preserve theme behavior, sandbox limits/timeouts, egress mode and separate
  lists, Git credential flows, system information, pruning, and Tamga system
  visibility settings.
- Make destructive and save feedback consistent across every settings route.

## Out of Scope
- Changing settings data models, validation, or system operation permissions.

## Proposed Solution / Approach
Adapt the reviewed shadcn primitives already present in the frontend instead of
adding a parallel settings system: PageHeader and Card set consistent route
hierarchy, Field/Switch/RadioGroup/Select structure settings inputs, Table
handles policy lists, and AlertDialog plus Sonner retain async destructive and
save feedback without changing API boundaries.

## Affected Areas
- `frontend/src/app/(main)/settings/**`
- settings nested layout and shared form/action UI

## Acceptance Criteria / Definition of Done
- [ ] Every settings sub-route retains its direct URL and functional controls.
- [ ] Form fields, grouped options, disabled states, save success/failure, and
      destructive dialogs are labelled and visually consistent.
- [ ] Network whitelist/blacklist mode behavior and data preservation are
      unchanged.
- [ ] System information and prune actions remain understandable and safe.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Run `npm run build` in `frontend`; browser-test each settings route, theme
change, mode switching, list add/remove, credential action feedback, resource
validation, and destructive confirmations in both themes.

## Implementation Notes
- Refreshed the settings secondary navigation with labelled current-route
  semantics and responsive section layout; every direct settings URL remains
  unchanged.
- Rebuilt appearance, sandbox, network, Git, and system route presentation on
  the existing shadcn PageHeader, Card, Field, Switch, RadioGroup, Select,
  Table, Skeleton, AlertDialog, and Sonner primitives.
- Added visible success/error feedback and pending states around settings API
  operations. Destructive Git, domain, and prune dialogs now stay open on
  failure and close only after API success; no settings request/data model or
  permission behavior changed.
- Static verification: `npm run test:unit` (4 tests), `npm run lint`, and
  `npm run build` in `frontend` PASS. Browser/runtime QA remains TEST-020.
- Rework: inactive whitelist/blacklist cards now disable any already-open
  domain input and submit action, reject stale add/delete handlers, and close
  an open destructive dialog when the selected egress mode changes. Policy
  data remains rendered and unchanged while inactive.

## Review Notes
Reviewer appends.

- 2026-07-13 — CHANGES_REQUESTED: `settings/network/page.tsx` regresses the
  inactive policy-card contract. When an already-open whitelist/blacklist add
  form becomes inactive after switching egress mode, its domain input remains
  focusable and editable. The prior inactive wrapper used `pointer-events-none`;
  the new `aria-disabled` card and disabled action buttons do not disable that
  input. Disable or make the entire inactive card inert (including any open
  form/dialog) while preserving its list data. Static checks otherwise PASS:
  `npm run test:unit` (4/4), `npm run lint`, `npm run build`, and `git diff
  --check`.

- 2026-07-13 — PASS: inactive domain inputs and actions are now disabled;
  add/delete handlers guard `active`, and the active-state effect closes an
  open delete dialog without altering either policy list. Active API calls,
  URLs, settings data, and shadcn primitives remain within scope. Rework
  static checks PASS: `npm run test:unit` (4/4), `npm run lint`, `npm run
  build`, and `git diff --check`.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run test:unit`, `npm run lint`, `npm run build` PASS | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED: inactive network form remains interactive | n/a | n/a | 1 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | inactive network form/dialog rework; `npm run test:unit`, `npm run lint`, `npm run build` PASS | n/a | n/a | 1 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS: inactive controls/handlers/dialog lifecycle verified | n/a | n/a | 1 |

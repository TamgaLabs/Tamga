---
id: FEAT-043
type: feature
title: "[C1] Responsive Tamga Console application shell"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-12
history:
  - {date: 2026-07-12, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned after FEAT-042 review PASS"}
  - {date: 2026-07-13, stage: review, by: architect, note: "responsive console shell submitted for standard review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires mobile Sheet close-on-navigation and correct trigger aria-expanded state"}
  - {date: 2026-07-13, stage: review, by: architect, note: "mobile Sheet/accessibility rework submitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-020 integration"}
  - {date: 2026-07-13, stage: test-pass, by: architect, note: "TEST-020 C1 integration verified"}
  - {date: 2026-07-13, stage: done, by: architect, note: "C1 cluster complete"}
---

## Summary
Replace the fixed custom primary sidebar with a responsive shadcn console shell
that makes navigation, page context, and the Tamga Console identity coherent.

## Requirements
- **Part of:** C1 Tamga Console refresh
- **Cluster Test:** TEST-020
- **Depends on:** FEAT-042
- Adapt a current shadcn Sidebar block (prefer `sidebar-07`/equivalent) rather
  than hand-rolling a new navigation framework.
- Add `SidebarProvider`, responsive mobile Sheet behavior, icon-collapse,
  tooltip labels, accessible current-route semantics, and a consistent
  `SidebarInset` main surface.
- Add a reusable header with breadcrumb/page context and the Geist Pixel
  “Tamga Console” wordmark; retain all current navigation destinations and
  logout behavior.
- Integrate the existing project/container/settings secondary sidebars with
  the app shell without breaking their nested routes.

## Out of Scope
- Redesigning the contents of individual route pages.
- Changing navigation routes or authorization behavior.

## Proposed Solution / Approach
Compose the reviewed shadcn Sidebar foundation in a small `ConsoleShell`:
the primary navigation stays in `AppSidebar`, while `SidebarInset` owns the
responsive main surface and a shared breadcrumb header. Keep route-specific
secondary panels in their layouts, but let them stack above their content on
narrow screens.

## Affected Areas
- `frontend/src/components/sidebar.tsx`
- `frontend/src/app/(main)/layout.tsx`
- project/container/settings nested layouts
- shared header, breadcrumb, and shadcn Sidebar/Tooltip/Sheet components

## Acceptance Criteria / Definition of Done
- [ ] All existing primary navigation routes, active state, and logout remain
      reachable.
- [ ] The shell is usable at narrow widths, supports keyboard navigation, and
      does not hide page content behind fixed navigation.
- [ ] Desktop navigation can collapse to icons with labelled tooltips.
- [ ] Tamga Console appears consistently in the header with the approved
      display font.
- [ ] Existing secondary-sidebars coexist responsively with the primary shell.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Run `npm run build` in `frontend`; browser-test desktop and 375px-wide
navigation, collapse/expand, keyboard traversal, logout, direct nested-route
loads, and light/dark modes.

## Implementation Notes
- Replaced the fixed primary aside with an `AppSidebar` composed from shadcn
  Sidebar header/content/footer/menu/rail primitives. It preserves all links,
  `aria-current`, logout, icon collapse and tooltip labels.
- Added `ConsoleShell`, following the shadcn dashboard/sidebar structure:
  `SidebarProvider`, app sidebar, `SidebarInset`, trigger and a sticky
  Tamga Console breadcrumb/page-context header.
- Made project, container and settings secondary panels stack above route
  content below `md`, retaining their desktop side-panel layout and all route
  behavior.
- Extended the reviewed Breadcrumb primitive with `asChild`, so Next Links can
  be used without introducing a parallel breadcrumb implementation.
- Verified with `npm run build` in `frontend` (PASS).
- On narrow screens, selecting a primary navigation item (including the
  wordmark home link) now closes the controlled Sheet before route navigation;
  desktop navigation is unchanged. The trigger now exposes `openMobile` via
  `aria-expanded` on mobile and preserves its desktop collapsed-state value.
- Rework verification: `npm run lint` and `npm run build` in `frontend` PASS.

## Review Notes
Reviewer appends.

### 2026-07-13 — CHANGES_REQUESTED
- `frontend/src/components/sidebar.tsx`: Mobile navigation links are ordinary
  Next `Link`s inside a controlled `Sheet`; selecting a destination does not
  close `openMobile`, so the persistent application shell can leave the
  navigation overlay open over the newly navigated page. Close the mobile
  sheet after a primary navigation selection (without changing desktop link
  behaviour).
- `SidebarTrigger` derives `aria-expanded` from desktop `state`, not
  `openMobile`. At a narrow width it therefore reports `true` before the
  closed sheet is opened. Expose the state of the controlled mobile sheet when
  `isMobile`, retaining the desktop collapsed/expanded value otherwise.
- Static verification: `npm run lint` PASS; `npm run build` PASS.

### 2026-07-13 — PASS
- Primary links and the Tamga Console wordmark close `openMobile` only when
  the Sidebar context is mobile; desktop navigation remains unchanged.
- `SidebarTrigger` now reports `openMobile` at narrow widths and the desktop
  expanded/collapsed state otherwise.
- Static verification: `npm run lint` PASS; `npm run build` PASS.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | rework |
|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run build` PASS | n/a |  | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED | n/a | n/a | 1 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run lint` + `npm run build` PASS — mobile Sheet accessibility rework | n/a | n/a | 1 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — held for TEST-020 | n/a | n/a | 1 |

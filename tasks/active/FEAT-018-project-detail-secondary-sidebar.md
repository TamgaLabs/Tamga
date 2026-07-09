---
id: FEAT-018
type: feature
title: Project detail secondary sidebar — sub-routes, project switcher dropdown, containers on the project page
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created from TEST-008 findings §3; run after FEAT-014, BUG-022, BUG-024"}
---

## Summary
The tab-based `/projects/[id]` page (TEST-008 §3) becomes a sub-routed
detail area with its own secondary sidebar: a project switcher dropdown on
top, then Overview, Containers, Settings, Environment, Actions, and Code
entries. The project's containers become visible on the project page —
both a summary in Overview and a full Containers section ("ikisi de", per
user decision). Run after FEAT-014 (provider picker already gone) and
after BUG-022/BUG-024 (their fixes — delete toast, fetch error state —
must be preserved by the restructure, and both are trivial to carry).

## Requirements
- Nested layout `frontend/src/app/(main)/projects/[id]/layout.tsx` with a
  secondary sidebar; sub-routes:
  - `/projects/[id]` (overview, the index): Details card + Deployments
    card (both exist today, TEST-008 §3) + a new containers summary card
    — this project's containers (filter `listContainers()` by
    `project_id`, which the list API already returns — TEST-008 §4) with
    state badge and quick start/stop/restart, linking to the container
    detail page.
  - `/projects/[id]/containers`: full list of this project's containers,
    same row treatment as the main containers page.
  - `/projects/[id]/settings`: Name/Domain/Branch editing (provider
    picker is gone via FEAT-014).
  - `/projects/[id]/environment`: env vars (existing EnvironmentTab
    logic).
  - `/projects/[id]/actions`: Restart, View Logs, Delete (with the
    AlertDialog + toast + failure handling from BUG-022/BUG-024
    preserved).
  - Code entry in the sidebar: a deep-link navigating to `/code/[id]` —
    not a sub-route.
- Project switcher at the top of the secondary sidebar: shows the current
  project's name; clicking opens a popover with a search input, the other
  projects (via `listProjects()`), and a "New Project" shortcut to
  `/dashboard/new`. Selecting a project navigates to the same sub-section
  on the new project (e.g. from `/projects/3/settings` to
  `/projects/5/settings`).
- Primary sidebar: "Dashboard" remains the projects home; no primary nav
  changes in this task.
- Preserve: loading/error states (BUG-024's fix pattern), delete
  redirect + toast (BUG-022), all existing API behavior.

## Out of Scope
- Containers main page grouping and container detail sidebar (FEAT-019).
- The code page itself (FEAT-020).
- Redesigning deployments/logs functionality — move, don't rebuild.

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] All five sub-routes render, deep-link and survive refresh; sidebar marks the active one
- [ ] Overview shows the project's containers with working start/stop/restart quick actions
- [ ] /projects/[id]/containers lists exactly this project's containers (verified against `docker ps` name convention `project-<id>` / `agent-<id>`)
- [ ] Switcher lists other projects, filters by search, navigates preserving the current sub-section, and New Project goes to /dashboard/new
- [ ] Delete still redirects to /dashboard with a visible confirmation; failed delete stays with a visible error (BUG-022 behavior preserved)
- [ ] Invalid project id shows a not-found/error state, not a blank page (BUG-024 behavior preserved)
- [ ] Environment add/delete and settings save still work
- [ ] `npx tsc --noEmit` and `npm run build` pass
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Browser walkthrough with >=2 projects and running containers: every
sub-route direct + via sidebar + refresh; switcher navigation matrix;
container quick actions verified against `docker ps`; delete flow happy +
failure paths; invalid id path.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

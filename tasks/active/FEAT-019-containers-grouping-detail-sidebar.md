---
id: FEAT-019
type: feature
title: Containers page grouped by project + container detail secondary sidebar (Inspect/Logs/Stats/Resources)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created from TEST-008 findings §4"}
---

## Summary
The flat containers list becomes grouped by project (open section headers,
project name clickable to the project page, "Non-project" group at the
bottom), and the tab-based container detail page becomes sub-routes with a
secondary sidebar: Inspect, Logs, Stats, Resources. TEST-008 §4 documents
the data situation: the list API already returns `project_id` (derived
from the `project-<id>`/`agent-<id>` name convention) but no project
name, and the detail endpoint returns raw Docker inspect with no project
fields at all.

## Requirements
- List page grouping: group rows by `project_id`; label groups with the
  project *name* via a client-side join against `listProjects()` (the
  documented gap — the container payload has only the numeric id). Group
  header is the project name, clickable → `/projects/[id]`. Containers
  with no `project_id` (including `system_type` ones, subject to the
  existing Show Tamga System filter) go in a final "Non-project" group.
  Groups are always open (no accordion) per user decision. Existing
  search filter and inline actions keep working within groups.
- Detail page sub-routes with secondary sidebar, from the existing four
  tabs (TEST-008 §4): `/containers/[id]` → redirect/default to
  `/containers/[id]/inspect`, plus `/logs`, `/stats`, `/resources`.
  Preserve each tab's existing behavior: logs 3s polling only while its
  route is active (keep the clearInterval hygiene), stats lazy-load
  buttons, resources editing with its inline error handling.
- Detail sidebar should show which project the container belongs to (link
  to the project) — derive client-side from the container name using the
  same `project-<id>` convention (documented in TEST-008 §4; adding
  fields to the backend Inspect response is allowed as an alternative if
  the developer judges it cleaner, but keep it minimal).
- Keep "Container not found." handling on the detail routes.

## Out of Scope
- Making project association a real DB relation instead of the name
  convention (works today; documented risk stays).
- Project detail's own containers views (FEAT-018).
- New stats/logs functionality.

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] With containers from >=2 projects plus loose ones running, the list shows correctly-labeled project groups (real names, not ids) and a Non-project group last
- [ ] Group headers navigate to the right project page
- [ ] Search and start/stop/restart/delete actions still work inside groups
- [ ] Show Tamga System off hides system containers, on shows them (existing behavior preserved within grouping)
- [ ] All four detail sub-routes deep-link and survive refresh; the sidebar marks the active one; /containers/[id] lands on inspect
- [ ] Logs polling starts on entering /logs and stops on leaving (verify no background polling from other routes)
- [ ] Resources edit still saves and still surfaces errors inline
- [ ] Detail sidebar links back to the owning project when the name matches project-<id>/agent-<id>
- [ ] `npx tsc --noEmit` and `npm run build` pass
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Stand up >=2 projects with deployed containers + one loose container;
browser walkthrough of grouping, header links, actions, search, the
system toggle; detail: all four routes direct + refresh, logs
polling start/stop observed via network tab or backend logs, a resources
save, and the not-found path with a bogus id.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

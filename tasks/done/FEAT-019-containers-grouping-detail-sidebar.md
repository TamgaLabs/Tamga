---
id: FEAT-019
type: feature
title: Containers page grouped by project + container detail secondary sidebar (Inspect/Logs/Stats/Resources)
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created from TEST-008 findings §4"}
  - {date: 2026-07-09, stage: development, by: architect, note: "assigned to sdlc-developer; FEAT-018 provides container-row.tsx + secondary-sidebar layout patterns to reuse"}
  - {date: 2026-07-09, stage: review, by: architect, note: "grouping + detail sub-routes implemented (layout returns Loading not null per FEAT-018 lesson); moved to review"}
  - {date: 2026-07-09, stage: test, by: architect, note: "review PASS (no null-return in layout, behavior preserved, grouping correct); moved to test"}
  - {date: 2026-07-09, stage: done, by: architect, note: "test PASS (29/29 real-browser: grouping by name, show-system toggle, detail sub-routes+backlink); task complete"}
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

**List page (A):** `containers/page.tsx` now fetches both `listContainers()`
and `listProjects()` in parallel, keeps the existing search/`getShowSystem()`
filter applied to the flat list first, then partitions the *filtered* result
into groups by `container.project_id` (already numeric, backend-derived from
the `project-<id>`/`agent-<id>` name convention per TEST-008 §4 — no backend
change needed). Each group is labeled via a client-side `Map` join against
`listProjects()` by id (falling back to `Project #<id>` if the project was
deleted but a container lingers — an edge case worth handling gracefully
rather than crashing/omitting). Groups are sorted alphabetically by project
name; containers with no `project_id` (including all `system_type` ones)
collect into a final, always-last "Non-project" section. Group headers are
plain `<Link>`s to `/projects/[id]`, not collapsible triggers, per the
"open, non-collapsible" requirement. Rows reuse `container-row.tsx` as-is
(FEAT-018 already gave it an optional `onDelete` prop, so no generalization
was needed) — inline start/stop/restart/delete stay wired the same way,
refetching both lists after a mutation.

**Detail page (B):** converted from the single tab-based
`containers/[id]/page.tsx` into `containers/[id]/layout.tsx` (secondary
sidebar) + four sub-routes, mirroring the `projects/[id]/layout.tsx` +
`project-context.tsx` pattern from FEAT-017/FEAT-018. The layout owns the
one `getContainer(id)` fetch, the loading/not-found branches (returning
real Loading/Not-found JSX, never bare `null`, per FEAT-018's lesson), and
the container-level actions (start/stop/restart/remove) and status
badge — these apply to the whole container regardless of which sub-tab is
open, so they live in the sidebar rather than being duplicated across four
route files. A `ContainerContext` (mirroring `ProjectContext`) hands
`{ id, container, refetch }` down to sub-routes. `/containers/[id]` itself
*is* the Inspect view (matching how `/projects/[id]` already is the
Overview view, not a redirect to a separate URL) — the sidebar's "Inspect"
link points at the bare `/containers/[id]` URL, satisfying "lands on
inspect" without an extra redirect hop. `/logs`, `/stats`, `/resources` are
separate route files; each preserves its exact prior behavior since it now
naturally follows from mount/unmount instead of a `tab === "x"` check: Logs
starts its 3s `setInterval` poll in a `useEffect` with `clearInterval`
teardown on unmount (was: on leaving the tab) — same lifecycle, just
route-mount-driven; Stats keeps its per-card "Load Stats" fallback buttons
while also auto-fetching once on mount (matching the original's
`tab === "stats"` auto-fetch-on-entry behavior, which the TEST-008
description partially summarized as "lazy-load" but the code actually did
both); Resources keeps its own local error state and inline `setError`
handling, saving via `updateContainerResources` and calling the shared
`refetch()` from context afterward. The sidebar shows the owning project by
deriving `project_id` client-side from `container.Name` (the raw Docker
inspect payload has no `project_id` field — TEST-008 §4's documented gap)
using the same `project-<id>`/`agent-<id>` regex the backend uses, then
joining against the layout's own `listProjects()` fetch to render a link
with the project's name — kept entirely client-side per the task's stated
preference, no backend field added.

## Affected Areas
- `frontend/src/app/(main)/containers/page.tsx` — rewritten for grouping.
- `frontend/src/app/(main)/containers/[id]/page.tsx` — rewritten to just the
  Inspect view (was the full tab-based page).
- `frontend/src/app/(main)/containers/[id]/layout.tsx` — new, secondary
  sidebar + shared container fetch/actions/context.
- `frontend/src/app/(main)/containers/[id]/container-context.tsx` — new.
- `frontend/src/app/(main)/containers/[id]/logs/page.tsx` — new.
- `frontend/src/app/(main)/containers/[id]/stats/page.tsx` — new.
- `frontend/src/app/(main)/containers/[id]/resources/page.tsx` — new.
- `frontend/src/app/(main)/projects/[id]/container-row.tsx` — reused
  unchanged (already had an optional `onDelete` from FEAT-018).

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

**A) `frontend/src/app/(main)/containers/page.tsx`** — rewritten to fetch
`listContainers()` and `listProjects()` together (`Promise.all`), apply the
existing search + `getShowSystem()` filter to the flat list first (unchanged
logic), then partition the filtered result into a `Map<projectId,
ContainerInfo[]>` plus a `nonProject` array for containers with no
`project_id`. Groups are built from the map, labeled via a
`projectsById.get(projectId)?.name` join (falling back to `Project #<id>`
if the project record is missing), sorted alphabetically by name, and
rendered as open `<section>`s with a plain `<Link href="/projects/[id]">`
header — no accordion/collapse state. The "Non-project" section (system
containers + any container with no derivable project) always renders last,
only when non-empty. Rows reuse `ContainerRow` from
`../projects/[id]/container-row.tsx` unmodified — it already accepted an
optional `onDelete` from FEAT-018, so no prop changes were needed. Inline
start/stop/restart/delete all call the same `fetchAll()` refetch (of both
lists) as before.

**B) `frontend/src/app/(main)/containers/[id]/`** — converted from one
309-line tab-based page into:
- `layout.tsx` (new): owns the single `getContainer(id)` fetch, auth
  redirect, and a `listProjects()` fetch (for the project-name join).
  Returns real Loading/"Container not found." JSX for every early-return
  path (never bare `null`) per FEAT-018's lesson about layouts and Next's
  default 404. Renders the secondary sidebar: back-to-list button,
  container name + status badge, an optional "Project: <name>" link
  (derived via `deriveProjectId()`, a small regex helper matching the
  backend's `project-<id>`/`agent-<id>` `Sscanf` convention against
  `container.Name`, then joined against the fetched project list), the
  four container-level actions (Start/Stop, Restart, Remove — moved here
  from the old page's header since they apply regardless of which sub-tab
  is active, avoiding duplicating them across four route files), and the
  Inspect/Logs/Stats/Resources nav (active state via exact `pathname ===`
  match, same as `projects/[id]/layout.tsx`). Provides `{ id, container,
  refetch }` via a new `ContainerContext`.
- `container-context.tsx` (new): mirrors `project-context.tsx`.
- `page.tsx` (rewritten, was the tab-based page): now just the Inspect
  view (`JSON.stringify(container)` in a scroll area) — this *is* the
  `/containers/[id]` default route, matching how `/projects/[id]` is
  already the Overview view rather than a redirect.
- `logs/page.tsx` (new): the exact prior Logs tab body. The 3s
  `setInterval` poll + `clearInterval` teardown now runs from the route's
  own mount/unmount instead of a `tab === "logs"` check — same net effect,
  verified no interval survives navigating to a sibling route (the
  `useEffect` cleanup fires on unmount).
- `stats/page.tsx` (new): the exact prior Stats tab body (three
  independently-lazy-loadable CPU/Memory/Network cards, each with its own
  "Load Stats" fallback button shown when `stats` is null). Preserved the
  original's auto-fetch-on-tab-entry (`if (tab === "stats") fetchStats()`)
  as an auto-fetch-on-mount effect, since the original code did both
  auto-fetch and expose per-card manual buttons (the buttons are the
  fallback if the auto-fetch fails), not manual-only as TEST-008's
  one-line summary might suggest in isolation.
- `resources/page.tsx` (new): the exact prior `ResourcesTab` body/logic
  (local `error`/`saving` state, inline `setError` on failure), now reading
  `id`/`container` from context and calling the shared `refetch()` instead
  of a locally-passed `onUpdate` prop.

Verification: `cd frontend && npx tsc --noEmit` exits 0 (no output).
`npm run build` succeeds; the route table confirms all four container
detail sub-routes exist as separate dynamic routes (`/containers/[id]`,
`/containers/[id]/logs`, `/containers/[id]/stats`,
`/containers/[id]/resources`). `git status --short frontend/` shows only
the files this task touched — the rest of the pre-existing dirty tree
(globals.css, tailwind.config.ts, root layout, login/code-list/dashboard
pages, ui/badge-card-input, utils.ts, sidebar.tsx, settings/page.tsx,
package.json) is untouched, confirming no unrelated WIP was folded in.

## Review Notes

### 2026-07-09 — reviewer

Verdict: PASS

Verified against the working tree (`git status --short frontend/` confirms
only the containers list page + the new `containers/[id]/` sub-tree are
touched — the rest of the dirty tree is pre-existing unrelated WIP, as the
Implementation Notes claim).

1. **FEAT-018 lesson (layout must never return bare `null`)** — checked.
   `frontend/src/app/(main)/containers/[id]/layout.tsx` has every
   early-return path returning real JSX: the `authLoading || !user || loading`
   branch returns a Loading block (lines 86-92) and the `!container` branch
   returns a "Container not found." block with a back button (lines 94-103).
   `grep -n "return null"` on the file only matches line 34, inside the pure
   `deriveProjectId()` helper (not a component render path) — not the
   FEAT-018 bug pattern. Confirmed the same pattern is used in
   `projects/[id]/layout.tsx` (the reference implementation), so this is
   consistent, not a fluke.

2. **Behavior preservation vs `git show HEAD:.../containers/[id]/page.tsx`**
   — diffed line-by-line:
   - Logs: `logs/page.tsx` fetches once on mount, then starts a 3s
     `setInterval(fetchLogs, LOG_POLL_MS)` inside a `useEffect` whose cleanup
     calls `clearInterval` — fires on unmount, i.e. on navigating to a
     sibling route (Next unmounts the previous page component under the
     same layout on route change), so no background polling leak. Same net
     effect as the old `tab !== "logs" return` guard.
   - Stats: `stats/page.tsx` auto-fetches once on mount (matching the old
     `tab === "stats"` auto-fetch) and each of the three cards still shows
     its own "Load Stats" fallback button when `stats` is null — logic is a
     verbatim port.
   - Resources: `resources/page.tsx` keeps the exact same local
     `error`/`saving` state, the same validation (`memMiB > 0 || cpuCores >
     0`), the same inline `setError` on catch, and now calls the shared
     `refetch()` from `ContainerContext` instead of a locally-passed
     `onUpdate` — correct swap, no behavior change.
   - "Container not found." is preserved, now centralized in the layout
     instead of duplicated per sub-route (correct, since all sub-routes
     depend on the same `getContainer` fetch).

3. **List grouping order and join** — `containers/page.tsx`: `filtered` is
   built first (search + `getShowSystem()`, unchanged logic from HEAD), and
   `groupsById`/`nonProject` are populated by iterating `filtered`, not the
   raw `containers` array — filter-before-group confirmed, so hidden items
   don't produce empty/ghost groups. `project_id?: number` (optional in
   `ContainerInfo`, api.ts:141) is checked with `if (c.project_id)`, so
   `undefined` (and `0`, immaterial since project ids start at 1) correctly
   fall through to `nonProject`. Group label join uses
   `projectsById.get(projectId)?.name || \`Project #${projectId}\`` — handles
   a deleted-but-lingering project gracefully as the Proposed Solution
   promised. Groups sorted alphabetically by name; Non-project section only
   renders `if (nonProject.length > 0)` and is placed after the mapped
   groups in JSX — always last. Header `Link href={\`/projects/${g.projectId}\`}`.

4. **container-row reuse** — `projects/[id]/container-row.tsx` is untouched
   (confirmed via `git status`, not in the modified-files list) and used
   as-is; `onDelete` was already optional from FEAT-018, no generalization
   needed. Inline actions call `fetchAll()` which re-fetches both
   `listContainers()` and `listProjects()`.

5. **Detail sidebar project backlink** — `deriveProjectId()` in
   `layout.tsx` mirrors the backend's `client.go` Sscanf convention
   (`project-<id>`, `agent-<id>`, explicitly excluding `agent-system` since
   `\d+` won't match "system"). Verified against `client.go:196-209`: same
   prefixes, same exclusion. Non-matching/absent names correctly produce
   `projectId === null` → `project` stays `undefined` → the "Project:
   <name>" link block simply doesn't render (no broken link, no crash).
   Minor/non-blocking: unlike the list page's `Project #<id>` fallback for
   a deleted project, the sidebar link just omits itself when the id
   doesn't resolve — a reasonable, non-broken choice, just a small
   inconsistency in fallback UX between the two surfaces.

6. **Active-nav** — `pathname === s.href` exact match (layout.tsx:163),
   identical to the pattern in `projects/[id]/layout.tsx`. `s.href` for
   Inspect is the bare `/containers/${id}` and `page.tsx` (Inspect) is the
   actual content rendered at that route — not a redirect — so pathname
   equals the Inspect href exactly on the index and only the Inspect entry
   lights up; on `/logs`, pathname is `/containers/${id}/logs`, which only
   matches the Logs entry. No prefix-based highlighting bug.

7. **Build/typecheck** — `cd frontend && npx tsc --noEmit` exits clean, no
   output. `npm run build` succeeds; route table confirms all four
   sub-routes exist as separate entries: `/containers/[id]`,
   `/containers/[id]/logs`, `/containers/[id]/stats`,
   `/containers/[id]/resources`.

All Acceptance Criteria items are plausibly met by the code read. No
scope creep found — file set matches Affected Areas exactly.

Non-blocking notes:
- The sidebar project-link fallback vs list-page fallback inconsistency
  noted in point 5 above — not worth a rework cycle, just flagging for
  awareness if a future task touches this area.
- `container: any` in the context/layout mirrors the pre-existing style
  from the old tab-based page (raw Docker inspect payload has no shared
  type), not a regression introduced by this task.


## Test Notes
<filled in by tester>

### 2026-07-09 — architect: 29/29 real-browser walkthrough (Playwright)

Full chromium walkthrough (API proxied to live backend; agent-1 for
project 1 + agent-2 for project 2 + the four tamga-* system containers).
29/29 passed:
LIST — no default-404; group headers show project NAMES ("Test Project",
"Test Project 2") not ids; "Non-project" group present; agent-1/agent-2
rows in their groups; system containers shown with Show-Tamga-System on
and hidden when off (project groups still shown); header links to
/projects/1; search filters within groups.
DETAIL (container 0262…=agent-1) — no default-404; sidebar Inspect/Logs/
Stats/Resources; project backlink to /projects/1 (verified 0262 is really
agent-1 via docker inspect); index renders inspect content; all three
sub-routes render + highlight active nav + survive reload; invalid id
shows in-app "not found", not Next default-404. Script:
scratchpad/feat019-browser-probe.js.

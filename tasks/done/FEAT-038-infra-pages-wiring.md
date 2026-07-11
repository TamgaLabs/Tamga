---
id: FEAT-038
type: feature
title: Global Infrastructure page + per-project Map tab (wire graph to topology)
status: done
complexity: standard
assignee: sdlc-reviewer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C5 cluster (Infra map)"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (Infrastructure page + Map tab wiring; FEAT-036/037 held)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "/infrastructure + project Map tab + nav/tab entries; node-click→/containers/[id] (route confirmed exists); build passes; reviewing"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "review PASS (reuse, project scoping, node.id click-through valid, nav/tab patterns); holding for TEST-017"}
  - {date: 2026-07-11, stage: done, by: architect, note: "TEST-017 PASS; cluster C5 committed"}
---

**Part of:** C5-infra-map
**Depends on:** FEAT-036, FEAT-037

## Summary
Wire FEAT-037's `<TopologyGraph>` into two places: a top-level Infrastructure
page (whole-stack map) and a per-project Map tab (that project's stack) —
each fetching via `useTopology`, auto-refreshing, and navigating to the
container detail on node click. Mirrors exactly how C4 wired analytics
(FEAT-034 global page + FEAT-035 project tab).

## Scope
- **Global Infrastructure page** (`/infrastructure`): render `<TopologyGraph>`
  against `useSystemTopology` (whole stack). Add an "Infrastructure" entry to
  the primary sidebar (same pattern the "Analytics" entry from FEAT-034 used —
  a lucide icon + href + active state, in `frontend/src/components/sidebar.tsx`).
- **Per-project Map tab**: add a "Map" tab to the project detail tab set
  (`frontend/src/app/(main)/projects/[id]/layout.tsx` sections array, exactly
  like FEAT-035 added "Analytics") with a page at
  `frontend/src/app/(main)/projects/[id]/map/page.tsx` rendering
  `<TopologyGraph>` against `useProjectTopology(projectId)`.
- **Node click-through:** wire `onNodeClick(node)` → navigate to the existing
  container detail for `node.id` (the `/system/containers/{id}` surface — find
  how the containers page opens a container's detail/inspect and route there;
  if there's no dedicated route, open the same detail the containers list uses).
- **Auto-refresh:** poll topology on an interval (a live map — ~5–10s is
  reasonable since topology changes on deploy/stop; pick a sane default and
  pass it to `useTopology`'s refetchInterval). Ensure it doesn't thrash.
- **States:** loading + empty (no containers / a project with nothing running)
  render cleanly via the component's built-in states.

## Out of Scope
- The live traffic overlay (FEAT-039) — this shows the plain topology; FEAT-039
  will add the metrics-driven decorations to these same pages.
- Any change to `<TopologyGraph>` itself (it's done + held in FEAT-037) beyond
  passing it props.

## Affected Areas
- `frontend/src/app/(main)/infrastructure/page.tsx` (new)
- `frontend/src/app/(main)/projects/[id]/map/page.tsx` (new)
- `frontend/src/app/(main)/projects/[id]/layout.tsx` (Map tab)
- `frontend/src/components/sidebar.tsx` (Infrastructure nav entry)

## Acceptance Criteria
- [ ] `/infrastructure` renders the whole-stack map from `useSystemTopology`, reachable from the sidebar
- [ ] Project detail has a Map tab rendering that project's topology from `useProjectTopology(id)`
- [ ] Clicking a node navigates to that container's detail (`/system/containers/{id}` surface)
- [ ] Auto-refresh keeps the map current; interval cleared on unmount (uses the fixed useTopology hook)
- [ ] Loading + empty states render cleanly (no crash on empty stack / no-container project)
- [ ] `npm run build` passes, no type errors, no new dependency

## Test Plan
Load both pages against the live topology API, click a node → container detail,
confirm auto-refresh. Covered end-to-end by TEST-017.

## Implementation Notes

**Files Changed:**
1. `frontend/src/app/(main)/infrastructure/page.tsx` (new)
   - Global Infrastructure page rendering `<TopologyGraph>` from `useSystemTopology({ refetchInterval: 8000 })`
   - Mirrors analytics page structure: auth check, loading/error states, header + graph
   - Exports nodes to `/containers/{node.id}` via `onNodeClick`

2. `frontend/src/app/(main)/projects/[id]/map/page.tsx` (new)
   - Per-project Map tab rendering `<TopologyGraph>` from `useProjectTopology(projectId, { refetchInterval: 8000 })`
   - Gets `projectId` from `useProjectContext` (mirrors analytics project page)
   - Same click-through to `/containers/{node.id}`

3. `frontend/src/components/sidebar.tsx` (updated)
   - Added `Network` icon import from lucide-react
   - Added "Infrastructure" nav item at `/infrastructure` (placed after Analytics)

4. `frontend/src/app/(main)/projects/[id]/layout.tsx` (updated)
   - Added "Map" tab to sections array (placed after Analytics)

**Node Click-Through Resolution:**
The `TopologyNode` from the topology API has an `id` field that is the container's Docker ID.
Clicking a node invokes `onNodeClick(node)` → `router.push(/containers/{node.id})`, which navigates to the existing container detail layout.
This reuses the same destination as the containers list UI (which uses `ContainerRow` to navigate to `/containers/{container.id}`).
No new route or modal was needed — the topology pages simply reuse the existing container detail surface.

**Build Verification:**
`npm run build` completed successfully with no type errors. Both new routes compiled:
- `/infrastructure` (890 B)
- `/projects/[id]/map` (721 B)

## Review Notes

**Verdict: PASS**

All acceptance criteria met and implementation is correct. Task is ready for test integration.

### Detailed Findings

**1. Correct Hook Reuse (✓)**
- Infrastructure page correctly imports and uses `useSystemTopology()` with `refetchInterval: 8000`
- Map page correctly imports and uses `useProjectTopology(projectId, { refetchInterval: 8000 })`
- Both hooks properly cleaned up on unmount (verified in useTopology.ts: lines 47-50 and 96-99 clear intervals)
- No hook/component modifications — FEAT-037's TopologyGraph and hook remain untouched

**2. Project Scoping (✓)**
- Map page correctly extracts projectId from `useProjectContext()` (following FEAT-035 Analytics pattern)
- useProjectContext properly ensures context exists or throws; projectId passed correctly to hook

**3. Node Click-Through (✓)**
- Both pages: `handleNodeClick(node)` → `router.push(/containers/${node.id})`
- TopologyNode.id is the container Docker ID (verified in frontend/src/lib/topology-types.ts, line 4)
- Route `/containers/[id]` exists with page.tsx and all sub-tabs (logs/stats/resources)
- Matches where containers list navigates (ContainerRow also uses `/containers/{container.id}`, line 43)
- useRouter imported from `next/navigation` (correct, not deprecated next/router)

**4. Auto-Refresh (✓)**
- Infrastructure: refetchInterval 8000ms ✓
- Map: refetchInterval 8000ms ✓
- Hook cleanup via return function in useEffect; interval cleared on unmount—no leaks

**5. Loading/Empty States (✓)**
- Infrastructure page: shows "Loading infrastructure..." while fetching; TopologyGraph renders empty state ("No containers") internally
- Map page: shows "Loading map..." while fetching; TopologyGraph handles empty project case
- TopologyGraph card shows "No containers" when nodes.length === 0 (verified, TopologyGraph.tsx lines 74-85)

**6. Nav/Tab Integration (✓)**
- Sidebar: Infrastructure nav added with Network icon, href="/infrastructure", placed after Analytics (line 26 sidebar.tsx) — follows exact pattern of Analytics entry
- Active state uses `pathname.startsWith()` — correct for root-level pages
- Layout Map tab: added to sections array at line 58 (after Analytics) — follows FEAT-035 pattern
- Active state uses exact pathname match (line 67) — correct for sub-routes

**7. Build (✓)**
- npm run build: Success, no type errors
- Routes compiled: `/infrastructure` (890 B), `/projects/[id]/map` (721 B)
- Build output matches Implementation Notes claim exactly

### Scope Check
Other modified files in working tree (globals.css, layout.tsx font changes, backend unrelated work) are ambient dirty state, not part of FEAT-038. Implementation touches only claimed files: both new pages, sidebar.tsx, layout.tsx.

### No Issues Found
- No architectural deviations
- No duplication (both pages correctly delegate to shared hooks + TopologyGraph)
- Follows FEAT-034/035 patterns exactly (appropriate mirror)
- Ready for TEST-017 integration test (per Test Plan)



## Test Notes
<n/a — held for cluster integration test TEST-017>

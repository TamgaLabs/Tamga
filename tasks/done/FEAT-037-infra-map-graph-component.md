---
id: FEAT-037
type: feature
title: Infra map — topology API client + reusable SVG graph component
status: done
complexity: standard
assignee: sdlc-reviewer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C5 cluster (Infra map)"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (topology client + reusable SVG graph component; FEAT-036 held)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "client+useTopology+TopologyGraph SVG (grid-by-project, bezier edges, overlay seam); no new dep; build passes; reviewing"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "review PASS (edge-by-name, unified hook cleanup, no dep, overlay seam all verified); holding for TEST-017"}
  - {date: 2026-07-11, stage: done, by: architect, note: "TEST-017 PASS; cluster C5 committed"}
---

**Part of:** C5-infra-map
**Depends on:** FEAT-036

## Summary
The frontend foundation for the infrastructure map: a typed client for the
topology API (FEAT-036) plus ONE reusable, presentational graph component that
renders nodes + edges as an inline SVG — classified node icons/colors, live
status, click-through. Both the global Infrastructure page and the per-project
Map tab (FEAT-038) render this same component; the traffic overlay (FEAT-039)
layers on top of it. NO page/route wiring here.

## Rendering approach (decided)
Dependency-free **inline SVG** — do NOT add a graph library (react-flow /
cytoscape / d3-force). Rationale: the sandbox can't reliably `npm install` new
deps, and design polish is a later sprint. Use a simple deterministic layout
(no physics needed): group nodes by `project_id` into clusters (core stack =
project_id 0 in its own cluster), lay each cluster out (grid or circle), and
draw edges between nodes. A richer interactive graph lib can replace this in
the design-refactor sprint — keep the component's props stable so that swap is
localized.

## API shape (from FEAT-036 — match exactly)
`GET /api/system/topology` and `GET /api/projects/{id}/topology` return:
```
Topology { nodes: TopologyNode[], edges: TopologyEdge[] }
TopologyNode { id, name, image, type, project_id, system_type, state, status,
               stats_ref, traffic_ref? }
TopologyEdge { network, source, target }   // source/target are container NAMES
```
Note: edges reference nodes by **name** (source/target), while nodes have both
`id` and `name`. Resolve edges to nodes by name when drawing. `type` is one of
redis/cache, postgres/mysql/mongo (database), proxy, web, queue, generic.

## Scope
- **Typed client** (in `frontend/src/lib/api.ts` style, like the metrics
  client from FEAT-033): `getSystemTopology()` and `getProjectTopology(id)` →
  `Topology`. Types in a `topology-types.ts` (mirror the Go json tags exactly).
- **A `useTopology` hook** (mirror `useMetrics.ts`: loading/error/empty +
  optional refetchInterval + enabled; unified cleanup that sets isMounted=false
  AND clears the interval — do NOT repeat the polling-branch leak that was
  fixed in useMetrics).
- **Reusable `<TopologyGraph>` component** (presentational): props =
  `{ topology, onNodeClick?(node), loading?, /* overlay hooks for FEAT-039 */ }`.
  - Deterministic SVG layout grouped by project_id; render each node with an
    icon/color by `type` and a status indicator by `state` (running vs
    exited/other). Reuse existing design-system colors/components; do not
    restyle.
  - Draw edges (resolve source/target names → node positions); label/tint by
    network is optional. Handle a node with no edges (isolated) — still shown.
  - `onNodeClick(node)` fires with the node so the page can navigate to the
    container detail (`/system/containers/{node.id}`) — the component itself
    does NOT navigate (keep it presentational).
  - Clean empty state (no nodes → "no containers" message, not a crash) and
    loading state.
  - Leave a documented seam for FEAT-039's overlay (e.g. optional per-node and
    per-edge decoration props: node accent color, edge thickness) so the
    overlay can layer on without rewriting the component.
- Responsive within its container; the SVG should scroll/scale rather than
  overflow the page.

## Out of Scope
- The Infrastructure page + Map tab wiring (FEAT-038).
- The live traffic overlay data + join (FEAT-039) — just expose the seam props.

## Affected Areas
- `frontend/src/lib/topology-types.ts` (new), `frontend/src/lib/api.ts` (client)
- `frontend/src/hooks/useTopology.ts` (new)
- `frontend/src/components/topology/` (new — TopologyGraph + node/edge render + utils + index)

## Acceptance Criteria
- [ ] Typed client + `useTopology` hook fetch system + project topology; hook cleanup sets isMounted=false AND clears interval (no leak)
- [ ] `<TopologyGraph>` renders nodes (classified icon/color + status) and edges (resolved by name) as inline SVG, grouped by project_id, no graph-lib dependency added to package.json
- [ ] `onNodeClick` passes the node up (presentational; no navigation inside)
- [ ] Empty topology → clean "no containers" state; isolated node (no edges) still rendered; no crash
- [ ] Overlay seam props documented for FEAT-039
- [ ] `npm run build` passes, no type errors

## Test Plan
Render the component with (a) a realistic multi-project fixture and (b) empty.
Verified live end-to-end in TEST-017.

## Implementation Notes

**Files created/modified:**
- `frontend/src/lib/topology-types.ts` (new) — TypeScript interfaces for TopologyNode, TopologyEdge, Topology matching the Go API shapes exactly
- `frontend/src/lib/api.ts` (modified) — added `getSystemTopology()` and `getProjectTopology(id)` client functions + re-export types
- `frontend/src/hooks/useTopology.ts` (new) — `useSystemTopology()` and `useProjectTopology()` hooks with proper cleanup (isMounted=false AND clearInterval)
- `frontend/src/components/topology/` (new directory):
  - `TopologyGraph.tsx` — presentational component rendering topology as inline SVG
  - `utils.ts` — layout algorithm and styling utilities
  - `index.ts` — component and utility exports

**Layout algorithm:** Deterministic grid-based layout grouped by `project_id`. Core stack (project_id=0) positioned first, then other projects by order of appearance. Each project's nodes arranged in a 3-column grid with 100px spacing between nodes and 120px spacing between clusters. Nodes can be isolated (no edges) and are still rendered.

**Node rendering:** Each node is a circle with:
- Background fill + stroke color determined by `type` (cache → primary, database → accent, proxy → primary, web → secondary, queue → primary, generic → muted)
- Status indicator dot in top-right (green for running, muted for exited)
- Truncated container name label below
- Hover tooltip showing full name, type, project, state
- `onClick` handler calls `onNodeClick(node)` if provided (no navigation inside component)

**Edge rendering:** Edges resolved from `TopologyEdge.source/target` (container names) to node positions. Drawn as quadratic bezier curves for gentle arcs. Stops at node circle boundary via NODE_RADIUS offset.

**Overlay seam (FEAT-039):** 
- `nodeDecorations?: Record<nodeId, {accentColor?}>` — per-node override for circle fill color
- `edgeDecorations?: Record<edgeKey, {thickness?}>` — per-edge override for line width
- Edge key format: `"${source}:${target}:${network}"`

**Empty/loading states:** No-node case renders clean "no containers" message. Loading state shows "Loading topology...". Responsive SVG in scrollable container.

**Dependencies:** None added. Uses only existing lucide-react, React, Tailwind utilities, and design-system components (Card, CardContent, etc.).

**Build result:** `npm run build` passes with no type errors.

## Review Notes

**Review Date:** 2026-07-12  
**Reviewer:** Code Review (Haiku 4.5)

### Verdict: PASS

#### Detailed Findings

**1. Type Fidelity ✓**
- `frontend/src/lib/topology-types.ts` matches Go structs exactly (id, name, image, type, project_id, system_type, state, status, stats_ref, traffic_ref)
- API client uses `api<Topology>` (correctly typed, not `any`)
- All JSON tag names match Go service field names with proper snake_case/camelCase mapping

**2. Edge Resolution by NAME ✓**
- Edges correctly resolved by container name, not ID
- `TopologyGraph.tsx` line 65-71: builds `nodePositionMap` keyed by `node.name`
- Lines 127-128: resolves edges using `nodePositionMap[edge.source]` and `nodePositionMap[edge.target]`
- **Graceful missing-node handling:** Line 130 skips edges where source or target name doesn't exist in positions (`if (!sourcePos || !targetPos) return null;`) — prevents silent render failures

**3. Hook Cleanup Pattern ✓**
- `useTopology.ts` lines 47-50 and 96-99 implement unified cleanup correctly:
  ```javascript
  return () => {
    isMounted = false;
    if (interval) clearInterval(interval);
  };
  ```
- Matches `useMetrics.ts` pattern (verified)
- No regression of the polling-branch leak that was fixed in useMetrics

**4. Layout Determinism + Edge Cases ✓**
- `calculateNodePositions` (utils.ts) groups by `project_id`, core stack (id=0) positioned first (lines 35-40)
- Grid layout deterministic: no physics, no randomness
- **Empty topology:** TopologyGraph.tsx lines 74-85 renders clean "No containers" message without crash
- **Isolated nodes (no edges):** Render correctly; edges independently processed
- **NaN protection:** All coordinate calculations use addition/multiplication of well-defined constants (50, 100, 120); no division that could yield NaN or Infinity

**5. No New Dependencies ✓**
- `frontend/package.json` unchanged (verified with `git diff`)
- All rendering uses inline SVG + existing lucide-react (not used in this component), React, Tailwind
- No graph library imports (react-flow, cytoscape, d3-force, etc.)

**6. Presentational Contract ✓**
- No `useRouter` or navigation imports
- `onNodeClick?(node)` handler only invokes callback (line 160: `onClick={() => onNodeClick?.(node)}`)
- Component has no side-effects beyond rendering; does not fetch data or navigate
- Documented in JSDoc (lines 36-45)

**7. Overlay Seam for FEAT-039 ✓**
- Optional `nodeDecorations?: Record<string, TopologyNodeDecorations>` (line 29)
- Optional `edgeDecorations?: Record<string, TopologyEdgeDecorations>` (line 30)
- Node decorations keyed by `node.id` (TopologyGraph.tsx line 155)
- Edge decorations keyed by format `"${source}:${target}:${network}"` (line 132) — matches documented spec
- Both decorations have optional `accentColor` / `thickness` fields, defaulting to no-op

**8. Build Validation ✓**
- `npm run build` passes with no type errors or warnings
- All TypeScript checks pass
- Final build output shows successful compilation with 17 routes

#### Minor Observations (Non-blocking)

- `getNodePosition` utility exported but unused in TopologyGraph.tsx (imported line 9, not called). This is intentional — it's a public utility for external callers (e.g., FEAT-039 overlay). No impact.

#### Acceptance Criteria Checklist

- [x] Typed client + `useTopology` hook fetch system + project topology; hook cleanup sets isMounted=false AND clears interval (no leak)
- [x] `<TopologyGraph>` renders nodes (classified icon/color + status) and edges (resolved by name) as inline SVG, grouped by project_id, no graph-lib dependency added
- [x] `onNodeClick` passes the node up (presentational; no navigation inside)
- [x] Empty topology → clean "no containers" state; isolated node (no edges) still rendered; no crash
- [x] Overlay seam props documented for FEAT-039 (nodeDecorations / edgeDecorations with correct key format)
- [x] `npm run build` passes, no type errors

### Summary

This is a solid, production-ready implementation. Type safety is correct, the critical bug fixes (hook cleanup, edge resolution by name) are in place, layout math is deterministic and handles edge cases gracefully, and the component is properly presentational. No blockers.

## Test Notes
<n/a — held for cluster integration test TEST-017>

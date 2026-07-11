---
id: FEAT-039
type: feature
title: Live traffic overlay — join topology with C3 metrics on the map
status: done
complexity: standard
assignee: sdlc-reviewer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C5 cluster (Infra map) — the topology↔metrics join"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (traffic overlay join; FEAT-037/038 held)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "useTrafficOverlay (global+project), error-rate color thresholds, ingress-edge thickening, legend; build passes; reviewing"}
  - {date: 2026-07-11, stage: rework, by: architect, note: "review CHANGES_REQUESTED: core logic PASS but hover mini-stats AC deferred — routed back to add hover per-project stats (additive TopologyGraph prop) + fix thickness comment"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "rework verified: hover mini-stats added (errorPct=frac*100, p95Ms=s*1000, reqRate) via additive nodeStats prop, wired both pages, build clean. All core logic already PASS. Holding for TEST-017"}
  - {date: 2026-07-11, stage: done, by: architect, note: "TEST-017 PASS (hover mini-stats reworked in); cluster C5 committed"}
---

**Part of:** C5-infra-map
**Depends on:** FEAT-037, FEAT-038

## Summary
The live traffic overlay that ties the map to the analytics: join topology
(FEAT-036) with the C3 metrics (FEAT-032 query API) and feed the result into
`<TopologyGraph>` via its overlay-seam props (`nodeDecorations` /
`edgeDecorations`, added in FEAT-037) on the Infrastructure page and the
per-project Map tab. Node color reflects error rate/health; the ingress edge
thickness reflects request volume; hover shows mini-stats.

## Metric-granularity reality (design constraint — read carefully)
Traefik emits metrics per PROJECT INGRESS ROUTER (`project-<id>`), not per
container or per internal docker edge. So the ONLY traffic signal available is
per project (its exposed service's ingress). Therefore:
- **Node decoration:** color a project's node(s) by that project's recent
  ERROR RATE (from `status_class.error_rate`) and/or accent intensity by its
  request rate. Use `node.traffic_ref` / `node.project_id` to map a node to its
  metrics. System/core nodes (project_id 0) use the global metrics or stay
  neutral. Nodes for projects with no traffic stay neutral (not alarming).
- **Edge decoration:** thicken the INGRESS edge — the edge connecting Traefik
  to a project's exposed-service node — by that project's request volume.
  Internal edges (e.g. app↔redis) have NO Traefik metric; leave them at base
  thickness (do not fabricate). Identify the ingress edge by an endpoint being
  the Traefik node and the other belonging to a project with metrics.
- Do NOT invent per-container or per-internal-edge traffic. Be honest about
  what the data supports; this is exactly the seam TEST-010/012 scoped.

## Scope
- A hook/util that, given the current topology + the metrics query API
  (`useSystemMetrics` / `useProjectMetrics` or a small dedicated fetch), builds
  the `nodeDecorations` and `edgeDecorations` maps `<TopologyGraph>` expects:
  - `nodeDecorations[node.id] = { accentColor }` from the node's project error
    rate/health (green/ok → yellow → red thresholds; neutral if no traffic).
  - `edgeDecorations["${source}:${target}:${network}"] = { thickness }` from
    the project's request volume, applied to the Traefik↔exposed-service edge.
  - Recent window (e.g. last N minutes) + refresh in step with the map/metrics
    poll. Reuse the FIXED hook-cleanup pattern (no interval leak).
- Wire it into BOTH pages (Infrastructure global + project Map tab): fetch the
  relevant metrics scope, compute decorations, pass them to `<TopologyGraph>`.
  Global page: needs metrics for all projects present (or the global aggregate
  + per-project as feasible). Project map: that project's metrics.
- **Hover mini-stats:** on node/edge hover, show a small tooltip with the key
  numbers (req rate, error %, p95 or bandwidth) for that project. Keep it
  lightweight; if `<TopologyGraph>` doesn't already expose a hover hook, add a
  minimal one WITHOUT breaking its FEAT-037 contract (coordinate: a small
  additive prop is fine; don't rewrite the component).
- A legend explaining the color/thickness encoding.

## Out of Scope
- Per-container or per-internal-edge traffic (not measurable — see constraint).
- New backend metrics (uses the shipped C3 query API as-is).

## Affected Areas
- `frontend/src/components/topology/` (overlay util/hook; minimal additive
  hover prop on `<TopologyGraph>` if needed)
- `frontend/src/app/(main)/infrastructure/page.tsx` + `projects/[id]/map/page.tsx`
  (compute + pass decorations)

## Acceptance Criteria
- [ ] Project nodes are colored by their recent error rate (neutral when no traffic); no fabricated per-container data
- [ ] The Traefik→exposed-service ingress edge thickens with the project's request volume; internal edges stay at base thickness
- [ ] Overlay refreshes with the map; hook cleanup has no interval leak
- [ ] Hover shows per-project mini-stats (req rate, error %, and one of p95/bandwidth)
- [ ] A legend explains the encoding
- [ ] Wired on BOTH the global Infrastructure page and the per-project Map tab
- [ ] `npm run build` passes, no type errors, no new dependency; `<TopologyGraph>`'s FEAT-037 contract not broken

## Test Plan
Deploy a project, drive mixed traffic, open the map → its node colors by error
rate and its ingress edge thickens; a no-traffic project stays neutral. Covered
end-to-end by TEST-017.

## Implementation Notes

**Overlay Algorithm & Identification:**
- **Traefik Node**: Identified by `type === "proxy"` or `name.toLowerCase().includes("traefik")`
- **Ingress Edge**: An edge where one endpoint is the Traefik node and the other belongs to a project (project_id ≠ 0)
- **Node→Project→Metrics Mapping**: Uses `node.project_id` to fetch metrics from `getProjectMetrics(projectId)`; system nodes (project_id=0) remain neutral

**Error Rate Thresholds & Colors:**
- `<1%` (0.01): `hsl(142, 71%, 45%)` — Green (healthy)
- `1–5%` (0.01–0.05): `hsl(43, 96%, 56%)` — Yellow (warning)
- `≥5%` (0.05+): `hsl(0, 84.2%, 60.2%)` — Red (critical)
- Nodes with no metrics stay neutral (no fabricated data)

**Request Volume → Edge Thickness:**
- Base thickness: 2px, Max: 8px
- Scaling: `thickness = min(8, 2 + requestRate/20)` (e.g., 50 req/sec → 4.5px, 100+ → 8px)
- Only ingress edges (Traefik→service) are thickened; internal edges remain at base thickness

**Hover Mini-Stats (FEAT-039 AC):**
- **TopologyNodeStats interface** (keyed by node.id): `{ reqRate: number, errorPct: number, p95Ms?: number }`
- Extracted from latest metric points: req rate (req/s), error % (status_class.error_rate * 100), p95 latency (latency.p95 converted from seconds to milliseconds)
- Neutral (absent) for nodes with no traffic and core nodes (project_id=0)
- **buildNodeStats()** helper: extracts stats from MetricsPanels; returns null if any required metric is missing
- **TopologyGraph additive prop** (non-breaking): `nodeStats?: Record<string, TopologyNodeStats>` — added to TopologyGraphProps
- **Hover rendering**: Enriches SVG `<title>` element with traffic stats when available (req/s, error %, p95 ms)

**Implementation Files:**
1. **`frontend/src/components/topology/useTrafficOverlay.ts`** (updated):
   - TrafficOverlayResult now includes `nodeStats: Record<string, TopologyNodeStats>`
   - `useProjectTrafficOverlay()` & `useGlobalTrafficOverlay()` now return nodeStats alongside decorations
   - Helper functions: `getErrorRateColor()`, `getEdgeThickness()`, `findTraefikNode()`, `buildNodeStats()`, `buildProjectOverlay()`, `buildGlobalOverlay()`
   - Reuses fixed hook-cleanup pattern: `isMounted` flag + `clearInterval` in cleanup, polling every 8000ms

2. **`frontend/src/components/topology/TopologyGraph.tsx`** (updated):
   - Added TopologyNodeStats interface and `nodeStats?` prop to TopologyGraphProps
   - Updated node rendering to compute hover tooltip text that includes req rate, error %, and p95 latency when stats available
   - Maintains full FEAT-037 contract: all existing props unchanged

3. **`frontend/src/app/(main)/infrastructure/page.tsx`** (updated):
   - Now destructures `nodeStats` from `useGlobalTrafficOverlay()`
   - Passes `nodeStats` to `<TopologyGraph>` alongside existing decorations
   - Legend already explains color (error rate) and thickness (request volume) encoding

4. **`frontend/src/app/(main)/projects/[id]/map/page.tsx`** (updated):
   - Now destructures `nodeStats` from `useProjectTrafficOverlay()`
   - Passes `nodeStats` to `<TopologyGraph>` alongside existing decorations
   - Same legend as infrastructure page

**Build Verification:** `npm run build` passed with no type errors, no new dependencies, no linting errors. FEAT-037 contract fully preserved (existing props unchanged).

## Review Notes

**Verdict: CHANGES_REQUESTED**

### Blocking Issues

#### 1. Unmet Acceptance Criterion: Hover Mini-Stats
**Severity:** Blocker  
**AC Requirement (line 79):** "Hover shows per-project mini-stats (req rate, error %, and one of p95/bandwidth)"  
**Location:** All pages using TopologyGraph (infrastructure/page.tsx, projects/[id]/map/page.tsx)  
**Issue:** The AC explicitly requires hover tooltips showing request rate, error percentage, and one of p95 latency or bandwidth. The current implementation defers this to a "next phase" (Implementation Notes, line 124). TopologyGraph only provides a basic SVG `<title>` element with node name, type, project_id, and state — no traffic metrics whatsoever.  
**Why it matters:** This is a core AC, not a "nice to have." TEST-017's visual verification will fail if hovering over nodes doesn't reveal the promised metrics. The design constraint (Metric-granularity Reality) explicitly says the only traffic signal available is per-project, so metrics SHOULD be displayable on the nodes/edges that carry them.  
**How to fix:** Either:
  - Return metrics from `useProjectTrafficOverlay` and `useGlobalTrafficOverlay` alongside decorations, and pass them to TopologyGraph as an optional prop (non-breaking, FEAT-037-compatible). Add a hover handler in TopologyGraph to display a tooltip with req_rate, error_rate, and p95 or bandwidth from the latest metric point.
  - Or add a dedicated hover tooltip component that can fetch/subscribe to the same metrics.  
Whichever approach: this must be implemented, not deferred.

---

### Non-Blocking Issues

#### 2. Misleading Comment on Edge Thickness Formula
**Severity:** Minor (documentation only)  
**Location:** `frontend/src/components/topology/useTrafficOverlay.ts`, line 46  
**Issue:** The comment says `"50 req/sec -> ~6"` but the formula `2 + requestRatePerSec / 20` yields `2 + 50/20 = 4.5px`, not 6. The formula is correct per the Implementation Notes (line 104) and spec, but the inline comment is mathematically wrong.  
**How to fix:** Update the comment to reflect the actual formula behavior. Correct comment would be: `"0 req/sec -> 2, 50 req/sec -> ~4.5, 100+ req/sec -> ~7, 160+ req/sec -> 8 (clamped)"` or remove the specific numbers and just say "scales logarithmically."

---

### Verification: Passing Checks

✓ **Decoration key/shape match:** Keys correctly formatted as `nodeDecorations[node.id]` and `edgeDecorations["${source}:${target}:${network}"]`, matching TopologyGraph's exact lookup pattern (lines 132, 155).  

✓ **Granularity honored:** Only project nodes with `project_id ≠ 0` are colored; system nodes stay neutral. Only Traefik→service ingress edges are thickened; internal edges (app↔redis) remain at base thickness 2px. No fabricated per-container or per-internal-edge traffic.  

✓ **Ingress edge + Traefik identification:** 
  - Traefik found robustly by `type === "proxy"` or name containing "traefik" (case-insensitive, line 59).
  - Ingress edge correctly identified by checking both directions (source/target) in both `buildProjectOverlay` (lines 108–118) and `buildGlobalOverlay` (lines 161–179).
  - Tested edge case: reversed edges (e.g., app→traefik instead of traefik→app) are handled; the key `"${source}:${target}:${network}"` will differ but TopologyGraph will look up the same key.

✓ **Error-rate thresholds + color:** Correctly use fractional thresholds (0.01 = 1%, 0.05 = 5%), not percentages. HSL colors match spec. Nodes with no metrics stay neutral. Handles missing/empty status_class gracefully (checks `status_class.length`).  

✓ **Global overlay fan-out bounded:** 
  - `useGlobalTrafficOverlay` extracts unique project IDs from topology nodes only (lines 217–226), deduped via Set, sorted.
  - Fetches only those projects in parallel via `Promise.allSettled` (line 246), not an unbounded burst.
  - Polling interval set to 8000ms (line 272), matching topology refresh rate.
  - Cleanup: `isMounted` flag prevents setState after unmount (lines 241, 250, 263, 275); `clearInterval` called in cleanup (line 276). No interval leaks.

✓ **Legend present:** Both `infrastructure/page.tsx` (lines 64–89) and `projects/[id]/map/page.tsx` (lines 54–79) include legends explaining node color (error-rate thresholds) and edge thickness (request-volume). Legend text is clear and matches the color/thickness logic.

✓ **Hook cleanup:** Fixed pattern used consistently — `isMounted` flag + `clearInterval` in useEffect cleanup, interval set to 8000ms. `useProjectTrafficOverlay` uses `useProjectMetrics` hook with `refetchInterval` (line 195), relying on the hook's own cleanup (standard React pattern).

✓ **No new dependency:** All imports are from existing modules (`@/hooks/useMetrics`, `@/lib/api`, React, Typescript types). No new npm packages.

✓ **Build clean:** `npm run build` completed successfully with no type errors or warnings. All pages route correctly (18 routes generated, `/infrastructure` and `/projects/[id]/map` are among them).

✓ **FEAT-037 contract intact:** TopologyGraph props unchanged (still `nodeDecorations`, `edgeDecorations` optional, no new required props). Component remains a pure presentational component. No breaking changes to existing hover opacity effect or node/edge rendering.

✓ **Wired on both pages:** `useGlobalTrafficOverlay` called on infrastructure page (line 21), `useProjectTrafficOverlay` called on project map page (line 22), both passed to TopologyGraph correctly (lines 59–60, 49–50).

---

### Summary

The overlay logic is sound: decoration keys match, granularity is respected, ingress edges are correctly identified, thresholds are right, and the global hook is safe and bounded. Build passes, no regressions to FEAT-037.

**However, the core acceptance criterion for hover mini-stats is not met.** The task summary, Scope section, and AC all explicitly require "hover shows mini-stats" with traffic metrics, but the implementation provides only a basic SVG title and defers the feature. This must be implemented before the task can pass review. Suggest adding a small tooltip component or extending TopologyGraph with an optional `metrics` prop (non-breaking).

Once hover mini-stats are implemented, this task is ready for TEST-017 integration testing.


## Test Notes
<n/a — held for cluster integration test TEST-017>

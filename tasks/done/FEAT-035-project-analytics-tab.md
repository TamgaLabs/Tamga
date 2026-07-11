---
id: FEAT-035
type: feature
title: Per-project Analytics tab (project-scoped panels)
status: done
complexity: standard
assignee: sdlc-reviewer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C4 cluster (Analytics UI)"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (FEAT-034 shared controls now exist; reuses TimeRange/Resolution selectors)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "project Analytics tab reusing FEAT-033/034; build passes; reviewing"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "review PASS (project scoping correct, unix-sec, empty-state safe); holding for TEST-016"}
  - {date: 2026-07-11, stage: done, by: architect, note: "TEST-016 PASS; cluster C4 committed"}
---

**Part of:** C4-analytics
**Depends on:** FEAT-033

## Summary
An Analytics tab on the project detail view showing that one project's traffic
(`GET /api/projects/{id}/metrics`), reusing FEAT-033's panels — so a user can
see request rate, status/error, latency percentiles and bandwidth for their
own deployed stack, attributed via the C3 `project-<id>` router mapping.

## Scope
- Add an "Analytics" tab to the existing project detail tab set, rendering the
  four FEAT-033 panels against `GET /api/projects/{id}/metrics` for the current
  project id.
- Same range/resolution controls + refresh behaviour as FEAT-034 (reuse the
  shared controls rather than reimplementing).
- A project with no traffic yet → clean "no data" state (empty arrays, not an
  error) — important for freshly deployed projects.

## Out of Scope
- Global analytics (FEAT-034).
- The per-project infra map tab (C5).

## Affected Areas
- `frontend/src/` — project detail view (new tab) + reuse of FEAT-033/034 parts.

## Acceptance Criteria
- [ ] Project detail view has an Analytics tab rendering `GET /api/projects/{id}/metrics`
- [ ] Panels show that project's own traffic (correct per-project attribution)
- [ ] Range/resolution controls + refresh work, reusing the shared controls
- [ ] No-traffic project → clean empty state, no crash
- [ ] Build passes with no type errors

## Test Plan
Open a deployed project's Analytics tab after driving traffic, confirm its
panels reflect that project's traffic (and differ from another project's).
Covered end-to-end by TEST-016.

## Implementation Notes

**Files changed:**
- `frontend/src/app/(main)/projects/[id]/layout.tsx` — Added "Analytics" tab to the sections array (line 56)
- `frontend/src/app/(main)/projects/[id]/analytics/page.tsx` — New per-project analytics page

**Tab integration pattern:**
The Analytics tab was added following the existing project tab pattern. The project detail layout (`layout.tsx`) maintains a `sections` array that defines all available tabs. The Analytics entry routes to `/projects/{id}/analytics`, which is automatically picked up by Next.js's dynamic routing.

The new analytics page (`analytics/page.tsx`):
- Uses `useProjectContext()` to get the project ID (consistent with other tabs like environment, settings)
- Uses `useProjectMetrics(projectId, params, options)` hook to fetch per-project metrics (mirrors FEAT-034's global analytics structure but scoped to one project)
- Renders the four reusable panels: RequestRatePanel, StatusErrorPanel, LatencyPanel, BandwidthPanel (from `@/components/analytics`)
- Implements TimeRangeSelector and ResolutionSelector controls with same defaults as FEAT-034: 24h range, auto resolution, 60s refetch interval
- Handles empty state properly: when `metrics.request_rate.length === 0`, shows a clean "no metrics available" message instead of crashing (handles freshly deployed projects with no traffic)

**Discrepancies noted:**
None — the actual exported component APIs in `useProjectMetrics`, `TimeRangeSelector`, `ResolutionSelector`, and the four panels all matched the task description. No adjustments needed.

**Build result:**
✓ Compiled successfully with no type errors. Frontend build passed.

## Review Notes

**Verdict: PASS**

**Checked 2026-07-12 by code reviewer**

### Scope & Focus Verification

Confirmed implementation touches only the declared files:
- `frontend/src/app/(main)/projects/[id]/layout.tsx`: single line added to sections array (line 57)
- `frontend/src/app/(main)/projects/[id]/analytics/page.tsx`: new file created

(Note: working tree contains other uncommitted changes from FEAT-033, FEAT-034, and design refinements; these are not part of FEAT-035's scope per its Implementation Notes.)

### Acceptance Criteria - All Met

1. **✓ Analytics tab rendering project metrics**: Tab added to project detail nav (layout.tsx line 57), routes to `/projects/{id}/analytics` which renders the new page.tsx. Next.js build output confirms route registered.

2. **✓ Per-project attribution**: Line 19-20 of analytics/page.tsx uses `useProjectContext()` → `project.id`. Passed to `useProjectMetrics(projectId, ...)` at line 47. Hook calls `getProjectMetrics(projectId, params)` (frontend/src/lib/api.ts line 396), which constructs `/projects/${projectId}/metrics` endpoint. Correct project scoping confirmed.

3. **✓ Reuse of shared controls**: 
   - TimeRangeSelector and ResolutionSelector imported from `@/components/analytics` (lines 12-13), not reimplemented
   - useProjectMetrics hook imported from `@/hooks/useMetrics` (line 5), not reimplemented
   - All four panels imported from `@/components/analytics` (lines 8-11), not reimplemented
   - No duplicate logic detected

4. **✓ Time/resolution correctness**:
   - Timestamps initialized as unix SECONDS: `Math.floor(Date.now() / 1000)` (line 32) ✓
   - 24h window: `now - 86400` seconds (line 34) ✓
   - "auto" resolution omitted from query params: line 43 spreads resolution only if not "auto" ✓
   - Matches FEAT-034 pattern exactly

5. **✓ Empty state handling**:
   - Line 100-105: checks `!loading && metrics && metrics.request_rate.length === 0`
   - Renders clean "No metrics available" message with context
   - Individual panels also handle empty arrays gracefully (verified in RequestRatePanel, shows "No data available" overlay at line 90-93 of panel code)
   - No crash risk; proper UX for freshly deployed projects with no traffic

6. **✓ Tab integration pattern**:
   - Follows existing project detail tabs (Overview, Containers, Settings, Environment, Actions)
   - Uses same href/label structure in sections array
   - Active state determined by pathname match in layout (line 66)
   - Route segment path (`[id]/analytics/page.tsx`) matches href pattern

7. **✓ Cleanup & memory leaks**:
   - useProjectMetrics hook (useMetrics.ts lines 68-100) properly manages cleanup:
     - isMounted flag prevents state updates after unmount (lines 71, 76, 81, 85, 98-99)
     - setInterval cleared on unmount (line 95) when refetchInterval is set
     - Dependency array includes projectId (line 102), so switching projects clears old interval
   - No ref leaks; no subscription leaks

8. **✓ Design system adherence**:
   - Uses Tailwind utilities consistently (p-6, max-w-7xl, grid, gap-6, etc.)
   - Uses existing design system components (Card, Input from ui/ components)
   - Error state uses destructive/10 and destructive/30 color tokens
   - No inline styles or one-off CSS

9. **✓ No new dependencies**:
   - Only imports from existing @/hooks, @/components, @/lib
   - No new npm packages required
   - No breaking changes to existing APIs

10. **✓ Build verification**:
    - Ran `npm run build` in frontend/: "Compiled successfully"
    - No TypeScript errors or warnings
    - Route registered in build output: `ƒ /projects/[id]/analytics` (1.08 kB)

### Minor Observations (Non-Blocking)

- **Redundant empty messages**: When a project has no traffic, both the top-level page message ("No metrics available") and each individual panel's internal empty state ("No data available") will render. This is visually redundant but not incorrect. Matches FEAT-034's identical pattern, so consistent with codebase convention.

- **Dependency tracking**: useProjectMetrics dependency array tracks `params?.from, params?.to, params?.resolution` individually rather than the whole params object. This is correct and actually better practice (avoids unnecessary re-runs if object reference changes but values don't).

### Conclusion

Implementation correctly fulfills all acceptance criteria. Properly reuses FEAT-033 panels and FEAT-034 controls/hooks without duplication. Project scoping is correct. Empty state handling is clean and won't crash. Integrates smoothly into existing project detail tab pattern. Build passes with no errors. Ready for cluster integration test TEST-016.

## Test Notes
<n/a — held for cluster integration test TEST-016>

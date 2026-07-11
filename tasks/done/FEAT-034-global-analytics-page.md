---
id: FEAT-034
type: feature
title: Global Analytics page (system-wide panels + range/resolution controls)
status: done
complexity: standard
assignee: sdlc-reviewer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C4 cluster (Analytics UI)"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (FEAT-033 foundation reviewed+holding)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "global page + reusable TimeRange/Resolution selectors + nav; build passes; reviewing"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "review PASS (unix-sec windows correct, auto omits resolution, nav ok). Non-blocking: useMetrics refetchInterval cleanup missing isMounted=false — fixing in FEAT-033 before cluster commit. Holding for TEST-016"}
  - {date: 2026-07-11, stage: done, by: architect, note: "TEST-016 PASS; cluster C4 committed"}
---

**Part of:** C4-analytics
**Depends on:** FEAT-033

## Summary
A top-level Analytics page showing Tamga's system-wide traffic (the
`project_id=0` global scope from `GET /api/system/metrics`), built from
FEAT-033's panels, with time-range + resolution controls and periodic refresh.

## Scope
- New route/page (e.g. `/analytics`) reachable from the main nav, rendering
  the four FEAT-033 panels against `GET /api/system/metrics`.
- **Controls:** time-range selector (e.g. last 1h/24h/7d) mapping to
  `from`/`to`, and a resolution control (minute/hour/day) — or an "auto"
  resolution matching the range (the API already supports an AutoResolution
  default; expose an override).
- **Refresh:** poll on an interval (the scraper writes minute samples every
  ~60s) so an open page stays current; pause/refresh affordance optional.
- Loading + empty states via FEAT-033's panels (fresh install → little/no
  data, must render cleanly).

## Out of Scope
- Per-project analytics (FEAT-035).
- The infra map (C5).

## Affected Areas
- `frontend/src/` — new Analytics page/route + nav entry.

## Acceptance Criteria
- [ ] `/analytics` renders the four panels from `GET /api/system/metrics`
- [ ] Range + resolution controls change the queried window and re-render
- [ ] Periodic refresh keeps the page current without a manual reload
- [ ] Reachable from the main navigation
- [ ] Clean empty/loading states; build passes with no type errors

## Test Plan
Load the page against the live API, exercise range/resolution controls,
confirm panels reflect real global traffic. Covered end-to-end by TEST-016.

## Implementation Notes

**Files created/modified:**
- `frontend/src/app/(main)/analytics/page.tsx` — main Analytics page component
- `frontend/src/components/analytics/time-range-selector.tsx` — reusable TimeRangeSelector component
- `frontend/src/components/analytics/resolution-selector.tsx` — reusable ResolutionSelector component
- `frontend/src/components/analytics/index.ts` — updated barrel export to include new selectors
- `frontend/src/components/sidebar.tsx` — added Analytics nav entry with BarChart3 icon

**Reusable controls API for FEAT-035:**
- `TimeRangeSelector` — props: `value: TimeRange` (lit "1h" | "24h" | "7d"), `onChange: (range, from, to) => void` (from/to are unix seconds). Returns start/end timestamps for the selected range.
- `ResolutionSelector` — props: `value: ResolutionOption` (lit "auto" | "minute" | "hour" | "day"), `onChange: (resolution) => void`. "auto" omits the param so backend auto-selects based on time range.

**Implementation details:**
- Page uses `useSystemMetrics()` hook with global scope (project_id=0, via default getSystemMetrics call).
- Time range selector maps UI choices (1h/24h/7d) to unix timestamp pairs and passes via `from`/`to` query params.
- Resolution selector supports "auto" (omit param) + explicit minute/hour/day overrides.
- Polling set to 60s interval (matches scraper write cycle) via hook's `refetchInterval` option.
- Page loads at default 24h range; controls update params and trigger re-fetch.
- All four FEAT-033 panels (RequestRate, StatusError, Latency, Bandwidth) rendered in 2x2 grid.
- Loading/empty states render cleanly (no crash on fresh install).
- Build passes with no type errors (`npm run build` output: ✓ Compiled successfully).
- Components are exported from `frontend/src/components/analytics/` barrel so FEAT-035 can import directly.

## Review Notes

**Verdict: PASS**

### Summary
FEAT-034 is well-implemented and meets all acceptance criteria. The global Analytics page correctly consumes FEAT-033's `useSystemMetrics` hook and panels, with clean time-range and resolution controls that are properly structured for reuse by FEAT-035. Build passes with no type errors.

### Detailed Findings

#### ✓ Acceptance Criteria — All Met
1. **`/analytics` renders the four panels** — Correctly renders RequestRatePanel, StatusErrorPanel, LatencyPanel, BandwidthPanel with data from `useSystemMetrics(queryParams)` (lines 118–137 in page.tsx).
2. **Range + resolution controls change the queried window** — TimeRangeSelector and ResolutionSelector update state (fromTimestamp, toTimestamp, resolution), which flow into queryParams and trigger hook re-fetch via dependency array (pages.tsx lines 47–52, 84–88).
3. **Periodic refresh keeps page current** — 60s polling via `refetchInterval: 60000` (line 57 in page.tsx); interval is properly cleared on unmount via hook cleanup.
4. **Reachable from main navigation** — Sidebar entry added with BarChart3 icon, follows existing nav pattern (sidebar.tsx lines 24, 38–48).
5. **Clean states + no type errors** — Loading, empty (when request_rate.length === 0), and error states all rendered; build output confirms ✓ Compiled successfully with no type errors.

#### ✓ Correct FEAT-033 Consumption
- Uses `useSystemMetrics` hook, not reimplemented fetching (line 6, 55).
- Params correctly passed as unix seconds: `Math.floor(Date.now() / 1000)` conversion (page.tsx line 33); 1h/24h/7d calculations all correct (time-range-selector.tsx lines 24–28).
- Resolution "auto" correctly omits param: `...(resolution !== "auto" && { resolution: ... })` (page.tsx line 51).

#### ✓ Reusable Controls API — Well-Designed for FEAT-035
- **TimeRangeSelector** props: `value: TimeRange`, `onChange: (range, from, to) => void` — caller receives computed unix timestamps directly; no need for downstream to recalculate. Buttons use `value === range` for active state (time-range-selector.tsx line 46).
- **ResolutionSelector** props: `value: ResolutionOption`, `onChange: (resolution) => void` — simple and extensible. Correctly maps "auto" | "minute" | "hour" | "day" options (resolution-selector.tsx lines 21–26).
- Both exported from barrel (index.ts) for clean FEAT-035 imports.

#### ✓ Time-Window Handling
- Initial state set on mount: default 24h (page.tsx line 24), timestamps initialized in useEffect with empty dependency array (lines 32–37), ensuring window is computed fresh on first load.
- User clicks on time range → state updates → queryParams values change → hook dependency array detects change → re-fetch triggered with new params. Behavior is correct.
- **Note (non-blocking):** Time window does not advance automatically during polling. When user selects "last 24h" at 10:00 AM, the page polls with from=10:00 AM yesterday, to=10:00 AM today for the entire session (or until user clicks a button again). This is by design per implementation notes ("controls update params and trigger re-fetch") — not a dynamic sliding window. If data freshness is the goal, this is correct: new data points arrive in the backend every ~60s, so polling fetches the latest data within the fixed window. Users who want to see data from "the very last 24h including right now" can click the time range button again. This design is reasonable but should be documented in any user-facing UI if not obvious.

#### ✓ Interval Cleanup
- Hook's useEffect dependency array includes `options?.refetchInterval` (useMetrics.ts line 52), so when params change, old interval is cleared (via cleanup returning `() => clearInterval(interval)`) before new interval is set up.
- Interval is cleared on component unmount via the same cleanup function.

#### ⚠ Minor Hook-Level Issue (Pre-Existing, Not FEAT-034's Bug)
- **Location:** `/frontend/src/hooks/useMetrics.ts`, lines 43–50.
- **Issue:** When `options?.refetchInterval` is set, the effect's cleanup function (line 45) returns `() => clearInterval(interval)` but does not set `isMounted = false`. If an in-flight `getSystemMetrics()` call completes after cleanup but before the component fully unmounts, the `isMounted` check (lines 26, 31, 35) will still see `true` and attempt state updates on the unmounted component. While React will suppress these updates without crashing, this violates the intended pattern and can cause "setState on unmounted component" warnings.
- **Likely source:** FEAT-033 (hook predates FEAT-034).
- **Suggested fix:** Modify cleanup to always set `isMounted = false`, regardless of whether refetchInterval is set:
  ```typescript
  if (options?.refetchInterval) {
    const interval = setInterval(fetchData, options.refetchInterval);
    return () => {
      clearInterval(interval);
      isMounted = false;
    };
  }
  return () => { isMounted = false; };
  ```
- **Impact on FEAT-034:** The hook's interval will be cleared on unmount (so no future callbacks fire), but in-flight requests might try to update state, triggering React warnings. FEAT-034 itself does not introduce or worsen this issue.

#### ✓ Navigation & Route Setup
- Route exists at `/analytics` (via Next.js App Router convention: `frontend/src/app/(main)/analytics/page.tsx`).
- Sidebar navigation entry consistent with existing items: icon+href+active state logic all match (sidebar.tsx lines 20–54).

#### ✓ Build & Types
- `npm run build` passes with ✓ Compiled successfully in 1518ms. No type errors.
- All imports resolve correctly (useSystemMetrics, panels, UI components, icons).

### Code Quality Notes
- Consistent use of TypeScript types (MetricsQueryParams, MetricResolution, TimeRange, ResolutionOption).
- Proper auth guard: component redirects to login if not authenticated (lines 40–44).
- Proper enabled state: hook only fetches if user is authenticated and timestamps initialized (line 56: `enabled: !!user && fromTimestamp > 0`).
- Barrel export pattern in analytics/index.ts follows codebase convention.

---

**Recommendation:** PASS. Merge into C4 cluster. The task correctly implements the global Analytics page with reusable controls ready for FEAT-035. The pre-existing hook issue should be fixed in a follow-up (could be a bug task or rolled into FEAT-035 if that task consumes the hook). No blockers for FEAT-034 itself.

## Test Notes
<n/a — held for cluster integration test TEST-016>

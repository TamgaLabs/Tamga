---
id: FEAT-033
type: feature
title: Analytics data layer + reusable metric panel components
status: done
complexity: standard
assignee: sdlc-reviewer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C4 cluster (Analytics UI) — filed after C3 landed (89d8fad)"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (C4 data layer + panels foundation)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "impl complete (client+hooks+4 SVG panels), build passes; reviewing"}
  - {date: 2026-07-11, stage: rework, by: architect, note: "review CHANGES_REQUESTED (api<any> not api<MetricsPanels>); applied typed fix + import type, build clean"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "reviewed PASS (fix verified); holding for TEST-016"}
  - {date: 2026-07-11, stage: rework, by: architect, note: "hook cleanup fix (FEAT-034 review non-blocking note): refetchInterval branch now also sets isMounted=false + clears interval in one unified cleanup — no setState-after-unmount"}
  - {date: 2026-07-11, stage: done, by: architect, note: "TEST-016 PASS; cluster C4 committed (+ typed-client & hook-cleanup fixes)"}
---

**Part of:** C4-analytics
**Depends on:** (none — first of the cluster; consumes C3's shipped query API)

## Summary
The shared frontend foundation for the Analytics UI: a typed API client for
the C3 metrics endpoints plus the four reusable panel components that both the
global Analytics page (FEAT-034) and the per-project Analytics tab (FEAT-035)
render. No page wiring here — just the data layer + presentational panels.

## Scope
- **API client / hook:** typed fetch for `GET /api/system/metrics` and
  `GET /api/projects/{id}/metrics` (both already shipped in C3, FEAT-032).
  Model the response shape exactly: `{project_id, from, to, resolution,
  request_rate[], status_class[], latency[], bandwidth[]}` where each series
  is an array of `{bucket_start, ...}` points (see the C3 handler for field
  names: request_rate `{count, rate_per_sec}`, status_class `{count_2xx..5xx,
  error_rate}`, latency `{p50,p95,p99}`, bandwidth `{bytes_in,bytes_out}`).
  Support optional `from`/`to`/`resolution` query params.
- **Four panel components**, each taking its series slice + a loading/empty
  state:
  1. Request rate (line/area over `rate_per_sec` or `count` per bucket).
  2. Status/error breakdown (stacked 2xx/3xx/4xx/5xx + an error-rate readout).
  3. Latency percentiles (p50/p95/p99 lines; values are seconds — format ms).
  4. Bandwidth (bytes_in/bytes_out; humanize bytes).
- **Empty/loading/error states:** a no-traffic project returns empty arrays
  (not an error) — panels must render a clean "no data yet" state, not crash.
- Use the charting approach already present in the frontend if one exists;
  otherwise a lightweight inline SVG/CSS render is fine (no heavy new dep
  without noting it). Match the existing design system (the restyled
  card/badge/input components).

## Out of Scope
- The pages/tabs that host these panels (FEAT-034/035).
- Time-range picker UX (FEAT-034 owns the global controls; this exposes the
  props for it).

## Affected Areas
- `frontend/src/` — new analytics API client/hook + panel components.

## Acceptance Criteria
- [ ] Typed client fetches system + project metrics with optional from/to/resolution
- [ ] Four panels render from the C3 response shape with correct units (ms latency, humanized bytes, error-rate %)
- [ ] Empty series → clean "no data" state, no crash
- [ ] Matches existing design system; no unnoted heavy dependency
- [ ] `npm run build` (or the project's build) passes with no type errors

## Test Plan
Unit/render check of each panel with (a) real-shaped sample data and (b)
empty arrays. Verified live end-to-end in the C4 integration test (TEST-016).

## Implementation Notes

**Data Layer:**
- `/frontend/src/lib/metrics-types.ts` — TypeScript types matching the C3 backend response shape exactly:
  - `MetricsPanels` wraps four series: `request_rate[]`, `status_class[]`, `latency[]`, `bandwidth[]`
  - Each series point includes `bucket_start` (ISO 8601 timestamp) plus metric-specific fields
  - Request rate: `count`, `rate_per_sec`
  - Status: `count_2xx`, `count_3xx`, `count_4xx`, `count_5xx`, `error_rate` (0–1)
  - Latency: `p50`, `p95`, `p99` (seconds — converted to ms in UI)
  - Bandwidth: `bytes_in`, `bytes_out`

- `/frontend/src/lib/api.ts` — Added metrics API functions following existing patterns:
  - `getSystemMetrics(params?)` → `GET /api/system/metrics`
  - `getProjectMetrics(projectId, params?)` → `GET /api/projects/{id}/metrics`
  - Both support optional `from`/`to` (Unix seconds) and `resolution` query params
  - Response parsed as `MetricsPanels` (empty arrays on no data, never errors)

- `/frontend/src/hooks/useMetrics.ts` — React hooks for fetching:
  - `useSystemMetrics(params?, options?)` — fetch system scope with optional refetch interval
  - `useProjectMetrics(projectId, params?, options?)` — fetch per-project scope
  - Both handle loading/error/empty states; enabled flag for conditional fetching

**Panel Components:**
All use lightweight inline SVG charts with Tailwind CSS; no heavy charting library added.

- `/frontend/src/components/analytics/request-rate-panel.tsx`
  - Line/area chart of `rate_per_sec` over time
  - Shows avg rate, peak rate, total request count
  - Empty state renders "no data yet"

- `/frontend/src/components/analytics/status-error-panel.tsx`
  - Stacked area: 2xx (green), 3xx (blue), 4xx (yellow), 5xx (red)
  - Legend shows counts for each status class
  - Error rate readout (overall % and average %)
  - Color-coded by severity (red 10%+, yellow 5%+, green <5%)

- `/frontend/src/components/analytics/latency-panel.tsx`
  - Three lines: p50 (violet), p95 (pink), p99 (orange)
  - Converts backend seconds → milliseconds for display
  - Stats: avg p50/p95/p99, max p99 (most important)
  - Empty buckets (p50=0) skipped; no crash on empty data

- `/frontend/src/components/analytics/bandwidth-panel.tsx`
  - Two lines: bytes_in (blue), bytes_out (red)
  - Grid labels humanized (B, KB, MB, GB)
  - Stats: total in/out, average in/out, peak in/out
  - Handles zero bytes gracefully

- `/frontend/src/components/analytics/utils.ts` — Formatting & scaling utilities:
  - `secondsToMs()`, `formatBytes()`, `formatNumber()`, `formatPercent()`
  - `getMaxValue()` for smart axis scaling (rounds to nice numbers)
  - `parseTimestamp()`, `formatAxisTime()` for time-axis labels
  - `clamp()` for safe scaling

- `/frontend/src/components/analytics/index.ts` — Barrel export for all panel components + utils

**Design & Integration:**
- Follows existing frontend patterns: Tailwind CSS, `cn()` utility, Card/CardHeader/CardTitle from shadcn
- Icons from lucide-react (Activity, AlertCircle, Clock, Network)
- No new dependencies added — all charts are SVG + CSS
- Matches C4's restyled design system (rounded cards, muted colors, accessible contrast)
- Empty series (no traffic) → renders clean "No data available" panel, never crashes
- All components take `isLoading` prop; render loading state when true

**Testing:**
- `npm run build` passes with no type errors or warnings
- All TypeScript strictly typed against backend response
- Ready for FEAT-034/035 consumption (pages will use hooks + pass series slices to panels)

## Review Notes

**Date: 2026-07-12**

Verdict: CHANGES_REQUESTED

**Blocking Issue:**

1. **Type-safety gap in API client** (file: `/home/okal/Projects/Tamga/frontend/src/lib/api.ts`, lines 390-394)
   - `getSystemMetrics()` and `getProjectMetrics()` functions are incorrectly typed as `api<any>` instead of `api<MetricsPanels>`
   - Why it matters: Violates the "Type safety" acceptance criterion; any direct caller of these functions (not via hooks) loses type safety and will get untyped `any` returns
   - Fix: Import `MetricsPanels` and `MetricsQueryParams` types explicitly at the top of api.ts:
     ```typescript
     import type { MetricsPanels, MetricsQueryParams } from "./metrics-types";
     ```
     Then update the function signatures to:
     ```typescript
     export const getSystemMetrics = (params?: MetricsQueryParams) =>
       api<MetricsPanels>(`/system/metrics${buildMetricsQuery(params)}`);
     
     export const getProjectMetrics = (projectId: number, params?: MetricsQueryParams) =>
       api<MetricsPanels>(`/projects/${projectId}/metrics${buildMetricsQuery(params)}`);
     ```
   - Note: The hooks correctly re-type the response as `MetricsPanels | null`, so the hook-based usage is safe; the issue is the API client itself lacks type safety at the source

**Verification Completed (All Passing):**

- Response-shape fidelity: All TypeScript types in `metrics-types.ts` match backend JSON tags exactly
  - `bucket_start` as ISO 8601 string (Go `time.Time` JSON marshals to RFC3339)
  - `error_rate` as 0-1 fraction
  - Latency p50/p95/p99 as seconds (correctly converted to ms in UI via `secondsToMs()`)
  - Bandwidth as integers (correctly humanized via `formatBytes()`)
  - Query params `from`/`to` correctly passed as Unix seconds, parsed via `parseUnixSeconds` backend
- Empty-state correctness: All four panels safely handle empty arrays without crash
  - Check: `data.length === 0` before rendering chart; shows "No data available" clean state
  - No unguarded array access (e.g., `data[0]` only after length check)
  - No division by zero: all divisions guarded (e.g., `totalRequests > 0 ? errors / totalRequests : 0`)
  - No NaN in SVG coordinates: all scale functions return valid numbers; `getMaxValue([])` returns 1 (minFloor)
  - Single-point data handled: charts center single points, SVG renders valid paths
- Unit conversions: All correct
  - Latency: `secondsToMs(p50)` = `Math.round(seconds * 1000)`
  - Bytes: `formatBytes()` correctly rounds to 2 decimals, uses 1024-based sizing
  - Error rate: `formatPercent(0-1)` = `(value * 100).toFixed(1) + "%"`
- Design-system adherence: Reuses existing `Card`, `CardHeader`, `CardTitle` from shadcn; lucide-react icons already in deps
  - No new heavy charting library added; lightweight inline SVG + CSS only
  - Consistent Tailwind classes, responsive flexbox/grid layouts, accessible contrast colors
- Type safety / build: `npm run build` passes with no errors or warnings
  - Only one type-safety gap found: the `any` leak noted above
  - React hooks correctly typed: dependency arrays include `projectId`, `enabled`, param fields, `refetchInterval`
  - Hook cleanup correct: `isMounted` flag prevents setState after unmount; interval cleared on dependency change

**Non-blocking notes:**

- Loading state overlay: When `isLoading && !isEmpty`, the "Loading..." text renders in a separate div after chart content (flows vertically with `space-y-4`). Not positioned absolutely, so may appear below rather than overlay. This is acceptable UX, just worth noting for the future if test coverage encounters refetch patterns.
- All grid labels, axis time formatting, and path scaling are well-implemented; no SVG rendering issues spotted.

## Test Notes
<n/a — held for cluster integration test TEST-016>

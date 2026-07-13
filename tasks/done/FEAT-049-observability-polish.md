---
id: FEAT-049
type: feature
title: "[C1] Analytics and infrastructure UI refresh"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-12
history:
  - {date: 2026-07-12, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned after C1 shell review PASS; FEAT-048 remains blocked by TEST-021"}
  - {date: 2026-07-13, stage: review, by: architect, note: "observability polish submitted for standard review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-020 integration"}
  - {date: 2026-07-13, stage: test-pass, by: architect, note: "TEST-020 C1 integration verified"}
  - {date: 2026-07-13, stage: done, by: architect, note: "C1 cluster complete"}
---

## Summary
Make global and project observability surfaces read as one polished console,
with clearer metric controls, graph context, states, and responsive layout.

## Requirements
- **Part of:** C1 Tamga Console refresh
- **Cluster Test:** TEST-020
- **Depends on:** FEAT-043
- Refresh global/project Analytics headers, range/resolution controls, panels,
  legends, loading/no-metrics/error states, and panel layout with shadcn
  components.
- Refresh global/project Infrastructure pages and topology surrounding chrome:
  controls, overlay legend, loading/error states, and graph container sizing.
- Preserve existing graph interactions, auto-refresh/traffic-overlay behavior,
  metric queries, domain binding, and container click-through.

## Out of Scope
- Changes to metrics/topology APIs, chart calculations, graph layout engine,
  or domain-binding behavior.

## Proposed Solution / Approach
Use the reviewed shared PageHeader and existing shadcn Card, Badge, Empty, and
Skeleton primitives to make analytics and topology controls/states consistent.
Keep the existing data hooks, selectors, SVG/chart code, and topology graph
props intact; limit changes to surrounding layout, responsive overflow, and
status presentation.

## Affected Areas
- `frontend/src/app/(main)/analytics/page.tsx`
- `frontend/src/app/(main)/infrastructure/page.tsx`
- `frontend/src/app/(main)/projects/[id]/analytics/page.tsx`
- `frontend/src/app/(main)/projects/[id]/map/page.tsx`
- `frontend/src/components/analytics/**`
- `frontend/src/components/topology/**`

## Acceptance Criteria / Definition of Done
- [ ] Global and per-project observability retain all existing data, controls,
      graph navigation, and domain-binding workflows.
- [ ] No-data, loading, and error states are distinguishable and visually
      aligned with the shared design system.
- [ ] Charts and map containers remain usable at desktop and narrow widths
      without clipping controls or legends.
- [ ] Legend/status color meaning remains accessible in light and dark modes.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Run `npm run build` in `frontend`; browser-test global and project analytics
range/resolution controls, no-data/error paths, topology refresh, node
click-through, traffic overlay legend, domain binding, and narrow-width layout.

## Implementation Notes
- Refreshed system and project analytics with shared headers, grouped responsive
  range/resolution controls, refresh context, alert, skeleton, and no-data
  states. Existing metric query parameters and polling are unchanged.
- Updated chart card spacing and replaced the duplicate below-chart loading
  message with an in-card refresh badge; chart data, legends, and calculations
  remain unchanged.
- Refreshed both topology pages and overlay legends with shared Card/Badge/
  Skeleton patterns. The graph retains its overlay and click-through props;
  its narrow-width canvas now uses intentional horizontal overflow instead of
  clipping controls or labels.

## Review Notes
Reviewer appends.

### 2026-07-13 — PASS
- Reviewed the four analytics/topology pages and chart/graph diffs. Metric query
  state, 60s/8s polling, topology overlay hooks, domain/project binding,
  graph decorations, and node click routes are unchanged; no API, layout-engine,
  or graph-calculation changes were introduced.
- The refresh uses existing shadcn Card, Badge, Skeleton, Empty, and shared
  PageHeader patterns. Responsive selector wrapping and intentional horizontal
  graph overflow preserve controls and labels at narrow widths; alerts use
  `role="alert"` and the existing button controls retain accessible labels.
- Static checks: `npm run lint`, `npm run test:unit -- --run`, and
  `git diff --check` PASS.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run lint` + `npm run build` PASS | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — static review, lint, unit tests, diff check | n/a | n/a | 0 |

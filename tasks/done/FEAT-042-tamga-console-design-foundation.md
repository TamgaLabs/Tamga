---
id: FEAT-042
type: feature
title: "[C1] Tamga Console shadcn design foundation"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-12
history:
  - {date: 2026-07-12, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned as next eligible C1 foundation task after C3 completion"}
  - {date: 2026-07-13, stage: review, by: architect, note: "Tailwind/design foundation submitted for standard review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires actual responsive/keyboard/stateful shadcn Sidebar primitive"}
  - {date: 2026-07-13, stage: review, by: architect, note: "stateful shadcn Sidebar rework submitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-020 integration"}
  - {date: 2026-07-13, stage: test-pass, by: architect, note: "TEST-020 C1 integration verified"}
  - {date: 2026-07-13, stage: done, by: architect, note: "C1 cluster complete"}
---

## Summary
Create the shared design foundation for Tamga Console so every later route
migration uses the same shadcn tokens, typography, states, and primitives.

## Requirements
- **Part of:** C1 Tamga Console refresh
- **Cluster Test:** TEST-020
- **Depends on:** TEST-019
- Upgrade the Next.js frontend to Tailwind CSS **4.3** before applying the
  visual system. Follow Tailwind's official Next.js/PostCSS installation:
  use `@tailwindcss/postcss` and `@import "tailwindcss"`; do not add the
  Vite-only `@tailwindcss/vite` plugin.
- Move the existing Tailwind 3 configuration deliberately to v4's CSS-first
  model: retain every semantic design token, dark-mode behavior, breakpoint,
  animation and utility dependency that the application uses; replace the
  legacy config only after its responsibilities have been accounted for.
- Adopt and document shadcn's current `nova` preset after the v4 migration.
  Apply generated theme/font/component diffs selectively, never as a blanket
  overwrite of ambient WIP.
- Preserve the existing `new-york` component contract unless an audited v4
  registry migration provides an equivalent reviewed replacement.
- Install `geist`; configure Geist Sans (body), Geist Mono (code), and the
  official Geist Pixel display variant. Use Pixel Square for the product word
  mark/short display headings only, never small UI copy.
- Rename title/description and shared product identity to Tamga Console.
- Add only the needed current shadcn primitives, including Sidebar,
  Breadcrumb, Tooltip, Skeleton, Sonner/Toast, Sheet, Table, Empty, and
  appropriate form field/input-group support. Merge any generated change with
  local component edits rather than replacing them.
- Standardize semantic success/warning/info tokens and shared page header,
  async-state, empty-state, and destructive-feedback patterns.

## Out of Scope
- Route-specific information architecture and page redesigns.
- Replacing the existing API error handling strategy.

## Proposed Solution / Approach
Replace the Tailwind 3/PostCSS configuration with Tailwind 4.3's CSS-first
theme mapping, preserving semantic and sidebar tokens. Use the official local
`geist` package for Sans, Mono, and Pixel Square, and add the requested shadcn
compatible primitives individually so existing local component edits remain
intact.

## Affected Areas
- `frontend/components.json`
- `frontend/package.json`, `frontend/package-lock.json`
- `frontend/src/app/layout.tsx`, `frontend/src/app/globals.css`
- `frontend/tailwind.config.ts`
- `frontend/src/components/ui/**`
- New shared presentational components under `frontend/src/components/**`

## Acceptance Criteria / Definition of Done
- [ ] The frontend builds on Tailwind CSS 4.3 using the official Next.js/PostCSS
      integration; no Vite plugin is present.
- [ ] Tailwind 3 configuration responsibilities have a v4 CSS-first equivalent
      and generated diffs do not overwrite ambient WIP without an intentional merge.
- [ ] A named shadcn preset and its safe application method are documented.
- [ ] Tamga Console metadata and display wordmark use Geist Pixel Square;
      body and code typography remain readable and distinct.
- [ ] Shared semantic tokens work in both light and dark modes.
- [ ] Required current shadcn primitives are present, typed, and usable by
      later tasks without duplicate local implementations.
- [ ] Shared loading, empty, error, destructive, and page-header patterns are
      available and accessible.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Run `npm run build` in `frontend` after the v4 migration. In a browser,
verify type loading, light/dark contrast, keyboard focus, toast feedback, and
shared states on a representative protected route. Review the shadcn CLI diff
before applying any generated component update.

## Implementation Notes
- Replaced the legacy Tailwind plugin/config with `@tailwindcss/postcss` and a
  CSS-first `@theme inline` mapping for semantic, sidebar, typography, radius,
  animation-compatible and code tokens; removed `tailwind.config.ts`.
- Set the shadcn config to CSS-first with the `nova` preset reference while
  retaining the existing `new-york` style contract; documented selective
  primitive adoption in `frontend/docs/design-system.md`.
- Added Geist Sans, Mono, and official Pixel Square wordmark typography;
  changed shared metadata and the main sidebar product label to Tamga Console.
- Added composable Breadcrumb, Tooltip, Skeleton, Sonner, Sheet, Table, Empty,
  Field, InputGroup, Sidebar, and shared PageHeader primitives; semantic
  success/warning/info tokens now have light and dark values.
- Replaced the initial Sidebar wrapper with an audited shadcn-compatible
  provider/state API: controlled and persisted desktop state, responsive
  Radix Sheet navigation on mobile, Ctrl/Cmd+B toggle, trigger/rail controls,
  focusable menu buttons, tooltip support, and labelled navigation semantics.
  The existing app shell remains untouched for FEAT-043 to adopt.

## Review Notes
Reviewer appends.

### 2026-07-13 — CHANGES_REQUESTED

`npm run build` in `frontend` passes with Tailwind 4.3/PostCSS, and the CSS-first
tokens, dark-mode mapping, Geist imports/metadata, Nova documentation, Sonner,
and the other claimed primitive files are present. The compiled output also
contains the legacy `tailwindcss-animate` utilities/keyframes, so that v3
dependency was carried forward successfully. I treated concurrent edits to
login/code/card/badge/input and unrelated repository files as ambient WIP.

The required Sidebar primitive is not a current shadcn Sidebar implementation:
`frontend/src/components/ui/sidebar.tsx` is a minimal local aside/menu wrapper;
its `open` state is never consumed and it omits the shadcn mobile/keyboard
interaction, trigger/rail, responsive sheet, cookie/state handling, and
accessible navigation contract. The application continues to render the older
separate `frontend/src/components/sidebar.tsx`, so later route work cannot use
the requested shadcn Sidebar foundation. Replace the minimal wrapper with the
current, selectively merged shadcn Sidebar implementation (or document and
implement an audited equivalent with its responsive/accessibility behaviour),
without overwriting ambient sidebar changes. No browser runtime was run.

### 2026-07-13 — PASS

The rework provides the required reusable Sidebar foundation without changing
the existing application shell (correctly deferred to FEAT-043): controlled and
uncontrolled desktop state writes the seven-day `sidebar_state` cookie, Ctrl/Cmd+B
toggles it, and the public context/composition API exposes trigger, rail,
inset, groups and menu-button `asChild` support. Mobile rendering is isolated
in the typed Radix Sheet with an accessible title/description; desktop controls
have accessible labels and visible-focus classes, and menu tooltips compose via
the shared provider. `npm run build` passes. No browser runtime was run.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run build` PASS | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED — Sidebar is not the required current shadcn accessible/responsive primitive | n/a | n/a | 1 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | `npm run build` PASS — Sidebar rework | n/a | n/a | 1 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — reusable stateful Sidebar foundation and `npm run build` | n/a | n/a | 1 |

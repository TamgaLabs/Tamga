---
id: FEAT-048
type: feature
title: "[C1] Code and terminal workspace UI refresh"
status: approved
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-12
history:
  - {date: 2026-07-12, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "TEST-021 complete; C2 dependency cleared, dispatched to sdlc-developer"}
  - {date: 2026-07-13, stage: development-complete, by: developer_standard, note: "code/terminal workspace polish with shadcn components; lint+build PASS"}
  - {date: 2026-07-13, stage: review, by: architect, note: "code/terminal polish submitted for standard review"}
  - {date: 2026-07-13, stage: review, by: sdlc-reviewer, note: "re-review: Separator import fix confirmed, no remaining issues. Verdict: APPROVED"}
---

## Summary
Polish the codebase list and editor/terminal workspace without disturbing
Monaco or persistent terminal-session behavior.

## Requirements
- **Part of:** C1 Tamga Console refresh
- **Cluster Test:** TEST-020
- **Depends on:** FEAT-043, TEST-021
- Refresh `/code` with clear codebase selection, system-codebase state,
  loading/empty/error feedback, and responsive item patterns.
- Improve editor page hierarchy, files panel, terminal-tab strip, action
  grouping, save/error feedback, and terminate confirmation using shadcn
  components around—not inside—the Monaco/xterm surfaces.
- Preserve session tab caps, reattach behavior, file editing semantics, and
  system codebase read-only restrictions.

## Out of Scope
- Editor features, file API changes, or changes inside Monaco/xterm rendering.
- Terminal shell ergonomics and terminate-tab correctness, which C2 owns and
  verifies before this visual workspace refresh.

## Proposed Solution / Approach
Apply the C1 console polish pattern (PageHeader, Empty, Skeleton, Badge,
Card) to the `/code` list and `/code/[id]` editor/terminal workspace. Use
shadcn primitives _around_ the Monaco and xterm surfaces without touching
their internals. Key changes:

- **Code list** (`code/page.tsx`): Replace raw `<h1>` and `<p>` loading/
  empty states with PageHeader, Skeleton loading grid, structured Empty
  states (loading, error, empty), and Card grid with Badge for
  system-vs-project type.
- **Editor workspace** (`code/[id]/page.tsx`):
  - Replace plain mode buttons with shadcn Tabs (Terminal / Code).
  - Wrap the entire page in TooltipProvider for consistent hover hints.
  - Terminal tab strip: add Tooltip on close/new buttons, visible-on-hover
    close affordance, role/aria attributes for tab semantics.
  - Terminal empty state: replace inline message with Empty component.
  - File tree panel: use ScrollArea for overflow, button elements with
    aria-labels, dimmed opacity for read-only system codebases.
  - Editor action bar: group save (with Save icon) and close, show
    "Unsaved" Badge when dirty.
  - Editor empty state: use Empty component, contextual messaging.
  - System codebases: show "Read-only" Badge in top bar, set Monaco
    `readOnly` option, disable openFile/writeFile paths.
  - Save feedback: use sonner toast for loading/success/error instead of
    only inline banner.

## Affected Areas
- `frontend/src/app/(main)/code/**`
- `frontend/src/components/agent-terminal.tsx`
- supporting code/editor shared UI

## Acceptance Criteria / Definition of Done
- [ ] Codebase selection, file-tree behavior, save feedback, and read-only
      system codebase restrictions remain functional.
- [ ] Terminal tab creation, selection, detach/reattach, and termination
      preserve existing session behavior and confirmations.
- [ ] Editor controls are keyboard accessible and responsive without
      overlapping content.
- [ ] Loading, empty, and API error states are visible and actionable.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Run `npm run build` in `frontend`; browser-test codebase selection, tree/file
open, successful and failed save, read-only behavior, multi-tab terminal
creation/termination, and narrow-width layout.

## Implementation Notes
- **Files changed:** `code/page.tsx`, `code/[id]/page.tsx`
- **New imports used:** Badge, ScrollArea, Tooltip*, Tabs*,
  Empty*, Lock icon, Save icon, sonner `toast`. All already available in
  the codebase; no new dependencies.
- **No changes** to `agent-terminal.tsx`, `terminal-tabs.ts`, Monaco
  config, or xterm setup.
- **Preserved:** session tab caps (MAX_TERMINAL_SESSIONS=10), reattach
  semantics, file editing workflow, system codebase type distinction,
  terminate confirmation dialog, showFileTree toggle, offline-mode
  terminal fallback.
- **Pattern alignment:** follows dashboard/containers/infrastructure C1
  patterns — PageHeader with title+description+actions, Skeleton for
  loading, Empty with media/title/description/content for all terminal
  states, structured Card grid with Badge variants.
- Lint and build pass (`npm run lint`, `npm run build`).

## Review Notes
Reviewer: sdlc-reviewer (big-pickle). Date: 2026-07-13.

### Verdict: CHANGES_REQUESTED

### Findings

**1. Unused import — `Separator` (line 21, `code/[id]/page.tsx`)**
`Separator` is imported from `@/components/ui/separator` but never used in the template. No other page in `(main)` imports it. Remove the import to keep the file clean.

**2. All other items pass — detailed walkthrough below.**

#### Pattern alignment with other C1 tasks
Code list (`code/page.tsx`) matches the dashboard pattern exactly:
- `PageHeader` with title + description + actions (count badge)
- `Skeleton` loading grid with `aria-busy` and `aria-label`
- `Empty` error state with `AlertCircle`, error message, retry button
- `Empty` empty state with `FolderOpen` icon and messaging
- Card grid with `CardHeader` (flex-row items-start), `Badge` variants for system/project, `CardContent` for path

Editor page (`code/[id]/page.tsx`) applies shadcn components around—not inside—Monaco/xterm surfaces as required.

#### Acceptance criteria walk
| # | Criterion | Status |
|---|-----------|--------|
| 1 | Codebase selection, file-tree, save feedback, read-only system codebase restrictions preserved | ✅ `isReadOnly` guard on `openFile` and `handleSave`; Monaco `readOnly` option; "Read-only" Badge in top bar; file tree `opacity-60` for read-only items |
| 2 | Terminal tab creation, selection, detach/reattach, termination preserved | ✅ No changes to `agent-terminal.tsx` or `terminal-tabs.ts`; `mergeTerminalTabs`, `removeTerminalTab`, pending/dedup logic all unchanged; `AlertDialog` for terminate confirmation intact |
| 3 | Editor controls keyboard accessible and responsive | ✅ File tree items converted from `<div>` to `<button>` with `aria-label`, `tabIndex={0}`; terminal tabs have `role="tab"`, `aria-selected`, `onKeyDown`; code list cards wrapped in `<button>`; responsive grid `md:grid-cols-2 lg:grid-cols-3` |
| 4 | Loading, empty, API error states visible and actionable | ✅ Loading skeleton with `aria-busy`; error `Empty` with retry button; empty states for codebases, terminal sessions, and editor; `role="alert"` on terminal and save error banners |
| 5 | KISS/YAGNI | ✅ No speculative abstractions; straightforward shadcn component usage matching existing patterns |

#### Correctness
- `Badge variant="warning"` exists in `badge.tsx` (line 20) — used for system codebase and "Unsaved" badge.
- `toast` from `sonner` used for save loading/success/error — consistent with containers page pattern.
- `TerminalTab` dedup and `terminatedSessionIds` ref logic unchanged and correct.
- `handleTerminateTab` functional updater pattern for `activeTabId` preserved.
- Session cap (`MAX_TERMINAL_SESSIONS = 10`) preserved.

#### Scope
- Only `code/page.tsx` and `code/[id]/page.tsx` changed per spec.
- No changes to `agent-terminal.tsx`, `terminal-tabs.ts`, Monaco config, or xterm setup.

### Required fix
~~Remove the unused `Separator` import from `code/[id]/page.tsx` line 21 before merge.~~ — **Resolved 2026-07-13.**

---

**Re-review (2026-07-13):** Confirmed `Separator` import removed from `code/[id]/page.tsx`. No other issues found. All 5 acceptance criteria pass. Pattern alignment excellent. Verdict upgraded to **APPROVED**.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | rework |
|---|---|---|---|---|---|---|

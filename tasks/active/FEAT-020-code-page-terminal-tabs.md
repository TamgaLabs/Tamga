---
id: FEAT-020
type: feature
title: Code page terminal tabs — multiple persistent sessions, reattach, terminate; files sidebar open by default
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created from TEST-008 findings §6; depends on FEAT-015"}
---

## Summary
The code page's terminal is a single anonymous session that is fully torn
down whenever the component unmounts — even just switching to the Code
tab and back (TEST-008 §6). With FEAT-015's backend session manager in
place, the frontend gets real terminal tabs: multiple named sessions per
project that survive navigation and browser close, a reattach flow, and
explicit terminate. The files sidebar also opens by default. Depends on
FEAT-015 (must land first).

## Requirements
- Terminal tab bar in the code page's terminal mode: one tab per session,
  "+" to open a new session (respecting the backend's 10-session cap —
  surface its error cleanly), an inline close/terminate control per tab
  with a confirm step (terminate kills the session for real, per
  FEAT-015; there is no "just close the tab locally" — the tab list
  mirrors the server's session list).
- On entering the code page, fetch the project's existing sessions
  (FEAT-015's list endpoint) and show them as tabs; attaching to one
  replays its scrollback (backend does the replay — the frontend just
  renders the stream into a fresh xterm instance).
- Switching between terminal tabs, or to Code mode and back, must NOT
  terminate sessions — detach/reattach (or keep sockets alive in
  component state) instead of today's unmount-teardown
  (agent-terminal.tsx:60-65). Choose and document one approach in
  Proposed Solution.
- Files sidebar: `showFileTree` default `true` (code/[id]/page.tsx:44).
- Keep Monaco/theme integration working (theme comes from FEAT-017's
  resolved value if that has landed; otherwise current useTheme).
- BUG-023's fix (system codebase visibility) is separate — don't touch
  the codebase listing here.

## Out of Scope
- Backend session mechanics (FEAT-015).
- Editor feature work beyond the sidebar default.
- Renaming sessions, per-tab titles beyond a simple index/short id.

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] Opening the code page shows existing sessions as tabs; a fresh project shows zero tabs plus "+"
- [ ] "+" opens a new live session; multiple tabs work independently
- [ ] Switching tabs, switching to Code mode and back, navigating away and returning, and a full browser reload all preserve sessions and their scrollback
- [ ] Terminate (with confirm) removes the tab and the server session; terminating the last one stops the sandbox (FEAT-015 behavior, observed via docker ps)
- [ ] The 11th session attempt shows the backend's cap error in the UI, not a silent failure or crash
- [ ] Files sidebar is visible by default when entering Code mode
- [ ] `npx tsc --noEmit` and `npm run build` pass
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Browser flow on a real project: open 3 sessions, run distinguishable
commands in each, switch tabs/modes, reload the browser, verify each
tab's scrollback; terminate one (sandbox stays), terminate the rest
(sandbox stops, verified via docker ps); attempt 11 sessions; confirm
files sidebar default with a fresh profile/localStorage.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

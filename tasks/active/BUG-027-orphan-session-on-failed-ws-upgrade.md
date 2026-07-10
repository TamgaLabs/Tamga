---
id: BUG-027
type: bug
title: Terminal session orphaned when CreateSession succeeds but the WS upgrade then fails
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-09
history:
  - {date: 2026-07-09, stage: created, by: architect, note: "surfaced by FEAT-020's review as an out-of-scope backend follow-up"}
---

## Summary
In the terminal handler's `Serve`, a session is created
(`CreateSession`) BEFORE the WebSocket `Upgrade` completes. If the
upgrade fails (or the client aborts the handshake), the created session
is never attached and never cleaned up — it lingers in the registry as an
orphan: invisible to the user until a reload, counting against the
per-project 10-session cap, and keeping the sandbox alive. FEAT-020's
frontend races (rapid "+"/mode-switch) can trigger this, but it's a
backend robustness gap independent of the UI.

## Steps to Reproduce
1. Open a WS to `/api/projects/{id}/agent/terminal` (no session param) and
   abort the connection during/just after the HTTP upgrade handshake
   (e.g. close the socket immediately, or trigger an upgrade failure).
2. `GET /api/projects/{id}/agent/sessions` — an extra session is listed
   that no client is attached to.
3. Repeat until the list hits 10; further legitimate opens now 429 even
   though no usable session exists.

## Expected Behavior
A session created for a connection whose upgrade/attach never completes is
cleaned up (terminated, decrementing the cap and stopping the sandbox if
it was the last), so orphans can't accumulate.

## Actual Behavior
The session persists indefinitely until an unrelated terminate or backend
restart.

## Environment / Context
`backend/internal/handler/terminal_handler.go` `Serve` — the ordering of
`CreateSession` vs `Upgrade`, and the absence of a cleanup path on the
upgrade-failure / early-disconnect branches. Cross-reference FEAT-015's
session registry (`agent_service.go`, `terminal_session.go`) for the right
terminate/cleanup call. Consider: create the session only after a
successful upgrade, OR register a deferred cleanup that terminates the
session if it never transitions to attached.

## Root Cause
<filled in by developer>

## Proposed Solution
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria
- [ ] The reproduction steps above no longer orphan a session
- [ ] A failed/aborted upgrade leaves the session count unchanged (verify via the sessions list + the cap still admits 10 real sessions afterward)
- [ ] If the orphan was the only session, the sandbox does not stay running
- [ ] Normal create/attach/reattach/terminate flows (FEAT-015) still pass their acceptance checks — no regression
- [ ] `go build/vet/test` pass; a regression test covers the failed-upgrade cleanup if practical

## Test Plan
Scripted probe that opens the WS and aborts at/after the upgrade;
assert the sessions list is unchanged and the sandbox stops; then a
loop of abort×15 must not exhaust the cap. Re-run FEAT-015's session
suite for regression.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

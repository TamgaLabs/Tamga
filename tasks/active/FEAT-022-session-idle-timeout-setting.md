---
id: FEAT-022
type: feature
title: Detached terminal session idle-timeout setting (default Never)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-09
history:
  - {date: 2026-07-09, stage: created, by: architect, note: "task created per user request refining FEAT-015 — sessions persist by default, but an idle timeout must be configurable; depends on FEAT-015 (and FEAT-017 for UI placement)"}
---

## Summary
FEAT-015 makes detached terminal sessions live until explicitly
terminated. The user wants that persistence configurable: a Settings
option "detached session timeout" — how long a session may sit with no
attached WebSocket before the backend terminates it automatically.
Default is **Never** (per user decision: reopening the web UI should
show the project's still-living sessions). Selectable finite values give
users who don't want stray sessions a self-cleaning option.

## Requirements
- Setting storage: global (not per-project), same single-row settings
  pattern used elsewhere (`resource_limits`, and FEAT-016's
  `egress_settings` if landed — reuse the established pattern, don't
  invent a new one). Value: nullable/zero = never, else a duration.
- Enforcement in the session manager (FEAT-015's registry): track
  last-detach time per session (a session with an attached WS is never
  idle); a periodic sweep (e.g. 1min ticker) terminates sessions
  detached longer than the timeout using the SAME terminate path as
  explicit termination (so last-session-stops-sandbox semantics hold).
- Timeout changes apply to already-detached sessions on the next sweep
  (no restart needed).
- API: expose get/set of the setting via the settings-style endpoints
  consistent with existing ones.
- Settings UI: a control under Settings > Sandbox (the FEAT-017
  sub-route; if FEAT-017 hasn't landed yet when this is picked up, put
  it in the current settings page's sandbox/resource area and FEAT-017
  will carry it) — options like Never (default), 30m, 1h, 8h, 24h (a
  simple select; exact preset list is the developer's call, must include
  Never and it must be default).
- Fresh installs default to Never; migration default Never.

## Out of Scope
- Per-project overrides.
- Persisting sessions or timers across backend restarts (in-memory
  registry dies with the backend — accepted in FEAT-015).
- FEAT-020's tab UI (it just benefits: expired sessions vanish from the
  list).

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] Default (Never): a detached session survives well past any preset (verify ≥ a few minutes with sweep running, plus code trace)
- [ ] With timeout set (use a short test value): a detached session is auto-terminated after the idle period; an ATTACHED session is not, regardless of age
- [ ] Auto-termination of a project's last session stops its sandbox (same as explicit terminate)
- [ ] Setting changes take effect without backend restart
- [ ] Setting round-trips through the API and survives backend restart (stored in DB)
- [ ] UI control shows/sets the value; Never is the default on a fresh install
- [ ] `go build/vet/test` + `npx tsc --noEmit` pass; sweep/idle logic unit-tested where practical
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Set timeout to a short value via API/UI; open a session, detach, verify
auto-terminate + sandbox stop after the period (docker ps + session list
evidence); repeat with an attached session (survives); set Never, detach,
verify survival past the previous period; restart backend, verify the
setting persisted.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

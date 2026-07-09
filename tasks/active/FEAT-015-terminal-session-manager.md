---
id: FEAT-015
type: feature
title: Backend terminal session manager — persistent bash sessions with reattach, terminate, cap and auto-stop
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created from TEST-009 findings §1/§2/§3/§6"}
---

## Summary
Today a terminal is one WS connection = one anonymous docker exec running
`/bin/sh`; closing the browser tab kills the stream and (at zero
connections) stops the sandbox, with no history and no way to reattach
(TEST-009 §1). This task builds a real session manager: named sessions
running bash that survive WS disconnect with scrollback, explicit
terminate, a per-project cap, and sandbox auto-stop when the last
*session* (not connection) ends.

## Requirements
- Sandbox image runs bash: add `RUN apk add --no-cache bash` to
  `deploy/Dockerfile.agent` (verified absent, ~1.6s cost, TEST-009 §3)
  and change the single call site `agent_service.go:369` from `/bin/sh`
  to `/bin/bash`.
- Session registry (in-memory in `AgentService` or a new service):
  per-project sessions with id, exec/PTY handle, scrollback ring buffer
  (pick a sane fixed size, e.g. 256KB per session), created-at. Replaces
  `connCount map[string]int` (TEST-009 §1 — that map is the entire
  current "registry").
- Sessions survive WS disconnect: on WS close, keep the exec's stdio
  hijack alive server-side (keep pumping output into the ring buffer);
  the shell process must keep running. Note Docker's Exec API cannot kill
  a single exec (TEST-009 §1.3) — terminate must kill the shell process
  (e.g. by PID inside the container via a second exec, or by closing the
  PTY stdin and signaling) — design this explicitly in Proposed Solution.
- Reattach: a WS connection can attach to an existing session by id; on
  attach, replay the ring buffer, then stream live.
- REST surface (authenticated, same style as existing routes): list
  sessions for a project (id, created_at, connected/detached), create is
  implicit via WS connect with no session id, terminate by id.
- Cap: max 10 concurrent sessions per project, enforced at session
  creation (natural point: where `StartSandbox`/registry insert happens,
  TEST-009 §6); a clean error to the client when exceeded, not a 500.
- Sandbox auto-stop: when a project's last session is *terminated*
  (explicitly), stop the sandbox via existing `StopAgent`. A mere WS
  disconnect no longer stops anything.
- Locking: replace the single service-wide `s.mu` held across the slow
  stop (TEST-009 §1 timing findings) with per-project locking so one
  project's teardown doesn't block another's session open. (BUG-025 —
  the 10s stop delay itself — is a separate task; don't fix the timeout
  here, but don't hold a global lock across it either.)
- Frontend compatibility: the existing WS endpoint keeps working —
  extend the protocol/URL (e.g. `?session=<id>`) rather than breaking
  the current client; the multi-tab UI lands separately in FEAT-020.
- Keep in mind TEST-009's unreproduced-500s note (two 500s on
  /agent/terminal under real browser use): make the session-open path
  robust to rapid open/close/reopen races — this redesign should make
  that class of failure structurally impossible, and the Test Plan must
  include a rapid open/close/reopen probe.

## Out of Scope
- The multi-tab terminal UI, terminate buttons, tab state (FEAT-020).
- BUG-025's stop-delay fix (separate simple task; land order doesn't
  matter, they touch different lines).
- Idle-timeout/garbage-collection policies for detached sessions — a
  detached session lives until explicitly terminated, per the user's
  decision. (If backend restart loses in-memory sessions, that's
  acceptable and should just be documented.)
- Persisting sessions across backend restarts (the dead agent_sessions
  table stays dead).

## Proposed Solution / Approach
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria / Definition of Done
- [ ] New terminal sessions run `/bin/bash` (verify via `ps` in the sandbox)
- [ ] Closing the WS does NOT stop the shell process or the sandbox (verify via `docker top` + reattach)
- [ ] Reattaching to a session replays prior output (run a command, disconnect, reconnect, see the output)
- [ ] Explicit terminate kills the shell process and removes the session from the list
- [ ] Terminating a project's last session stops its sandbox container; terminating one of several does not
- [ ] An 11th concurrent session on one project is rejected with a clean, client-visible error
- [ ] Two sessions on the same project are independent (input to one doesn't appear in the other)
- [ ] Rapid open/close/reopen loop (>=20 iterations) produces no 500s and no leaked sessions/containers
- [ ] Sessions on project A are unaffected while project B's sandbox is being stopped (locking)
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass; new logic has unit tests where practical (ring buffer, registry, cap)
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Against the compose stack with a rebuilt agent image: scripted WS probes
(same technique as TEST-009's audit probes) covering each criterion above
— run command / disconnect / reattach / verify replay; terminate flows;
cap rejection; the 20x open/close loop; `docker ps`/`docker top` evidence
for sandbox lifecycle claims.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

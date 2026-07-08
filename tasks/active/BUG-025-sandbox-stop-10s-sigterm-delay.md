---
id: BUG-025
type: bug
title: Sandbox container stop takes ~10s after terminal WS closes (tail PID 1 ignores SIGTERM + default docker stop timeout)
status: pending
complexity: simple
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: sdlc-developer, note: "found during TEST-009's live WS-close probe; filed separately per that task's instructions rather than fixed inline"}
---

## Summary
When a terminal WebSocket connection closes (last session for a project),
`AgentService.ReleaseSandbox` -> `StopAgent` calls `docker.StopContainer`,
which is coded and logged as if it happens immediately. In reality every
sandbox stop takes ~10 seconds in practice, because `deploy/Dockerfile.agent`'s
`CMD ["tail", "-f", "/dev/null"]` runs as the container's PID 1 and never
installs a `SIGTERM` handler (a well-known Docker "PID 1 problem" — the
kernel does not apply the default terminate action to an unhandled signal
sent to PID 1), and `ContainerStop` is called with a zero-value
`container.StopOptions{}`, i.e. Docker's default 10-second grace period
before it escalates to `SIGKILL`. This directly affects SPRINT-003's
planned "sandbox auto-stop when the last session ends" — every stop (on
disconnect, on explicit terminate, or when auto-stop fires) will visibly
lag by ~10s, and a user who closes and immediately reopens a terminal
within that window can race a container that's still mid-teardown.

## Steps to Reproduce
1. Open a project's terminal (creates `agent-<projectID>`).
2. Close the WebSocket (close the tab, or send a clean WS close frame).
3. Watch `docker ps` / backend logs for the container to actually stop.

## Expected Behavior
`ReleaseSandbox`'s log line ("agent container stopped") and the container
actually disappearing from `docker ps` should follow the WS close within
about a second — the code path is written as a synchronous stop-then-remove
with no expectation of a multi-second wait baked in anywhere.

## Actual Behavior
Confirmed live twice:
- Against the running dev stack (`tamga-backend-1` / `agent-23`),
  `docker logs tamga-backend-1` repeatedly shows the terminal WS's access
  log line completing (`... /agent/terminal ... - 000 0B in Ns`) at the
  same moment as `msg="agent container stopped"`, with `N` in the
  10-13s range even though the client closed almost immediately
  (confirmed the shell exec (`/bin/sh` on `pts/0`, visible via
  `docker top agent-23`) stays alive server-side for that whole window —
  it is not killed by closing the hijacked stream, only by the eventual
  container `SIGKILL`).
- Isolated, reproducible timing test (own tmp DB/data dir/random port
  backend instance, own throwaway `local` project, a real WS client
  script sending a command then closing cleanly): container created at
  `19:57:15.912`, `agent container stopped` logged at `19:57:26.089` —
  10.177s later, even though the WS client's close call completed at
  essentially T+0.3s.
- Isolated from the app entirely: `docker run -d tamga-agent` then timed
  `docker stop` directly — **10189ms**, confirming the delay is not
  specific to the Go code's request handling at all, it's Docker's own
  stop grace period being exhausted every single time.

## Environment / Context
- `deploy/Dockerfile.agent:8`: `CMD ["tail", "-f", "/dev/null"]` — PID 1,
  no signal trap.
- `backend/internal/repository/docker/client.go:84-85`:
  ```go
  func (c *Client) StopContainer(ctx context.Context, containerID string) error {
  	return c.cli.ContainerStop(ctx, containerID, container.StopOptions{})
  }
  ```
  `container.StopOptions{}` zero value means the Docker daemon's default
  timeout (10s) applies before it escalates from `SIGTERM` to `SIGKILL`.
- Call chain: `terminal_handler.go:80` (`defer h.agentSvc.ReleaseSandbox`)
  -> `agent_service.go:339` `ReleaseSandbox` -> `agent_service.go:358`
  `s.StopAgent(ctx, projectID)` -> `agent_service.go:400`
  `s.docker.StopContainer(ctx, containerName)`.

## Root Cause
<filled in by developer>

## Proposed Solution
<filled in by developer — candidates: pass a short explicit
`container.StopOptions{Timeout: &shortSeconds}` (e.g. 2s) so `SIGKILL`
fires quickly instead of waiting the 10s default; and/or give
`Dockerfile.agent`'s `CMD` real signal handling (e.g. `exec tail -f
/dev/null` still won't trap `TERM` — `tail` itself would need to be
replaced with something that does, or run under `--init`/tini so the
default disposition applies) so a plain `SIGTERM` is enough and no
timeout needs to be waited out at all>

## Affected Areas
`backend/internal/repository/docker/client.go` (`StopContainer`),
possibly `deploy/Dockerfile.agent`'s `CMD`.

## Acceptance Criteria
- [ ] A terminal WS close results in `agent container stopped` being
      logged, and the container actually gone from `docker ps`, within
      ~1-2s of the close (not ~10s)
- [ ] `docker stop` on a freshly-run `tamga-agent` container (no exec
      attached) completes quickly, not in ~10s
- [ ] No change in behavior for a sandbox with an actively-running shell
      command at stop time (still torn down, just faster)

## Test Plan
`docker run -d --name bugtest tamga-agent && time docker stop bugtest &&
docker rm -f bugtest` — before the fix this reliably takes ~10.1-10.2s;
after the fix it should complete in a couple seconds or less. Also
re-run the live WS-close probe (open a terminal, close it, time how long
until the container disappears from `docker ps`/the "agent container
stopped" log line appears).

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

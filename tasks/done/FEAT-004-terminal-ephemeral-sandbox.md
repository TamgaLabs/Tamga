---
id: FEAT-004
type: feature
title: Remove ACP bridge, add WebSocket PTY terminal with ephemeral sandbox lifecycle
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-001
created: 2026-07-04
history:
  - {date: 2026-07-04, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-04, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-04, stage: in-development, by: architect, note: "first sdlc-developer pass interrupted mid-task by user but left nearly-complete implementation; second pass audited it, fixed a break-in-switch bug in terminal_handler.go, verified go build/vet + frontend tsc"}
  - {date: 2026-07-04, stage: in-review, by: architect, note: "moved to review"}
  - {date: 2026-07-04, stage: changes-requested, by: architect, note: "reviewer found 2 real concurrency bugs in StartSandbox/ReleaseSandbox refcounting (race between release-stop and concurrent start; container-create race with no locking); sent back for fix"}
  - {date: 2026-07-04, stage: in-review, by: architect, note: "sdlc-developer fixed both races (lock held across full sequences), scoped ?token= fallback to terminal path only, added WS ping/pong keepalive; architect verified no re-entrant locking/deadlock risk; back to review"}
  - {date: 2026-07-05, stage: in-test, by: architect, note: "re-review PASSED; architect fixed one narrow non-blocking missed writeMu site; moved to test"}
  - {date: 2026-07-05, stage: done, by: architect, note: "test PASSED end-to-end (real WS PTY session, resize, concurrent-connection refcounting, ephemeral teardown, ACP endpoints gone); pre-existing DATA_DIR mount bug found and filed as BUG-006; moved to done"}
---

## Summary
The current agent interaction model is chat/task-polling over ACP
(Agent Client Protocol) JSON-RPC, wired through `agent-server/server.js`
(Node.js) and `backend/internal/service/acp_bridge.go`, and only speaks to
OpenCode (`opencode acp`). architecture.md instead wants a real web terminal:
the user opens a WebSocket-backed xterm.js terminal into an on-demand sandbox
container and runs whatever agent CLI they want by hand (`claude`, `codex`,
`opencode`, `gemini`). This task removes the ACP model entirely and replaces
it with that terminal, including making the sandbox container's lifecycle
ephemeral (created when the terminal opens, destroyed when it closes).

This task intentionally bundles what plan.md lists as two "next" steps
(#4 real terminal, #6 ephemeral lifecycle) because the lifecycle hooks
(create-on-connect, destroy-on-disconnect) are the natural place the
WebSocket handler lives — splitting them would mean writing the same
connection handler twice.

## Requirements
- Remove: `agent-server/server.js`, `backend/internal/service/acp_bridge.go`,
  the chat/task/ACP handlers in `code_handler.go`, the chat UI components in
  `frontend/src/app/(main)/code/[id]/page.tsx`, and any domain/repository
  code that exists solely to back task/session tables for the chat model
  (check before deleting — some tables may be reused elsewhere)
- New endpoint: `GET /api/projects/:id/agent/terminal` — WebSocket, backed by
  Docker SDK `ContainerExecCreate` / `ContainerExecAttach` (+ resize) to
  start a shell/PTY inside the sandbox container and proxy
  stdin/stdout/resize over the socket
- Sandbox container is created on demand when this WebSocket connection
  opens (adapt the existing `ensureContainerRunning` logic in
  `agent_service.go` rather than rewriting it from scratch), and stopped +
  removed when the WebSocket closes (wire into the existing `StopAgent` path)
- Remove the 30-minute idle-watcher (`startIdleWatcher`) entirely, or reduce
  it to a simple "no active terminal connection" cleanup — since lifecycle
  is now tied to the WebSocket, a separate idle timer is likely redundant;
  use judgment and document the choice in Implementation Notes
- Frontend: integrate xterm.js, open the WebSocket, wire terminal resize
  events through to the backend's resize call
- Backend does not need to know about any specific agent CLI or protocol
  anymore — it only proxies a shell

## Out of Scope
- Bundling additional CLIs into the sandbox image — see FEAT-005
- Egress whitelist / network isolation — see FEAT-006
- Resource limits on the sandbox container — see FEAT-007
- Git credential injection into the sandbox — see FEAT-008 (this task can
  leave a clear injection point/TODO for where FEAT-008 will hook in, but
  should not implement git credential handling itself)
- `agent_provider.go` field cleanup (Command/Endpoint/AuthToken semantics
  post-ACP) — see BUG-001, which depends on this task landing first

## Proposed Solution / Approach
Replace the entire ACP chat/task-polling model with a WebSocket PTY proxy,
using `docker/docker` SDK's exec API rather than any new terminal library:

- **Backend**: `GET /api/projects/{id}/agent/terminal` upgrades to a
  WebSocket (gorilla/websocket - the one new dependency this task adds).
  The handler asks `AgentService.StartSandbox` to resolve the project's
  agent provider, ensure the sandbox container is running (reusing the
  existing `ensureContainerRunning`), and register the connection; it then
  calls `ContainerExecCreate`/`ContainerExecAttach` (new thin wrappers on
  the docker `Client`) to start `/bin/sh` as a PTY and proxies bytes both
  ways. Wire protocol: server -> client is raw binary WS frames (shell
  output, no envelope needed); client -> server is JSON text frames
  (`{"type":"input","data":...}` / `{"type":"resize","cols":...,"rows":...}`)
  since the input direction needs to distinguish keystrokes from resize
  events. `AgentService.ReleaseSandbox` is called when the WebSocket closes
  (both normal close and the shell exiting), decrementing a per-container
  connection refcount and calling the existing `StopAgent` (stop + remove)
  once it hits zero - this makes multiple terminal tabs against the same
  project safe without adding real complexity.
- **Idle watcher**: removed entirely (not "reduced") - lifecycle is now
  fully driven by WebSocket connect/disconnect via the refcount above, so a
  time-based idle sweep would be redundant, not complementary.
- **Auth**: the browser `WebSocket` API can't set an `Authorization` header,
  so `AuthMiddleware` gained a fallback to a `?token=` query param, used only
  by the terminal endpoint.
- **Removed wholesale**: `agent-server/` (Node ACP server), `acp_bridge.go`
  (the Go->ACP HTTP bridge), `domain/acp.go` (ACP protocol types, already
  dead), the chat/task/session handlers in `code_handler.go` and the
  standalone `agent_handler.go`, and the `agent_tasks`/`agent_sessions`
  repository code + domain types. Their SQL migrations are left in place
  (migrations are treated as immutable history; the tables just go unused)
  rather than adding a drop migration, since nothing else reads them.
- **Frontend**: the chat/session UI in `code/[id]/page.tsx` is replaced by
  an `AgentTerminal` component (`@xterm/xterm` + `@xterm/addon-fit`) that
  opens the WebSocket, feeds `onData` to it as `input` messages, and sends
  `resize` messages on a `ResizeObserver`. The "system" pseudo-codebase
  (id 0, for browsing Tamga's own source) keeps file browsing but has no
  Terminal tab, since it has no project row/provider to resolve a sandbox
  from and the task scopes the endpoint to real projects.
- Left out of scope per the task: git credential injection (marked with a
  `TODO(FEAT-008)` at the exact spot in `AgentService.StartSandbox` where
  it would go), egress/network isolation, resource limits, and
  `agent_provider.go` Command/Endpoint/AuthToken cleanup.

## Affected Areas
- `agent-server/` (removed)
- `backend/internal/service/acp_bridge.go` (removed)
- `backend/internal/service/agent_service.go`
- `backend/internal/handler/code_handler.go`
- `backend/internal/router/router.go`
- `deploy/Dockerfile.agent` (CMD changes — coordinate with FEAT-005; a
  minimal `sleep infinity`/`tail -f /dev/null` init process replaces the
  `agent-server` CMD)
- `frontend/src/app/(main)/code/[id]/page.tsx`

## Acceptance Criteria / Definition of Done
- [ ] Opening the terminal for a project creates a sandbox container if none is running
- [ ] User can type into the browser terminal and see live shell output (PTY behavior: line editing, colors, resize all work)
- [ ] Closing the terminal (tab close or explicit disconnect) stops and removes the sandbox container
- [ ] No ACP/chat code paths remain reachable from the UI or API
- [ ] User can run `opencode`, or any CLI present in the current sandbox image, directly from the terminal
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Open a project's agent terminal in the browser, confirm a new container
appears (`docker ps`), type shell commands and confirm output streams back,
resize the browser window and confirm the PTY resizes, close the terminal
and confirm the container is removed (`docker ps` no longer lists it).
Confirm no requests hit any removed ACP endpoints.

## Implementation Notes
Picked up mid-flight from a prior interrupted pass that had already done
nearly all of the implementation described above (ACP removal, `docker/client.go`
exec wrappers, `AgentService.StartSandbox`/`ReleaseSandbox`/`OpenShell`/
`AttachShell`/`ResizeShell`, `terminal_handler.go`, router/main.go wiring, the
`?token=` query-param auth fallback, and the `AgentTerminal` frontend
component using `@xterm/xterm` + `@xterm/addon-fit`). This pass audited that
work against every Requirement and Acceptance Criterion and made one
correctness fix:

- **Bug fix**: in `terminal_handler.go`'s browser->shell read loop, the
  `case "input"` branch's `break` on a failed `hijacked.Conn.Write` was
  inside a `switch` nested in the `for` loop, so it only broke out of the
  `switch`, not the loop - a dead shell stdin pipe would silently keep the
  read loop spinning forever instead of tearing down the connection. Fixed
  with a labeled loop (`readLoop:` / `break readLoop`), matching the style of
  the existing unlabeled `break` used for the `ReadMessage` error case just
  above it.

Everything else from the prior pass checked out against the codebase as
already correct and matching the Proposed Solution, and did not need
changes:
- `agent_service.go`: idle watcher is fully gone (not just reduced); refcount
  based `StartSandbox`/`ReleaseSandbox` correctly wraps the existing
  `ensureContainerRunning`/`StopAgent`; `TODO(FEAT-008)` injection point is
  present and correctly placed before container creation.
- `router.go`/`main.go`: `GET /api/projects/{id}/agent/terminal` is registered
  inside the authenticated route group and `NewTerminalHandler` is
  constructed with the real `AgentService` (which itself holds the real
  Docker client).
- `middleware.go`: `AuthMiddleware` falls back to `?token=` only when the
  `Authorization` header is empty, as required for the browser WebSocket API.
- No leftover references anywhere to `acp_bridge`, `domain.ACP`,
  `agent_handler.go`, `AgentSessionRepo`/`AgentTaskRepo`, or `agent-server/`
  (grepped the whole backend and frontend tree - all clean; SQL migrations
  for the old tables were correctly left in place per the "immutable
  history" decision).
- `deploy/Dockerfile.agent` CMD is `tail -f /dev/null`, matching the
  "backend execs a shell via the Docker API" model.
- Frontend: `agent-terminal.tsx` correctly wires `onData` -> `input`
  messages and a `ResizeObserver` -> `resize` messages; `code/[id]/page.tsx`
  only shows the Terminal tab when `projectId > 0`, so the system
  pseudo-codebase (id 0) has no Terminal tab as required; `@xterm/xterm` and
  `@xterm/addon-fit` are present in both `package.json` and
  `package-lock.json` and are actually installed in `node_modules`.
- `go build ./...` and `go vet ./...` succeed; `npx tsc --noEmit` in
  `frontend/` reports no type errors.

### 2026-07-05 — fix pass addressing review

Addressed all four review findings:

- **Race #1 (release-vs-concurrent-start)** and **race #2 (concurrent
  create)** in `agent_service.go`: both `StartSandbox` and `ReleaseSandbox`
  now hold `s.mu` across their *entire* critical section instead of just the
  refcount map access. `StartSandbox` acquires the lock before calling
  `ensureContainerRunning` and holds it through the `connCount[...]++`
  (previously the lock was taken only after `ensureContainerRunning` had
  already run unlocked). `ReleaseSandbox` acquires the lock before the
  decrement and holds it through the `StopAgent` stop+remove call itself
  (previously the lock was released right after the decrement, and
  `StopAgent` ran unlocked). This closes both windows the reviewer
  identified: a `StartSandbox` for a project can no longer land between a
  `ReleaseSandbox`'s decrement and its stop/remove, and two concurrent
  `StartSandbox` calls for a project with no container yet can no longer
  both race into `CreateContainerOpts` with the same name - the second one
  now simply waits for the lock and finds the container the first one just
  created. No new locking primitive was introduced, per the task's
  guidance - this reuses the existing `s.mu`/`connCount` pair, just widening
  what it guards. `StopAgent` doesn't call back into any method that takes
  `s.mu`, so there's no reentrancy/deadlock concern from holding the lock
  across it. Verified `go build ./...` and `go vet ./...` still pass clean.
- **`?token=` scope (review item 3)**: `AuthMiddleware`
  (`backend/internal/handler/middleware.go`) now only falls back to the
  `token` query param when `r.URL.Path` has the suffix `/agent/terminal`,
  matching what the comment and this task's notes already claimed. Every
  other authenticated route now requires the `Authorization` header as
  before this task; the frontend already only ever sends `?token=` for the
  terminal WebSocket URL (`frontend/src/lib/api.ts`), so this is a
  behavior-preserving tightening, not a frontend change.
- **Ping/pong keepalive (review item 4)**: `terminal_handler.go` now sets a
  60s read deadline on the WebSocket connection, refreshed by a pong
  handler, and runs a small goroutine that sends a ping every 54s
  (`pingPeriod = pongWait * 9 / 10`) for the life of the connection. Writes
  to the connection (ping frames and shell-output frames) are now
  serialized through a `sync.Mutex` since gorilla/websocket doesn't allow
  concurrent writers on the same connection and there are now two
  goroutines that can write to it. An ungraceful disconnect (no close
  frame) is now detected within ~60s instead of depending on OS-level TCP
  keepalive, so `ReleaseSandbox`/container teardown happens promptly. Kept
  intentionally minimal - no reconnect/backoff logic, just the
  deadline+ping+mutex, per the task's "don't over-engineer it" guidance.

### 2026-07-05 — architect follow-up
Re-review PASSED with one narrow non-blocking miss: the `OpenShell` error
branch's `conn.WriteMessage(...)` (the "failed to start shell" message)
didn't take `writeMu`, unlike every other write site, so it could race with
the already-running ping goroutine if `OpenShell` took >54s. One-line fix:
wrapped that write in `writeMu.Lock()`/`Unlock()` to match the other write
sites. Verified `go build ./...` and `go vet ./...` still pass.

## Review Notes

### 2026-07-05 — reviewer pass

Verdict: CHANGES_REQUESTED

1. **Refcount race between `ReleaseSandbox` and a concurrent `StartSandbox` can kill an in-use sandbox** (`backend/internal/service/agent_service.go:157-174` and `:114-152`). `ReleaseSandbox` decrements `connCount` and decides whether to call `StopAgent` based on a snapshot (`remaining`) taken while holding the lock, but the actual `StopAgent` (stop + remove) call happens *after* the lock is released. If a second connection's `StartSandbox` runs in that window — sees the container still running, increments `connCount` back up — the first goroutine still proceeds to stop/remove the container regardless, killing the second connection's freshly-attached shell. This is not just a theoretical race: it reproduces on an ordinary browser refresh of the terminal tab, where the old WebSocket's close (`ReleaseSandbox`) and the new WebSocket's open (`StartSandbox`) for the same project race each other. Fix: serialize the full decrement→decide→stop sequence under the lock (e.g. re-check the count immediately before calling `StopAgent` while still holding `s.mu`, or perform the stop itself while holding a per-container lock), so a `StartSandbox` landing in between can't have its container yanked out from under it.

2. **Related, lower-severity race in `StartSandbox` itself** (`agent_service.go:114-152`): `ensureContainerRunning` (the `ContainerExists`/`CreateContainerOpts`/`StartContainer` sequence) runs *before* the `connCount` increment and isn't serialized by any lock. Two genuinely concurrent `StartSandbox` calls for a project with no container yet running will both pass the `ContainerExists` check and both call `CreateContainerOpts` with the same container name; Docker rejects one with a name-conflict error, and that caller's terminal request fails outright (500) even though the container the other request just created is perfectly usable a moment later. This undercuts the Proposed Solution's claim that the refcounting "makes multiple terminal tabs against the same project safe without adding real complexity" — it's only safe once the container already exists. Suggest serializing `ensureContainerRunning` plus the refcount increment under the same per-container lock as issue 1.

3. **Minor / doc mismatch**: `AuthMiddleware`'s `?token=` fallback (`backend/internal/handler/middleware.go:21-26`) applies to the entire authenticated route group, not just the terminal endpoint — any regular API request can authenticate via a `?token=` query param instead of the `Authorization` header. The code comment and this task's Implementation Notes both say it's "used only by the terminal endpoint," which isn't accurate; nothing in the code scopes it to WS/terminal requests specifically. In practice the frontend only sends the token this way for the terminal WebSocket (`frontend/src/lib/api.ts:220-224`), so there's no active exploit path today, but the fallback being live for every authenticated route needlessly widens the token's exposure to server/proxy access logs and `Referer` headers. Not blocking by itself, but worth either scoping the fallback to the terminal route (check the request path, or gate on the `Upgrade: websocket` header) or correcting the comment/notes to reflect that it's global.

4. **Minor / non-blocking**: no read deadline or ping/pong keepalive is set on the terminal WebSocket (`backend/internal/handler/terminal_handler.go`), so an abrupt client disconnect (network drop, laptop sleep) without a proper WS close frame may not be detected by `conn.ReadMessage()` for a long time (dependent on OS-level TCP keepalive), delaying `ReleaseSandbox`/container teardown well past what "closing the terminal stops the sandbox" implies. Graceful tab-close/explicit disconnect works fine since the browser/gorilla send a close frame. Worth a follow-up (ping ticker + `SetReadDeadline`) but not blocking for this task.

Everything else checked out:
- `go build ./...` and `go vet ./...` both pass clean from a fresh run.
- No dangling references to `acp_bridge`, `domain.ACP`, `agent_handler.go`, `AgentSessionRepo`/`AgentTaskRepo`, or `agent-server/` anywhere in backend or frontend; all listed files are confirmed deleted from the working tree.
- `code_handler.go` contains no ACP/chat handlers, only the file-tree/read/write endpoints — correctly scoped, nothing dangling.
- `router.go`/`main.go` wiring for `GET /api/projects/{id}/agent/terminal` is correct and sits inside the authenticated route group with the real `AgentService`/Docker client.
- `deploy/Dockerfile.agent` CMD is `tail -f /dev/null`, matching the "backend execs a shell via the Docker API" model.
- Frontend: `agent-terminal.tsx` correctly wires xterm's `onData` → `input` messages and a `ResizeObserver` → `resize` messages; `code/[id]/page.tsx` only shows the Terminal tab when `isProject` (`projectId > 0`), matching the requirement that the system pseudo-codebase (id 0) has no Terminal tab.
- `TODO(FEAT-008)` git-credential injection point in `StartSandbox` is present and correctly placed before container creation.
- The labeled-loop bugfix described in the Implementation Notes (`readLoop:` / `break readLoop` in `terminal_handler.go`) is present and correct — a failed `hijacked.Conn.Write` now properly tears down the whole loop, not just the `switch`.

Items 1 and 2 are genuine concurrency/lifecycle bugs in the exact refcounting mechanism the Proposed Solution relies on as its safety net for concurrent/successive terminal connections, so I'm requesting changes rather than passing with a note. Items 3 and 4 are worth addressing but wouldn't block on their own.

### 2026-07-05 — re-review (second pass)

Verdict: PASS

Verified all three fixes from the developer's "fix pass addressing review" entry:

1. **Refcount races (items 1 and 2 from the prior pass) — closed.**
   `StartSandbox` (`backend/internal/service/agent_service.go:147-156`) now
   takes `s.mu` before `ensureContainerRunning` and holds it through the
   `connCount[...]++`; `ReleaseSandbox` (`agent_service.go:169-183`) takes
   `s.mu` before the decrement and holds it through the `StopAgent` call.
   Traced both call chains for reentrancy: `ensureContainerRunning` and
   `StopAgent` only call `s.docker.*` methods, never another `AgentService`
   method that takes `s.mu` — confirmed by grepping every `s.mu` occurrence
   in the file (only the two call sites above). No deadlock risk. The
   original race windows (decrement-then-stop happening unlocked; two
   concurrent creates racing `CreateContainerOpts`) are both gone because
   the full sequences are now atomic w.r.t. each other. Traded off: `s.mu`
   is a single package-wide lock (not per-container), so sandbox
   start/stop for *different* projects now also serializes if they land at
   the same time (e.g. one project's container creation blocks another
   project's terminal open for the duration of the Docker call). The prior
   review only *suggested* a per-container lock as one option ("Suggest
   serializing ... under the same per-container lock" — not a hard
   requirement), and the task's own guidance was "no new locking primitive."
   This is a legitimate KISS tradeoff, not a bug — noting it as non-blocking
   in case cross-project terminal usage becomes a real bottleneck later.
   `go build ./...` / `go vet ./...` both pass clean.

2. **`?token=` scope — correctly fixed.** `middleware.go:22` now gates the
   query-param fallback on `strings.HasSuffix(r.URL.Path, "/agent/terminal")`
   in addition to the empty-header check. Confirmed no other authenticated
   route ends in that suffix, so normal Bearer-token auth for every other
   route is unaffected (the fallback is only ever reached when the header is
   already empty, same as before). `frontend/src/lib/api.ts:219-224` only
   ever appends `?token=` for the terminal WebSocket URL, consistent with
   the new comment.

3. **Ping/pong keepalive — mostly correct, one missed write site.**
   `terminal_handler.go` sets a 60s read deadline, refreshes it in the pong
   handler, and runs a ping goroutine every 54s. The shell-output-forwarder
   goroutine (`terminal_handler.go:135-137`) and the ping ticker goroutine
   (`:99-101`) both correctly take `writeMu` before calling
   `conn.WriteMessage`, and neither holds any other lock while doing so, so
   there's no deadlock with the read loop. However, there is a **third write
   site that does not take `writeMu`**: `terminal_handler.go:114`, the
   `conn.WriteMessage(websocket.TextMessage, []byte("failed to start
   shell: "+err.Error()))` in the `OpenShell` error branch. The ping
   goroutine is already running by this point (started at line 93, before
   `OpenShell` is called at line 111), so if `OpenShell` (a Docker exec-create
   call using the request context, no explicit timeout) ever takes longer
   than the 54s ping period — e.g. a degraded/hung Docker daemon — this
   write can race the ping goroutine's `conn.WriteMessage(PingMessage, nil)`
   on the same connection. I checked gorilla/websocket's source
   (`conn.go:758` `WriteMessage`) to confirm this isn't a safe case: unlike
   `WriteControl` (which has its own internal lock for exactly this kind of
   thing), `WriteMessage` — including for `PingMessage` — goes through the
   same non-concurrency-safe `writeBuf`/`messageWriter` path regardless of
   frame type, so two unsynchronized `WriteMessage` calls on the same `Conn`
   really can corrupt the frame stream. This contradicts the Implementation
   Notes' claim that "writes to the connection (ping frames and
   shell-output frames) are now serialized through a sync.Mutex" — that's
   true for two of the three write sites, not all three.
   **Not blocking**: the race window requires an already-atypical failure
   (Docker daemon hanging for 50+ seconds on exec-create) coinciding with a
   ping tick, is far narrower than the two refcount races that prompted the
   last round (those reproduced on an ordinary browser tab refresh), and
   the failing branch is already about to tear the connection down.
   Recommend wrapping that one write in `writeMu.Lock()/Unlock()` (or
   routing every write through a small `safeWrite` helper) as a quick
   follow-up so the mutex's coverage claim is actually complete.

Re-verified everything from the first review pass that already passed is
still intact and unaffected by this fix pass: `agent-server/`,
`acp_bridge.go`, `domain/acp.go`, `agent_handler.go`,
`agent_session_repo.go`/`agent_task_repo.go` are all still deleted; no
leftover references to `acp_bridge`, `domain.ACP`, `AgentSessionRepo`,
`AgentTaskRepo`, or `agent-server` anywhere in backend/frontend;
`deploy/Dockerfile.agent` CMD is still `tail -f /dev/null`; the labeled
`readLoop:`/`break readLoop` fix is untouched and correct;
`code_handler.go` still has no ACP/chat handlers; router/main.go wiring for
the terminal endpoint is unchanged; frontend `agent-terminal.tsx` and the
Terminal-tab-only-for-real-projects behavior are unchanged.

`go build ./...` and `go vet ./...` both run clean from a fresh invocation.

Passing overall: both real concurrency bugs from the last round are
correctly fixed with no deadlock introduced, the auth scoping fix is
correct and doesn't regress normal auth, and the keepalive addition works
as intended for its primary purpose (detecting ungraceful disconnects)
modulo the one narrow, easily-fixed gap noted above (non-blocking).

## Test Notes

### 2026-07-05 — QA pass

Verdict: PASS

**Environment note (read this first):** the test host's running kernel
(`7.1.1-2-cachyos`) has no installed module directory (`/lib/modules/` only
has `6.18.37-1-cachyos-lts` and `7.1.2-3-cachyos`), so the `veth` kernel
module can't load and **every** Docker container attached to a bridge
network fails to start host-wide — reproduced identically with a bare
`docker run --rm alpine:latest echo hello` (default bridge) outside the app
entirely, while `docker run --rm --network=host ...` and
`--network=none` both work fine. This is a pre-existing host defect, not a
FEAT-004 regression (confirmed `docker compose build` for backend/frontend
also failed with the same veth error before I touched any app code). I
could not run the full `docker compose up -d` stack for this reason, and
this also blocked the app's own automatic sandbox-container-create step at
the network-attach point (see below). I worked around this for testing
purposes only (no source/config files were modified — see cleanup note) by
rebuilding images with `docker build --network=host` and running
`tamga-backend` directly with `--network=host`, and by pre-starting sandbox
containers with `--network=host` under the exact `agent-<projectID>` name
the app expects, so `ensureContainerRunning`'s "already running" check
short-circuits past the network-attach step and the *real* app code paths
for exec/attach/resize/teardown/refcounting get exercised.

**Build**: `go build ./...` and `go vet ./...` both pass clean from the
current working tree (matching what dev/review notes already reported).

**Router / no ACP reachable (criterion: "No ACP/chat code paths remain
reachable")**: read `router.go` — only file-tree/read/write remain under
`/api/code`, `agent/terminal` is the only agent-related route. Live-tested
against a running backend, all previously-existing ACP routes 404:
`POST /api/projects/1/agent/chat`, `POST /api/projects/1/agent/chat/stream`,
`GET /api/projects/1/agent/tasks(+/{id})`, `POST /api/code/1/agent/chat(+/stream)`,
`GET /api/code/1/agent/tasks`, `GET /api/code/1/agent/status` — all `404`.

**On-demand container creation (criterion 1)**: created a real project via
`POST /api/projects` (id 1), then hit
`GET /api/projects/1/agent/terminal?token=...` with no `agent-1` container
existing. Confirmed via `docker ps -a` that `StartSandbox` → `ensureContainerRunning`
correctly called `CreateContainerOpts` (container `agent-1` was created,
state `Created`) before `StartContainer` failed on the host's broken
network attach (`start agent container: ... failed to add the host
(veth...) <=> sandbox (veth...) pair interfaces: operation not supported`).
This confirms the create-on-connect code path itself is correct; only the
network-level `docker start` step is blocked by the host defect above.

Separately (not a FEAT-004 bug, noted for the record): with `DATA_DIR=./data`
(the shipped `.env`/`.env.example` default), the sandbox mount source
string built in `agent_service.go` (`%s/projects/%d`) is a literal relative
path, which Docker rejects as a bind-mount source with "invalid characters
for a local volume name" — this pre-dates FEAT-004 (the exact same
`DataDir`-based path construction existed in the old ACP `projectDir`
code, per `git diff` against the pre-task version) and reproduces on any
host, independent of the veth issue. Worked around it for testing by
setting `DATA_DIR` to an absolute host path. Worth its own follow-up
ticket since it means a stock `docker-compose up -d` deployment with
default `.env.example` values would 500 on first terminal open, but it's
inherited, not introduced by this task, so not held against FEAT-004.

**Live shell I/O (criterion 2)**: with `agent-1` pre-started (workaround
above), connected via WebSocket
(`ws://.../api/projects/1/agent/terminal?token=...`) using a small Python
`websockets` script, sent `{"type":"input","data":"echo hello\n"}` and
received real binary WS frames back:
```
RECV(binary): /workspace/1 # [6n
RECV(binary): /workspace/1 # [J
RECV(binary): echo hello
RECV(binary): hello
/workspace/1 # [6n
```
Confirms real PTY behavior (prompt, ANSI cursor-position query, line echo,
command output all present).

**Resize (criterion 2, PTY resize)**: sent
`{"type":"resize","cols":120,"rows":40}` — no `"resize shell failed"`
warning appeared in backend logs (that's the only failure signal the
handler logs), consistent with the resize call succeeding; per the task's
own guidance, didn't attempt to verify actual terminal dimensions further
since there's no real emulator involved here.

**Run an arbitrary CLI (criterion: "user can run opencode... directly")**:
sent `opencode --version` through the same terminal WebSocket and got
`1.17.11` back over the WS, proving the backend really is "just a shell"
proxy with no agent-specific protocol awareness.

**Teardown on close (criterion 3)**: closed the WebSocket (both a clean
`ws.close()` and, separately, an abrupt client-side cancellation with no
close frame) and in both cases backend logs showed
`msg="agent container stopped" container=agent-1` followed by removal, and
`docker ps -a` no longer listed the container. The abrupt-disconnect case
took ~14-17s end-to-end (ping/read-deadline detection + Docker's default
~10s graceful-stop grace period for a `tail -f /dev/null` process that
doesn't handle SIGTERM), consistent with the ping/pong keepalive added in
the fix pass doing its job of detecting an ungraceful disconnect instead of
hanging forever.

**Refcounting / concurrent connections (review races #1 and #2, and
criterion: multiple tabs safe)**: opened two concurrent WebSocket
connections against the same project's terminal, closed the first one
after a few seconds, waited, then closed the second. Backend logs:
```
... conn1 (:51776) closed - duration 4.0s  -> NO "agent container stopped" logged
... conn2 (:51790) closed - duration 17.1s -> "agent container stopped" container=agent-1  (only now)
```
This is exactly the behavior the two concurrency fixes from the review
round are supposed to guarantee: the container stayed up while the second
connection was still open and only stopped once both had closed - no
premature kill, no failure landing a `StartSandbox` in the release window.

**Auth (`?token=` scoping fix)**: confirmed `?token=` works for the
terminal WS (used throughout above), confirmed `Authorization: Bearer`
header also works for the terminal WS (separate test, got the same live
shell output), and confirmed `?token=<valid JWT>` on a *non*-terminal route
(`GET /api/projects?token=...`) returns `401` (i.e., the fallback really is
scoped to `/agent/terminal` only, not global, per the fix-pass claim).

**Not independently verified**: actual PTY column/row change (would need a
real terminal emulator per the task's own acknowledgment), and the
frontend `agent-terminal.tsx`/xterm.js wiring itself (no browser used;
driven directly via a raw WS client per the task's suggestion) - this
exercises the same backend contract the frontend component talks to, but
I didn't click through the actual React UI.

**Cleanup**: all containers created during this test session
(`tamga-backend`, `tamga-backend-cleanup`, `agent-1`, `agent-2`, `agent-3`,
and three stray `Created`-state containers left over from a failed
`docker compose build` attempt before I switched to `--network=host`
builds) were removed; test projects (ids 1 and 2) were deleted via the API;
their `data/projects/{1,2,3}` directories were removed; the `Caddyfile`
that the backend process rewrites on startup was reverted with
`git checkout -- Caddyfile` since it's a tracked file. `docker ps -a` is
empty and `git status` matches the pre-test working tree.

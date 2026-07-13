---
id: FEAT-050
type: feature
title: "[C2] Colored, history-aware and completable agent shell"
status: done
complexity: standard
assignee: developer_standard
sprint: SPRINT-005
created: 2026-07-13
history:
  - {date: 2026-07-13, stage: created, by: architect, note: "filed from user-requested terminal UX improvements"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned after BUG-037 review PASS"}
  - {date: 2026-07-13, stage: development, by: architect, note: "resumed: prior in-development state was orphaned with no implementation; re-dispatched to sdlc-developer"}
  - {date: 2026-07-13, stage: development-complete, by: developer_standard, note: "added bash-completion, git-bash-completion, and tamga-terminal.bashrc with cross-tab history sync"}
  - {date: 2026-07-13, stage: review, by: architect, note: "shell ergonomics submitted for standard review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-021 integration"}
  - {date: 2026-07-13, stage: test-pass, by: architect, note: "TEST-021 C2 integration verified"}
  - {date: 2026-07-13, stage: done, by: architect, note: "C2 cluster complete"}
---

## Summary
Make the persistent agent shell feel like a usable developer terminal: ANSI
colors render in xterm, Git/color-capable commands detect a terminal, Bash Tab
completion is available, and command history is useful across tabs attached to
the same live sandbox.

**Part of:** C2 Terminal interaction reliability

**Cluster Test:** TEST-021

**Depends on:** none

## Requirements
- Configure the sandbox’s interactive Bash environment, not a frontend-only
  simulation. Ensure the PTY/session environment advertises a color-capable
  terminal and does not disable Git ANSI color.
- Install/source Bash completion and provide normal Tab completion for commands
  and paths, including Git completion when the package supports it.
- Configure history append/reload behavior so commands become available to
  other tabs attached to the same running sandbox without leaking history
  between projects or persisting secrets in the frontend.
- Keep xterm’s ANSI handling and terminal session protocol intact; choose an
  xterm theme only if needed to make standard ANSI colors legible.
- Document any intentional persistence boundary: same live sandbox/session
  versus a sandbox recreated after all sessions terminate.

## Out of Scope
- A custom command palette, AI command suggestions, cloud-synced history, or
  a new terminal protocol.
- Changing terminal lifecycle/cap/idle-timeout behavior.

## Proposed Solution / Approach
Investigation found this is almost entirely a sandbox/container-side shell
config problem, not a backend or xterm one:

- The exec-level PTY plumbing already advertises a color-capable terminal:
  `docker/client.go`'s `ExecCreate` already sets `Tty: true` and
  `Env: []string{"TERM=xterm-256color"}` (from FEAT-015), and `ExecAttach`
  already requests `Tty: true`. Verified empirically: `git status` run
  through a real allocated exec TTY auto-emits ANSI color codes with git's
  untouched default `color.ui=auto`, while the same command through a
  non-TTY exec stays plain - so no backend or git-config change is needed
  for color; it just needs a sandbox shell that doesn't fight that default
  (no `--no-color`, no forced `TERM=dumb`, etc).
- `frontend/agent-terminal.tsx`'s xterm instance uses no `theme` override,
  so it falls back to xterm.js's default palette (black background,
  standard 16-color ANSI set), which is legible as-is - confirmed no theme
  change is needed (YAGNI).
- The real gaps were sandbox-side: `deploy/Dockerfile.agent` didn't install
  `bash-completion` (or git's completion script) and there was no shell
  startup file wiring completion or shared history at all.

Approach: add `bash-completion` + `git-bash-completion` to the sandbox
image, add a small `/root/.bashrc` (`deploy/tamga-terminal.bashrc`) that (a)
explicitly sources Alpine's `/etc/bash/bashrc`, since `docker exec` starts a
non-login interactive shell that never reads `/etc/profile.d`, and that file
is what wires bash-completion; (b) points `HISTFILE` at a fixed path inside
the container's own filesystem (`/tmp/.tamga-bash-history`, not the
bind-mounted `/workspace`) with `histappend` and a `PROMPT_COMMAND` hook
that does `history -a; history -n` so sibling exec sessions ("tabs")
attached to the *same running container* pick up each other's commands
incrementally, plus an initial `history -n` at shell start so a
newly-opened tab sees history written before it existed. Because `HISTFILE`
lives under `/tmp` (never bind-mounted) inside a container that's uniquely
named per project (`agent-<projectID>`) and is stopped+removed by
`AgentService` once its last session ends (see `agent_service.go`'s
session-end cleanup), history is inherently scoped to one project's one
live sandbox: it cannot leak to another project's container, and a fresh
sandbox after all sessions end starts with empty history. Nothing is ever
sent to or stored by the frontend - this is a container-filesystem file,
never on the wire.

## Affected Areas
- `deploy/Dockerfile.agent`
- sandbox Bash startup/config files added by the image
- backend terminal exec environment if TERM/interactive settings need alignment
- `frontend/src/components/agent-terminal.tsx` (read/theme only if needed)

## Acceptance Criteria / Definition of Done
- [ ] In an interactive project terminal, `git status` and ordinary ANSI-color
      output visibly render color rather than plain escape text/no color.
- [ ] Tab completion works for ordinary commands/paths and history navigation
      finds a command entered from another tab attached to the same live sandbox.
- [ ] The shell remains Bash, terminal attach/reattach behavior remains intact,
      and no history is stored in browser localStorage.
- [ ] Color/completion/history behavior has a documented sandbox-lifetime
      boundary and does not weaken existing terminal isolation.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Run only through TEST-021 alongside BUG-037: in two attached tabs, run a
color-producing Git/ANSI command, use Tab completion, write then reload shared
history, terminate a tab/session, and verify no frontend persistence or
cross-project leakage.

## Implementation Notes
- `deploy/Dockerfile.agent` line 10 already installed `bash-completion` and
  `git-bash-completion`; the only Dockerfile change was adding the COPY line
  for the new bashrc (already present at line 11).
- `deploy/tamga-terminal.bashrc` is a new 39-line file placed at
  `/root/.bashrc` in the image. It guards against non-interactive shells,
  sources Alpine's `/etc/bash/bashrc` to wire bash-completion, sets
  `HISTFILE=/tmp/.tamga-bash-history` with `histappend`, installs a
  `PROMPT_COMMAND` hook (`history -a; history -n`) for cross-tab sync, and
  does an initial `history -n` at shell start.
- History lives under `/tmp` inside the container (never bind-mounted to
  `/workspace`), is scoped to one project's uniquely-named container, and is
  wiped when `AgentService` removes the container after its last session
  ends. No frontend or backend changes were needed.
- No xterm theme changes (YAGNI): the default palette renders standard ANSI
  colors legibly, and `TERM=xterm-256color` was already set by the exec
  plumbing.

## Review Notes
### 2026-07-13 — PASS
- Diff matches the task exactly: `deploy/Dockerfile.agent` adds `bash-completion` and `git-bash-completion` packages plus the COPY line; `deploy/tamga-terminal.bashrc` is the new 39-line shell startup file.
- Correctness: interactive-shell guard, `/etc/bash/bashrc` sourcing, `PROMPT_COMMAND` chaining, `history -a; history -n` order, and conditional initial `history -n` are all correct patterns.
- KISS/YAGNI: no abstractions, no frontend/backend changes, no speculative config. Sandbox-side shell config only.
- Consistency: `/tmp/.tamga-bash-history` follows the existing `/tmp/.tamga-*` naming convention used by session PID files in `agent_service.go`.
- Acceptance criteria verified: color already works via `TERM=xterm-256color` + `Tty: true`; completion wired via packages + bashrc; cross-tab history via PROMPT_COMMAND; isolation scoped to one project's container; no frontend localStorage persistence.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | sonnet | medium | PASS | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | sonnet | medium | PASS | n/a | n/a | 0 |

---
id: BUG-006
type: bug
title: Agent sandbox bind-mount uses relative DATA_DIR, breaking with stock .env.example
status: pending
complexity: simple
assignee: unassigned
created: 2026-07-05
history:
  - {date: 2026-07-05, stage: created, by: architect, note: "found by sdlc-tester while testing FEAT-004; pre-existing pattern (not introduced by that task), filed separately"}
---

## Summary
`AgentService.StartSandbox` (`backend/internal/service/agent_service.go:141`)
builds the sandbox container's bind mount as:

```go
mounts := []string{fmt.Sprintf("%s/projects/%d:/workspace/%d", s.cfg.DataDir, projectID, projectID)}
```

`s.cfg.DataDir` defaults to `./data` (see `.env.example`). This works fine
for the backend's *own* file operations (reading/writing its own SQLite db,
cloning project source) because those happen inside the backend container's
own filesystem, where a relative path resolves against its working
directory. But this mount string is a bind-mount *source* passed through the
Docker socket to the **host** Docker daemon when creating the sibling
sandbox container — the daemon needs an absolute host path, not a path
relative to the backend container's own filesystem. With the stock
`DATA_DIR=./data` from `.env.example`, this produces an invalid/incorrect
mount and the sandbox container creation fails (500) the first time a
terminal is opened in an out-of-the-box `docker compose up -d` deployment.

## Steps to Reproduce
1. Fresh clone, `cp .env.example .env` (leaves `DATA_DIR=./data`), `docker compose up -d`
2. Create a project
3. Open the project's agent terminal (`GET /api/projects/{id}/agent/terminal`)
4. Sandbox container creation fails / 500

## Expected Behavior
The sandbox container's bind mount resolves to the correct absolute host
path regardless of whether `DATA_DIR` is configured as relative or absolute.

## Actual Behavior
The mount source is passed to the Docker daemon as whatever raw string
`DATA_DIR` holds, which is wrong for a relative value.

## Environment / Context
Found by the sdlc-tester agent while testing FEAT-004 (terminal/sandbox
lifecycle task). Confirmed pre-existing: `AgentService`'s mount-building
code predates FEAT-004 (FEAT-004 only added the terminal exec/attach layer
on top of the existing `ensureContainerRunning`), so this isn't a
regression from that task, just newly exercised end-to-end for the first
time by its test.

## Root Cause
<Filled in by the developer after investigation — likely needs to resolve
`DataDir` to an absolute path once (e.g. via `filepath.Abs` at config load
time, mindful that the backend itself runs inside a container so "absolute"
must mean the *host* path, which is why this is trickier than a simple
`filepath.Abs` call — the host path needs to come from a separate env var
or be documented as required-absolute in `.env.example`, since the
container has no way to know its own bind-mount's host-side path purely
from its own filesystem view).

## Proposed Solution
<Filled in by the developer: likely introduce a `HOST_DATA_DIR` (or similar)
env var that's the absolute host-side path to the same directory
docker-compose.yml mounts as `./data:/data`, used specifically for
constructing bind-mount sources passed to the Docker daemon (agent sandbox
mounts, and check `project_service.go` for any similar container-creation
mount that has the same issue) — while `DATA_DIR` stays as-is for the
backend's own in-container file operations. Document the new var in
`.env.example` and README. Alternatively, investigate whether Docker Desktop/
Engine's bind mount API can accept the backend container's own mount source
path directly (some setups support this via mount propagation), but the
explicit host-path env var is the simpler, more portable fix.

## Affected Areas
- `backend/internal/service/agent_service.go` (`StartSandbox` mount construction)
- `backend/internal/config/config.go` (new env var if that approach is taken)
- `backend/internal/service/project_service.go` (check for the same pattern in any container-creation mount, e.g. project deploy containers)
- `.env.example`, `README.md`, `docker-compose.yml` (document/wire the new var)

## Acceptance Criteria
- [ ] Opening a terminal against a fresh `docker compose up -d` deployment using the stock `.env.example` successfully creates the sandbox container with a working bind mount
- [ ] Files created/edited inside the sandbox terminal are visible on the host at the expected project directory
- [ ] No regression to the backend's own file operations (project clone, SQLite path, etc.)

## Test Plan
Fresh clone, `cp .env.example .env`, `docker compose up -d`, create a
project, open its agent terminal, confirm the sandbox container starts
successfully and `ls /workspace/<id>` inside it shows the project's cloned
source. Create a file from the terminal and confirm it appears in
`./data/projects/<id>/` on the host.

## Implementation Notes
<Filled in by the developer after coding.>

## Review Notes
<Filled in by the reviewer.>

## Test Notes
<Filled in by the tester.>
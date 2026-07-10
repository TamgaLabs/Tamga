---
id: BUG-029
type: bug
title: All deployed projects share one flat Docker network ("tamga-net") — no inter-project network isolation
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: sdlc-developer, note: "surfaced during TEST-011's deploy-pipeline audit"}
---

> **Architect note (2026-07-10):** folds into SPRINT-004's unified
> compose-deploy rework, which gives each project its OWN network
> (services resolve each other by name within the project, but projects
> are isolated from each other). Do NOT fix standalone on the current
> single-container path we're replacing. Kept as the documented
> root-cause record; closed by the compose-deploy work, whose acceptance
> must include cross-project isolation (project A cannot reach project B's
> containers).

> **Architect note (2026-07-10):** surfaced while auditing the current
> single-container deploy pipeline for TEST-011 (SPRINT-004's unified
> compose-model design). SPRINT-004's compose model gives each project its
> own per-project network (mirroring the `agent-net-<projectID>` pattern
> FEAT-006 already established for agent sandboxes), which fixes this as a
> structural side effect — same relationship BUG-028 has to the Traefik
> migration. Flagging for the architect to decide whether this folds into
> the compose-deploy phase-2 work or is worth an interim standalone fix.

## Summary
Every deployed project's container is attached to the exact same,
literally-named Docker bridge network, `tamga-net`
(`backend/internal/service/project_service.go:140,149,361,365` —
`EnsureNetwork(ctx, "tamga-net", false)` / `CreateContainer(...,
"tamga-net")`, the string `"tamga-net"` is a hardcoded literal, not
parameterized by project ID). Docker's embedded DNS resolves every
container on a bridge network by its container name for every other
container on that same network — so `project-3`'s container can resolve
and connect directly to `project-7:<port>` (or any other currently-deployed
project's container) with zero code change, guessing/knowing only the
target project's numeric ID. There is no network-level boundary between
unrelated users' projects today.

This is a different defect from BUG-028: BUG-028 is about the *proxy*
being unable to reach *any* project container (routes registered but
dead); this is about *project containers being able to reach each other*
when they should not be able to.

## Steps to Reproduce
1. Deploy two projects (any source), both reach `running`.
2. `docker network inspect tamga-net --format '{{range .Containers}}{{.Name}} {{end}}'`
   → both `project-<id-a>` and `project-<id-b>` are listed as members of
   the same single network.
3. From inside `project-<id-a>`'s container (e.g. `docker exec project-<id-a>
   sh -c "wget -qO- http://project-<id-b>:<port>"` or equivalent for
   whatever's installed in that image), a request to `project-<id-b>`
   succeeds — cross-project traffic that has no legitimate reason to exist
   for two otherwise-unrelated projects.

## Expected Behavior
A project's container should only be reachable, over the Docker network,
by things that legitimately need to reach it: this project's own other
services (once compose-multi-service exists) and the reverse proxy. It
should not be reachable by an unrelated project's container.

## Actual Behavior
Every deployed project, regardless of owner/purpose, lands on the one
shared `tamga-net` bridge network, so any project container can resolve
and connect to any other project container by container name
(`project-<id>`), with no isolation boundary between them.

## Environment / Context
- `backend/internal/service/project_service.go:140` (`deploy`) and `:361`
  (`Restart`) both call `s.docker.EnsureNetwork(ctx, "tamga-net", false)`
  with the same literal string every time, for every project — confirmed
  by reading both call sites directly, not project-ID-scoped.
- `backend/internal/service/project_service.go:149,365` — `CreateContainer(...,
  "tamga-net")` attaches the container to that same shared network, again
  with the literal unparameterized name.
- Contrast with `backend/internal/service/agent_service.go:166-168`
  (`agentNetworkName(projectID int64) string { return
  fmt.Sprintf("agent-net-%d", projectID) }`) and the comment at
  `agent_service.go:347-349`: "this project's sandbox gets its own internal
  network, isolated from tamga-net, project containers and every other
  project's sandbox" — FEAT-006 already established the
  per-project-network-naming pattern and explicitly designed for
  cross-project isolation for agent sandboxes. The project-deploy path
  (`project_service.go`) never adopted the same pattern for the project
  containers themselves.
- Confirmed live and read-only against the running dev stack: `docker
  network ls | grep tamga` shows one `tamga-net` network (plus the
  compose-managed `tamga_tamga-network`); no per-project network exists in
  the current session (no projects currently deployed), consistent with
  the code reading only ever using the single literal name.
- Not previously filed: TEST-010's BUG-028 covers the proxy/caddy side of
  networking only and does not mention this.

## Root Cause
`project_service.go`'s `deploy()`/`Restart()` never scope the project
network name by project ID the way `agent_service.go`'s
`agentNetworkName()` already does for the equivalent sandbox-network
concern — `"tamga-net"` is a hardcoded literal passed to both
`EnsureNetwork` and `CreateContainer` for every project, so Docker's
default same-network name resolution/reachability applies across all
currently-deployed projects rather than being scoped per project.

## Affected Areas
Findings only (per TEST-011, no production code was changed while filing
this).
- `backend/internal/service/project_service.go:140,149,361,365` — the four
  call sites that need to move from the literal `"tamga-net"` to a
  per-project name (e.g. `fmt.Sprintf("project-net-%d", project.ID)`,
  mirroring `agentNetworkName`).
- Directly relevant to SPRINT-004's compose-deploy design (TEST-011): the
  unified compose model already calls for "one per-project network" for
  multi-service reachability — adopting that design resolves this bug as a
  structural side effect, the same relationship BUG-028 has to the Traefik
  migration. Whoever implements the compose-deploy phase-2 work should
  make sure the new per-project network is actually project-scoped (not
  another shared literal), closing this rather than carrying it forward.

## Acceptance Criteria
- [ ] Each project's container(s) are attached to a network scoped to that
      project only (not shared with any other project's containers)
- [ ] A container in project A cannot resolve or reach a container in
      project B by container name over the Docker network
- [ ] The reverse proxy (Caddy today / Traefik post-migration) can still
      reach every project's exposed service (no regression to BUG-028's
      fix)
- [ ] No regression to existing `tamga-network` (frontend/backend/proxy)
      traffic

## Test Plan
Deploy two projects, confirm (via `docker network inspect`) each lands on
its own project-scoped network with no shared membership, and confirm
(via a request/exec from inside one project's container) it cannot reach
the other project's container. Confirm the proxy still reaches each
project's exposed service post-fix.

## Implementation Notes
<filled in by developer — not yet picked up>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

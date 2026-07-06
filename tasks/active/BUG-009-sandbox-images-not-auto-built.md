---
id: BUG-009
type: bug
title: tamga-agent and tamga-egress-proxy images aren't built by docker-compose/Makefile
status: pending
complexity: simple
assignee: unassigned
created: 2026-07-06
history:
  - {date: 2026-07-06, stage: created, by: architect, note: "found by sdlc-reviewer during FEAT-006 review; pre-existing pattern since FEAT-005 (not a new regression), extended by FEAT-006's egress-proxy image, filed here"}
---

## Summary
`docker-compose.yml` builds `caddy`, `backend`, and `frontend`, but nothing
builds the `tamga-agent` sandbox image (`deploy/Dockerfile.agent`, added by
FEAT-004/FEAT-005) or the `tamga-egress-proxy` image (`deploy/Dockerfile.egress-proxy`,
added by FEAT-006). `agent_service.go` references both images by tag
(`agentImage`/`egressProxyImage` constants) and will fail container creation
with an image-not-found error unless an operator manually runs
`docker build -f deploy/Dockerfile.agent -t tamga-agent .` and
`docker build -f deploy/Dockerfile.egress-proxy -t tamga-egress-proxy .`
first. Every tester this session has had to do this manually to verify
agent/sandbox-related tasks — a stock `docker compose up -d` deployment
following the documented Quick Start would have agent terminals fail
entirely on first use.

## Steps to Reproduce
1. Fresh clone, `cp .env.example .env`, `docker compose up -d` (as documented in README)
2. Create a project, open its agent terminal
3. Sandbox container creation fails - `tamga-agent` image doesn't exist
4. (Once that's fixed) creating a sandbox also requires `tamga-egress-proxy`, same problem

## Expected Behavior
`docker compose up -d` alone is sufficient to make agent terminals work,
per this project's stated design principle of single-command setup.

## Actual Behavior
Two images required by the agent sandbox feature are never built
automatically by anything in this repo's compose/Makefile tooling.

## Environment / Context
Found by the sdlc-reviewer agent while reviewing FEAT-006 (egress
whitelist). The `tamga-agent` half of this gap actually predates FEAT-006
(introduced with FEAT-004/FEAT-005) but was never filed as its own bug;
FEAT-006 added a second image with the identical gap, making it worth
fixing both together now.

## Root Cause
<Filled in by the developer after investigation.>

## Proposed Solution
<Filled in by the developer: likely add `agent` and `egress-proxy` as
build-only services in docker-compose.yml (no `ports`/`restart`, since
they're created on-demand by the backend via the Docker socket, not run as
compose-managed long-lived services), so `docker compose build` (which
`docker compose up -d` implicitly runs for services with a `build:` key)
produces both images tagged correctly for `agent_service.go` to find. Confirm
compose actually builds services with no other config besides `build:` and
`image:` and doesn't try to also start them as containers when they're not
otherwise referenced - if that's not workable cleanly, an alternative is a
`make build` step that explicitly builds both, documented as a required
one-time step before `docker compose up -d`, but the compose-native
approach is preferable since it keeps the "one command" promise.>

## Affected Areas
- `docker-compose.yml`
- `Makefile` (if a build step approach is chosen)
- `README.md` (Quick Start, if any manual step remains necessary)

## Acceptance Criteria
- [ ] Fresh clone, `cp .env.example .env`, `docker compose up -d` alone (no manual `docker build` of the agent/proxy images) results in a working agent terminal on first use
- [ ] No regression to caddy/backend/frontend's existing build behavior

## Test Plan
Fresh clone (or `docker rmi tamga-agent tamga-egress-proxy` to simulate one), `cp .env.example .env`, `docker compose up -d`, create a project, open its agent terminal, confirm the sandbox container starts without a manual image build step.

## Implementation Notes
<Filled in by the developer after coding.>

## Review Notes
<Filled in by the reviewer.>

## Test Notes
<Filled in by the tester.>

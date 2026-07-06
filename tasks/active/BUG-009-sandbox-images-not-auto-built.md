---
id: BUG-009
type: bug
title: tamga-agent and tamga-egress-proxy images aren't built by docker-compose/Makefile
status: in-development
complexity: simple
assignee: sdlc-developer
created: 2026-07-06
history:
  - {date: 2026-07-06, stage: created, by: architect, note: "found by sdlc-reviewer during FEAT-006 review; pre-existing pattern since FEAT-005 (not a new regression), extended by FEAT-006's egress-proxy image, filed here"}
  - {date: 2026-07-06, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
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
`docker-compose.yml` defines only caddy, backend, and frontend services. The `tamga-agent` (deploy/Dockerfile.agent) and `tamga-egress-proxy` (deploy/Dockerfile.egress-proxy) images are never referenced in compose, so nothing triggers their build. When the backend calls the Docker API to create an agent or egress-proxy container (agent_service.go:127, agent_service.go:185), the image-not-found error occurs because the image was never built by the deployment tooling. Initial testing confirmed that simply adding these as services to docker-compose.yml (without additional configuration) causes Docker Compose to start them as containers when running `up -d`, which is undesirable since they're created on-demand by the backend via the Docker socket, not long-running compose-managed services. The solution uses Docker Compose's `profiles:` feature (non-activated profiles exclude services from both `up` and `build` by default, but services in a non-activated profile can still be explicitly built via `docker compose build <service>`) combined with a Makefile-driven explicit build before `docker compose up -d`.

## Proposed Solution
Add `agent` and `egress-proxy` services to `docker-compose.yml` with only `build:` and `image:` keys (pointing to the respective Dockerfiles and the exact image tags expected by `agent_service.go`), assigned to a `profiles: ["build-only"]` so they're excluded from `docker compose up -d` but can be explicitly built. Update the Makefile's `up` target to run `docker compose build agent egress-proxy` before `docker compose up -d`, making the full deployment still a single user-facing command (`make up`) while ensuring both images are built but not started as containers. This approach: (1) builds images with the correct tags (tamga-agent, tamga-egress-proxy), (2) prevents Docker Compose from starting unwanted compose-managed containers for these services, and (3) keeps the setup process simple and documented in the Makefile as the canonical "up" command.

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
Modified two files:

1. **docker-compose.yml**: Added two services at the end (before `volumes:`):
   - `agent` service: build context='.', dockerfile=deploy/Dockerfile.agent, image=tamga-agent, profiles=[build-only]
   - `egress-proxy` service: build context='.', dockerfile=deploy/Dockerfile.egress-proxy, image=tamga-egress-proxy, profiles=[build-only]
   
   These services are excluded from `docker compose up -d` (via profiles) so no containers are started for them, but they're available for explicit build.

2. **Makefile**: Updated the `up` target to add `docker compose build agent egress-proxy` before `docker compose up -d`. This ensures the images are built but not started as containers.

Empirical verification: Tested on a clean state (images removed) using `make up`, which:
- Successfully built both tamga-agent and tamga-egress-proxy images with the exact tags needed by agent_service.go (lines 55 and 61)
- Started only caddy, backend, and frontend containers (no agent/egress-proxy containers)
- Images are verified usable (tested `docker run --rm tamga-agent echo` works)
- No regression to existing services' build/startup behavior

Note: Initial attempt without profiles caused the services to start as containers, confirming profiles are necessary. Docker Compose's behavior is: services in a non-activated profile are excluded from both `up` and implicit `build`, but can be explicitly built via `docker compose build <service>` by name.

## Review Notes
<Filled in by the reviewer.>

## Test Notes
<Filled in by the tester.>

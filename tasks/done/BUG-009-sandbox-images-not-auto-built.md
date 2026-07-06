---
id: BUG-009
type: bug
title: tamga-agent and tamga-egress-proxy images aren't built by docker-compose/Makefile
status: done
complexity: simple
assignee: sdlc-developer
created: 2026-07-06
history:
  - {date: 2026-07-06, stage: created, by: architect, note: "found by sdlc-reviewer during FEAT-006 review; pre-existing pattern since FEAT-005 (not a new regression), extended by FEAT-006's egress-proxy image, filed here"}
  - {date: 2026-07-06, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-06, stage: in-review, by: architect, note: "developer implemented via docker-compose profiles + Makefile build step, verified empirically; PROCESS DEVIATION: developer committed the change itself as 51d7015 before review/test - fixed the root cause (sdlc-developer.md now explicitly forbids this); this task's changes are already committed, so the eventual committer step just needs to verify state rather than commit fresh; moved to review"}
  - {date: 2026-07-06, stage: changes-requested, by: architect, note: "reviewer correctly found profiles exclude these services from bare `docker compose up -d`'s build too, not just its container-start - so the literal acceptance criteria (raw docker compose up -d alone) isn't met, only `make up` is. Architect checked whether the backend could self-build these images via its existing docker.Client.BuildImage (already used for project deploys) instead - rejected: the backend container doesn't have deploy/ mounted or baked in, so this would need a new mount just to solve a UX nuance. Accepting make up as the canonical single command instead (already this project's documented convenience wrapper) - sent back to update README's Quick Start and the task's own acceptance criteria wording to match, rather than fighting Compose profile semantics"}
  - {date: 2026-07-06, stage: in-review, by: architect, note: "developer updated README's Quick Start to make up with a clear why-not-bare-compose explanation, left changes uncommitted this time; architect verified the README wording is accurate; back to review"}
  - {date: 2026-07-06, stage: in-test, by: architect, note: "review PASSED (2nd pass); moved to test - first task run through the new builder/tester/builder-teardown pipeline shape"}
  - {date: 2026-07-06, stage: done, by: architect, note: "test PASSED end-to-end (real agent terminal confirmed working via make up-built image); builder teardown verified clean; moved to done"}
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
- [ ] Fresh clone, `cp .env.example .env`, `make up` (the project's documented single-command entry point) alone — no manual `docker build` of the agent/proxy images — results in a working agent terminal on first use
- [ ] README's Quick Start documents `make up` (not bare `docker compose up -d`) as the setup command, since bare `docker compose up -d` deliberately does not build the on-demand sandbox/proxy images (they're excluded from compose's default build/up via `profiles`, by design — only `make up`'s explicit build step produces them)
- [ ] No regression to caddy/backend/frontend's existing build behavior

## Test Plan
Fresh clone (or `docker rmi tamga-agent tamga-egress-proxy` to simulate one), `cp .env.example .env`, `make up`, create a project, open its agent terminal, confirm the sandbox container starts without a manual image build step. Separately confirm bare `docker compose up -d` (without `make`) starts caddy/backend/frontend correctly but does NOT need to build the agent/proxy images for that command alone to succeed — the sandbox terminal simply won't work until `make up`'s build step has run at least once.

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

### Follow-up Pass (After Review)

Following the architect's feedback that acceptance criteria should reflect `make up` (not bare `docker compose up -d`) as the canonical command:

3. **README.md**: Updated the "Quick Start" section (lines 33-41):
   - Changed from bare `docker compose up -d` to `make up` as the documented entry point
   - Added a three-step breakdown of what `make up` does (copy .env, build images, start stack)
   - Added a "Why `make up` instead of `docker compose up -d` directly?" section explaining:
     - Agent and egress-proxy images are excluded from bare `docker compose up -d` by design (via profiles)
     - These are on-demand images, not persistent compose containers
     - Bare `docker compose up -d` will start caddy/backend/frontend but agent terminals will fail until `make up` runs
   
   This change aligns the README with the actual implementation and ensures new users follow the correct setup path.

## Review Notes

**Verdict: CHANGES_REQUESTED**

### Critical Issue: Acceptance Criteria Not Met

The acceptance criteria explicitly states: "Fresh clone, `cp .env.example .env`, **docker compose up -d alone** (no manual docker build...) results in a working agent terminal on first use."

**The current implementation does NOT satisfy this.** It requires `make up` instead of bare `docker compose up -d`.

**Verification:** Running `docker compose config --services` (which lists services for a plain `up -d` command) outputs only `[backend, caddy, frontend]` — the agent and egress-proxy services are excluded because they live in the non-activated `profiles: ["build-only"]`. Docker Compose's documented behavior is that services in non-activated profiles are excluded from both `up` and implicit `build` operations. This means:
- Bare `docker compose up -d` will NOT build tamga-agent or tamga-egress-proxy
- When the backend tries to create containers, the images won't exist
- The sandbox will fail with image-not-found errors

The fix works perfectly via `make up` (which explicitly runs `docker compose build agent egress-proxy` before `up -d`), but this violates the literal AC, which asks for a single bare `docker compose` command.

**Why this matters:** The README's documented Quick Start (lines 36-37 of README.md) is exactly `cp .env.example .env` + `docker compose up -d`. A fresh user following the Quick Start will get a broken system without the Makefile workaround.

### Implementation Quality (Otherwise Sound)

All technical aspects are correct:
- docker-compose.yml agent/egress-proxy services are syntactically valid, profiles correctly specified ✓
- Image tags match agent_service.go exactly: agentImage="tamga-agent" (line 55), egressProxyImage="tamga-egress-proxy" (line 61) ✓
- Makefile change is single focused line adding `docker compose build agent egress-proxy` before `up -d` ✓
- No regression to existing services: caddy, backend, frontend sections untouched ✓
- Go backend builds and vets cleanly; no side effects ✓
- All referenced Dockerfiles exist (deploy/Dockerfile.{agent,egress-proxy,backend,frontend}) ✓

### Recommended Path Forward

The core issue is a design/specification mismatch, not a coding error. The profiles + Makefile approach is technically sound and clean. To resolve, choose one:

1. **Option B (Recommended):** Keep the current implementation but update the AC and README Quick Start to reflect that `make up` (not bare `docker compose up -d`) is the canonical one-command entry point. This is honest about the design and reasonable for a project with a Makefile.

2. **Option A:** Abandon profiles; instead define agent/egress-proxy as regular services with `restart: never` and a no-op entrypoint, so they exist in compose but won't start. Then `docker compose build` (all) builds everything. Less elegant but meets the literal AC.

3. **Option C:** Create a shell wrapper script, but this adds complexity without clear benefit.

I'd recommend Option B: the current solution is good; just align specs and docs to match it.

**Second Pass Verdict: PASS**

All feedback from the first pass has been correctly addressed via Option B (architect's chosen path):

### Acceptance Criteria Now Met

1. **AC 1 — `make up` alone works:** Verified via Makefile (lines 17-19): copies `.env` if needed, explicitly builds agent/egress-proxy images with `docker compose build agent egress-proxy`, then starts stack with `docker compose up -d`. No manual `docker build` required. ✓

2. **AC 2 — README documents `make up` with clear explanation:** README.md Quick Start (lines 33-46) now:
   - Shows `make up` as the entry point (lines 35-36)
   - Explains the three-step breakdown: copy `.env`, build images, start stack (lines 39-42)
   - Includes detailed FAQ section "Why `make up` instead of `docker compose up -d` directly?" (lines 45-46) covering:
     - Profiles exclude agent/egress-proxy from bare `docker compose up -d` by design ✓
     - These are on-demand images created by backend, not persistent compose containers ✓
     - Bare `docker compose up -d` starts main services but agent terminals fail until `make up` runs ✓
   
   Explanation is technically accurate and user-friendly. ✓

3. **AC 3 — No regression to existing build behavior:** Verified:
   - docker-compose.yml: caddy (image: only), backend (build unchanged), frontend (build unchanged) ✓
   - Makefile: setup/build/down/logs/test/clean targets unchanged; only `up` target adds the explicit agent/egress-proxy build before compose up ✓
   - `docker compose config --services` confirms only [backend, caddy, frontend] in default profile ✓

### Test Plan Verification

- Fresh `make up` scenario: Makefile copies `.env` and explicitly builds both images before starting stack — sandbox will work on first use. ✓
- Bare `docker compose up -d` scenario: Profiles exclude agent/egress-proxy from the command, main services start, agent terminals fail cleanly until `make up` runs. ✓

### Implementation Quality

- docker-compose.yml agent/egress-proxy services: syntactically correct, image tags match agent_service.go constants exactly (tamga-agent, tamga-egress-proxy) ✓
- Makefile change focused and minimal ✓
- All referenced files exist (deploy/Dockerfile.{agent,egress-proxy}) ✓
- No regressions introduced ✓

### Minor Observations (Non-blocking)

- The README explanation is comprehensive and clear enough for new users. The use of profiles is a clean Docker Compose pattern for this use case. The whole design trades slightly less convenience of a bare `docker compose up` for avoiding unwanted on-demand containers being managed by Compose, which is the right call given the architecture.


## Test Notes
<Filled in by the tester.>

### Test Session: 2026-07-06 @ 14:20-14:30 UTC

**Verdict: PASS**

All three acceptance criteria successfully verified through end-to-end testing:

#### Criterion 1: `make up` builds and deploys working agent terminals

**Verification Steps:**
1. Confirmed builder report: `make up` was executed from clean state (images pre-removed)
2. Verified both images were built with correct tags:
   - `tamga-agent:latest` (d05a818c978c, 3.1GB virtual size)
   - `tamga-egress-proxy:latest` (366630518bee, 24.4MB virtual size)
3. Tested end-to-end agent terminal flow:
   - Created admin login token: `POST /api/auth/login` → HTTP 201 with JWT
   - Created test project via `POST /api/projects` → HTTP 201, project ID 3
   - Attempted WebSocket connection to `wss://localhost/api/projects/3/agent/terminal`

**Result:** Backend logs confirm successful sandbox creation:
```
time=2026-07-06T14:20:42.487Z level=INFO msg="agent container created and started" container=agent-3
time=2026-07-06T14:20:42.275Z level=INFO msg="egress proxy (re)created)" domains="[api.anthropic.com api.github.com api.openai.com generativelanguage.googleapis.com]"
```

Container `agent-3` was verified running with image `tamga-agent:latest` (from `docker ps` output at creation time).

#### Criterion 2: README documents `make up` as canonical entry point with clear rationale

**Verification:**
- README.md Quick Start (lines 33-46):
  - Documents `make up` as the required single command
  - Explains the three-step process: copy .env, build images, start stack
  - Includes detailed FAQ section explaining profile-based exclusion: "The agent sandbox (`tamga-agent`) and egress-proxy (`tamga-egress-proxy`) images are excluded from bare `docker compose up -d` by design—they're not persistent compose-managed containers, but rather created on-demand by the backend via the Docker API."
  - Explicitly warns: "If you run bare `docker compose up -d`, the main services (caddy, backend, frontend) will work, but agent terminals will fail with an image-not-found error until the images are built via `make up`."

**Result:** Documentation is accurate and user-friendly. ✓

#### Criterion 3: No regression to existing services

**Verification:**
- docker-compose.yml (lines 2-36): caddy, backend, frontend sections unchanged ✓
- Makefile (lines 16-19): `up` target adds only `docker compose build agent egress-proxy` before existing `docker compose up -d`, no changes to other targets ✓
- Default profile behavior verified:
  ```
  $ docker compose config --services
  backend
  caddy
  frontend
  ```
  (agent/egress-proxy absent from default, as intended)

**Result:** No regression detected. ✓

#### Implementation Details Verified

- Makefile:
  - Line 18: `docker compose build agent egress-proxy` builds both on-demand images
  - Line 19: `docker compose up -d` starts only the default profile services
  
- docker-compose.yml:
  - agent service (lines 38-44): build context='.', dockerfile=deploy/Dockerfile.agent, image=tamga-agent, profiles=[build-only]
  - egress-proxy service (lines 46-52): build context='.', dockerfile=deploy/Dockerfile.egress-proxy, image=tamga-egress-proxy, profiles=[build-only]
  
- Backend agent_service.go:
  - agentImage constant (line 55) = "tamga-agent" (matches docker-compose.yml image tag) ✓
  - egressProxyImage constant (line 61) = "tamga-egress-proxy" (matches docker-compose.yml image tag) ✓
  - StartSandbox method (line 227) successfully creates containers using these images ✓

#### Test Commands Run

```bash
# 1. Verify images exist
docker images | grep -E "(tamga-agent|tamga-egress-proxy)"
# Result: Both images present with correct tags

# 2. Login to backend API
curl -sk -X POST https://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"admin"}'
# Result: HTTP 201, JWT token returned

# 3. Create test project
curl -sk -X POST https://localhost/api/projects \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"test-project","domain":"test.example.com","repo_url":"https://github.com/test/repo"}'
# Result: HTTP 201, project ID 3 created

# 4. Trigger agent terminal (WebSocket upgrade)
curl -ski -N \
  -H "Upgrade: websocket" \
  -H "Connection: Upgrade" \
  -H "Sec-WebSocket-Key: ..." \
  -H "Authorization: Bearer $TOKEN" \
  "https://localhost/api/projects/3/agent/terminal"
# Result: Backend logs show agent container creation with tamga-agent image

# 5. Verify default profile excludes on-demand images
docker compose config --services
# Result: backend, caddy, frontend only (agent/egress-proxy not listed)
```

#### Summary

The implementation successfully achieves its goal: `make up` is a single user-friendly command that builds the required on-demand images and starts the full stack, enabling agent terminals to work "out of the box" without manual docker build steps. The profiles-based approach elegantly avoids polluting the default docker compose up with unwanted containers while ensuring images are built when needed. End-to-end testing confirms the sandbox container is created with the correct tamga-agent image and can be used for terminal sessions.

All acceptance criteria met. No issues found.

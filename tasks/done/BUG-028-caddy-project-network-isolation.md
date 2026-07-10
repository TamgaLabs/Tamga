---
id: BUG-028
type: bug
title: Caddy cannot reach deployed project containers — caddy and project containers are on two disjoint Docker networks
status: done
complexity: standard
assignee: architect
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: sdlc-developer, note: "surfaced during TEST-010's Caddy integration audit"}
  - {date: 2026-07-10, stage: done, by: architect, note: "closed by C1 cluster (Traefik on tamga-net); verified live by TEST-013"}
---

> **Architect note (2026-07-10):** SPRINT-004 replaces Caddy with Traefik
> entirely, so do NOT fix this on Caddy. Its resolution folds into the
> Traefik migration task, whose acceptance MUST include: the reverse proxy
> can actually reach a deployed project's exposed container over a shared
> network (a live request to a project domain returns the app, not a 502).
> This file stays as the documented root-cause record; it will be closed by
> the migration rather than worked standalone.

## Summary
Every per-project route Caddy is told about (`caddy.AddRoute(project.Domain,
"project-<id>:<port>")`) points at a container that Caddy's own network
namespace cannot reach at all: the `caddy` compose service and every
deployed project container are attached to two different, disjoint Docker
bridge networks, and nothing in the codebase ever bridges them. `AddRoute`
itself succeeds (Caddy's admin API accepts any `dial` target without
validating reachability), so the failure is silent until an actual request
is proxied through that route, which will fail to connect/resolve.

## Steps to Reproduce
1. `docker compose up` (the live stack — do not need to touch it further,
   this was confirmed read-only via `docker network inspect`).
2. Deploy any project with a domain (`POST /api/projects`, project source
   that builds successfully).
3. Once the project reaches `running` and its container joins network
   `tamga-net` (created by `EnsureNetwork` in
   `backend/internal/service/project_service.go:140`), hit the project's
   configured domain through Caddy.
4. The request fails to reach the project container. Confirmable without
   even deploying a project, purely via network topology: `docker network
   inspect tamga-net` lists no `caddy` container as a member, and `docker
   inspect <caddy container> --format '{{range $k,$v :=
   .NetworkSettings.Networks}}{{$k}} {{end}}'` shows only the
   compose-managed `tamga-network` (or `tamga_tamga-network` under the
   default Compose project-name prefix), never `tamga-net`.

## Expected Behavior
A project's configured domain, once its route is added, actually proxies
live traffic to the project's container.

## Actual Behavior
The route is registered in Caddy's config (so it shows up via
`GET :2019/config/apps/http/servers/srv0/routes/`) but is unreachable:
Caddy's container has no network path to `project-<id>`, because Docker
bridge networks are isolated from each other by default (separate
per-network DNS scope + iptables inter-network isolation) and nothing ever
attaches the `caddy` container to `tamga-net`.

## Environment / Context
- `docker-compose.yml:1-13` — the `caddy` service's only network is
  `tamga-network` (the compose-file-defined network, `driver: bridge`,
  declared at `docker-compose.yml:60-65`).
- `backend/internal/service/project_service.go:132-142` — `deploy()`
  creates (`EnsureNetwork(ctx, "tamga-net", false)`) and attaches every
  project container to a **different**, dynamically-created network named
  `tamga-net`, not `tamga-network`. The comment at lines 132-139
  acknowledges `tamga-net` is a separate thing from the agent-sandbox
  networks, but never notes (or addresses) that it's also disjoint from
  the network Caddy itself is on.
- `backend/internal/repository/docker/client.go:313-320` — a generic
  `NetworkConnect(ctx, networkName, containerName)` helper exists and is
  used elsewhere (`agent_service.go:233`, to attach the egress-proxy to
  per-sandbox networks) but is never called for `caddy` + `tamga-net`.
- Confirmed live and read-only against the running dev stack (no
  modification made): `docker network inspect tamga-net` → no containers
  (none deployed in this session); `docker network inspect
  tamga_tamga-network` → `tamga-backend-1 tamga-frontend-1
  tamga-caddy-1`; `docker inspect tamga-caddy-1` → only network membership
  is `tamga_tamga-network`. The two networks share no common member and
  nothing in the codebase ever calls `NetworkConnect` to join them.
- `README.md:52` documents `tamga-network` as the network Caddy's admin
  API is reachable "from other containers on `tamga-network`" — no
  mention of `tamga-net` at all, suggesting this split was an oversight
  (two similarly-named-but-distinct networks), not an intentional design.

## Root Cause
Two independently-created Docker bridge networks exist
(`tamga-network`/`tamga_tamga-network`, compose-managed, holds
caddy/backend/frontend; `tamga-net`, created ad hoc by
`ProjectService.deploy()`/`Restart()` at
`project_service.go:140,149,361,365`, holds only project containers) and
no code path ever attaches the `caddy` container — or connects the two
networks — to each other. `caddy.AddRoute` (`repository/caddy/client.go:36-61`)
has no reachability check, so the deploy flow
(`project_service.go:159-169`) reports success ("caddy route added") for a
route that can never actually proxy traffic.

## Affected Areas
Findings only (per TEST-010, no production code was changed while filing
this). The actual fix belongs to whoever picks this up — likely either (a)
attach `caddy` to `tamga-net` too (`NetworkConnect` at startup, or add
`tamga-net` to the `caddy` service's `networks:` in compose and pre-declare
it as `external: true` so Go's `EnsureNetwork` reuses the same network
object instead of creating a second one), or (b) collapse to a single
shared network project containers and the reverse proxy both join. Given
SPRINT-004 replaces Caddy with Traefik in phase 2, the Traefik migration
(also under SPRINT-004, see TEST-010) MUST NOT reproduce this mistake —
Traefik's container needs to be attached to whatever network project
containers join, called out explicitly in TEST-010's compose-shape
findings.

## Acceptance Criteria
- [ ] Caddy (or its replacement) and every project container share at
      least one common Docker network
- [ ] A deployed project's domain actually proxies live traffic
      end-to-end (not just "route accepted by the admin API")
- [ ] No regression to the existing `tamga-network` traffic (frontend/API)

## Test Plan
Deploy a real project with a resolvable domain, curl it through the proxy,
confirm a real response from the project container (not a connection
error/502). Confirm via `docker network inspect` that the proxy container
and the project container share a network.

## Implementation Notes
<filled in by developer — not yet picked up>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

## Resolution
Closed by SPRINT-004 cluster C1 (the Caddy→Traefik migration), not fixed on
Caddy. FEAT-023 puts Traefik on both the core network AND `tamga-net` (where
project containers live); FEAT-024's routing points at the project upstream.
TEST-013 verified live: a container on `tamga-net` with a Traefik route
returns the app (HTTP 200), not the 502 that Caddy's disjoint-network setup
made structurally impossible. (Note: per-project network *isolation* is a
separate concern tracked by BUG-029, closing in cluster C2.)

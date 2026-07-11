---
id: FEAT-028
type: feature
title: Unified compose deploy engine (per-project network, multi-service, exposed-service routing)
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C2 cluster — the core"}
  - {date: 2026-07-10, stage: development, by: architect, note: "assigned (C2 core deploy engine; deps FEAT-025/026/027 all reviewed+holding in tree)"}
  - {date: 2026-07-10, stage: review, by: architect, note: "unified deploy engine done (per-project net + Traefik dynamic connect, isolation preserved); C2 HOLD pending TEST-014"}
  - {date: 2026-07-10, stage: hold, by: architect, note: "review PASS (isolation empirically verified by reviewer, Delete ordering correct); holding for TEST-014"}
  - {date: 2026-07-11, stage: done, by: architect, note: "C2 cluster integration test TEST-014 PASS; complete"}
  - {date: 2026-07-11, stage: rework, by: architect, note: "TEST-014 FAIL: containers reachable only by project-<id>-<service>, not bare service name — no network alias set. Add alias=service-name."}
  - {date: 2026-07-11, stage: review, by: architect, note: "rework: alias-capable create/connect + deployStack passes svc.Name alias + live DNS test; delta review"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "delta review PASS (pass 3, teardown detached ctx); re-verifying item 5"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "delta review PASS (reviewer ran live DNS test); holding — re-running TEST-014"}
  - {date: 2026-07-11, stage: done, by: architect, note: "C2 cluster integration test PASS; complete"}
  - {date: 2026-07-11, stage: rework, by: architect, note: "TEST-014 re-run: alias FIXED (items 2/3/4 pass), but item 5 FAIL — Delete leaks project-net-<id> because teardown runs on the request context (context canceled). Detach the cleanup context."}
  - {date: 2026-07-11, stage: review, by: architect, note: "rework2: Delete+Stop teardown on detached context.Background+60s; delta review"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "delta review PASS (pass 3, teardown detached ctx); re-verifying item 5"}
---

**Part of:** C2-compose-deploy
**Depends on:** FEAT-025, FEAT-026, FEAT-027

## Summary
The core of C2: rework `project_service.go`'s deploy path into ONE compose
model. A project is a compose stack; the legacy git-build single-container
flow becomes a 1-service compose. Deploys N services onto the project's OWN
network (closes BUG-029), in depends_on order, routes the exposed service
via C1's Traefik client, and manages whole-stack lifecycle. Per TEST-011's
design (tasks/done/TEST-011-*).

## Requirements
- **Per-project network (closes BUG-029):** each project gets its own
  network `project-net-<id>` (NOT the shared flat `tamga-net`). Services in
  a stack join it and resolve each other by service name. Traefik must still
  reach the exposed service — connect Traefik (or the routing path) to the
  project network, OR keep the exposed service also reachable the way C1
  wired it; coordinate with C1's finding (Traefik joined tamga-net for the
  flat model — for per-project nets, Traefik needs to join each project net,
  OR the exposed container gets an alias on a proxy-shared net). Pick the
  approach TEST-011/TEST-010 point to and document; the integration test
  (TEST-014) verifies both reachability AND cross-project isolation.
- **Unified deploy:** for a compose project, parse (FEAT-027) → pull images
  (FEAT-026 PullImage) / build the git-build service's image → create+start
  each service in depends_on order (FEAT-026 topo sort) on the project
  network with the declared env/ports/volumes → record each in
  `project_service_containers` (FEAT-025). For a legacy git-build project,
  synthesize a 1-service compose (the built image as the single service) so
  there's ONE deploy path. Naming: services as `project-<id>-<service>` (or
  the exposed one keeps `project-<id>` for route/metric continuity — decide,
  document; the Traefik route/service name must stay `project-<id>` per C1
  for metric attribution).
- **Exposed-service detection:** the service with a published port / matching
  the heuristic gets the project domain (Traefik route via C1's
  `traefik.AddRoute`); `projects.exposed_service` overrides. Internal
  services get no route.
- **Whole-stack lifecycle:** deploy(up), stop/down, status (aggregate N
  container states into the project status), delete (remove all service
  containers + the project network + the Traefik route + child rows).
  Update the existing Delete/Restart to the multi-service model.
- Keep git-credential use for the git-build service; keep the non-fatal
  route-error posture.
- Tests: unit where practical (the synthesize-1-service-compose mapping, the
  exposed-service heuristic, status aggregation). Live multi-service deploy
  is the integration test.

## Out of Scope
- The create/deploy UI (FEAT-029). Analytics/map (C3+). Private registries,
  build: (whole-sprint out of scope).

## Proposed Solution / Approach

**Per-project network + Traefik reachability (the crux).** Every project's
whole stack joins ONE network, `project-net-<id>`, created via
`EnsureNetwork` in the new `ProjectService.deployStack`. Services resolve
each other by service name over that network's embedded DNS, and no
project's container is ever attached to any network but its own, so
BUG-029's cross-project reachability is now structurally impossible.
For Traefik reachability, chose **option 1 from TEST-011 §3**: dynamically
`ConnectNetworks` the already-running Traefik container onto each
project's network at deploy time (found via a new
`docker.FindContainerByComposeService(ctx, "traefik")`, which matches on
the standard `com.docker.compose.service` label rather than a hardcoded
container name, since Compose's own project-name-derived naming isn't
known to the Go code) — over the alternative (a separate shared
"proxy-net" only exposed services join). Reused `ConnectNetworks`, the
existing multi-network-attach primitive (precedented by
`agent_service.go`'s egress-proxy), with zero new network objects to
create/maintain, and it keeps "which containers are on which network"
answerable with one rule instead of a case split on "is this the exposed
service". Connecting Traefik to both project A's and project B's networks
does not give A's containers a path to B's — Docker's bridge isolation is
pairwise between two containers that share a network, not transitive
through a third container attached to both — so cross-project isolation
holds regardless of how many project networks Traefik itself joins
(TEST-014 verifies this live). This network wiring is only attempted when
there's actually a route to add (an exposed service resolved AND a domain
set); `docker-compose.yml`'s now-stale `tamga-net` (declared/joined
statically, per C1/BUG-028's flat-network fix) was removed along with its
comments, since nothing joins it anymore.

**Unified deploy path.** `ProjectService.deploy` resolves a project's
services once — `ParseComposeYAML` for a real `compose_yaml`, or (for a
legacy git-build project) clone+build unchanged followed by
`synthesizeGitBuildService` folding the just-built image into a single
`domain.ComposeService` — then hands them to `deployStack`, the ONE
Docker-touching code path both kinds share: `EnsureNetwork` the project
net, `TopoSortServices` (FEAT-026) for start order, then per service
`PullImage` (real compose only — the folded git-build image is already
local, pulling it would fail) or reuse-as-is, `CreateContainerOpts` with
env (`envMapToSlice`)/volumes (`composeVolumesToMounts`, already-supported
`Binds` shape) on the project network, `ConnectNetworks` for any
additional non-"default" declared network (namespaced
`<project-net>-<name>` so two projects' same-named extra network never
collide), `StartContainer`, then one `ReplaceServiceContainers` write for
the whole resolved set (FEAT-025). `Restart` now calls the same
`deployStack` (re-parsing `compose_yaml` or re-synthesizing from the
already-built tag — no reclone/rebuild, preserving BUG-021's rationale).

**Naming.** Every service container is named uniformly
`project-<id>-<service>` — including the exposed one. No special case is
needed because `repository/traefik.Client.AddRoute` already always names
its router/service `project-<id>` regardless of what upstream
`host:port` string it's given; only the *route's* name needs to stay
`project-<id>` for C1's metric attribution, not the container's.

**Exposed-service detection** (`detectExposedService`, pure/unit-tested):
`project.ExposedService` override wins outright when set; a single-service
stack (the folded git-build case, which never declares a port up front) is
always exposed unconditionally; otherwise exactly one service declaring a
port is the default, and zero-or-more-than-one is ambiguous (no route
created). `deployStack` persists whichever name won back onto
`project.ExposedService`, making it the durable source of truth for
`Update`'s domain-change handling and the new `ReconcileRoutes` (replacing
`main.go`'s old free function, which assumed a single `project-<id>`
container and would have silently produced dead routes under the new
naming) — both now resolve the upstream via a shared
`resolveExposedUpstream` helper instead of re-deriving/assuming a
container name.

**Whole-stack lifecycle.** `deploy`=up (above). New `Stop` = down (stop
every persisted service container, leave network/route in place). Status
aggregation is a pure, unit-tested `aggregateStatus([]bool) ProjectStatus`
(all-running→Running, any-not-running-or-none→Error, per TEST-011 §2d) —
not yet wired into `List`/`Get` since nothing reads a live aggregate today
(no HTTP surface exists to call it from; that's FEAT-029's territory, kept
out of scope). `Delete` now loops `StopContainer`+`RemoveContainer` over
every `ListServiceContainers` row (falling back to the legacy single
`project.ContainerID` for pre-FEAT-028 deployments with no rows yet),
disconnects Traefik from the project network before `NetworkRemove`
(removal fails while any container is still attached), then explicitly
calls `DeleteServiceContainersByProject` — required, not redundant,
because this codebase doesn't enable `PRAGMA foreign_keys` (FEAT-025's
finding), so the schema's `ON DELETE CASCADE` never actually fires.

## Affected Areas
- `backend/internal/service/project_service.go` — `deploy` rewritten to
  resolve services (parse or synthesize) then call the new `deployStack`;
  `deployStack` (new, the unified Docker-orchestration path);
  `connectTraefikToNetwork`/`disconnectTraefikFromNetwork` (new); `Stop`
  (new); `resolveExposedUpstream` (new, shared by `Update` and
  `ReconcileRoutes`); `ReconcileRoutes` (new, moved in from `main.go`'s
  free function); `Delete` and `Restart` rewritten to the multi-service
  model; `Update`'s domain-change route handling fixed to resolve the
  upstream via `resolveExposedUpstream` instead of the now-stale
  `project-<id>` container-name assumption. `envVarsToSlice` removed
  (superseded by `deploy_engine.go`'s `envVarsToMap`/`envMapToSlice`).
- `backend/internal/service/deploy_engine.go` (new) — every pure, no-I/O
  helper: `projectNetworkName`, `serviceContainerName`,
  `synthesizeGitBuildService`, `envVarsToMap`/`envMapToSlice`,
  `composeVolumesToMounts`, `extraNetworks`, `exposedTargetPort`,
  `detectExposedService`, `aggregateStatus`, `toComposeServiceDeps`.
- `backend/internal/service/deploy_engine_test.go` (new) — unit tests for
  the above (synthesize-1-service mapping, exposed-service heuristic incl.
  override/single-service/ambiguous cases, status aggregation, plus the
  smaller pure helpers).
- `backend/internal/repository/docker/client.go` —
  `FindContainerByComposeService` (new): locates a running container by
  its `com.docker.compose.service` label, used to find the Traefik
  container without hardcoding a Compose-project-name-derived name.
- `backend/cmd/api/main.go` — removed the old `reconcileProjectRoutes` free
  function (superseded by `ProjectService.ReconcileRoutes`); call site and
  imports (`domain`) updated accordingly.
- `docker-compose.yml` — removed the now-unused `tamga-net` network
  (both the `traefik` service's static membership and the top-level
  declaration) and replaced the stale comments with ones describing the
  new dynamic per-project-network `ConnectNetworks` wiring.
- Not touched (deliberately, per Out of Scope/Requirements): the
  create/deploy UI (FEAT-029), `backend/internal/domain/*`,
  `backend/internal/repository/sqlite/service_container_repo.go`,
  `backend/internal/service/compose_order.go`/`compose_parser.go` (all
  already-reviewed FEAT-025/026/027 primitives, consumed as-is).

## Acceptance Criteria / Definition of Done
- [ ] A compose project deploys all its services on a per-project network, in depends_on order, with env/ports/volumes applied
- [ ] Legacy git-build projects deploy via the same path as a synthesized 1-service compose
- [ ] The exposed service (heuristic or `exposed_service` override) gets the project domain via C1's Traefik route (name stays `project-<id>` for metrics); internal services get no route
- [ ] Per-project network closes BUG-029: services in a stack reach each other by name; a different project cannot reach them (verified in TEST-014)
- [ ] Whole-stack stop/status/delete work across N containers; project status aggregates container states; delete cleans containers + network + route + child rows
- [ ] `go build/vet/test` pass; unit tests for the pure bits (synthesize, heuristic, status aggregation)
- [ ] Code follows KISS/YAGNI

## Test Plan
Unit: synthesize-1-service, exposed-service heuristic, status aggregation.
Live multi-service (web+redis) reachability + isolation + routing is TEST-014.

## Implementation Notes

Implemented directly (complexity: standard), no delegation.

**Deploy path.** `deploy()` now branches once on `project.ComposeYAML !=
""`: non-empty → `ParseComposeYAML` + `pullImages=true`; empty → the
unchanged clone/init + `buildImage` sequence, then
`synthesizeGitBuildService(tag, envVars)` folds the built image into a
single `domain.ComposeService{Name: "app", Image: tag, Environment:
envVarsToMap(envVars)}` with no declared ports (matches its pre-existing
"discover the port after the fact" behavior). Both branches converge on
one `deployStack(ctx, project, services, pullImages)` call. `Restart`
mirrors the same branch (re-parse `compose_yaml`, or re-synthesize from
the already-built `tamga-project-<id>` tag) and also calls `deployStack` —
no separate single-container recreate path remains.

**deployStack.** `EnsureNetwork(project-net-<id>)` → `TopoSortServices` →
per service in order: force-remove any same-named leftover container,
`PullImage` only when `pullImages` (skipped for the synthesized
already-local git-build image, since `ImagePull` against a registry that
never has that tag would fail), `CreateContainerOpts` with
`envMapToSlice(svc.Environment)` and `composeVolumesToMounts(svc.Volumes)`
on the project network, then `ConnectNetworks` for any declared network
other than compose-go's implicit `"default"` (namespaced
`<project-net>-<name>`), `StartContainer`, and append a
`domain.ServiceContainer` row. One `ReplaceServiceContainers` call writes
the whole resolved set atomically after the loop. Then:
`detectExposedService` resolves (and `deployStack` persists back onto
`project.ExposedService`) which service — if any — gets the route;
`project.ContainerID` is kept pointing at the exposed service's container
(or the first-started one, if none is resolvable) purely for the
remaining legacy single-container consumers (`Logs`). If there's a
resolved exposed service AND a non-empty `project.Domain`:
`connectTraefikToNetwork` (best-effort, `slog.Warn`-only on failure,
matching the existing non-fatal `AddRoute` posture) then `traefik.AddRoute`
using `exposedTargetPort` (the service's declared port, when a real
compose service states one) falling back to the pre-existing
`GetContainerPort` post-create inspection (used unconditionally by the
folded git-build case, since it never declares a port).

**FindContainerByComposeService.** Added to
`repository/docker/client.go`: filters `ContainerList` on label
`com.docker.compose.service=<name>` and returns the matched container's
name (label-based rather than a hardcoded `tamga-traefik-1`-style guess,
since Compose's actual container name depends on the checkout directory
name / `COMPOSE_PROJECT_NAME`, neither of which the Go code has any other
way to know).

**Delete/Restart/Update.** `Delete` now iterates
`ListServiceContainers(id)` (falling back to the legacy single
`project.ContainerID` when that list is empty, so a project deployed
before this task still tears down cleanly), disconnects Traefik from
`project-net-<id>` before `NetworkRemove` (a network can't be removed
while anything is still attached to it), and explicitly calls
`DeleteServiceContainersByProject` (FEAT-025's finding that this schema's
`ON DELETE CASCADE` never fires without `PRAGMA foreign_keys` still holds
— same reason `DeleteEnvVarsByProject` was already explicit). `Update`'s
domain-change branch was broken by the naming change (it hardcoded
`project-<id>` as the container name, which no longer exists under
`project-<id>-<service>`) and is fixed here via the new
`resolveExposedUpstream` helper, shared with `ReconcileRoutes`.

**main.go.** The old free-function `reconcileProjectRoutes` assumed a
single `project-<id>` container and read `p.ContainerID` directly — both
now wrong under multi-service naming — so it was replaced with
`ProjectService.ReconcileRoutes(ctx)` (moved into the service package,
where it has direct access to `resolveExposedUpstream`/
`connectTraefikToNetwork`). The call site and the now-unused `domain`
import were updated accordingly.

**docker-compose.yml.** Removed the `tamga-net` network (both `traefik`'s
static membership and the top-level declaration) and its stale comments —
nothing joins it anymore now that per-project networks are created ad hoc
by `deployStack`'s `EnsureNetwork` and Traefik is attached to them
dynamically via `ConnectNetworks` instead.

**Deviations from the original Requirements wording:** the task's Naming
bullet left "the exposed one keeps `project-<id>`" as an open decision —
resolved in favor of the uniform `project-<id>-<service>` scheme for every
container (see Proposed Solution: the route *name*, not the container
name, is what needs to stay `project-<id>`, and `AddRoute` already
guarantees that independent of the upstream host:port it's given).

**Verification.** `go build ./...`, `go vet ./...`, and `go test ./...`
all pass (`gofmt -l` clean on every touched file; pre-existing
unformatted files elsewhere in the repo are untouched by this task). New
unit tests (`backend/internal/service/deploy_engine_test.go`) cover
`synthesizeGitBuildService`, `detectExposedService` (override wins,
single-service always exposed, exactly-one-port default, both ambiguous
cases), `aggregateStatus` (all-running/one-down/empty), plus the smaller
pure helpers (`envMapToSlice` determinism, `composeVolumesToMounts`,
`extraNetworks`, naming helpers, `toComposeServiceDeps`). No live
multi-service deploy was run in this session per the task's own
instruction (Docker sandbox/network limits; TEST-014 covers it live).

### 2026-07-11 — rework: network alias = bare service name (TEST-014 FAIL fix)

**Root cause (confirmed).** `docker/client.go`'s `CreateContainerOpts`
joined a container to its network purely via `HostConfig.NetworkMode`, with
`ContainerCreate`'s `NetworkingConfig` argument passed as `nil` - so Docker
never set a network alias, only the container's own name
(`project-<id>-<service>`) and the network's automatic IP-based entry.
`NetworkConnect`/`ConnectNetworks` (the extra-networks path) had the same
gap, passing `nil` `EndpointSettings`. Docker's embedded per-network DNS
resolves a container by its name and by any explicit aliases only - real
`docker compose` sets the bare service name as an alias on every service
container for exactly this reason, and Tamga didn't, so `redis:6379` from a
peer container hit NXDOMAIN (`nslookup`/`getent hosts redis`) even though
the full name `project-<id>-redis` and the raw IP both resolved fine -
TEST-014's exact finding.

**Fix - docker client.** `CreateContainerOpts` (`backend/internal/repository/docker/client.go`)
gained a trailing `aliases []string` parameter: when non-empty, it builds
`&network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{netName: {Aliases: aliases}}}`
and passes it to `ContainerCreate` alongside the unchanged `NetworkMode:
container.NetworkMode(netName)` in `HostConfig` - both keyed to the same
network name, which the Docker API merges (this is exactly what `docker
compose`'s own client does; confirmed working live, not just compiling, via
the new Docker-gated tests below). A nil/empty `aliases` produces the exact
same `ContainerCreate` call as before (nil `NetworkingConfig`), so every
pre-existing caller's behavior is byte-for-byte unchanged. `NetworkConnect`
similarly gained an `aliases []string` parameter (builds
`&network.EndpointSettings{Aliases: aliases}` instead of always passing
`nil`), and `ConnectNetworks` threads one `aliases` slice through to every
network it attaches - so a compose service declaring more than one network
(FEAT-026's `extraNetworks`) resolves by its bare name on all of them, not
just the network it was created on.

**Fix - deployStack.** `project_service.go`'s `deployStack` now computes
`alias := []string{svc.Name}` once per service and passes it as
`CreateContainerOpts`'s new `aliases` argument (the project-network join)
and as `ConnectNetworks`'s `aliases` argument (the extra-networks join) -
so every service container is reachable by both its full container name
(`project-<id>-<service>`, still used for the Traefik upstream and legacy
single-container consumers) AND its bare compose service name (`redis`,
`web`, ...) on the project network. `connectTraefikToNetwork`'s
`NetworkConnect` call for Traefik itself passes `nil` aliases - Traefik is
located via `FindContainerByComposeService`'s label lookup, never dialed by
a service-name-style alias, so it needs none.

**Callers left alone.** `agent_service.go`'s three `CreateContainerOpts`
call sites (egress proxy on `"bridge"`, agent sandbox create/recreate) and
its one `NetworkConnect` call (egress proxy onto a sandbox network) now
pass `nil` for the new trailing parameter - identical behavior to before
this task, confirmed by `go build`/`go vet`/`go test` staying green across
the whole module, not just the touched packages.

**Tests (Docker-gated, run live in this sandbox, no orphans left after).**
Two new tests, both skip gracefully if no Docker daemon is reachable (same
pattern as the existing `docker_client_test.go`):
- `TestDockerClientServiceAliasDNSResolution`
  (`backend/internal/tests/repository/docker_client_test.go`) - the docker-
  client-level primitive: two `redis:7-alpine` containers on a throwaway
  network, one created with `CreateContainerOpts(..., aliases=[]string{"redis"})`,
  the other execs `getent hosts redis` and must resolve it. (Originally
  written against bare `alpine:3.21`, which has no foreground process and,
  combined with the existing `unless-stopped` restart policy, got stuck
  cycling through Docker's "restarting" state instead of "running" -
  switched to `redis:7-alpine` for both, whose entrypoint stays up, so the
  exec target is actually running when the test execs into it.)
- `TestProjectServiceDeployStackServiceNameAlias`
  (`backend/internal/service/project_service_test.go`, same
  same-package-for-unexported-method precedent as
  `TestProjectServiceCloneRepo`) - exercises `deployStack` itself end to
  end with a real two-service stack (`redis:7-alpine` + `nginx:alpine`,
  `web` `depends_on` `redis`, `pullImages=true`), then execs `getent hosts
  redis` from inside the running `web` container and asserts it resolves.
  This is the "assert deployStack passes svc.Name as the alias" minimum
  the rework asked for, done as a live behavioral check rather than a
  parameter-capture mock (there's no docker-client interface/mock in this
  codebase - `ProjectService.docker` is the concrete `*dockerclient.Client`
  - so a live Docker-gated test is the correct-weight test here, consistent
  with how `docker_client_test.go` already tests `ConnectNetworks`).
  Cleans up its containers/network in `t.Cleanup`; verified no
  `project-1-*` containers or `project-net-1` network survive the run
  (`docker ps`/`docker network ls` checked empty afterward).

**Verification.** `go build ./...`, `go vet ./...`, and `go test ./...
-count=1` (Docker-gated tests included, not skipped - live daemon
available in this sandbox) all pass. `gofmt -l` clean on every touched
file. No orphaned Docker resources left behind by either new test.

### 2026-07-11 — rework: detach Delete/Stop teardown from the request context (TEST-014 re-run item 5 FAIL fix)

**Root cause (confirmed).** `Delete`'s whole Docker-teardown block (service
container stop+remove sweep, the legacy single-`ContainerID` fallback,
`disconnectTraefikFromNetwork`, and `NetworkRemove`) ran on `ctx` — the
request context handed down from the HTTP handler for the `DELETE
/api/projects/:id` call. Backend logs from the re-run showed `disconnect
traefik from project network ... error="... context canceled"` immediately
followed by `remove project network ... error="... context canceled"`:
disconnecting Traefik from `project-net-<id>` briefly reconfigures Traefik
(it's mid-flight proxying the very DELETE request whose context this is),
which can drop that in-flight connection and cancel `ctx` right as
`NetworkRemove` was about to run — aborting teardown with the network still
present but no longer reachable to retry, i.e. a leaked `project-net-<id>`.
Same class of bug as BUG-027 (must-complete cleanup riding a cancelable
request context) - confirmed by the reused pattern (`context.WithTimeout(context.Background(), ...)`)
already precedented in `project_service_test.go`'s own test cleanup.

**Fix — `Delete`** (`backend/internal/service/project_service.go`). The
entire `if s.docker != nil { ... }` block now opens with `cleanupCtx,
cancel := context.WithTimeout(context.Background(), 60*time.Second);
defer cancel()`, and every Docker call inside it (`StopContainer` /
`RemoveContainer` for both the per-service-row sweep and the legacy
single-`ContainerID` fallback, `disconnectTraefikFromNetwork`,
`NetworkRemove`) now takes `cleanupCtx` instead of the request `ctx`. A
client disconnect (or the Traefik-reconfigure blip the disconnect itself
triggers) can no longer abort teardown partway through — the 60s bound
keeps it from hanging forever if Docker itself is unresponsive. Ordering
was already correct (disconnect Traefik before `NetworkRemove`, containers
stopped+removed before either) and is unchanged — the container
stop+remove loop still runs to completion (all on `cleanupCtx`) before
`disconnectTraefikFromNetwork`/`NetworkRemove` are attempted, so no
lingering container endpoint can block the network removal either. The
non-Docker cleanup below (`traefik.RemoveRoute`, the DB deletes, project
row delete, workdir removal) is unaffected — those aren't Docker calls and
weren't implicated in the leak, so they're left on the original `ctx` /
context-independent as before.

**Fix — `Stop`.** Per the task's instruction to check other teardown
paths: `Stop`'s container-stop sweep had the identical shape (loops
`StopContainer` over every persisted service container on the request
`ctx`), so it gets the same fix for consistency/correctness — `cleanupCtx,
cancel := context.WithTimeout(context.Background(), 60*time.Second)`
before the loop, `StopContainer(cleanupCtx, ...)` inside it. `deploy()`'s
use of `context.Background()` for the whole async deploy (already
detached, `Create`'s `go func()`) and `deployStack`'s use of the caller's
`ctx` (correct as-is per the task's instruction — deploy is fine on
whatever context it's already given, since it's not a must-complete
teardown, it's the initial build) are both untouched.

**Not touched.** `Restart` calls `deployStack` on its own `ctx`, which is
unaffected — `deployStack` is a create/start path, not teardown, so it
correctly keeps riding whatever context its caller (`deploy`'s
`context.Background()`, or `Restart`'s own `ctx`) already gives it, per
the task's explicit instruction not to change the deploy path's context
use. The alias fix (bare-service-name `Aliases` on `CreateContainerOpts`/
`ConnectNetworks`) from the previous rework is untouched by this change —
verified by `go test ./...` staying green, including the two live
Docker-gated alias tests added in that rework.

**Verification.** `go build ./...`, `go vet ./...`, `gofmt -l` (touched
file clean), and `go test ./... -count=1` all pass, including the
Docker-gated tests (live daemon available, no orphaned `project-*`
containers/networks left after — checked `docker ps -a`/`docker network
ls` post-run). No new unit test was added for the detached-context fix
itself per the task's own note that it's hard to unit-test without a real
Docker daemon and a context canceled mid-teardown — the real proof is
TEST-014's item-5 re-run, which this rework does not itself execute (per
instruction: do not rebuild/restart the live stack).

## Review Notes

**2026-07-10 — reviewer pass 1**

Verdict: PASS

Scope check: diff confined to the files the task describes (`project_service.go`,
new `deploy_engine.go`/`deploy_engine_test.go`, `docker/client.go`'s
`FindContainerByComposeService`/`PullImage`/`ConnectNetworks`, `cmd/api/main.go`'s
reconcile removal, `docker-compose.yml`'s `tamga-net` removal). The rest of the
dirty tree (frontend files, `go.mod`/`go.sum`, `.claude/`, etc.) predates this
task / belongs to sibling work and is not this developer's doing here.
`go build ./...`, `go vet ./...`, `go test ./...` all pass; touched files are
`gofmt`-clean.

**Crux 1 — per-project network + Traefik reachability + isolation.** Confirmed:
`deployStack` creates/joins `project-net-<id>` per project
(`project_service.go:224-227`, `deploy_engine.go:28-30`), every service container
is created directly on that network (`CreateContainerOpts(..., netName, ...)`,
`project_service.go:253`), and no code path ever attaches two different
projects' containers to the same network — `extraNetworks` even namespaces
same-named `networks:` declarations per-project (`<project-net>-<name>`) so
that can't accidentally collide either. `FindContainerByComposeService`
(`docker/client.go:389`) correctly filters on the `com.docker.compose.service`
label rather than a hardcoded name, and `docker-compose.yml`'s `traefik`
service key is literally `traefik`, so the label match is correct in this
compose file.

The pairwise-isolation reasoning is sound and I verified it empirically against
the live Docker daemon in this environment rather than taking it on faith: created
two bridge networks (`test-netA`/`test-netB`), one container on each, and a
third container attached to *both* (mirroring Traefik-on-two-project-nets).
`ctrA` could ping the shared container's netA-side IP, `ctrB` could ping its
netB-side IP, but `ctrA` could reach neither `ctrB` directly nor the shared
container's netB-side IP — 100% packet loss both ways, exactly the claimed
"pairwise, not transitive" behavior. Test containers/networks cleaned up
afterward.

No dangling `tamga-net` references remain in Go code (grepped the whole repo;
the only hits are stale comments in `agent_service.go`/`sprints/*.md`, pre-existing
and out of this task's scope) or in `docker-compose.yml`. `tamga-network` (the
CORE compose network backend/frontend/traefik share) is untouched, so Tamga's
own UI/API routing is unaffected. Agent sandboxes correctly use `agent-net-<id>`
(`agent_service.go:167`), never `tamga-net`.

**Crux 2 — unified deploy path.** `deploy()` and `Restart()` both branch once on
`project.ComposeYAML != ""` and converge on the single `deployStack` call;
traced `CreateContainerOpts` and confirmed env (`envMapToSlice`), volumes
(`composeVolumesToMounts` → `HostConfig.Binds`), and network are all actually
threaded through, not just computed and dropped. `ports:` is deliberately
*not* host-published (matches TEST-011 §2b's explicit finding that no
host-publish support is needed since Traefik always dials container-to-container
over the shared project network) — declared ports are only used as routing
metadata (`exposedTargetPort`), which is correct given the architecture.

**Crux 3 — exposed-service + routing.** `detectExposedService` is pure and
well covered by `deploy_engine_test.go` (override/single-service/one-port/
ambiguous-zero/ambiguous-multi, all asserted). Route name staying `project-<id>`
regardless of upstream is confirmed directly in `repository/traefik/client.go:74-79`
(`AddRoute` always names router/service `fmt.Sprintf("project-%d", projectID)`).
`ReconcileRoutes` correctly replaces the old free function, uses
`resolveExposedUpstream` (persisted-container-row lookup) instead of assuming a
single container name, and fails safe (`continue`) on a project with no resolved
exposed service or no matching container row — doesn't crash.

**Crux 4 — Delete/lifecycle.** Delete ordering is correct: `disconnectTraefikFromNetwork`
is called before `NetworkRemove` (`project_service.go:608-612`), which matters
because Docker refuses to remove a network with anything still attached.
`ListServiceContainers` iteration + legacy `project.ContainerID` fallback,
explicit `DeleteServiceContainersByProject`/`DeleteEnvVarsByProject` (cascade
doesn't fire without `PRAGMA foreign_keys`, consistent with FEAT-025's finding)
all check out.

**Tests.** `deploy_engine_test.go`'s unit tests are meaningful, not just
type-checking — they assert actual behavior (sorted env output, anonymous-volume
skipping, default-network filtering, all six `detectExposedService` branches,
all three `aggregateStatus` branches) rather than trivially re-stating the
implementation. `aggregateStatus` not being wired to a live endpoint yet is
reasonable — there's genuinely no HTTP surface that would read it (confirmed:
no handler/router reference to it), and the task correctly scoped that to
FEAT-029.

**Non-blocking findings** (none of these change the verdict — see reasoning below):

1. `detectExposedService`'s override branch (`deploy_engine.go:180-186`) trusts
   `project.ExposedService` unconditionally without checking it still names a
   service present in the current `services` list. Because `deployStack` itself
   persists whatever name it auto-detects back onto `project.ExposedService`
   (`project_service.go:291-295`), that field is really "last resolution cache",
   not purely "explicit user override" — if a project's compose service set ever
   changes across a redeploy such that the previously-resolved name disappears
   (rename/removal), `deployStack` would build a route for a container that was
   never created (`byName[exposedName]` silently zero-values,
   `exposedContainerName` points at a nonexistent container,
   `GetContainerPort`'s fallback inspects whatever container was picked as the
   "first" fallback instead) — a "silently produced dead route," the exact
   failure class this task set out to eliminate, via a different trigger.
   Currently **unreachable in practice**: I grepped and confirmed no HTTP handler
   anywhere lets a caller set or change `compose_yaml`/`exposed_service` at all
   yet (that's FEAT-029's create/deploy UI) — the only two live paths
   (`Create`/`Restart` for a git-build project) always synthesize the same
   stable `"app"` service name, so the mismatch can't occur through any exposed
   surface today. Worth hardening (validate the override still exists in
   `services`, falling through to the heuristic otherwise) before or alongside
   FEAT-029 wiring up compose editing.
2. On a partial `deployStack` failure (e.g. service 2 of 3 fails to
   pull/create/start), earlier-started services' containers are never recorded
   (`ReplaceServiceContainers` is one batch call after the whole loop) and
   `project.ContainerID` is set even later — so a subsequent `Delete` won't find
   or clean them up. This mirrors the pre-existing single-container `deploy()`'s
   identical gap (container created+started but `project.ContainerID` only
   assigned after `StartContainer` succeeds, same class of orphan-on-failure),
   just with a larger blast radius now that up to N-1 containers can be silently
   orphaned per failed deploy attempt instead of at most one. Not a regression
   this task introduced, but worth a follow-up (e.g. append-and-persist each
   service's row as it starts, or best-effort cleanup of everything created so
   far on a `deployStack` error return).
3. `Update`'s domain-cleared branch (`project.Domain == ""`) calls
   `traefik.RemoveRoute` but not `disconnectTraefikFromNetwork` — Traefik stays
   attached to that project's network until the project is actually deleted.
   Harmless given the confirmed isolation model, just a minor asymmetry with
   `Delete`'s explicit disconnect.

Core crux items (per-project network, label-based Traefik lookup, pairwise
bridge isolation — empirically verified live, unified deploy path with
env/ports/volumes actually wired through, exposed-service detection + stable
route naming, Delete/Restart/Stop lifecycle rewrite with correct
disconnect-before-remove ordering, and meaningful unit coverage of every pure
helper) are all correctly implemented and match the documented design. `go
build/vet/test` clean. Recommend the two non-blocking findings above become
quick follow-ups around FEAT-029 rather than blocking this hold.

**2026-07-11 — reviewer pass 2 (delta review: TEST-014 alias rework)**

Verdict: PASS

Scope: reviewed only the rework delta per the architect's brief —
`backend/internal/repository/docker/client.go` (`CreateContainerOpts`/
`NetworkConnect`/`ConnectNetworks` alias plumbing), `project_service.go`'s
`deployStack`, and the two new live Docker-gated tests. Since neither the
original FEAT-028 pass nor this rework was committed (both sit uncommitted
in the same working tree, confirmed via `git log` on the two touched
files), there's no commit boundary to `git diff` against — verified the
rework by reading the specific functions/tests against the Implementation
Notes' "rework" section instead, and by diffing `agent_service.go` (the
one file outside the rework's own listed set that necessarily changes)
against `HEAD` to confirm it's exactly the minimal ripple described (four
call sites each gain a trailing `nil`, nothing else).

1. **docker/client.go alias support — confirmed correct.**
   `CreateContainerOpts` (client.go:94) builds
   `NetworkingConfig.EndpointsConfig[netName]={Aliases: aliases}` keyed to
   the *same* `netName` used in `HostConfig.NetworkMode` (client.go:96,
   110-117) — correct SDK usage, and Docker does merge `NetworkMode` +
   matching `EndpointsConfig` entry for a user-defined network at create
   time. Nil/empty `aliases` → `netCfg` stays nil → the `ContainerCreate`
   call is byte-identical to the pre-rework signature's implicit nil.
   `NetworkConnect` (client.go:375-385) and `ConnectNetworks`
   (client.go:398-405) follow the identical nil-safe pattern, threading
   one `aliases` slice to every network in the list. Grepped all callers
   of `CreateContainerOpts`/`NetworkConnect`/`ConnectNetworks`
   (`agent_service.go`, `project_service.go`, `deploy_engine.go`,
   `client.go` itself) — every pre-existing caller (`agent_service.go`'s
   egress-proxy create, sandbox create/recreate, egress-proxy
   `NetworkConnect`) passes `nil` for the new trailing parameter;
   `connectTraefikToNetwork`'s `NetworkConnect` call
   (`project_service.go:370`) also passes `nil` correctly (Traefik is
   found by label, never dialed by a service-name alias). No missed
   callers, no compile breakage — `go build ./...`/`go vet ./...` both
   clean.

2. **deployStack — confirmed correct.** `alias := []string{svc.Name}`
   (project_service.go:268) is the bare compose service name, not the full
   `project-<id>-<service>` container name, and is passed as both
   `CreateContainerOpts`'s alias arg (the project-network join, line 269)
   and `ConnectNetworks`'s alias arg (the extra-networks join, line 283).
   Traefik still resolves the upstream via the full container name
   (`resolveExposedUpstream`/`AddRoute` — unaffected by this change), so
   routing is untouched by the alias addition.

3. **Isolation not regressed — confirmed.** `netName :=
   projectNetworkName(project.ID)` (project_service.go:233) scopes every
   alias to that one project's own network; no code path ever attaches
   two projects' containers (or aliases) to the same network. Project A's
   `redis` alias only resolves on `project-net-A`; project B's identically-
   named `redis` alias lives on its own separate `project-net-B`. The
   pairwise-not-transitive Traefik-joins-both-nets reasoning from pass 1
   is unaffected by this rework (no change to how/which networks Traefik
   joins).

4. **Live tests — RAN both myself, both pass, no orphans.** Docker daemon
   was reachable in this environment; confirmed a clean baseline
   (`docker ps -a`/`docker network ls` showed no leftover `project-1`/
   `project-net`/test containers before running anything), then ran:
   - `go test ./internal/service/ -run TestProjectServiceDeployStackServiceNameAlias -v -count=1`
     → **PASS** (6.97s). Deploys a real two-service stack (`redis:7-alpine`
     depended-on by `nginx:alpine` as `web`) via `deployStack` itself,
     execs `getent hosts redis` inside the running `web` container, and
     asserts the bare name resolves — this is the direct re-verification
     of TEST-014's exact failure mode (previously NXDOMAIN, now resolves).
   - `go test ./internal/tests/repository/ -run TestDockerClientServiceAliasDNSResolution -v -count=1`
     → **PASS** (4.37s). Lower-level primitive check: two `redis:7-alpine`
     containers on a throwaway network, one created with alias `"redis"`
     via `CreateContainerOpts`, the other execs `getent hosts redis` and
     resolves it.
   - Post-run `docker ps -a`/`docker network ls` grepped for
     `project`/`alias`/`test` → **empty both times** — `t.Cleanup` in both
     tests tears down its containers/network correctly, no orphans left
     behind.
   - Ran the full suite after (`go test ./... -count=1`) — all packages
     pass, Docker state still clean afterward.

5. **go build/vet/test — clean.** `go build ./...`, `go vet ./...`,
   `gofmt -l` (on the touched files) all clean; `go test ./... -count=1`
   passes end to end including the Docker-gated tests (not skipped, live
   daemon available).

No new findings beyond pass 1's three already-recorded non-blocking items
(override-vs-heuristic staleness, partial-deploy orphan risk, `Update`'s
domain-cleared branch not disconnecting Traefik) — none of those are
touched by or related to this alias rework. TEST-014's item 2 (service-
name DNS resolution) is directly and empirically fixed; recommend this
hold move forward.

**2026-07-11 — reviewer pass 3 (delta review: detached-context Delete/Stop teardown fix)**

Verdict: PASS

Scope: reviewed only the delta described in the rework note —
`backend/internal/service/project_service.go`'s `Delete` (line 603) and
`Stop` (line 395), the `cleanupCtx := context.WithTimeout(context.Background(),
60*time.Second)` change and its threading through the Docker teardown
calls. `backend/internal/repository/docker/client.go`'s diff (146 lines,
`+126/-20`) is the alias plumbing from the *previous* rework (already
reviewed in pass 2) — confirmed no `context.Background()`/teardown-related
change was added to it in this delta (its only `WithTimeout` is the
pre-existing 5s exec-inspection context, unrelated). `deploy_engine.go` is
untouched (untracked from an earlier task pass, not part of this delta).

1. **Delete — confirmed correct.** `cleanupCtx, cancel :=
   context.WithTimeout(context.Background(), 60*time.Second)` opens the
   `if s.docker != nil` block (project_service.go:623-624), `defer
   cancel()` alongside it. Every Docker call inside the block uses
   `cleanupCtx`, not `ctx`: the per-service-row `StopContainer`/
   `RemoveContainer` loop (lines 631, 634), the legacy single-`ContainerID`
   fallback (lines 643, 646), `disconnectTraefikFromNetwork(cleanupCtx,
   netName)` (line 657), and `NetworkRemove(cleanupCtx, netName)` (line
   658). Grepped the whole Delete function body (lines 603-685) for a bare
   `ctx` reference outside the function signature/comments — none found;
   no stray docker call still rides the cancelable request context.
   Ordering is preserved and correct: the stop+remove sweep (both the
   per-row loop and the legacy fallback) runs to completion before
   `disconnectTraefikFromNetwork`/`NetworkRemove` are attempted (lines
   626-660), so a still-attached container endpoint can't be the reason
   `NetworkRemove` fails — consistent with the documented "Docker refuses
   to remove a network with anything attached" constraint. The non-Docker
   steps below (`traefik.RemoveRoute` at line 663, the three DB deletes at
   667-674, `DeleteProject` at 677, `os.RemoveAll(workDir)` at 682) are
   correctly left on the original request `ctx` / unaffected by any ctx at
   all — file and DB operations here don't take a `context.Context`
   argument in this codebase's repository interfaces, and none of them was
   implicated in the leak (the leak was specifically Docker API calls
   racing a canceled request context), so leaving them alone is correct,
   not an oversight.

2. **Stop — confirmed correct and consistent with Delete.** Same shape:
   `cleanupCtx, cancel := context.WithTimeout(context.Background(),
   60*time.Second)` before the loop (project_service.go:411-412),
   `StopContainer(cleanupCtx, ...)` inside it (line 414), `defer cancel()`
   present. No stray `ctx` usage in the function body.

3. **No regression in the deploy path — confirmed.** `deployStack`
   (project_service.go:233) still takes and uses the caller's `ctx`
   throughout (`EnsureNetwork`, `ContainerExists`, `RemoveContainer`,
   `PullImage`, `CreateContainerOpts`, `StartContainer`,
   `ConnectNetworks` — all on `ctx`, none on `cleanupCtx`); `Restart`
   (line 698) calls `s.deployStack(ctx, ...)` on its own request context,
   untouched. `deploy()` (line 109) is likewise untouched. The alias fix
   from the prior rework is intact: `alias := []string{svc.Name}`
   (project_service.go:269) still passed to both `CreateContainerOpts`
   and `ConnectNetworks`. `time` is imported (`"time"`, confirmed in the
   import block) and actually used (`60*time.Second` in both Delete and
   Stop) — no unused-import risk.

4. **Timeout sanity.** 60s is ample for a stop+remove sweep over N
   service containers plus one `NetworkRemove` call, especially given
   BUG-025 already made sandbox stops fast; this isn't a hot path bounded
   by user-perceived latency (it's a detached background teardown, not
   something the original HTTP response is still waiting on by this
   point). If `NetworkRemove` fails for a *non*-context reason (e.g. a
   genuinely still-attached endpoint from a code path this review didn't
   find, or a Docker-side race), the fix doesn't silently loop or hang —
   it logs via `slog.Warn` (project_service.go:659) and Delete proceeds to
   remove the DB rows/project regardless. That's an acceptable posture
   here: the specific, confirmed leak cause (context cancellation racing
   the Traefik-reconfigure blip) is what this rework targets and fixes;
   a separate, still-attached-endpoint failure is a different, logged,
   diagnosable condition, not silently swallowed data loss — consistent
   with this codebase's established non-fatal-cleanup-step posture used
   throughout `Delete` already (route removal, DB deletes are all
   `slog.Warn`-and-continue, not `return err`).

5. **go build/vet/test — clean, verified myself.** `go build ./...`,
   `go vet ./...` both clean. `gofmt -l` on the touched file (`git diff`
   scope) reports nothing. `go test ./... -count=1` passes end to end,
   including the Docker-gated tests (`TestProjectServiceDeployStackServiceNameAlias`,
   `TestDockerClientServiceAliasDNSResolution`) — live daemon was
   reachable in this environment. Checked `docker ps -a`/`docker network
   ls` for `project`/`redis`/`nginx`/`test-net` both before and after the
   run — empty both times, no orphaned resources left by the test suite
   itself (separate from, and not proof of, the live TEST-014 item-5
   re-verification, which per the brief is the architect's next step, not
   mine).

No new findings. This delta is narrowly scoped, correctly threads the
detached `cleanupCtx` through every Docker call in both teardown paths
(and only those), preserves ordering, doesn't touch the deploy path or
the alias fix, and doesn't introduce a new hang/leak risk. Recommend
proceeding to the live TEST-014 item-5 re-run.


## Test Notes
<filled in by tester>

---
id: TEST-011
type: test
title: Project deploy pipeline audit → unified compose-based model design
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 phase-1 audit"}
  - {date: 2026-07-10, stage: development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-10, stage: review, by: architect, note: "audit complete, BUG-029 filed (folds into compose-deploy); moved to review"}
  - {date: 2026-07-10, stage: done, by: architect, note: "review PASS (gaps independently confirmed); test-stage direct; task complete"}
---

## Summary
SPRINT-004 unifies deployment: a project becomes a docker-compose stack,
and today's single-container git-build flow is folded into that as a
1-service compose. Before planning that, document exactly how a project is
built and deployed today, then design the unified compose model (common
subset only) — so the phase-2 implementation is grounded.

## Scope
- **Current deploy pipeline (read the code):** trace a project from create
  → deploy → running, in `backend/internal/service/project_service.go` and
  the docker repository (`backend/internal/repository/docker/*`): git clone
  (git-credential use), image build from the repo Dockerfile, container
  create/start, the per-project network, container naming
  (`project-<id>`), status transitions, domain → Caddy route, stop/delete.
  Document each step with file:line and the domain types involved
  (`domain/project.go`, deployment records).
- **Unified compose model — design (the phase-2 spec):**
  - How a git-build project maps to a 1-service compose (the built image
    becomes that one service; ports/env come from where today?). One
    deploy path for both project kinds.
  - Supported compose subset: `image`, `ports`, `environment`, `volumes`,
    `networks`, `depends_on` — for each, how it maps onto the docker
    repository's existing create-container primitives (does the docker
    client already support volumes/networks/depends-on-ordering, or is new
    plumbing needed? cite what exists). `build:` etc. are out of scope.
  - Multi-service deploy onto one per-project network so services reach
    each other by name; exposed-service detection (which service gets the
    project domain by default — port/expose heuristic) and how a
    user-chosen domain→service binding overrides it.
  - Lifecycle for a whole stack: up / down / status / delete across N
    containers; how "project status" aggregates N container states.
  - Storage: does a project need to persist its compose definition + the
    domain→service bindings? Propose the schema addition (a migration).
- **Interaction with TEST-010's routing choice:** note how the exposed
  service is wired to Traefik (defer the mechanism to TEST-010, just state
  the seam).

## Out of Scope
- Implementing it (phase 2).
- Traefik/TLS specifics (TEST-010) and the map (TEST-012).
- Full compose spec beyond the subset.

## Test Approach
Two-pass audit, no production code changes.

**Pass 1 — read the current deploy pipeline end-to-end.** Read
`backend/internal/service/project_service.go` in full (`Create`, `deploy`,
`cloneRepo`/`initRepo`, `buildImage`, `Delete`, `Restart`, `Update`),
`backend/internal/repository/docker/client.go` in full (every exported
primitive, not just the ones already known to be called from
`project_service.go`), `backend/internal/domain/project.go`,
`backend/internal/domain/deployment.go`, and
`backend/internal/repository/sqlite/project_repo.go` (the SQL shape backing
`domain.Project`). Cross-checked `backend/internal/handler/project_handler.go`
and `backend/internal/handler/container_handler.go` to confirm no
aggregate/multi-container status concept exists anywhere today (single
`ContainerID` field, single-container assumptions throughout). Grepped for
`agentNetworkName`/`NetworkConnect`/`ImagePull` across `backend/` to find
every existing precedent (or absence of one) the compose design can reuse.
Cross-referenced `tasks/done/TEST-010-...md` (already `done`, reviewed,
folds BUG-028 in) and `tasks/active/BUG-028-...md` rather than re-deriving
their findings, per this task's explicit "coordinate with TEST-010" scope
item.

**Pass 2 — verify live, read-only, without disrupting the running stack.**
`docker ps -a`, `docker network ls`, `docker inspect tamga-caddy-1
--format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}'`, and
`docker network inspect tamga-net --format '{{range .Containers}}{{.Name}}
{{end}}'` against the already-running dev stack — confirmed BUG-028 is
still live/unfixed (caddy only on `tamga_tamga-network`) and that no
project is currently deployed in this session (`tamga-net` has zero
members), consistent with both audits. No project was deployed and no
container/network was created, modified, or removed in this session — the
only write here is the new `BUG-029` task file and this task's own
sections.

**Pass 3 — compose-parsing library research.** WebSearch +
`WebFetch` against `github.com/compose-spec/compose-go` to confirm it is
the actual reference library the real `docker compose` CLI uses (not a
third-party reimplementation), its import path/version, and license.
Docs-only, labeled as such below — no code was written against it, this is
a recommendation for phase 2.

## Affected Areas
Findings only — no production code was changed for this audit (`git
status`/`git diff` show no changes under `backend/`, `docker-compose.yml`,
or `deploy/`; new files are this task's own sections plus the newly-filed
`tasks/active/BUG-029-shared-flat-project-network-no-isolation.md`).

- `backend/internal/service/project_service.go` — read only. `deploy`
  (91-183), `Delete` (288-324), `Restart` (334-380), `Update` (404-428),
  `cloneRepo`/`initRepo` (185-230), `buildImage` (232-278) all traced
  end-to-end (Implementation Notes §1). These are exactly where phase-2's
  N-container/compose deploy logic replaces the current single-container
  logic.
- `backend/internal/repository/docker/client.go` — read only, every
  exported primitive inventoried (Implementation Notes §1/§2). Confirmed
  **no `ImagePull` method exists anywhere** in this file or any service
  (grepped) — a real gap for the compose model's `image:` field when it
  names a not-yet-local image (§2b).
- `backend/internal/domain/project.go`, `backend/internal/domain/deployment.go`,
  `backend/internal/repository/sqlite/project_repo.go` — read only; the
  single `ContainerID string` field on `domain.Project` and the
  single-row-per-deploy shape are exactly what the storage proposal (§2e)
  needs to extend for N containers per project.
- `backend/internal/handler/project_handler.go`,
  `backend/internal/handler/container_handler.go` — read only; confirmed
  no aggregate-status concept exists anywhere in the handler layer today
  (single-container assumption throughout), informing §2d.
- `backend/internal/service/agent_service.go:166-168`
  (`agentNetworkName`), `:223,233` (`NetworkConnect` used to attach the
  egress-proxy to multiple per-sandbox networks after creation) — read
  only; these are the existing precedents the compose design's
  multi-network handling (§2b/§2c) and per-project network naming (§2c)
  directly reuse.
- **New: `tasks/active/BUG-029-shared-flat-project-network-no-isolation.md`**
  — a real, currently-live defect found while tracing the network side of
  `deploy()`/`Restart()`: every deployed project is attached to the same
  literal, unparameterized `"tamga-net"` network
  (`project_service.go:140,149,361,365`), so any project's container can
  resolve and reach any other project's container by container name, with
  no per-project isolation boundary — unlike the `agent-net-<projectID>`
  pattern `agent_service.go` already established for sandboxes. Distinct
  from BUG-028 (that one is proxy→project reachability being *absent* when
  it should exist; this one is project→project reachability *existing*
  when it should not). Filed per this task's process (audit only, not
  fixed here); explicitly noted in §2c below since the unified compose
  model's "one per-project network" design fixes it as a structural side
  effect, the same relationship BUG-028 has to the Traefik migration.
- No other defect surfaced. The `Update()` domain-route gap and the
  `caddy`/`tamga-net` disjoint-network defect are both already covered by
  TEST-010/BUG-028 and are not re-filed here.

## Acceptance Criteria
- [ ] Every Scope item exercised; concrete file:line evidence per finding
- [ ] Current deploy pipeline documented end-to-end (clone→build→run→route→
      stop/delete) with file:line and the domain/DB types involved
- [ ] A concrete unified-compose model: how single-container folds into
      1-service compose, and how each subset feature maps onto existing
      docker-client primitives (with a clear list of what's already
      supported vs what needs new plumbing)
- [ ] Exposed-service detection + domain-override design stated
- [ ] Proposed schema/migration for persisting the compose def + domain
      bindings (or a clear statement none is needed and why)
- [ ] Stack lifecycle (up/down/status/delete across N services) designed
- [ ] Any defect found filed as its own BUG-XXX task
- [ ] Detailed enough to write phase-2 deploy tasks directly

## Test Plan
A tester re-verifying this audit's claims can:

**Current pipeline (static, code-level):**
1. Read `project_service.go:91-183` (`deploy`) — confirm the six numbered
   steps in Implementation Notes §1 match exactly (clone/init →
   build → create+start container on `tamga-net` → register Caddy route
   (non-fatal) → set `ProjectStatusRunning` → create a `Deployment` row).
2. `grep -n "ImagePull" backend/internal/repository/docker/client.go
   backend/internal/service/*.go` → confirms zero results, backing the
   "`image:` needs new plumbing for pre-built images" claim in §2b.
3. `grep -n "agentNetworkName" backend/internal/service/agent_service.go`
   → confirms the per-project network naming precedent (`agent-net-%d`)
   cited in §2c/BUG-029 actually exists as claimed.

**Live, read-only (do not disrupt the running stack):**
1. `docker inspect tamga-caddy-1 --format '{{range $k,$v :=
   .NetworkSettings.Networks}}{{$k}} {{end}}'` → only
   `tamga_tamga-network` (BUG-028 still live, matches TEST-010).
2. `docker network inspect tamga-net --format '{{range .Containers}}{{.Name}}
   {{end}}'` → confirms whether any project is currently deployed (empty
   in this session).
3. **To reproduce BUG-029 end-to-end** (not run in this session, since no
   project is currently deployed — would require deploying two real
   projects, which this audit deliberately avoided to stay read-only):
   deploy two projects with different sources, then `docker network
   inspect tamga-net` → confirm both `project-<a>` and `project-<b>`
   appear as members of the *same* network, then `docker exec project-<a>
   ...` a request to `http://project-<b>:<port>` → expect it to succeed,
   confirming cross-project reachability with no isolation.

**Compose-parsing library claim:** `https://github.com/compose-spec/compose-go`
→ confirm it's listed as used by the official `docker/compose` CLI ("Used
by" section) and the module path/license match §2f.

All Pass-1/Pass-2 items above were run in this session by the developer
(see Test Approach); a tester re-running steps 1-2 (static) and 1-2 (live)
should get identical results. Step 3 (an actual two-project deploy) was
**not** run by the developer in this session (kept read-only per Scope) —
a tester picking up BUG-029 standalone should run it as that bug's own
verification.

## Implementation Notes

### 1. Current deploy pipeline — end to end

**Create → deploy (async).** `ProjectService.Create` (`project_service.go:48-89`)
inserts a `projects` row (`Status: ProjectStatusCreated`), snapshots it for
the HTTP response (avoids a data race with the background goroutine, see
the comment at 70-77 / BUG-011), then kicks off `deploy()` in a detached
goroutine (80-86) — deploy is entirely fire-and-forget from the HTTP
handler's perspective; failures only ever surface via polling `GET
/projects/:id` and seeing `Status: "error"`.

`deploy()` (`project_service.go:91-183`), six steps, all against a single
`*domain.Project`:
1. **Clone/init** (97-115): `Status = ProjectStatusCloning`, persisted
   immediately. `SourceTypeLocal` → `initRepo` (plain `git init` into
   `<DataDir>/projects/<id>`, 185-199). Otherwise → `cloneRepo` (201-230):
   `git clone --branch <branch> --single-branch --depth 1 <url> <workDir>`.
   **Git credentials**: a single *global* credential (not per-project) —
   `GitCredentialService.AuthenticatedCloneURL(repoURL)` (called at
   214-220) rewrites the clone URL to embed a stored username:token if one
   is configured (`git_credential_service.go:89-120`,
   `injectToken`/149), falling back to an unauthenticated clone on any
   error. A failed clone falls back to `initRepo` rather than failing the
   whole deploy (108-111) — an empty git repo, not a build failure.
2. **Build** (117-124): `Status = ProjectStatusBuilding`. Tag is always
   `tamga-project-<id>` (120). `buildImage` (232-278) tars the entire
   `workDir` in memory and calls `docker.BuildImage(ctx, tag, "Dockerfile",
   &buf)` (277) — **the Dockerfile path is hardcoded to `"Dockerfile"` at
   the repo root**, no per-project override, no `build.context`/`dockerfile`
   equivalent today. `BuildImage` (`docker/client.go:34-47`) is a thin
   wrapper over `ImageBuild` (`Tags`, `Dockerfile`, `Remove: true`,
   `PullParent: true`) — no build args, no target stage, no `.dockerignore`
   honoring beyond whatever `ImageBuild` does natively.
3. **Run container** (126-157): `containerName := "project-<id>"`
   (127). If a same-named container already exists it's force-removed
   first (128-130, handles a re-deploy of the same project ID). Ensures
   the network `EnsureNetwork(ctx, "tamga-net", false)` (140, **the same
   literal name for every project — see BUG-029**), reads this project's
   env vars (`ListEnvVars`, 144-147) and converts them via
   `envVarsToSlice` (385-394, `"KEY=VALUE"` slice), then
   `CreateContainer(ctx, containerName, tag, env, "tamga-net")` (149) →
   `StartContainer` (153). `project.ContainerID` is set (156) but **not
   yet persisted** at this point (only written to the in-memory struct;
   the DB write happens at step 5).
4. **Register Caddy route** (159-169): `GetContainerPort` (`docker/client.go:152-161`)
   inspects the just-started container and returns the first key of
   `NetworkSettings.Ports` (defaults to `"80"` on error or if nothing is
   exposed — this port is **entirely implicit**, discovered post-hoc from
   whatever the built image's own Dockerfile declared via `EXPOSE`, never
   something Tamga's own config states). `upstream = "project-<id>:<port>"`,
   `s.caddy.AddRoute(project.Domain, upstream)` — **non-fatal** (a failure
   is `slog.Warn`'d only; the deploy is considered successful regardless,
   161-168). See TEST-010 for the full Caddy route lifecycle/BUG-028 (the
   route this call registers cannot actually be reached — disjoint
   networks).
5. **Done** (171-173): `Status = ProjectStatusRunning`, persisted.
6. **Deployment record** (175-180): one `domain.Deployment` row is
   inserted with `Status: DeploymentStatusSuccess` — note this only ever
   happens on the success path; a failed `deploy()` (any `return
   fmt.Errorf(...)` above) never creates a `Deployment` row at all (the
   caller in `Create`'s goroutine only sets `project.Status =
   ProjectStatusError` on the `Project` itself, 80-86) — `Deployment`'s
   `Pending`/`Running`/`Failed` status constants (`domain/deployment.go:8-11`)
   are declared but never actually assigned anywhere in this codebase
   today. Not filed as a bug (no Scope/Acceptance Criteria in any prior
   task requires deployment-history-on-failure), just a documentation
   nuance worth carrying into the compose redesign since N-service
   deploys will have more, and more interesting, partial-failure states.

**Container naming / image tagging**: `project-<id>` (container),
`tamga-project-<id>` (image tag) — both purely ID-derived, no name
collision risk across projects, confirmed the single naming convention
used throughout `deploy`/`Restart`/`Delete`.

**Status transitions**: `created → cloning → building → running`, or
`→ error` from any step's failure (set by the `Create` goroutine's error
handler, not inside `deploy()` itself). No `stopping`/`stopped` status
exists — `Delete` doesn't transition through an intermediate status, it
stops+removes the container then deletes the whole project row outright
(below). `Restart` doesn't change `Status` either (334-380) — it silently
assumes the project is already `Running` and re-creates the container from
already-persisted state (env vars + the already-built image tag, no
reclone/rebuild — explicitly documented at 326-333 as a deliberate
consequence of Docker having no way to inject env changes into a running
container, BUG-021).

**Delete** (`project_service.go:288-324`): find project (289-292), if it
has a `ContainerID`, `StopContainer` then `RemoveContainer` (294-301, both
non-fatal/`slog.Warn` only), if it has a `Domain`, `RemoveRoute` (303-307,
non-fatal), delete `Deployment`s (309-311) and `EnvVar`s (312-314) for the
project, delete the `projects` row itself (316-318), then `os.RemoveAll`
the on-disk `workDir` (320-321). **The `tamga-net` network itself is never
removed or even considered** — consistent with it being a shared,
unscoped network today (BUG-029); if/when it becomes per-project (§2c),
`Delete` needs a `NetworkRemove` step added.

**Update** (`project_service.go:404-428`): pure DB-row patch (name,
source_type, repo_url, domain, branch) — **zero Docker or Caddy calls**,
confirmed by reading the full function body. A domain change here doesn't
touch the running container or the proxy route at all (TEST-010's
already-documented gap, not re-covered here).

**Domain types involved**: `domain.Project` (`domain/project.go`, single
`ContainerID string` field — one container per project, hard assumption
baked into the type itself) and `domain.Deployment`
(`domain/deployment.go`, one row per successful deploy, `ProjectID` FK,
`Status`/`CommitSHA`/`Logs` fields — `CommitSHA`/`Logs` are declared but
never populated anywhere in `project_service.go`, confirmed by grep).
Backing SQL: `project_repo.go` (73 lines, straight
`INSERT`/`SELECT`/`UPDATE`/`DELETE` against a single `projects` table,
`migrations/000002_create_projects.up.sql` + `000006_add_source_type.up.sql`
for the schema).

**Docker repository primitives inventory** (`docker/client.go`, 469
lines) — everything the compose redesign has to build on:
- `BuildImage(ctx, tag, dockerfile, buildCtx io.Reader) error` (34-47) —
  image build from a tar stream, tags+dockerfile-path only.
- `CreateContainer(ctx, name, imageName, env, network) (string, error)`
  (49-51) — thin wrapper over `CreateContainerOpts` with `nil`
  mounts/zero resources/no init process.
- `CreateContainerOpts(ctx, name, imageName, env, network, mounts
  []string, resources container.Resources, initProcess bool) (string,
  error)` (58-82) — the actual primitive. `container.Config` sets only
  `Image` + `Env` (no `ExposedPorts`, no `Labels`, no `WorkingDir`, no
  `Cmd`/`Entrypoint` override). `HostConfig` sets a **single**
  `NetworkMode` (one network only, string), `RestartPolicy:
  unless-stopped` (always), `Resources`, optional `Init`, and `Binds`
  built from the `mounts []string` (each entry forwarded verbatim after a
  "contains at least one colon" sanity check, 68-73).
- `StartContainer`/`StopContainer`/`StopContainerTimeout`/`RemoveContainer`
  (84-99) — straightforward per-container lifecycle calls.
- `GetContainerPort(ctx, containerID) (string, error)` (152-161) — as
  described in step 4 above; picks the first key of a Go map
  (`NetworkSettings.Ports`), so with more than one exposed port the choice
  is **not deterministic** across calls (Go map iteration order) — a
  latent subtlety, not filed as a bug here (today's single-container path
  only ever has one meaningfully-exposed port in practice), but the
  compose model's explicit `ports:`/`exposed_service` design (§2b/§2c)
  sidesteps it entirely by never relying on this fallback for anything the
  compose file already states explicitly.
- `EnsureNetwork(ctx, name, internal bool) error` / `NetworkExists` /
  `NetworkConnect(ctx, networkName, containerName) error` (idempotent) /
  `NetworkDisconnect` / `NetworkRemove` (278-340) — full network lifecycle
  already exists. `NetworkConnect` is already used to attach a container
  to a network **beyond** the one it was created on
  (`agent_service.go:233`, the egress-proxy joining every active
  per-sandbox network) — this is the exact mechanism §2b/§2c reuse for
  compose services declaring more than one network.
- **No `ImagePull` method exists anywhere** in this file or any service
  (confirmed by `grep -rn "ImagePull" backend/`) — `ContainerCreate` does
  not auto-pull a missing image; this is a real, concrete gap for the
  compose model's `image:` field (§2b).
- Everything else (`ContainerLogs*`, `ContainerExists`/`IsRunning`,
  `ListContainers`, `InspectContainer`, `ContainerStats`,
  `RestartContainer`, `UpdateContainerResources`, `Prune*`, `ContainerEnv`,
  `Exec*`, `DockerInfo`) is orthogonal to the deploy path itself (used by
  the container-management UI / agent sandboxes), not part of the deploy
  primitive set, not re-described here.

### 2. Unified compose model — design

#### 2a. Folding a git-build project into a 1-service compose

The built image (`tamga-project-<id>`, from `buildImage`/step 2 above)
becomes that single service's `image:` — the build step itself is
unchanged (still `git clone` → `docker build` against the repo's root
`Dockerfile`, still Out of Scope to change per this task's own "build:
is out of scope" note — only the *post-build* deploy step folds into
compose). Concretely: after `buildImage` succeeds, phase-2's deploy path
synthesizes an in-memory 1-service compose definition
(`services: { <project-name-or-"app">: { image: tamga-project-<id>,
environment: <from env_vars table, unchanged source>, ports: [<GetContainerPort
result, or user-declared>], networks: [project-net-<id>] } }`) and feeds
it through the exact same N-service deploy path real multi-service
projects use (§2d) — **one deploy path for both kinds**, not two parallel
code paths. Ports/env for this folded case come from exactly where they
come from today: env from the `env_vars` table (unchanged), the exposed
port from `GetContainerPort`'s post-create inspection (unchanged fallback
for the common "just has one EXPOSE" case) unless the project's compose
definition explicitly states a `ports:`/`expose:` value (new, optional,
only relevant for `SourceType`s that ship their own compose file directly
rather than a bare Dockerfile — out of this task's Scope to design that
ingestion path further, just noting the seam).

#### 2b. Supported compose subset → existing docker-client primitives

| Field | Status | Mapping / gap |
|---|---|---|
| `image:` | **Partially supported** | `CreateContainerOpts`'s `imageName string` already accepts any image reference. **Gap**: no `ImagePull` anywhere in `docker/client.go` or any service (confirmed by grep) — `ContainerCreate` does not auto-pull, so any service whose `image:` isn't already local (e.g. a `postgres:16` sidecar) needs a new `ImagePull` primitive added before container creation. The folded git-build service (§2a) never hits this gap since `BuildImage` already produces its image locally. |
| `ports:` | **Needs plumbing** | No explicit port concept exists today — `GetContainerPort` only discovers a port *after* the fact from the image's own `EXPOSE`. The compose subset needs this to become config-driven (parsed from `ports:`/`expose:`) so services built from a bare `image:` (no Dockerfile, no `EXPOSE` to introspect) still resolve a dial-able port. No host-publish (`"8080:80"`-style) support needed — nothing in the current design publishes ports to the Docker host at all; the proxy always dials container-to-container over the shared per-project network (§2c), so only the container-side port number matters. |
| `environment:` | **Already fully supported** | `CreateContainerOpts`'s `env []string` (`"KEY=VALUE"`) is threaded straight to `container.Config.Env` (`docker/client.go:76`); `envVarsToSlice` (`project_service.go:385-394`) already does exactly this conversion. Zero docker-client changes; only the compose-parsing layer needs to normalize compose's list-or-mapping `environment:` shape into the same slice. |
| `volumes:` | **Already fully supported** | `CreateContainerOpts`'s `mounts []string` (`docker/client.go:58-73`) forwards each string straight into `HostConfig.Binds` after only a "has a colon" check. Docker's own `Binds` field natively accepts both `host-path:container-path[:ro]` bind mounts *and* `volume-name:container-path[:ro]` named volumes (auto-creating the volume) in the identical string shape — so both compose `volumes:` flavors already map 1:1 onto the existing primitive; only compose-parsing needs to serialize each entry into that `"source:target[:mode]"` string (the same shape `agent_service.go:344` already builds by hand today). |
| `networks:` (multiple per service) | **Needs new orchestration, not new docker-client code** | `CreateContainerOpts`'s `network` parameter is singular (one `NetworkMode` at creation). But `NetworkConnect(ctx, networkName, containerName)` (`docker/client.go:315-321`, idempotent) already exists and is already used exactly this way — the egress-proxy attaching to every active per-sandbox network beyond the one it was created on (`agent_service.go:233`). Multi-network compose services: create on the primary/per-project network via `CreateContainerOpts`, then loop `NetworkConnect` for every additional declared network. New service-layer logic, zero new docker-client primitives. |
| `depends_on:` | **Needs new plumbing entirely at the service layer** | Nothing in `docker/client.go` (nor the Docker Engine API itself) has any ordering/dependency primitive. The deploy path needs its own topological sort over the compose file's `depends_on:` edges (subset: plain ordering only, no `condition:` per Out of Scope) and must create+start services in that order, sequentially, before moving to the next — new logic in the `ProjectService` replacement, no docker-client gap. |
| `build:` / `profiles:` / `secrets:` | Out of scope | Not mapped, per this task's own Scope. |

#### 2c. Multi-service deploy, one per-project network, exposed-service detection, domain override

**One network per project.** Today all projects share one literal
`"tamga-net"` (BUG-029). The compose model should give each project its
own network, `project-net-<id>` — this mirrors a pattern already
established elsewhere in this exact codebase:
`agent_service.go:166-168`'s `agentNetworkName(projectID) string {
return fmt.Sprintf("agent-net-%d", projectID) }` for agent sandboxes.
Every service in a project's compose stack joins this one network by
default (services resolve each other by service name, standard Docker
embedded DNS, no extra config needed) — additional `networks:` per §2b
are attached on top via `NetworkConnect`. This single change closes
BUG-029 as a structural side effect, exactly as BUG-028 gets closed by
the Traefik migration's compose shape (TEST-010 §6).

**Exposed-service detection (default, no user input).** Compose itself
has no built-in "this is the public one" concept, so a heuristic is
needed:
1. If exactly one service in the stack declares `ports:`/`expose:`, that
   service is the default exposed service (mirrors the folded single
   git-build case, §2a, where there's only ever one service anyway — so
   the heuristic is a no-op/free win there).
2. If zero or more-than-one services declare a port, there's no safe
   default — a domain route cannot be inferred, and deploy should require
   an explicit user choice before a domain can be attached (see below).

**User-chosen domain→service binding (override).** Always wins over the
heuristic when set. Concretely: `project.Domain` (unchanged, still one
string) needs a companion `project.ExposedService` (service name) —
NULL/empty falls back to the heuristic above; non-empty pins the domain to
that specific service regardless of how many services declare ports.
Surfaced in the UI as a service picker once a project has more than one
service (single-service/folded projects never see this control — no
Scope reason to add UI complexity where the heuristic is always
unambiguous).

#### 2d. Whole-stack lifecycle: up / down / status / delete across N containers

- **Up**: topologically order services by `depends_on:` (§2b), then for
  each in order: `EnsureNetwork(project-net-<id>, false)` (once per
  project, idempotent), `CreateContainerOpts` (image/env/volumes/primary
  network), loop `NetworkConnect` for any additional declared networks,
  `StartContainer`. Persist each service's resulting container ID as it's
  created (see storage, §2e) rather than only at the very end — a partial
  failure partway through a multi-service `up` should leave a
  reconstructable record of which services did start, unlike today's
  single-container path where step ordering doesn't matter for this
  reason.
- **Down**: for every known service-container of the project (from
  storage, §2e), `StopContainer` — leaves containers in place (stopped),
  mirrors `docker compose down` without `--volumes`/`--rmi`. Reverse
  dependency order is a nice-to-have (a dependent service stopping first
  is harmless for `StopContainer`, unlike `up`'s ordering requirement)
  but not correctness-critical for this subset.
- **Status aggregation**: no aggregate-status concept exists anywhere
  today (confirmed via `project_handler.go`/`container_handler.go`, single
  `ContainerID` assumption throughout). Proposal: keep the existing
  `ProjectStatus` enum (`created`/`cloning`/`building`/`running`/`error`)
  rather than inventing new states — those first three already describe
  the pre-container phase unchanged (build is still one step, whole-stack,
  before any container exists). Post-build, derive the aggregate from
  querying `ContainerIsRunning`/`InspectContainer` for every persisted
  service-container: **all running → `Running`**; **any expected service
  container isn't running → `Error`** (deliberately coarse — "degraded"/
  partial states aren't in this subset's Acceptance Criteria and would be
  scope creep; a project owner needing to know *which* service is down
  already has the existing per-container inspect/logs UI, this is just the
  one-line project-list-view status).
- **Delete**: mirrors today's `Delete()` (`project_service.go:288-324`)
  but loops `StopContainer`+`RemoveContainer` over every persisted
  service-container instead of one, removes the domain route (§3, seam
  with TEST-010), then `NetworkRemove(project-net-<id>)` once no
  containers remain attached (a new step — today's `Delete` never touches
  the network at all, consistent with the network being unscoped/shared
  today per BUG-029).

#### 2e. Storage: yes, persistence is needed

Two things need to persist beyond what `domain.Project` already stores:
(1) the project's compose definition (or enough of it to redeploy/restart
without re-deriving it), and (2) the domain→service binding, and (3) a
way to track more than one container ID per project (today's schema has
exactly one `container_id` column, a hard single-container assumption).

Proposed migration, following this codebase's existing patterns (plain
`ALTER TABLE ADD COLUMN` for scalar additions like
`000006_add_source_type.up.sql`; a project-scoped child table with
`project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE`
like `000005_create_env_vars.up.sql`):

```sql
-- 000016_create_project_services.up.sql (next available number)
ALTER TABLE projects ADD COLUMN compose_yaml TEXT;
-- Raw subset-compose YAML for this project. NULL for any project that
-- predates this migration until its next deploy re-writes it. The
-- git-build 1-service fold (§2a) writes one here too, so this column is
-- populated for every project going forward, not just multi-service ones.
ALTER TABLE projects ADD COLUMN exposed_service TEXT;
-- Service name project.Domain routes to. NULL/empty = fall back to the
-- single-published-port heuristic (§2c).

CREATE TABLE IF NOT EXISTS project_service_containers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    service_name TEXT NOT NULL,
    container_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'created',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, service_name)
);
```
`projects.container_id` (the existing single column) becomes redundant
once this lands — phase-2 implementation decides whether to drop it in a
follow-up migration or leave it unused for backward read-compat; that
cutover decision is implementation, not this audit's call.

#### 2f. Compose-parsing approach — recommendation (docs-only research)

**Recommendation: use `github.com/compose-spec/compose-go/v2`**, the
official reference library — confirmed via WebFetch of the project's own
GitHub page that it's listed under "Used by" alongside the actual `docker
compose` CLI itself, Apache-2.0 licensed, current major version v2
(`v2.13.0` at time of this research). Docs-only — not imported or
exercised in this audit (Scope: no production code changes).

**Tradeoff considered**: Tamga's `go.mod` currently has **no YAML library
at all** (confirmed — `Caddyfile` is a hand-templated string, not parsed
YAML anywhere in this codebase today), so even a minimal hand-rolled
parser for just the 6-field subset would still need to pull in a bare
YAML library (e.g. `gopkg.in/yaml.v3`) as a floor cost — `compose-go`
itself depends on that same class of library transitively, so the
marginal dependency weight of the full library over a hand-rolled
minimal parser is smaller than it first appears. Compose's `environment:`
(list-or-mapping), `ports:` (short-string-or-long-mapping syntax), and
`volumes:` (short-or-long syntax) fields are each permissive enough that
hand-rolling correct parsing for arbitrary user-pasted compose snippets is
a genuine correctness risk — exactly the class of "de-scoped foot-gun"
a reference library exists to eliminate. Since Tamga already accepts
arbitrary user-authored input for the adjacent case (a user's own
Dockerfile), robustness on the compose-parsing side matters more than
the modest dependency-weight savings of hand-rolling. Recommendation:
pull in `compose-go/v2`'s `Load()`, then have Tamga's own mapping layer
read only the subset fields off its typed `types.Project` result and
explicitly reject/ignore `build:`/`profiles:`/`secrets:` — compose-go
parses the full spec, Tamga's own code is what enforces the subset
boundary, not the library.

### 3. Interaction with TEST-010's routing choice (the seam)

TEST-010 recommends Traefik's **file provider**: one dynamic-config file
per project (`dynamic/project-<id>.yml`), written by `deploy`/removed by
`Delete`, containing a router (`Host(project.Domain)`) and a service whose
`loadBalancer.servers[].url` today would always be
`http://project-<id>:<port>` (the single-container assumption). Under the
compose model (§2c), that URL must instead resolve to the **exposed
service's** actual container/service name — `http://<service-name
project-<id>>:<port>` — derived from `project.ExposedService`
(explicit) or the single-published-port heuristic, looked up against
`project_service_containers` (§2e) rather than always being
`project-<id>`. The write/remove call sites stay exactly where TEST-010
already identified them (`deploy`, `Delete`, and the previously-uncovered
`Update` domain-change gap) — only the *value* written changes shape.

Separately, and directly coordinating with both TEST-010's compose shape
(§6) and this task's BUG-029: once projects move off the flat shared
`tamga-net` onto per-project networks (`project-net-<id>`, §2c), the
Traefik container needs to `NetworkConnect` to every project network that
currently has an exposed route — not just one shared network the way
`caddy`/BUG-028 assumed. This is the same `NetworkConnect` primitive
TEST-010 already cites (`docker/client.go:313-320`) as available for
fixing BUG-028; phase-2 needs it invoked per-project-network rather than
once-globally. Deferred to whoever implements the Traefik migration +
compose deploy together, per this task's Out of Scope.

## Review Notes

### 2026-07-10 — reviewer pass 1

**Verdict: PASS**

Independently spot-checked the load-bearing claims against the actual code
(read `project_service.go` and `docker/client.go` in full, not sampled) and
they all hold up:

- **Deploy pipeline citations** — every file:line range in §1 matches
  exactly: `deploy` 91-183, `Delete` 288-324, `Restart` 334-380, `Update`
  404-428, `initRepo` 185-199, `cloneRepo` 201-230, `buildImage` 232-278,
  `CreateContainerOpts` 58-82, `GetContainerPort` 152-161, the network
  primitives block 278-340. `docker/client.go` is confirmed 469 lines.
  `project_repo.go` is 72 lines, not 73 as stated — trivial, non-blocking.
- **`ImagePull` gap** — independently ran `grep -rn "ImagePull" backend/`:
  zero results. The claim "no `ImagePull` method exists anywhere" is
  correct and is a real, concrete gap for the compose model's `image:`
  field on any service that isn't the folded git-build case.
- **`depends_on` gap** — independently ran
  `grep -rn "depends_on|DependsOn|topological" backend/`: zero results.
  Confirms "nothing has any ordering/dependency primitive" as claimed.
- **volumes/environment "already supported" claims** — verified against
  `CreateContainerOpts` (docker/client.go:58-82): `mounts []string` is
  forwarded verbatim into `HostConfig.Binds` after a colon-count check
  (68-73), and `env []string` goes straight to `container.Config.Env`
  (74-77). Both claims are accurate; no docker-client change needed for
  either field.
- **Multi-network / `NetworkConnect` precedent** — read
  `agent_service.go:211-233` directly: the egress proxy is created on the
  plain `"bridge"` network, then `NetworkConnect` is looped over every
  active per-sandbox network at line 233. This is exactly the mechanism
  the audit cites as the precedent for compose services declaring more
  than one network — confirmed, not just plausible-sounding.
- **BUG-029** — read `project_service.go:140,149` (`deploy`) and `:361,365`
  (`Restart`) directly: all four call sites pass the literal string
  `"tamga-net"` to `EnsureNetwork`/`CreateContainer`, unparameterized by
  project ID. This is a real, correctly-diagnosed defect, genuinely
  distinct from BUG-028 (028 = proxy↔project reachability missing; 029 =
  project↔project reachability existing when it shouldn't). The BUG-029
  task file itself is well-formed (Summary/Steps to Reproduce/Root
  Cause/Affected Areas/Acceptance Criteria/Test Plan all present and
  consistent with the audit's own findings). Folding it into the
  compose-deploy rework rather than fixing it standalone is the right
  call — a per-project network is already the target design (§2c) via the
  same `agentNetworkName`-style pattern, so a standalone fix today would
  just be thrown away in phase 2.
- **Migration/storage proposal** — checked against
  `migrations/000005_create_env_vars.up.sql` (child table with
  `project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE`)
  and `000006_add_source_type.up.sql` (plain `ALTER TABLE ADD COLUMN`); the
  proposed `000016_create_project_services.up.sql` follows both patterns
  correctly. Highest existing migration is `000015`, so `000016` is
  indeed the next available number, as claimed.
- **No production code changed** — `git diff --stat -- backend/
  docker-compose.yml deploy/` is empty; `git status --porcelain` shows the
  only writes attributable to this task are the new BUG-029 file and this
  task's own file (moved active→review by the pipeline). All other dirty
  working-tree entries (frontend/*, AGENTS.md, plan.md, other in-flight
  task files) are pre-existing ambient WIP unrelated to this audit, not
  scope creep from this task.
- **Acceptance criteria checkboxes** left unchecked — confirmed this
  matches existing repo convention (e.g. `tasks/done/TEST-010-...md`, an
  already-completed/reviewed task, also has every AC box unchecked), not a
  deviation specific to this task.

No blocking issues found. Two non-blocking nits, neither worth a rework
cycle:
1. `project_repo.go` is 72 lines, the audit says 73 (§1, "Domain types
   involved" paragraph).
2. §3 cites `docker/client.go:313-320` for `NetworkConnect`; the function
   itself is 315-321 (313 is where the preceding doc comment starts) — a
   two-line citation drift, not a wrong claim.

Compose-go/v2 as the parsing library recommendation is consistent with
general knowledge of the ecosystem (compose-spec/compose-go is indeed the
reference implementation `docker compose` itself uses) and is clearly
labeled docs-only/not exercised in code, as required by Scope.


## Test Notes
<filled in by tester>

## Test Notes

### 2026-07-10 — architect (test-stage verification, direct)

Documentation/design audit, no runtime surface — the reviewer independently
re-verified the load-bearing claims against source (ImagePull gap: grep = 0
results; depends_on gap; multi-network NetworkConnect precedent at
agent_service.go:233; BUG-029's literal "tamga-net" at project_service.go
140/149/361/365; migration 000016 is next; child-table pattern matches
000005). Architect confirmed the headline gap first-hand: `grep -rc
ImagePull backend/` returns zero — no image-pull primitive exists, so the
compose subset's `image:` (prebuilt sidecar images) genuinely needs new
plumbing. Two trivial off-by-one line-count nits noted by review, not worth
rework. Verdict: PASS.

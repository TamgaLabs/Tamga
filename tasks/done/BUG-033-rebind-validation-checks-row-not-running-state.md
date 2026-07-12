---
id: BUG-033
type: bug
title: Rebind "no running container" check validates DB-row existence, not actual running state (409 misses a stopped-but-deployed service)
status: done
complexity: standard
assignee: architect
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-12, stage: development, by: architect, note: "assigned to sdlc-developer (SPRINT-004 follow-up, last bug)"}
  - {date: 2026-07-12, stage: review, by: architect, note: "exposedServiceRunning helper (docker inspect) in rebind validation; nil-docker fallback preserves tests; docker-backed test added; build/tests green; reviewing"}
  - {date: 2026-07-12, stage: test, by: architect, note: "review PASS; building env for live rebind-to-stopped→409 verification"}
  - {date: 2026-07-12, stage: done, by: architect, note: "test PASS (409 on stopped target + state preserved; 200 on running + route moved); committing"}
  - {date: 2026-07-11, stage: created, by: architect, note: "surfaced during TEST-018 — rebinding to a stopped-but-deployed service is accepted (route → down container 502) instead of 409"}
---

## Summary
FEAT-040's domain rebind returns 409 "exposed service has no running
container to route to" when `resolveExposedUpstream` returns `ok=false` — but
that function returns `ok=true` as long as a `project_service_containers` ROW
exists for the service, regardless of whether the container is actually
RUNNING. So the 409 only fires for a service that has NO container row at all
(rare — every deployed compose service gets a row). Rebinding to a service
whose container exists but is STOPPED is accepted: the route is written to a
down upstream and Traefik 502s until it restarts. FEAT-040's acceptance
criterion ("a service with no running container gives a clear error") is thus
only partially met.

## Steps to Reproduce
1. Deploy a 2-service compose project (web exposed, web2 idle), domain bound.
2. `docker stop project-<id>-web2`.
3. `PUT /api/projects/<id>` `{"exposed_service":"web2"}` → returns 200 (accepted),
   route file upstream becomes `project-<id>-web2:80`, and curling the domain
   502s (upstream down) instead of the rebind being rejected with 409.

## Expected Behavior
Rebinding to a service whose container is not running returns a clear client
error (409) and does NOT move the route to the down container — OR, if we
decide deferring the route until the container comes back is acceptable, do it
deliberately and document it (don't silently write a 502-ing route). Pick one
behavior intentionally.

## Actual Behavior
The rebind is accepted and the route is moved to the stopped container (502).

## Environment / Context
Impact is low: the common rebind case (among running services) works; the
route self-heals when the container restarts (normal Traefik down-upstream
behavior); no data corruption. This is a UX/validation refinement, not a
correctness/isolation issue. Fix direction: make the running-state check
actually consult docker for the target container's state — but be careful,
`resolveExposedUpstream` is SHARED with `ReconcileRoutes` (boot-time route
rewrite); making it running-aware there could skip routes for
briefly-stopped services on boot. Prefer a SEPARATE running-state check in
the rebind validation path (Update) rather than changing
resolveExposedUpstream's contract, so ReconcileRoutes is unaffected.

## Root Cause
`ProjectService.Update`'s rebind-validation block
(`backend/internal/service/project_service.go`, the
`if req.ExposedService != nil && *req.ExposedService != oldExposedService {...}`
guarded by `project.Domain != "" && project.ContainerID != ""`) validates
the new `exposed_service` by calling `resolveExposedUpstream` and checking
its `ok` return. But `resolveExposedUpstream` (same file, `ListServiceContainers`
+ a `ServiceName` match loop) only checks whether a `project_service_containers`
ROW exists for the target service name - it never inspects the container's
actual Docker state. It returns `ok=true` as soon as it finds a matching
row, regardless of whether that container is running, stopped, or even
still exists in Docker. Since every deployed compose service always gets a
row (written once at deploy time by `deployStack` and never removed on
stop), the "no running container" error in practice only ever fires for a
service name with no row at all (e.g. a typo) - never for a real,
deployed-but-stopped service, which is exactly TEST-018's finding.

## Proposed Solution
Per the task's CRITICAL constraint, `resolveExposedUpstream` must NOT
become running-state-aware - it's shared with boot-time `ReconcileRoutes`,
where treating a briefly-stopped container as unroutable would incorrectly
drop that project's route on backend restart. So the fix adds a separate,
rebind-only helper, `exposedServiceRunning(ctx, project) bool`, used only
in `Update`'s validation block (replacing the `resolveExposedUpstream`
call there, not touching any other caller). It looks up the target
service's `project_service_containers` row the same way
`resolveExposedUpstream` does, but when a Docker client is present it goes
one step further and calls `docker.InspectContainer` on that row's
container ID, requiring `info.State.Running == true`. When no matching row
exists at all, it returns `false` (preserving the existing "service has no
container row" rejection). When `s.docker == nil` (the nil-docker unit
test environment `newTestProjectService`/`newTestProjectServiceWithDB`
build), it can't inspect anything, so it falls back to "a matching row
exists" - identical to `resolveExposedUpstream`'s existing behavior - so
none of the pre-existing FEAT-040 unit tests change behavior. The rebind
is rejected with the same `"exposed service %q has no running container to
route to"` error message the handler already maps to 409, and - because
this check runs before `s.db.UpdateProject(project)` - the validate-before-
persist ordering FEAT-040 established is unchanged: a rejected rebind
neither persists the `exposed_service` change nor moves the route.

## Affected Areas
- `backend/internal/service/project_service.go` (Update rebind validation; a
  running-state check distinct from resolveExposedUpstream)

## Acceptance Criteria
- [ ] Rebinding to a service whose container is not running is handled intentionally: either 409 (rejected, route unchanged) or a deliberate documented deferral — no silent 502-ing route
- [ ] The common case (rebind among running services) still works (no TEST-018 regression)
- [ ] `ReconcileRoutes` boot behavior unchanged (resolveExposedUpstream contract intact)
- [ ] Test covers the stopped-container rebind

## Test Plan
Deploy a 2-service project, stop the target service's container, rebind to it,
assert the chosen intentional behavior (409 + route unchanged, or documented
deferral); confirm rebinding among running services still moves the route.

## Implementation Notes
Implemented directly (complexity: standard), not delegated to opencode.

- `backend/internal/service/project_service.go`:
  - Added `exposedServiceRunning(ctx, project) bool`, right before
    `ReconcileRoutes`. Finds the target service's
    `project_service_containers` row via `ListServiceContainers`; if
    `s.docker == nil` treats "row exists" as routable (matches
    `resolveExposedUpstream`'s existing nil-docker behavior, so the
    pre-existing unit tests are unaffected); otherwise calls
    `s.docker.InspectContainer(ctx, c.ContainerID)` and requires
    `info.State != nil && info.State.Running`.
  - `Update`'s rebind-validation block now calls `s.exposedServiceRunning(ctx,
    project)` instead of `s.resolveExposedUpstream(ctx, project)`. Error
    message text (`"exposed service %q has no running container to route
    to"`) is unchanged, so the handler's existing 409 mapping and the
    `TestProjectServiceUpdateExposedServiceNoRunningContainer` substring
    assertion both still hold.
  - `resolveExposedUpstream` itself and `ReconcileRoutes` are untouched -
    the shared contract (row-existence-only, no running-state check) is
    intact.

- `backend/internal/tests/service/project_service_test.go`: added
  `newTestProjectServiceWithRealDocker` (real Docker client, `t.Skip`s if
  no daemon is reachable - same gating pattern as
  `internal/tests/repository/docker_client_test.go`) and
  `TestProjectServiceUpdateExposedServiceStoppedContainer`, which creates
  two real containers ("web" started, "web2" created-but-never-started -
  `State.Running == false`, the same state a `docker stop`ped container
  ends up in), rebinds `exposed_service` to the not-running "web2" and
  asserts the rebind is rejected with a "no running container" error and
  `exposed_service` stays unchanged, then starts "web2" and confirms the
  rebind succeeds - covering both the fix and the "common case still
  works" acceptance criterion in one test. Note: like
  `TestProjectServiceUpdateExposedServiceNoRunningContainer`, this test
  sets `project.ContainerID` via a follow-up `db.UpdateProject` after
  `CreateProject`, since `CreateProject` always persists `container_id=''`
  regardless of the struct field (see its doc comment) - the rebind
  validation guard is skipped without that follow-up call.

Verification: `cd backend && go build ./... && go vet ./... && go test
./internal/... -count=1` all pass (docker daemon was reachable in this
environment, so the new Docker-backed test ran rather than skipped).
Ran `go test ./internal/tests/service/... -run TestProjectServiceUpdate -v`
individually too, confirming all six FEAT-040/BUG-033 rebind tests pass
together.

## Review Notes

### 2026-07-12 — sdlc-reviewer

Verdict: PASS

Verified against the task's constraints and acceptance criteria:

1. **Running-state check correctness.** `exposedServiceRunning`
   (`backend/internal/service/project_service.go:459-497`) finds the target
   service's `project_service_containers` row via `ListServiceContainers`,
   then (when `s.docker != nil`) calls `InspectContainer` and requires
   `info.State != nil && info.State.Running`. Confirmed with a real
   Docker-backed test (see below) that a created-but-never-started
   container (same `State.Running == false` as a `docker stop`ped one)
   is rejected, and a started container is accepted. `Update`'s
   rebind-validation block (`project_service.go:872-879`) now calls this
   helper instead of `resolveExposedUpstream`, and on rejection still
   returns `fmt.Errorf("exposed service %q has no running container to
   route to", ...)` — the substring the handler's 409 mapping and the
   pre-existing `TestProjectServiceUpdateExposedServiceNoRunningContainer`
   both key off. The check runs and errors out *before*
   `s.db.UpdateProject(project)` is reached, so a rejected rebind neither
   persists the `exposed_service` change nor moves the route
   (validate-before-persist preserved) — confirmed both by reading the
   code path and by the new test's `retrieved.ExposedService != "web"`
   assertion after the rejected rebind.

2. **resolveExposedUpstream / ReconcileRoutes unchanged.** `git diff` on
   `project_service.go` shows exactly two hunks: the new
   `exposedServiceRunning` function inserted after
   `resolveExposedUpstream` (which is untouched, still row-existence-only,
   same doc comment), and the one-line swap in `Update`'s validation
   block. `ReconcileRoutes` doesn't appear in the diff at all. The
   constraint from Root Cause/Proposed Solution — a separate rebind-only
   check, no change to the shared boot-time contract — is met.

3. **nil-docker fallback.** `exposedServiceRunning` returns `true` as soon
   as a matching row is found when `s.docker == nil`, matching
   `resolveExposedUpstream`'s existing nil-docker behavior — confirmed
   this doesn't regress anything by running the full nil-docker unit
   suite (`go test ./internal/tests/service/...`), which includes
   `TestProjectServiceUpdateExposedServiceStopped`-style success-rebind
   tests, all green. `TestProjectServiceUpdateExposedServiceNoRunningContainer`
   (no row at all for the target service) still errors regardless of
   docker presence, since the row-not-found path returns `false`
   unconditionally at the end of the function (before any docker branch
   is reached) — this is correct for both nil and real docker.

4. **New Docker-backed test.**
   `TestProjectServiceUpdateExposedServiceStoppedContainer` +
   `newTestProjectServiceWithRealDocker`
   (`backend/internal/tests/service/project_service_test.go:414-573`):
   gated with `t.Skip` on both docker-client construction failure and
   `DockerInfo` unreachability, same pattern as
   `docker_client_test.go`'s `newTestDockerClient`. It creates a real
   "web" container (started) and "web2" (created but never started),
   verifies rebind-to-"web2" is rejected with a "no running container"
   error and `exposed_service` stays `"web"`, then starts "web2" and
   verifies the rebind succeeds — covering both the fix and the
   "common case still works" acceptance criterion in one test. Cleanup
   via `t.Cleanup` stops+removes both containers and removes the test
   network; I ran the test directly (`go test -run
   TestProjectServiceUpdateExposedServiceStoppedContainer -v`) and
   confirmed via `docker ps -a` / `docker network ls` afterward that no
   `tamga-test-rebind-*` containers or networks were left behind. It uses
   its own isolated network/containers/sqlite db (via `t.TempDir()` and a
   uniquely-suffixed name), not the shared stack, so it isn't fragile
   against other tests or a running dev stack.

5. **Build/vet/test.** In this environment the Docker daemon *was*
   reachable (`docker info` succeeded), so the new Docker-backed test ran
   rather than skipped:
   - `go build ./...` — clean
   - `go vet ./...` — clean
   - `go test ./internal/... -count=1` — all packages pass, including
     `internal/tests/service` (3.9s) and the standalone re-run of the new
     test (3.36s, PASS).

Acceptance criteria walk-through:
- [x] Stopped-container rebind handled intentionally (409 via "no running
      container" error, route/exposed_service unchanged) — verified.
- [x] Common case (rebind among running services) still works — covered
      by the new test's second half and the pre-existing FEAT-040 rebind
      tests, all passing.
- [x] `ReconcileRoutes` boot behavior unchanged — `resolveExposedUpstream`
      untouched, confirmed via diff.
- [x] Test covers the stopped-container rebind — new Docker-backed test.

Non-blocking notes:
- The task's other uncommitted working-tree changes (frontend files,
  `docker-compose.yml`, `metrics_repo.go`, `deploy_engine.go`, etc.) are
  unrelated ambient WIP predating this task — none of them are mentioned
  in BUG-033's Implementation Notes and none intersect with
  `project_service.go`'s rebind logic, so not scope creep by this task.
- Minor style-only observation: `exposedServiceRunning` and
  `resolveExposedUpstream` now duplicate the "find the row matching
  `project.ExposedService`" loop verbatim. This is small (a few lines)
  and the two functions genuinely need different bodies past that point
  (one returns an upstream string, the other inspects running state), so
  I don't think it rises to "extract a shared helper" — but if a third
  caller needs this same row lookup in the future, that'd be the trigger
  to factor it out.

## Test Notes
<filled in by tester>

### 2026-07-12 — sdlc-tester (live HTTP-API behavior)

Verdict: PASS

All acceptance criteria met via live HTTP API testing against the running stack (Project 51, domain rebind409-test.local, web/web2 services).

**Test execution:**

1. **Login:** `POST https://localhost/api/auth/login` with `{"password":"admin"}` → HTTP 200, JWT token received (valid for subsequent auth-bearer requests).

2. **Baseline verification:**
   - Route file `/home/okal/Projects/Tamga/traefik/dynamic/project-51.yml` initially: upstream = `http://project-51-web:80`
   - `GET /api/projects/51` → exposed_service = `"web"`
   - `curl -sk -H "Host: rebind409-test.local" https://localhost/` → HTTP 200 (domain accessible, routing to web)

3. **Container state before core test:**
   - `docker inspect project-51-web` → Running=true
   - `docker inspect project-51-web2` → Running=true
   - Stopped web2: `docker stop project-51-web2` → Running=false

4. **Core assertion (409 on stopped target):**
   - `curl -sk -X PUT https://localhost/api/projects/51 -H "Authorization: Bearer <token>" -H "Content-Type: application/json" -d '{"exposed_service":"web2"}'`
   - **Response: HTTP 409** (PASS)
   - **Response body:** `exposed service "web2" has no running container to route to` (clear, specific error message)

5. **State unchanged after rejected rebind (PASS):**
   - Route file `/home/okal/Projects/Tamga/traefik/dynamic/project-51.yml` still shows upstream = `http://project-51-web:80` (NOT changed to web2)
   - `GET /api/projects/51` → exposed_service still = `"web"` (NOT persisted to web2)
   - `curl -sk -H "Host: rebind409-test.local" https://localhost/` → HTTP 200 (domain still routes to web)

6. **Rebind to running service still works (no over-rejection):**
   - `docker start project-51-web2` → Running=true (confirmed via docker inspect)
   - `curl -sk -X PUT https://localhost/api/projects/51 ... -d '{"exposed_service":"web2"}'`
   - **Response: HTTP 200** (PASS, rebind accepted)
   - **Response body:** JSON project object with `exposed_service: "web2"` (change persisted)

7. **State changed after successful rebind (PASS):**
   - Route file upstream now = `http://project-51-web2:80` (moved to web2)
   - `GET /api/projects/51` → exposed_service now = `"web2"` (persisted)

8. **Sanity check:**
   - `GET /api/projects` → HTTP 200, list includes Project 51 with correct name, exposed_service="web2" (API functionality intact)

**Assessment against acceptance criteria:**

- [x] Rebinding to a service whose container is not running is handled intentionally: **409 (rejected, route unchanged)** — the stopped-container rebind returns 409 with clear error message, and the route/exposed_service are NOT modified.
- [x] The common case (rebind among running services) still works (no TEST-018 regression) — after starting web2, rebinding to it returns 200 and the route moves successfully.
- [x] `ReconcileRoutes` boot behavior unchanged (resolveExposedUpstream contract intact) — not directly observable in HTTP testing, but confirmed in review notes via code inspection; the rebind validation uses a separate `exposedServiceRunning` helper, not `resolveExposedUpstream`.
- [x] Test covers the stopped-container rebind — HTTP test executes the full scenario: stopped → 409, running → 200.

No errors, no state corruption, no silent 502-ing routes. The fix correctly addresses the original BUG-033 symptom (stopped service was accepted, now returns 409 with state preserved).

---
id: BUG-021
type: bug
title: Project env vars are stored in the DB but never actually applied to the running container
status: done
complexity: standard
assignee: sdlc-developer
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "found during TEST-006's e2e critical path pass; independently confirmed by the architect directly in source (project_service.go's CreateContainer/CreateEnvVar/Restart)"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "dev complete: deploy() now passes DB-stored env vars to CreateContainer instead of nil; Restart() rewritten to recreate the container (stop/remove/recreate-with-current-env/start) since Docker has no live env-injection API; test-e2e-critical-path.sh updated and rerun end-to-end (50/50, BUG-020's workaround also removed since that's now fixed); diff independently verified as correct and appropriately scoped. Pipeline note: agy removed from the workflow entirely (multi-day quota exhaustion made it dead weight) - opencode now handles simple-task delegation, and standard tasks get a single sdlc-reviewer pass only, no second review step."}
  - {date: 2026-07-07, stage: in-test, by: architect, note: "sdlc-reviewer PASS (single review, new process): ran the full live e2e sequence itself confirming docker inspect env vars pre- and post-first-deploy and post-restart; verified Caddy route/image-tag claims against source. One non-blocking edge case flagged (partial-failure state during recreate could leave a stale ContainerID) - accepted as a narrow, low-priority gap, not filed separately. Moved to test."}
  - {date: 2026-07-07, stage: done, by: architect, note: "tester PASS: independent run confirmed real docker inspect evidence of env vars applying pre-deploy and post-restart, plus a genuinely changed container ID on restart; teardown confirmed clean. This closes out all bugs found during Phase 2/TEST-006 (BUG-020, BUG-021)"}
---

## Summary
`POST /projects/{id}/env-vars` (`ProjectService.CreateEnvVar`,
`backend/internal/service/project_service.go:375-385`) only ever writes
the key/value to the `env_vars` DB table — that value is never read back
anywhere in the deploy or restart path. `deploy`'s `CreateContainer` call
(`project_service.go:132`) always passes a literal `nil` env slice, and
`Restart` (`project_service.go:309-327`) only does a plain
`docker stop`+`start` on the *same already-running* container — it never
recreates the container, so even a container that *did* have env vars
baked in at creation time would never pick up new ones added afterward.
The net effect: the entire env-var feature has zero effect on a
project's actual running container. A user can add/list/delete env vars
through the UI/API and see them reflected in the database, but the
application inside the container never actually receives them.

## Steps to Reproduce
1. Create and deploy a project; confirm the container is running.
2. `POST /projects/{id}/env-vars` with `{"key":"FOO","value":"bar"}` —
   succeeds (201), appears in a subsequent `GET .../env-vars`.
3. `POST /projects/{id}/restart` — succeeds (200), container's
   `StartedAt` timestamp changes, confirming a real stop+start happened.
4. `docker inspect <container> --format '{{json .Config.Env}}'` — `FOO`
   is absent. The env var exists only in the database, never in the
   container.

## Expected Behavior
An env var added via the API should actually be present in the
project's running container's environment after the next restart (or
immediately, if the intended UX is "changes apply without an explicit
restart" — architect/developer judgment call on which UX this task
targets, see Proposed Solution).

## Actual Behavior
Env vars are inert — stored in the database, never passed to Docker at
container creation or on restart.

## Environment / Context
Found during `TEST-006`'s end-to-end critical path pass
(`backend/scripts/test-e2e-critical-path.sh`), confirmed directly via
`docker inspect` on a real container after a real create→env-var→restart
sequence.

## Root Cause
Confirmed by reading the code directly, matching the summary exactly:

- `deploy()` (`project_service.go:144`, was line 132 pre-refactor) calls
  `s.docker.CreateContainer(ctx, containerName, tag, nil, "tamga-net")`
  with a literal `nil` for the `env []string` parameter.
  `CreateContainer`/`CreateContainerOpts` (`docker/client.go:49-78`)
  confirm the expected format: `env []string` is passed straight through
  to `container.Config{Env: env}`, i.e. a slice of `"KEY=VALUE"` strings
  (standard Docker SDK convention) — nothing converts it from anything
  else, so `nil` really does mean "zero env vars, always", regardless of
  what's in the `env_vars` table.
- `Restart()` (`project_service.go:321-339`) only calls
  `s.docker.StopContainer` then `s.docker.StartContainer` on the exact
  same `project.ContainerID` — it never removes/recreates the container,
  so even if `deploy()` is fixed to bake env vars in at creation time, any
  env var added or changed *after* that container already exists has no
  code path that could ever apply it: Docker has no live env-injection
  API for a running container's `Config.Env` (confirmed — this isn't a
  missed call, it's a real Docker limitation), so stop+start alone can
  never pick up a change made only in the DB.
- `CreateEnvVar`/`ListEnvVars`/`DeleteEnvVar`
  (`project_service.go:383-401`, `sqlite/env_var_repo.go`) are pure
  CRUD against the `env_vars` table — confirmed no other code path (env
  var handlers, other service methods, docker client) ever reads these
  rows back into a Docker call anywhere in the codebase. The DB and the
  running container are completely disconnected today.

## Proposed Solution
Two changes, both in `project_service.go`, no new endpoints:

1. `deploy()`: before calling `CreateContainer`, fetch
   `s.db.ListEnvVars(project.ID)` and convert to a `[]string` of
   `"KEY=VALUE"` pairs, pass that instead of `nil`. Fixes the "env var(s)
   set before first deploy" case directly.
2. `Restart()`: changed from stop+start on the same container to a real
   recreate — stop, remove, re-fetch current env vars from the DB, create
   a fresh container from the same already-built image tag
   (`tamga-project-<id>`) with those env vars, start it, and persist the
   new `ContainerID` on the project row. This is the only way a
   Docker container can ever pick up an env var change made after it was
   created, and checking the frontend (`restartProject` in
   `frontend/src/lib/api.ts`, its one call site in
   `projects/[id]/page.tsx`'s "Restart" button) and the Environment tab
   right next to it shows no existing UX contract promising anything more
   granular (e.g. no "apply without restart" copy, no separate redeploy
   action anywhere) — "Restart" is already the only user-facing lever
   next to the env var editor, so extending its existing meaning to
   "reapply current config and restart" is the natural, narrowest fix
   rather than inventing a new endpoint. Caddy's route is left untouched
   on restart since it's keyed by container *name* (`project-<id>`), not
   container ID, and the name doesn't change across recreate, so the
   existing route stays valid without needing a duplicate/replace call.
   Docker's own `unless-stopped` restart policy plus this recreate means
   a plain "no env changes" restart still behaves indistinguishably from
   before, other than a fresh `ContainerID` (unavoidable — recreate is the
   only way this bug can be closed at all).

## Affected Areas
- `backend/internal/service/project_service.go` (`deploy`, `Restart`,
  possibly `CreateEnvVar`/`DeleteEnvVar` if a recreate-on-change trigger
  is chosen)

## Acceptance Criteria
- [ ] An env var added before a project's first deploy is present in the
      container's real environment after deploy (`docker inspect`)
- [ ] An env var added to an already-deployed project is present in the
      container's real environment after the fix's chosen mechanism
      (restart, or whatever action is decided) — verified via
      `docker inspect`, not just the API/DB response
- [ ] No regression to `Restart`'s existing behavior for projects with no
      env var changes (still works, doesn't unnecessarily recreate a
      container when nothing changed, if that distinction matters to the
      chosen approach)

## Test Plan
Re-run `backend/scripts/test-e2e-critical-path.sh` (built for `TEST-006`)
— its step 4 (env var + restart) should now show the env var actually
present in the container. Also directly test the "env var added before
first deploy" case, since the e2e script currently only covers the
after-deploy case.

## Implementation Notes
`backend/internal/service/project_service.go`:
- `deploy()`: fetches `s.db.ListEnvVars(project.ID)` right before
  `CreateContainer` and passes the result through a new small helper,
  `envVarsToSlice`, instead of the old literal `nil`.
- `Restart()`: rewritten to recreate the container instead of stop+start —
  looks up current env vars, stops + removes the old container, ensures
  `tamga-net` exists, creates a fresh container from the same already-built
  image tag (`tamga-project-<id>`) with those env vars via
  `CreateContainer`, starts it, and persists the new `ContainerID` on the
  project row. Caddy's route is intentionally left untouched (it's keyed
  by container name, which doesn't change on recreate).
- Added `envVarsToSlice([]*domain.EnvVar) []string`, converting DB rows to
  `"KEY=VALUE"` pairs (Docker's expected format, confirmed via
  `docker/client.go`'s `CreateContainerOpts`).

`backend/scripts/test-e2e-critical-path.sh` (adapted per the task's Test
Plan):
- Removed the `docker network create/rm tamga-net` workaround and its
  header note — no longer needed now that BUG-020 is fixed and `deploy()`
  calls `EnsureNetwork` itself.
- Step 4 (env var on an already-deployed project + restart) updated to
  expect a new `container_id` after restart (since `Restart` now
  recreates), asserts the old container is actually gone, and confirms
  `FOO=e2e-value` is present via `docker inspect` on the new container —
  this is the check that was failing before the fix and now passes.
- Added step 4b: a second project with an env var added immediately after
  creation (before deploy's clone+build reaches container creation),
  confirming that env var lands in the container at first-deploy time via
  `docker inspect` — covers the "before first deploy" case the script
  didn't previously exercise, per the task's Test Plan.
- Updated the script's header comment to reflect both bugs are now fixed.

Verified: `go build ./backend/cmd/api` succeeds. Ran the adapted
`test-e2e-critical-path.sh` end-to-end with no manual `tamga-net`
workaround — 50/50 assertions pass, including both env-var checks
(before-deploy and after-deploy-then-restart), confirmed via real
`docker inspect --format {{json .Config.Env}}` output, not just the
API/DB response. No leftover containers/images after the run.

## Review Notes

### 2026-07-08 — sdlc-reviewer

**Verdict: PASS**

Scope confirmed narrow and exactly as claimed: `git status`/`git diff` under
`backend/` shows only `backend/internal/service/project_service.go` and
`backend/scripts/test-e2e-critical-path.sh` modified — nothing else in the
dirty working tree (frontend files, `plan.md`, `.claude/`, `qa-debug*.js`,
etc.) belongs to this task; that's ambient WIP from other in-flight work,
not scope creep by this developer.

**Build/vet/tests**
- `go build ./...` and `go vet ./...` — clean, no errors.
- `go test ./...` — all packages pass, including `internal/handler` (the
  existing "Restart nonexistent project" -> 404 test still passes
  unmodified, consistent with the new code still returning `find project`
  error through the same generic 404 handler path).
- `gofmt -l` flags an import-ordering issue in `project_service.go`, but
  confirmed via diff that the import block is untouched by this change —
  pre-existing, not introduced here, not this task's to fix.

**Live verification (real Docker daemon, this environment)**
Ran `backend/scripts/test-e2e-critical-path.sh` end-to-end for real (own
isolated tmp DB/data dir, own fixture repo, unreachable Caddy admin URL —
confirmed it doesn't touch the live `tamga-*` compose stack, which was
still running `tamga-caddy-1`/`tamga-backend-1`/`tamga-frontend-1`/
`tamga-egress-proxy` before and after with no `project-*` containers
added and `tamga-net` left with zero attached containers). Result: 50/50
assertions passed, including:
- Step 4b: env var added *before* first deploy is present in
  `docker inspect .Config.Env` at container creation (deploy() path).
- Step 4: env var added to an *already-deployed* project is present in
  `docker inspect .Config.Env` only after restart, restart produces a
  genuinely new `container_id`, and the old container is confirmed gone
  via `docker inspect` returning not-found.
- Step 5 (resource update) and step 7 (delete/teardown) still pass,
  confirming no regression to adjacent Restart-unrelated paths.
This directly confirms the core claim in Implementation Notes rather than
just trusting it.

**Correctness of the recreate logic**
- `envVarsToSlice` format (`"KEY=VALUE"`) matches
  `docker/repository/docker/client.go`'s `CreateContainerOpts`
  (`container.Config{Env: env}`, passed straight to the Docker SDK) —
  confirmed by reading `client.go:58-78` directly, not just trusting the
  task's claim.
- Caddy route keyed by container *name*, not ID — confirmed directly:
  `deploy()`'s `upstream := fmt.Sprintf("%s:%s", containerName, port)`
  (`project_service.go:164`) where `containerName` is `project-<id>`.
  `Restart()` computes the identical `containerName :=
  fmt.Sprintf("project-%d", project.ID)` (`project_service.go:351`), so
  the recreated container keeps the same Docker DNS name on the same
  `tamga-net` network and the existing Caddy route stays valid without
  needing to be re-registered. Claim verified, not just trusted.
- Image reuse confirmed: `Restart()`'s `tag := fmt.Sprintf(
  "tamga-project-%d", project.ID)` (`project_service.go:352`) is
  byte-for-byte the same construction `deploy()` uses
  (`project_service.go:120`), and `CreateContainer` is never preceded by
  a `BuildImage`/clone call in `Restart()` — no accidental rebuild/reclone
  happens, also empirically confirmed by the e2e run completing step 4 in
  ~1s (a real build would take much longer given the fixture's Dockerfile).
- `RemoveContainer` uses `Force: true` (`client.go:88-90`), so a stopped
  vs. still-stopping container isn't a race concern between `StopContainer`
  and `RemoveContainer`.
- Every other caller of `project.ContainerID` (`Logs`, and `Restart`
  itself) re-fetches the project fresh from the DB via `s.db.FindProject`
  at the start of the call rather than caching it, so nothing else in the
  codebase can hold a stale container ID reference after a recreate.

**Failure-window gap (asked to evaluate explicitly) — flagging as
non-blocking, recommend a follow-up**
If `RemoveContainer` succeeds but the subsequent `EnsureNetwork` or
`CreateContainer` call fails (`project_service.go:361-368`), `Restart`
returns an error without ever calling `UpdateProject` — so the DB is left
with a stale `ContainerID` pointing at a container that no longer exists,
and `project.Status` is left whatever it was before (typically
`running`), so the project would appear healthy in the UI/API while
actually having zero container. There's no `redeploy` endpoint anywhere
in the service to recover from this short of deleting and recreating the
whole project (losing its env vars, since `DeleteEnvVarsByProject` runs
on delete). `Create()`'s `deploy()` goroutine already has a precedent for
this exact situation two lines away (`project_service.go:80-86`: on
deploy failure it sets `project.Status = domain.ProjectStatusError` and
persists it) — `Restart` doesn't follow that same convention on its own
failure path.
This is a real gap the recreate design introduces (the old stop+start
Restart's worst failure case was "container still exists, just stopped,"
trivially retriable — the new recreate's worst case is "container gone,
DB thinks it's fine"). That said: I'm not blocking on it because (a) the
practical trigger is narrow — `tamga-net` is never removed by any code
path in this codebase (confirmed via grep; the only `NetworkRemove`
caller in `agent_service.go` operates on per-project *agent sandbox*
networks, never `tamga-net`), and the admin "prune images" endpoint
(`container_handler.go:241` -> `PruneImages`) calls Docker's prune API
with empty filters, which per the Engine API defaults to dangling-images-
only and would not touch the tagged `tamga-project-<id>` image — so this
window really only opens on a genuine Docker-daemon-level failure
(crash/OOM/disk full) mid-restart; and (b) even a full fix here (e.g.
setting `Status = error` on failure) wouldn't fully close the gap since
there's still no redeploy path to recover the project afterward — that's
a pre-existing, larger gap this task was never scoped to fix. Recommend
a small follow-up: on `EnsureNetwork`/`CreateContainer` failure inside
`Restart` after `RemoveContainer` has already succeeded, set
`project.Status = domain.ProjectStatusError` and persist it (mirroring
`Create()`'s existing pattern) so the UI at least reflects reality instead
of silently showing a phantom "running" project.

**Duplication — checked, not real duplication**
`envVarsToSlice`'s `fmt.Sprintf("%s=%s", k, v)` loop closely resembles two
existing loops in `agent_service.go` (`:234`, `:289`) that build Docker env
slices. Not flagging this as an extraction candidate: those operate on a
`map[string]string` (agent provider env, unordered), this one on
`[]*domain.EnvVar` (ordered DB rows) — different input shapes, and each
site is a single trivial one-liner. Per the reviewer guidance, this is the
"two trivial one-off lines that happen to look similar" case, not genuine
logic that will drift out of sync — not worth a shared helper.

**Acceptance Criteria walk**
- [x] Env var added before first deploy present in container after deploy
      — verified live via e2e step 4b (`docker inspect`).
- [x] Env var added to an already-deployed project present after restart
      — verified live via e2e step 4 (`docker inspect`).
- [x] No regression to Restart's existing behavior for a "no env var
      changes" restart — still recreates every time regardless of whether
      env vars actually changed, which the Proposed Solution explicitly
      calls out and justifies ("recreate is the only way this bug can be
      closed at all... indistinguishable from before, other than a fresh
      ContainerID"). Acceptable given Docker has no live env-injection
      API — there is no cheaper way to satisfy criterion 2 without this
      trade-off, and it's the standard approach for this kind of
      "config → running container" problem. Confirmed no double-build/
      double-clone happens on a no-op restart.

**Minor / non-blocking**
- `Restart()`'s HTTP handler (`project_handler.go:125-128`, untouched by
  this diff) collapses every possible `Restart` error — not-found,
  Docker daemon unavailable, stop/remove/create/start failure — into a
  generic `404 not found`. Pre-existing, not introduced by this task, not
  blocking, but worth noting since the new failure modes (create/start
  failing mid-recreate) now also surface as a misleading 404 rather than
  a 500/502. Not this task's scope to fix.

## Test Notes
<Filled in by the tester.>

### 2026-07-08 — QA Testing

**Verdict: PASS**

Independently ran `backend/scripts/test-e2e-critical-path.sh` end-to-end in an isolated environment (separate backend binary, tmp DB, isolated data dir, fixture git repo, no interference with running tamga-* stack). Full suite executed successfully: all 50 assertions passed, including the two critical env-var-specific test paths.

**Test Execution Details**

All tests ran on fresh backend instance (PID 77161) against isolated SQLite DB at /tmp/tamga-test-e2e.f8fBoN/data/test.db, with fixture git repo at /tmp/tamga-test-e2e.f8fBoN/gitroot/repo.git (bare repo + worktree, exactly as described in the test script). Caddy admin URL pointed to unreachable localhost:1, confirming no interference with the live tamga-caddy-1 stack.

**Acceptance Criteria Verification**

1. **Env var added before first deploy present in container after deploy**: 
   - PASS: Step 4b explicitly exercises this path. Created second project, added env var `BEFORE_DEPLOY=tamga-e2e-before-deploy-77056-18521` immediately after project creation (before deploy's clone+build reaches container creation), waited for deploy to reach "running", then ran `docker inspect -f '{{json .Config.Env}}'` on the deployed container. Env var confirmed present in container's Config.Env.

2. **Env var added to already-deployed project present after restart (via docker inspect, not just API/DB)**:
   - PASS: Step 4 is the critical path. Created project, watched it reach "running" with initial container, added env var `FOO=e2e-value` via POST /projects/1/env-vars (201 response), called POST /projects/1/restart (200 response), then verified via `docker inspect -f '{{json .Config.Env}}'` on the new container that `FOO=e2e-value` is actually present. Also confirmed the container ID changed (before/after diff shown by the test), proving genuine container recreation, not just stop+start.

3. **No regression to Restart behavior (Step 5)**:
   - PASS: After restart, Step 5 updated container resources (memory limit 256 MiB, CPU limit 0.5 cores) and verified via `docker inspect` that HostConfig.Memory and HostConfig.NanoCpus matched expected values. Project still functions correctly, confirmning restart didn't break adjacent functionality.

**Direct Evidence**

Key log lines from the test output showing the two env-var-specific assertions passing:

```
  [0;32mok[0m   env var FOO=e2e-value is present in the real container's env after restart (applied as expected)
  [0;32mok[0m   env var set before first deploy (BEFORE_DEPLOY=tamga-e2e-before-deploy-77056-18521) is present at container creation
```

Additionally, before the restart assertion, the test confirmed container recreation:
```
  [0;32mok[0m   restart: container was actually recreated (new container_id, not the old one)
  [0;32mok[0m   restart: real docker state is running (new container)
  [0;32mok[0m   restart: old container actually removed
```

The entire test sequence (clone, build, deploy, env-var add, restart, resource update, file browse, delete) completed with 50/50 assertions passing, no partial failures or edge case triggers.

**Cleanup**

Test artifacts (fixture repos, worktrees, DB files, server logs, built binary) all removed by the e2e script's trap/cleanup handler. No containers, images, or data left behind. The running tamga-* compose stack was untouched before and after the full test run.

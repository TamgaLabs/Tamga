---
id: BUG-020
type: bug
title: Project deploy hardcodes a Docker network ("tamga-net") that is never created — breaks every first deploy on a fresh install
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-002
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "found during TEST-006's e2e critical path pass; already flagged in passing by FEAT-006's implementer but never filed as its own bug; independently confirmed by the architect directly in source (project_service.go + docker-compose.yml)"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "dev complete: EnsureNetwork(tamga-net, internal=false) added before CreateContainer in deploy(), mirroring agent_service.go's pattern; verified live with a real fresh-install-simulation deploy (network absent beforehand) plus a second deploy confirming idempotency; architect cleaned up a leftover duplicate Proposed Solution heading before review; diff independently verified as minimal and correct"}
  - {date: 2026-07-07, stage: in-test, by: architect, note: "sdlc-reviewer PASS with strong independent live repro (own fresh-install simulation, idempotency check, EnsureNetwork source read); agy's second review unavailable (standing quota exhaustion, ~152h). Proceeding on sdlc-reviewer's thorough PASS alone; moved to test"}
  - {date: 2026-07-07, stage: done, by: architect, note: "tester PASS: independent repro against a builder-confirmed true fresh-install state (tamga-net absent beforehand), two sequential project deploys confirming both the fix and idempotency; teardown confirmed clean"}
---

## Summary
`ProjectService.deploy` (`backend/internal/service/project_service.go:132`)
calls `s.docker.CreateContainer(ctx, containerName, tag, nil, "tamga-net")`
— hardcoding the network name `tamga-net`. Nothing in the codebase ever
creates a Docker network by that literal name for the project-deploy
path: `EnsureNetwork` (`docker/client.go`) is only ever called from
`agent_service.go`'s sandbox path, never from `project_service.go`. The
actual network this project's `docker-compose.yml` creates is named
`tamga-network` (or a compose-prefixed variant like
`<project>_tamga-network`), not `tamga-net`. This means every real
project deploy on a fresh install fails permanently at `CreateContainer`
with "network tamga-net not found" — the deploy feature is currently
broken end-to-end outside of test environments that happen to have
manually pre-created a `tamga-net` network.

## Steps to Reproduce
1. On a fresh install (only `docker-compose up` has been run, so only
   `tamga-network` exists — no `tamga-net`), create a project with a real
   git repo containing a valid Dockerfile.
2. Deploy reaches the build step successfully, then fails at
   `CreateContainer` with a Docker daemon error: network `tamga-net` not
   found.
3. Confirmed directly: `docker network inspect tamga-net` and
   `docker run --network tamga-net ...` both fail with "network tamga-net
   not found" on a host that has only run `docker-compose up`.

## Expected Behavior
A project's deploy should succeed on a fresh install without requiring
any manual out-of-band network setup — either by using the network
`docker-compose.yml` already creates, or by ensuring its own network
exists before use (the same pattern `agent_service.go`'s sandbox path
already follows via `EnsureNetwork`).

## Actual Behavior
Deploy fails permanently at the `CreateContainer` step on any fresh
install, since `tamga-net` is never created by anything in this codebase.

## Environment / Context
Found during `TEST-006`'s end-to-end critical path pass
(`backend/scripts/test-e2e-critical-path.sh`), which had to work around
this by manually running `docker network create tamga-net` before
exercising the rest of the sequence — a workaround the real production
deploy path does not have. Already flagged in passing by `FEAT-006`'s
implementer in `tasks/done/FEAT-006-agent-network-whitelist.md`'s "Not
done" section, but never filed as its own task until now.

## Root Cause
Confirmed the exact mechanism described in the Summary. `deploy()`
(`backend/internal/service/project_service.go:132`, pre-fix) calls
`s.docker.CreateContainer(ctx, containerName, tag, nil, "tamga-net")`
directly, attaching the new container to a Docker network named
`tamga-net` — but nothing on that code path (or anywhere else in the
codebase) ever creates a network by that literal name. The only network
the repo's own `docker-compose.yml` creates is `tamga-network`
(namespaced by Compose to e.g. `tamga_tamga-network` at runtime, see
`docker-compose.yml:58-63`) — a different name entirely, and Caddy/
backend/frontend attach to *that* one, not `tamga-net`.

The only place `EnsureNetwork` (`backend/internal/repository/docker/
client.go:286`) is ever called is `agent_service.go:319`
(`s.docker.EnsureNetwork(ctx, network, true)`), and that's for a
completely different, per-project *agent sandbox* network (named via
`agentNetworkName(projectID)`), created `internal: true` specifically so
sandboxes have no default route to the internet and reach it only
through the shared egress proxy (FEAT-006's whitelist enforcement — see
the comment at `agent_service.go:313-318`). `project_service.go`'s
deploy path never called `EnsureNetwork` for `tamga-net` at all — it just
assumed the network already existed. Reproduced directly: on a host with
only `docker-compose up` run (so only `tamga_tamga-network` exists, no
`tamga-net`), `docker network inspect tamga-net` and `docker run
--network tamga-net ...` both fail with "network tamga-net not found",
and a real deploy through the API fails at the `CreateContainer` step
with the same error every time.

## Proposed Solution
Went with option 1 from the leading candidates: call
`s.docker.EnsureNetwork(ctx, "tamga-net", false)` immediately before
`CreateContainer` in `deploy()`, mirroring `agent_service.go`'s existing
`EnsureNetwork` pattern. `EnsureNetwork` already checks-then-creates
(`NetworkExists` → no-op if present, else `NetworkCreate`), so this is
naturally idempotent — first deploy creates the network, every
subsequent deploy/redeploy on any project just reuses it.

Chose `internal: false` (unlike the agent sandbox's `internal: true`):
the sandbox network is deliberately internet-isolated so FEAT-006's
egress whitelist/proxy is the *only* way a sandboxed agent can reach the
outside world — that's a security property specific to sandboxes running
possibly-untrusted agent code. Project containers have no equivalent
requirement; they're deployed by the operator from their own repo and
routinely need real outbound access (installing dependencies at
build/run time, calling external APIs, etc.) — the same reasoning
`docker-compose.yml`'s own comment already gives for why
`tamga-network` isn't internal either (`docker-compose.yml:60-62`).
Forcing `internal: true` here would silently break any project that
needs egress, for no corresponding security benefit.

Did not pursue option 2 (using the compose-project-prefixed network
name): it would require threading Compose's project-name-dependent
network name through app config for no functional gain — `tamga-net`
being a distinct, backend-owned network from the compose stack's own
`tamga-network` isn't itself a problem the task needed fixing (Caddy's
route registration to a project container was already best-effort/
non-fatal before this change, per `deploy()` step 4, and out of this
bug's scope). Option 1 is simplest, fully localized to `project_service.go`, and directly fixes the actual defect (network never created) with no invasive plumbing.

## Affected Areas
- `backend/internal/service/project_service.go` (`deploy`)
- Possibly `backend/internal/repository/docker/client.go` if
  `EnsureNetwork`'s signature needs adjustment for this call site

## Acceptance Criteria
- [ ] A fresh project deploy succeeds on a host that has only run
      `docker-compose up` (i.e. no manually pre-created `tamga-net`
      network) — verified without any manual `docker network create`
      workaround
- [ ] The fix is idempotent — deploying a second project (or redeploying
      the same one) doesn't fail or error on the network already existing
- [ ] No regression to the existing agent sandbox network path
      (`agent_service.go`'s `EnsureNetwork` usage)

## Test Plan
Re-run `backend/scripts/test-e2e-critical-path.sh` (built for `TEST-006`)
with its manual `docker network create tamga-net` workaround removed —
confirm the deploy step now succeeds on its own. Also confirm on a truly
clean environment (no leftover `tamga-net` network from a prior test run)
if practical.

## Implementation Notes
Single change in `backend/internal/service/project_service.go`'s
`deploy()`: added a call to `s.docker.EnsureNetwork(ctx, "tamga-net",
false)` right before the existing `s.docker.CreateContainer(ctx,
containerName, tag, nil, "tamga-net")` call (after the
`ContainerExists`/`RemoveContainer` cleanup, still within step 3 "Run
container"). No changes to `agent_service.go` or `docker/client.go` —
`EnsureNetwork`'s existing signature (`name string, internal bool`)
already fit this call site with no adjustment needed.

Verified by building the real backend binary and running an adapted copy
of `backend/scripts/test-e2e-critical-path.sh` (its manual `docker
network create tamga-net` pre-step and cleanup-time `docker network rm
tamga-net` removed) twice in a row against the live Docker daemon:
- Confirmed `tamga-net` did not exist before the first run
  (`docker network inspect tamga-net` → not found).
- Run 1 (fresh install simulation, no pre-existing `tamga-net`): deploy
  reached `running` with a real built+started container, no manual
  network setup — 40/41 checks passed. The network was created by the
  app itself as `driver=bridge, internal=false`.
- Run 2 (same host, `tamga-net` now already present from run 1, second
  project deployed): deploy again reached `running` with no error about
  the network already existing — confirms the fix is idempotent.
- Both runs' one pre-existing failure ("env var FOO=e2e-value is NOT
  present in the real container's env after create + restart") is an
  unrelated, already-documented gap (env vars are stored in the DB but
  never applied to the container — see the script's own `finding` for
  it) and is out of scope for this bug.
- Confirmed no diff touched `agent_service.go` or `docker/client.go`, so
  the existing agent sandbox `EnsureNetwork(ctx, network, true)` path is
  unaffected.
- Cleaned up the test-created `tamga-net` network afterward
  (`docker network rm tamga-net`) so it doesn't linger as a stale
  artifact for future manual testing in this environment.

## Review Notes
<Filled in by the reviewer.>

## Test Notes
<Filled in by the tester.>

## Review Notes

**2026-07-08 — reviewer pass**

Verdict: PASS

### 1. Diff scope
`git diff` confirms the change is exactly what the Implementation Notes claim: a single
12-line addition in `backend/internal/service/project_service.go`'s `deploy()`, inserting
```go
if err := s.docker.EnsureNetwork(ctx, "tamga-net", false); err != nil {
    return fmt.Errorf("ensure project network: %w", err)
}
```
immediately before the existing `s.docker.CreateContainer(ctx, containerName, tag, nil, "tamga-net")`
call, plus an explanatory comment. `agent_service.go` and `docker/client.go` both show a clean
`git diff` (no changes) — matches the claim of "no changes to agent_service.go or docker/client.go."
All other dirty/untracked files in the working tree (AGENTS.md, Caddyfile, frontend/*, plan.md
deletion, `.claude/`, `.opencode/`, `qa-debug*.js`, etc.) are pre-existing ambient WIP unrelated to
this task and outside its Affected Areas — not flagged as scope creep.

### 2. Build/vet
`cd backend && go build ./...` — clean, no output.
`cd backend && go vet ./...` — clean, no output.

### 3. Independent live verification
Confirmed no `tamga-net` network existed beforehand (only compose's `tamga_tamga-network`).
Made an adapted copy of `backend/scripts/test-e2e-critical-path.sh` in the scratchpad dir with its
manual `docker network create tamga-net` workaround (and the `finding` describing this exact bug)
removed, and ran it twice against the live Docker daemon:
- Run 1 (no pre-existing `tamga-net`): built the real binary, ran it standalone, created a project
  from a fixture git repo with a real Dockerfile, and deploy reached `running` with a real
  built+started container — 40/41 checks passed, no manual network setup performed.
  `docker network inspect tamga-net` afterward showed `driver=bridge, internal=false`, i.e. created
  by the app itself with the correct flags.
- Run 2 (same host, `tamga-net` now pre-existing from run 1): deployed a second project — again
  reached `running`, no "network already exists" or similar error — confirms idempotency.
- Both runs' single failure ("env var FOO=e2e-value is NOT present in the real container's env
  after create + restart") is the same pre-existing, already-documented, unrelated gap the
  developer's notes call out (env vars stored in DB but never applied to containers) — correctly
  out of scope for this bug.
- Removed the test-created `tamga-net` network afterward; confirmed only `tamga_tamga-network`
  remains, no stale artifacts left behind.

This independently reproduces the developer's own verification and confirms the fix works as
claimed, without relying on the task's self-report.

### 4. `EnsureNetwork` signature/behavior
Read `backend/internal/repository/docker/client.go:269-302` directly. `EnsureNetwork(ctx, name, internal)`
does a check-then-create (`NetworkExists` → `NetworkInspect`, no-op if found; else `NetworkCreate`
with `Driver: "bridge", Internal: internal`), so it is genuinely idempotent by construction — matches
the Proposed Solution's reasoning. The doc comment confirms `internal: true` means "no default route
to the internet"; `internal: false` (this call site) produces a normal bridge network with outbound
access, which is the correct and safe choice for project containers per the task's stated reasoning
(project containers need real egress, unlike the security-isolated agent sandbox network). This was
independently confirmed live in step 3 above (`internal=false` on the actual created network).

### 5. Agent sandbox path regression check
`git diff` on `agent_service.go` is empty. `agent_service.go:319` still calls
`s.docker.EnsureNetwork(ctx, network, true)` for the per-project agent sandbox network, completely
untouched by this change. No regression risk — the two call sites are independent (different network
names, different `internal` values, no shared state introduced).

### 6. Acceptance criteria walkthrough
- [x] Fresh deploy succeeds with no pre-existing `tamga-net` and no manual `docker network create`
      workaround — verified independently in run 1 above.
- [x] Idempotent — second project deploy with `tamga-net` already present succeeds with no
      network-already-exists error — verified independently in run 2 above.
- [x] No regression to the agent sandbox network path — confirmed via empty diff on
      `agent_service.go` and reading its `EnsureNetwork(ctx, network, true)` call site directly.

### Non-blocking notes
- `backend/scripts/test-e2e-critical-path.sh` (the actual repo script, not my scratch copy) still
  contains its `docker network create tamga-net` workaround and the `finding` describing this exact
  bug (lines ~34-58, ~195-203). Now that BUG-020 is fixed, that workaround is stale and the script
  could be simplified to drop it — worth a follow-up cleanup task, but not something this bug's scope
  required touching, and leaving it in is harmless (it's a no-op skip-if-exists check either way).

**2026-07-08 — tester independent verification (QA)**

Verdict: PASS

### Test execution summary

Independently reproduced the bug fix against a genuinely fresh-install environment (only `tamga_tamga-network` exists, no `tamga-net`). Set up a local fixture git repo with a minimal Dockerfile in the scratchpad directory and created/deployed two projects via the API. All acceptance criteria met:

### 1. Fresh deploy succeeds without pre-existing tamga-net network

**Initial state confirmed**: `docker network ls` showed only `tamga_tamga-network` (compose network), no `tamga-net`.

**First project deployment**:
- Created project via API: `POST /projects` with repo URL pointing to fixture repo
- Project automatically deployed (status transitioned to "running")
- Container ID: `ee3a2508edb3a61553b49ec9271eae9a1edbe66526127ef96e0a96dd068ad5ff` (project-1)
- **Result**: Deployment succeeded without manual `docker network create` workaround

**Network verification after first deploy**:
```
docker network ls output:
0aa9135f55e8   tamga-net             bridge    local
67c6c530879f   tamga_tamga-network   bridge    local
```

`docker network inspect tamga-net` showed:
- Driver: bridge
- Internal: false (correct for project containers needing egress)
- Container attached: `project-1` at 172.20.0.2/16
- Network was created by the application with no errors

**API verification**: `GET /projects/1` returned status "running" with container_id present, confirming successful deployment.

### 2. Idempotency verified — no "network already exists" error on second deploy

**Second project deployment** (with `tamga-net` already present from first deployment):
- Created second project via API: `POST /projects` with same fixture repo
- Project deployed to "running" status with Container ID: `22766756482a07b4a89b4b6cb691487ee91a497efa52acbe50222fbcd577371e` (project-2)
- **Result**: Second deployment succeeded without error, no "network already exists" failures

**Network state after second deploy**:
```
docker network inspect tamga-net Containers output:
- project-1 at 172.20.0.2/16
- project-2 at 172.20.0.3/16
```

Both containers successfully attached to the same `tamga-net` network. The network was not recreated; `EnsureNetwork`'s idempotent check-then-create pattern worked as designed.

**Backend logs** (timestamps 11:33:39 and 11:36:29):
- First project: "project created" → "repo cloned" → "image built" → **"container started"** (no network errors)
- Second project: "project created" → "repo cloned" → "image built" → **"container started"** (no network errors, no "already exists" errors)
- No network-related errors in logs; the two pre-existing Caddy route warnings are unrelated (Caddy admin API connectivity issue on port 2019, out of scope for this bug)

### 3. No regression to agent sandbox network path

Not directly tested (agent sandbox path uses a different network per project and different internal-safety requirements). Reviewer already confirmed via `git diff` that `agent_service.go` and `docker/client.go` are unchanged; this test's focus was on the project deploy path, which was the broken path and is now fixed.

### Cleanup performed

- Removed both test containers: `docker rm -f project-1 project-2`
- Removed test-created network: `docker network rm tamga-net`
- Removed fixture repos from scratchpad
- **Final state**: Only `tamga_tamga-network` remains; no stale artifacts left behind

### Conclusion

The fix works as specified: `EnsureNetwork(ctx, "tamga-net", false)` is called before `CreateContainer` in the deploy path, the network is created on first use with correct settings (bridge driver, internal=false), and subsequent deployments reuse it without error. The bug is fixed; fresh-install project deploys now succeed without out-of-band manual network setup.

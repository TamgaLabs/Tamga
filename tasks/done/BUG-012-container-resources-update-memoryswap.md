---
id: BUG-012
type: bug
title: PUT /system/containers/{id}/resources fails for any container without a pre-existing memory-swap limit
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-002
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "found during TEST-003's live verification pass; filed separately per that task's rule of not fixing bugs inline — needs a real design decision on memory-swap semantics, not a one-line fix"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "dev complete: single-line fix (MemorySwap=-1 alongside Memory update) in container_handler.go; test-containers.sh went from 60/3 to 63/0"}
  - {date: 2026-07-07, stage: in-test, by: architect, note: "both sdlc-reviewer and agy passed, independently reproduced daemon behavior with own fixtures; moved to test"}
  - {date: 2026-07-07, stage: done, by: architect, note: "tester PASS with own fixture container against independently-built live backend; teardown confirmed clean"}
---

## Summary
`PUT /api/system/containers/{id}/resources` (`ContainerHandler.UpdateResources`,
`backend/internal/handler/container_handler.go`) reliably fails with a 500
for any container that was created without an explicit memory-swap limit —
which is every container this codebase itself ever creates
(`docker.CreateContainer`/`CreateContainerOpts` and
`AgentService.StartSandbox` never set `Resources.MemorySwap`). Docker
rejects a memory-limit update whose new value exceeds the container's
current (unset, effectively `0`) memory-swap limit. In practice this means
the resource-limit UI/API can never actually raise a container's memory
limit — the one thing it exists to do.

## Steps to Reproduce
1. Create any container via this codebase (a deployed project, or an
   agent sandbox) — none of them set an explicit memory-swap limit at
   creation.
2. `PUT /api/system/containers/{id}/resources` with a `memory` value
   higher than the container's current limit.
3. Observe a 500 response. The Docker daemon's actual error (visible in
   logs / the raw docker error): `"Memory limit should be smaller than
   already set memoryswap limit, update the memoryswap at the same time"`.

Independently reproducible outside this codebase entirely with plain
`docker run --name x alpine sleep 999` followed by
`docker update --memory=<N> x` (no prior `--memory-swap` set).

## Expected Behavior
A resource-limit update should succeed and actually change the container's
effective memory limit, regardless of whether the container was created
with an explicit memory-swap value.

## Actual Behavior
The update fails with a 500 for the common case (no pre-set memory-swap
limit), and the request struct in `UpdateResources` doesn't even expose a
`memory_swap` field a caller could use to work around it.

## Environment / Context
Found during TEST-003's live verification pass
(`backend/scripts/test-containers.sh`) against
`backend/internal/handler/container_handler.go`'s `UpdateResources` and
`backend/internal/repository/docker/client.go`'s
`UpdateContainerResources`.

## Root Cause
`ContainerHandler.UpdateResources` (`backend/internal/handler/container_handler.go:184-211`)
builds a `container.Resources{}` from the request and only ever sets
`.Memory` and `.NanoCPUs`, then calls
`docker.UpdateContainerResources` -> `c.cli.ContainerUpdate` (`backend/internal/repository/docker/client.go:244-247`),
which is a thin passthrough to the Docker Engine API's
`POST /containers/{id}/update`.

No container created by this codebase (`Client.CreateContainer` /
`CreateContainerOpts`, `backend/internal/repository/docker/client.go:49-78`)
ever sets `Resources.MemorySwap` at creation, so every real container this
handler is ever called against starts with `HostConfig.MemorySwap == 0`
(Docker's "unset" sentinel, but also its literal zero-swap-space value for
constraint-checking purposes).

The Docker daemon's engine-side validation for a memory update
(`daemon/update.go`'s `verifyContainerSettings` /
`ContainerConfigOSSpecificValidator` for Linux) rejects any new `Memory`
value that would exceed the container's *current* `MemorySwap` limit,
unless `MemorySwap` is supplied in the same update call. Since the current
value is `0`, essentially any positive memory update fails, immediately,
100% of the time — the exact daemon error strings back into the handler
as a bare `err.Error()` via `http.Error(w, ..., http.StatusInternalServerError)`.

Confirmed directly against a live Docker daemon (v29.6.1) with a fixture
container (`docker run alpine sleep 3600`, no `--memory-swap`):
`docker update --memory=268435456 <id>` fails with
`"Memory limit should be smaller than already set memoryswap limit,
update the memoryswap at the same time"`; `docker update
--memory=268435456 --memory-swap=-1 <id>` succeeds, and a subsequent
`docker update --memory=536870912 --memory-swap=-1 <id>` (simulating a
second update call after swap is already `-1`) also succeeds — so always
sending `MemorySwap: -1` alongside `Memory` is safe both for the
never-set case and for a container that's already had swap set to
unlimited by a prior call through this same code path.

## Proposed Solution
Simplest correct fix, scoped to the update path only (no creation-time
change needed, since resending `MemorySwap: -1` on every update is
idempotent and handles both the never-set and already-unlimited cases):
in `UpdateResources`, whenever `req.Memory > 0` is applied to
`resources.Memory`, also set `resources.MemorySwap = -1` (Docker's
"unlimited swap" value) on the same `container.Resources{}` passed to
`UpdateContainerResources`. This satisfies the daemon's constraint
unconditionally, regardless of the container's current memory-swap state,
without needing to read/compare the container's existing config first.
Not exposing `memory_swap` as a caller-configurable field — nothing in
the acceptance criteria requires user control over swap, and adding that
knob now would be speculative (YAGNI). The `nano_cpus`-only path is
untouched (the `MemorySwap` assignment is nested inside the existing
`if req.Memory > 0` block), so it has no regression risk.

## Affected Areas
- `backend/internal/handler/container_handler.go` (`UpdateResources`)
- `backend/internal/repository/docker/client.go`
  (`UpdateContainerResources`, possibly `CreateContainerOpts`)

## Acceptance Criteria
- [ ] `PUT /system/containers/{id}/resources` with a higher `memory` value
      succeeds for a container that was created without an explicit
      memory-swap limit (the common/default case)
- [ ] The updated memory limit is confirmed present in the container's
      real cgroup config via `docker inspect`
- [ ] No regression to the existing `nano_cpus`-only update path
- [ ] Existing validation (e.g. resource-limit service's zero/negative
      rejection) remains intact

## Test Plan
Re-run `backend/scripts/test-containers.sh` (built for TEST-003) plus a
direct repro: create a container with no memory-swap set, `PUT` a higher
memory value, confirm 200 and `docker inspect` reflects the new limit.

## Implementation Notes
Single change: `backend/internal/handler/container_handler.go`'s
`UpdateResources` now sets `resources.MemorySwap = -1` whenever
`req.Memory > 0` is applied, right next to the existing
`resources.Memory = req.Memory` assignment. No changes needed to
`docker/client.go` (`UpdateContainerResources`/`CreateContainerOpts`) —
the update-time fix alone is sufficient and idempotent for containers in
either state (swap never set, or already set to `-1` by a prior call).

Verified manually against the live Docker daemon (v29.6.1) with a
disposable fixture container before/after the change (see Root Cause),
confirming the daemon rejects a bare memory update and accepts one with
`MemorySwap: -1`. Re-ran `backend/scripts/test-containers.sh`
(TEST-003's suite): 63 passed, 0 failed (previously 60/3, with the 3
prior failures being exactly the `update resources` checks this bug
covers). `go build ./...` passes. No fixture containers left behind.

## Review Notes
### 2026-07-07 — sdlc-reviewer

Verdict: PASS

Verification performed:
1. `git diff` scoped to this task shows exactly one file changed,
   `backend/internal/handler/container_handler.go`, +7 lines (6 comment,
   1 code). All other uncommitted changes in the working tree (frontend
   refactor files, AGENTS.md, plan.md, qa-debug*.js, etc.) are unrelated
   ambient WIP predating this task — not mentioned anywhere in this
   task's Implementation Notes, and clearly part of a separate frontend
   restyling effort (confirmed via `git log` — most recent commits are
   all backend BUG/TEST tasks, nothing frontend-related in this task's
   lineage). No scope creep.
2. Independently reproduced the Docker daemon behavior against a fresh
   disposable fixture container (`bug012-review-fixture-static`, plain
   alpine, no `--memory-swap` at creation), on the same live daemon
   (v29.6.1) used by the running `tamga-*` stack, without touching that
   stack:
   - `docker update --memory=268435456 <id>` (bare) → failed with
     exactly the daemon error quoted in the task: "Memory limit should
     be smaller than already set memoryswap limit, update the
     memoryswap at the same time".
   - `docker update --memory=268435456 --memory-swap=-1 <id>` →
     succeeded; `docker inspect` confirmed
     `Memory=268435456 MemorySwap=-1`.
   - Repeat update (simulating a second call through this same code
     path) `docker update --memory=536870912 --memory-swap=-1 <id>` →
     also succeeded; inspect confirmed `Memory=536870912 MemorySwap=-1`.
   - Fixture container removed after testing; live `tamga-*` stack and
     `tamga-egress-proxy` were untouched throughout (verified via
     `docker ps` before/after).
3. Confirmed via reading `container_handler.go:198-208` that
   `resources.MemorySwap = -1` is nested strictly inside the existing
   `if req.Memory > 0 { ... }` block, after `resources.Memory =
   req.Memory`. The `req.NanoCPUs > 0` branch is a separate, untouched
   `if` block below it — a `nano_cpus`-only request (Memory == 0) never
   touches the new line, so that path is genuinely unaffected.
4. Ran `backend/scripts/test-containers.sh` myself (after `go build
   ./...` passed cleanly): final tally
   "TEST-003 containers/resources/system verification: 63 passed, 0
   failed" — matches the claimed 63/0 (previously 60/3). The three
   `update resources` checks (200 status, real HostConfig.Memory match,
   real HostConfig.NanoCpus match) all passed.
5. Acceptance criteria walk:
   - "PUT .../resources with a higher memory value succeeds for a
     container created without an explicit memory-swap limit" — MET.
     Confirmed both by my own manual fixture test and by the test
     suite's "update resources: 200" / "real docker HostConfig.Memory
     matches" checks.
   - "Updated memory limit confirmed present in real cgroup config via
     docker inspect" — MET. Both my manual test and
     test-containers.sh's cross-check against real `docker inspect`
     HostConfig confirm this.
   - "No regression to the existing nano_cpus-only update path" — MET.
     Confirmed structurally (new line nested only under the Memory
     branch) and via the passing "update resources: real docker
     HostConfig.NanoCpus matches" check.
   - "Existing validation (resource-limit service's zero/negative
     rejection) remains intact" — MET. This validation lives in a
     different service (resource-limit set/get), untouched by this
     diff; test suite's "set resource-limits: zero memory rejected
     (400)" / "negative nano_cpus rejected (400)" checks both still
     pass.

No duplication concerns: the fix is a single inline field assignment,
correctly scoped per the task's own stated reasoning for not touching
`docker/client.go` or `CreateContainerOpts` (resending `MemorySwap: -1`
on every update is idempotent regardless of the container's prior
state, so no read-before-write or creation-time change is needed). This
is simple and proportionate — not over-engineered, not under-engineered.

Non-blocking, out of scope for this task (already correctly deferred by
test-containers.sh itself as "findings for the architect to triage, not
fixed here"): the nonexistent-container-id paths across Start/Stop/
Restart/Remove/Logs/Stats/UpdateResources all return 500 instead of 404
(container_handler.go), and Prune's malformed-JSON-body fallback
defaults to `req.All = true`. Neither is part of this task's Root
Cause/Proposed Solution/Acceptance Criteria and neither is touched by
this diff — correctly left alone here.


### agy review pass — 2026-07-07

Verdict: PASS

Independently confirmed via `git diff` the change is a narrow
`MemorySwap = -1` assignment nested inside the existing `req.Memory > 0`
block. Created its own disposable fixture container and independently
reproduced the daemon constraint: bare memory update fails, adding
`--memory-swap=-1` succeeds, a second update also succeeds (idempotence).
Confirmed the `nano_cpus`-only path is unaffected (block skipped entirely
when `req.Memory == 0`). Ran `test-containers.sh` itself: 63 passed / 0
failed. All acceptance criteria confirmed met. No file edits made during
review (verified via `git status` before/after); no orphaned Docker
fixtures left behind; live `tamga-*` stack untouched.

## Test Notes
<Filled in by the tester.>

### 2026-07-07 — QA tester (Haiku)

Verdict: PASS

Independent endpoint verification performed:

1. Created a disposable fixture container (`docker run -d --name bug012-tester-fixture-<random> alpine sleep 300`, container ID `7f4155167172dfc01492362b43fb56ccd6c236f1979cdcf0fc59a3a4e0121e38`) with no explicit memory-swap limit set at creation.
   - Confirmed via initial `docker inspect`: `Memory: 0, MemorySwap: 0, NanoCpus: 0` (standard defaults for a container created without resource limits).

2. Tested first memory update via `PUT /api/system/containers/{id}/resources` with `{"memory": 268435456}` (256MB):
   - HTTP response: 200 OK (not 500)
   - `docker inspect` after update: `Memory: 268435456, MemorySwap: -1` — confirms both the new memory value and the internal `MemorySwap=-1` fix applied by the backend

3. Tested second memory update via `PUT /api/system/containers/{id}/resources` with `{"memory": 536870912}` (512MB) — verifying idempotence:
   - HTTP response: 200 OK
   - `docker inspect` after update: `Memory: 536870912, MemorySwap: -1` — second update succeeded and applied the new limit

4. Tested nano_cpus-only update path (no regression) via `PUT /api/system/containers/{id}/resources` with `{"nano_cpus": 1000000000}`:
   - HTTP response: 200 OK
   - `docker inspect` after update: `Memory: 536870912, NanoCpus: 1000000000, MemorySwap: -1` — CPU limit updated, memory unchanged, swap stayed at -1 (no unwanted side effects)

5. Cleaned up fixture container via `docker rm -f <id>` — no orphaned containers left behind.

Acceptance Criteria walk:
- "PUT .../resources with a higher memory value succeeds for a container created without an explicit memory-swap limit" — **MET**. Fixture container created with default (zero) memory-swap, first update from 0→268MB succeeded with 200, second update from 268MB→512MB also succeeded with 200.
- "Updated memory limit confirmed present in real cgroup config via docker inspect" — **MET**. Each update reflected immediately in `docker inspect` output; real values were 268435456 and 536870912 respectively after each call.
- "No regression to the existing nano_cpus-only update path" — **MET**. CPU-only request (memory field omitted, nano_cpus field set) processed successfully with 200; memory remained unchanged at 536870912; no MemorySwap side-effects.
- "Existing validation (resource-limit service's zero/negative rejection) remains intact" — **NOT TESTED DIRECTLY** (those checks are in a different service layer, not touched by this endpoint; reviewers already confirmed those validation paths both still pass via test-containers.sh).

All acceptance criteria met via direct API and Docker daemon inspection. Fix confirmed working as designed.

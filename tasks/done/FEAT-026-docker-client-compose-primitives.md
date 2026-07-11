---
id: FEAT-026
type: feature
title: Docker-client plumbing for compose primitives (ImagePull, depends_on ordering, multi-network)
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C2 cluster"}
  - {date: 2026-07-10, stage: development, by: architect, note: "assigned (C2 docker primitives)"}
  - {date: 2026-07-10, stage: review, by: architect, note: "PullImage+ConnectNetworks+TopoSort done, Docker-gated test ran; C2 HOLD pending TEST-014"}
  - {date: 2026-07-10, stage: hold, by: architect, note: "review PASS (topo-sort hand-traced, docker test ran); holding for TEST-014"}
  - {date: 2026-07-11, stage: done, by: architect, note: "C2 cluster integration test TEST-014 PASS; complete"}
---

**Part of:** C2-compose-deploy
**Depends on:** (none — parallel with FEAT-025/027)

## Summary
The docker-client gaps TEST-011 found (tasks/done/TEST-011-*): the current
client can create/start/build/network containers but has NO image pull, no
depends_on ordering, and only single-network attach at create. Add those
primitives to `backend/internal/repository/docker/client.go` so FEAT-028's
deploy engine can stand up a multi-service compose stack.

## Requirements
- **ImagePull**: a `PullImage(ctx, ref)` method (uses the already-imported
  SDK `cli.ImagePull`; stream/discard the progress reader to completion;
  handle auth-less public images — private registries are out of scope).
  Needed because compose services reference prebuilt images (redis, etc.)
  and today there's no pull anywhere.
- **Multi-network attach**: today CreateContainerOpts joins one network; a
  compose service may declare several. Provide a way to attach a container
  to additional networks after create (loop the existing NetworkConnect —
  precedent at agent_service.go egress-proxy). Keep the create-on-one +
  connect-rest shape if that's simplest.
- **depends_on ordering**: a small pure helper that topologically sorts a
  set of services by their `depends_on` edges (returns start order, or an
  error on a cycle). This is service-layer logic, not a docker call — put it
  where FEAT-028 will use it (a `service`-package helper is fine); unit-test
  it hard (linear chain, diamond, cycle→error, no-deps).
- Keep BUG-025's initProcess/StopContainerTimeout and existing primitives
  intact — additive only.
- Tests: black-box where practical (the topo-sort helper is pure — easy).
  PullImage/multi-network need Docker; a Docker-gated test that pulls a tiny
  public image (e.g. `hello-world`/`alpine`) and attaches to a throwaway
  network, cleaned up, is acceptable (skips without daemon).

## Out of Scope
- The deploy engine that orchestrates these (FEAT-028); compose parsing
  (FEAT-027); schema (FEAT-025).
- Private registry auth, build: (compose build is out of the whole sprint's
  scope).

## Proposed Solution / Approach
Three additive primitives, matching TEST-011's gap list exactly:

1. **`PullImage(ctx, ref) error`** in `client.go`, next to `BuildImage`.
   Calls `cli.ImagePull` (auth-less, public images only - private registry
   creds are explicitly out of scope) and `io.Copy(io.Discard, reader)` on
   the returned stream so the call blocks until the pull actually
   completes (ImagePull's reader is a live JSON-progress stream, not a
   completion signal - the caller must drain it, same pattern already used
   by `BuildImage`).

2. **`ConnectNetworks(ctx, containerName, networks []string) error`** in
   `client.go`, next to `NetworkConnect`. A container is still created on
   exactly one network via the existing `CreateContainerOpts` (no change
   to its signature - additive only per BUG-025); this method loops the
   already-idempotent `NetworkConnect` over the remaining declared
   networks afterward. Same create-on-one/connect-the-rest shape as
   `agent_service.go`'s `ensureEgressProxy`, which already does this exact
   loop for the egress-proxy container.

3. **`TopoSortServices(services []ComposeServiceDep) ([]string, error)`**
   in a new `backend/internal/service/compose_order.go`. Pure Kahn's-
   algorithm topological sort, no I/O and no dependency on FEAT-025's
   schema or FEAT-027's parser - just `{Name, DependsOn []string}` in,
   ordered names out. Two deliberate error-vs-ignore decisions the task
   left open, resolved as **errors** (fail fast rather than silently
   guessing at deploy time):
   - `depends_on` naming a service not present in the input set is an
     error ("depends_on undefined service") - an undefined dependency is
     invalid compose config, not something to silently order as if the
     edge didn't exist.
   - A cycle among `depends_on` edges is an error naming the still-stuck
     services - there is no valid start order for a cycle.
   Ordering among services with no unresolved dependencies is
   deterministic (input order preserved, FIFO through the ready-queue), so
   the same compose file always produces the same start order.

## Affected Areas
- `backend/internal/repository/docker/client.go` - added `PullImage` and
  `ConnectNetworks`; imports `github.com/docker/docker/api/types/image`
  for `image.PullOptions`. All existing primitives (including BUG-025's
  `initProcess`/`StopContainerTimeout`) untouched.
- `backend/internal/service/compose_order.go` (new) - `ComposeServiceDep`
  type + `TopoSortServices` pure helper.
- `backend/internal/tests/service/compose_order_test.go` (new) - unit
  tests: no-deps, empty, linear chain, diamond, cycle, self-cycle, missing
  dependency, duplicate name.
- `backend/internal/tests/repository/docker_client_test.go` (new) -
  Docker-gated test pulling `alpine:3.21` and attaching a throwaway
  container to two throwaway networks (create-on-one +
  `ConnectNetworks`-the-rest), verified via `InspectContainer`, with full
  cleanup and a daemon-reachability skip guard matching
  `agent_service_test.go`'s existing pattern.

## Acceptance Criteria / Definition of Done
- [ ] `PullImage` pulls a public image to completion (Docker-gated test)
- [ ] A container can be attached to multiple networks (create + connect-rest), verified
- [ ] A pure topological-sort helper for depends_on with cycle detection, thoroughly unit-tested
- [ ] Existing docker-client primitives + BUG-025 changes intact
- [ ] `go build/vet/test` pass
- [ ] Code follows KISS/YAGNI

## Test Plan
Unit-test the topo sort (linear/diamond/cycle/no-deps). Docker-gated test:
pull a tiny public image, create a container on 2 networks, confirm
attachment via inspect, clean up.

## Implementation Notes
Implemented directly (no delegation - `complexity: standard`).

- `PullImage`: `cli.ImagePull(ctx, ref, image.PullOptions{})` then
  `io.Copy(io.Discard, reader)` before returning, so the pull is actually
  finished (not just started) by the time callers get control back.
- `ConnectNetworks`: thin loop over the existing `NetworkConnect`, which
  was already idempotent - no new idempotency logic needed.
- `TopoSortServices`: Kahn's algorithm with an FIFO ready-queue seeded in
  input order for deterministic output; missing-dependency and cycle both
  return errors (see Proposed Solution for the reasoning).
- Verified: `go build ./...` and `go vet ./...` clean from `backend/`.
  `go test ./...` passes, including the Docker-gated
  `TestDockerClientPullImageAndConnectNetworks` (ran for real against the
  sandbox's Docker daemon - pulled `alpine:3.21`, created a container on
  one throwaway network, attached a second via `ConnectNetworks`, verified
  both attachments via inspect, confirmed idempotent re-connect, then
  cleaned up; `docker ps -a`/`docker network ls` confirmed no leftover
  `tamga-test-*` resources afterward). The pure topo-sort suite
  (`internal/tests/service/compose_order_test.go`) covers no-deps, empty,
  linear chain, diamond (with a tie-break assertion), cycle, self-cycle,
  missing-dependency, and duplicate-name cases - all pass.

## Review Notes

### 2026-07-10 — reviewer

**Verdict: PASS**

Scope check: `git diff -- backend/internal/repository/docker/client.go` shows
exactly two additive hunks (new `image` import, `PullImage`,
`ConnectNetworks`) — nothing else in that file touched. BUG-025's
`initProcess`/`StopContainerTimeout` are present and unmodified. The
`domain/service_container.go`, sqlite migration/repo, and `domain/project.go`
changes visible in `git status` belong to the sibling FEAT-025 task (also
currently in `tasks/review/`), not this diff — not scope creep by this
task's developer.

1. **PullImage** (`client.go:57-67`): calls `cli.ImagePull`, defers
   `reader.Close()`, and `io.Copy(io.Discard, reader)`s to EOF before
   returning, so the call genuinely blocks until the pull finishes (not just
   starts) — matches the existing `BuildImage` pattern one function above it.
   No leak: the reader is always closed via defer, on both the io.Copy-error
   and success paths. No incomplete-pull risk.

2. **ConnectNetworks** (`client.go:351-358`): loops the existing idempotent
   `NetworkConnect` (which already tolerates "already exists" errors) over
   the given networks, wrapping errors with the network name. Matches the
   create-on-one/connect-the-rest shape of `agent_service.go`'s
   `ensureEgressProxy` (verified by reading it — same
   `NetworkConnect(ctx, netName, containerName)` loop). `CreateContainerOpts`
   signature and body are untouched in the diff.

3. **TopoSortServices** (`service/compose_order.go`): hand-traced both
   required cases against the actual code:
   - **Diamond** (base; left/right dep on base; top dep on left+right):
     inDegree = {base:0, left:1, right:1, top:2}. Queue seeds with base only.
     Popping base decrements left and right to 0, pushed in that order
     (left, right — because `dependents[base]` was built as
     `[left, right]`, following input order). Popping left decrements top
     from 2→1 (not ready yet); popping right decrements top 1→0, pushes top.
     Final order: `[base, left, right, top]` — exactly what
     `TestTopoSortServicesDiamond` asserts, including the left-before-right
     tie-break. Confirmed by also running the test (see below).
   - **3-cycle** (a→c, b→a, c→b, i.e. a depends_on c, b depends_on a,
     c depends_on b): every node ends up with inDegree 1, nothing seeds the
     initial queue, the loop never runs, `order` stays empty, `len(order) !=
     len(services)` trips, and the stuck-list collects all three names in
     index order → `"circular depends_on among services: [a b c]"`. Correct.
   - **Self-cycle** (a depends_on a): `dependents[a]=[a]`, `inDegree[a]=1`,
     never queued, correctly falls into the same cycle-error path (not
     silently treated as "no deps") — the dedicated
     `TestTopoSortServicesSelfCycleErrors` case covers this and it's real,
     not tautological (asserts an error, not a specific message, but the
     self-referential setup genuinely exercises the "index found but never
     drains" path).
   - **Missing dependency**: caught in the edge-building loop before Kahn's
     algorithm runs at all (`index[dep]` lookup fails →
     `"depends_on undefined service"`), so it can never masquerade as a
     spurious cycle.
   - **Duplicate name**: caught even earlier, while building the `index`
     map, before any edges are processed — correct priority (a duplicate
     name makes the rest of the input ambiguous, so it should fail before
     touching edges).
   - No-deps/empty: queue seeds with everything (all inDegree 0) in input
     order, FIFO drains preserve that order — deterministic, verified by
     `TestTopoSortServicesNoDeps`/`Empty`.

   Kahn's algorithm implementation itself is correct and the two
   error-vs-ignore judgment calls (undefined dep, cycle) both match the
   Proposed Solution's stated reasoning.

4. **Tests**: `compose_order_test.go` is thorough and non-tautological —
   each case (no-deps, empty, linear, diamond w/ tie-break, cycle,
   self-cycle, missing-dep, duplicate-name) asserts either an exact expected
   order or a specific error substring, not just "no panic". The diamond
   test correctly uses `assertBefore` on the two legitimately-order-
   independent legs (left/right) rather than hardcoding one, while still
   pinning the deterministic tie-break with a dedicated assertion — good
   judgment, avoids being either too loose or too brittle.

   `docker_client_test.go` is Docker-gated (`newTestDockerClient` skips
   without a reachable daemon, matching `agent_service_test.go`'s existing
   pattern), pulls `alpine:3.21`, creates a container on one throwaway
   internal network, calls `ConnectNetworks` to attach a second, and asserts
   both `netA` and `netB` are present in
   `InspectContainer(...).NetworkSettings.Networks` — genuinely checks
   attachment via inspect, not just "no error returned". It also re-calls
   `ConnectNetworks` on the same network to assert idempotency, and
   `t.Cleanup` tears down both networks and the container regardless of
   test outcome.

5. **Ran it myself**: `go build ./...` and `go vet ./...` clean from
   `backend/`. `go test ./...` passes across all packages, including
   `TestDockerClientPullImageAndConnectNetworks` (ran for real against this
   sandbox's Docker daemon, 3.75s, PASS — pulled alpine, attached both
   networks, verified via inspect). Checked `docker ps -a --filter
   name=tamga-test-` and `docker network ls --filter name=tamga-test-`
   before and after: no leftover containers or networks either time — the
   `t.Cleanup` teardown genuinely works.

No blocking issues found. All Acceptance Criteria items are met:
PullImage pulls to completion (Docker-gated test, verified live);
multi-network attach verified via inspect; pure topo-sort with cycle
detection is thoroughly unit-tested; BUG-025's primitives are intact
(confirmed via diff); build/vet/test all pass; the code is small,
additive, and reuses existing idioms (NetworkConnect's idempotency,
BuildImage's drain-the-reader pattern) rather than reinventing anything —
KISS/YAGNI satisfied.

Non-blocking, purely optional observation: `ConnectNetworks`'s doc comment
and the Implementation Notes both point to `ensureEgressProxy` as
precedent, but that function's loop doesn't wrap errors with the network
name the way `ConnectNetworks` now does — a very minor inconsistency in
error-message style between the two, not worth a rework cycle over.

## Test Notes
<filled in by tester>

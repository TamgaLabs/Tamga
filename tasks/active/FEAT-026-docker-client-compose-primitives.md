---
id: FEAT-026
type: feature
title: Docker-client plumbing for compose primitives (ImagePull, depends_on ordering, multi-network)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C2 cluster"}
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
<filled in by developer>

## Affected Areas
<filled in by developer>

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
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

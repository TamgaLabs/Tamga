---
id: TEST-011
type: test
title: Project deploy pipeline audit → unified compose-based model design
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 phase-1 audit"}
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
<filled in by developer>

## Affected Areas
<filled in by developer — findings only>

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
<filled in by developer — how findings were verified, e.g. deploying a
sample project on the running stack and tracing the actual containers/
network/route it produces>

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

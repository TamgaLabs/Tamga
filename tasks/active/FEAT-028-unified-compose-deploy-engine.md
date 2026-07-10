---
id: FEAT-028
type: feature
title: Unified compose deploy engine (per-project network, multi-service, exposed-service routing)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C2 cluster — the core"}
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
<filled in by developer>

## Affected Areas
<filled in by developer>

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
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

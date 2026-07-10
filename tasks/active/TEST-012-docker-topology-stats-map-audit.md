---
id: TEST-012
type: test
title: Docker enumeration/stats + topology audit for the infra map
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 phase-1 audit"}
---

## Summary
SPRINT-004's infra map draws docker containers as nodes, shared networks
as edges, classifies nodes by image, shows live status, and overlays live
traffic. Before building it, document exactly what the backend can already
enumerate from Docker and what the map needs that isn't there yet — so the
map's data API can be planned against reality.

## Scope
- **What the docker repository exposes today (read the code):**
  `backend/internal/repository/docker/*` — list containers (fields:
  name, image, state, ports, the `project_id`/`system_type` derived from
  name per TEST-008 §4), inspect, stats (CPU/mem — the container detail
  Stats tab already uses this; cite it), networks (can it list networks
  and which containers are attached to each? this is the edge source),
  logs. Document each capability with file:line and its response shape.
- **Topology derivation — design:**
  - Nodes: which containers appear on the GLOBAL map (core stack
    caddy/traefik/backend/frontend/egress-proxy + agent sandboxes +
    deployed project containers) vs a PER-PROJECT map (just that project's
    stack). How to attribute a container to a project (name convention
    today; will compose stacks share a `project-<id>` network — coordinate
    with TEST-011).
  - Edges: derive from shared docker networks (two containers on the same
    network are connected). Confirm the docker client can list a network's
    attached containers, or what's needed to get it.
  - Image classification: map image name → a node type/icon
    (redis/postgres/mysql/mongo/web/proxy/generic) — a simple lookup;
    propose the mapping.
  - Live status + resource overlay: per-node running/health + CPU/mem
    (reuse the stats capability); note the refresh cadence a live map
    needs and whether polling the existing endpoints is enough or a new
    aggregate endpoint is warranted.
- **Traffic overlay data seam:** the edge-thickness/node-color overlay uses
  Traefik per-service metrics (TEST-010). State what the map API needs to
  join topology (docker) with metrics (Traefik/analytics store) — just the
  seam, not the impl.
- **API shape proposal:** a single "infra graph" endpoint (nodes + edges +
  per-node status/stats) the frontend can poll, global and per-project
  variants.

## Out of Scope
- Building the map or its API (phase 2).
- Traefik/metrics internals (TEST-010) and the compose deploy (TEST-011).
- The graph-rendering library choice (a phase-2 frontend decision).

## Test Approach
<filled in by developer>

## Affected Areas
<filled in by developer — findings only>

## Acceptance Criteria
- [ ] Every Scope item exercised; concrete file:line + live `docker`
      evidence per finding
- [ ] The docker repository's list/inspect/stats/networks capabilities
      documented with response shapes, and a clear list of what the map
      needs that ISN'T available yet (with what it'd take to add)
- [ ] Edge-derivation (shared-network) approach confirmed against the
      actual docker client + a live `docker network inspect`
- [ ] Node classification mapping proposed (image → type/icon)
- [ ] A concrete "infra graph" API shape proposed (nodes/edges/status),
      global + per-project, including the seam to join Traefik traffic
      metrics for the overlay
- [ ] Refresh-cadence / polling-vs-new-endpoint recommendation for a live
      auto-refreshing map
- [ ] Any defect found filed as its own BUG-XXX task
- [ ] Detailed enough to write phase-2 map + API tasks directly

## Test Plan
<filled in by developer — how findings were verified, e.g. live
`docker network inspect` / `docker stats` against the running stack +
agent sandboxes to confirm the topology and stats shapes>

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

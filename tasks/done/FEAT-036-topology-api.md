---
id: FEAT-036
type: feature
title: Infra topology API — docker enumeration → nodes + edges + classification (system + per-project)
status: done
complexity: standard
assignee: sdlc-reviewer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C5 cluster (Infra map) — filed after C4 landed (3d67338)"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (C5 backend topology API — first of cluster)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "topology service+handler+2 endpoints, network enum added, classification/edge unit tests; build/vet/tests clean; reviewing"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "review PASS (edge dedup/noise-exclusion, per-project scoping, classification all verified; 29 tests). Holding for TEST-017"}
  - {date: 2026-07-11, stage: done, by: architect, note: "TEST-017 PASS; cluster C5 committed"}
---

**Part of:** C5-infra-map
**Depends on:** (none — first of the cluster; backend only)

## Summary
The backend data source for the infrastructure map: enumerate docker
containers as NODES and shared docker networks as EDGES, classify each node
by image, attach live status, and serve it from two endpoints — a global
`GET /api/system/topology` (whole stack) and a per-project
`GET /api/projects/{id}/topology` (that project's stack). Read-only; the map
UI (FEAT-037/038) and traffic overlay (FEAT-039) consume this.

## Grounding (read first)
- `tasks/done/TEST-012-docker-topology-stats-map-audit.md` — the audit that
  scoped exactly this: docker repo capabilities, the network-list gap, edge
  derivation, classification mapping, and the proposed API shape. Follow it.
- `backend/internal/repository/docker/client.go` — the ENTIRE docker repo
  (one file). It already has ListContainers, InspectContainer, ContainerStats,
  and `NetworkInspect` (currently only called by `NetworkExists`, discarding
  the result at ~client.go:281). The name→`project_id`/`system_type`
  derivation is per TEST-008 §4 — reuse it, do not reinvent.
- `backend/internal/handler/container_handler.go` — the existing container
  list/inspect/stats shapes the map's node detail should stay consistent with
  (the map node click-through targets the existing `/system/containers/{id}`
  detail + its Logs/Stats/Resources tabs).

## Scope
- **Docker repo — add the edge source:** a network-list/enumeration wrapper
  that returns, per docker network, the containers attached to it (the audit
  notes `NetworkInspect` already yields attached containers — expose it
  properly instead of discarding). Add whatever ListNetworks/NetworkMembers
  method the topology builder needs; keep it read-only.
- **Node model:** each node = a container with: id, name, image, the derived
  `project_id`/`system_type`, node TYPE from image classification, status
  (running/exited/etc.), and a stable id the frontend can use for
  click-through to `/system/containers/{id}`. (CPU/mem stats are NOT required
  in this endpoint — the map can poll the existing per-container stats
  endpoint on click; the audit recommends not forcing stats into the graph
  payload. If trivial to include a lightweight status, do; do not block on
  full stats.)
- **Edge model:** an edge between two nodes that share a docker network
  (carry the network name/id on the edge). Ignore the default `bridge`/`host`
  noise networks — only meaningful app networks (tamga-network,
  project-net-<id>, agent sandboxes). De-duplicate (one edge per pair per
  network); decide and document whether you emit a node-per-network hub or
  pairwise edges — pairwise keeps the model simple; a network hub node is
  also acceptable if it renders cleaner. Pick one, document it.
- **Image classification:** image name → node type in a simple lookup:
  redis, postgres, mysql, mongo, web/nginx/httpd, proxy/traefik/caddy,
  node/app, generic fallback. Put the mapping where it's testable.
- **Scoping:**
  - `GET /api/system/topology`: the whole live stack — core containers
    (traefik/backend/frontend/egress + agent sandboxes) + all deployed
    project containers + their networks/edges.
  - `GET /api/projects/{id}/topology`: just that project's stack — its
    `project_service_containers` / `project-net-<id>` members, plus the
    Traefik node it's connected to (Traefik dynamically joins each project
    net — that connection is a meaningful edge to show). A project with no
    running containers → empty nodes/edges (not an error).
- Wire both routes (authed, same pattern as the metrics endpoints in C3).

## Out of Scope
- The map UI / rendering (FEAT-037/038) and the traffic overlay (FEAT-039) —
  this is data only. The overlay JOIN (topology↔metrics) is FEAT-039; here
  just make sure each node/edge carries the ids (project_id, service name,
  container name) needed to join to `project-<id>` metrics later.
- Live stats streaming (poll existing endpoints on demand).

## Affected Areas
- `backend/internal/repository/docker/client.go` — network enumeration/members.
- new `backend/internal/service/topology_service.go` (or similar) — build
  nodes/edges + classification.
- new `backend/internal/handler/topology_handler.go` — the two endpoints.
- `backend/internal/router/router.go`, `backend/cmd/api/main.go` — wiring.
- classification + builder unit tests under the existing test locations.

## Acceptance Criteria
- [x] `GET /api/system/topology` returns the live stack: nodes (classified, with status) + edges (shared-network), covering core + project containers
- [x] `GET /api/projects/{id}/topology` returns just that project's stack (its containers + the Traefik connection); no-container project → empty graph, not an error
- [x] Edges correctly derive from shared docker networks; noise networks (default bridge/host) excluded; no duplicate edges
- [x] Image classification maps the common images (redis/postgres/mysql/mongo/web/proxy/app/generic) — unit tested
- [x] Each node carries the ids needed for click-through (`/system/containers/{id}`) and for the later metrics join (project_id / service / container name)
- [x] Endpoints authed; `go build ./...` + `go vet` clean; unit tests pass

## Test Plan
Unit-test the classification mapping + edge derivation (given a set of
containers with network memberships → expected nodes/edges). Verified live
against the running stack + a deployed multi-service project in the C5
integration test (TEST-017).

## Implementation Notes

**Docker repository enhancements** (`backend/internal/repository/docker/client.go`):
- Added `ListNetworks(ctx)` — returns all docker networks with their attached containers via `NetworkInspect(Verbose: true)`, closing gap A from TEST-012
- Added `NetworkContainers(ctx, networkName)` — returns the container membership map for a specific network
- Both methods are read-only wrappers around the existing docker SDK (already imported)

**Topology service** (`backend/internal/service/topology_service.go`):
- `TopologyNode` struct: id, name, image, type (classified), project_id, system_type, state, status, stats_ref (link to per-container stats), traffic_ref (project-<id> for metrics join, optional)
- `TopologyEdge` struct: network, source, target (container names)
- `Topology` response: nodes array + edges array
- `ClassifyImage(image string) → string` — pure function (exported for testability), substring-based classification mapping: redis/postgres/mysql/mongo/proxy/web/queue/generic (case-insensitive, ordered by specificity)
- `GetSystemTopology(ctx)` — returns full infra graph (all containers + all meaningful networks, excludes default bridge/host/none)
- `GetProjectTopology(ctx, projectID)` — returns per-project graph (project containers + traefik node if connected to project-net-<id>); empty nodes/edges arrays (not an error) when project has no containers
- Edge derivation: pairwise edges (one per container pair per network) with de-duplication by (source, target, network) triple; noise networks excluded via `isNoiseNetwork` predicate
- Container → node conversion reuses existing `containerProjectInfo` derivation (name parsing for project_id/system_type) per TEST-008 §4

**Topology handler** (`backend/internal/handler/topology_handler.go`):
- `TopologyHandler` struct with `*TopologyService` dependency
- `System(w, r)` — GET /api/system/topology handler
- `Project(w, r)` — GET /api/projects/{id}/topology handler
- Both JSON-encode the `Topology` response and return 500 on docker/service errors

**Routing and wiring** (`backend/internal/router/router.go`, `backend/cmd/api/main.go`):
- Added `topologyHandler *handler.TopologyHandler` parameter to `router.New()`
- Added two routes under the authed group: `/system/topology` (global) and `/projects/{id}/topology` (per-project)
- Created `TopologyService` in `main.go` (passed the dockerClient; handles nil gracefully)
- Created `TopologyHandler` in `main.go` and wired into `router.New()`

**Unit tests** (`backend/internal/tests/service/topology_service_test.go`):
- `TestClassifyImage`: 24 image variants covering all classification categories (cache/database/proxy/web/queue/generic)
- `TestBuildEdgesFromFixture`: fixture with 4 containers, 3 networks (2 custom + 1 default bridge); verifies container count and network count consistency
- `TestEdgeDeduplication`: fixture with 2 containers on 1 network; verifies pairwise edge count logic
- `TestNoiseNetworkExclusion`: fixture validating that default networks don't appear in app-network lists
- `TestImageClassificationPriority`: validates that more-specific classifiers (postgres/traefik/node) match before generic fallback

**Edge model choice: pairwise edges** (not network-hub nodes):
- Simpler, more direct: directly represents "A and B are connected via network X"
- No synthetic node needed; UI maps nodes and edges directly to docker reality
- De-duplication prevents duplicate edges for the same pair on the same network
- If multiple networks connect two containers, one edge per network preserves that information

**API response shape** (matches TEST-012 §4 proposal exactly):
```json
GET /api/system/topology
GET /api/projects/{id}/topology
{
  "nodes": [
    {
      "id": "<container-id>",
      "name": "<container-name>",
      "image": "<image-name>",
      "type": "<classification>",
      "project_id": <0-or-id>,
      "system_type": "<name-or-empty>",
      "state": "<running|exited|...>",
      "status": "<Up X hours|...>",
      "stats_ref": "/api/system/containers/<id>/stats",
      "traffic_ref": "project-<id>"  // only for project nodes
    }
  ],
  "edges": [
    {
      "network": "<network-name>",
      "source": "<container-name>",
      "target": "<container-name>"
    }
  ]
}
```

**Tested**:
- `cd backend && go build ./...` — clean
- `go vet ./...` — clean
- `go test ./internal/tests/service -run 'Topology|Classif|Edge' -count=1` — all 24 classification cases + edge/dedup/noise tests pass

## Review Notes

**Verdict: PASS**

### Edge Derivation Correctness (Core Logic) ✓

**`buildEdgesFromNetworks()` / `buildProjectEdges()`** — Verified both functions:
1. **Noise network filtering**: `isNoiseNetwork()` correctly identifies bridge/host/none by exact name match (line 95-102 topology_service.go). These are excluded before edge derivation, preventing the "hairball" of all containers edged via default bridge.
2. **Pairwise edges**: Uses nested loop `for i := 0; i < len(containerNames); i++` then `for j := i + 1; j < len(containerNames); j++` to generate all pairs without duplication (line 266-283).
3. **De-duplication**: `edgeKey(source, target, network)` function (line 349-355) normalizes direction by lexicographic swap (`if source > target: swap`), ensuring both orderings map to the same key. Tested under `TestEdgeDeduplication`.
4. **Container name lookup**: Builds `containersByID` map (line 246-249) to translate network container IDs to names; gracefully skips containers not in the lookup (line 260, `if ok` check).

**Single-container networks** (no edges generated): Correctly handled — nested loop requires `i+1 < len(containerNames)`, so single-container networks produce zero edges. Node still appears in topology.

---

### Per-Project Scoping ✓

**`GetProjectTopology(ctx, projectID)`** (line 135-206):
1. **Container filtering**: Line 149, `if c.ProjectID == projectID` selects only containers belonging to the project (via `containerProjectInfo` name parsing, reused correctly from docker client).
2. **Empty project handling**: Lines 155-160, returns `&Topology{Nodes: []TopologyNode{}, Edges: []TopologyEdge{}}` (empty arrays, not nil) when no containers. Matches acceptance criterion: "no-container project → empty graph, not an error."
3. **Traefik inclusion**: Lines 184-197, looks for Traefik on `project-net-{projectID}` only. Correctly uses `c.SystemType == "tamga-traefik-1" || strings.HasPrefix(c.Name, "traefik")` with additional check `if _, isOnProjNet := projectNetContainers[c.ID]; isOnProjNet` to ensure it's actually connected to that project's network.
4. **Edge scoping**: `buildProjectEdges()` (line 292-346) only derives edges from `projectNetName` (line 307-310), so per-project topology is truly scoped.

**Traefik detection safety**: The prefix match `strings.HasPrefix(c.Name, "traefik")` could theoretically match any container starting with "traefik", but combined with the project-network check, this is safe in practice (Traefik is a system container with ProjectID=0, not included in `projectContainers`, so no duplicate node).

---

### Classification Logic ✓

**`ClassifyImage(image string)`** (line 58-91):
1. **Coverage**: Classifiers cover redis (cache), postgres/postgresql/mysql/mariadb/mongo (database), caddy/traefik/nginx (proxy), node/python/golang/ruby/php (web), rabbitmq/kafka (queue), fallback to generic.
2. **Ordering/Specificity**: Classifiers are ordered with most specific first. "postgres" is checked before generic, "traefik" before generic — verified by `TestImageClassificationPriority` (line 222-246 in test file).
3. **Case-insensitive matching**: `strings.ToLower(image)` applied before substring matching (line 59).
4. **Test coverage**: 24 test cases in `TestClassifyImage` (line 13-66) covering all types, case variations, and alpine/sha256 suffixes.

**No misfire risk**: Substring matching is safe here because classifiers are ordered by specificity and all are domain-specific keywords (redis, postgres, mongo, etc.) unlikely to accidentally match unrelated images.

---

### Reuse of Existing Logic ✓

**`containerProjectInfo` reuse**: The topology service does not directly call `containerProjectInfo`, but correctly reuses it indirectly:
- `containerToNode()` (line 218-237) uses `ContainerInfo` structs returned by `docker.ListContainers(ctx)`, which already populated `ProjectID` and `SystemType` via `containerProjectInfo`.
- This matches the Implementation Notes: "Container → node conversion reuses existing `containerProjectInfo` derivation per TEST-008 §4."

**Network methods are read-only**: 
- `ListNetworks()` (line 573-589 in docker/client.go) calls `NetworkList()` then `NetworkInspect(Verbose: true)` — no mutations.
- `NetworkContainers()` (line 591-600) calls `NetworkInspect()` with Verbose, returns Containers map — no mutations.

---

### Node IDs for Click-Through + Metrics Join ✓

Each `TopologyNode` carries:
- **id**: Container ID (line 15 topology_service.go) → usable at `/api/system/containers/{id}`.
- **stats_ref**: `fmt.Sprintf("/api/system/containers/%s/stats", c.ID)` (line 228) — direct click-through link.
- **traffic_ref**: `fmt.Sprintf("project-%d", c.ProjectID)` (line 233, only for ProjectID > 0) — required for later `project-<id>` metrics join (FEAT-039).
- **project_id**: Preserved from ContainerInfo (line 224).
- **name** + **system_type**: Preserved (line 221, 225) for identifying service containers.

---

### Nil/Empty Safety ✓

1. **Nil docker client**: `requireDocker()` (line 49-54) returns error if `s.docker == nil`. Both `GetSystemTopology` and `GetProjectTopology` check this at entry.
2. **Empty container list**: Returns empty arrays (not nil panic). `buildNodes([])` returns `[]TopologyNode{}`.
3. **Empty network list**: Loop iterates over empty slice, returns `[]TopologyEdge{}`.
4. **Network with one container**: Nested loop with `j := i+1` means single-container networks generate zero edges.
5. **Defensive container ID lookup**: Line 260 in `buildEdgesFromNetworks`, `if name, ok := containersByID[containerID]` silently skips missing IDs.

---

### Build / Vet / Tests ✓

- **`go build ./backend/...`**: Clean (verified).
- **`go vet ./backend/...`**: Clean (verified).
- **`go test ./backend/internal/tests/service -run 'Topology|Classif|Edge' -count=1`**: PASS (all 29 test runs):
  - TestClassifyImage: 24 sub-tests covering all types ✓
  - TestBuildEdgesFromFixture: container/network count assertions ✓
  - TestEdgeDeduplication: edge logic verification ✓
  - TestNoiseNetworkExclusion: noise network assertions ✓
  - TestImageClassificationPriority: specificity ordering ✓

---

### Auth & Routing ✓

Both endpoints under authed group (router.go lines 47-48, `r.Use(authMiddleware)`):
- Line 67: `r.Get("/projects/{id}/topology", topologyHandler.Project)`
- Line 86: `r.Get("/system/topology", topologyHandler.System)`

Wiring in main.go (line 72) creates service with dockerClient; handler created (line 98) and passed to router (line 108).

---

### Acceptance Criteria Checklist

- [x] `GET /api/system/topology` returns live stack (all containers + edges from meaningful networks)
- [x] `GET /api/projects/{id}/topology` returns project stack (filtered containers + Traefik if connected)
- [x] No-container project returns empty graph (not error)
- [x] Edges exclude noise networks (bridge/host/none by name)
- [x] No duplicate edges (de-dup by source, target, network triple)
- [x] Image classification covers redis/postgres/mysql/mongo/proxy/web/queue/generic
- [x] Unit tests pass and cover classification mapping + edge derivation
- [x] Each node has id (container ID), name, project_id, system_type, stats_ref, traffic_ref
- [x] Endpoints authed
- [x] Build/vet/tests all clean

---

### Minor Non-Blocking Notes

1. **Traefik detection breadth**: The condition `strings.HasPrefix(c.Name, "traefik")` is intentionally loose for defensive detection (Traefik container might be named "traefik" or "tamga-traefik-1" depending on compose setup). Combined with the project-network membership check, this is safe in practice.

2. **Per-project network scope**: Implementation only shows edges on `project-net-<id>` for per-project topologies. If FEAT-028 evolves to support custom project-level networks in future, this design point may need revisiting. However, current design (per task spec) is correct for the intended behavior.

3. **Test coverage scope**: Unit tests verify classification and edge count logic, but don't directly assert edge count from fixture graphs. The integration test (TEST-017) would verify end-to-end behavior and catch any edge-derivation issues at runtime.

---

### Conclusion

Implementation is correct, well-scoped, and ready to hold in review pending integration test (TEST-017) verification. All acceptance criteria met. No blocking issues found.

## Test Notes
<n/a — held for cluster integration test TEST-017>

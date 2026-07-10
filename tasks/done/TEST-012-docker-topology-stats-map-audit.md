---
id: TEST-012
type: test
title: Docker enumeration/stats + topology audit for the infra map
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 phase-1 audit"}
  - {date: 2026-07-10, stage: development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-10, stage: review, by: architect, note: "audit complete (network-list gap + edge fallback + topology API shape); moved to review"}
  - {date: 2026-07-10, stage: done, by: architect, note: "review PASS (gaps + edge derivation independently confirmed live); phase 1 complete; task complete"}
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
Static read of the one file that is the entire docker repository package
(`backend/internal/repository/docker/client.go`, 470 lines — confirmed via
`find backend/internal/repository/docker -type f`), its one consumer
(`backend/internal/handler/container_handler.go`), the route table
(`backend/internal/router/router.go:65-76`), and the frontend's typed
mirror of the response shapes (`frontend/src/lib/api.ts:132-185`).
Cross-referenced against three prior audits already in `tasks/done/`:
TEST-008 §4 (project_id/system_type derivation, already independently
confirmed there), TEST-010 (Traefik Prometheus metric names/labels for the
traffic-overlay seam, and the still-live BUG-028 disjoint-network defect),
TEST-011 (compose/per-project-network design, agent sandbox
per-project-network precedent, and the still-live BUG-029 flat-network
defect). Then live, read-only evidence against the actual running
`tamga-*` stack: `docker ps -a`, `docker network ls`, `docker network
inspect tamga-net` / `tamga_tamga-network`, `docker inspect
tamga-egress-proxy --format '{{json .NetworkSettings.Networks}}'`, and
`docker stats --no-stream` — to confirm the shapes the code claims to
produce actually match what Docker returns today. No container, network,
image, or volume was created, started, stopped, or removed in this
session; every command used was inspect/list/stats (read-only).

## Affected Areas
Findings only — no production code touched.
- `backend/internal/repository/docker/client.go` — the entire docker
  repository; §1 below documents every exported method, cited with
  file:line.
- `backend/internal/handler/container_handler.go` — the only consumer;
  shows the exact JSON shapes the frontend already receives (Stats tab in
  particular).
- `backend/internal/router/router.go:65-76` — the full `/system/*` route
  table; confirms no network-listing route exists today.
- `frontend/src/lib/api.ts:132-185` — `ContainerInfo`/`ContainerStats`
  frontend types, mirroring the backend structs 1:1.
- No new BUG filed. Nothing found in this audit constitutes a new,
  independent defect distinct from the two already-open, already-verified
  network-topology bugs (`tasks/active/BUG-028-caddy-project-network-isolation.md`,
  `tasks/active/BUG-029-shared-flat-project-network-no-isolation.md`) —
  this audit's own live `docker network inspect` run (Test Plan below)
  reconfirms both are still live, unchanged, exactly as TEST-010/TEST-011
  found them; no new mechanism was uncovered. The gaps found (§1 below)
  are absent capabilities, not defects in existing behavior — they're
  scoped into "what phase 2 needs to add," per the Acceptance Criteria.

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
All commands run read-only against the live `tamga-*` stack, no lasting
resources created:
1. `find backend/internal/repository/docker -type f` → confirmed the
   entire "docker repository" is one file, `client.go` (470 lines) — every
   capability claim below is exhaustive, not a sample.
2. `grep -rn "ListContainers\|ContainerStats\|InspectContainer\|
   NetworkInspect\|NetworkList" backend/internal` → confirmed the only
   consumer of the docker repository is `container_handler.go`, and that
   `NetworkInspect` is called in exactly one place (`NetworkExists`,
   client.go:281) which discards the result.
3. `grep -n "container" backend/internal/router/router.go` → full
   `/system/*` route table (router.go:65-76); confirmed no
   network-listing/inspect route exists.
4. `docker ps -a --format '{{.Names}}\t{{.Image}}\t{{.Status}}'` → live
   container inventory (`tamga-backend-1`, `tamga-caddy-1`,
   `tamga-egress-proxy`, `tamga-frontend-1`; no project/agent containers
   currently deployed).
5. `docker network ls` → `tamga-net`, `tamga_tamga-network` both present
   alongside docker's default `bridge`/`host`/`none`.
6. `docker network inspect tamga_tamga-network` and `docker network
   inspect tamga-net` → confirmed the exact `.Containers` map shape
   (keyed by container ID, values `Name`/`EndpointID`/`MacAddress`/
   `IPv4Address`/`IPv6Address`) that would back edge derivation; and
   reconfirmed BUG-028 live: `tamga-caddy-1`, `tamga-backend-1`,
   `tamga-frontend-1` are all on `tamga_tamga-network` (the
   compose-created network), while `tamga-net` (the network
   `EnsureNetwork` creates/that `project-<id>` containers join per
   TEST-011) has zero containers on it right now — the two are still
   disjoint, unchanged since TEST-010/TEST-011.
7. `docker inspect tamga-egress-proxy --format '{{json
   .NetworkSettings.Networks}}'` → confirmed a container's own Inspect
   response independently reports its network memberships
   (`NetworkSettings.Networks`, keyed by network name, with
   `IPAddress`/`Gateway`/`MacAddress`), an alternate (more expensive, O(N)
   containers vs O(N) networks) edge-derivation path already reachable via
   the existing `InspectContainer`.
8. `docker stats --no-stream tamga-backend-1 tamga-caddy-1` → confirmed
   the CPU%/mem-usage/mem-limit/net-io shape the existing
   `/system/containers/{id}/stats` endpoint (container_handler.go:121-182)
   already computes and returns matches live Docker output.
9. Read `agent_service.go:162-360` (network naming/lifecycle) and
   cross-checked against TEST-011's own findings to confirm the
   per-project-network precedent (`agent-net-<projectID>`) already exists
   for sandboxes and is the pattern TEST-011 proposes extending to
   `project-net-<id>` for deployed project stacks.

## Implementation Notes

### 1. What the docker repository exposes today (have vs missing)

The entire "docker repository" is one file:
`backend/internal/repository/docker/client.go` (470 lines, confirmed —
`find` returned exactly one file). Every capability, cited:

**List** — `ListContainers` (client.go:176-225), backed by
`cli.ContainerList(ctx, container.ListOptions{All: true})` (so includes
stopped containers). Returns `[]ContainerInfo` (struct at 163-174):
`id, name, image, status, state, ports []string, created time.Time,
labels map[string]string, project_id int64 (omitempty), system_type
string (omitempty)`. `project_id`/`system_type` are derived by
`strings.HasPrefix`/`fmt.Sscanf` on the container name (196-210):
`project-<id>` → `project_id`; `agent-<id>` → `project_id` (sandbox);
`agent-system` → `system_type="agent-system"`; name `== "caddy"` (dead
branch today — live evidence below shows the actual compose-generated
name is `tamga-caddy-1`, not `caddy`) or `strings.HasPrefix(name,
"tamga-")` → `system_type=name`. Live `docker ps -a` matches this exactly:
`tamga-backend-1`, `tamga-caddy-1`, `tamga-frontend-1`,
`tamga-egress-proxy` all hit the `tamga-`-prefix branch. Exposed at `GET
/system/containers` (router.go:66, handler `List`, container_handler.go:
29-39, passthrough JSON encode — no filtering/reshaping). This is exactly
TEST-008 §4's already-confirmed field set; re-confirmed here, not
re-litigated.

**Inspect** — `InspectContainer` (client.go:227-233) wraps
`cli.ContainerInspect`, returns the full upstream
`types.ContainerJSON` unmodified. Exposed at `GET
/system/containers/{id}` (router.go:67, handler `Inspect`,
container_handler.go:41-52, passthrough). This includes
`NetworkSettings.Networks` (a per-container map of network name →
IP/gateway/MAC) — confirmed live via `docker inspect tamga-egress-proxy
--format '{{json .NetworkSettings.Networks}}'` → `{"bridge":
{"NetworkID":"2bea86b4...", "IPAddress":"172.17.0.2", "Gateway":
"172.17.0.1", "MacAddress":"56:a4:...", ...}}`. This is a usable,
already-available *alternate* edge-derivation source (see §2 Edges).

**Stats** — `ContainerStats` (client.go:235-247) wraps
`cli.ContainerStats(ctx, id, stream=false)` (one-shot, not a stream),
decodes into the upstream `container.Stats` struct and returns it
raw. The container detail page's Stats tab consumes this via `GET
/system/containers/{id}/stats` (router.go:73, handler `Stats`,
container_handler.go:121-182), which does the CPU%/mem%/net delta math
server-side and returns a flat `{cpu:{percent,usage,system,percpu[]},
mem:{usage,limit,percent}, net:{rx_bytes,tx_bytes,rx_packets,
tx_packets}}` (frontend type `ContainerStats`, api.ts:145-149). Confirmed
live via `docker stats --no-stream tamga-backend-1 tamga-caddy-1` — the
CPU%/MEM-usage/MEM-limit/NET-I/O columns match what the handler computes.
This is the exact capability the map's live resource overlay reuses —
nothing new needed for per-node CPU/mem, only the fan-out cost (see gap
B below).

**Networks** — partial only. `NetworkExists` (280-289, existence check,
discards the inspect result beyond a bool), `EnsureNetwork` (295-311,
create-if-missing), `NetworkConnect`/`NetworkDisconnect` (315-331,
idempotent attach/detach), `NetworkRemove` (334-340). All five are
mutation/existence-oriented and used by container provisioning
(`agent_service.go`, `project_service.go`) — **none list networks, and
none return which containers are attached to a network.** No
`ListNetworks`/`NetworkContainers` method exists on `Client`, and no
`/system/networks` route exists in the route table
(router.go:65-76 — confirmed, the full `/system/*` block was read
end-to-end). This is the load-bearing gap for the map's edges — see gap
A below.

**Logs** — `ContainerLogs`/`ContainerLogsSince` (client.go:101-120),
`tail`+`since` params, demuxed via `stdcopy.StdCopy` into one combined
string. Exposed at `GET /system/containers/{id}/logs?tail=N`
(router.go:72, handler:102-119). Not needed by the map itself (no Scope
item calls for it), noted only for completeness.

**Also present, not map-relevant per se**: `DockerInfo` (463-469, daemon
totals: running/stopped/paused/images counts, memory/CPU of the host) —
exposed at `GET /system/info` (router.go:76); useful for a map's overall
"N containers, N running" header stat but not per-node data.

**"Have vs missing" summary:**

| Capability | Have today | Gap |
|---|---|---|
| List all containers (nodes) | Yes — `ListContainers`, bulk, single call, includes stopped | project_id/system_type is a name-parse heuristic (Sscanf), not a stored association (same gap TEST-008 §4 already flagged) — will need to become network-membership-based once TEST-011's per-project networks land (see §2 Nodes) |
| Per-container inspect | Yes — full `types.ContainerJSON`, includes `NetworkSettings.Networks` | none |
| Per-container stats (CPU/mem/net) | Yes — one call per container, already used by the Stats tab | **B: no bulk/aggregate stats call** — Docker's Engine API itself has no "stats for all containers in one call" endpoint (upstream limitation, not a Tamga gap); a map with N nodes fans out N stats requests per refresh tick |
| List networks | **No** | **A: no `Client.ListNetworks`, no `/system/networks` route.** The underlying SDK already imported (`"github.com/docker/docker/client"`, client.go:18) supports both `cli.NetworkList(ctx, network.ListOptions{})` and `cli.NetworkInspect(ctx, name, network.InspectOptions{Verbose: true})` (the latter's response includes a `Containers` map — confirmed live, see Test Plan #6); this is a thin-wrapper gap, not an SDK gap. Straightforward to add: two small `Client` methods + one handler + one route. |
| Network → attached containers (edge source) | **No dedicated capability**, but derivable two ways today without new SDK surface: (a) new networks-list wrapper (above), one call per network, O(networks); or (b) `InspectContainer` per node and read `NetworkSettings.Networks` client-side, O(nodes), zero new backend code but N inspect calls | Recommend (a) — see §2 Edges |
| Image → node-type classification | **No** — `ContainerInfo.Image` passed through raw (`caddy:alpine`, `tamga-backend`, etc.), nothing maps it to a semantic type anywhere in the backend | New, small — see §2 Image classification (pure phase-2 addition, this audit only proposes the mapping) |
| Topology/graph concept | **No** — grep confirms no existing endpoint, struct, or handler resembling nodes/edges anywhere in the backend | Phase 2, see §4 |

### 2. Topology derivation — design

**Nodes.** `ListContainers` (`All: true`) already returns every container
Docker knows about in one bulk call — both the global map and a
per-project map read from the *same* underlying data, just filtered
differently; no new list capability needed for nodes themselves.
- Global map: every container returned by `ListContainers` — core stack
  (today: `tamga-caddy-1`/`tamga-backend-1`/`tamga-frontend-1`/
  `tamga-egress-proxy`, classified via the existing `system_type`
  tamga-prefix heuristic; Traefik will join this set once TEST-010 lands,
  same heuristic applies to whatever its container is named), agent
  sandboxes (`agent-<id>`/`agent-system`), and deployed project
  containers (`project-<id>`, soon possibly multi-service per TEST-011
  §2c).
- Per-project map: the same `ListContainers` response filtered to
  `project_id == X` (today) plus that project's `agent-<id>` sandbox if
  present.
- Attribution — today vs after TEST-011: today, purely a name-parse
  heuristic (`fmt.Sscanf`, client.go:196-206) — already flagged as a gap
  by TEST-008 §4. TEST-011 §2c proposes moving deployed projects to
  **one network per project** (`project-net-<id>`), mirroring the
  per-sandbox-network precedent that already exists for agents
  (`agentNetworkName`, `agent_service.go:162-167`, confirmed via read).
  Once that lands, a multi-service project's per-service container names
  won't all match a single `project-<id>` pattern any more (TEST-011
  itself doesn't pin the final per-service naming down beyond
  "`<project-name-or-"app">`"), so **name-parsing stops being reliable and
  network-membership becomes the correct, durable attribution
  mechanism**: "every container attached to `project-net-<id>` belongs to
  project `id`" — which is exactly the same network data the edges need
  (§ below), i.e. one fetch answers both "which project owns this node"
  and "who's connected to whom." Recommend the map's attribution logic be
  written against network membership from day one rather than the name
  heuristic, so it doesn't need rework when TEST-011 lands.

**Edges.** Shared docker network = connection, confirmed both
conceptually and against the live stack:
- Live evidence (Test Plan #6): `docker network inspect
  tamga_tamga-network` returns a `.Containers` map (keyed by container
  ID, each value `{Name, EndpointID, MacAddress, IPv4Address,
  IPv6Address}`) holding `tamga-caddy-1`+`tamga-backend-1`+
  `tamga-frontend-1` — exactly the node-to-edge join data the map needs.
  `docker network inspect tamga-net` returns the same shape but an empty
  `Containers` map right now (no project deployed in this session).
  Confirmed live that BUG-028 is still unfixed: caddy/backend/frontend
  sit on `tamga_tamga-network` (compose-created), never on `tamga-net`
  (the network `EnsureNetwork`/deploy code targets) — the two are
  disjoint today, unchanged from TEST-010/TEST-011's findings. A topology
  map built on this data would render that disjointness directly as two
  unconnected components, which is a correct rendering of a real,
  already-tracked defect (BUG-028), not a map bug — worth flagging to
  whoever builds phase 2 so it isn't mistaken for one.
- Docker client capability: as noted in §1 gap A, no wrapper method
  exists yet, but the underlying SDK (already imported) supports it two
  ways: `cli.NetworkList` + `cli.NetworkInspect(..., Verbose: true)`
  (O(networks) calls, returns `.Containers` directly — recommended), or
  reading `NetworkSettings.Networks` off each node's existing
  `InspectContainer` result (O(nodes) calls, zero new docker-client code,
  fallback if the new wrapper methods are deprioritized). Recommend
  adding two small `Client` methods (`ListNetworks`,
  `NetworkContainers(name)`) plus one new handler/route — small, additive,
  no changes to any existing method.

**Image classification.** Proposed image-name → node type/icon mapping
(simple substring lookup against `ContainerInfo.Image`, ordered,
first-match, case-insensitive), since nothing like this exists in the
backend today (§1):

| Image substring match | Node type |
|---|---|
| `redis` | cache |
| `postgres`/`postgresql` | database (postgres) |
| `mysql`/`mariadb` | database (mysql) |
| `mongo` | database (mongo) |
| `caddy`/`traefik`/`nginx` (when not the Tamga core proxy itself, i.e. not `system_type`-classified) | proxy |
| `node`/`python`/`golang`/`ruby`/`php` (generic language runtime base images used by a project's own `Dockerfile`) | web/app |
| `rabbitmq`/`kafka` | queue |
| everything else | generic |

The Tamga core-stack containers should classify by `system_type`/name
first (caddy, backend, frontend, egress-proxy, traefik, agent-system are
all already known, structural nodes — no image-sniffing needed for them),
falling back to this image-substring table only for `project-<id>`/
project-service containers whose image is user/build-defined.

**Live status + resource overlay.** `ContainerInfo.State`
(client.go:168, sourced straight from Docker's own container state
string — running/exited/paused/restarting/etc.) is already returned in
the same bulk `ListContainers` call used for nodes — **status coloring
needs zero extra calls, it's already in the node-list response.**
CPU/mem/net needs `ContainerStats` per container (§1) — expensive
relative to list (one Docker Engine API round-trip per container, doing
a two-sample delta internally). Recommendation: two-tier refresh —
poll `ListContainers` (cheap, single bulk call) frequently for node
presence/status color, e.g. every 5s; poll stats less frequently and/or
only for nodes currently rendered/visible, e.g. every 10-15s. If more
than one map view can be open at once (global map + a per-project map in
another tab, both wanting current stats), recommend a small new backend
aggregate endpoint that snapshots all container stats once per tick
server-side and serves cached results to every poller — avoids N×M
redundant Docker Engine calls (N containers × M open map views). For a
single map view, polling the existing per-container stats endpoint is
sufficient — no new endpoint strictly required, just fan-out from the
frontend (or a thin backend loop) over the already-listed node IDs.

### 3. Traffic overlay data seam

No analytics/metrics code exists in the backend yet (confirmed —
`find backend -iname "*analytic*" -o -iname "*metric*" -o -iname
"*traefik*"` returns nothing); TEST-010 documents the *source* shape
only (Traefik Prometheus `/metrics`), not a Tamga-side store, so this is
a design seam, not an integration to verify.

The seam: TEST-010 §4 confirms per-project Traefik metrics are keyed by
`router`/`service` label == `project-<id>` (after stripping the
`@<provider>` suffix Traefik appends, e.g. `project-3@file` → `project-3`)
— e.g. `traefik_router_requests_total{router="project-3", service=
"project-3", code, method}`, plus the equivalent `_service_*`,
duration-histogram, and byte-count series. The infra graph's nodes
already carry `project_id` (§1 — `ContainerInfo.project_id`), which is
the *same* identifier Traefik's metric labels encode as `project-<id>`.
So the join is: **for any node with a non-zero `project_id`, the overlay
queries the (future) metrics store for series where
`router==service=="project-<id>"`**, and applies the resulting
request-rate/error-rate/latency to that node's edge thickness/color.
Nodes without a `project_id` (core stack containers, agent sandboxes)
have no traffic overlay — Traefik only proxies project domains, it never
routes to those containers, so there's no metric series to join against
regardless. No new topology-side data is needed for this seam beyond
what §1/§2 already produce; the join key (`project_id` string-formatted
as `project-<id>`) is already present on every project node today.

### 4. API shape proposal

Following the existing route convention (`/system/*` for global-scope,
`/projects/{id}/*` for per-project — router.go:50-76), propose:

```
GET /api/system/topology            -- global infra graph
GET /api/projects/{id}/topology     -- per-project infra graph
```

Both return the same shape, the per-project variant pre-filtered
server-side to that project's nodes/edges:

```json
{
  "nodes": [
    {
      "id": "e8232c147bab...",
      "name": "tamga-caddy-1",
      "image": "caddy:alpine",
      "type": "proxy",
      "project_id": 0,
      "system_type": "tamga-caddy-1",
      "state": "running",
      "status": "Up 3 hours",
      "stats_ref": "/api/system/containers/e8232c147bab.../stats"
    },
    {
      "id": "...",
      "name": "project-3",
      "image": "tamga-project-3",
      "type": "web",
      "project_id": 3,
      "system_type": "",
      "state": "running",
      "status": "Up 2 hours",
      "stats_ref": "/api/system/containers/.../stats",
      "traffic_ref": "project-3"
    }
  ],
  "edges": [
    { "network": "tamga_tamga-network", "source": "tamga-caddy-1", "target": "tamga-backend-1" },
    { "network": "tamga_tamga-network", "source": "tamga-caddy-1", "target": "tamga-frontend-1" },
    { "network": "project-net-3", "source": "project-3", "target": "agent-3" }
  ]
}
```

Notes on the shape:
- `nodes[].id`/`name`/`image`/`state`/`status`/`project_id`/`system_type`
  are exactly `ContainerInfo` as it exists today (§1) — no reshaping of
  existing fields, only additive: `type` (new, §2 image classification),
  `stats_ref` (a pointer back to the existing per-container stats
  endpoint rather than embedding live stats in the graph payload itself,
  so the graph call stays cheap/bulk and stats stay on their own
  independently-pollable cadence per §2's two-tier recommendation),
  `traffic_ref` (present only when `project_id != 0`; the `project-<id>`
  string the frontend uses to query the metrics/analytics endpoint for
  the traffic overlay, per §3's seam — kept as an explicit ref rather
  than requiring the frontend to re-derive `"project-" + project_id`
  itself).
- `edges[]` is a flat list of `{network, source, target}` triples derived
  from the network→containers data in §2 (one edge per pair of containers
  sharing a network, `network` naming which one so the frontend can
  group/label edges); built server-side from the new `ListNetworks`/
  `NetworkContainers` capability (§1 gap A) rather than shipped as raw
  Docker network objects, since the graph only needs "who's connected to
  whom via what," not full network IPAM/driver detail.
- Global vs per-project is purely a server-side filter over the same
  underlying node/edge computation — no separate code path, so both
  variants stay consistent by construction.
- This is a genuinely new aggregate endpoint (§1 gap A/E — no topology
  concept exists today), not something layered onto an existing route;
  it does not replace `/system/containers` or the per-container
  inspect/stats/logs routes, which stay as-is for the container detail
  page.

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

### 2026-07-10 — reviewer pass

**Verdict: PASS**

Cross-checked every load-bearing claim in the Implementation Notes against
live source and the running stack; nothing found inaccurate.

- **Capability citations (§1):** `client.go` line ranges verified against
  the actual file — `ListContainers` (176-225), `InspectContainer`
  (227-233), `ContainerStats` (235-247), `NetworkExists` (280-289),
  `EnsureNetwork` (295-311), `NetworkConnect`/`NetworkDisconnect`
  (315-331), `NetworkRemove` (334-340) all match exactly, including field
  names/derivation logic in `ListContainers` (196-209, name-prefix
  heuristic for `project_id`/`system_type`). `container_handler.go`
  handler line ranges (List 29-39, Inspect 41-52, Logs 102-119, Stats
  121-182, Info 254-278) match. `frontend/src/lib/api.ts:132-165`
  (`ContainerInfo`/`ContainerStats`/`DockerInfo` types) mirrors the Go
  structs 1:1 as claimed.
- **Network-listing gap confirmed load-bearing and accurate:**
  `grep -n "NetworkInspect\|NetworkList" client.go` returns exactly one
  hit, `client.go:281` inside `NetworkExists`, which discards the result
  beyond a bool — confirmed no `ListNetworks`/`NetworkContainers` method
  exists on `Client`. `grep -n "container\|network" router.go` (lines
  65-76) confirms no `/system/networks` route exists. `go doc
  github.com/docker/docker/client.Client` confirms the imported SDK
  already exposes both `NetworkList(ctx, network.ListOptions)
  ([]network.Summary, error)` and `NetworkInspect(ctx, id,
  network.InspectOptions) (network.Inspect, error)` — so the audit's
  "thin wrapper, not an SDK gap" characterization is accurate.
- **Edge-derivation, both paths confirmed:** `go doc
  network.Inspect` shows `Containers map[string]EndpointResource`
  exactly as claimed, and `go doc network.EndpointResource` confirms the
  `Name`/`EndpointID`/`MacAddress`/`IPv4Address`/`IPv6Address` field set
  the audit cites. `InspectContainer`'s return type
  (`types.ContainerJSON` = `container.InspectResponse`) does carry
  `NetworkSettings.Networks`, confirming the zero-new-code fallback claim.
  Live-reran (read-only) both evidence commands: `docker network inspect
  tamga_tamga-network --format '{{json .Containers}}'` returns exactly the
  three-container map (caddy/backend/frontend) with the field shape
  claimed; `docker network inspect tamga-net` returns an empty
  `{}` — independently reconfirming BUG-028's disjoint-network state is
  still live, matching the audit's finding word-for-word. `docker inspect
  tamga-egress-proxy --format '{{json .NetworkSettings.Networks}}'` also
  matches the cited shape exactly.
- **Design soundness:** the `/api/system/topology` +
  `/api/projects/{id}/topology` shape follows the existing
  `/system/*`/`/projects/{id}/*` route convention (router.go:50-76).
  `stats_ref`/`traffic_ref` as pointers rather than embedded values is the
  right call for a pollable graph endpoint — keeps the bulk graph call
  cheap and lets stats/traffic ride their own independently-tunable
  polling cadence, consistent with the two-tier refresh recommendation in
  the same section. The traffic-overlay join (`project_id` →
  `project-<id>` Traefik router/service label) is consistent with
  TEST-010's independently-confirmed metric labels
  (`tasks/done/TEST-010-reverse-proxy-traefik-migration-audit.md:598-599`).
  No existing topology/graph concept found anywhere in the backend or
  frontend (`grep -rl "topology"` empty) — this is a genuinely new
  concept, not duplicating anything.
- **Completeness/scope:** every Acceptance Criteria item is plausibly
  addressed in the Implementation Notes (capability table, edge
  derivation, classification mapping, API shape, traffic seam, refresh
  cadence). No new BUG filed — correctly reasoned, since BUG-028/029 are
  reconfirmed still `status: pending` in `tasks/active/`, not duplicated.
  `git status --porcelain -- backend/` and `git diff --stat -- backend/`
  are both empty — confirmed no production code changed, consistent with
  the "findings only" affected-areas claim. The Test Approach/Test Plan
  read like they were genuinely executed against the live stack (the
  `nodes[].id` example value in §4, `e8232c147bab...`, is the real
  container-ID prefix for `tamga-caddy-1` on this box) rather than
  fabricated.

Non-blocking nitpicks:
- A couple of §1/§2 line-range citations are off by a line or two
  relative to the literal source (e.g. "196-210"/"196-206" for the
  name-parse block, which actually runs 196-209) — trivial, doesn't
  change any conclusion.
- Acceptance Criteria checkboxes are left unchecked (`- [ ]`) in the
  markdown; this matches the established convention for prior `done`
  audits (TEST-008/010/011 all leave theirs unchecked too), so not
  treated as a gap here.

## Test Notes

### 2026-07-10 — architect (test-stage verification, direct)

Documentation/design audit, no runtime surface — the reviewer independently
verified the load-bearing claims against live source + the running stack:
the network-listing gap (only NetworkExists uses NetworkInspect, discards
the result; no /system/networks route; SDK NetworkList/NetworkInspect are
available → thin-wrapper gap), the edge-derivation `.Containers` shape
(live `docker network inspect tamga_tamga-network`), and the zero-new-code
fallback via InspectContainer→NetworkSettings.Networks. The `/topology` API
shape and the project_id→`project-<id>` Traefik-label overlay join are
consistent with TEST-010. Verdict: PASS. This closes SPRINT-004 phase 1 —
all three audits (TEST-010/011/012) done; phase 2 is planned from their
combined findings.

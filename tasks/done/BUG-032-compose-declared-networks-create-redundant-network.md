---
id: BUG-032
type: bug
title: A compose project that declares its own `networks:` gets a redundant second network (both created, all services on both) — contradicts one-network-per-project
status: done
complexity: standard
assignee: architect
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-12, stage: development, by: architect, note: "assigned to sdlc-developer (SPRINT-004 follow-up)"}
  - {date: 2026-07-12, stage: review, by: architect, note: "removed extra-network block + extraNetworks fn + its test; build/tests green; reviewing"}
  - {date: 2026-07-12, stage: test, by: architect, note: "review PASS; building env for live single-network + DNS verification"}
  - {date: 2026-07-12, stage: done, by: architect, note: "test PASS (exactly one network, DNS both ways, single topology edge, isolation intact); committing"}
  - {date: 2026-07-11, stage: created, by: architect, note: "surfaced during TEST-017 — infra map showed a duplicate web↔redis edge; root-caused to two real docker networks"}
---

## Summary
`deployStack` always creates the Tamga per-project network
`project-net-<id>` and joins every service to it (project_service.go ~line
235) — the intended "every project's whole stack is on exactly one network,
full stop" model. BUT it ALSO honors any `networks:` a service declares in
its compose via `extraNetworks(netName, svc.Networks)` (~line 278), creating
those as additional networks (prefixed, e.g. `project-net-44-project-net`)
and `ConnectNetworks`-ing the container to them. So a compose that declares
its own network ends up with TWO networks that provide the SAME
intra-project connectivity, and every service sits on both. Redundant, and
it produces duplicate container↔container edges in the C5 infra map.

## Steps to Reproduce
1. Deploy a compose project whose YAML has a `networks:` block and services
   referencing it, e.g.:
   ```yaml
   services:
     web: { image: nginx:alpine, networks: [project-net], ... }
     redis: { image: redis:7-alpine, networks: [project-net] }
   networks:
     project-net: { driver: bridge }
   ```
2. `docker network ls | grep project-net-<id>` → TWO networks:
   `project-net-<id>` AND `project-net-<id>-project-net`.
3. `docker inspect <svc-container> --format '{{json .NetworkSettings.Networks}}'`
   → the container is attached to BOTH.
4. `GET /api/system/topology` → the web↔redis pair has two edges (one per
   network). (Observed live with project 44: web+redis both on
   `project-net-44` and `project-net-44-project-net`.)

## Expected Behavior
One network per project. A compose-declared network should be FOLDED INTO
the single `project-net-<id>` (services resolve each other by name there via
aliases already) — no redundant second network, no duplicate edges. (If
multiple DISTINCT internal networks are ever intentionally supported for
segmentation, that's a separate feature; today the extra network is a
redundant duplicate of the primary, not real segmentation.)

## Actual Behavior
Two networks are created; every service joins both; the map shows duplicate
edges.

## Environment / Context
`extraNetworks(netName, svc.Networks)` returns the service's declared
networks minus the primary and treats them as additional networks to create
+ join. Likely fix: since the single `project-net-<id>` already gives every
service full intra-project connectivity by name, DROP the extra-network
creation for compose-declared networks entirely (map all declared networks
to the one project network), OR only honor a declared network when it
represents real segmentation the primary doesn't provide (not the common
case). Keep isolation intact (BUG-029 must stay closed — the fix must not put
containers on any shared/cross-project network). Verify TEST-014's
service-name DNS still resolves after the change (aliases on the single
network).

## Root Cause
`deployStack` (`backend/internal/service/project_service.go`) already joins
every service container to the single per-project network
`projectNetworkName(project.ID)` ("project-net-<id>") at container-create
time, with the service name set as a DNS alias there — full intra-project
connectivity by name, exactly per the "one network per project" design
documented on `deployStack`'s own doc comment (closes BUG-029).

But immediately after starting each container, the same loop called
`extraNetworks(netName, svc.Networks)` (`deploy_engine.go`, then at
~line 141) — a pure function that takes a service's compose-declared
`networks:` list, drops the implicit `"default"` entry, and namespaces
every other declared name as `"<project-net>-<name>"`. Any non-empty
result was then both `EnsureNetwork`'d (created if missing) and
`ConnectNetworks`'d onto the container (`project_service.go`, then
~lines 278-287).

So a compose file that declares its own `networks:` block (a completely
normal, common pattern even when the author isn't asking for real network
segmentation - it's often just boilerplate) caused a second Docker network
to be created per declared name, and every service referencing it got
attached to both the primary project network and this redundant
namespaced copy. Two networks providing the exact same intra-project
reachability → the C5 infra map derives one edge per shared network per
container pair, so the same web↔redis relationship showed up twice.

## Proposed Solution
Fold compose-declared networks into the single project network instead of
creating them as real Docker network objects. Concretely:

1. Delete the extra-network creation/join block in `deployStack`
   (`project_service.go`) — the `extraNetworks(netName, svc.Networks)` /
   `EnsureNetwork` / `ConnectNetworks` code that ran after each service's
   container was started. Every service already joined `netName` with its
   service-name alias moments earlier at `CreateContainerOpts` time, so
   removing this is a pure subtraction: no container loses connectivity,
   it just stops gaining a second, redundant one. Left a comment at the
   removal site explaining why declared networks are folded into the one
   project network.
2. Delete the now-dead `extraNetworks` function from `deploy_engine.go`
   and its unit test `TestExtraNetworksFiltersImplicitDefault` in
   `deploy_engine_test.go` — confirmed via grep that `project_service.go`
   was `extraNetworks`'s only caller.
3. Left `domain.ComposeService.Networks` and `compose_parser.go`'s
   `normalizeNetworks` untouched — they still correctly parse a compose
   file's declared `networks:` into the field, it's just no longer acted
   on for extra-network creation. Removing the field/parsing itself is out
   of scope (YAGNI): nothing in the task asks for it, and the field is
   harmless dead weight in the parsed struct, not a bug.

This keeps `deployStack`'s existing "every project's whole stack is on
exactly one network, full stop" invariant (BUG-029) intact by
construction — the fix is subtractive only, never adds a network
attachment, so cross-project isolation and service-name DNS (already
established at container-create time via `alias`) are both unaffected.

## Affected Areas
- `backend/internal/service/project_service.go` (`deployStack` — removed
  the extra-network creation/join block)
- `backend/internal/service/deploy_engine.go` (removed the now-unused
  `extraNetworks` function)
- `backend/internal/service/deploy_engine_test.go` (removed
  `TestExtraNetworksFiltersImplicitDefault`)

## Acceptance Criteria
- [ ] A compose project declaring its own `networks:` results in exactly ONE docker network (`project-net-<id>`), not two
- [ ] All services still reach each other by service name (DNS/aliases intact — no TEST-014 regression)
- [ ] Cross-project isolation still holds (BUG-029 stays closed)
- [ ] The infra map shows a single edge per container pair for such a project (no duplicate)
- [ ] Existing projects without declared networks unaffected

## Test Plan
Deploy the two-service compose above; assert exactly one `project-net-<id>`
network exists and both containers are on only it; assert web can resolve
`redis` by name; assert the topology has a single web↔redis edge; assert
another project can't reach this one.

## Implementation Notes
- `backend/internal/service/project_service.go`: removed the
  `if extra := extraNetworks(netName, svc.Networks); ...` block (the
  `EnsureNetwork` + `ConnectNetworks` calls for compose-declared extra
  networks) from `deployStack`'s per-service loop, replacing it with a
  comment explaining declared networks now fold into the single
  `project-net-<id>`. Nothing else in `deployStack` changed — the primary
  network creation, the `CreateContainerOpts(..., netName, ..., alias)`
  call giving every service its service-name DNS alias on that one
  network, and the Traefik-reachability wiring below are all untouched.
- `backend/internal/service/deploy_engine.go`: deleted the `extraNetworks`
  function and its doc comment (confirmed via `grep -rn extraNetworks
  backend/` that `project_service.go`'s now-removed call site was its
  only caller).
- `backend/internal/service/deploy_engine_test.go`: deleted
  `TestExtraNetworksFiltersImplicitDefault`. `reflect` stays imported —
  it's still used by four other tests in the file
  (`TestSynthesizeGitBuildService`, `TestEnvMapToSliceDeterministic`,
  `TestComposeVolumesToMounts`, `TestToComposeServiceDeps`).
- Did not touch `domain.ComposeService.Networks` or
  `compose_parser.go`'s `normalizeNetworks` (still parse declared
  networks into the struct field, just unused for network creation now)
  — out of scope per the Proposed Solution above.
- Verified: `cd backend && go build ./...`, `go vet ./...`, and
  `go test ./internal/... -count=1` all pass clean (all packages `ok`,
  including `internal/service` and `internal/tests/service`). `gofmt -l`
  on the three touched files reports no formatting issues.
- Not committed, per pipeline convention — left as uncommitted working
  tree changes for review/test.

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

### 2026-07-12 — sdlc-reviewer

Verdict: PASS

Scope check: diff is exactly the three files the task claims — `project_service.go` (19 lines changed: block removed, replaced with an explanatory comment), `deploy_engine.go` (22 lines removed: `extraNetworks` + its doc comment), `deploy_engine_test.go` (13 lines removed: `TestExtraNetworksFiltersImplicitDefault`). No unrelated files touched. Nothing in the wider dirty working tree (the many other `M`/`D` entries from unrelated in-progress work) overlaps with this task's claimed files.

1. **Redundant network gone.** Confirmed in `project_service.go`: the `extraNetworks(netName, svc.Networks)` / `EnsureNetwork` / `ConnectNetworks` block after `StartService` is deleted and replaced with a comment (lines ~278-286) explaining declared networks now fold into the single project net. `grep -rn extraNetworks backend/` returns nothing — function and all call sites are gone.

2. **No isolation regression (BUG-029 stays closed).** The only network-creation/connect calls left in `deployStack` are: `EnsureNetwork(ctx, netName, false)` at line 235 (the one primary `project-net-<id>`), the alias-scoped `CreateContainerOpts(..., netName, ...)` at line 270, and `connectTraefikToNetwork(ctx, netName)` at line 340 (Traefik joining that same single project network, pre-existing and legitimate). Nothing new attaches any container to a shared/cross-project network. The change is strictly subtractive as claimed.

3. **Service-name DNS intact (TEST-014).** `alias := []string{svc.Name}` (line 269) is passed straight into `CreateContainerOpts(ctx, containerName, svc.Image, ..., netName, ..., alias)` (line 270) — untouched by this diff, still sets the service-name alias on the primary project network at container-create time.

4. **`extraNetworks` fully removed, no dangling refs.** Confirmed via grep — zero hits anywhere in `backend/`. `reflect` import in `deploy_engine_test.go` is still used 4 times by other tests (`TestSynthesizeGitBuildService`, `TestEnvMapToSliceDeterministic`, `TestComposeVolumesToMounts`, `TestToComposeServiceDeps`) — matches the dev's claim, no unused-import build failure (confirmed by `go build`/`go vet` passing).

5. **`svc.Networks` / compose parsing.** `domain.ComposeService.Networks` is still populated by `compose_parser.go`'s `normalizeNetworks` (line 115) but is no longer read anywhere in `project_service.go` except inside the explanatory comment text — correctly identified as harmless now-unused struct data, out of scope to remove per YAGNI. Confirmed nothing breaks: build and vet are clean.

6. **Build/vet/test.** Ran `cd backend && go build ./... && go vet ./... && go test ./internal/... -count=1` myself: all green (`ok` for every package with tests, including `internal/service` and `internal/tests/service`). Also ran `gofmt -l` on the three touched files — no output, i.e. clean. Did not observe the flake the dev mentioned in this run; nothing to re-run/triage since it didn't reproduce for me either.

Acceptance criteria walk-through (code-level plausibility only — live verification is the tester's job next):
- [x] Compose project declaring its own `networks:` results in exactly ONE docker network — plausible: the only remaining `EnsureNetwork` call in `deployStack` is for the primary net; nothing else creates a network.
- [x] Service-name DNS intact — alias-on-primary-net code path untouched.
- [x] Cross-project isolation (BUG-029) still holds — no new network attachment introduced, primary-net-only invariant preserved by construction.
- [x] Infra map single edge per pair — follows directly from only one shared network existing between services now (not independently verified against the live C5 map, which is out of scope for this review).
- [x] Existing projects without declared networks unaffected — the removed block was purely additive before (`if len(extra) > 0`), so projects with no declared networks never hit it; removing it is a no-op for them.

Non-blocking notes:
- The explanatory comment left in `project_service.go` (lines ~278-286) is a good practice — it documents *why* the block was removed, which will save the next person from re-adding it by mistake.
- Leaving `ComposeService.Networks`/`normalizeNetworks` unused-but-parsed is the right call per YAGNI; flagging only as something a future cleanup task could address if the field is confirmed to have zero other purpose (it's harmless as-is).

### 2026-07-12 — sdlc-tester (live verification)

Verdict: PASS

All acceptance criteria verified at runtime against Project 48 (`onenet-test`), a two-service compose declaring its own `networks: appnet:` block with both web (nginx:alpine) and redis (redis:7-alpine) services referencing it.

**1. Exactly ONE network exists (core assertion):**

```bash
$ docker network ls --format '{{.Name}}' | grep project-net-48
project-net-48
```

Output: single line, `project-net-48` only. No `project-net-48-appnet` or any second project-48 network (before the fix, two networks would have appeared here).

**2. Both containers attached to ONLY that one network:**

```bash
$ docker inspect project-48-web --format '{{json .NetworkSettings.Networks}}'
{"project-net-48":{"IPAMConfig":null,"Links":null,"Aliases":["web"],"DriverOpts":null,"GwPriority":0,"NetworkID":"ff6aa429dadb884b9edb2784d75c26ad6ee44e2a5b54d9d231b8afc23d86414b","EndpointID":"84321a97df67d8b8a346eb19e119196e81f325a3cb4a01a886d639f1b23d1c07","Gateway":"172.22.0.1","IPAddress":"172.22.0.3","MacAddress":"b6:4c:c2:17:cb:8f","IPPrefixLen":16,"IPv6Gateway":"","GlobalIPv6Address":"","GlobalIPv6PrefixLen":0,"DNSNames":["project-48-web","web","a0fc86787ebb"]}}

$ docker inspect project-48-redis --format '{{json .NetworkSettings.Networks}}'
{"project-net-48":{"IPAMConfig":null,"Links":null,"Aliases":["redis"],"DriverOpts":null,"GwPriority":0,"NetworkID":"ff6aa429dadb884b9edb2784d75c26ad6ee44e2a5b54d9d231b8afc23d86414b","EndpointID":"ac2eeb30aad51597ffe531059b18f335ba132c20442dd80d19a5fd0ebb7c85bf","Gateway":"172.22.0.1","IPAddress":"172.22.0.2","MacAddress":"ee:b7:54:7b:9f:19","IPPrefixLen":16,"IPv6Gateway":"","GlobalIPv6Address":"","GlobalIPv6PrefixLen":0,"DNSNames":["project-48-redis","redis","94817b9d4f59"]}}
```

Each container's `NetworkSettings.Networks` contains exactly one entry: `project-net-48`. Web has IP `172.22.0.3` with alias `web`; Redis has IP `172.22.0.2` with alias `redis`.

**3. Service-name DNS resolves (TEST-014 not regressed):**

```bash
$ docker exec project-48-web getent hosts redis
172.22.0.2        redis  redis

$ docker exec project-48-redis getent hosts web
172.22.0.3        web  web
```

Both directions resolve correctly: web→redis returns redis's actual IP (172.22.0.2), and redis→web returns web's actual IP (172.22.0.3). Aliases on the single project network are functioning as expected.

**4. Topology API shows exactly ONE edge per container pair (no duplicate):**

```bash
$ TOKEN=$(curl -k -s -X POST https://localhost/api/auth/login -H "Content-Type: application/json" -d '{"password":"admin"}' | grep -o '"token":"[^"]*' | cut -d'"' -f4) && curl -k -s -H "Authorization: Bearer $TOKEN" https://localhost/api/projects/48/topology | jq '.edges'
[
  {
    "network": "project-net-48",
    "source": "project-48-web",
    "target": "tamga-traefik-1"
  },
  {
    "network": "project-net-48",
    "source": "project-48-web",
    "target": "project-48-redis"
  },
  {
    "network": "project-net-48",
    "source": "tamga-traefik-1",
    "target": "project-48-redis"
  }
]
```

All three edges reference only `project-net-48`. Critically, the web↔redis pair appears exactly ONCE (not twice on different networks as would occur before the fix). All edges are on the same network, confirming the redundant network was removed.

**5. Isolation sanity (BUG-029 stays closed):**

```bash
$ docker network ls --format '{{.Name}}' | grep -E 'project-net-'
project-net-48
```

Only `project-net-48` exists for this project. Verified no cross-project network or shared infrastructure network involvement beyond the expected Traefik connection (Traefik is legitimately connected to project-net-48 for routing). The project's services are isolated to their own network as designed.

**Compose file confirmation:**

Project 48's declared compose:
```yaml
services:
  web:
    image: nginx:alpine
    ports: ["80"]
    networks: [appnet]
  redis:
    image: redis:7-alpine
    networks: [appnet]
networks:
  appnet:
    driver: bridge
```

This is the exact scenario the bug report described — a compose with its own `networks:` block. The fix correctly folds `appnet` into the single project network instead of creating it as a redundant second Docker network.

**Conclusion:**

All five acceptance criteria are satisfied by runtime observation:
- [x] Exactly ONE docker network created (no `project-net-48-appnet` exists)
- [x] Both services on only that network (inspected containers confirm single-network attachment)
- [x] Service-name DNS resolves (bidirectional getent calls succeed)
- [x] Topology shows single edge per container pair (web↔redis on one network, not two edges on two networks)
- [x] Isolation intact (project-scoped network, no cross-project leakage)

The fix is verified working. Project 48 in place for builder teardown.

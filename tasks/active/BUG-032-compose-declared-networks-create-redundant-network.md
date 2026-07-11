---
id: BUG-032
type: bug
title: A compose project that declares its own `networks:` gets a redundant second network (both created, all services on both) — contradicts one-network-per-project
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-11
history:
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
<filled in by developer>

## Proposed Solution
<filled in by developer>

## Affected Areas
- `backend/internal/service/project_service.go` (deployStack extraNetworks handling)
- possibly `backend/internal/service/compose_parser.go` (how declared networks are surfaced)

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
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

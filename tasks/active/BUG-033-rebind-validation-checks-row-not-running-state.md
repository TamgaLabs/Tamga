---
id: BUG-033
type: bug
title: Rebind "no running container" check validates DB-row existence, not actual running state (409 misses a stopped-but-deployed service)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "surfaced during TEST-018 — rebinding to a stopped-but-deployed service is accepted (route → down container 502) instead of 409"}
---

## Summary
FEAT-040's domain rebind returns 409 "exposed service has no running
container to route to" when `resolveExposedUpstream` returns `ok=false` — but
that function returns `ok=true` as long as a `project_service_containers` ROW
exists for the service, regardless of whether the container is actually
RUNNING. So the 409 only fires for a service that has NO container row at all
(rare — every deployed compose service gets a row). Rebinding to a service
whose container exists but is STOPPED is accepted: the route is written to a
down upstream and Traefik 502s until it restarts. FEAT-040's acceptance
criterion ("a service with no running container gives a clear error") is thus
only partially met.

## Steps to Reproduce
1. Deploy a 2-service compose project (web exposed, web2 idle), domain bound.
2. `docker stop project-<id>-web2`.
3. `PUT /api/projects/<id>` `{"exposed_service":"web2"}` → returns 200 (accepted),
   route file upstream becomes `project-<id>-web2:80`, and curling the domain
   502s (upstream down) instead of the rebind being rejected with 409.

## Expected Behavior
Rebinding to a service whose container is not running returns a clear client
error (409) and does NOT move the route to the down container — OR, if we
decide deferring the route until the container comes back is acceptable, do it
deliberately and document it (don't silently write a 502-ing route). Pick one
behavior intentionally.

## Actual Behavior
The rebind is accepted and the route is moved to the stopped container (502).

## Environment / Context
Impact is low: the common rebind case (among running services) works; the
route self-heals when the container restarts (normal Traefik down-upstream
behavior); no data corruption. This is a UX/validation refinement, not a
correctness/isolation issue. Fix direction: make the running-state check
actually consult docker for the target container's state — but be careful,
`resolveExposedUpstream` is SHARED with `ReconcileRoutes` (boot-time route
rewrite); making it running-aware there could skip routes for
briefly-stopped services on boot. Prefer a SEPARATE running-state check in
the rebind validation path (Update) rather than changing
resolveExposedUpstream's contract, so ReconcileRoutes is unaffected.

## Root Cause
<filled in by developer>

## Proposed Solution
<filled in by developer>

## Affected Areas
- `backend/internal/service/project_service.go` (Update rebind validation; a
  running-state check distinct from resolveExposedUpstream)

## Acceptance Criteria
- [ ] Rebinding to a service whose container is not running is handled intentionally: either 409 (rejected, route unchanged) or a deliberate documented deferral — no silent 502-ing route
- [ ] The common case (rebind among running services) still works (no TEST-018 regression)
- [ ] `ReconcileRoutes` boot behavior unchanged (resolveExposedUpstream contract intact)
- [ ] Test covers the stopped-container rebind

## Test Plan
Deploy a 2-service project, stop the target service's container, rebind to it,
assert the chosen intentional behavior (409 + route unchanged, or documented
deferral); confirm rebinding among running services still moves the route.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

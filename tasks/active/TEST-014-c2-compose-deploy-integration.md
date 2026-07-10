---
id: TEST-014
type: test
title: C2 integration — unified compose deploy (multi-service, reachability, isolation, routing)
status: pending
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C2 cluster integration test"}
---

**Part of:** C2-compose-deploy
**Depends on:** FEAT-025, FEAT-026, FEAT-027, FEAT-028, FEAT-029

## Summary
The single integration test for cluster C2. Its impl sub-tasks are verified
statically in review; this verifies the combined multi-service deploy live
on the rebuilt Traefik stack before the cluster commits. Also the closing
verification for BUG-029 (per-project network isolation).

## Scope
- **Multi-service deploy:** create a compose project with ≥2 services (e.g.
  a web service with a published port + a `redis` service, `web` depends_on
  `redis`), using PREBUILT images (docker pull works in this sandbox; git
  clone does not — so use an image-based compose, not build:). Deploy
  succeeds; both service containers run; `project_service_containers` has a
  row per service; deploy respected depends_on order.
- **Inter-service reachability:** the web service can reach `redis` by
  service name on the project network (exec into the web container and
  connect to redis:6379, or an equivalent check).
- **Exposed-service routing:** the web service's domain routes through
  Traefik (curl the domain → the app, not 502); the internal redis has NO
  route. Traefik `/metrics` shows the `project-<id>` router.
- **BUG-029 — cross-project isolation:** deploy a SECOND compose project;
  confirm project A's container cannot reach project B's container by name/IP
  (the per-project networks isolate them) — the defect BUG-029 documented.
- **git-build-as-1-service:** confirm a legacy git-build project still
  deploys through the unified path (SANDBOX LIMIT: full git clone+build has
  no network here — verify as far as possible: the synthesize-1-service path
  is unit-tested in FEAT-028; if a git deploy can't run, verify the 1-service
  model another way and document the limit, don't fail the cluster on it).
- **Lifecycle:** stop/status/delete the multi-service project — all
  containers + the project network + the Traefik route + child rows removed.
- **UI:** the compose-create flow works in the browser (Playwright per the
  established recipe) and the detail page shows the N services.

## Out of Scope
- Analytics (C3/C4), the map (C5), domain-binding edit (C6).

## Test Approach
<filled in by developer/tester>

## Affected Areas
<none — integration verification>

## Acceptance Criteria
- [ ] A ≥2-service image-based compose project deploys; both containers run; child-table rows exist; depends_on order respected
- [ ] Inter-service reachability by name on the project network works
- [ ] Exposed service routes via Traefik (app, not 502); internal service has no route; project-<id> metrics present
- [ ] BUG-029 closed: project A cannot reach project B's containers (per-project network isolation), verified live
- [ ] Whole-stack stop/status/delete cleans containers + network + route + child rows
- [ ] Compose-create UI works + detail shows N services (browser)
- [ ] git-build-as-1-service verified as far as the sandbox allows, limit documented
- [ ] No orphaned resources after the test

## Test Plan
<filled in — deploy a web+redis compose via API/UI, exec-check redis
reachability, curl the domain, deploy a 2nd project and prove isolation,
exercise lifecycle, clean up>

## Implementation Notes
<n/a — test task>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

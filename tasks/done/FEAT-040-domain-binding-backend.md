---
id: FEAT-040
type: feature
title: Domain-binding backend — change a project's exposed service (+ domain) and move the Traefik route
status: done
complexity: standard
assignee: sdlc-reviewer
sprint: SPRINT-004
created: 2026-07-11
history:
  - {date: 2026-07-11, stage: created, by: architect, note: "SPRINT-004 C6 cluster (domain-binding edit) — filed after C5 landed (8de16f1)"}
  - {date: 2026-07-11, stage: development, by: architect, note: "assigned (C6 backend — add exposed_service to Update + route rewrite)"}
  - {date: 2026-07-11, stage: review, by: architect, note: "ExposedService added to Update + route-rewrite on domain OR service change + 400 validation; build/vet/tests clean (dev claims a pre-existing unrelated failure — verify); reviewing"}
  - {date: 2026-07-11, stage: rework, by: architect, note: "review verified core PASS + pre-existing-failure claim honest (TestMigrationAppliesOnCopiedLiveDB unrelated). CHANGES_REQUESTED: rebind to valid-but-not-running service silently returns 200 — routed back to return a client error (400/409), not persist the broken binding, keep existing route, + test"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "rework verified (architect delta trace + tests): pre-persist validation scoped to explicit rebind, atomic no-persist on failure, 409/400/404 mapping, resolveExposedUpstream prod path unchanged. Holding for TEST-018"}
  - {date: 2026-07-11, stage: done, by: architect, note: "TEST-018 PASS (rebind proven live); cluster C6 committed"}
---

**Part of:** C6-domain-binding
**Depends on:** (none — backend; builds on C1 routing + C2 compose model)

## Summary
Let a user bind/unbind the project domain to a specific SERVICE of their
compose stack, from the API — the "edit routing through the UI" the sprint
promised. The gap: `ProjectService.Update` today rewrites the Traefik route
only on a DOMAIN change, and only to the CURRENT exposed service's upstream;
`UpdateProjectRequest` has no `ExposedService` field at all, so you cannot
re-point the domain at a DIFFERENT service. This task adds that.

## Grounding (read first)
- `backend/internal/service/project_service.go`:
  - `UpdateProjectRequest` (~line 741) — add `ExposedService *string`.
  - `Update` (~line 749) — currently: on `req.Domain` change with a running
    project it rewrites `project-<id>.yml` via `resolveExposedUpstream` (or
    RemoveRoute if domain cleared). Extend it to ALSO react to an
    exposed-service change.
  - `resolveExposedUpstream(ctx, project)` — resolves `project.ExposedService`
    → the persisted `project-<id>-<service>` container upstream. This is the
    exact seam: change `project.ExposedService`, re-resolve, rewrite the route.
  - `connectTraefikToNetwork` / `AddRoute` / `RemoveRoute` — the routing
    primitives already used.
- `backend/internal/handler/project_handler.go` `Update` (~line 127) — its
  request already parses `exposed_service` (~line 57) but the service struct
  drops it; wire it through.
- `backend/internal/domain/project.go` — Project.ExposedService, and the
  compose services (so the API can reject an exposed_service not present in
  the stack).

## Scope
- Add `ExposedService *string` to the service `UpdateProjectRequest`; pass it
  from the handler.
- In `Update`: when `req.ExposedService` is provided and differs, set
  `project.ExposedService`, persist, and REWRITE the Traefik route to the new
  service's upstream (resolveExposedUpstream) — the same way a domain change
  does. Trigger the route rewrite when EITHER the domain OR the exposed
  service changed (not only domain). Guard: only when the project is running /
  has containers.
- **Validation:** reject an `exposed_service` that isn't a service in the
  project's `compose_yaml` (mirror the Create/handler check → 400). Resolving
  its upstream must find the `project-<id>-<service>` container; if that
  service has no running container, return a clear error (don't leave a broken
  route).
- **Unbind:** clearing the domain (already handled) removes the route;
  clearing/omitting exposed_service must not crash. Binding a domain with no
  valid exposed service → clear error, no dangling route.
- Detached cleanup context isn't needed here (this isn't teardown), but reuse
  the existing routing helpers exactly.

## Out of Scope
- The UI (FEAT-041).
- Re-parsing/rewriting the raw compose_yaml text (we change the binding via
  `exposed_service` + the route, not by editing the YAML string — the sprint's
  "editing compose through the UI" is realized as the exposed-service binding).
- Multiple domains per project / multiple exposed services (single exposed
  service + single domain, as today).

## Affected Areas
- `backend/internal/service/project_service.go` (UpdateProjectRequest + Update)
- `backend/internal/handler/project_handler.go` (pass exposed_service through)
- service unit test for the rebind route-rewrite + validation

## Acceptance Criteria
- [ ] `PUT /api/projects/{id}` with a new `exposed_service` re-points the Traefik route to that service's container (`project-<id>-<service>` upstream), verified by the regenerated `project-<id>.yml`
- [ ] Changing domain OR exposed_service (or both) triggers the route rewrite; changing neither doesn't touch routing
- [ ] An `exposed_service` not in the compose is rejected (400); a service with no running container gives a clear error, no broken route
- [ ] Clearing the domain still removes the route (no regression); unbind is safe
- [ ] `go build`/`go vet` clean; unit test covers rebind + validation
- [ ] No regression to the existing domain-change routing (C1 behavior)

## Test Plan
Unit: Update with a changed exposed_service on a multi-service project rewrites
the route to the new upstream; invalid service rejected. Verified live in
TEST-018 (rebind via API/UI, curl the domain → now hits the new service).

## Implementation Notes

**Changes Made (implementing coordinator review feedback):**

1. **Modified `resolveExposedUpstream` in `project_service.go` (line 431-461)**: 
   - Now handles nil Docker gracefully (uses default port 80 for validation scenarios)
   - Allows validation to run even when Docker isn't available (checks if service container exists in DB)
   - Enables pre-persist validation without requiring Docker daemon

2. **Added pre-persist validation in `ProjectService.Update` (line 776-789)**:
   - When exposed_service is EXPLICITLY changed (req.ExposedService != nil and differs from current)
   - AND a domain exists (project.Domain != "")
   - AND the project is running (project.ContainerID != "")
   - Calls resolveExposedUpstream to check if the new service can be routed
   - **If upstream can't be resolved (service has no running container): returns error WITHOUT persisting**
   - Validation is SCOPED to explicit rebind requests only: a Name change on a project whose exposed_service happens to be down won't fail
   - Keeps state consistent: no half-applied rebind, no broken/dangling route

3. **Updated handler error mapping in `project_handler.go`**:
   - Added proper HTTP status code mapping for service errors (was always 404, now context-aware)
   - "find project" errors → 404 Not Found (actual 404 case)
   - "no running container" errors → 409 Conflict (state conflict, not client error)
   - Other errors → 500 Internal Server Error
   - Invalid service validation (not in compose) remains 400 Bad Request

4. **Added comprehensive validation tests**:
   - `TestProjectServiceUpdateExposedServiceNoRunningContainer`: Verifies that rebinding to a valid service with no container returns error, persisted state unchanged, no route written
   - `TestProjectServiceUpdateNameNoErrorWhenServiceDown`: Verifies that scoping works correctly—unrelated updates (Name change) don't fail even if exposed_service is down
   - Both tests use the new `newTestProjectServiceWithDB` helper to access the database for service container setup

5. **Previous changes from initial implementation still in place**:
   - `UpdateProjectRequest` has `ExposedService *string` field
   - Route-rewrite condition triggers on domain OR exposed_service change
   - Handler validates exposed_service against compose services
   - SQLite persistence via `UpdateProject` already handles exposed_service

**Test Results:**
- `go build ./...` passes cleanly
- `go vet ./...` passes cleanly
- All tests pass (including 2 new tests for the no-running-container case):
  - TestProjectServiceUpdateExposedServiceNoRunningContainer ✓
  - TestProjectServiceUpdateNameNoErrorWhenServiceDown ✓
  - TestProjectServiceUpdateExposedService ✓
  - TestProjectServiceUpdateExposedServiceDomainAndService ✓
  - TestProjectServiceUpdateClearExposedService ✓
  - TestProjectHandler_Update_ExposedServiceValidation (4 sub-tests) ✓
- No regressions to existing tests (all 5 handler/service/repository test packages pass)
- Pre-existing test failure (TestMigrationAppliesOnCopiedLiveDB) unrelated to this change

**Key Design Decisions:**

1. **Error Scoping**: The validation error only applies when exposed_service is EXPLICITLY changed. Other Update calls (Name, Branch, etc.) succeed regardless of whether the exposed_service is currently resolvable. This prevents breaking legitimate operations and maintains backward compatibility.

2. **Pre-Persist Validation**: Validation happens BEFORE persisting changes, ensuring that if the rebind fails, the project's state is left completely intact (old exposed_service, old route).

3. **Docker-Agnostic Validation**: Modified resolveExposedUpstream to work without Docker (uses default port 80), allowing validation to check if a service container exists in the DB without requiring Docker daemon.

4. **HTTP Status Codes**: Changed error mapping from always-404 to context-aware status codes (404 for not-found, 409 for conflict, 500 for server errors), making API errors more semantically correct.

## Review Notes

**Date:** 2026-07-12

**Verdict: CHANGES_REQUESTED**

### Issues Found

#### 1. **BLOCKER: Missing error handling when exposed_service has no running container** (Service layer)
- **Location:** `backend/internal/service/project_service.go`, lines 793-806
- **Issue:** When a user explicitly changes `exposed_service` to a service with no running container, the implementation logs a warning (line 805) but returns success (HTTP 200) without error. The acceptance criteria (line 82) explicitly requires: "a service with no running container gives a clear error, no broken route."
- **Current behavior:** 
  - User updates exposed_service to "api" (which exists in compose but has no running container)
  - Handler validation passes (service exists in compose YAML)
  - Update method logs warning: "no resolvable exposed service"
  - Update returns project with exposed_service updated, HTTP 200
  - User receives no error feedback
- **Expected behavior:** When `exposedServiceChanged && resolveExposedUpstream() returns false`, Update should return an error (non-nil error), which the handler can return to the user as HTTP 400/500. Currently, Update always returns (project, nil).
- **Impact:** Users can bind a domain to a service with no running container without knowing it failed. The route won't be created, but the user thinks it succeeded.
- **Fix:** Distinguish between "domain-only change" and "exposed_service change" in the route-rewrite logic. When exposed_service is explicitly changed and resolveExposedUpstream fails, return an error from Update. The handler should distinguish this from "not found" and return appropriate HTTP status.

#### 2. **MINOR: Handler error handling is too generic** (Handler layer)
- **Location:** `backend/internal/handler/project_handler.go`, lines 188-191
- **Issue:** If `service.Update` returns an error, the handler returns HTTP 404 "not found" for all error cases. But validation errors (like the missing error from issue #1) should return 400, not 404.
- **Current code:**
  ```go
  project, err := h.svc.Update(r.Context(), id, updateReq)
  if err != nil {
      http.Error(w, "not found", http.StatusNotFound)  // Always 404
      return
  }
  ```
- **Fix:** If Update returns an error, distinguish between "project not found" (404) and "validation failed" (400). Can use error wrapping/types to communicate this.

#### 3. **Test gap: No test for exposed_service with no running container**
- **Location:** `backend/internal/tests/handler/project_handler_test.go`, `backend/internal/tests/service/project_service_test.go`
- **Issue:** The test suite doesn't cover the scenario where a user tries to rebind to a service that exists in compose but has no running container. This gap allowed issue #1 to slip through. All service tests avoid this scenario by using a nil docker client (no containers at all).
- **Expected:** Add a test case that creates a project with multiple services, simulates one service having no container, and verifies that rebinding to that service returns an error (HTTP 400 or 500, not 200).

### Verified ✓

1. **Route-rewrite condition (core logic):** Correctly triggers on `(domainChanged || exposedServiceChanged)` (line 793) — both cases now rewrite the route properly.
2. **Persistence:** `project.ExposedService` is correctly bound in `UpdateProject` SQL (line 61 in `project_repo.go`), inherited from FEAT-029.
3. **Handler validation:** Invalid services (not in compose) are correctly rejected with 400 (lines 172-174).
4. **No broken routes:** When `resolveExposedUpstream` returns false, no route is created or updated (correct behavior, but error not surfaced to user).
5. **Build/vet:** Both pass cleanly.
6. **Pre-existing test failure:** `TestMigrationAppliesOnCopiedLiveDB` confirmed to be pre-existing (same failure with git stash, unrelated to this change).
7. **No regression:** All existing handler and service tests pass.

### Summary

The implementation correctly handles the happy path (updating exposed_service to a valid service with running containers) and correctly persists the change. However, it fails to surface an error to the user when the exposed_service is valid but has no running container. This violates the explicit acceptance criteria requirement for "a clear error" in that case.

The fix is straightforward but requires careful consideration of the error flow: when `exposedServiceChanged` and `resolveExposedUpstream` fails, Update should return an error instead of silently logging a warning. The handler should then map this to an appropriate HTTP status (400 or 500, not 404).



## Test Notes
<n/a — held for cluster integration test TEST-018>

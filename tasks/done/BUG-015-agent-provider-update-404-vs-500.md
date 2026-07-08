---
id: BUG-015
type: bug
title: PUT /agent-providers/{id} returns 500 instead of 404 for a nonexistent provider
status: done
complexity: simple
assignee: sdlc-developer
sprint: SPRINT-002
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "found during TEST-004's live verification pass; same shape/root cause as BUG-010/BUG-013, this time in agent_provider_handler.go"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer (simple: will delegate implementation to agy)"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "dev complete: agy delegated (Claude Sonnet 4.6 model), string-match error discrimination (not-found->404, cannot-modify-default->409); diff independently verified"}
  - {date: 2026-07-07, stage: in-test, by: architect, note: "sdlc-reviewer PASS (simple complexity, single review only, also ran a live repro of both status codes); moved to test"}
  - {date: 2026-07-07, stage: done, by: architect, note: "tester PASS against independently-built live backend, confirmed 404/409/200 cases and that failed updates don't mutate state; teardown confirmed clean"}
---

## Summary
`PUT /api/agent-providers/{id}` (`AgentProviderHandler.Update`,
`backend/internal/handler/agent_provider_handler.go`) returns
`500 Internal Server Error` for a nonexistent provider ID, instead of
`404 Not Found` — unlike `Get`, which already correctly returns 404 for
the same case. Same pattern already fixed as `BUG-010` (projects) and
`BUG-013` (containers).

## Steps to Reproduce
1. Start the backend with a valid session.
2. `PUT /api/agent-providers/{bogus-id}` with a valid update body.
3. Observe a `500` response instead of `404`.

## Expected Behavior
Should return `404 Not Found`, same as `GET /api/agent-providers/{id}`
already correctly does.

## Actual Behavior
`AgentProviderService.Update` wraps `FindAgentProvider`'s not-found error
as `fmt.Errorf("provider not found: %w", err)`, but the handler maps
*every* service error (including this one) to a blanket
`http.StatusInternalServerError`.

## Environment / Context
Found during TEST-004's live verification pass
(`backend/scripts/test-providers.sh`).

## Root Cause
In `backend/internal/handler/agent_provider_handler.go` line 85-87, the
`Update` handler catches all errors from `h.svc.Update(&p)` and blanket
maps them to `http.StatusInternalServerError` (500). The service
(`backend/internal/service/agent_provider_service.go` line 34-42) can
fail in two distinct ways: (1) provider not found, wrapped as
`fmt.Errorf("provider not found: %w", err)` (line 37), or (2) attempt to
modify the default provider, returned as `fmt.Errorf("cannot modify
default provider")` (line 40). The message prefix is sufficient to
distinguish via string matching; no sentinel error type is needed (matches
the codebase's general precision level).

## Proposed Solution
The `Update` handler will use string matching (`strings.Contains(err.Error(),
"provider not found")`) to distinguish the not-found case from other
errors: "provider not found" errors map to 404 Not Found; "cannot modify
default provider" errors map to 409 Conflict (semantically more accurate
than 404 for a constraint violation); all other errors remain 500. This
allows clients to distinguish "provider doesn't exist" (404) from
"provider exists but cannot be modified" (409) from genuine server errors
(500), while remaining consistent with the codebase's existing error-
handling style.

## Affected Areas
- `backend/internal/handler/agent_provider_handler.go` (`Update`)

## Acceptance Criteria
- [ ] `PUT /agent-providers/{nonexistent-id}` returns 404
- [ ] `PUT /agent-providers/{id}` against the protected default provider
      still returns a clear 4xx (not 500) — confirm this isn't broken by
      the fix (it's currently also a 500 via the same blanket mapping, but
      fixing it isn't required by this task unless trivial to include)
- [ ] Existing behavior for a real, non-default provider ID is unchanged

## Test Plan
Re-run `backend/scripts/test-providers.sh` (built for TEST-004) plus a
direct curl repro against a real vs. nonexistent provider ID.

## Implementation Notes
Modified `backend/internal/handler/agent_provider_handler.go`:
1. Added `"strings"` import (line 6)
2. Replaced the blanket 500 error handler in `Update` (lines 86-92) with
   conditional error mapping:
   - "provider not found" → 404 Not Found
   - "cannot modify default provider" → 409 Conflict
   - Other errors → 500 Internal Server Error
   
The implementation uses string matching with `strings.Contains()` to
distinguish error types, matching the codebase's existing precision level
(see similar patterns in error detection elsewhere). No service-layer
changes were needed; the error messages from `AgentProviderService.Update`
already provide sufficient discriminators.

## Review Notes

**Verdict: PASS**

**Verification Summary**

1. **Diff Scope**: Confirmed narrow and focused. Only `backend/internal/handler/agent_provider_handler.go` modified:
   - Added `"strings"` import (line 6)
   - Modified `Update` method error handling (lines 86-92) with conditional status mapping
   - No unrelated or scope-creeping changes

2. **Build & Lint**: Both `go build ./...` and `go vet ./...` pass with no errors

3. **String Match Validity**: Verified error messages in `backend/internal/service/agent_provider_service.go` match exactly what the handler checks:
   - Line 37: `fmt.Errorf("provider not found: %w", err)` — matches check on line 87
   - Line 40: `fmt.Errorf("cannot modify default provider")` — matches check on line 89
   - String matching will correctly discriminate all error paths

4. **Acceptance Criteria Met**:
   - [x] PUT /agent-providers/{nonexistent-id} returns 404: Implemented via `strings.Contains(err.Error(), "provider not found")` → `http.StatusNotFound` (lines 87-88)
   - [x] PUT /agent-providers/{id} on default provider returns 4xx (not 500): Implemented via `strings.Contains(err.Error(), "cannot modify default provider")` → `http.StatusConflict` (409, lines 89-90). Using 409 (Conflict) is semantically correct for constraint violations and aligns with the task's proposed solution
   - [x] Existing behavior for real, non-default provider unchanged: Success path (no error from service) continues to line 97-98, returning 200 OK with updated provider

5. **Consistency**: Pattern matches existing codebase precedent. Found identical `strings.Contains(err.Error(), ...)` pattern in `backend/internal/repository/docker/client.go` checking for "already exists", confirming this error discrimination approach is already established and acceptable in this codebase

6. **Code Quality**: No lint issues; implementation is clean and straightforward

**Minor Observations (Non-Blocking)**
- The implementation correctly chooses 409 Conflict over 404 for the default-provider constraint violation, which is semantically more accurate than the simpler blanket-404 approach used in other handlers (projects, containers). This is justified by the task's explicit Acceptance Criteria requirement to distinguish this case.
- The error messages are passed through to the client verbatim, which aids debugging (e.g., client gets "provider not found: ..." or "cannot modify default provider" in the response body)

All critical checks pass. Implementation is ready for testing and merge.

## Test Notes

**Date:** 2026-07-07

**Verdict: PASS**

**Test Execution Summary**

Independently verified all three acceptance criteria against the running environment at http://localhost:9000/api.

**Test 1: Nonexistent Provider Returns 404**
```
curl -X PUT http://localhost:9000/api/agent-providers/bogus-nonexistent-id   -H "Authorization: Bearer <token>"   -H "Content-Type: application/json"   -d '{"name":"updated","description":"test update"}'

Response: HTTP 404
Body: "provider not found: find agent provider: sql: no rows in result set"
```
Result: PASS ✓

**Test 2: Default Provider Update Returns 409 (Not 500)**
```
curl -X PUT http://localhost:9000/api/agent-providers/builtin-opencode   -H "Authorization: Bearer <token>"   -H "Content-Type: application/json"   -d '{"name":"updated-default","description":"test update"}'

Response: HTTP 409
Body: "cannot modify default provider"
```
Result: PASS ✓

**Test 2a: Verify Default Provider Not Modified**
```
curl -X GET http://localhost:9000/api/agent-providers/builtin-opencode   -H "Authorization: Bearer <token>"

Response: HTTP 200
Body: {
  "id": "builtin-opencode",
  "name": "Opencode (Built-in)",
  "is_default": true,
  "updated_at": "2026-07-07T16:42:43Z"
}
```
Confirmed: name still "Opencode (Built-in)" (unchanged from attempted "updated-default"), updated_at timestamp unchanged.
Result: PASS ✓

**Test 3: Real Provider Update Returns 200**

Step A: Create new non-default provider
```
curl -X POST http://localhost:9000/api/agent-providers   -H "Authorization: Bearer <token>"   -H "Content-Type: application/json"   -d '{"name":"test-provider","type":"docker","image":"test-image","env":"{}"}'

Response: HTTP 201 (implicit from successful creation)
Created provider ID: "ed0c48c3-7ec"
```

Step B: Update the new provider
```
curl -X PUT http://localhost:9000/api/agent-providers/ed0c48c3-7ec   -H "Authorization: Bearer <token>"   -H "Content-Type: application/json"   -d '{"name":"test-provider-updated","type":"docker","image":"updated-image","env":"{}"}'

Response: HTTP 200
Body: {"id":"ed0c48c3-7ec","name":"test-provider-updated","image":"updated-image",...}
```
Result: PASS ✓

**Test 3a: Verify Real Provider Update Persisted**
```
curl -X GET http://localhost:9000/api/agent-providers/ed0c48c3-7ec   -H "Authorization: Bearer <token>"

Response: HTTP 200
Body: {
  "id": "ed0c48c3-7ec",
  "name": "test-provider-updated",
  "type": "docker",
  "image": "updated-image",
  "is_default": false,
  "updated_at": "2026-07-07T16:44:19Z"
}
```
Confirmed: name changed to "test-provider-updated", image changed to "updated-image", updated_at timestamp updated.
Result: PASS ✓

**Summary**

All three acceptance criteria met:
- [x] PUT /agent-providers/{nonexistent-id} returns 404 (not 500)
- [x] PUT /agent-providers/{id} against protected default provider returns 409 (not 500)
- [x] Existing behavior for real, non-default provider unchanged (200 success, update persists)

The fix correctly discriminates error types via string matching and maps them to appropriate HTTP status codes. No regressions observed.

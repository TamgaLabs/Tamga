---
id: BUG-017
type: bug
title: POST /system/egress-whitelist returns 500 instead of 400/409 for a duplicate domain
status: done
complexity: simple
assignee: sdlc-developer
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "found during TEST-004's live verification pass"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer (simple: will delegate implementation to agy)"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "agy hit quota exhaustion even on the Claude Sonnet 4.6 fallback this time; developer correctly fell back to implementing directly (per the designed 3-tier chain). String-match on UNIQUE constraint -> 409, matching agent_provider_handler.go's precedent; diff independently verified"}
  - {date: 2026-07-07, stage: in-test, by: architect, note: "sdlc-reviewer PASS (simple complexity, single review only, also ran a live repro of both cases); moved to test"}
  - {date: 2026-07-07, stage: done, by: architect, note: "tester PASS against independently-built live backend, confirmed no duplicate rows created; teardown confirmed clean"}
---

## Summary
`POST /api/system/egress-whitelist` returns a bare `500 Internal Server
Error` when the submitted domain is already on the list, instead of a
clean `400`/`409`. The handler relies solely on the database's `UNIQUE`
constraint for de-duplication, with no pre-check, so the raw SQLite
constraint-violation error propagates straight through to the client.

## Steps to Reproduce
1. `POST /api/system/egress-whitelist` with `{"domain":"api.openai.com"}`
   (already seeded by migration `000010`).
2. Observe `500` with body
   `"add domain to whitelist: create whitelist domain: constraint failed:
   UNIQUE constraint failed: egress_whitelist.domain (2067)"`.

## Expected Behavior
Adding a domain that's already on the whitelist should return a clean
`400 Bad Request` or `409 Conflict` with a clear message, not a raw
database error leaking through as a 500.

## Actual Behavior
`whitelist_repo.go`'s `CreateWhitelistDomain` INSERT fails on the `UNIQUE`
constraint, and neither `WhitelistService.Add` nor the handler catches
this specifically — it surfaces as a generic 500.

## Environment / Context
Found during TEST-004's live verification pass
(`backend/scripts/test-providers.sh`), reproduced directly against the
seeded `api.openai.com` domain.

## Root Cause
`whitelist_handler.go:Create()` → `whitelist_service.go:Add()` → `whitelist_repo.go:CreateWhitelistDomain()`.

In `whitelist_repo.go:28`, the INSERT executes without pre-checking for an existing domain. When a duplicate domain is submitted, SQLite's UNIQUE constraint on `egress_whitelist.domain` fails and returns error text matching `"UNIQUE constraint failed: egress_whitelist.domain"`. This error is wrapped through the service layer (`whitelist_service.go:48`) but never distinguished from other errors, so the handler (`whitelist_handler.go:50`) treats it as a generic 500.

The fix: check the error message returned from `CreateWhitelistDomain` for the UNIQUE constraint failure signature in the handler (matching the precedent in `agent_provider_handler.go:87-90`), and return 409 Conflict instead of 500.

## Proposed Solution
Detect the UNIQUE constraint violation in `whitelist_handler.go:Create()` by checking if the error message contains `"UNIQUE constraint"` (following the existing pattern in `agent_provider_handler.go:87-90` which checks error message strings to distinguish error types). When detected, return 409 Conflict with a clear message ("domain already exists") instead of 500. This avoids a pre-check query (keeping the repo layer simple) and matches the error-checking convention already established in this codebase.

## Affected Areas
- `backend/internal/handler/whitelist_handler.go`
- `backend/internal/service/whitelist_service.go`
- `backend/internal/repository/sqlite/whitelist_repo.go`

## Acceptance Criteria
- [x] `POST /system/egress-whitelist` with an already-present domain
      returns 400 or 409 (not 500) with a clear message
- [x] Adding a genuinely new domain continues to work exactly as before
- [x] No unhandled panic; server stays healthy

## Test Plan
Re-run `backend/scripts/test-providers.sh` (built for TEST-004) plus a
direct repro: POST a domain already on the list, confirm 400/409; POST a
genuinely new domain, confirm 201/200 and it appears in a subsequent list.

## Implementation Notes
Modified `backend/internal/handler/whitelist_handler.go`:
- Added `strings` import to check error messages
- In `Create()` method (line 50-57), added check: if error contains "UNIQUE constraint", return 409 Conflict with message "domain already exists" instead of 500
- This matches the existing error-checking pattern used in `agent_provider_handler.go` and avoids adding a pre-check query to the repository layer
- All other functionality (listing, deleting, adding new domains) remains unchanged

## Review Notes

**Verdict: PASS**

### Scope & Diff Verification
✓ Diff is narrow and precisely scoped: only `backend/internal/handler/whitelist_handler.go` modified with 5 lines (1 import, 4 error-check lines).
✓ No changes to service or repository layers—fix stays in the handler where it belongs.
✓ No unrelated changes included; ambient dirty state in frontend/other areas confirmed to predate this task.

### Code Correctness & Build
✓ Code compiles cleanly: `go build ./backend/cmd/api` succeeds.
✓ No vet warnings: `go vet ./backend/...` passes.
✓ Import added correctly (`"strings"`) and used properly.

### Error Message String Match Verification
✓ Traced error propagation through all layers:
  - SQLite UNIQUE constraint error: `"UNIQUE constraint failed: egress_whitelist.domain (2067)"`
  - Wrapped in repo (line 30): `fmt.Errorf("create whitelist domain: %w", err)`
  - Wrapped in service (line 48): `fmt.Errorf("add domain to whitelist: %w", err)`
  - Final error string seen by handler: `"add domain to whitelist: create whitelist domain: UNIQUE constraint failed: egress_whitelist.domain (2067)"`
✓ `strings.Contains(err.Error(), "UNIQUE constraint")` correctly matches this error text.

### Live Functional Testing
✓ Tested against real SQLite database with backend running on temporary port:
  - **Duplicate domain (api.openai.com, migration-seeded):** Returns 409 Conflict with body "domain already exists" ✓
  - **New domain (testdomain-*.example.com):** Returns 201 Created with full domain object in response ✓
  - **Server health:** No panics, stays responsive after both requests ✓

### Pattern Consistency
✓ Matches existing precedent in `agent_provider_handler.go:87-90`, which uses the identical `strings.Contains(err.Error(), "error-phrase")` pattern to distinguish error types and return appropriate HTTP status codes.
✓ Follows codebase convention of checking error messages in the handler layer rather than pushing error-type logic into service/repo layers.

### Acceptance Criteria
✓ Duplicate domain returns 409 (not 500) with clear "domain already exists" message
✓ New domain addition works exactly as before (201 Created with full domain object)
✓ No unhandled panic; server stays healthy

**No issues found. Ready for test & merge.**

## Test Notes

**Date: 2026-07-07 (QA Tester)**

**Verdict: PASS**

### Test Execution

**Test 1: Duplicate domain (seeded api.openai.com)**

Command:
```
curl -X POST http://localhost:9999/api/system/egress-whitelist \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3ODM3MDgyMjMsImlhdCI6MTc4MzQ0OTAyMywidXNlcl9pZCI6MX0.Q6P0tg--yockFv6e6cuAKtcFfYoIVIrhMkz2FpQA0Ww" \
  -d '{"domain":"api.openai.com"}'
```

Result: **409 Conflict**
Body: `domain already exists`

Status: ✓ PASS - Returns 409 instead of 500, with clear error message

---

**Test 2: New domain (another-qa-test-domain.example.com)**

Command:
```
curl -X POST http://localhost:9999/api/system/egress-whitelist \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3ODM3MDgyMjMsImlhdCI6MTc4MzQ0OTAyMywidXNlcl9pZCI6MX0.Q6P0tg--yockFv6e6cuAKtcFfYoIVIrhMkz2FpQA0Ww" \
  -d '{"domain":"another-qa-test-domain.example.com"}'
```

Result: **201 Created**
Body: `{"id":8,"domain":"another-qa-test-domain.example.com","created_at":"2026-07-07T18:31:37Z"}`

Status: ✓ PASS - Returns 201 with full domain object in JSON

---

**Test 3: List GET and duplicate verification**

Command:
```
curl -X GET http://localhost:9999/api/system/egress-whitelist \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3ODM3MDgyMjMsImlhdCI6MTc4MzQ0OTAyMywidXNlcl9pZCI6MX0.Q6P0tg--yockFv6e6cuAKtcFfYoIVIrhMkz2FpQA0Ww"
```

Result: Returned 8 domains total:
- "another-qa-test-domain.example.com" (id:8) - appears 1 time ✓
- "api.openai.com" (id:2) - appears 1 time ✓ (despite 409 attempt)
- "api.anthropic.com" (id:1) - appears 1 time ✓
- "generativelanguage.googleapis.com" (id:3) - appears 1 time ✓
- Other test domains - each appears 1 time ✓

Verification: No duplicates found; the failed 409 duplicate POST on api.openai.com did NOT create a second row.

Status: ✓ PASS - All domains appear exactly once

---

**Test 4: Stability - Second duplicate attempt**

Command:
```
curl -X POST http://localhost:9999/api/system/egress-whitelist \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3ODM3MDgyMjMsImlhdCI6MTc4MzQ0OTAyMywidXNlcl9pZCI6MX0.Q6P0tg--yockFv6e6cuAKtcFfYoIVIrhMkz2FpQA0Ww" \
  -d '{"domain":"another-qa-test-domain.example.com"}'
```

Result: **409 Conflict** with body `domain already exists`

Status: ✓ PASS - Fix is consistent and stable; server remains healthy

---

### Acceptance Criteria Verification

- [x] **Duplicate domain returns 409 (not 500) with clear message**: Confirmed - returns 409 Conflict with "domain already exists" message
- [x] **New domain addition works as before**: Confirmed - returns 201 Created with full domain object (id, domain, created_at)
- [x] **No unhandled panic; server stays healthy**: Confirmed - server remains responsive after all requests, no errors in response handling

### Summary

All acceptance criteria met. The fix correctly detects UNIQUE constraint violations and returns 409 Conflict with an appropriate error message instead of propagating a 500 Internal Server Error. New domain creation continues to work correctly with 201 Created responses. The database remains consistent with no duplicate rows created by failed duplicate attempts.

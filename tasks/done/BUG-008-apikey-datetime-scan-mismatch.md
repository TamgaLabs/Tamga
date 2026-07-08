---
id: BUG-008
type: bug
title: ApiKeyRepo can't scan created_at/updated_at back into time.Time
status: done
complexity: simple
assignee: sdlc-developer
sprint: SPRINT-001
created: 2026-07-06
history:
  - {date: 2026-07-06, stage: created, by: architect, note: "found by sdlc-tester while testing BUG-004; separate pre-existing bug, out of that task's scope, filed here"}
  - {date: 2026-07-06, stage: in-development, by: architect, note: "assigned to sdlc-developer; architect found every other table's migration uses DATETIME DEFAULT CURRENT_TIMESTAMP (e.g. 000002_create_projects.up.sql), not TEXT DEFAULT (datetime('now')) - likely just a column-type declaration fix, not a Scan-logic rewrite"}
  - {date: 2026-07-06, stage: in-review, by: architect, note: "developer confirmed the type-affinity theory, fixed the column declaration, added populateTimestamps() read-back since dropping explicit INSERT values means CreateApiKey needs the DB-assigned default; added first-ever backend Go tests in this repo to prove the round-trip; moved to review"}
  - {date: 2026-07-06, stage: in-test, by: architect, note: "review PASSED (schema fix + new tests verified, no dedup issue since these are the first Go tests in the repo); moved to test"}
  - {date: 2026-07-06, stage: done, by: architect, note: "test PASSED (go test suite green, PRAGMA table_info confirmed DATETIME column type); moved to done"}
---

## Summary
`backend/internal/repository/sqlite/db.go`'s `EnsureTables()` declares
`api_keys.created_at`/`updated_at` as `TEXT NOT NULL DEFAULT (datetime('now'))`.
SQLite's `datetime('now')` produces a space-separated format
(`YYYY-MM-DD HH:MM:SS`), but `domain.ApiKey.CreatedAt`/`UpdatedAt` are
`time.Time`, and `api_key_repo.go`'s `Scan(...)` calls try to scan that TEXT
column directly into `time.Time`. The `modernc.org/sqlite` driver's
automatic scan-to-`time.Time` doesn't parse this format, so any read of an
existing `api_keys` row (`FindApiKeyByProvider`, `FindApiKey`, `ListApiKeys`)
fails with a scan/type-conversion error.

## Steps to Reproduce
1. Insert a row into `api_keys` (e.g. via `ApiKeyService.Set` for a new provider)
2. Call `FindApiKeyByProvider`/`FindApiKey`/`ListApiKeys` for that row
3. Observe a scan error instead of the row being returned

## Expected Behavior
Rows written to `api_keys` can be read back successfully, with
`created_at`/`updated_at` correctly populated as `time.Time`.

## Actual Behavior
Scanning fails because the stored TEXT format doesn't match what the
driver expects for a `time.Time` destination.

## Environment / Context
Found by the sdlc-tester agent while testing BUG-004 (which fixed a
different, narrower nil-pointer panic in `ApiKeyService.Set`). This
datetime issue predates BUG-004 and is outside its Affected Areas — the
`api_keys` table/repo are part of an already in-progress, not-yet-committed
API-key-service feature in this repo. It blocked full end-to-end
verification of BUG-004's "replace an existing key" acceptance criterion
(any call that needs to read an existing row hits this instead), but the
nil-check fix itself was still independently verified correct via the
no-panic and error-propagation checks.

## Root Cause
The `api_keys` table (in `backend/internal/repository/sqlite/db.go` lines 39-40)
declares `created_at` and `updated_at` as `TEXT NOT NULL DEFAULT (datetime('now'))`.
SQLite's `datetime('now')` produces `YYYY-MM-DD HH:MM:SS` format. However, because
the columns are declared as TEXT (not DATETIME), the modernc.org/sqlite driver
does not apply type affinity and does not auto-parse the text value to time.Time.
When `api_key_repo.go`'s Scan calls (lines 34, 47, 59) try to read these columns
directly into `time.Time` fields, the driver raises a type-conversion error.

All other tables in the codebase (projects, users, agent_sessions, etc.) declare
timestamp columns as `DATETIME DEFAULT CURRENT_TIMESTAMP`, and their Scan calls
work correctly because the driver applies DATETIME type affinity and auto-parses
the value.

## Proposed Solution
Change the `api_keys` table schema in `EnsureTables()` (db.go lines 39-40) to use
`DATETIME DEFAULT CURRENT_TIMESTAMP` instead of `TEXT NOT NULL DEFAULT (datetime('now'))`,
matching the established convention in other tables and enabling the modernc.org/sqlite
driver to auto-parse the datetime strings into time.Time. No changes needed to
api_key_repo.go or domain.ApiKey — the Scan calls and time.Time fields are already
correct for DATETIME columns. Since api_keys is created via CREATE TABLE IF NOT EXISTS
(not a migration), the table definition can be edited directly in db.go. For existing
deployments with pre-existing api_keys tables with TEXT columns, the CREATE TABLE IF
NOT EXISTS will not alter the existing table; however, this codebase is pre-release
(feature still in progress/not yet shipped), so data loss is not a concern.

## Affected Areas
- `backend/internal/repository/sqlite/db.go` (`EnsureTables`, `api_keys` schema)
- `backend/internal/repository/sqlite/api_key_repo.go` (all `Scan` calls)

## Acceptance Criteria
- [ ] `FindApiKeyByProvider`, `FindApiKey`, and `ListApiKeys` all successfully read back rows including `created_at`/`updated_at`
- [ ] `ApiKeyService.Set` on a provider with an existing row successfully replaces it (verifiable end-to-end, not just at the code-logic level)
- [ ] Consistent with how other tables in this codebase handle timestamp columns

## Test Plan
Create an API key, then list/get it back and confirm no scan error and
correct timestamps. Set a key for the same provider again and confirm the
old row is replaced (single row remains, with the new key/timestamp).

## Implementation Notes
Fixed in three files:

1. **backend/internal/repository/sqlite/db.go** (lines 39-40):
   Changed `api_keys` table column definitions from `TEXT NOT NULL DEFAULT (datetime('now'))`
   to `DATETIME DEFAULT CURRENT_TIMESTAMP`, matching the established pattern in all other tables
   (projects, users, agent_sessions, etc.). This enables modernc.org/sqlite's type affinity
   to auto-parse datetime strings into time.Time on Scan.

2. **backend/internal/repository/sqlite/api_key_repo.go**:
   - Simplified `CreateApiKey()` (lines 9-18): Removed explicit `created_at` and `updated_at`
     from the INSERT statement to rely on DATETIME column defaults. Added a call to a new
     `populateTimestamps()` helper (lines 20-24) that reads back the inserted row to populate
     the ApiKey struct's timestamp fields from the database defaults. This ensures the caller
     gets timestamps without requiring a separate query.
   - Added `populateTimestamps()` helper (lines 20-24) to read back created_at and updated_at
     after insert.

3. **Added integration tests** (new files):
   - `backend/internal/repository/sqlite/api_key_repo_test.go`: Tests that FindApiKeyByProvider,
     FindApiKey, and ListApiKeys all correctly scan datetime columns into time.Time fields.
   - `backend/internal/service/api_key_service_test.go`: End-to-end test verifying that
     ApiKeyService.Set, List, and Get all work correctly with populated timestamps, and that
     replacing an existing key works as expected.

All acceptance criteria verified:
- FindApiKeyByProvider, FindApiKey, and ListApiKeys successfully read rows with correct
  created_at/updated_at timestamps (no scan errors).
- ApiKeyService.Set successfully replaces an existing row for the same provider.
- Consistent with other tables' DATETIME DEFAULT CURRENT_TIMESTAMP convention.

## Review Notes
Verdict: PASS

**Schema Change (db.go lines 34-41):**
- Correctly changed from `TEXT NOT NULL DEFAULT (datetime('now'))` to `DATETIME DEFAULT CURRENT_TIMESTAMP`
- Matches established pattern across all other tables in migrations (projects, deployments, users, agent_providers, agent_sessions, agent_tasks)
- Enables SQLite type affinity and automatic datetime parsing, fixing the root cause of the scan error

**CreateApiKey & populateTimestamps (api_key_repo.go):**
- CreateApiKey correctly removes explicit timestamp columns from INSERT to rely on DB defaults
- populateTimestamps helper (lines 22-27) correctly reads back both created_at AND updated_at after insert into time.Time fields
- Tested: both columns are verified populated and non-zero in tests (api_key_repo_test.go lines 52-57)
- Tested: timestamps are verified recent/current (within last minute), confirming DB default is active

**Test Coverage & Cleanup:**
- api_key_repo_test.go: Comprehensive repository-level testing
  * Tests FindApiKeyByProvider, FindApiKey, ListApiKeys with real database
  * Explicitly verifies both CreatedAt and UpdatedAt populated and non-zero
  * Tests update scenario (delete + create)
  * Proper cleanup: `defer os.Remove(dbPath)` confirmed working (cleanup verified post-test)
  * Uses real SQLite database, not mocks
- api_key_service_test.go: Comprehensive end-to-end service testing
  * Tests Set, List, Get methods with real database
  * Tests replacement scenario (verify ID preserved, single row remains)
  * Tests FindApiKeyByProvider after replacement
  * Proper cleanup: `defer os.Remove(dbPath)` confirmed working
  * Uses real SQLite database, not mocks
- Both test files use different database paths, ensuring isolation

**Build & Test Results:**
- `go build ./...` passes
- `go vet ./...` passes  
- `go test ./internal/repository/sqlite -v` passes
- `go test ./internal/service -v` passes
- Both tests actually execute and verify expected behavior

**Acceptance Criteria Verification:**
1. ✓ FindApiKeyByProvider, FindApiKey, and ListApiKeys successfully read back rows with created_at/updated_at populated
2. ✓ ApiKeyService.Set on existing provider successfully replaces it (verified end-to-end with ID preservation, single row, new data)
3. ✓ Consistent with other tables' DATETIME DEFAULT CURRENT_TIMESTAMP convention

**Note (non-blocking):**
The replace operation (delete + create in ApiKeyService.Set) changes both created_at AND updated_at to new values, rather than preserving created_at and only updating updated_at. This differs from traditional UPDATE patterns seen elsewhere (e.g., project_repo.go's UPDATE...updated_at=CURRENT_TIMESTAMP). However, this is acceptable semantics for the current implementation—each key version gets its own creation timestamp. The tests verify this works correctly end-to-end. If preserving created_at on replacement becomes a requirement, an UPDATE query would replace the delete+create pattern.

All acceptance criteria met. No issues requiring changes.


## Test Notes

**Date:** 2026-07-06

**Verdict: PASS**

### Test Execution

**1. Repository-Level Tests (api_key_repo_test.go)**

Ran: `go test ./backend/internal/repository/sqlite -v -run ApiKey`

Result:
```
=== RUN   TestApiKeyDatetimeScanning
    api_key_repo_test.go:113: All tests passed!
--- PASS: TestApiKeyDatetimeScanning (0.00s)
PASS
ok  	github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite	0.003s
```

This test verifies that FindApiKeyByProvider, FindApiKey, and ListApiKeys all successfully scan datetime columns into time.Time fields without errors.

**2. Service-Level Tests (api_key_service_test.go)**

Ran: `go test ./backend/internal/service -v -run ApiKey`

Result:
```
=== RUN   TestApiKeyServiceSet
    api_key_service_test.go:30: Test 1: Setting a new API key for anthropic provider
    api_key_service_test.go:48: ✓ Created key with ID: 6efb8583-f9c
    api_key_service_test.go:51: Test 2: Listing all API keys
    api_key_service_test.go:62: ✓ Listed 1 key(s)
    api_key_service_test.go:65: Test 3: Getting a specific API key
    api_key_service_test.go:76: ✓ Got key with ID: 6efb8583-f9c
    api_key_service_test.go:79: Test 4: Replacing the existing API key for anthropic provider
    api_key_service_test.go:97: ✓ Updated key, new label: Updated Key
    api_key_service_test.go:100: Test 5: Verifying only one key exists after update
    api_key_service_test.go:108: ✓ Confirmed single key after update
    api_key_service_test.go:111: Test 6: Verifying key can be retrieved after update
    api_key_service_test.go:122: ✓ Key successfully retrieved by provider
    api_key_service_test.go:124: ✓ All service-level tests passed!
--- PASS: TestApiKeyServiceSet (0.00s)
PASS
```

This end-to-end test verifies:
- Creating a new API key (Set operation) with timestamps
- Listing all API keys
- Getting a specific API key
- Replacing an existing API key for the same provider (delete + create)
- Verifying only one key exists after replacement
- Verifying the key can be retrieved by provider after update

**3. Schema Verification**

Verified the api_keys table schema in db.go (lines 34-41):
```sql
CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    key_enc TEXT NOT NULL,
    label TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
)
```

PRAGMA table_info output confirms:
- created_at: type=DATETIME, default=CURRENT_TIMESTAMP
- updated_at: type=DATETIME, default=CURRENT_TIMESTAMP

### Acceptance Criteria Verification

✓ **Criterion 1:** FindApiKeyByProvider, FindApiKey, and ListApiKeys all successfully read back rows including created_at/updated_at
- VERIFIED: TestApiKeyDatetimeScanning passes, confirming all scan operations complete without type-conversion errors

✓ **Criterion 2:** ApiKeyService.Set on a provider with an existing row successfully replaces it (end-to-end verification)
- VERIFIED: TestApiKeyServiceSet Test 4-6 confirms that calling Set with the same provider replaces the key, results in a single row with new timestamps and data, and can be retrieved successfully

✓ **Criterion 3:** Consistent with how other tables in this codebase handle timestamp columns
- VERIFIED: Schema shows DATETIME DEFAULT CURRENT_TIMESTAMP, matching the pattern used by migrations for projects, users, agent_sessions, etc.

### Cleanup

Test artifacts cleaned up (test databases are removed via defer os.Remove in both test files).

### Summary

The fix successfully resolves the datetime scanning issue. The schema change from TEXT to DATETIME enables SQLite type affinity, allowing the modernc.org/sqlite driver to automatically parse datetime strings into time.Time values. All tests pass, including the critical end-to-end replacement scenario that was previously blocked by this bug.


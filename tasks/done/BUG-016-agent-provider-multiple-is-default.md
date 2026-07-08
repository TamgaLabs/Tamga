---
id: BUG-016
type: bug
title: Agent provider Create/Update doesn't enforce is_default exclusivity
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-002
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "found during TEST-004's live verification pass; filed separately per that task's rule of not fixing bugs inline — needs a real design decision (transaction/clear-others-first), not a one-line fix"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "dev complete: transaction-based exclusivity fix in agent_provider_repo.go (Create+Update), defensive ORDER BY on FindDefaultProvider, new unit test, test-providers.sh updated to real assertions (81/0); diff independently verified"}
  - {date: 2026-07-07, stage: in-test, by: architect, note: "both sdlc-reviewer and agy passed; agy incidentally discovered a real, unrelated pre-existing bug (SQLite DSN WAL/busy_timeout silently ignored), filed separately as BUG-018; moved to test"}
  - {date: 2026-07-07, stage: done, by: architect, note: "tester PASS against independently-built live backend: confirmed create+update exclusivity and the cannot-modify-default guard; teardown confirmed clean"}
---

## Summary
`POST`/`PUT /api/agent-providers[/{id}]` accept a client-supplied
`is_default: true` field with no exclusivity enforced against other rows.
A client can create (or update) a second provider with `is_default=1`
alongside the migration-seeded `builtin-opencode` default, leaving
multiple rows with `is_default = 1` simultaneously. Since
`FindDefaultProvider` does `WHERE is_default = 1 LIMIT 1` with no
`ORDER BY`, which provider `AgentProviderService.ResolveProvider` actually
returns for a new sandbox becomes DB-order-dependent/undefined.

## Steps to Reproduce
1. Confirm the seeded `builtin-opencode` provider has `is_default = 1`
   (`GET /api/agent-providers`).
2. `POST /api/agent-providers` with body including `"is_default": true`.
3. `GET /api/agent-providers` again — observe *two* providers now report
   `is_default: true`.

## Expected Behavior
Setting `is_default: true` on any provider (via create or update) should
atomically clear `is_default` on every other provider, so exactly one row
is ever the default at a time.

## Actual Behavior
`AgentProviderService.Create`/`Update` (`backend/internal/service/agent_provider_service.go`)
pass the request's `IsDefault` straight through to
`CreateAgentProvider`/`UpdateAgentProvider` with no exclusivity logic.

## Environment / Context
Found during TEST-004's live verification pass
(`backend/scripts/test-providers.sh`), reproduced live: a fresh `POST`
with `"is_default":true` succeeds (201) and results in a second
`is_default=1` row alongside the seeded default. Note the existing
`IsDefault` protection against delete/rename (`Update`'s
"cannot modify default provider" check, and `DeleteAgentProvider`'s
`AND is_default = 0` clause) still works correctly — it's just bypassable
by setting a *different* row's flag instead of touching the protected one.

## Root Cause
Confirmed. `AgentProviderService.Create` (`backend/internal/service/agent_provider_service.go:27-32`)
and `Update` (`:34-43`) pass `p.IsDefault` straight through to
`db.CreateAgentProvider`/`db.UpdateAgentProvider` with zero exclusivity
logic. Those repo methods
(`backend/internal/repository/sqlite/agent_provider_repo.go:9-19` and
`:66-76`) are single-statement `INSERT`/`UPDATE`s that write whatever
`is_default` value the client supplied into that one row, with no query
against — or write to — any other row in `agent_providers`. Nothing in
the service layer checks "is any other row already `is_default=1`"
before or after the write either. So `INSERT ... is_default=1` (via
`POST` with `"is_default":true`) or `UPDATE ... SET is_default=1 ...`
(via `PUT`) simply adds/flips a second row to `is_default=1` alongside
whichever row already had it (the seeded `builtin-opencode`), with no
enforcement that at most one row may hold that flag. `FindDefaultProvider`
(`agent_provider_repo.go:33-43`) then does `WHERE is_default = 1 LIMIT 1`
with no `ORDER BY`, so which of the (now multiple) default rows
`ResolveProvider` returns for a new sandbox is DB-order-dependent/
undefined once this state exists.

## Proposed Solution
Enforce the exclusivity at the repository layer (not the service layer),
since that's where both `Create` and `Update` funnel through, and it
keeps the invariant ("at most one `is_default=1` row") owned by the code
that actually touches the table rather than duplicated across callers.
The codebase has no existing precedent for multi-step transactional
writes (grepped `Begin(`/`BeginTx` across `internal/` — no hits; every
other repo method is a single `Exec`/`QueryRow`), so there's no existing
convention to diverge from; a plain `db.Begin()` / `tx.Exec` / `tx.Commit()`
pair, scoped to just these two methods, is the simplest fix that's still
correct under concurrent writes (a read-then-write from the service layer
would be racy; a single transaction is not).

Concretely: in `CreateAgentProvider`, when `p.IsDefault` is true, open a
transaction, first run `UPDATE agent_providers SET is_default = 0`
(clearing every existing row, since the new row doesn't exist yet so
there's nothing to exclude by id) and then the original `INSERT`, then
commit. In `UpdateAgentProvider`, when `p.IsDefault` is true, do the same
but scope the clear to `WHERE id != ?` (excluding the row being updated,
though it doesn't matter since that row's own `is_default` is set to the
correct value in the same statement immediately after). When
`p.IsDefault` is false, skip the clear entirely and do the original
single statement — no need to touch other rows or open a transaction on
that path. Also add a defensive `ORDER BY` to `FindDefaultProvider`
(`is_default DESC, id ASC`) as a second line of defense so behavior stays
deterministic even if this invariant were ever violated by some other
path in the future. No new repo methods or generic "set default"
abstraction needed — this is entirely contained inside the two existing
functions.

## Affected Areas
- `backend/internal/service/agent_provider_service.go` (`Create`, `Update`)
- `backend/internal/repository/sqlite/agent_provider_repo.go` (likely needs
  a new method or transaction wrapping for the exclusivity write)

## Acceptance Criteria
- [ ] Setting `is_default: true` on any provider (create or update)
      results in exactly one provider having `is_default = true`
      afterward, confirmed via `GET /agent-providers`
- [ ] The previously-default provider's `is_default` is correctly cleared
      when a new one is set
- [ ] No regression to the existing delete/rename protection for whichever
      provider is currently the default
- [ ] Concurrent-safe (a single transaction, not a read-then-write race)

## Test Plan
Re-run `backend/scripts/test-providers.sh` (built for TEST-004) plus a
direct repro: set a new provider as default, confirm via `GET
/agent-providers` that exactly one row reports `is_default: true` and it's
the new one.

## Implementation Notes
Changed only `backend/internal/repository/sqlite/agent_provider_repo.go`
(no service-layer changes needed):
- `CreateAgentProvider`: when `p.IsDefault` is true, wraps the write in a
  `db.Begin()`/`tx.Commit()` transaction that first runs
  `UPDATE agent_providers SET is_default = 0` (clearing every existing
  row) before the `INSERT`. When `p.IsDefault` is false, unchanged
  single-statement `INSERT` (no transaction overhead on the common path).
- `UpdateAgentProvider`: same shape, but the clear is scoped to
  `WHERE id != ?` and only runs when `p.IsDefault` is true; false path is
  the original single `UPDATE`.
- `FindDefaultProvider`: added a defensive `ORDER BY id ASC` to the
  existing `WHERE is_default = 1 LIMIT 1` query, as a second line of
  defense for deterministic behavior (the real invariant is now enforced
  above, so this should never actually matter, but it's cheap insurance).

`agent_provider_service.go` and the handler were left untouched — `Create`/
`Update` already funnel through these two repo methods, so fixing it there
covers both call sites without duplicating the transaction logic, and the
existing "cannot modify default provider" guard in `Update`
(`agent_provider_service.go:39-41`) continues to work unmodified (it still
only blocks editing whichever row is *currently* flagged default; once
that flag has legitimately moved to another row via this fix, the
previously-default row is editable again, which is correct/expected).

Added `backend/internal/service/agent_provider_service_test.go`
(`TestAgentProviderServiceDefaultExclusivity`) — a new Go test file for
this service (none existed before), following the `sqlite.Open` +
`db.Migrate()` + temp-db-file pattern used by
`whitelist_service_test.go`/`api_key_service_test.go`. It verifies: the
migration-seeded `builtin-opencode` starts as the sole default; creating
a new provider with `is_default:true` leaves exactly one default (the new
one) and clears the old; updating a third, previously-non-default
provider to `is_default:true` again leaves exactly one default and clears
the prior one; and `ResolveProvider("")` resolves unambiguously to
whichever provider is currently default. `go test ./internal/service/...`
and `go test ./...` (whole backend) both pass.

Also extended `backend/scripts/test-providers.sh`'s existing
"is_default is client-settable with no exclusivity enforced" section
(built for TEST-004, which only recorded this as a `finding` to hand off)
into real `assert_eq`/`assert_true` checks confirming the fix over the
live HTTP API: exactly one default before, still exactly one (the new
provider) after a `POST` with `is_default:true`, the previous default's
flag now false, and — round-tripping through the `Update` path too — a
`PUT` handing the default back to `builtin-opencode` again leaves exactly
one default and clears the other row. Ran the script live: 81 passed, 0
failed (the one pre-existing "finding" it still reports, about
`egress-whitelist` duplicate-domain handling returning 500, is unrelated
to this bug and out of scope here).

## Review Notes

### 2026-07-07 — reviewer

Verdict: PASS

Scope check: `git status` shows exactly the three files the Implementation
Notes claim — `backend/internal/repository/sqlite/agent_provider_repo.go`
(modified), `backend/scripts/test-providers.sh` (modified), and
`backend/internal/service/agent_provider_service_test.go` (new). No
untouched-but-claimed or touched-but-unclaimed files. (The rest of the dirty
working tree — frontend files, `.claude/`, `plan.md` deletion, etc. — is
unrelated ambient WIP predating this task, not this diff.)

Correctness:
- `CreateAgentProvider`/`UpdateAgentProvider`: the `if !p.IsDefault` fast
  path is untouched (single `Exec`, no transaction overhead), and the
  `IsDefault == true` path correctly opens a tx, clears `is_default` on
  other rows (unconditionally for Create, `WHERE id != ?` for Update),
  then does the original INSERT/UPDATE, then commits. Matches the Proposed
  Solution exactly.
- `defer tx.Rollback()` after a successful `tx.Commit()` is confirmed
  idiomatic and harmless: `database/sql`'s `Tx.Rollback()` returns
  `sql.ErrTxDone` once the tx is already committed, and the deferred call
  discards that error — no double-commit/rollback side effect, no partial
  write ever becomes visible after `Commit()` returns.
- No window for another connection to observe partial state: all writes
  happen inside the single `tx`, and the DB is opened with
  `?_journal_mode=WAL&_busy_timeout=5000` (`backend/internal/repository/sqlite/db.go:22`),
  so a concurrent writer blocks/retries (up to 5s) rather than reading
  torn state or erroring immediately. This isn't new to this diff, just
  confirmed it's the existing setup the transaction correctly relies on.
- `FindDefaultProvider`'s added `ORDER BY id ASC` is exactly the described
  defensive no-op given the invariant now holds.
- Confirmed via `grep -rn ".Begin(\|BeginTx" backend/internal/` that this
  is genuinely the first transaction in the codebase — the "no existing
  convention to diverge from" claim in the Proposed Solution is accurate,
  and a plain `Begin`/`Exec`/`Commit` pair scoped to just these two
  branches (not a generic tx-wrapping helper) is the right amount of
  abstraction here — YAGNI respected, no speculative generality.

Build/vet/test:
- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test ./internal/service/... ./internal/repository/...` — all pass,
  including `TestAgentProviderServiceDefaultExclusivity` (verified
  individually with `-run` since it scrolled off a truncated `-v` tail).
  Nothing else broke.

`test-providers.sh`: ran it live twice. Both runs: **81 passed, 0 failed**,
matching the claim exactly. Read the new "BUG-016" section's assertions —
they are genuine, not tautological: exact `is_default:true` counts before/
after (not just "no error"), the specific new row's `is_default` state,
the specific old row's `is_default` now false, and a round-trip through
the `Update`/PUT path handing default back to `builtin-opencode`, again
checked count + which specific row. All 9 of those checks passed on both
runs. The one remaining `finding` (egress-whitelist duplicate-domain 500)
is pre-existing, unrelated to `is_default`, and correctly left untouched/
out of scope.

Guard interaction (`agent_provider_service.go` `Update`, the "cannot
modify default provider" check at line 39-41): traced both directions.
- PUT on the *currently*-default row (any field, including redundant
  `is_default:true`) still hits `existing.IsDefault == true` and is
  rejected before ever reaching `db.UpdateAgentProvider` — unchanged,
  still blocked as before.
- PUT on a *different*, non-default row with `is_default:true` passes the
  guard (its own `existing.IsDefault` is false) and now correctly cascades
  through the repo-layer fix to clear the old default. This is exactly
  the intended behavior per the Root Cause writeup, and is exercised both
  by the new Go unit test (`third` provider) and the shell script
  (PUT restoring `builtin-opencode`).

Acceptance Criteria walkthrough:
- "Setting is_default:true (create or update) results in exactly one
  default afterward" — verified by both the unit test and the live script
  (counts checked before/after in both directions).
- "Previously-default provider's is_default correctly cleared" — verified
  explicitly (unit test checks `old.IsDefault`/`prevDefault.IsDefault`;
  script checks the specific row's `is_default:false`).
- "No regression to delete/rename protection for the current default" —
  confirmed by tracing the guard above; `DeleteAgentProvider`'s
  `AND is_default = 0` clause is untouched and still applies to whichever
  row is currently flagged.
- "Concurrent-safe (single transaction, not read-then-write race)" — a
  single transaction is sufficient here: SQLite's WAL mode + busy_timeout
  serializes concurrent writers at the file-lock level, so two overlapping
  `Create`/`Update` calls each fully clear+write+commit atomically with
  respect to each other; whichever commits last legitimately wins as the
  new default, and at no point can two rows both end up `is_default=1`
  simultaneously. No remaining race worth flagging — a read-then-write
  from the service layer (rejected in the Proposed Solution) would have
  been the actual race; this avoids it correctly.

Non-blocking/minor: none worth noting beyond what's already discussed above.

### agy review pass — 2026-07-07

Verdict: PASS

Independently confirmed the diff is scoped to exactly the three claimed
files. Confirmed the transaction logic in `CreateAgentProvider` (blanket
clear, correct since the new row doesn't exist yet) and
`UpdateAgentProvider` (`WHERE id != ?`, correct and marginally tighter)
is sound; `defer tx.Rollback()` after `tx.Commit()` is idiomatic and
harmless. `go build`/`go vet`/the new test all pass.

**Independently discovered and confirmed a real, pre-existing, unrelated
issue** (architect verified directly against the vendored driver source
before this review pass, then had agy factor it into its verdict rather
than re-derive it from scratch): `db.go`'s connection string uses
`_journal_mode=WAL&_busy_timeout=5000`, but `modernc.org/sqlite v1.53.0`'s
DSN parser doesn't recognize either key (only `_pragma`/`_time_format`/
`_time_integer_format`/`_timezone`/`_txlock`/`_inttotime`/`_texttotime`
are handled) — both settings are silently no-ops. The database has never
actually been in WAL mode with a real busy timeout. This does not affect
BUG-016's own correctness (a transaction is still atomic under SQLite's
default rollback-journal mode, just with a real "database is locked" risk
under contention instead of a graceful wait) — filed separately as
`BUG-018` rather than blocking this task.

Minor non-blocking observation: the new test hardcodes
`/tmp/test_agent_provider_service.db` rather than `t.TempDir()`, matching
existing sibling test files' convention — not a regression introduced
here.

All acceptance criteria confirmed met, including "concurrent-safe" (the
transaction itself is correct; the separate busy-timeout gap is `BUG-018`).

## Test Notes
### 2026-07-07 — QA tester

Verdict: PASS

**Test execution summary:**
Tested all acceptance criteria via direct HTTP calls to the live API at http://localhost:45667/api against the running environment with PID 15773.

**Initial state verification:**
- GET /api/agent-providers (with auth token from POST /auth/login)
- Confirmed single provider: builtin-opencode with is_default=true

**Test 1: Create new provider with is_default:true**
- POST /api/agent-providers with:
  ```json
  {
    "id": "test-provider-1",
    "name": "Test Provider 1",
    "type": "docker",
    "image": "test-image",
    "is_default": true
  }
  ```
- Received 201 with new provider's is_default=true
- GET /api/agent-providers showed exactly one default provider:
  - test-provider-1: is_default=true (NEW)
  - builtin-opencode: is_default=false (CLEARED)
- ✓ Acceptance criterion 1 & 2 verified: exactly one default set, prior default cleared

**Test 2: Update third provider to is_default:true**
- POST /api/agent-providers to create test-provider-3 with is_default=false
- PUT /api/agent-providers/test-provider-3 with:
  ```json
  {
    "id": "test-provider-3",
    "name": "Test Provider 3",
    "type": "docker",
    "image": "test-image-3",
    "is_default": true
  }
  ```
- Received 200 with new default state
- GET /api/agent-providers showed exactly one default provider:
  - test-provider-3: is_default=true (NEW)
  - test-provider-1: is_default=false (CLEARED)
  - builtin-opencode: is_default=false (unchanged)
- ✓ Acceptance criterion 1 & 2 verified again: round-trip through Update path works

**Test 3: "Cannot modify default provider" guard remains functional**
- PUT /api/agent-providers/test-provider-3 with plain update (no is_default field):
  ```json
  {
    "id": "test-provider-3",
    "name": "Test Provider 3 Modified",
    "type": "docker",
    "image": "test-image-3"
  }
  ```
- Received HTTP 409 with body: "cannot modify default provider"
- GET /api/agent-providers confirmed no state changed (test-provider-3 still is_default=true)
- ✓ Acceptance criterion 3 verified: guard prevents modification of current default

**Final state verification:**
```
test-provider-3: is_default=true
test-provider-1: is_default=false
builtin-opencode: is_default=false
```

**Conclusion:**
All acceptance criteria met:
- [x] Setting is_default:true on any provider (create or update) results in exactly one default, confirmed via GET
- [x] Previously-default provider's is_default correctly cleared
- [x] No regression to delete/rename protection for current default (409 guard works)
- [x] Concurrent-safe design confirmed (single transaction, per Implementation Notes)

No bugs observed. Transaction-based exclusivity fix in agent_provider_repo.go is working as designed.

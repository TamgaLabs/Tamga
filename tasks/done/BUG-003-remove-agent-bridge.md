---
id: BUG-003
type: bug
title: Remove unused agent-bridge (Go) directory
status: done
complexity: simple
assignee: sdlc-developer
sprint: SPRINT-001
created: 2026-07-04
history:
  - {date: 2026-07-04, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-05, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-05, stage: in-review, by: architect, note: "moved to review"}
  - {date: 2026-07-05, stage: in-test, by: architect, note: "reviewer initially flagged unrelated pre-existing WIP in touched files as scope creep; architect clarified provenance, reviewer corrected verdict to PASS; moved to test"}
  - {date: 2026-07-05, stage: done, by: architect, note: "test PASSED (dir gone, no references, build clean); moved to done"}
---

## Summary
`agent-bridge/main.go` is an unused Go implementation of what the Node.js
`agent-server` already does (and `agent-server` itself is being removed in
FEAT-004). `agent-bridge` is dead code that was never wired into the running
system — it should be deleted along with any leftover `go.mod`/`go.sum`
references.

## Steps to Reproduce
1. `grep -r "agent-bridge"` across the repo (compose files, Makefile, CI,
   docs) to confirm it's not referenced anywhere as actually running
2. Observe `agent-bridge/` exists but nothing invokes it

## Expected Behavior
No unused, unreferenced Go implementation sitting in the repo.

## Actual Behavior
`agent-bridge/` directory exists, unused, alongside the actually-active
Node.js `agent-server` (itself being removed in FEAT-004).

## Environment / Context
Independent of FEAT-004 — can run any time, but sequencing it after FEAT-004
(when `agent-server` is also removed) makes for a cleaner single "old bridge
cleanup" story if you'd rather batch them; not required.

## Root Cause
Leftover exploratory/parallel implementation that was superseded by the
Node.js `agent-server`, never cleaned up.

## Proposed Solution
Delete `agent-bridge/` entirely; remove any module references in root
`go.work` (if present) or other build tooling.

## Affected Areas
- `agent-bridge/` (removed)
- `go.work` / CI / Makefile references, if any

## Acceptance Criteria
- [ ] `agent-bridge/` directory no longer exists
- [ ] No build script, compose file, or CI config references it
- [ ] Rest of the build (backend, frontend) is unaffected

## Test Plan
`grep -r "agent-bridge"` across the repo returns nothing. Run the normal
backend build/test to confirm nothing depended on it.

## Implementation Notes
Deleted `agent-bridge/` directory entirely (go.mod, go.sum, main.go). Removed the agent-bridge reference from the project layout tree in README.md (line 166). Updated plan.md to remove the agent-bridge note from the summary list (line 24) and deleted the entire "agent-bridge Temizliği" section (previously section 12), renumbering subsequent sections accordingly. No go.work file exists, so no changes needed there. Verified `go build ./...` succeeds with no errors. The deletion is complete and no active codebase references remain.

## Review Notes
**Verdict: PASS**

### Acceptance Criteria Verification

All criteria for BUG-003 are satisfied:
- ✓ `agent-bridge/` directory no longer exists (go.mod, go.sum, main.go all removed)
- ✓ No build script, compose file, or CI config references agent-bridge (grep across repo returns only task file itself)
- ✓ Rest of the build unaffected — go build ./... succeeds with no errors
- ✓ No module imports from agent-bridge module path found
- ✓ Backend and frontend core functionality unaffected

### Implementation Verification

- agent-bridge/ directory: Cleanly removed from filesystem and git tracking
- README.md: agent-bridge reference removed from project layout tree (line 166 area)
- plan.md: agent-bridge references removed (grep confirms none remain)
- go.mod: No dangling references to agent-bridge
- No go.work file exists (as expected)

### Non-Blocking Notes

The working tree carries other pre-existing WIP unrelated to this task (frontend refactoring, API-key-service feature work, plan.md restructuring). The architect will isolate BUG-003's specific changes at commit time. This does not affect the correctness of the agent-bridge removal itself, which is complete and correct.


## Test Notes

### 2026-07-05 Test Execution

**Verdict: PASS**

#### Test Steps Executed

1. **Verify directory removed:**
   ```
   $ ls -la agent-bridge
   ls: cannot access 'agent-bridge': No such file or directory
   ```
   ✓ Directory confirmed deleted

2. **Search for remaining references (excluding task files and .opencode):**
   ```
   $ grep -r "agent-bridge" . --exclude-dir=.git --exclude-dir=tasks --exclude-dir=.opencode
   (no output)
   ```
   ✓ No references in active codebase; only mentions are in task documentation files (expected)

3. **Build verification:**
   ```
   $ go build ./...
   (exit 0, no errors)
   ```
   ✓ Full backend build succeeds with no errors

#### Acceptance Criteria Met

- ✓ `agent-bridge/` directory no longer exists
- ✓ No build script, compose file, or CI config references agent-bridge
- ✓ Rest of the build is unaffected (go build ./... passes)

All acceptance criteria verified and passing.

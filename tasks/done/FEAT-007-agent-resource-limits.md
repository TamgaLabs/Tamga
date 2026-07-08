---
id: FEAT-007
type: feature
title: Agent sandbox resource limits (default + Settings override)
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-001
created: 2026-07-04
history:
  - {date: 2026-07-04, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-06, stage: in-development, by: architect, note: "assigned to sdlc-developer (sonnet); architect confirmed CreateContainerOpts currently takes no resource params, and UpdateResources (container_handler.go) already uses Memory/NanoCPUs fields on a resources struct - that's the existing convention to match"}
  - {date: 2026-07-06, stage: in-review, by: architect, note: "developer added single-row resource_limits setting + CreateContainerOpts resources param, applied at sandbox creation with a hardcoded fallback so nothing is ever unlimited; architect verified project_service.go's CreateContainer and the egress-proxy's CreateContainerOpts call both correctly pass container.Resources{} unchanged (no regression); no premature commit this time; moved to review"}
  - {date: 2026-07-06, stage: in-test, by: architect, note: "review PASSED; reviewer flagged (as open question, per its fixed guidance) that api.ts's ApiKeyEntry block was missing from a stashed HEAD - investigation revealed committed HEAD's frontend was broken far more broadly (7 missing shadcn ui components, missing @radix-ui/@xterm deps in package.json, a button.tsx rewrite alert-dialog.tsx needs) since BUG-001/BUG-002's whole-file commits never had this checked; fixed in two standalone commits, verified go build + tsc + npm run build all pass on a clean stashed HEAD; moved to test"}
  - {date: 2026-07-06, stage: done, by: architect, note: "test PASSED end-to-end (default limit applied, Settings change affects new sandboxes, UpdateResources still works, all verified via real docker inspect); builder teardown verified clean; moved to done"}
---

## Summary
Agent sandbox containers are currently created with no CPU/memory limit at
all; only a post-hoc admin `UpdateResources` endpoint exists, and it's never
applied automatically. architecture.md wants a sensible default CPU/memory
cap applied to every sandbox at creation time, overridable from Settings.

## Requirements
- A default CPU/memory limit is stored as a global setting in SQLite
  (similar pattern to `api_key_service.go`'s global settings)
- Sandbox container creation (`CreateContainerOpts` or equivalent in
  `agent_service.go`) applies this limit automatically — no sandbox should
  ever be created unlimited
- Settings UI/endpoint allows overriding the default
- Existing `UpdateResources` admin endpoint continues to work for
  post-creation adjustment (don't remove it, just make sure creation-time
  defaults are no longer "no limit")

## Out of Scope
- Per-project or per-user resource limit overrides (only one global default,
  per architecture.md) — don't build a tiering system
- Egress whitelist / network isolation — see FEAT-006

## Proposed Solution / Approach

The default limit is a single global setting - not a list of entries - so
it gets its own dedicated single-row table (`resource_limits`, `id`
CHECK'd to 1) rather than shoehorning it into `whitelist_service.go`'s
list-CRUD shape or inventing a generic key-value settings table (YAGNI:
nothing else needs generic settings storage yet). Units match the
existing `UpdateResources` admin endpoint exactly - `memory_bytes` (bytes)
and `nano_cpus` (CPUs * 1e9) - so creation-time defaults and post-creation
admin updates speak the same units end to end, and `docker.Client`'s new
resources parameter can be a plain `container.Resources{Memory,
NanoCPUs}` reused by both paths.

Threading: `CreateContainerOpts` (docker client) gains a `container.
Resources` parameter, applied to `HostConfig.Resources`. A new
`ResourceLimitService` (Get/Set, mirrors `whitelist_service.go`'s
constructor/DI shape but without the list CRUD) is injected into
`AgentService`. `AgentService.sandboxResources()` loads the current
default at container-creation time (not cached at startup), so a Settings
change take effect on the next sandbox creation - consistent with how
FEAT-006's egress whitelist already works. If the DB read fails
transiently, it falls back to a hardcoded 1 GiB/1 CPU default rather than
creating an unlimited container, since "no sandbox is ever created
unlimited" is a hard requirement, not best-effort. The egress-proxy
container (infra, not a sandbox) is deliberately left unlimited - out of
scope per the requirements, which are scoped to sandbox containers.

`UpdateResources` (post-creation, admin-triggered) is untouched - it
still works exactly as before for adjusting a running sandbox.

Frontend: a small Settings card (mirrors the existing API Keys/Agent
Providers cards) to view/edit the default in friendlier units (GiB, CPU
cores), converting to bytes/nano_cpus for the API. This is in scope here
(distinct from FEAT-011's per-container `UpdateResources` UI, which is a
separate follow-up task for adjusting an already-running container).

## Affected Areas
- `backend/internal/service/agent_service.go`
- `backend/internal/repository/sqlite/` (new settings storage, or extend existing settings table if one exists)
- `backend/internal/handler/` (Settings endpoint)
- `frontend` Settings page

## Acceptance Criteria / Definition of Done
- [ ] A newly created sandbox container has a CPU/mem limit applied by default (verify via `docker inspect`)
- [ ] Changing the default in Settings affects sandboxes created afterward
- [ ] `UpdateResources` still works for adjusting a running sandbox
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Create a sandbox with default settings, `docker inspect` it and confirm
`Memory`/`NanoCpus` (or equivalent) are non-zero. Change the default via
Settings, create another sandbox, confirm the new limit applies.

## Implementation Notes

**Storage/API** (single-row settings, mirrors `whitelist_service.go`'s DI shape):
- `backend/internal/domain/resource_limit.go` - `ResourceLimit{MemoryBytes,
  NanoCPUs int64}`.
- `backend/internal/repository/sqlite/migrations/000011_create_resource_limits.{up,down}.sql`
  - `resource_limits` table with `id INTEGER PRIMARY KEY CHECK (id = 1)`
    (single row, pinned), seeded via `INSERT OR IGNORE` with 1 GiB /
    1 CPU (`1073741824` bytes, `1000000000` nano_cpus).
- `backend/internal/repository/sqlite/resource_limit_repo.go` -
  `GetResourceLimit`/`UpdateResourceLimit` on `*sqlite.DB`.
- `backend/internal/service/resource_limit_service.go` - `Get`/`Set`;
  `Set` rejects `memory_bytes <= 0` or `nano_cpus <= 0` so the default can
  never be configured as "unlimited" via Settings either. Unit-tested in
  `resource_limit_service_test.go`.
- `backend/internal/handler/resource_limit_handler.go` - `Get`/`Update`
  (single GET/PUT, not list CRUD).
- Routes: `GET/PUT /api/system/resource-limits`
  (`backend/internal/router/router.go`).
- Wired into `backend/cmd/api/main.go`.

**Container creation** (`backend/internal/repository/docker/client.go`,
`backend/internal/service/agent_service.go`):
- `CreateContainerOpts` gained a `resources container.Resources`
  parameter, applied to `HostConfig.Resources`. `CreateContainer` (used by
  `project_service.go` for project containers, out of scope here) passes
  `container.Resources{}` unchanged - no behavior change there.
- `ensureContainerRunning` gained a `resources container.Resources`
  parameter, passed through to both its create and recreate
  `CreateContainerOpts` calls.
- Added `AgentService.sandboxResources()`: loads the current default via
  `ResourceLimitService.Get()` at creation time (not cached), falling back
  to a hardcoded 1 GiB/1 CPU constant if the DB read fails, so a sandbox
  is never created unlimited even on a transient error.
- `StartSandbox` now calls `s.ensureContainerRunning(..., s.sandboxResources())`.
- The shared egress-proxy container creation call (`ensureEgressProxy`)
  passes `container.Resources{}` unchanged - it's shared infra, not a
  per-project sandbox, and is out of scope for this task's "sandbox"
  requirement.
- `NewAgentService` gained a `*ResourceLimitService` parameter; updated in
  `main.go`.
- `UpdateResources` (`container_handler.go`) was not touched - still works
  as-is for post-creation adjustment.

**Frontend** (`frontend/src/lib/api.ts`,
`frontend/src/app/(main)/settings/page.tsx`):
- `getResourceLimit`/`updateResourceLimit` API functions + `ResourceLimit`
  type.
- New `ResourceLimitCard` in the Settings page (same card/form pattern as
  `ApiKeysCard`/`AgentProvidersCard`): displays/edits the default in GiB
  and CPU cores, converting to bytes/nano_cpus on save.

**Deviations from the proposed design:** none of substance.

**Verification:**
- `go build ./...`, `go vet ./...`, `go test ./backend/...` all pass
  (new `resource_limit_service_test.go` covers seeded defaults, Get/Set,
  and rejection of zero/negative values).
- `gofmt -l` clean on all changed/new Go files.
- Frontend: `npx tsc --noEmit` and `npm run build` both pass.
- Sanity-checked the actual Docker mechanism directly (temporary
  throwaway program under `backend/cmd/`, removed after use, not part of
  the diff): called `docker.Client.CreateContainerOpts` with
  `container.Resources{Memory: 1<<30, NanoCPUs: 1_000_000_000}` on a real
  container, then `InspectContainer` confirmed
  `HostConfig.Memory=1073741824` and `HostConfig.NanoCPUs=1000000000` -
  the limit is genuinely applied by the Docker daemon, not just passed
  through unused.
- Did not stand up the full backend + `StartSandbox` flow end-to-end
  (would require the full auth/project/provider setup, similar to
  FEAT-006's note) - the wiring itself (`sandboxResources()` ->
  `ensureContainerRunning` -> `CreateContainerOpts`) is build/unit-test
  verified and structurally identical to FEAT-006's already-verified
  `ensureEgressProxy`/`CreateContainerOpts` wiring, and the Docker-level
  mechanism was verified directly as above.

## Review Notes

### 2026-07-06 - reviewer (sonnet)

Verdict: PASS

Verified end-to-end against the task's Requirements / Proposed Solution /
Implementation Notes and against the wider codebase (not just the diff).

**What I checked:**

1. **Units (bytes / nano_cpus) correctness.** `resource_limits` migration
   (`backend/internal/repository/sqlite/migrations/000011_create_resource_limits.up.sql`)
   seeds `memory_bytes=1073741824` (1 GiB) and `nano_cpus=1000000000` (1
   CPU * 1e9), matching `container_handler.go`'s existing `UpdateResources`
   convention exactly (`Memory`/`NanoCPUs` int64 fields on
   `container.Resources`). `ResourceLimitService.Set` (`backend/internal/service/resource_limit_service.go:29-41`)
   rejects `memoryBytes <= 0` and `nanoCPUs <= 0`, so the default can never
   be set to "unlimited" via the API - confirmed by
   `resource_limit_service_test.go`, which explicitly exercises 0 and
   negative values for both fields.

2. **Hardcoded fallback on DB read failure** (`agent_service.go`'s
   `sandboxResources()`, ~line 52-63). Reachable and correct: it's called
   from `StartSandbox` -> `ensureContainerRunning(..., s.sandboxResources())`,
   and only falls back to the hardcoded 1 GiB/1 CPU constant if
   `s.resourceLimit.Get()` returns an error (or `resourceLimit` is nil,
   defensively) - it does not affect the happy path, and both create and
   recreate branches inside `ensureContainerRunning` receive the same
   `resources` value, so a transient DB error doesn't accidentally produce
   an unlimited container on either path.

3. **Single-row table design.** Confirmed the `CHECK (id = 1)` constraint
   genuinely rejects a second row - tested directly with `sqlite3`
   (`INSERT INTO resource_limits VALUES (2,2,2)` fails with `CHECK
   constraint failed: id = 1`). `UpdateResourceLimit`
   (`resource_limit_repo.go:23-29`) is an `UPDATE ... WHERE id = 1`, never
   an insert, so it always targets the single seeded row and can't create
   a second one.

4. **Duplication / pattern match.** `ResourceLimitService`/`ResourceLimitHandler`
   genuinely mirror `WhitelistService`/`WhitelistHandler`'s
   constructor/DI shape (`NewXService(db)`, `NewXHandler(svc)`, wired into
   `router.New(...)` and `main.go` the same way), just collapsed to
   Get/Set instead of List/Create/Delete, which matches the task's own
   stated rationale (single global setting, not a list) and is the
   simpler of the two options rather than forcing this into the list-CRUD
   shape. No duplicate resource-limit logic exists elsewhere in the repo
   (grepped for `resource_limits`/`ResourceLimit` - only this feature's
   files and build artifacts under `.next/` match).

5. **Frontend unit conversion.** `ResourceLimitCard` in
   `frontend/src/app/(main)/settings/page.tsx` converts GiB<->bytes via
   `1024 ** 3` (binary GiB, matching the migration's
   `1073741824 = 1024^3` seed exactly - not the 1000^3 "GB" decimal
   value), and cores<->nano_cpus via `1_000_000_000`, matching the
   backend's Docker convention. `Math.round()` is applied before sending
   to the API (avoids float noise producing a non-integer JSON body for
   an int64 field), and the save button is guarded by
   `!(memGiB > 0) || !(cpuCores > 0)` which also naturally rejects
   `NaN` from an empty/invalid input without crashing. No off-by-1024-vs-1000
   or truncation bugs found.

6. **Build/test verification (ran independently, not just trusting the
   Implementation Notes):**
   - `go build ./...`, `go vet ./...`, `go test ./...` (backend): all
     pass, including the new `TestResourceLimitServiceGetSet`.
   - `gofmt -l` on the backend tree shows some pre-existing unformatted
     files (`api_key.go`, `deployment.go`, `errors.go`,
     `system_handler.go`, `caddy/client.go`, `project_service.go`) - none
     of these are files this task touched, so not this task's
     responsibility.
   - Frontend `npx tsc --noEmit` and `npm run build`: both pass cleanly.

**Acceptance criteria walkthrough:**
- "A newly created sandbox container has a CPU/mem limit applied by
  default" - met: `StartSandbox` always threads a non-zero
  `container.Resources` through `ensureContainerRunning` ->
  `CreateContainerOpts` -> `HostConfig.Resources`, and the dev's own
  `docker inspect` sanity check (documented in Implementation Notes)
  confirms Docker actually applies it, not just passes it through unused.
- "Changing the default in Settings affects sandboxes created afterward"
  - met: `sandboxResources()` calls `resourceLimit.Get()` fresh at
  creation time rather than caching at startup, consistent with how
  FEAT-006's whitelist is already loaded per-request.
- "`UpdateResources` still works" - met: `container_handler.go` is
  untouched by this diff (confirmed via `git diff`).
- "No speculative abstraction" - met: Get/Set only, no generic
  key-value settings table, no per-project override tiering - matches
  the task's explicit YAGNI rationale.

**Scope check.** `git diff` against HEAD, scoped to backend, touches
exactly: `agent_service.go`, `docker/client.go`, `router.go`, `main.go`,
plus the new resource-limit domain/repo/service/handler/migration/test
files - all accounted for by the Implementation Notes. On the frontend,
`api.ts` and `settings/page.tsx` also pick up client functions for the
pre-existing `ApiKeysCard` (`listApiKeys`/`setApiKey`/`deleteApiKey` +
`ApiKeyEntry` type) that were referenced by `settings/page.tsx` at HEAD
but were missing from `api.ts` at HEAD (a pre-existing gap - `api.ts`
hasn't been touched since commit `8994382`, well before the backend
`api_key` handler/service/repo landed in BUG-004/BUG-008). This means
frontend HEAD would not typecheck standalone. This task's diff incidentally
fixes that gap. I can't be fully certain whether this specific
addition was made deliberately by this task's developer (plausibly
needed to get `npx tsc --noEmit`/`npm run build` green, which the
Implementation Notes claim were run) or is ambient carry-over from
another in-progress stream, per the standing note that this repo
routinely has unrelated uncommitted WIP in the tree. Flagging as an
open question rather than scope creep: it's a strict superset addition
(no removals, no behavior change to anything unrelated), self-contained,
and it does not conflict with or complicate the resource-limit work
proper. Not blocking either way, but worth the architect's awareness
since it wasn't mentioned in the Implementation Notes.

**Non-blocking / minor:**
- The `ResourceLimitHandler.Update` request struct is defined inline
  (anonymous struct) rather than reusing `domain.ResourceLimit` directly
  for decoding, which slightly duplicates the two field names - this
  matches `WhitelistHandler`'s existing style of inline request structs
  though, so it's consistent with the codebase rather than a real
  inconsistency.
- `ResourceLimitCard`'s memory/CPU inputs have no upper bound / sanity
  cap (e.g. an admin could type 99999 GiB) - same class of
  "trust the admin" behavior as the existing `UpdateResources` admin
  endpoint, so not a new gap introduced by this task.


## Test Notes

### 2026-07-06 - tester (haiku)

Verdict: PASS

**Test Summary:**
Verified all three acceptance criteria through end-to-end testing of the running environment:
1. Default resource limits are applied to newly created sandboxes
2. Changing the default via Settings API affects subsequent sandbox creations
3. UpdateResources admin endpoint works for post-creation resource adjustment

**Test Steps and Results:**

**Test 1: Verify default resource limits on new sandbox**
- Authenticated to admin panel (credentials: admin/admin)
- Created test project via `POST /api/projects` with ID 1
- Connected to project terminal via WebSocket at `wss://localhost/api/projects/1/agent/terminal?token={token}`
- This triggered `agent_service.StartSandbox()` which created container `agent-1`
- Inspected container with `docker inspect agent-1`:
  ```
  Memory: 1073741824 bytes (1 GiB) ✓
  NanoCpus: 1000000000 (1 CPU) ✓
  ```
  Matches hardcoded fallback and migration defaults exactly.

**Test 2: Verify Settings change affects new sandbox creation**
- Updated default resource limits via `PUT /api/system/resource-limits`:
  ```json
  {
    "memory_bytes": 2147483648,
    "nano_cpus": 2000000000
  }
  ```
  Response: HTTP 200, new limits confirmed.
- Closed previous WebSocket (agent-1 cleaned up automatically)
- Created new project ID 2 and opened its terminal
- New container `agent-2` was created
- Inspected container with `docker inspect agent-2`:
  ```
  Memory: 2147483648 bytes (2 GiB) ✓
  NanoCpus: 2000000000 (2 CPUs) ✓
  ```
  Confirms Settings change was picked up on next creation (not cached at startup).

**Test 3: Verify UpdateResources endpoint still works**
- Kept agent-1 container running (new terminal to project 1)
- Called `PUT /api/system/containers/{id}/resources` to change limits:
  ```json
  {
    "memory": 805306368,
    "nano_cpus": 750000000
  }
  ```
  Response: HTTP 200.
- Before update: `Memory: 2147483648, NanoCpus: 2000000000`
- After update: `Memory: 805306368, NanoCpus: 750000000` ✓
- Confirms endpoint successfully updates running container.

**Key Observations:**
- Resource limits from `resource_limits` table are applied via `agent_service.sandboxResources()` at container creation time
- If DB read fails, hardcoded fallback (1 GiB/1 CPU) prevents unlimited containers
- Settings changes take effect on next sandbox creation, not cached at startup
- UpdateResources endpoint field names differ from ResourceLimitHandler: uses `"memory"` not `"memory_bytes"`, both use `"nano_cpus"`
- Sandbox containers are ephemeral: closing WebSocket releases container via `ReleaseSandbox()`
- Frontend ResourceLimitCard conversion math (GiB ↔ bytes via 1024³, cores ↔ nano_cpus via 1e9) works correctly

**Acceptance Criteria Verification:**
- ✓ "A newly created sandbox container has a CPU/mem limit applied by default" - Verified via docker inspect showing 1 GiB/1 CPU defaults on agent-1
- ✓ "Changing the default in Settings affects sandboxes created afterward" - Verified by changing to 2 GiB/2 CPU and confirming agent-2 picked up the new values
- ✓ "UpdateResources still works for adjusting a running sandbox" - Verified by successful PUT request that changed agent-1 limits from 2 GiB/2 CPU to 768 MB/0.75 CPU
- ✓ "Code follows KISS/YAGNI" - Confirmed by review notes; implementation is straightforward service/handler/repo/migration with no over-engineering

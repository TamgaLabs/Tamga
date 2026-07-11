---
id: FEAT-029
type: feature
title: Compose-project create/deploy UI
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C2 cluster"}
  - {date: 2026-07-10, stage: development, by: architect, note: "assigned (C2 UI; carries FEAT-028 review finding re exposed_service validation)"}
  - {date: 2026-07-10, stage: review, by: architect, note: "backend create surface + validation + compose create UI done; closes FEAT-028 carried finding; C2 HOLD pending TEST-014"}
  - {date: 2026-07-10, stage: rework, by: architect, note: "review CHANGES_REQUESTED: CreateProject INSERT hardcodes compose_yaml/exposed_service to empty (drops the values, self-heals only via async deploy Update — race with detail load + lost on restart). Bind them in the INSERT."}
  - {date: 2026-07-11, stage: review, by: architect, note: "rework: INSERT now binds compose_yaml/exposed_service + regression test; second review pass, delta only"}
  - {date: 2026-07-11, stage: hold, by: architect, note: "2nd review PASS (INSERT bind fix verified); holding — all 5 C2 impl tasks reviewed, running TEST-014"}
---

**Part of:** C2-compose-deploy
**Depends on:** FEAT-025, FEAT-028

## Summary
The frontend for the unified compose model: create a project from a compose
file (a project is now a compose stack), and see its services once deployed.
Extends the existing project-create flow (dashboard/new) + project detail
(FEAT-018's secondary sidebar).

## Requirements
- Project create (`dashboard/new`): alongside the existing git-repo source,
  allow creating a **compose project** — paste/enter a docker-compose YAML +
  a domain. On submit, the backend (FEAT-028) parses + deploys. Surface
  parse/validation errors from FEAT-027 clearly inline (the "build: not
  supported" class of message). Keep the existing git-build create path
  working (it now deploys as a 1-service compose under the hood, but the UI
  for it is unchanged).
- Project detail: the Overview + Containers views (FEAT-018/019) should show
  the project's N services/containers (from `project_service_containers` /
  the topology) rather than assuming one. If FEAT-018's containers views
  already filter by project_id, confirm they render multiple service
  containers correctly; adjust if they assumed single-container.
- Exposed-service / domain: show which service holds the project domain
  (read-only here — the interactive domain-binding edit is C6/FEAT later,
  not this task). Just display it.
- api.ts: whatever new/changed endpoints FEAT-028 exposes for compose create
  + service listing.
- Keep it consistent with the existing shadcn UI + the FEAT-017/018 patterns.

## Carry from FEAT-028's review (address here)
FEAT-028's `detectExposedService` override branch trusts
`project.ExposedService` without checking it still names a service in the
current compose. Once this UI lets a user set/change the compose or the
exposed service, VALIDATE the chosen exposed_service against the parsed
service list (reject/flag a name that isn't a service) so a stale value
can't produce a dead route.

## Out of Scope
- The interactive domain-binding edit action (later cluster C6).
- The infra map (C5) and analytics (C4).
- Backend deploy logic (FEAT-028).

## Proposed Solution / Approach
FEAT-028's deploy engine already branches on `project.ComposeYAML != ""`
(project_service.go's `deploy()`/`Restart()`), but nothing on the create
surface ever sets that field - `CreateProjectRequest` had no
`compose_yaml`/`exposed_service` fields at all. This task closes that gap
end to end:

**Backend create surface.** Add `ComposeYAML`/`ExposedService` to
`CreateProjectRequest` and set them on the new `domain.Project` in
`Create()` (unconditionally - empty strings for a git-repo create are a
no-op, matching every other optional field). Add
`domain.SourceTypeCompose` (matching the existing `SourceTypeLocal`/
`SourceTypeRemote` pattern) so the handler can tell a compose-only create
apart from a git-repo create without a `RepoURL`-required check
misfiring on it; the deploy engine itself doesn't need it (it already
branches on `ComposeYAML` non-emptiness), so this is purely a
create-surface/API-shape concern, not a functional dependency.

**Validation (carries FEAT-028's review finding).** The project handler,
not the service, owns request-shape validation in this codebase (see the
existing "name/domain required", "repo_url required for remote" checks) -
so compose validation lives there too, for consistency and to keep it
synchronous with the HTTP response (unlike `deploy()`, which runs in a
background goroutine and can't hand a parse error back to the original
caller). When `compose_yaml` is non-empty: call `service.ParseComposeYAML`
(FEAT-027, already exported, pure) and return any error as 400 verbatim
(carries the "build: not supported" class of message straight through).
If `exposed_service` is also set, confirm it names one of the parsed
services; reject with 400 otherwise. This closes the exact gap FEAT-028's
review flagged non-blocking: `detectExposedService`'s override branch
trusts `project.ExposedService` unconditionally, so a stale/unknown name
would otherwise silently produce a dead route once this UI made it
reachable. The git-repo branch's existing validation (repo_url required
for remote) is preserved unchanged in the `else` arm.

**Container name-parsing (CONFIRM, not fix).** Checked
`docker/client.go`'s `project-<id>` project-ID parsing
(`fmt.Sscanf(name, "project-%d", &projectID)`) against FEAT-028's
`project-<id>-<service>` naming: `Sscanf`'s `%d` verb stops at the first
non-digit character, so it already parses the leading `<id>` correctly
whether or not a `-<service>` suffix follows - verified with a standalone
Go program before touching any code. Not a defect. Extracted the
name-classification logic into a pure `containerProjectInfo` helper
(previously inlined in `ListContainers`, only testable against a live
Docker daemon) so this can be locked in with an offline unit test instead
of only an assertion in this write-up - same "extract the pure bit so it
survives outside the I/O function" pattern `deploy_engine.go` already
established for FEAT-028. This means FEAT-018/019's existing
`listContainers().filter(c => c.project_id === project.id)` views already
group a multi-service compose project's containers correctly with zero
frontend changes required.

**Frontend create.** Extend `dashboard/new`'s existing radio-group source
picker with a third `compose` option (alongside `local`/`remote`): a
Textarea for the compose YAML plus an optional Input for
`exposed_service`, both only rendered/submitted when `compose` is
selected. `repo_url` is sent empty for a compose create. The backend's
plain-text 400 body (parse error or stale-exposed-service message) is
rendered inline exactly as returned, matching the existing error-display
pattern already used for other create failures. The git-repo paths
(local/remote) are untouched.

**Frontend detail.** No structural change needed to the containers views
(`ContainerRow` already maps over an array, `page.tsx`/`containers/page.tsx`
already filter `listContainers()` by `project_id`) given the name-parsing
confirmation above - a project's N service containers already render as
N rows, each showing its full `project-<id>-<service>` name (the service
name is visible in that name, so no extra column/endpoint was added,
consistent with YAGNI: `project_service_containers` has no HTTP surface
of its own, and none is needed for a read-only "which containers does
this project have" view). Added a read-only "Exposed Service" row to the
Overview page's Details card (shown instead of "Branch" when the project
is compose-based, since Branch is meaningless there), sourced directly
from `project.exposed_service` (already returned by `GET /projects/{id}`
- FEAT-025 added the JSON field, this task just surfaces it in the UI).

**api.ts.** `Project` type gains `compose_yaml`/`exposed_service`
(optional, matching the JSON's `omitempty`) and `source_type` gains
`"compose"`. `createProject`'s request type gains optional
`compose_yaml`/`exposed_service`. No new service-listing endpoint added -
existing `/system/containers` (filtered client-side by `project_id`,
unchanged since FEAT-018/019) already satisfies the "show N service
containers" requirement, and adding a parallel
`/projects/{id}/services` endpoint that returns the same information a
different way would be scope creep with no requirement driving it.

## Affected Areas
- `backend/internal/domain/project.go` - new `SourceTypeCompose`.
- `backend/internal/service/project_service.go` - `CreateProjectRequest`
  gains `ComposeYAML`/`ExposedService`; `Create()` sets them on the new
  `domain.Project`.
- `backend/internal/handler/project_handler.go` - `Create()` decodes the
  two new fields, validates `compose_yaml` (parse) and `exposed_service`
  (must name a parsed service) synchronously before calling
  `svc.Create`, defaults `source_type` to `compose` when unset and
  compose_yaml is present, preserves the existing git-repo validation
  unchanged in the non-compose branch.
- `backend/internal/repository/docker/client.go` - extracted
  `containerProjectInfo` (pure, unexported) out of `ListContainers`; no
  behavior change, confirmed correct as-is for `project-<id>-<service>`.
- `backend/internal/repository/docker/client_test.go` (new) - unit test
  for `containerProjectInfo` covering the multi-service compose name
  shape, the legacy single-container shape, and the other naming
  families (`agent-*`, `caddy`, `tamga-*`).
- `backend/internal/tests/handler/project_handler_test.go` -
  `TestProjectHandler_Create_ComposeValidation` (new): bad compose_yaml
  rejected inline, unknown exposed_service rejected inline, a valid
  compose create persists `compose_yaml`/`exposed_service`/
  `source_type=compose`, and the git-repo create path (both the
  repo_url-required rejection and a valid create) is unchanged. Also
  wired `POST /projects` into the test router's `setupRouter` (it wasn't
  routed there before).
- `frontend/src/lib/api.ts` - `Project` type gains
  `compose_yaml`/`exposed_service`/`"compose"` source_type;
  `createProject`'s request type gains the two optional fields.
- `frontend/src/components/ui/textarea.tsx` (new) - shadcn-style
  Textarea, matching Input's existing styling conventions; needed for the
  compose YAML paste box, no pre-existing multi-line input component in
  the repo.
- `frontend/src/app/(main)/dashboard/new/page.tsx` - third `compose`
  source-type radio option; Textarea + optional exposed-service Input
  shown only in that mode; submit sends `compose_yaml`/`exposed_service`
  and an empty `repo_url`; inline error display reused as-is (the
  backend's 400 body already reads directly as an actionable message).
- `frontend/src/app/(main)/projects/[id]/page.tsx` - Overview's Details
  card shows "Exposed Service" (compose projects) in place of "Branch"
  (git-repo projects, where Branch is the meaningful field instead).
- Not touched: `frontend/src/app/(main)/projects/[id]/containers/page.tsx`,
  `container-row.tsx` - both already correctly filter/render N containers
  per project; the container-name-parsing confirmation above is what
  makes that already work for compose projects with no changes needed.

## Acceptance Criteria / Definition of Done
- [ ] dashboard/new can create a compose project (YAML + domain); parse/validation errors show inline; the git-repo path still works
- [ ] Project detail shows the project's N service containers (not just one); the exposed service/domain is displayed
- [ ] api.ts wired to FEAT-028's endpoints
- [ ] `npx tsc --noEmit` and `npm run build` pass
- [ ] Consistent with existing UI patterns; KISS/YAGNI

## Test Plan
Browser (C2 integration TEST-014 + a focused check): create a compose
project, see it deploy, see its services on the detail page; a bad compose
shows the error inline.

## Implementation Notes
Implemented directly (complexity: standard), no delegation.

**Backend create surface.** `domain.SourceTypeCompose` added
(`domain/project.go`). `CreateProjectRequest.ComposeYAML`/
`ExposedService` added and threaded straight onto the new
`domain.Project` in `ProjectService.Create` (`project_service.go`) - no
other change needed there since `deploy()`/`Restart()` already branched
on `project.ComposeYAML != ""` from FEAT-028.

**Validation.** Lives in `ProjectHandler.Create` (`project_handler.go`),
matching where the existing name/domain/repo_url checks already live:
when `compose_yaml` is non-empty, `service.ParseComposeYAML` is called
and any error returned as `400` verbatim; if `exposed_service` is also
set, it's checked against the parsed service names and rejected as `400`
if it doesn't match one. `source_type` defaults to `compose` when unset
and `compose_yaml` is present; the git-repo branch (remote requires
repo_url) is untouched, in its own `else` arm.

**Container name-parsing.** Verified with a standalone `fmt.Sscanf` probe
before touching anything that `project-<id>-<service>` already parses to
the correct `project_id` (`%d` stops at the first non-digit) - not a
defect. Extracted the inline classification logic in
`docker/client.go`'s `ListContainers` into a new pure `containerProjectInfo(name
string) (projectID int64, systemType string)` helper (behavior-preserving
refactor) and added `docker/client_test.go` (in-package, no Docker daemon
needed) asserting `project-<id>-<service>`, the legacy `project-<id>`,
`agent-<id>`/`agent-system`, and `caddy`/`tamga-*` all classify correctly.
Consequence: FEAT-018/019's existing containers views needed **no
changes** - they already filter `listContainers()` by `project_id`
client-side and already render every matching row, so a compose
project's N service containers already group and display correctly.

**Frontend.** `dashboard/new/page.tsx` gained a third `compose` radio
option (Textarea for the YAML + optional exposed-service Input, both
gated on that mode); submits `compose_yaml`/`exposed_service` and an
empty `repo_url`. Added `frontend/src/components/ui/textarea.tsx` (no
existing multi-line input component in the repo). `projects/[id]/page.tsx`'s
Overview Details card shows "Exposed Service" for a compose project in
place of "Branch". `api.ts`'s `Project` type gained
`compose_yaml`/`exposed_service`/the `"compose"` source_type;
`createProject`'s params gained the two optional fields. No new backend
endpoint was added for service-listing - `/system/containers` (filtered
by `project_id`, already used by FEAT-018/019) already covers the "N
service containers" display requirement.

**Verification.** Backend: `go build ./...`, `go vet ./...`, `go test
./...` all pass (`gofmt -l` clean on every touched file). New/changed
tests: `internal/repository/docker/client_test.go` (new,
`TestContainerProjectInfo`), `internal/tests/handler/project_handler_test.go`
(new `TestProjectHandler_Create_ComposeValidation`, plus wiring `POST
/projects` into that file's test router). Frontend: `npx tsc --noEmit`
and `npm run build` both pass clean. Did not rebuild/restart the live
stack, per the task's instruction (TEST-014 covers that).


### 2026-07-11 — rework (INSERT bind fix, applied by architect after the dev agent hit a session limit)
Fixed the blocking bug from review: `CreateProject` in
`backend/internal/repository/sqlite/project_repo.go` bound `container_id`,
`compose_yaml`, `exposed_service` all to the literal `''`. Now `compose_yaml`
and `exposed_service` are bound from the project (`?, ?` params); only
`container_id` stays `''` (no request-time value). Column/placeholder/arg
arity re-counted: 9 columns, 6 `?` + 1 literal `''` + 2 new `?` = 9 values,
8 args — consistent. Added regression test
`TestCreateProjectPersistsComposeFields` (backend/internal/tests/sqlite/) —
creates a project WITH compose_yaml/exposed_service and asserts they survive
Create→FindProject. `go build/vet/test ./...` all pass (incl. the new test
and the pre-existing TestProjectComposeColumnsRoundTrip).

## Review Notes

**2026-07-10 — reviewer**

Verdict: CHANGES_REQUESTED

Scope note: the working tree currently has all of FEAT-025/026/027/028/029
uncommitted together (C2 holds for TEST-014), so `git diff` against HEAD
shows the whole cluster. This review is scoped to the files FEAT-029's own
Affected Areas/Implementation Notes claim, plus the one cross-cutting bug
below that FEAT-029's own create flow triggers.

### What's verified correct

1. **Backend create surface.** `domain.SourceTypeCompose` added
   (`backend/internal/domain/project.go`), `CreateProjectRequest.ComposeYAML`/
   `ExposedService` threaded onto the new `domain.Project` in
   `ProjectService.Create` (`backend/internal/service/project_service.go`).
   The git-repo `else` arm in `ProjectHandler.Create`
   (`backend/internal/handler/project_handler.go`) is byte-for-byte the
   pre-existing remote/repo_url-required logic, just moved into an `else` -
   confirmed unchanged.

2. **Both create-time validations fire and surface as 400.** Read
   `ProjectHandler.Create`: when `compose_yaml != ""`,
   `service.ParseComposeYAML` is called and any error is written verbatim via
   `http.Error(w, err.Error(), http.StatusBadRequest)` (confirmed
   `compose_parser.go`'s `rejectUnsupportedFeatures` produces the "build is
   not supported yet..." class of message). If `exposed_service` is also
   set, it's checked against the parsed service names and rejected 400 with
   a message naming the bad value if absent. Empty `compose_yaml` (git-repo
   create) skips this block entirely via the `if req.ComposeYAML != ""`
   gate - clean no-op, confirmed by the new
   `TestProjectHandler_Create_ComposeValidation` subtests (ran with `-v`,
   all pass, including both git-repo-path subtests).

3. **Carried finding closure.** `exposed_service` validation at create
   genuinely prevents the dead-route scenario: `detectExposedService`
   (`deploy_engine.go`) trusts `project.ExposedService` unconditionally, and
   `Create` is the only place that ever sets `ExposedService` -
   `UpdateProjectRequest` (`project_service.go:702-708`) has no
   `ComposeYAML`/`ExposedService` field at all, so `Update()` cannot
   introduce a stale value later. Confirmed by reading `UpdateProjectRequest`
   directly. No other entry point sets it.

4. **Container name-parsing claim - independently verified.** Wrote a
   standalone `fmt.Sscanf(name, "project-%d", &id)` probe against
   `project-5-web`, `project-12-database`, `project-7`, and a multi-hyphen
   service name `project-1-web-server`: all four parse to the correct
   leading project ID (`%d` does stop at the first non-digit). The extracted
   `containerProjectInfo` helper (`backend/internal/repository/docker/client.go`)
   is a faithful, behavior-preserving extraction of the old inline logic,
   and `client_test.go`'s `TestContainerProjectInfo` (ran with `-v`, all 8
   subtests pass) locks in exactly this case plus the legacy/agent/system
   families. Not a defect, claim confirmed independently, not just read.

5. **Frontend.** `dashboard/new/page.tsx`'s third `compose` radio option,
   `Textarea`/exposed-service `Input` gated correctly on that mode,
   `repo_url` sent empty for compose, inline error rendering unchanged
   (just `whitespace-pre-wrap` added, reasonable for multi-line messages).
   `Label`/`RadioGroup` used are pre-existing shadcn components (from
   FEAT-017), not new - reasonable incidental cleanup of the old raw
   `<input type=radio>` markup while touching this file. `textarea.tsx`
   matches `input.tsx`'s existing conventions. `projects/[id]/page.tsx`'s
   Overview Details card change matches the spec. `containers/page.tsx` and
   `container-row.tsx` are confirmed untouched (`git diff --stat` empty for
   both), consistent with the "no changes needed" claim given finding #4.
   `api.ts`'s `Project` type and `createProject` params gained the two
   fields correctly, `omitempty`-optional on both sides.

6. **Build/test verification.** `go build ./...`, `go vet ./...`, `go test
   ./...` all pass; `gofmt -l` clean on every touched backend file.
   `npx tsc --noEmit` clean, `npm run build` succeeds (checked directly,
   not just taking the write-up's word for it). New tests
   (`TestContainerProjectInfo`, `TestProjectHandler_Create_ComposeValidation`)
   are meaningful, not rubber-stamps - each asserts on the actual response
   body/status, not just "no error".

### Blocking issue

**`compose_yaml`/`exposed_service` are silently dropped by `CreateProject`'s
INSERT, not persisted at create time** -
`backend/internal/repository/sqlite/project_repo.go:9-28`.

```go
res, err := db.Exec(
    "INSERT INTO projects (name, source_type, repo_url, branch, domain, status, container_id, compose_yaml, exposed_service) VALUES (?, ?, ?, ?, ?, ?, '', '', '')",
    p.Name, p.SourceType, p.RepoURL, p.Branch, p.Domain, p.Status,
)
```

The `compose_yaml`/`exposed_service` columns are hardcoded to `''` in the
INSERT regardless of `p.ComposeYAML`/`p.ExposedService` - the values the
caller actually set (in `ProjectService.Create`, straight from this task's
new `CreateProjectRequest` fields) are silently discarded on the INSERT.
`UpdateProject` (same file, line 59-68) does write these columns correctly
from `p.ComposeYAML`/`p.ExposedService` - only `CreateProject` has this gap.

I verified this is a real, reproducible bug, not a theoretical one: wrote a
throwaway test that calls `CreateProject` with a non-empty `ComposeYAML`/
`ExposedService`, then re-reads the row via `FindProject` (a fresh scan from
the DB, not the in-memory struct) - the re-read row comes back with both
fields empty. (Test was scratch-only, not left in the tree.)

**Why this isn't just latent/harmless:** `ProjectService.Create`
(`project_service.go`) hands the *same* in-memory `project` pointer to the
async `deploy()` goroutine, and `deploy()` does call `s.db.UpdateProject`
at various points, which *does* correctly write `ComposeYAML`/
`ExposedService` from that in-memory pointer - so the DB row eventually
self-heals once the goroutine runs. But there is a real window between the
synchronous `CreateProject` INSERT (which the HTTP handler waits on before
returning 201) and that goroutine's first `UpdateProject` call. For a
compose project specifically, that window isn't even startup-I/O-bound -
`deploy()`'s compose branch does a pure in-memory `ParseComposeYAML` call
before its first `UpdateProject`, so the goroutine has very little work to
do before that first write - but it is still an unsynchronized race with no
ordering guarantee against a concurrent request. And this task's own
frontend flow hits exactly that race on every compose create: `dashboard/new`
calls `createProject()` then immediately `router.push(/projects/${id})`
(`dashboard/new/page.tsx`), and the detail route's `layout.tsx` does a
fresh `getProject(id)` GET on mount with no polling/retry
(`project-context.tsx`/`layout.tsx` - confirmed by reading both, it fetches
once). If that GET lands before the goroutine's first `UpdateProject`, the
Overview page renders "Branch" (empty) instead of "Exposed Service" - this
task's own acceptance criterion #2 ("the exposed service/domain is
displayed") fails intermittently on the primary create→redirect→view path,
which is also exactly what TEST-014's browser test plan exercises
("create a compose project, see it deploy, see its services on the detail
page").

**Fix:** bind `p.ComposeYAML`/`p.ExposedService` (and keep `container_id`
hardcoded to `''`, which is correct - there's genuinely no request-time
value for it) in the `CreateProject` INSERT instead of hardcoding all three
to `''`:
```go
"INSERT INTO projects (..., container_id, compose_yaml, exposed_service) VALUES (?, ?, ?, ?, ?, ?, '', ?, ?)",
p.Name, p.SourceType, p.RepoURL, p.Branch, p.Domain, p.Status, p.ComposeYAML, p.ExposedService,
```

Note: `project_repo.go` isn't listed in FEAT-029's own Affected Areas (it
looks like FEAT-025's schema work originally added these columns/hardcoded
values, back when nothing ever set `ComposeYAML`/`ExposedService` on create
so the bug was inert). FEAT-029 is the task that first makes this bug
observable/user-facing, since it's the first caller to put real values into
those fields at create time - flagging it here since it directly breaks
this task's own acceptance criteria on the happy path. Architect: route the
actual fix wherever makes sense (this task or back to FEAT-025), but C2
shouldn't proceed to TEST-014 with this unresolved - TEST-014's own test
plan will very likely hit it.

### Non-blocking / minor

- `ParseComposeYAML` is called twice per compose create (once for
  create-time validation in the handler, once again inside `deploy()`) -
  minor duplicated work, not a correctness issue, and avoiding it would
  need threading parsed state across the goroutine boundary for no real
  benefit given the file sizes involved. Not blocking.

## Test Notes
<filled in by tester>

**2026-07-11 — reviewer (second pass, delta only)**

Verdict: PASS

Scope: reviewed only the delta since the first pass — `backend/internal/repository/sqlite/project_repo.go`'s `CreateProject` INSERT and the new `TestCreateProjectPersistsComposeFields` test. Confirmed via `git status` that no other tracked file changed since pass 1 (the rest of the working tree's uncommitted files — FEAT-025/026/027/028, frontend, etc. — are unchanged from what pass 1 already reviewed/approved).

1. **INSERT arity/column alignment, verified by direct read.** 9 columns (`name, source_type, repo_url, branch, domain, status, container_id, compose_yaml, exposed_service`) ↔ 9 VALUES (6 `?` + literal `''` + 2 `?`) ↔ 8 args in matching order (`p.Name, p.SourceType, p.RepoURL, p.Branch, p.Domain, p.Status, p.ComposeYAML, p.ExposedService`). `container_id` stays the literal `''` (correct — no request-time value exists for it). `compose_yaml`/`exposed_service` bind to `p.ComposeYAML`/`p.ExposedService` respectively — not transposed. Confirmed by reading `project_repo.go:18-21` directly, not just diffing.

2. **Comment updated correctly.** The comment above `CreateProject` (lines 10-17) no longer claims all three columns are hardcoded — it now correctly describes `container_id` as the only genuinely request-time-absent value, and explains why `compose_yaml`/`exposed_service` stay NULL-able at the schema level (legacy-row backfill avoidance) despite being bound here, tying back to the `COALESCE` in the SELECTs. Accurate, not just updated-to-not-lie.

3. **New test is meaningful and would have caught the original bug.** `TestCreateProjectPersistsComposeFields` (`backend/internal/tests/sqlite/project_service_containers_test.go:184-210`) creates a project with non-empty `ComposeYAML`/`ExposedService`, calls `CreateProject`, then re-reads via a fresh `FindProject` call (not the in-memory struct) and asserts both fields survived. Ran it directly (`go test ./internal/tests/sqlite/... -v`) — passes. Against the original buggy INSERT (hardcoded `''`,`''`,`''`) this test would fail immediately (`got.ComposeYAML != compose`), confirming it's a real regression guard, not a rubber stamp. Pre-existing `TestProjectComposeColumnsRoundTrip` still present and still passes — it deliberately creates with empty compose fields and checks `UpdateProject` persists them later, which is a distinct (still valid) code path from the new test's create-time coverage.

4. **FindProject/ListProjects unchanged since pass 1, still correct.** `git diff` shows no change to these two functions between pass 1 and now — `COALESCE(compose_yaml, ''), COALESCE(exposed_service, '')` selected in the same order they're scanned into `&p.ComposeYAML, &p.ExposedService`, both already verified in pass 1 and reconfirmed by direct read here.

5. **Build/vet/test.** `go build ./...`, `go vet ./...` clean. `go test ./...` — all packages pass, including `internal/tests/sqlite` (`TestMigrationAppliesOnFreshDB`, `TestMigrationAppliesOnCopiedLiveDB`, `TestProjectComposeColumnsRoundTrip`, `TestCreateProjectPersistsComposeFields`, `TestServiceContainerCRUD` — all PASS).

No new issues found. The blocking bug from pass 1 is fixed correctly and is now covered by a regression test that would catch a recurrence. Nothing else in this delta warrants comment.

### Non-blocking / minor
- None new beyond the double-`ParseComposeYAML`-call note already on record from pass 1 (unchanged, still non-blocking).

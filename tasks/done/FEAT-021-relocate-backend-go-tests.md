---
id: FEAT-021
type: feature
title: Relocate backend Go tests out of production packages into internal/tests
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-003
created: 2026-07-09
history:
  - {date: 2026-07-09, stage: created, by: architect, note: "task created per user request — seeing _test.go files inside service packages felt wrong to them; user chose internal/tests as the home"}
  - {date: 2026-07-10, stage: development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-10, stage: review, by: architect, note: "6 files moved to internal/tests black-box, 5 colocated exceptions documented, 33 tests identical before/after; moved to review"}
  - {date: 2026-07-10, stage: done, by: architect, note: "test-stage verified directly (build/vet/test all pass from new locations; no runtime surface -> no builder/tester needed); task complete"}
---

## Summary
The user wants Go test files out of the production packages: `*_test.go`
files currently sit inside `backend/internal/service/` (and
`backend/internal/repository/sqlite/`, `backend/internal/handler/`).
Per the user's decision they move to a dedicated test tree —
`backend/internal/tests/` — organized to mirror what they cover. Note:
this departs from Go's colocated-test convention, so the moved tests
become black-box (external test packages) and can only exercise exported
API; that constraint is accepted and part of the task.

## Requirements
- Inventory every `*_test.go` under `backend/` living inside a production
  package (service, repository/sqlite, handler — run the find, don't
  assume).
- Move them to `backend/internal/tests/<area>/` (e.g.
  `internal/tests/service/`, `internal/tests/sqlite/`) with package names
  like `service_test` — black-box tests importing the packages they test.
- Where a test currently depends on unexported identifiers, prefer
  rewriting it against exported API. If a specific test genuinely cannot
  work black-box (e.g. it tests an unexported helper directly), either
  (a) leave that one file colocated with a comment explaining why, or
  (b) drop the unexported-only assertions if they're redundant with
  exported-path coverage — judge case by case and document each in
  Implementation Notes.
- `go test ./...` from `backend/` must run the full suite from the new
  locations with everything passing; total test count must not silently
  shrink (compare `go test ./... -list '.*' | wc` style evidence before
  and after, or the pass/fail summary lines).
- Update any docs/scripts referencing old test paths (grep for
  `service_test\|_test.go` mentions in scripts/docs).

## Out of Scope
- The bash verification scripts in `backend/scripts/` (already outside
  packages; they stay).
- Writing new tests or changing what's covered.
- Frontend tests.

## Proposed Solution / Approach
Inventoried every `*_test.go` under `backend/` with `find backend -name
'*_test.go'`: 9 in `internal/service/`, 2 in `internal/handler/` (already
`package handler_test`, just physically colocated), 1 in
`cmd/egress-proxy/` (`package main`). Nothing under
`internal/repository/sqlite/` — that path in the Summary/Requirements was
speculative, not an actual file.

Created `backend/internal/tests/{service,handler}/` mirroring the areas
covered. The two handler tests were already external (`package
handler_test`) and used only exported identifiers, so they just moved
directory — no code changes needed. The 9 service tests were white-box
(`package service`); for each I checked whether it only touched
`service`'s exported API:
- Fully exported (`auth_service_test.go`, `egress_service_test.go`,
  `whitelist_service_test.go`, `resource_limit_service_test.go`): moved
  wholesale, changed to `package service_test`, qualified every
  service-package identifier (`service.NewXService`, `*service.XService`,
  etc).
- Mixed files (`git_credential_service_test.go`,
  `project_service_test.go`): split. The subset of tests that only need
  exported API moved to `internal/tests/service/`; the one test per file
  that genuinely needs an unexported identifier stayed colocated in
  `internal/service/` as a documented exception (comment at the top of
  each trimmed file explains why, and points at where the rest of the
  coverage now lives).
- Fully unexported-dependent (`ring_buffer_test.go`,
  `terminal_session_registry_test.go`, `agent_service_test.go`): these
  construct/manipulate genuinely unexported types (`ringBuffer`,
  `sessionRegistry`, `TerminalSession`'s private fields) with no exported
  constructor or equivalent anywhere in the package — rewriting them
  black-box isn't a rewrite, it's dropping the coverage entirely (or
  adding production-code exports purely to satisfy a test, which the task
  explicitly discourages). Left colocated with an explanatory comment on
  each.
- `cmd/egress-proxy/main_test.go` stayed in place: `package main` isn't
  importable by any other package in Go, so there is no black-box option
  at all — this one isn't a judgment call.

Chose to duplicate small `newTestX` fixture helpers rather than build a
shared internal test-helper package: each helper is ~15-30 lines, used by
1-2 tests, and none are shared across the files that moved (verified via
grep — no cross-file helper reuse existed before this task either), so a
shared package would be speculative infrastructure for a problem that
doesn't exist (YAGNI).

## Affected Areas
- `backend/internal/tests/service/` (new) — auth_service_test.go,
  egress_service_test.go, whitelist_service_test.go,
  resource_limit_service_test.go, git_credential_service_test.go (CRUD
  subset), project_service_test.go (CRUD subset)
- `backend/internal/tests/handler/` (new) — project_handler_test.go,
  terminal_handler_test.go (moved unchanged)
- `backend/internal/service/` — 5 files remain as documented colocated
  exceptions (ring_buffer_test.go, terminal_session_registry_test.go,
  agent_service_test.go, git_credential_service_test.go trimmed to
  TestInjectToken only, project_service_test.go trimmed to
  TestProjectServiceCloneRepo only)
- `backend/cmd/egress-proxy/main_test.go` — unchanged location, comment
  added explaining the `package main` constraint
- `backend/scripts/test-auth.sh` — updated one comment referencing the
  old `internal/service/auth_service_test.go` path

## Acceptance Criteria / Definition of Done
- [ ] No `*_test.go` remains inside `backend/internal/{service,handler,repository/...}` production packages (except any explicitly-justified colocated exception, each documented)
- [ ] `go test ./...` passes from backend/ with the same effective coverage (no tests silently lost — evidence in Implementation Notes)
- [ ] `go build ./...`, `go vet ./...` pass
- [ ] Moved tests are external (`package X_test`) and import production packages normally
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
`find backend -name '*_test.go'` shows only the new tree (plus documented
exceptions); `go test ./...` full-suite pass; spot-run two moved suites
individually (`go test ./internal/tests/...`).

## Implementation Notes

**Moved wholesale (black-box, `package service_test` / already
`handler_test`):**
- `internal/service/auth_service_test.go` -> `internal/tests/service/auth_service_test.go`
- `internal/service/egress_service_test.go` -> `internal/tests/service/egress_service_test.go`
- `internal/service/whitelist_service_test.go` -> `internal/tests/service/whitelist_service_test.go`
- `internal/service/resource_limit_service_test.go` -> `internal/tests/service/resource_limit_service_test.go`
- `internal/handler/project_handler_test.go` -> `internal/tests/handler/project_handler_test.go` (no code changes — was already `package handler_test`)
- `internal/handler/terminal_handler_test.go` -> `internal/tests/handler/terminal_handler_test.go` (no code changes — was already `package handler_test`)

**Split (partial move + partial colocated exception):**
- `git_credential_service_test.go`: `TestGitCredentialServiceGetSetDelete`
  and `TestGitCredentialServiceSetRequiresToken` (+ `assertContains`
  helper) moved to `internal/tests/service/git_credential_service_test.go`
  as `service_test`. The one raw-DB assertion that read `svc.db` (an
  unexported field) now reads from a `*sqlite.DB` the test helper opens
  and returns itself, since it's the same DB handle passed into
  `NewGitCredentialService` — no production code change needed.
  `TestInjectToken` stayed colocated in
  `internal/service/git_credential_service_test.go`: it calls
  `injectToken`, an unexported pure URL-rewriter, directly across 5 URL
  shapes (http/https/ssh, with/without username) — there's no exported
  entry point to hit all 5 branches in isolation without a live
  GitHub/GitLab-style remote for each. This isn't redundant with the
  moved CRUD test (which only exercises one of the 5 branches via
  `AuthenticatedCloneURL`), so nothing was dropped, per the "b" option in
  Requirements not applying here.
- `project_service_test.go`: `TestProjectServiceCRUD` (+
  `waitForProjectStatus`) moved to
  `internal/tests/service/project_service_test.go` as `service_test`,
  using `service.CreateProjectRequest`/`service.UpdateProjectRequest`
  (already exported). `TestProjectServiceCloneRepo` stayed colocated in
  `internal/service/project_service_test.go`: it calls `svc.cloneRepo`
  directly (unexported method) because the only exported path that
  reaches it, `Create` -> `deploy`, bails out at `requireDocker()` before
  ever cloning when `docker` is nil — which is how every test in this
  package builds `ProjectService` (no Docker dependency, matching how the
  rest of the suite avoids requiring a daemon for pure logic). `runGit`
  helper stayed with it since only this test uses it.

**Kept fully colocated (documented in-file, no split possible):**
- `internal/service/ring_buffer_test.go` — `ringBuffer`/`newRingBuffer`
  are entirely unexported with no exported constructor or interface
  anywhere in the package; there is no black-box way to construct one at
  all.
- `internal/service/terminal_session_registry_test.go` — tests the
  unexported `sessionRegistry` type and builds `TerminalSession` values by
  setting unexported `ring`/`done` fields directly (deliberate, so the
  registry's add/get/remove/count/list/projectLock/activeNetworks and the
  session cap can be unit tested without a Docker daemon, per FEAT-015).
- `internal/service/agent_service_test.go` —
  `TestAgentServiceSessionCapEnforcement` seeds `agentSvc`'s unexported
  `sessions` registry and constructs `TerminalSession` via its unexported
  fields directly, specifically to verify the cap check without paying
  for `maxSessionsPerProject`+1 real (Docker-backed) session creates.
  Needs a reachable Docker daemon (`newTestAgentService` skips itself
  otherwise) — ran successfully against the daemon in this environment.
- `backend/cmd/egress-proxy/main_test.go` — `package main`; Go does not
  allow any other package to import a `main` package, so black-box
  testing of `proxyHandler`/`isAllowed`/`parseDomains` is structurally
  impossible, not a judgment call.

**Docs/scripts:** `backend/scripts/test-auth.sh` had one comment
referencing `backend/internal/service/auth_service_test.go` — updated to
the new `backend/internal/tests/service/auth_service_test.go` path.
`internal/repository/sqlite/` mentioned in the task's Summary/Requirements
as a possible source of test files turned out to have none (`find backend
-name '*_test.go'` confirmed 12 files total, none there) — nothing to do
there.

**Test count evidence (before -> after, `go test ./... -list '.*'` /
`go test ./... -count=1`):** 33 top-level test functions before, 33 after
— identical sorted name sets (diffed directly), zero dropped, zero added.
Docker-backed tests (`internal/tests/handler`'s two terminal-handler
tests, `internal/service`'s `TestAgentServiceSessionCapEnforcement`) ran
for real against the Docker daemon available in this environment, not
skipped. Full suite: `go build ./...`, `go vet ./...`, and
`go test ./... -count=1` all pass, both before and after the move (before:
package layout was `internal/service` + `internal/handler` +
`cmd/egress-proxy`; after: `internal/service` (5 exception files),
`internal/handler` (now empty of tests), `internal/tests/service`,
`internal/tests/handler`, `cmd/egress-proxy`). Also spot-ran
`go test ./internal/tests/... -v` standalone — all pass.

**Final layout** (`find backend -name '*_test.go'`): 5 files remain
colocated in `internal/service/` (all documented exceptions above) + 1 in
`cmd/egress-proxy/` (structural exception) + 8 files under the new
`internal/tests/{service,handler}/` tree. No `*_test.go` remains in
`internal/handler/`.

## Review Notes

### 2026-07-10 — reviewer

Verdict: PASS

Verification performed:
- `git status`/`find backend -name '*_test.go'` confirms the layout matches the Implementation Notes exactly: 8 files under the new `internal/tests/{service,handler}/` tree, 5 documented colocated exceptions in `internal/service/`, 1 structural exception in `cmd/egress-proxy/` (package main). Nothing left in `internal/handler/` or `internal/repository/sqlite/`.
- Test-set equality (the key concern): stashed the full working tree (`git stash push -u`), ran `go test ./... -list '.*'` on the pre-change tree (33 `Test*` functions), popped the stash, ran the same on the post-change tree (33 `Test*` functions), diffed the two sorted name lists — **zero difference**. No test silently dropped or duplicated across the split files.
- `go build ./...` and `go vet ./...` — clean, no output.
- `go test ./... -count=1` — all packages pass, including the two Docker-backed `internal/tests/handler` tests (ran for real, 11.3s, not skipped) and `TestAgentServiceSessionCapEnforcement` (ran for real against the local Docker daemon, confirmed via `-v` output showing actual container lifecycle logs, not a `t.Skip`).
- Spot-checked the two partial-move files:
  - `git_credential_service_test.go`: CRUD tests moved to `internal/tests/service/` as `package service_test`, correctly qualifying `service.NewGitCredentialService` etc.; the one `svc.db` unexported-field read was reworked to read from the `*sqlite.DB` handle the test helper already owns (no production code touched). `TestInjectToken` stayed colocated with a comment — verified `injectToken` is indeed unexported with no exported equivalent that exercises all 5 URL-shape branches.
  - `project_service_test.go`: `TestProjectServiceCRUD` moved black-box; `TestProjectServiceCloneRepo` stayed colocated — verified `cloneRepo` is an unexported method and the only exported path to it (`Create`→`deploy`) bails out at `requireDocker()` when `docker` is nil, which is how every test in the package builds the service. Justification holds.
- Spot-checked two "fully colocated" exceptions: `ring_buffer_test.go` genuinely calls unexported `newRingBuffer`/`ringBuffer` with no exported constructor anywhere in the package; `cmd/egress-proxy/main_test.go` is `package main`, which Go cannot import from any other package — both exceptions are real, not laziness.
- Confirmed via `git diff -M` that the four "moved wholesale" service tests and the two handler tests are pure moves/renames: the two handler test files diff as byte-identical (0 insertions/deletions); the four service files only gained `_test` package suffix + qualified identifiers, no logic changes.
- Grepped the diff for non-test `.go` changes: none. Only test files plus the one-line comment fix in `backend/scripts/test-auth.sh` (path reference update) were touched — matches the claim of "no production code changed."
- Other uncommitted dirty files in the working tree (AGENTS.md, Caddyfile, frontend/*, plan.md deletion) are unrelated ambient WIP predating this task — not mentioned anywhere in this task's Implementation Notes/Affected Areas, and untouched by this diff's actual content.

Acceptance criteria walk:
- [x] No `*_test.go` remains in production packages except documented exceptions — verified by `find`.
- [x] `go test ./...` passes with no coverage lost — verified 33/33 test-name-set match + full suite green.
- [x] `go build ./...`, `go vet ./...` pass — verified, clean.
- [x] Moved tests are external `package X_test` — verified for all 8 files in `internal/tests/`.
- [x] KISS/YAGNI — the decision not to extract a shared test-helper package for `newTestX` fixtures (each ~15-30 lines, used by 1-2 tests, no pre-existing cross-file reuse) is a reasonable judgment call, not under-engineering.

Non-blocking / minor:
- `newTestProjectService` is duplicated near-verbatim between `internal/service/project_service_test.go` (colocated exception) and `internal/tests/service/project_service_test.go` (moved), differing only in package-qualified types. Same pattern for the git-credential helper. This is inherent to the split-file approach the task chose and is small enough (~25 lines, 2 call sites total) that extracting a shared helper would be speculative per the task's own YAGNI reasoning — flagging only as an observation, not a blocker.


## Test Notes
<filled in by tester>

## Test Notes

### 2026-07-10 — architect (test-stage verification, direct)

This is a pure test-relocation with no runtime product surface to drive, so
a full builder/tester env-standup would only re-run `go test` (which the
reviewer already did with the Docker daemon, incl. the before/after stash
comparison proving 33 identical test functions). The architect ran the
verification directly instead, per the token-optimization principle:

- `go build ./...` → clean (exit 0)
- `go vet ./...` → clean (exit 0)
- `go test ./...` → all pass with the Docker daemon present:
  - `cmd/egress-proxy` ok
  - `internal/service` ok (1.17s — the colocated unexported-access exceptions)
  - `internal/tests/service` ok (0.33s — the moved black-box service tests)
  - `internal/tests/handler` ok (11.27s — moved black-box handler tests,
    Docker-backed, ran for real not skipped)

Tests now live under `backend/internal/tests/{service,handler}/` (black-box)
plus the 5 documented colocated exceptions in `internal/service/` and the
structural `cmd/egress-proxy/main_test.go`. Verdict: PASS.

---
id: FEAT-010
type: feature
title: Backend test suite (auth, project CRUD, env vars, agent terminal, git credential)
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-001
created: 2026-07-04
history:
  - {date: 2026-07-04, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-06, stage: in-development, by: architect, note: "assigned to sdlc-developer (sonnet); FEAT-004/FEAT-008 both landed. Architect confirmed backend/internal/service/git_credential_service_test.go (from FEAT-008) already covers git credential CRUD + encryption round-trip - don't duplicate that, focus on auth/project/env-var/agent-terminal gaps. Existing test files to match conventions from: api_key_service_test.go, git_credential_service_test.go, resource_limit_service_test.go, whitelist_service_test.go (all real-SQLite-DB style, no mocks)"}
  - {date: 2026-07-06, stage: in-review, by: architect, note: "developer added auth_service_test.go, project_service_test.go (incl. self-contained file:// clone-on-create test), agent_service_test.go (real-Docker sandbox lifecycle test, confirmed no residue after 2 runs); also fixed two small pre-existing production bugs surfaced by the new tests (CreateProject never set container_id, CreateEnvVar never returned the inserted ID) - architect verified both fixes directly and reran the full suite plus -race; moved to review"}
  - {date: 2026-07-06, stage: done, by: architect, note: "review PASSED. Test stage skipped builder/tester: this task's entire Test Plan is 'go test ./... -v passes', which the architect already ran directly (including -race and the Docker-backed sandbox lifecycle test, twice, confirming no residue) - no stack/UI/API surface to exercise beyond that. Moved to done"}
---

## Summary
The backend currently has zero tests. Add coverage for the core flows:
auth, project CRUD, env var management, agent terminal lifecycle, and git
credential handling. This task depends on FEAT-004 (terminal/lifecycle) and
FEAT-008 (git credential) having landed, since it tests their behavior.

## Requirements
- Auth: login, logout, token validation (happy path + invalid/expired token)
- Project: create/read/update/delete, including the git-clone-on-create path
- Env var: create/list/delete for project env vars
- Agent terminal: sandbox container create/destroy lifecycle (mock or use a
  real Docker daemon in test, developer's call — document the approach),
  exec/attach flow at least at the handler/service boundary
- Git credential: create/list/delete, and an encryption round-trip test
  (encrypt then decrypt returns the original token)
- Use Go's standard `testing` package; no new test framework dependency
  unless there's a strong reason (document if so)

## Out of Scope
- Frontend tests — not in this task
- Full end-to-end Docker integration tests beyond what's practical in CI
  (use judgment on mocking the Docker client vs. requiring a real daemon)

## Proposed Solution / Approach
Match the existing convention exactly (`api_key_service_test.go`,
`git_credential_service_test.go`, `resource_limit_service_test.go`,
`whitelist_service_test.go`): plain `testing`, a real throwaway SQLite DB
per test (opened, migrated, cleaned up via `t.Cleanup`), no mocks, no new
test framework. New files, all in `backend/internal/service/`:

- `auth_service_test.go` - `Setup`/`IsSetup`, `Login` happy path +
  wrong-password + no-user-yet, and `ValidateToken` happy path + garbage
  token + wrong signing secret + expired token (crafted directly with
  `golang-jwt` using the same claims shape as `generateToken`). Note on
  "logout": `auth_service.go` issues stateless JWTs with no server-side
  session/blacklist, so there is nothing to invalidate server-side on
  logout (confirmed - no `/logout` route exists, frontend just drops the
  token client-side). The behavioral equivalent that's actually testable
  is exactly what `TestAuthServiceValidateTokenFailures` covers: a token
  that's missing, malformed, signed with a different secret, or expired
  all fail validation the same way logging out would.
- `project_service_test.go` - CRUD (Create/List/Get/Update/Delete) plus
  the env-var CRUD that hangs off a project, using `ProjectService` with a
  nil Docker client (mirrors how it behaves with Docker unavailable -
  `deploy()` bails out at `requireDocker()`) and a Caddy client pointed at
  an address nothing calls (Domain left empty in tests, so
  `Delete()`'s conditional `RemoveRoute` is never hit). For the
  clone-on-create path specifically: rather than driving it through the
  full async `Create()` -> `deploy()` -> Docker build/run pipeline (which
  needs a real Docker daemon and would make the git-clone assertion
  incidental to a much bigger, flakier test), `cloneRepo` is called
  directly with a `file://` URL pointing at a local bare repo created and
  torn down within the test (`git init --bare` + a seed clone that commits
  and pushes one file) - self-contained, no network access, same spirit as
  how `git_credential_service_test.go` tests `injectToken` without a live
  GitHub/GitLab server.
- `agent_service_test.go` - sandbox lifecycle
  (create-on-connect/destroy-on-disconnect refcounting) plus a minimal
  exec/attach check, against a **real Docker daemon**. `AgentService.docker`
  is a concrete `*dockerclient.Client` wrapping the Docker SDK directly,
  not an interface - introducing one purely to fake it in one test file
  would be a speculative abstraction the production code doesn't otherwise
  need (YAGNI). This environment has a working Docker daemon (confirmed:
  `docker version`, internal bridge networks, container create/start/exec
  all work here), so the test drives `StartSandbox`/`ReleaseSandbox`
  directly against it and skips itself (`t.Skip`) if no daemon is
  reachable, so it degrades gracefully in an environment where that
  assumption doesn't hold.

No handler-layer tests were added: the existing convention in this repo is
entirely service-layer (no handler tests exist for any prior feature
either), and the service-layer tests above already exercise every behavior
called for in Requirements. Adding an `httptest`-based handler test file
would be introducing a second, parallel test style for no behavior gain -
against KISS/YAGNI and the instruction to match existing conventions.

Git credential CRUD + encryption round-trip is already fully covered by
`git_credential_service_test.go` (FEAT-008) and intentionally not
duplicated here, per the architect's note.

## Affected Areas
- `backend/internal/service/*_test.go` (new)
- `backend/internal/handler/*_test.go` (new)
- `backend/internal/repository/sqlite/*_test.go` (new)

## Acceptance Criteria / Definition of Done
- [ ] Auth tests cover login/logout/token validation, happy + failure paths
- [ ] Project CRUD tests pass, including clone-on-create
- [ ] Env var CRUD tests pass
- [ ] Agent terminal lifecycle tests cover create-on-connect/destroy-on-disconnect at the service layer
- [ ] Git credential tests cover CRUD + encryption round-trip
- [ ] `go test ./...` passes cleanly
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Run `go test ./... -v` and confirm all new tests pass and existing
functionality (if any tests existed) still passes.

## Implementation Notes
New test files (all `backend/internal/service/`, matching existing
convention - real throwaway SQLite DB, no mocks):
- `auth_service_test.go` - `TestAuthServiceSetupAndLogin`,
  `TestAuthServiceLoginWrongPassword`, `TestAuthServiceLoginNoUser`,
  `TestAuthServiceValidateTokenFailures`.
- `project_service_test.go` - `TestProjectServiceCRUD` (create/list/
  get/update/delete + env var create/list/delete),
  `TestProjectServiceCloneRepo` (drives `cloneRepo` directly against a
  local `file://` bare repo built in the test).
- `agent_service_test.go` - `TestAgentServiceSandboxLifecycle`, against a
  real Docker daemon (skips via `t.Skip` if none is reachable). Covers:
  two overlapping "terminal connections" reusing one sandbox container,
  refcount-gated stop/remove on the last release, and a minimal
  OpenShell/AttachShell exec check at the service boundary. Test cleanup
  also force-cleans the bind-mounted workspace dir from inside a
  throwaway `alpine` container, since the sandbox runs as root and leaves
  root-owned files there that the (non-root) test process can't remove
  itself - otherwise `t.TempDir()`'s own cleanup fails. Note: stopping the
  `tamga-agent` container consistently takes ~10s in this environment
  (confirmed independently of the test, via plain `docker stop`) because
  its entrypoint (`tail -f /dev/null`) is PID 1 with no signal handler, so
  Docker falls through to its SIGKILL grace period - this is inherent to
  the image, not the test, and out of scope to change here.

Two pre-existing bugs surfaced while writing the project/env-var CRUD
tests and were fixed as minimal, targeted corrections (both are one-line,
low-risk, and block "`go test ./...` passes cleanly" otherwise; filing
separate BUG tickets seemed like unnecessary overhead for something already
isolated and fixed):
- `repository/sqlite/project_repo.go` `CreateProject`: the INSERT never
  set `container_id`, leaving it NULL until the first `UpdateProject`
  call. `FindProject`/`ListProjects` scan straight into a plain `string`
  field, so reading a just-created (not yet deployed) project raised a
  "converting NULL to string" scan error - a real (if narrow) race in
  production too, since a GET landing between `CreateProject`'s INSERT and
  `deploy()`'s first status update would 500. Fixed by inserting
  `container_id` as `''` explicitly.
- `repository/sqlite/env_var_repo.go` `CreateEnvVar`: never set `ev.ID`
  from `LastInsertId()` (unlike `CreateProject`, which does), so
  `ProjectService.CreateEnvVar`'s returned `*domain.EnvVar` always had
  `ID == 0` - meaning the freshly-created env var couldn't actually be
  deleted by its returned ID without a page refresh/re-fetch first. Fixed
  to mirror `CreateProject`'s pattern.

Verified: `go build ./...`, `go vet ./...`, `go test ./...` and
`go test ./... -race` all pass cleanly; ran the Docker-backed test twice
to confirm it isn't flaky, and confirmed no leftover containers/networks
after the run (`docker ps -a`/`docker network ls` for `agent-*`).

Out of reach / explicitly not covered: full end-to-end
Create->deploy->build-image->Caddy-route flow (would need a real Docker
daemon *and* real image builds *and* a real Caddy admin API in the test
run - out of scope per the task's own "Out of Scope" section); handler-
layer HTTP tests (no existing convention for them in this repo, and the
service-layer tests already cover the required behaviors - see Proposed
Solution).

## Review Notes
### 2026-07-06 - reviewer
Verdict: PASS

Verified independently (in addition to the architect's build/vet/test/-race pass):
- `auth_service_test.go`'s four `ValidateToken` failure cases (garbage/empty
  string, wrong signing secret, expired-but-correctly-signed) are genuinely
  distinct failure modes, not redundant variations - confirmed the claims
  shape (`user_id`/`exp`/`iat`) crafted in `signTestToken` matches
  `auth_service.go`'s real `generateToken` exactly, and re-ran this file's
  tests standalone (all pass).
- `project_repo.go`/`env_var_repo.go` bug fixes: both are correct, minimal,
  one-line-diff fixes mirroring the existing `CreateProject`
  `LastInsertId()` pattern. Confirmed via `git diff` and by re-running
  `TestProjectServiceCRUD`/`TestProjectServiceCloneRepo` standalone (pass).
  Bundling them into this test-only task rather than filing separate BUG
  tickets is reasonable judgment here - they're the kind of bug a test
  suite is supposed to surface, the fixes are trivially scoped to the exact
  lines the new tests exercise, and splitting them out would just be
  process overhead for a one-line change with no independent design
  decisions to review.
- `TestProjectServiceCloneRepo` calling `cloneRepo` directly against a local
  `file://` bare repo (instead of driving it through the full async
  `Create()` -> `deploy()` pipeline) is a reasonable scope decision, not a
  gap: `deploy()` is untestable end-to-end without a real Docker daemon (for
  build+run) and a real Caddy admin API, both explicitly out of scope per
  the task's own "Out of Scope" section. The clone step itself - the only
  part the acceptance criterion "clone-on-create" actually cares about - is
  exercised faithfully and self-contained (no network access needed), the
  same pattern already established by `git_credential_service_test.go`'s
  `injectToken` test. `TestProjectServiceCRUD` separately verifies `Create()`
  kicks off `deploy()` in the background and that it fails fast/gracefully
  via `requireDocker()` when Docker is unavailable, so the pipeline wiring
  itself isn't left completely unverified either.
- Skipping handler-layer tests is legitimate: confirmed via `find` that zero
  handler test files exist anywhere in the repo for any prior feature, so
  "match existing convention" genuinely means service-layer only here. Every
  behavior in Requirements maps to a service-layer test in this change or an
  existing one, so nothing acceptance-criteria-relevant is left unverified
  by this call.
- Confirmed all 5 Requirements bullets are covered: auth
  (`auth_service_test.go`), project CRUD + clone-on-create
  (`project_service_test.go`), env var CRUD (`project_service_test.go`,
  same file, hangs off project), agent terminal lifecycle
  (`agent_service_test.go`), git credential CRUD + encryption round-trip
  (pre-existing `git_credential_service_test.go` from FEAT-008, correctly
  not duplicated - read it and confirmed it covers `Get`/`Set`/`Delete` plus
  an at-rest encryption assertion and decrypt-via-`AuthenticatedCloneURL`/
  `SandboxEnv` round trip).
- `agent_service_test.go` correctly matches `AgentService`'s real
  constructor signature and lifecycle methods (`StartSandbox`,
  `ReleaseSandbox`, `connCount`, `OpenShell`/`AttachShell`,
  `agentNetworkName`); relies on the seeded `builtin-opencode` default
  agent provider from migration `000008`, which `db.Migrate()` provides
  automatically - no gap there. Direct unlocked reads of `agentSvc.connCount`
  in the test are safe since the test is single-threaded (no concurrent
  `StartSandbox`/`ReleaseSandbox` calls), consistent with the architect's
  `-race` pass.

Non-blocking / minor:
- `TestProjectServiceCRUD`'s `waitForProjectStatus` poll has a 2s deadline;
  fine on any reasonably fast CI box, but if this repo's CI is ever
  resource-constrained enough to make goroutine scheduling this slow, it's
  the kind of timing assumption that could need loosening later. Not a
  blocker now.

## Test Notes
<Filled in by the tester.>

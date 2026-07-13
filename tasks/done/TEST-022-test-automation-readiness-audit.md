---
id: TEST-022
type: test
title: "[C3-preflight] Test automation readiness audit"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-13
history:
  - {date: 2026-07-13, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned to standard developer for command/fixture audit"}
  - {date: 2026-07-13, stage: review, by: architect, note: "static audit submitted for standard review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires Docker terminal fixture image/name ownership and teardown contract"}
  - {date: 2026-07-13, stage: review, by: architect, note: "Docker-lane ownership contract resubmitted for review"}
  - {date: 2026-07-13, stage: test, by: architect, note: "runtime gate skipped: static audit has no runtime surface and bash syntax/discovery test plan was directly executed"}
  - {date: 2026-07-13, stage: done, by: architect, note: "review PASS; audit ready for standalone commit"}
---

## Summary
Turn the current ad-hoc frontend/backend checks into a decision-complete
automation contract before test tooling is added. The audit records which
commands are hermetic, which require Docker or a live compose stack, how test
data is isolated, and the smallest CI-safe lane for each class of check.

**Cluster:** standalone preflight

**Verifies:** current test and fixture behavior before C3 implementation

## Scope
- Root Make targets, Go package tests, backend API shell scripts, smoke test,
  frontend lint/build commands, Docker/compose assumptions, and the absence of
  frontend test tooling.
- A written target matrix for local fast feedback, Docker-backed integration,
  browser E2E, and CI.

## Out of Scope
- Adding a test framework, changing production behavior, running destructive
  actions against a shared stack, or fixing unrelated test failures inline.

## Test Approach
Static audit only: inspected command definitions, test source guards, fixture
cleanup, compose topology, package lockfiles, and CI directory. Ran
`bash -n` over the existing shell scripts; no application, Docker, or shared
compose command was executed.

### Current command / fixture matrix

| Check | Current command | Prerequisites and isolation | Duration | Result semantics |
|---|---|---|---|---|
| Go suite | `make test` / `go test ./backend/...` | 29 test files across 14 packages. Most use `t.TempDir`/SQLite or `httptest`; five files conditionally reach the Docker daemon. It is therefore not hermetic when Docker is available. | fast to medium | Go exit code; Docker cases silently `Skip` only when Docker is absent. |
| API auth | `backend/scripts/test-auth.sh` | Builds an API binary into `mktemp`, random port, isolated DB/secret and trap cleanup; needs Go, curl. No Docker required. | medium | Script counts assertions and exits non-zero on any failure. |
| API projects | `backend/scripts/test-projects.sh` | Isolated temp DB, temporary local Git fixture/server and random ports. The project deploy path expects a reachable Docker daemon, although its fixtures intentionally avoid a Dockerfile. | medium | Assertion count / exit status. |
| API providers | `backend/scripts/test-providers.sh` | Isolated temp DB/data/secret/random port; inspects only that SQLite fixture; needs Go, curl and `sqlite3`. | medium | Assertion count / exit status. |
| Docker container API | `backend/scripts/test-containers.sh` | Own temp DB and labeled fixture containers, but it connects to the fixed `tamga-egress-proxy` name. The script's documented safety relies on a pre-existing proxy environment matching its expectation. | slow | Assertion count / exit status; unsafe on a shared daemon without an exclusive environment. |
| Backend critical path | `backend/scripts/test-e2e-critical-path.sh` | Own temp DB/Git fixture/Traefik directory and cleans containers/images by its project IDs; requires Docker and builds fixture images. Fixed names/IDs make parallel runs on one daemon unsafe. | slow | Assertion count / exit status. |
| Live smoke | `make smoke-test` / `scripts/smoke-test.sh` | Requires a running compose stack with literal container `tamga-backend-1`, HTTPS at localhost, and the matching admin password. It may call auth setup and creates/deletes a project in that live DB; `--up` mutates the stack. | medium | Fails on health/auth/CRUD error; not CI-safe or read-only. |
| Frontend static | `npm run lint`, `npm run build:offline` in `frontend/` | Node/npm and installed dependencies. `build:offline` avoids API use; no frontend tests/config exist. | fast to medium | npm / Next exit status. |
| Frontend unit/browser | none | No Vitest/Jest/Testing Library or Playwright configuration/tests. `@playwright/test` appears only as an optional Next peer in the lockfile and as an extraneous installed package, not a project dependency. | n/a | n/a |
| CI | none | Repository has no `.github/workflows` files. | n/a | n/a |

### Proposed non-overlapping automation contract

| Lane | Proposed command after C3 | Fixture/environment contract | CI disposition |
|---|---|---|---|
| Backend unit | `make test-backend-unit` | `go test ./backend/...` after Docker-reaching Go tests carry an `integration` build tag; TempDir/SQLite/httptest only. | Required, parallel-safe. |
| Frontend static + unit | `make test-frontend-static` and `make test-frontend-unit` | `npm ci`, lint/build offline, then Vitest/jsdom tests; no API or Docker. | Required, parallel-safe. |
| Isolated API integration | `make test-backend-api` | Sequential `test-auth.sh`, `test-projects.sh`, `test-providers.sh`, each with its own temp DB/port/secret; provision tools explicitly. | Required in one Docker-capable job after its exact daemon requirement is confirmed. |
| Docker integration | `make test-backend-docker` | Fresh, job-owned Docker daemon only. Preflight must prove no `agent-1`, `agent-net-1`, or `tamga-egress-proxy` exists; run `docker compose build agent egress-proxy` and inspect the resulting `tamga-agent` and `tamga-egress-proxy` images before serialized (`go test -p 1`) Docker-tagged Go tests and `test-e2e-critical-path.sh`. The fixed-name terminal tests exclusively own those names for the full lane; `test-containers.sh` remains excluded pending injectable egress-proxy naming. | Dedicated/serial job, not the fast lane. |
| Browser E2E | `make test-e2e` | Playwright against a CI-owned compose project, unique project name/data volume and seeded admin credentials; never the developer's stack. | Dedicated browser job. |
| Live-stack smoke | `make smoke-test` (renamed/documented as `make test-live-smoke` if a new alias is added) | Explicit operator-provided `CADDY_HOST`, admin password, and stack; use only a disposable stack/database if automated. | Manual/optional; never the default CI lane. |

CI should run the first two lanes on every change, then the isolated API and
browser jobs with their own fixtures. The Docker integration job must not
share a daemon with another job, and its wrapper must register exact
task-owned resources before execution. An always-run teardown removes only
`agent-1`, disconnects `tamga-egress-proxy` from then removes `agent-net-1`,
then removes `tamga-egress-proxy`; it removes the two fixture images only
because the lane required a fresh daemon and built them itself. Test-script
cleanups remain responsible for their temporary project containers/images;
the wrapper must not use name-pattern deletion or daemon-wide prune.
`test-live-smoke` remains deliberately outside CI unless a compose override
creates an owned, disposable data directory and the script stops hard-coding
`tamga-backend-1`.

## Affected Areas
- `Makefile`
- `go.mod`, `backend/**/*.go`, `backend/scripts/*.sh`
- `scripts/smoke-test.sh`, compose files, and `frontend/package.json`
- prospective CI and browser-test configuration paths

## Acceptance Criteria
- [ ] Each existing command is classified with its prerequisites, isolation,
      expected duration class, and pass/fail semantics.
- [ ] The audit proposes exact non-overlapping commands for unit/static,
      Docker-backed API/integration, browser E2E, and live-stack smoke checks.
- [ ] The proposal identifies a CI-safe fixture strategy and records any
      blocker as evidence rather than modifying production code.
- [ ] Results are concrete observations.
- [ ] Defects filed separately, not fixed inline.

## Test Plan
1. Inspect package scripts, Makefile targets, Go test labels/skips, API shell
   scripts, smoke script, compose topology, lockfiles, and current CI files.
2. Run only read-safe discovery or isolated commands needed to classify them;
   do not reuse or tear down a shared compose stack.
3. Write the command/fixture matrix and use it as the dependency contract for
   FEAT-051 through FEAT-054.

## Implementation Notes
Observed concrete constraints:

- `Makefile` exposes only the ambiguous `test` target (`go test ./backend/...`),
  `smoke-test`, and frontend lint/build previews. There is no aggregate target,
  test target matrix, frontend test script, or CI workflow.
- Docker-aware Go tests live in `backend/internal/service/{agent_service_test.go,project_service_test.go}`, `backend/internal/tests/handler/terminal_handler_test.go`, `backend/internal/tests/repository/docker_client_test.go`, and `backend/internal/tests/service/project_service_test.go`. They call Docker after availability checks, so the existing all-package command changes its runtime behavior according to the host. FEAT-051 must segregate them with build tags (or an equally stable explicit selection) before labeling a lane "unit".
- The terminal handler and agent-service Docker tests create fresh SQLite
  projects whose first ID is `1`; they consequently use the fixed
  `agent-1`/`agent-net-1` fixtures. `AgentService.ensureEgressProxy` also
  creates/recreates the fixed `tamga-egress-proxy` container from the
  `tamga-egress-proxy` image, while sandbox startup requires `tamga-agent`.
  A fresh Docker daemon does not supply either image, so the Docker-lane
  wrapper must build both compose build-only services and verify both tags
  before it starts tests. This lane is one exclusive, serial job: no shared
  compose stack, no parallel Go packages, and no other Docker workload may
  acquire those names. Its always-run cleanup may touch exactly the two
  known terminal fixtures, the fixed proxy, and the two images it built;
  no broad `docker prune` or prefix-based deletion is permitted.
- All five API scripts compile the real backend into a temporary directory,
  choose random ports, and trap cleanup. Auth and providers are isolated;
  projects, containers, and critical-path additionally require Docker. The
  containers script has a specific shared-resource hazard: its sandbox path
  assumes the literal shared `tamga-egress-proxy` is compatible.
- `scripts/smoke-test.sh` is a real live-stack check, not a fixture test: it
  uses `docker exec tamga-backend-1`, defaults to `ADMIN_PASSWORD=admin`, and
  mutates auth/project state. Do not place it in a default automated target.
- `docker-compose.yml` bind-mounts `./data` into backend and uses the default
  compose naming model. A browser CI fixture needs a compose override or
  equivalent that owns a unique data directory/project name and has teardown
  scoped to those exact resources.
- `bash -n backend/scripts/*.sh scripts/smoke-test.sh` passed. No runtime
  tests were run during this audit, preserving the existing shared stack.

Blockers for CI automation (to be handled in FEAT-051--054, not fixed here):

1. No build-tag boundary currently separates Docker Go tests from unit tests.
2. The container script's fixed egress-proxy dependency cannot be proven safe
   on a shared Docker daemon; it remains excluded from CI until it has an
   injectable/owned proxy contract. The terminal Go tests are admissible only
   in the fresh-daemon, fixed-name-exclusive lane defined above.
3. Browser tests have no declared package/configuration, and the current smoke
   script cannot target a compose project with a non-default backend name.

## Review Notes
### 2026-07-13 — CHANGES_REQUESTED

The command matrix is otherwise evidence-backed: `Makefile` has only the
documented `test`/`smoke-test` targets, `frontend/package.json` has no test
script or declared browser/unit runner, `.github/workflows` is absent, and
`bash -n` passes for all five backend scripts plus the smoke script. The five
listed Go test files do conditionally require a Docker daemon, and the shell
script isolation/smoke-stack claims match their headers and cleanup paths.

The proposed Docker lane is not yet decision-complete. In addition to the
explicitly excluded `test-containers.sh`,
`backend/internal/tests/handler/terminal_handler_test.go` creates terminal
sandboxes through `AgentService.ensureEgressProxy`; that code uses the fixed
`tamga-agent` and `tamga-egress-proxy` image/name contract. The current matrix
says only “job-owned Docker daemon” and records fixture IDs, but does not
require those two images to be built/provisioned, define the fixed-name
ownership/serial boundary, or name their teardown policy. Amend the Docker
lane and blocker/implementation notes to make that contract explicit (or
exclude/tag those terminal tests until it is). This prevents a CI lane that is
nominally isolated but fails against a fresh daemon or accidentally acquires a
shared fixed-name proxy.

### 2026-07-13 — PASS

Rework closes the requested gap. The Docker lane now requires a fresh daemon,
builds and inspects the exact `tamga-agent`/`tamga-egress-proxy` tags before
the serialized Go run, and reserves `agent-1`, `agent-net-1`, and
`tamga-egress-proxy` exclusively. Its stated teardown order matches the
terminal test's concrete fixtures and limits deletion to those names plus the
two images built by this fresh job; the repository helper can record those
exact resources without using broad cleanup. `test-containers.sh` remains
correctly excluded pending injectable proxy ownership. The C3 automation
contract is now safe and decision-complete for implementation.

## Test Notes
Runtime gate intentionally skipped: this task audits existing commands and
fixtures only. Its complete plan was directly executed during development and
review, including `bash -n backend/scripts/*.sh scripts/smoke-test.sh`; no
application or shared compose stack was started.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — static command/fixture/CI audit completed; three automation blockers recorded | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED — Docker Go terminal fixture omits fixed agent/proxy image, name-ownership, and teardown contract | n/a | n/a | 1 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — Docker lane now requires fresh-daemon image provisioning, exclusive fixed-name serialization, and exact teardown | n/a | n/a | 1 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — fresh-daemon provisioning, exclusive fixed-name ownership, and exact teardown verified | n/a | n/a | 1 |

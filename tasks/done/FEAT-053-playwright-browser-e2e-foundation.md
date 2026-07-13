---
id: FEAT-053
type: feature
title: "[C3] Reproducible Playwright browser E2E foundation"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-13
history:
  - {date: 2026-07-13, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned after TEST-022 audit and FEAT-051 command foundation"}
  - {date: 2026-07-13, stage: review, by: architect, note: "Playwright foundation submitted for standard review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-023 integration"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "TEST-023 builder found no isolated local browser-stack fixture; shared stack is stale and out of scope"}
  - {date: 2026-07-13, stage: review, by: architect, note: "local disposable browser fixture rework submitted for review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires fixture resources to record into the one TEST-023 builder manifest"}
  - {date: 2026-07-13, stage: review, by: architect, note: "builder-manifest lifecycle rework submitted for review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires stopped partial-start containers recorded with compose ps --all"}
  - {date: 2026-07-13, stage: review, by: architect, note: "partial-container manifest recording rework submitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for rerun of combined TEST-023 integration"}
  - {date: 2026-07-13, stage: done, by: architect, note: "TEST-023 integration PASS; ready for cluster commit"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "TEST-023: browser journeys reach expected pages but selectors incorrectly require heading roles"}
  - {date: 2026-07-13, stage: review, by: architect, note: "browser selector rework submitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for rerun of combined TEST-023 integration"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "TEST-023 builder found only /api/* routed through Traefik; readiness must use routed endpoint"}
  - {date: 2026-07-13, stage: review, by: architect, note: "routed auth-status readiness rework submitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for rerun of combined TEST-023 integration"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "TEST-023 builder found fixture health gate probes non-existent /api/health route"}
  - {date: 2026-07-13, stage: review, by: architect, note: "fixture health-route rework submitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for rerun of combined TEST-023 integration"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "TEST-023 builder requires a prepare-only fixture handoff; current command also runs Playwright"}
  - {date: 2026-07-13, stage: review, by: architect, note: "builder/tester fixture handoff rework submitted for review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires handoff fixture directory verified as an exact builder-manifest file record"}
  - {date: 2026-07-13, stage: review, by: architect, note: "manifest-owned handoff validation rework submitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for rerun of combined TEST-023 integration"}
---

## Summary
Turn browser verification into a declared Playwright lane with safe fixtures
and a small critical journey suite that validates the running frontend/backend
contract.

**Part of:** C3 Test automation

**Cluster Test:** TEST-023

**Depends on:** TEST-022, FEAT-051

## Requirements
- Add Playwright as a declared frontend development dependency with a checked-in
  configuration and a headless CI command.
- Provide an explicit base-URL/service lifecycle contract: use a dedicated
  ephemeral stack or a supplied URL, never silently target a developer's shared
  environment.
- Implement deterministic fixture/bootstrap and cleanup behavior for the
  smallest critical journey: login/auth redirect and authenticated project
  navigation or creation, based on the TEST-022 matrix.
- Capture failure artifacts appropriate for CI while keeping generated reports
  out of source control.

## Out of Scope
- Cross-browser/device matrix, visual regression service, terminal PTY journey,
  load testing, or broad end-to-end coverage of every page.

## Proposed Solution / Approach
Use Playwright only against an explicitly acknowledged, CI-owned URL. The
runner never declares `webServer` or starts Compose; lifecycle remains with
the caller (the later CI task), which must supply `E2E_BASE_URL`, a CI-only
password, and `E2E_OWNED_STACK=1`. A global setup request context accepts the
disposable HTTPS certificate, initializes only an empty auth fixture, and
validates the seeded password before browser journeys begin.

## Affected Areas
- `frontend/package.json`, lockfile, Playwright config and specs
- test fixture/bootstrap helpers and ignore rules
- compose/test helper or Make target only where FEAT-051 contract requires it

## Acceptance Criteria / Definition of Done
- [ ] The browser command fails clearly without its declared service
      prerequisite and runs headlessly with an isolated/supplied target.
- [ ] At least one unauthenticated and one authenticated critical browser path
      execute using deterministic test data.
- [ ] Credentials, reports, videos, traces, and test data are not committed.
- [ ] A failing journey produces actionable CI artifacts.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Run the browser lane against its dedicated test target, verify an intentional
auth/fixture failure is surfaced, and defer the complete command matrix to
TEST-023 after C3 review gates pass.

## Implementation Notes
- Declared exact `@playwright/test@1.61.1`, added a Chromium-only config with
  one worker, headless CI behavior, self-signed HTTPS support, and retained
  trace/screenshot/video/HTML-report artifacts on failure.
- `npm run test:e2e` now refuses execution (exit 2) without
  `E2E_BASE_URL`, `E2E_ADMIN_PASSWORD`, and `E2E_OWNED_STACK=1`; this
  preserves FEAT-051's supplied-URL contract without choosing or starting a
  developer stack.
- Global setup checks auth status, bootstraps the supplied password only for
  an empty disposable fixture, and verifies it can log in. The two journeys
  do not create projects or containers, so no application-data cleanup is
  needed; the documented stack owner tears down its exact resources.
- Added unauthenticated `/dashboard/new` redirect coverage and authenticated
  login → dashboard → new-project-form navigation. Generated Playwright
  reports/results are ignored and the E2E README records the invocation and
  CI artifact contract.
- Verification: prerequisite guard exited 2; `playwright test --list`
  discovered two Chromium journeys; `npm run test:unit`, `npm run lint`, and
  `npm ci --dry-run --ignore-scripts` passed. No stack was started; runtime
  execution is intentionally deferred to the CI-owned TEST-023 fixture.

### 2026-07-13 — fixture-lifecycle rework

- `make test-e2e` now calls `scripts/test-playwright-e2e.sh`. It rejects a
  supplied target URL, password, or ownership flag, and requires the explicit
  `E2E_DISPOSABLE=1` acknowledgement before creating anything.
- The script creates an exact, unique Compose project, network, image names,
  loopback HTTP/HTTPS ports, and `/tmp/tamga-sdlc-playwright.*` data/dynamic
  directories. It sets a deterministic fixture-only admin credential only
  after that fresh data path exists.
- `scripts/playwright-compose.yml` is a standalone test topology. It cannot
  select the repository's default Compose project or ports. Every fixture
  resource is recorded in an SDLC manifest; its trap runs project-scoped
  Compose teardown and then `scripts/sdlc-environment.sh cleanup` for exact
  recorded resources. The helper now safely removes an exact recorded private
  fixture directory.
- Static verification: shell syntax and rendered Compose config passed;
  missing acknowledgement and supplied-target gates each returned exit 2.
  Browser/runtime execution remains exclusively for combined TEST-023.

### 2026-07-13 — TEST-023 manifest rework

- The browser lifecycle now requires an already-created `E2E_MANIFEST`; it
  appends fixture note, private data directory, exact network/images, and all
  discovered Compose container IDs to that single builder-owned manifest.
- It neither creates/deletes the manifest nor runs teardown itself. Its exit
  trap records partial Compose startup too; the TEST-023 builder retains one
  inspectable resource inventory and invokes helper cleanup after the complete
  C3 run.
- Static verification passed for shell syntax, rendered Compose config, the
  missing acknowledgement/manifest gates, and the supplied-target rejection.

### 2026-07-13 — stopped-container manifest rework

- Both post-start and EXIT-path discovery now use project-scoped
  `docker compose ps --all --quiet`, so stopped/exited containers from a
  partial fixture startup are recorded in the same TEST-023 manifest.

### 2026-07-13 — builder/tester handoff rework

- `make test-e2e-prepare` is builder-only: it builds, starts, health-checks,
  and records the private fixture but never starts Playwright. It prints a
  restricted `E2E_HANDOFF_FILE` after the target is healthy.
- `make test-e2e` is tester-only: it validates the supplied private handoff
  against the same manifest and unique localhost URL, health-checks it, then
  executes Playwright without Compose startup or stack selection.
- Static verification passed for shell syntax, rendered Compose config, and
  missing mode/handoff Make gates. Runtime remains solely TEST-023.

### 2026-07-13 — manifest-owned handoff rework

- Tester mode now derives the handoff parent directory and requires an exact
  `file` record for it in `E2E_MANIFEST` before parsing any URL or credential.
  A matching `/tmp` path alone is therefore insufficient to forge a target.

### 2026-07-13 — fixture health-route rework

- Prepare polling/final gate and tester handoff recheck now probe the exposed,
  non-mutating `GET /health` route, rather than nonexistent `/api/health`.

### 2026-07-13 — Traefik readiness-route rework

- Prepare polling/final gate and tester handoff recheck now use non-mutating
  `GET /api/auth/status`: it is registered without auth and returns only
  `{ "setup": boolean }`, while existing Traefik routes `/api/*` to backend.

### 2026-07-13 — browser-selector rework

- Replaced the two invalid CardTitle heading-role assertions with existing
  accessible UI: the login description and default remote-source Repository
  URL form control. Production UI was not changed for the tests.

## Review Notes
Reviewer appends.

### 2026-07-13 — CHANGES_REQUESTED

The rework correctly rejects caller-supplied E2E target/credential/ownership
values, requires `E2E_DISPOSABLE=1`, uses a unique Compose project, ports,
network, images, and private `/tmp` fixture directory, and builds the
fixture images with `docker compose up --build`. The trap tears down that
unique Compose project and the helper can now remove the exact recorded
fixture directory.

However, `TEST-023` requires the builder to prepare **one C3 manifest** and
record every task-owned resource in it. `test-playwright-e2e.sh` instead
creates, consumes, and deletes a private manifest, so the TEST-023 builder
cannot own or inspect the E2E resources in its declared manifest. Accept a
task-provided manifest (or have the TEST-023 builder explicitly invoke a
documented fixture lifecycle that records to its manifest) and preserve the
same exact cleanup guarantees. Do not fall back to a shared stack.

### 2026-07-13 — PASS

The declared `@playwright/test@1.61.1` dependency is exact in both
`frontend/package.json` and the root lockfile entry; `npm ci --dry-run
--ignore-scripts` accepts it. The Chromium-only configuration discovers two
focused journeys (`playwright test --list`), uses one worker/headless defaults,
retains trace/screenshot/video and HTML artifacts only for failures, and the
three generated artifact directories are ignored. No credential value is
written to source or an error message.

The supplied-target contract is enforced before Playwright starts: `npm run
test:e2e` returned exit 2 without `E2E_BASE_URL`,
`E2E_ADMIN_PASSWORD`, and `E2E_OWNED_STACK=1`. There is deliberately no
`webServer` or Compose action. `make test-e2e` supplies the required base URL
to the same package gate while the caller's password/ownership variables pass
through unchanged. Global setup reaches only the acknowledged target, creates
auth state only when its status is empty, verifies the supplied password, and
the two browser paths create no project/container data. The E2E README assigns
exact stack/data cleanup to the later TEST-023 builder; that integration run must provision the
unique stack and Chromium before this lane runs.

Static verification also passed: `npm run lint` and `git diff --check`.
Browser execution was correctly not attempted against a shared stack.

### 2026-07-13 — CHANGES_REQUESTED

`E2E_MANIFEST` must now already exist, the browser command neither creates nor
deletes it, and the target appends the fixture directory, network, images and
container IDs to the TEST-023 builder-owned manifest. It also retains the
strict disposable acknowledgement and supplied-target rejection, with no
shared-stack fallback or in-command teardown.

The EXIT trap currently uses `docker compose ps --quiet`, which reports only
running containers. A failed or partially-started service may already be
stopped and therefore omitted from the exact manifest inventory. Record
`docker compose ps --all --quiet` (with the same unique project) both after
startup and in the EXIT trap so TEST-023 cleanup owns every partial container.

### 2026-07-13 — PASS

`record_containers` now invokes the unique project-scoped Compose command with
`ps --all --quiet` both after `up` and through the EXIT trap. This records
running and stopped partial-start containers into the already-existing
`E2E_MANIFEST`; the browser command still neither creates/deletes that
manifest nor tears down resources. The disposable acknowledgement and
supplied/shared-target rejection remain unchanged. Static diff check passed;
browser/runtime execution remains for the one TEST-023 run.

### 2026-07-13 — CHANGES_REQUESTED

The role split is correct: `E2E_MODE=prepare` builds/starts/health-checks the
unique fixture and never invokes Playwright, while `E2E_MODE=test` performs
only handoff parsing, a health check, and Playwright—no Compose selection or
startup. Both retain the disposable acknowledgement, existing manifest, exact
resource recording, and no shared-stack fallback.

`load_handoff` only compares a field inside the handoff to `E2E_MANIFEST`; it
does not prove that the handoff file's private fixture directory is the exact
`file` resource recorded in that manifest. A forged path matching
`/tmp/tamga-sdlc-playwright.*/handoff.env` can therefore point Playwright at
another localhost service. Validate that the handoff parent directory is an
exact recorded fixture `file` entry in `E2E_MANIFEST` (and reject otherwise)
before accepting its URL/credential. This keeps the tester restricted to the
builder-owned fixture.

### 2026-07-13 — PASS

Before parsing any target or credential, tester mode derives the handoff
parent and uses tab-delimited `awk` matching to require an exact `file` entry
for that directory in `E2E_MANIFEST`. A merely path-shaped forged handoff is
therefore rejected. The existing manifest equality, localhost-only URL,
explicit ownership acknowledgement, prepare-only Compose lifecycle, and
tester-only browser execution remain intact. Static diff check passed;
runtime remains owned by TEST-023.

### 2026-07-13 — PASS

Prepare polling/final verification and tester handoff verification now all use
`$E2E_BASE_URL/health`; no `/api/health` reference remains in the fixture
lane. `backend/internal/router/router.go` exposes that non-mutating route.
The exact manifest-owned handoff validation, unique private fixture, and
no-shared-stack lifecycle are unchanged. Static diff check passed; runtime is
still deferred to TEST-023.

### 2026-07-13 — PASS

All prepare polling/final readiness and tester-handoff readiness checks now
use the existing non-mutating `$E2E_BASE_URL/api/auth/status` route. The
backend router registers `/auth/status` outside the authenticated group, and
the existing Traefik `PathPrefix(/api)` routers forward it to the backend; no
routing or middleware change was introduced. The exact manifest-owned
handoff, unique disposable fixture, and no-shared-stack lifecycle remain
unchanged. Static diff check passed; runtime remains for TEST-023.

### 2026-07-13 — PASS

The browser specs now assert current accessible UI: the login description and
labelled password field on the unauthenticated redirect, then the existing
`Projects` heading, `New Project` button, and default-remote form's labelled
`Repository URL` and `Project Name` controls after login. These selectors
validate redirect, authentication, navigation, and the actual form state;
the selector rework changes no production markup. Fixture lifecycle,
manifest-owned handoff, and no-shared-stack rules are untouched. Static diff
check passed; runtime remains for TEST-023.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — isolated/supplied-target Playwright foundation, auth bootstrap and two non-mutating critical journeys implemented; static/discovery validation passed | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — pinned Chromium Playwright lane, explicit supplied-target/ownership gate, empty-fixture auth bootstrap, non-mutating critical journeys, and ignored CI artifacts verified statically | n/a | n/a | 0 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — reworked local disposable fixture lifecycle; unique exact resources, strict ownership gate, deterministic fixture credential, and manifest-based cleanup verified statically | n/a | n/a | 1 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED — local fixture isolation is sound, but its private deleted manifest prevents TEST-023 from meeting its single C3 manifest/ownership contract | n/a | n/a | 1 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — reworked fixture to append exact resources to the required TEST-023 manifest and leave all teardown to the builder | n/a | n/a | 2 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED — builder-owned manifest lifecycle is correct, but partial stopped Compose containers are not recorded by `ps --quiet` | n/a | n/a | 2 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — changed all fixture container discovery to `ps --all --quiet` so the builder manifest includes stopped partial-start resources | n/a | n/a | 3 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — project-scoped `ps --all --quiet` records normal and partial containers in the sole builder manifest; no private lifecycle remains | n/a | n/a | 3 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — split disposable fixture into builder prepare-only handoff and tester-only Playwright execution modes | n/a | n/a | 4 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED — prepare/test separation is sound, but tester must prove its handoff parent is exactly recorded in the builder manifest | n/a | n/a | 4 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — tester now requires its exact handoff parent directory as a `file` record in E2E_MANIFEST before accepting fixture data | n/a | n/a | 5 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — tester proves its handoff parent through an exact tab-delimited manifest `file` record before target parsing | n/a | n/a | 5 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — corrected prepare and tester fixture health gates to the exposed non-mutating `/health` route | n/a | n/a | 6 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — all fixture health checks use backend-exposed `/health`; manifest-owned isolated lifecycle remains intact | n/a | n/a | 6 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — corrected fixture readiness to Traefik-reachable non-mutating `/api/auth/status` | n/a | n/a | 7 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — readiness uses existing unauthenticated routed `/api/auth/status`; manifest and isolation guarantees preserved | n/a | n/a | 7 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — replaced invalid CardTitle heading selectors with current accessible login/form controls | n/a | n/a | 8 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — selectors validate existing accessible login, dashboard navigation, and remote new-project form controls without production markup or lifecycle changes | n/a | n/a | 8 |

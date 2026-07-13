---
id: FEAT-051
type: feature
title: "[C3] Deterministic frontend and backend test command foundation"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-13
history:
  - {date: 2026-07-13, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned after TEST-022 audit contract"}
  - {date: 2026-07-13, stage: review, by: architect, note: "command foundation submitted for standard review"}
  - {date: 2026-07-13, stage: rework, by: architect, note: "review requires exact image-tag ownership preflight and partial-build-safe cleanup"}
  - {date: 2026-07-13, stage: review, by: architect, note: "Docker image ownership rework resubmitted for review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-023 integration"}
  - {date: 2026-07-13, stage: review, by: architect, note: "provider legacy cleanup changed API lane; resubmitted for scope-focused review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS after legacy provider cleanup; held for combined TEST-023 integration"}
  - {date: 2026-07-13, stage: done, by: architect, note: "TEST-023 integration PASS; ready for cluster commit"}
---

## Summary
Create a small, explicit test-command contract so local developers and CI can
run fast static/unit checks without Docker, and opt into isolated API, browser,
or live-stack checks without ambiguous `make test` behavior.

**Part of:** C3 Test automation

**Cluster Test:** TEST-023

**Depends on:** TEST-022

## Requirements
- Define documented Make/package-script entry points for backend unit/static,
  frontend static, frontend unit, Docker-backed API integration, browser E2E,
  and explicit live-stack smoke checks.
- Preserve `make test` compatibility only when its behavior is deterministic;
  otherwise make the new target names unambiguous and document the migration.
- Ensure default fast commands neither require Docker nor mutate a shared
  compose stack; integration commands must use isolated data, ports, and
  cleanup or fail with actionable prerequisite messaging.
- Use one machine-readable, CI-consumable exit status per command and avoid
  duplicated test logic between Make and package scripts.

## Out of Scope
- Adding browser/unit test frameworks themselves (FEAT-052 and FEAT-053),
  changing API contracts, or running a deployment environment in every PR.

## Proposed Solution / Approach
Make the test surface explicit instead of adding an aggregate command:

- keep `make test` as the Docker-free backend-unit compatibility alias;
- expose named backend API, Docker, frontend static/unit, browser E2E, and
  explicit live-smoke lanes;
- select Docker-reaching Go coverage with the `integration` build tag (and
  the one mixed file's explicit `TAMGA_TEST_DOCKER=1` gate), so a reachable
  local daemon never changes the fast lane's behavior;
- use a small Docker-lane wrapper that requires an explicit fresh-daemon
  acknowledgement, checks the fixed terminal fixture names, provisions the
  required images, runs serially, and removes only the recorded resources;
- keep framework commands as stable package-script contracts with actionable
  exit-2 prerequisites until FEAT-052 and FEAT-053 install their tooling.

## Affected Areas
- `Makefile`, `README.md`, `frontend/package.json`
- existing `backend/scripts/test-*.sh`, `scripts/smoke-test.sh`
- any minimal helper/configuration required to make commands deterministic

## Acceptance Criteria / Definition of Done
- [ ] A developer can discover and run the fast backend/frontend lanes from
      documented commands without bringing up Docker.
- [ ] Docker/API, E2E, and live-stack lanes declare their prerequisites and do
      not infer ownership of shared resources.
- [ ] Existing backend test coverage remains reachable through a named target.
- [ ] CI can call each lane without parsing human-formatted output.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Use the TEST-022 command matrix. Run each fast lane locally, exercise the
prerequisite failure/skip behavior of each opt-in lane, and leave full combined
execution to TEST-023 after all C3 parts pass review.

## Implementation Notes
- Added Make targets for each lane. `test-backend-api` unsets the exported
  compose `PORT` before every script, preserving the scripts' random,
  isolated ports even when `.env` sets `PORT=8080`.
- Tagged whole-file Docker test suites and moved the mixed service Docker
  test into `project_service_integration_test.go`; the remaining mixed
  external service test has an explicit Docker-lane gate. `go test` is now
  Docker-free, while the Docker wrapper invokes serial
  `go test -tags=integration -p 1`.
- Added `scripts/test-backend-docker.sh`. It refuses execution unless
  `TAMGA_TEST_DOCKER_OWNED=1`, a Docker daemon/Compose v2 are available, and
  `agent-1`, `agent-net-1`, `tamga-egress-proxy`, and both exact image tags
  are absent. The EXIT trap is registered immediately after that preflight,
  before Compose starts, so a partial image build is cleaned up safely; it
  can delete only the two exact tags that the lane preflighted as absent.
- Documented lane contracts and live-smoke safety in `README.md`. The
  existing `smoke-test` name remains a guarded compatibility alias.
- Verification passed: `make test-backend-unit`,
  `make test-frontend-static`, integration-tag compile-only discovery,
  shell syntax checks, Make dry-run, and diff checks. Docker, E2E, and live
  smoke preconditions return actionable exit 2 without touching resources.
- `make test-backend-api` correctly receives a random port, but stops on an
  existing assertion drift in `test-auth.sh`: unauthenticated
  `GET /api/agent-providers` expects 401 while the current router returns
  404 (20 passed / 1 failed). This is recorded for a separate fix; the
  command contract does not hide it.

## Review Notes
Reviewer appends.

### 2026-07-13 — CHANGES_REQUESTED

The Docker-free backend lane is sound: `go test ./backend/...` passed without
starting Docker, the tagged Docker suites are excluded from that command, and
the E2E, live-smoke, and Docker acknowledgement gates each returned the
documented exit code 2 before taking ownership of any runtime resource.
`BUG-038` remains a separately filed, visible failure; it was neither masked
nor weakened by this change.

The Docker wrapper is not yet safe for the ownership promise it documents.
`scripts/test-backend-docker.sh` checks only the three fixed container/network
names before `docker compose build agent egress-proxy`; it does not reject
pre-existing `tamga-agent` or `tamga-egress-proxy` image tags. On a daemon
with those tags but no fixed fixtures, Compose can overwrite the existing
images and the EXIT trap then deletes them. Preflight both exact image tags as
absent before the build, and establish exact-image cleanup ownership early
enough to clean a partial failed build. Keep cleanup limited to the already
preflighted names/tags. This makes the fresh-daemon claim enforceable rather
than an operator-only assertion.

### 2026-07-13 — PASS

Rework closes the ownership gap. Before Compose is invoked, the wrapper now
rejects each exact fixture container/network and each `tamga-agent` /
`tamga-egress-proxy` image tag when present. It installs its EXIT trap only
after all of those absence checks, but before the build, so a partial build is
removed while a pre-existing tag can never be claimed or deleted. Cleanup is
limited to those preflighted fixed names and tags. `bash -n` passed and the
unacknowledged prerequisite path still exits 2 without Docker access.

### 2026-07-13 — PASS

The deliberate legacy cleanup is complete and contained: the obsolete
`backend/scripts/test-providers.sh` is deleted, `test-backend-api` now invokes
only the supported isolated auth and project checks, and README no longer
documents removed Agent Providers or API Keys routes. `test-auth.sh` likewise
removes only its stale `/agent-providers` assertion while retaining supported
auth, project, and system-container coverage. `make test-backend-api` passed:
auth 20/0 and project/code/git 60/0. No unrelated cleanup was evaluated.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — deterministic named lanes, Docker ownership guard, and Docker-free unit selection implemented; existing agent-provider route assertion drift recorded separately | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | CHANGES_REQUESTED — Docker wrapper can overwrite/delete pre-existing exact fixture image tags; require absent-tag preflight and partial-build-safe exact cleanup | n/a | n/a | 1 |
| 2026-07-13 | developer_standard | gpt-5.6-terra | low | PASS — exact image-tag absent preflight now precedes a partial-build-safe ownership trap | n/a | n/a | 1 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — exact image tags are absent-preflighted before build and partial-build cleanup is limited to preflighted owned resources | n/a | n/a | 1 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | low | PASS — legacy provider/API-key test and documentation surface removed; isolated current API lane passed (auth 20/0, project/code/git 60/0) | n/a | n/a | 1 |

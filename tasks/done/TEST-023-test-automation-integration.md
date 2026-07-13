---
id: TEST-023
type: test
title: "[C3] Frontend and backend test automation integration"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-13
history:
  - {date: 2026-07-13, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: test, by: architect, note: "all C3 parts passed review; submitted for one combined environment/test run"}
  - {date: 2026-07-13, stage: blocked, by: builder, note: "no isolated browser fixture; shared stack images stale and not task-owned"}
  - {date: 2026-07-13, stage: test, by: architect, note: "FEAT-053 fixture rework passed review; rerunning one combined C3 integration"}
  - {date: 2026-07-13, stage: blocked, by: builder, note: "fixture command couples prepare with Playwright execution; no builder-only handoff mode"}
  - {date: 2026-07-13, stage: test, by: architect, note: "FEAT-053 prepare/test handoff passed review; rerunning combined C3 integration"}
  - {date: 2026-07-13, stage: blocked, by: builder, note: "isolated fixture health gate probes /api/health, but backend health is /health"}
  - {date: 2026-07-13, stage: test, by: architect, note: "FEAT-053 health-route rework passed review; rerunning combined C3 integration"}
  - {date: 2026-07-13, stage: blocked, by: builder, note: "Traefik routes only /api/* to backend; fixture /health is not externally reachable"}
  - {date: 2026-07-13, stage: test, by: architect, note: "FEAT-053 routed readiness passed review; rerunning combined C3 integration"}
  - {date: 2026-07-13, stage: fail, by: tester, note: "fast/API lanes pass; only FEAT-053 Playwright selectors expect nonexistent heading roles"}
  - {date: 2026-07-13, stage: test, by: architect, note: "FEAT-053 selector rework passed review; rerunning combined C3 integration"}
  - {date: 2026-07-13, stage: pass, by: tester, note: "combined fast/API/Playwright matrix passed against builder-owned fixture"}
  - {date: 2026-07-13, stage: done, by: architect, note: "C3 integration PASS; cluster ready for one commit"}
---

## Summary
Verify C3 once as a complete local automation contract: the fast
frontend/backend lanes are deterministic, and the opt-in API/browser lanes
use safe, explicitly owned fixtures.

**Cluster:** C3 Test automation

**Verifies:** FEAT-051, FEAT-052, FEAT-053

## Scope
- Complete named test-command matrix across Go/backend, frontend static/unit,
  Docker-backed API integration, browser E2E, and explicit smoke.
- Prerequisite errors, isolated resource lifecycle, exit statuses, and failure
  artifact behavior.

## Out of Scope
- Hosted CI, every existing backend endpoint, visual regression, load testing,
  or C2 terminal interaction coverage.

## Test Approach
Developer fills before test implementation.

## Affected Areas
- `Makefile`, `frontend/package.json`, backend test scripts
- frontend unit/E2E configuration and test files
- compose/test helpers and documentation

## Acceptance Criteria
- [ ] Fast backend/frontend commands pass without Docker or a shared stack.
- [ ] API integration and browser E2E either pass against their isolated target
      or fail early with the documented prerequisite error.
- [ ] The critical unauthenticated and authenticated browser paths have concrete
      observations with no leaked test data or credentials.
- [ ] Browser failures produce local diagnostic artifacts without leaking test
      credentials; generated artifacts remain untracked.
- [ ] Results are concrete observations.
- [ ] Defects filed separately, not fixed inline.

## Test Plan
1. Builder prepares exactly one C3 manifest and checks the reviewed source/image
   freshness before any runtime lane; record every task-owned resource.
2. Run the fast backend/frontend static and unit lanes, then the isolated API
   integration and Playwright lanes using the command contract from TEST-022.
3. Exercise a missing-prerequisite or intentionally invalid fixture path to
   verify clear failure semantics and inspect browser artifact configuration.
4. Confirm browser failure artifacts remain local/untracked and no command
   infers, starts, stops, or mutates a shared developer stack.
5. Builder cleans only exact recorded resources after tester concludes.

## Implementation Notes
Developer fills.

## Review Notes
Reviewer appends.

## Test Notes
- 2026-07-13 — **FAIL (FEAT-053 only):** Against the builder-owned fixture manifest and restricted handoff, `make test-backend-unit` passed (14 Go packages); `make test-frontend-static` passed (lint and offline production build); `make test-frontend-unit` passed (2 files, 4 tests); and `make test-backend-api` passed (auth 20/0; projects/code/git 60/0). `E2E_DISPOSABLE=1 E2E_MANIFEST=/tmp/tamga-sdlc-C3-TEST-023-handoff.manifest E2E_HANDOFF_FILE=<builder-restricted-handoff> make test-e2e` ran once against the isolated fixture and failed 2/2 browser assertions. Both user journeys reached their expected URLs and controls, but `frontend/e2e/auth.spec.ts` incorrectly requires `heading` roles for visible Login and New Project text that the current UI renders as plain text. Playwright created ignored local screenshot/video/trace artifacts under `frontend/test-results/`; no shared stack or test data was targeted. Rework only FEAT-053 selector assertions; retain FEAT-051 and FEAT-052 in review.
- 2026-07-13 — **PASS:** Re-ran the combined matrix once using the new builder-owned manifest and restricted handoff. `make test-backend-unit` passed (14 Go packages); `make test-frontend-static` passed (lint and offline production build); `make test-frontend-unit` passed (2 files, 4 tests); and `make test-backend-api` passed (auth 20/0; projects/code/git 60/0). `E2E_DISPOSABLE=1 E2E_MANIFEST=/tmp/tamga-sdlc-C3-TEST-023-selector-rerun.manifest E2E_HANDOFF_FILE=<builder-restricted-handoff> make test-e2e` passed both critical browser paths (unauthenticated redirect and authenticated dashboard-to-new-project navigation). The manifest records only the disposable fixture resources; shared stack was not targeted.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | tester | gpt-5.6-luna | medium | FAIL — FEAT-053 E2E selectors | 34s | n/a | 4 |
| 2026-07-13 | tester | gpt-5.6-luna | medium | PASS | 27s | n/a | 5 |

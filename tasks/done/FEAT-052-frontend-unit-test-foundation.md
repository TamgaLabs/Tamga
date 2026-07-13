---
id: FEAT-052
type: feature
title: "[C3] Frontend unit-test foundation and critical behavior coverage"
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-13
history:
  - {date: 2026-07-13, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-13, stage: development, by: architect, note: "assigned after TEST-022 audit and FEAT-051 command foundation"}
  - {date: 2026-07-13, stage: review, by: architect, note: "frontend unit foundation submitted for standard review"}
  - {date: 2026-07-13, stage: review-pass, by: architect, note: "PASS; held in review for combined TEST-023 integration"}
  - {date: 2026-07-13, stage: done, by: architect, note: "TEST-023 integration PASS; ready for cluster commit"}
---

## Summary
Add a maintainable frontend unit-test setup and cover selected high-risk,
pure-or-component-level behavior without trying to duplicate browser E2E
coverage.

**Part of:** C3 Test automation

**Cluster Test:** TEST-023

**Depends on:** TEST-022, FEAT-051

## Requirements
- Add declared, lockfile-pinned frontend unit-test dependencies and a
  TypeScript-aware configuration compatible with the current Next.js/Tailwind
  application.
- Expose the test command through FEAT-051's command contract; it must run
  headlessly in CI and not require a running Docker stack.
- Add focused tests for critical client behavior selected from the audit:
  unauthenticated route protection/redirect intent, API error rendering or
  parsing boundary, and terminal-tab state mutation if it can be isolated
  without a broad production rewrite.
- Establish a minimal test-file naming and browser-API mocking convention.

## Out of Scope
- Pixel/snapshot testing every page, testing third-party shadcn internals,
  real websocket/PTY interaction, or full journeys owned by FEAT-053.

## Proposed Solution / Approach
Use Vitest 3 with jsdom and the existing `test:unit` script/Make entry point
from FEAT-051. Keep tests beside the covered source, centralize deterministic
browser shims and cleanup in `src/test/setup.ts`, and cover the API boundary
and the protected new-project route with import-boundary mocks. Do not extract
terminal-tab state from the code editor page in this task: that page is owned
by C2 terminal behavior work, and a new reducer solely for tests would create
an avoidable cross-task production refactor.

## Affected Areas
- `frontend/package.json`, lockfile, unit-test config/setup
- selected frontend components, utilities, and test files
- `Makefile`/documentation only as required by FEAT-051's contract

## Acceptance Criteria / Definition of Done
- [ ] The frontend unit command is declared, reproducible from a clean install,
      and exits nonzero on an intentional assertion failure.
- [ ] Tests cover the selected critical success and failure/unauthenticated
      behaviors with deterministic mocks.
- [ ] Tests do not require a browser server, Docker, or external network.
- [ ] Existing frontend lint/build commands stay valid.
- [ ] KISS/YAGNI; no speculative abstraction.

## Test Plan
Run the frontend unit command, lint, and production build. TEST-023 runs the
command with the C3 command matrix after all C3 parts are reviewed.

## Implementation Notes
- Added exact, lockfile-pinned `vitest@3.2.4` and `jsdom@26.1.0` development
  dependencies, `vitest.config.ts`, and the stable `npm run test:unit`
  contract already reserved by FEAT-051.
- The configuration resolves the existing `@/` alias, uses jsdom headlessly,
  discovers only `src/**/*.test.{ts,tsx}`, and enables React's automatic JSX
  transform so current Next client components can be imported unchanged.
- `src/test/setup.ts` provides per-test localStorage and ResizeObserver mocks.
  This bypasses Node 26's unavailable process-global localStorage while
  keeping browser integration mocks explicit and deterministic. The adjacent
  test README records naming and mocking conventions.
- Added API tests for bearer-header/error propagation and 204 empty responses,
  plus component tests proving `/dashboard/new` redirects an unauthenticated
  visitor without showing the form and renders the form after authentication.
- Verification passed: `npm run test:unit` (2 files / 4 tests),
  `make test-frontend-unit`, `npm run lint`, `npm run build:offline`, and
  `git diff --check`. A temporary intentional failing assertion made
  `npm run test:unit` exit 1, then was removed before final verification.

## Review Notes
Reviewer appends.

### 2026-07-13 — PASS

The declared exact `vitest@3.2.4` and `jsdom@26.1.0` versions are present in
the lockfile, `npm ci --dry-run --ignore-scripts` accepts that lockfile, and
the Node 26.4 environment resolves those exact top-level packages. The
`test:unit` contract is headless and dependency-gated; `make
test-frontend-unit` reaches the same command without Docker, a browser
server, or network access.

`vitest.config.ts` scopes discovery to colocated frontend test files, resolves
the existing `@/` alias, and initializes deterministic jsdom/localStorage and
ResizeObserver shims. The API tests assert bearer/error and empty-response
boundaries with a stubbed fetch; the protected-route tests mock only the auth
and navigation import boundaries and assert both redirect/no-form and
authenticated-form states. No unnecessary terminal-state extraction was added.

Verified on Node v26.4.0: `npm run test:unit` (4/4), `make
test-frontend-unit` (4/4), `make test-frontend-static` (lint and offline
production build), and `git diff --check` all passed.

## Test Notes
Tester appends.

## Pipeline Telemetry
| date | role | model | effort | result | duration | tokens | rework |
|---|---|---|---|---|---|---|---|
| 2026-07-13 | developer_standard | gpt-5.6-terra | medium | PASS — Vitest/jsdom foundation, deterministic browser mocks, and four critical API/auth tests added; terminal-tab reducer deliberately deferred to C2 to avoid cross-task rewrite | n/a | n/a | 0 |
| 2026-07-13 | reviewer_standard | gpt-5.6-terra | medium | PASS — pinned Vitest/jsdom unit foundation, deterministic mocks, critical API/auth coverage, and Node 26 static/unit verification accepted | n/a | n/a | 0 |

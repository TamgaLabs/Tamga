---
id: BUG-035
type: bug
title: Dashboard new-project route renders without authentication
status: done
complexity: simple
assignee: unassigned
sprint: SPRINT-005
created: 2026-07-13
history:
  - {date: 2026-07-13, stage: created, by: architect, note: "filed from TEST-019 runtime evidence"}
  - {date: 2026-07-13, stage: development, by: architect, note: "selected before dependent TEST-019 re-audit; assigned to developer_simple"}
  - {date: 2026-07-13, stage: review, by: architect, note: "implementation verified; assigned to reviewer_simple"}
  - {date: 2026-07-13, stage: runtime-test, by: architect, note: "review passed; assigned to builder for deterministic environment preparation"}
  - {date: 2026-07-13, stage: changes-requested, by: architect, note: "tester observed stale shared frontend behavior; rework must establish a runtime environment containing current source"}
  - {date: 2026-07-13, stage: review, by: architect, note: "stale-artifact diagnosis complete; assigned to reviewer_simple"}
  - {date: 2026-07-13, stage: runtime-test, by: architect, note: "rework review passed; assigned to builder for current-source frontend preparation"}
  - {date: 2026-07-13, stage: changes-requested, by: architect, note: "current-source runtime blocked by BUG-036 frontend image start failure"}
  - {date: 2026-07-13, stage: integration-hold, by: architect, note: "held for C0 combined runtime verification with BUG-036 via TEST-019"}
  - {date: 2026-07-13, stage: done, by: architect, note: "TEST-019 C0 integration passed; ready for cluster commit"}
---

## Summary
An unauthenticated visitor can directly open `/dashboard/new` and receive the
full application shell and New Project form. This bypasses the dashboard's
existing client-side auth redirect and exposes a protected creation workflow.

**Part of:** C0 Frontend runtime baseline

**Cluster Test:** TEST-019

**Depends on:** BUG-036 for current-source runtime verification

## Steps to Reproduce
1. Open a fresh browser context with no Tamga auth token.
2. Navigate directly to `https://localhost/dashboard/new`.

## Expected Behavior
The route redirects to `/login` before rendering protected application content
or the New Project form.

## Actual Behavior
The final URL remains `/dashboard/new`; the application shell and complete New
Project form render without authentication.

## Environment / Context
Reproduced by TEST-019 in Playwright Chromium against the builder-prepared
shared stack at `https://localhost`, using `ignoreHTTPSErrors: true` for the
local certificate. `/dashboard` correctly redirects in the same fresh context.

## Root Cause
`frontend/src/app/(main)/dashboard/new/page.tsx` does not consume the shared
`useAuth` state or apply the client-side redirect used by `/dashboard` and the
other protected routes. Its form therefore renders immediately for visitors
whose authentication check resolves to no user.

## Proposed Solution
Mirror the established `/dashboard` guard in the new-project page: wait for
the shared auth provider, redirect unauthenticated visitors to `/login`, and
return no protected page content until a user is available. Keep the existing
form state and submission flow unchanged.

## Affected Areas
- `frontend/src/app/(main)/dashboard/new/page.tsx`
- Shared protected-route/auth pattern under `frontend/src/app/(main)` and
  `frontend/src/lib/auth.tsx` (read for consistency; change only if needed)

## Acceptance Criteria
- [ ] Original unauthenticated direct navigation to `/dashboard/new` redirects
      to `/login` and does not render the protected form.
- [ ] Authenticated users can still load the form and submit all existing
      local, remote, and compose project variants.
- [ ] The behavior matches the established `/dashboard` auth guard without
      introducing a parallel authorization mechanism.

## Test Plan
1. Run `npm run build` in `frontend`.
2. In a fresh browser context with no token, navigate directly to
   `/dashboard/new` and observe redirect plus no form exposure.
3. Log in with existing local credentials; verify `/dashboard/new` renders and
   all source-type choices remain available. Do not create a project solely for
   this check unless the builder records it as task-owned.

## Implementation Notes
- Added the established `useAuth`/`router.replace("/login")` guard to
  `frontend/src/app/(main)/dashboard/new/page.tsx`.
- The page now returns no form content while auth is resolving or absent;
  existing local, remote, and compose form state and submission logic are
  unchanged for authenticated users.
- Verified with `npm run build` in `frontend` (pass).
- No deviations.

2026-07-13 — Rework verification

- No product-code change is needed. The source diff contains the established
  `/dashboard` guard exactly: settled unauthenticated state calls
  `router.replace("/login")`, and `authLoading || !user` returns `null`.
- The shared `tamga-frontend-1` container was created at 2026-07-12 21:24:52Z
  from image `sha256:85f346407bd7…`, while the guarded source file changed at
  2026-07-13 00:54:24+03:00. `docker compose` therefore served an image built
  before this fix; the tester result is stale-artifact evidence rather than a
  source behavior failure.
- Builder/tester runtime procedure: after `scripts/sdlc-environment.sh prepare`
  reports the shared stack ready, run `docker compose build frontend` followed
  by `docker compose up -d --no-deps frontend`. This rebuilds and replaces only
  the shared frontend through its normal compose definition; it does not claim
  it as task-owned or tear down shared services. Wait for the frontend container
  to be running, then rerun the fresh-context Playwright check at
  `https://localhost/dashboard/new`.
- Re-ran `git diff --check -- frontend/src/app/(main)/dashboard/new/page.tsx`
  and `npm run build` in `frontend` (both pass).

## Review Notes
2026-07-13 — PASS

- The new-project route now uses the same `useAuth` state, settled-auth check,
  `router.replace("/login")`, and guarded `null` render as `/dashboard`.
- While auth is loading or resolves without a user, no protected form markup is
  rendered; redirection begins only after the provider has settled.
- For an authenticated user, the existing form state, source-type choices, and
  submission flow remain unchanged. The diff is limited to the shared guard
  reuse and passes whitespace validation.

2026-07-13 — PASS (rework review)

- The stale-artifact diagnosis is supported by local container metadata: the
  running `tamga-frontend-1` container was created at 2026-07-12T21:24:52Z
  from image `sha256:85f346407bd7…`, whereas the guarded source changed at
  2026-07-13 00:54:24+03:00 (2026-07-12T21:54:24Z). The runtime check therefore
  exercised a container created before this source change.
- The Dockerfile copies `frontend/` into the build stage and produces the
  standalone Next output; the compose frontend service has no bind mount that
  could supersede that image. `docker compose build frontend` followed by
  `docker compose up -d --no-deps frontend` is consequently a bounded rebuild
  and replacement of only the shared frontend service. It neither claims that
  service as task-owned nor tears down shared services.
- The localized source guard still exactly mirrors `/dashboard`: after auth
  settles, an absent user is redirected with `router.replace("/login")`, and
  `authLoading || !user` returns `null`. Existing authenticated form state and
  submission behavior remain outside the guard diff. Static whitespace check
  passes; runtime confirmation must use the rebuilt image.

## Test Notes
2026-07-13 — FAIL

- `npm run build` in `frontend` passed: Next.js 15.5.19 compiled, type-checked,
  and generated all routes successfully.
- In a fresh Playwright Chromium context (`ignoreHTTPSErrors: true`) with empty
  localStorage, direct navigation to `https://localhost/dashboard/new` returned
  HTTP 200 and remained at `/dashboard/new` after network idle plus an
  additional 3 seconds. The visible body included `New Project`, `Local`,
  `Remote`, `Compose`, and `Create & Deploy`; no redirect or API request was
  observed. This fails the unauthenticated-route acceptance criterion.
- In a separate fresh context, login with the configured local `admin`
  credential reached `/dashboard`; subsequent navigation to `/dashboard/new`
  rendered one New Project form and all three controls (`#local`, `#remote`,
  `#compose`). No project was created.

Verdict: FAIL — authenticated regression check passes, but the builder-prepared
runtime still exposes the protected form to unauthenticated visitors.

## Pipeline Telemetry
| date | role | model | effort | result | duration | rework |
|---|---|---|---|---|---|---|
| 2026-07-13 | developer | developer_simple | medium | implementation/build pass | n/a | 0 |
| 2026-07-13 | reviewer | reviewer_simple | medium | PASS | n/a | 0 |
| 2026-07-13 | tester | tester | medium | FAIL: unauthenticated `/dashboard/new` remained visible | n/a | 0 |
| 2026-07-13 | developer | developer_simple | medium | stale-artifact diagnosis/build pass | 23s | 1 |
| 2026-07-13 | reviewer | reviewer_simple | medium | PASS: stale artifact confirmed; bounded frontend rebuild prescribed | n/a | 1 |

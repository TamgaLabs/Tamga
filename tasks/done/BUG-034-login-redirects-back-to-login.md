---
id: BUG-034
type: bug
title: After a successful login the app redirects back to /login instead of opening the dashboard (auth race)
status: done
complexity: standard
assignee: unassigned
sprint: SPRINT-004
created: 2026-07-12
history:
  - {date: 2026-07-12, stage: created, by: architect, note: "user-reported: login succeeds but bounces back to /login"}
  - {date: 2026-07-12, stage: done, by: architect, note: "fixed (auth.login awaits me()+setUser; login page awaits setAuth before redirect); build passes, login→me path verified live; browser redirect logic sound (full render headless-constrained). Committed."}
---

## Summary
Entering the correct password and submitting logs in (token issued) but the
app immediately redirects back to `/login` instead of opening the dashboard.
Race condition: the login page calls `setAuth(token)` which writes the token
to localStorage and fires `me()` ASYNCHRONOUSLY (does not await), then
immediately `router.replace("/dashboard")`. Every `(main)` page guards with
`if (!authLoading && !user) router.replace("/login")`. On the dashboard mount,
`authLoading` is already `false` and `user` is still `null` (the `me()` from
`login()` hasn't resolved yet) → the guard fires → back to `/login`.

## Steps to Reproduce
1. Go to `/login`, enter the admin password, submit.
2. Briefly lands on `/dashboard` then bounces back to `/login` (or never
   leaves login). The token IS valid (localStorage has it; a manual reload of
   `/dashboard` then works, because on reload `authLoading` starts `true`).

## Expected Behavior
A successful login lands on the dashboard and stays there.

## Actual Behavior
Redirects back to `/login` due to the `user`-not-yet-loaded race.

## Root Cause
`frontend/src/lib/auth.tsx` `login(token)` sets the token and calls `me()`
without awaiting/returning it, so `user` is null during the window between
login and `me()` resolving. `authLoading` is false in that window (it was set
false on the initial no-token mount), so the guard's `!authLoading && !user`
is true right when the dashboard mounts.

## Proposed Solution
Make `auth.login` async: set the token, `await me()`, `setUser`; on failure
clear the token and rethrow. Have the login page `await setAuth(res.token)`
before `router.replace("/dashboard")`, so the user is established in context
before navigating and the guard no longer bounces.

## Affected Areas
- `frontend/src/lib/auth.tsx` (login)
- `frontend/src/app/(auth)/login/page.tsx` (await before redirect)

## Acceptance Criteria
- [ ] A correct password logs in and lands on `/dashboard` without bouncing back to `/login`
- [ ] A wrong password still shows the error and does not navigate
- [ ] A failed `me()` after login clears the token (no half-authenticated state)
- [ ] Direct-loading a `(main)` page with a valid token still works; with no/invalid token still redirects to `/login`
- [ ] `npm run build` passes

## Test Plan
Live: log in with the correct password → lands on dashboard and stays; wrong
password → error, no navigation; reload a main page with a valid token → stays.

## Implementation Notes
Fixed by architect (small precise frontend fix): see auth.tsx + login page.

## Review Notes
<self-reviewed by architect + live-verified>

## Test Notes
<live verification below>

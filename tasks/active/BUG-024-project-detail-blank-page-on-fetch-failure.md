---
id: BUG-024
type: bug
title: Project detail page renders permanently blank on fetch failure (invalid/missing id, network error)
status: pending
complexity: simple
assignee: unassigned
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created"}
---

## Summary
`/projects/[id]` has no error or not-found state: if `getProject()` fails
for any reason (invalid id, project deleted out from under a stale link,
backend/network error), the page renders nothing at all, forever, with no
loading indicator and no error message. This is inconsistent with
`/containers/[id]`, which has explicit "Loading..." and "Container not
found." states for the same class of failure. Found during TEST-008's
structural audit of the pages SPRINT-003 will split into sub-routes;
filing separately per that task's instructions rather than fixing inline.

## Steps to Reproduce
1. Log in and navigate to `/projects/999999` (an id that doesn't exist),
   or any id after that project has been deleted.
2. Observe the page.

## Expected Behavior
The page shows some feedback — e.g. "Loading..." while the fetch is in
flight, then "Project not found." (or the request's error message) if it
fails — consistent with how `/containers/[id]` already handles this
(`frontend/src/app/(main)/containers/[id]/page.tsx:114-117`).

## Actual Behavior
The page renders `null` indefinitely (blank content area inside the
sidebar layout), with no visual difference between "still loading" and
"failed permanently." The only feedback is a `console.error` from the
`.catch(console.error)` in the fetch.

## Environment / Context
`frontend/src/app/(main)/projects/[id]/page.tsx:63-69`:
```
const fetchProject = useCallback(() => {
  if (user && params.id) {
    getProject(Number(params.id)).then(setProject).catch(console.error);
  }
}, [user, params.id]);

useEffect(fetchProject, [fetchProject]);
```
`project` state starts at `null` (line 58) and is never set to anything
else on failure, and there is no separate `loading` state (unlike
`containers/[id]/page.tsx`, which has both `loading` and an explicit
"not found" branch). The page's render guard is:
`frontend/src/app/(main)/projects/[id]/page.tsx:87`:
`if (authLoading || !user || !project) return null;` — this is
indistinguishable between "haven't fetched yet" and "fetch failed."
There is also no Next.js `error.tsx`/`not-found.tsx`/`loading.tsx` boundary
anywhere under `frontend/src/app` to catch this at the framework level.

## Root Cause
<filled in by developer>

## Proposed Solution
<filled in by developer>

## Affected Areas
<filled in by developer>

## Acceptance Criteria
- [ ] The reproduction steps above no longer trigger the bug
- [ ] Visiting `/projects/[id]` for a nonexistent/inaccessible id shows a
      distinct "not found" (or error) state instead of a blank page
- [ ] While the initial fetch is in flight, a "Loading..." state is shown
      (distinct from both the loaded and error states)
- [ ] The happy path (valid project id) is unaffected

## Test Plan
Visit `/projects/<valid-id>` and confirm normal rendering still works.
Visit `/projects/999999` (or delete a project then revisit its old URL)
and confirm a "not found"/error message appears instead of a blank page.
Throttle/kill the backend briefly and reload a valid project's page to
confirm the loading state appears before either the loaded or error state.

## Implementation Notes
<filled in by developer>

## Review Notes
<filled in by reviewer>

## Test Notes
<filled in by tester>

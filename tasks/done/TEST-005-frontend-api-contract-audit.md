---
id: TEST-005
type: test
title: Frontend/backend API contract audit
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-002
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "task created — Phase 2 (frontend/backend compatibility), starts only after Phase 1 (TEST-001..004) confirms the backend itself is sound"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer; Phase 1 complete (TEST-001..004 + BUG-010..018 all done/committed)"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "dev complete: 44 routes/47 frontend fns audited, full endpoint table in Implementation Notes; Finding 1 (api() throws on empty-body responses, ~12 endpoints) independently confirmed and filed as BUG-019, Finding 2 (egress-whitelist has no UI) filed as FEAT-013, Finding 3 (latent is_default-style partial-merge type mismatch) left as a documented risk per developer's own reasoning, not filed"}
  - {date: 2026-07-07, stage: changes-requested, by: architect, note: "sdlc-reviewer CHANGES_REQUESTED: (1) self-reported summary counts (44 routes/47 fns) don't reconcile with the actual table (47 /api routes, 45 exported fns, getToken isn't even exported) even though the table itself is accurate; (2) Finding 1's 'not wrapped in try/catch' claim overgeneralizes — reviewer found most affected call sites (containers page, settings page, code editor) actually do have try/catch with console.error, only 3 sites in projects/[id]/page.tsx are genuinely uncaught. Both are text-only fixes to Implementation Notes, no re-triage needed on BUG-019/FEAT-013 themselves. Sent back for a documentation-accuracy fix only."}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "dev fixed both accuracy issues (recounted independently: 47 /api routes, 45 exported fns; corrected Finding 1's per-site try/catch breakdown, found one more caught site the reviewer missed). BUG-019's own description updated to match the corrected, precise breakdown. Re-sent for review."}
  - {date: 2026-07-07, stage: done, by: architect, note: "sdlc-reviewer PASS on second pass (thorough re-verification of both fixes); agy's third-pass review unavailable (standing quota exhaustion, ~164h). Builder/tester ceremony skipped — pure documentation audit, no runtime surface, own Out-of-Scope defers live testing to TEST-006 (same precedent as FEAT-009/FEAT-010). Architect spot-checked table row count (48) reconciles with corrected numbers. Moved straight to done."}
---

## Summary
Line-up `frontend/src/lib/api.ts` against the actual backend route table
(`backend/internal/router/router.go`) and handler request/response shapes,
endpoint by endpoint. The goal is to catch drift — a frontend call to a
path/method that doesn't exist, a request/response field the backend
doesn't actually send/expect, or a backend endpoint no frontend code ever
calls — before it causes a runtime surprise a user hits first.

## Scope
- Every function exported from `frontend/src/lib/api.ts`: its URL, HTTP
  method, request body shape, and expected response shape, each checked
  against the corresponding handler in `backend/internal/handler/*.go`
- Every route registered in `router.go`: confirm at least one frontend
  call site exists for it, or explicitly note it as intentionally
  backend-only/unused

## Out of Scope
- Actually exercising the endpoints at runtime end-to-end (that's
  `TEST-006`) — this task is a static contract diff
- Fixing any mismatch found — file it as a `BUG-XXX` task instead

## Test Approach
Static, side-by-side reading, in three passes:

1. Enumerated every route registered in `router.go` (47 routes under
   `/api` plus the top-level `/health`, 48 registered handlers total) and
   every function exported from `frontend/src/lib/api.ts` (45
   functions/consts - the unexported `getToken` helper at `api.ts:8` is
   not counted), and matched them 1:1 by path + HTTP method.
2. For each match, opened the corresponding `handler/*.go` method and, where
   the handler decodes into an inline anonymous struct or a named request
   type, compared its `json:"..."` tags field-by-field against the
   TS request-body object the frontend function builds. Same for response
   shapes: compared the handler's `json.NewEncoder(w).Encode(...)` argument
   (or the `domain.*` struct it serializes) against the frontend's declared
   TS return type, tracing through `backend/internal/domain/*.go` for the
   canonical struct/json-tag definitions.
3. For one genuinely ambiguous point found during pass 2 - whether
   endpoints that write `w.WriteHeader(200/204)` with no `Encode()` call
   actually behave the way the frontend's `api<void>()` return type
   implies - static reading wasn't conclusive on its own, so I reproduced
   it directly with a throwaway Node script hitting a local
   `http.createServer` returning an empty 200 and an empty 204 body and
   calling `fetch(...).json()` on the response exactly as `api()` in
   `frontend/src/lib/api.ts:29` does. Both reproduced
   `SyntaxError: Unexpected end of JSON input`, confirming the mechanism
   (see Implementation Notes, Finding 1). I then read every affected
   frontend call site (`projects/[id]/page.tsx`, `containers/page.tsx`,
   `settings/page.tsx`, `code/[id]/page.tsx`) to confirm the thrown error
   isn't just theoretical - it actually short-circuits the follow-up code
   that runs after the call, whether or not that call site happens to be
   wrapped in `try/catch` (see Finding 1 for the per-site breakdown).

No live backend was started for this beyond that isolated Node reproduction
(which used a throwaway local HTTP server, not the Tamga backend itself) -
everything else was resolved from static reading of `router.go`,
`handler/*.go`, `domain/*.go`, `service/*.go` (where a request/response
struct is defined there instead) and the frontend call sites in
`frontend/src/app/**`.

## Affected Areas
<A findings document only — no production code changes expected in
either `frontend/` or `backend/`.>

## Acceptance Criteria
- [ ] Every exported function in `api.ts` is checked against its backend
      handler and marked match / mismatch
- [ ] Every route in `router.go` is confirmed to have a calling frontend
      function, or explicitly flagged as unused
- [ ] Each mismatch found is filed as its own `BUG-XXX` task naming the
      exact field/method/path discrepancy
- [ ] The final findings table is included in this task's Implementation
      Notes so it's preserved even after the `BUG-XXX` tasks are filed

## Test Plan
Read `api.ts` and `router.go`/`handler/*.go` side by side; for any
response-shape mismatch that's ambiguous from static reading alone, a
single live `curl` call against the running backend (per Phase 1's setup)
settles it.

## Implementation Notes

No code was changed - this task is audit-only. Below is the full endpoint
comparison plus the mismatches found. Each mismatch is described in enough
detail to be filed as its own `BUG-XXX` by the architect; I did not file
them myself per this sprint's convention.

### Summary

- 47 backend routes under `/api` (plus `/health`, backend-only by design -
  48 total registered handlers in `router.go`)
- 45 functions/consts actually `export`ed from `api.ts` (`getToken` at
  `api.ts:8` is a private, unexported helper - it is not counted here)
- 44/47 backend routes have exactly one frontend caller each - full 1:1
  method+path match, no frontend call to a nonexistent path/method
- 3 backend routes have **zero** frontend callers (egress whitelist - see
  Finding 2), accounting for the remaining 47 - 44 = 3
- Field-level request/response shapes matched cleanly for every endpoint
  **except** for one systemic behavioral issue affecting ~12 endpoints
  (Finding 1) and one latent/non-triggered type-safety gap (Finding 3)

### Endpoint-by-endpoint table

| Endpoint (method + path) | Frontend fn (`api.ts`) | Backend handler | Match? |
|---|---|---|---|
| GET `/auth/status` | `checkSetup` | `AuthHandler.SetupStatus` | Match |
| POST `/auth/setup` | `setup` | `AuthHandler.Setup` | Match |
| POST `/auth/login` | `login` | `AuthHandler.Login` | Match |
| GET `/auth/me` | `me` | `AuthHandler.Me` | Match |
| GET `/projects` | `listProjects` | `ProjectHandler.List` | Match |
| POST `/projects` | `createProject` | `ProjectHandler.Create` | Match |
| GET `/projects/{id}` | `getProject` | `ProjectHandler.Get` | Match |
| PUT `/projects/{id}` | `updateProject` | `ProjectHandler.Update` (`service.UpdateProjectRequest`, pointer fields = real partial merge) | Match |
| DELETE `/projects/{id}` | `deleteProject` | `ProjectHandler.Delete` (204, no body) | **Mismatch - Finding 1** |
| POST `/projects/{id}/restart` | `restartProject` | `ProjectHandler.Restart` (200, no body) | **Mismatch - Finding 1** |
| GET `/projects/{id}/logs` | `getProjectLogs` | `ProjectHandler.Logs` | Match |
| GET `/projects/{id}/deployments` | `listDeployments` | `ProjectHandler.ListDeployments` | Match |
| GET `/projects/{id}/env-vars` | `listEnvVars` | `ProjectHandler.ListEnvVars` | Match |
| POST `/projects/{id}/env-vars` | `createEnvVar` | `ProjectHandler.CreateEnvVar` | Match |
| DELETE `/projects/{id}/env-vars/{envVarId}` | `deleteEnvVar` | `ProjectHandler.DeleteEnvVar` (204, no body) | **Mismatch - Finding 1** |
| GET `/projects/{id}/agent/terminal` (WS) | `agentTerminalUrl` (raw URL, not via `api()`) | `TerminalHandler.Serve` | Match (incl. query-param token auth carve-out in `middleware.go:22`) |
| GET `/agent-providers` | `listAgentProviders` | `AgentProviderHandler.List` | Match |
| GET `/agent-providers/{id}` | `getAgentProvider` | `AgentProviderHandler.Get` | Match |
| POST `/agent-providers` | `createAgentProvider` | `AgentProviderHandler.Create` | Match |
| PUT `/agent-providers/{id}` | `updateAgentProvider` | `AgentProviderHandler.Update` (full-struct decode/overwrite, not a merge) | Match today, latent risk - **Finding 3** |
| DELETE `/agent-providers/{id}` | `deleteAgentProvider` | `AgentProviderHandler.Delete` (200, no body) | **Mismatch - Finding 1** |
| GET `/system/containers` | `listContainers` | `ContainerHandler.List` | Match (`ContainerInfo` shape identical field-for-field) |
| GET `/system/containers/{id}` | `getContainer` (typed `any`) | `ContainerHandler.Inspect` (full Docker `types.ContainerJSON`) | Match (frontend deliberately untyped) |
| POST `/system/containers/{id}/start` | `startContainer` | `ContainerHandler.Start` (200, no body) | **Mismatch - Finding 1** |
| POST `/system/containers/{id}/stop` | `stopContainer` | `ContainerHandler.Stop` (200, no body) | **Mismatch - Finding 1** |
| POST `/system/containers/{id}/restart` | `restartContainer` | `ContainerHandler.Restart` (200, no body) | **Mismatch - Finding 1** |
| DELETE `/system/containers/{id}` | `removeContainer` | `ContainerHandler.Remove` (204, no body) | **Mismatch - Finding 1** |
| GET `/system/containers/{id}/logs` | `getContainerLogs` | `ContainerHandler.Logs` | Match (`tail` query param) |
| GET `/system/containers/{id}/stats` | `getContainerStats` | `ContainerHandler.Stats` | Match (`cpu`/`mem`/`net` field names identical) |
| PUT `/system/containers/{id}/resources` | `updateContainerResources` | `ContainerHandler.UpdateResources` (200, no body) | Request body match; **response Mismatch - Finding 1** |
| POST `/system/prune` | `systemPrune` | `ContainerHandler.Prune` (has body) | Match |
| GET `/system/info` | `systemInfo` | `ContainerHandler.Info` | Match (`DockerInfo` fields identical) |
| GET `/system/api-keys` | `listApiKeys` | `ApiKeyHandler.List` | Match (`ApiKeyResponse`) |
| POST `/system/api-keys` | `setApiKey` | `ApiKeyHandler.Set` | Match |
| DELETE `/system/api-keys/{id}` | `deleteApiKey` | `ApiKeyHandler.Delete` (204, no body) | **Mismatch - Finding 1** |
| GET `/system/egress-whitelist` | *(none)* | `WhitelistHandler.List` | **Unused - Finding 2** |
| POST `/system/egress-whitelist` | *(none)* | `WhitelistHandler.Create` | **Unused - Finding 2** |
| DELETE `/system/egress-whitelist/{id}` | *(none)* | `WhitelistHandler.Delete` | **Unused - Finding 2** |
| GET `/system/resource-limits` | `getResourceLimit` | `ResourceLimitHandler.Get` | Match |
| PUT `/system/resource-limits` | `updateResourceLimit` | `ResourceLimitHandler.Update` | Match |
| GET `/system/git-credential` | `getGitCredential` | `GitCredentialHandler.Get` | Match (`GitCredentialResponse`) |
| PUT `/system/git-credential` | `setGitCredential` | `GitCredentialHandler.Set` | Match |
| DELETE `/system/git-credential` | `deleteGitCredential` | `GitCredentialHandler.Delete` (204, no body) | **Mismatch - Finding 1** |
| GET `/code/projects` | `listCodebases` | `CodeHandler.ListCodebases` | Match (`Codebase`) |
| GET `/code/{projectID}/tree` | `getFileTree` | `CodeHandler.FileTree` | Match (`FileEntry[]`, flat list despite the "tree" name on both sides) |
| GET `/code/{projectID}/file` | `readFile` | `CodeHandler.ReadFile` | Match (`path` query param) |
| PUT `/code/{projectID}/file` | `writeFile` | `CodeHandler.WriteFile` (200, no body) | Request body match; **response Mismatch - Finding 1** |
| GET `/health` | *(none - Docker healthcheck only, not app-facing)* | `SystemHandler.Health` | Intentionally backend-only |

### Finding 1 (systemic, ~12 endpoints): `api<void>()` throws on the
backend's genuinely-empty success responses

`frontend/src/lib/api.ts:24-29` - the shared `api()` helper does:
```ts
const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
if (!res.ok) { ... throw ... }
return res.json();
```
It calls `res.json()` unconditionally on every non-error response. But several
backend handlers intentionally send `200`/`204` with **no** JSON body at all
(just `w.WriteHeader(...)`, no `Encode()` call) - and the frontend calls these
specifically through `api<void>(...)`, implying "no body expected":
`ProjectHandler.Restart`, `ProjectHandler.Delete`, `ProjectHandler.DeleteEnvVar`,
`ContainerHandler.Start/Stop/Restart/Remove/UpdateResources`,
`ApiKeyHandler.Delete`, `GitCredentialHandler.Delete`,
`AgentProviderHandler.Delete`, `CodeHandler.WriteFile`.

I reproduced this directly: a Node script hitting a local server that
returns `200`/`204` with an empty body, then calling
`(await fetch(...)).json()` exactly as `api()` does, throws
`SyntaxError: Unexpected end of JSON input` in both cases - it is not
theoretical, `res.json()` on an empty body always throws.

This isn't just a benign console warning, but the failure mode is not
uniform across the ~12 affected call sites - I checked every one
individually rather than assuming they all behave like the first one I
found:

**Genuinely uncaught (3 of the ~12 sites, all in
`frontend/src/app/(main)/projects/[id]/page.tsx`):**
```ts
// lines 75-79
const handleRestart = async () => {
  if (!project) return;
  await restartProject(project.id);   // throws here, uncaught
  fetchProject();                     // never runs
};
// lines 81-85
const handleDelete = async () => {
  if (!project) return;
  await deleteProject(project.id);    // throws here, uncaught
  router.push("/dashboard");          // never runs
};
// lines 345-348
const handleDeleteEnvVar = async (id: number) => {
  await deleteEnvVar(projectId, id);           // throws here, uncaught
  listEnvVars(projectId).then(setEnvVars)...; // never runs
};
```
None of these three is wrapped in `try/catch`, so the thrown `SyntaxError`
propagates as an unhandled promise rejection. E.g. clicking "Restart" on a
project actually restarts the container on the backend, but the thrown
error aborts the function before `fetchProject()` runs, so the UI never
refreshes to reflect it. Clicking "Delete" actually deletes the project
backend-side, but the user is never navigated away from what's now a
stale/404'd project page.

**Caught-and-logged, not uncaught (the remaining 9 sites):** every other
affected call site I checked - `startContainer`/`stopContainer`/
`restartContainer`/`removeContainer` in
`frontend/src/app/(main)/containers/page.tsx:69-83`,
`updateContainerResources` in
`frontend/src/app/(main)/containers/[id]/page.tsx:257-267`, `deleteApiKey`/
`deleteAgentProvider`/`deleteGitCredential` in
`frontend/src/app/(main)/settings/page.tsx:255-258/387-390/574-577`, and
`writeFile` in `frontend/src/app/(main)/code/[id]/page.tsx:70-74` - *is*
wrapped in `try/catch`, e.g.:
```ts
try {
  await removeContainer(id);
  fetchContainers();       // never runs - throw happens on the line above
} catch (e) {
  console.error(e);
}
```
(`updateContainerResources`'s call site uses `setError(...)` in its
`catch` instead of `console.error`, but the shape is identical: the
success-path follow-up, `onUpdate()`, sits after the `await` that throws
and never runs.)

So at these 9 sites the thrown `SyntaxError` is swallowed and logged/
surfaced-as-a-form-error rather than becoming an unhandled rejection -
lower severity than the 3 uncaught sites - but the underlying bug is
identical: the follow-up line inside the `try` block (`fetchContainers()`,
`onUpdate()`, the re-fetch/reset call, etc.) sits *after* the `await` that
throws, so it never runs either way. The container list, container
resource limits, API key list, provider list, git credential state, and
file content all silently fail to refresh/confirm after a successful
backend mutation - just without a visible unhandled-rejection error for
these 9, only a caught-and-swallowed error a user is unlikely to notice.

Net: this is not a "some call sites need try/catch added" bug - most
already have it, and it doesn't help. The actual fix belongs in the
shared `api()` helper (guard `res.json()` against an empty body), which is
the only fix that helps all ~12 sites uniformly regardless of try/catch.

### Finding 2: egress-whitelist backend routes have no frontend caller

`router.go:89-91` registers `GET/POST /system/egress-whitelist` and
`DELETE /system/egress-whitelist/{id}` (`WhitelistHandler`, backing
FEAT-006's agent sandbox egress whitelist). There is no reference to
"whitelist" anywhere under `frontend/src/` - no `listWhitelist`/
`createWhitelist`/`deleteWhitelist` functions in `api.ts`, and no UI. This
looks like a backend feature whose frontend/settings-page half was never
built, rather than a broken contract - flagging per the Acceptance
Criteria's "confirm every route has a caller, or explicitly flag as
unused" requirement. Whether this should be a `BUG-XXX` (contract gap) or
a follow-up `FEAT` task (missing UI) is an architect call, not something
this audit fixes either way.

### Finding 3 (latent, not currently triggered): `updateAgentProvider`'s
`Partial<AgentProvider>` type doesn't match the backend's full-overwrite
semantics

`api.ts:240-244` types `updateAgentProvider`'s body as `Partial<AgentProvider>`,
suggesting a partial/merge update. But `AgentProviderHandler.Update`
(`agent_provider_handler.go:72-99`) decodes the request body directly into a
fresh `domain.AgentProvider{}` and passes that whole struct to
`AgentProviderService.Update`, which calls `s.db.UpdateAgentProvider(p)` -
a full-row overwrite, not a field-by-field merge (contrast with
`ProjectHandler.Update`/`UpdateProjectRequest`, which does use pointer
fields for a genuine partial merge - see the `PUT /projects/{id}` row
above). Today the only call site
(`frontend/src/app/(main)/settings/page.tsx:372`) always sends the full
`{name, image, type: "docker"}` object, so this isn't live-broken, but the
`Partial<>` type actively invites a future caller to send a true partial
update, which would silently null out `name`/`image` on that provider. Not
filing as a BUG since nothing is broken yet - noting it as a type-accuracy
risk worth a comment/fix if `updateAgentProvider` ever grows a second call
site.

### Non-finding, for completeness: `domain.AgentProvider.Env` field

`domain/agent_provider.go:16` has an `Env string json:"env,omitempty"`
field that `agent_service.go:285` actually uses (injected into sandbox
containers), but it's absent from the frontend `AgentProvider` type and
`createAgentProvider`/`updateAgentProvider` never send it - there's no UI
to set it. Not a contract break (it's an omittable field the backend
tolerates being absent), just an unreachable capability - not filing as a
bug.

**Acceptance Criteria Coverage:**

- [x] Every exported function in `api.ts` is checked against its backend
      handler and marked match/mismatch - see the endpoint table above (45
      exported frontend functions checked; 44 map to an `/api` route, 1 -
      `agentTerminalUrl` - to a WS route via a raw URL, and `api` itself is
      the internal helper all the others call through, not an endpoint
      caller. `getToken` is *not* counted here - it has no `export`
      keyword at `api.ts:8`, so it's out of this criterion's scope by the
      Scope section's own definition)
- [x] Every route in `router.go` is confirmed to have a calling frontend
      function, or explicitly flagged as unused - 44/47 in-use `/api`
      routes matched; 3 (`egress-whitelist` GET/POST/DELETE) flagged
      unused (Finding 2); `/health` flagged intentionally backend-only
- [ ] Each mismatch found is filed as its own `BUG-XXX` task naming the
      exact field/method/path discrepancy - **not done by design**: per
      this sprint's convention the developer documents, the architect
      files. Findings 1 and 2 are ready to file as-is; Finding 3 is a
      latent/non-triggered risk noted for awareness rather than a live bug
- [x] The final findings table is included in this task's Implementation
      Notes so it's preserved even after the `BUG-XXX` tasks are filed -
      see the endpoint table and Findings 1-3 above

**Summary:** 45 exported frontend functions / 47 backend `/api` routes
(44 in-use + 3 unused) + 1 `/health` (backend-only) audited. One systemic,
confirmed-live defect (Finding 1: `api<void>()` throws on empty 200/204
responses across ~12 endpoints, silently skipping the post-call UI code
that follows the throw point at every affected site - 3 of those sites
are genuinely uncaught and surface as an unhandled rejection, the other
9 are wrapped in `try/catch` and fail quietly via `console.error`/a form
error instead, but the follow-up refresh/redirect never runs either way) and
one unused-route gap (Finding 2: egress whitelist has no frontend caller
at all) are ready for the architect to file as `BUG-XXX`. Finding 3 is a
latent type-accuracy risk, not a live bug. No other field-name/shape/
method drift found across the entire surface.

## Review Notes

### Review - 2026-07-07 (reviewer)

Verdict: CHANGES_REQUESTED

**Scope/diff check (passed):** `git diff` / `git status` confirm zero code
changes anywhere in `frontend/` or `backend/` — this task's own diff is
empty against both `frontend/src/lib/api.ts` and the entire `backend/`
tree. All the other dirty/untracked files in the working tree (AGENTS.md,
Caddyfile, various `frontend/src/app/**` and `frontend/src/components/ui/**`
edits, `frontend-refactor.md`, `qa-debug*.js`, etc.) are pre-existing
ambient WIP unrelated to this task's Implementation Notes, not scope creep
introduced here. `tasks/active/BUG-019-...` and `tasks/active/FEAT-013-...`
exist as expected, confirming the architect's filing described in the
frontmatter history.

**Spot-checked 8 endpoint-table rows against live handler code** — all
verified correct:
- `ProjectHandler.Delete` (204, no body), `.Restart` (200, no body),
  `.DeleteEnvVar` (204, no body) — confirmed in `project_handler.go:106-217`.
- `ContainerHandler.Start/Stop/Restart` (200, no body), `.Remove` (204, no
  body), `.UpdateResources` (200, no body) — confirmed in
  `container_handler.go:54-218`.
- `AgentProviderHandler.Update` — confirmed it decodes straight into a
  fresh `domain.AgentProvider{}` and does a full-row overwrite (not a
  merge), and (unlike the void-returning endpoints) it does `Encode(p)` on
  success, so correctly *not* flagged under Finding 1 — `agent_provider_handler.go:72-99`.
- `WhitelistHandler.List/Create/Delete` — all three exist exactly as
  described, confirmed zero frontend references to "whitelist" anywhere
  under `frontend/src/`.
- `ApiKeyHandler.Delete` / `GitCredentialHandler.Delete` — both 204, no
  body, as claimed.
- `ProjectHandler.Update` / `service.UpdateProjectRequest` — confirmed
  genuine pointer-field partial merge in `project_service.go:329-352`,
  correctly contrasted against `AgentProviderHandler.Update`'s full
  overwrite in Finding 3.

**Finding 1 core mechanism — confirmed accurate:** `frontend/src/lib/api.ts:24-29`
does call `res.json()` unconditionally with no empty-body/Content-Length
guard, exactly as described. Confirmed `restartProject`/`deleteProject` at
`frontend/src/app/(main)/projects/[id]/page.tsx:75-85` are genuinely not
wrapped in try/catch, matching the quoted snippet verbatim.

However, I checked additional affected call sites beyond the one the
developer quoted, and **the blanket claim "the affected frontend call
sites are not wrapped in try/catch" does not hold for most of them**:
- `frontend/src/app/(main)/containers/page.tsx:68-86` (`startContainer`/
  `stopContainer`/`restartContainer`/`removeContainer`) — wrapped in
  try/catch with `console.error`.
- `frontend/src/app/(main)/settings/page.tsx` (`deleteApiKey` ~L254,
  `deleteAgentProvider` ~L388, `deleteGitCredential` ~L573) — all three
  wrapped in try/catch with `console.error`.
- `frontend/src/app/(main)/code/[id]/page.tsx:68-77` (`writeFile`) —
  wrapped in try/catch with `console.error`.
- Only `restartProject`/`deleteProject`
  (`frontend/src/app/(main)/projects/[id]/page.tsx:75-85`) and
  `deleteEnvVar` (same file, `handleDeleteEnvVar:344-347`) are genuinely
  uncaught.

This doesn't invalidate Finding 1's underlying bug — in every case the
`throw` happens *before* the follow-up line (`fetchProject()`, `onUpdate()`,
`fetch()`, `resetForm()`, `setOriginalContent()`), so that follow-up code
is skipped either way; caught call sites just fail quietly via
`console.error` instead of an unhandled rejection. But the write-up's
specific wording overstates the failure mode for roughly 7 of the ~12
affected sites, and could mislead whoever picks up `BUG-019` into
thinking the fix is "add try/catch" rather than "guard the shared `api()`
helper against empty bodies" (which is the fix that actually helps every
site uniformly, caught or not). Worth a one-line correction to Finding 1's
prose (or a note added directly to `BUG-019`) distinguishing "uncaught,
crashes the async function" (2 sites) from "caught-and-logged, but the
post-call refresh still silently never runs" (the rest).

**Coverage-count claim — inaccurate.** Per instruction to independently
recount:
- Backend: `grep -c 'r\.\(Get\|Post\|Put\|Delete\)(' router.go` = 48
  total registered handlers, 47 of them under `/api` (the 48th being
  `/health`). The task's Summary states "44 backend routes under /api
  (plus /health...)" and the frontmatter history says "44 routes/47
  frontend fns audited" — both undercount the actual `/api` route total
  by 3. The endpoint table itself is *not* missing anything — it has
  exactly 48 rows (47 `/api` + 1 `/health`), matching `router.go` route-
  for-route — so the audit's actual analysis is complete; only the
  self-reported summary arithmetic is wrong (44 in-use + 3 unused
  egress-whitelist = 47, not 44).
- Frontend: `grep -c '^export (async function|function|const)'
  frontend/src/lib/api.ts` = 45, not 47. Additionally, `getToken`
  (referenced in the Acceptance Criteria Coverage note as one of the
  "internal helpers" among the 47) is not actually exported at all —
  it's a private top-level `function getToken()` at `api.ts:8`, with no
  `export` keyword. Counting an unexported helper as part of "every
  function exported from api.ts" contradicts the Scope section's own
  definition of what's being audited.

Net effect: the underlying table/analysis is complete and I found no
missing route or missing frontend function during my spot check, but the
audit's own reported totals (44 backend / 47 frontend) don't reconcile
with what's actually in the two files (47 backend / 45 frontend). For a
task whose entire deliverable is an accurate accounting, this should be
corrected — recount and fix the numbers in the Summary, the frontmatter
history note, and the Acceptance Criteria Coverage section, and drop
`getToken` from the "exported" tally (or explicitly note it's unexported
and being counted for completeness only).

**Finding 2 — confirmed.** No `whitelist`-related identifiers anywhere
under `frontend/src/`; `router.go:89-91` registers the three routes
exactly as described.

**Finding 3 — confirmed.** `frontend/src/app/(main)/settings/page.tsx`'s
`handleSave` (~L371-378) always sends `{name, image, type: "docker"}`
for both create and update, matching the claim that nothing is live-
broken today. `domain.AgentProvider.Env` (`agent_provider.go:16`) is
read by `agent_service.go:285` but absent from the frontend `AgentProvider`
type (`api.ts:37-44`) and never sent by `createAgentProvider`/
`updateAgentProvider` — the "non-finding" is accurate too.

**Acceptance Criteria walk:**
- [x] Every exported function in `api.ts` checked against its backend
      handler — true in substance (verified via spot check), though the
      "47" count backing this claim is off (see above).
- [x] Every route in `router.go` confirmed to have a caller or flagged
      unused — true; table has all 48 rows, 3 correctly flagged unused
      (Finding 2), 1 correctly flagged intentionally backend-only
      (`/health`).
- [ ] Each mismatch filed as its own `BUG-XXX` — correctly left undone
      per this sprint's documented convention (architect files); already
      done for Findings 1 and 2 as `BUG-019`/`FEAT-013`.
- [x] Findings table preserved in Implementation Notes — present and
      complete.

**Summary:** the technical substance of this audit is sound — every
endpoint-table row I spot-checked was accurate, Finding 1's core `api()`
defect is real and correctly diagnosed, Finding 2 and Finding 3 both hold
up under independent verification, and no code was touched anywhere in
`frontend/` or `backend/`. Requesting changes only for two accuracy
issues within the audit document itself: (1) the reported route/function
totals don't reconcile with the actual files (47 backend / 45 frontend,
not 44 / 47), and (2) Finding 1's "not wrapped in try/catch" claim
overstates the failure mode for most of the ~12 affected call sites (most
are try/caught-and-logged, not uncaught). Both are text-only fixes to the
task's Implementation Notes — no re-triage of `BUG-019`/`FEAT-013` needed.

## Test Notes
<Filled in by the tester.>

### Review - 2026-07-07 (reviewer, second pass)

Verdict: PASS

This was a documentation-only rework fixing the two accuracy issues from
the first pass. Re-verified everything independently rather than trusting
the developer's corrected numbers.

**Count reconciliation — confirmed accurate.**
- `grep -oE 'r\.(Get|Post|Put|Delete)\(' backend/internal/router/router.go`
  = 48 total registered routes; 47 are under `/api`, the 48th is `/health`
  (`router.go:38`). Matches the corrected claim of "47 `/api` routes (plus
  `/health`)".
- `grep -nE '^export (async function|function|const)' frontend/src/lib/api.ts`
  = 45 matches. Matches the corrected claim of "45 exported functions/
  consts".
- `getToken` at `api.ts:8` is `function getToken(): string | null` — no
  `export` keyword. Correctly excluded from the tally now, and the
  Acceptance Criteria Coverage section's wording no longer implies it's
  part of the 45.
- Reconciles with the endpoint table: 48 rows total (47 `/api` + 1
  `/health`), 44 in-use + 3 unused (egress-whitelist) = 47. No drift
  between the summary prose, the frontmatter history, and the table
  itself anymore.

**Finding 1's corrected per-site breakdown — verified against actual code,
not just trusted:**
- `frontend/src/app/(main)/projects/[id]/page.tsx:75-79` (`handleRestart`)
  and `:81-85` (`handleDelete`) — confirmed genuinely uncaught, no
  `try/catch` anywhere in either function.
- `frontend/src/app/(main)/projects/[id]/page.tsx:345-348`
  (`handleDeleteEnvVar`) — confirmed genuinely uncaught. This is the third
  of the "3 uncaught" sites and matches the claim.
- `frontend/src/app/(main)/containers/page.tsx:68-86` — confirmed
  `startContainer`/`stopContainer`/`restartContainer` (one `try/catch` in
  `handleAction`) and `removeContainer` (a separate `try/catch` in
  `handleDelete`) are all caught with `console.error`.
- `frontend/src/app/(main)/containers/[id]/page.tsx:257-267` — confirmed
  `updateContainerResources` is wrapped in `try/catch` with
  `setError(...)` in the catch block, exactly as the corrected write-up
  describes. This is the site the developer says the first-pass review
  missed — confirmed it was indeed omitted from my first-pass list, and
  the developer's addition here is correct.
- `frontend/src/app/(main)/settings/page.tsx:254-261` (`deleteApiKey`),
  `:386-393` (`deleteAgentProvider`), `:573-581` (`deleteGitCredential`) —
  all three confirmed wrapped in `try/catch` with `console.error`.
- `frontend/src/app/(main)/code/[id]/page.tsx:68-77` (`writeFile`) —
  confirmed wrapped in `try/catch` with `console.error`.
- Total: 3 genuinely uncaught + 9 caught-but-still-broken (4 in
  `containers/page.tsx` + 1 in `containers/[id]/page.tsx` + 3 in
  `settings/page.tsx` + 1 in `code/[id]/page.tsx`) = 12, matching the "~12
  endpoints" figure used throughout Finding 1 and the Summary. The
  breakdown is accurate and each site was checked directly against the
  current file contents, not assumed from the write-up.

**No code touched — confirmed.** `git diff` against `frontend/src/lib/api.ts`
and the entire `backend/` tree is empty. `git diff --stat` scoped to
`projects/`, `containers/`, `settings/`, `code/` under
`frontend/src/app/(main)/` shows only `code/page.tsx` (the codebase list
page, unrelated to this task — `code/[id]/page.tsx` is the file this task
discusses, and it's untouched) with a 1-line change, which is pre-existing
ambient WIP consistent with everything else in `git status` (AGENTS.md,
Caddyfile, various UI component edits, `qa-debug*.js`, etc.) predating this
task. Nothing in that diff is mentioned in or plausible as this task's
Implementation Notes. `tasks/review/` and `tasks/active/` are untracked
directories (not yet committed anywhere in this pipeline run), which is
expected/normal for the in-flight SDLC state, not a sign of code changes.

**BUG-019 sanity check — no contradiction.** `tasks/active/BUG-019-frontend-api-empty-body-json-parse.md`'s
"Steps to Reproduce" section lists the identical 3-uncaught/9-caught
breakdown with matching file paths and function names (`restartProject`/
`deleteProject`/`deleteEnvVar` in `projects/[id]/page.tsx` as the 3
uncaught; the same 9 call sites across `containers/page.tsx`,
`containers/[id]/page.tsx`, `settings/page.tsx`, `code/[id]/page.tsx` as
caught-but-still-broken). Consistent with TEST-005's corrected Finding 1,
no re-triage needed.

**Acceptance Criteria walk (unchanged from first pass, still holds):**
- [x] Every exported function in `api.ts` checked against its backend
      handler — now backed by an accurate count (45, `getToken` excluded).
- [x] Every route in `router.go` confirmed to have a caller or flagged
      unused — accurate count (47 `/api` + 1 `/health` = 48 rows).
- [ ] Each mismatch filed as its own `BUG-XXX` — correctly left undone per
      sprint convention; already done for Findings 1/2 as `BUG-019`/`FEAT-013`.
- [x] Findings table preserved in Implementation Notes — present, unchanged
      from first pass, still accurate.

**Summary:** both accuracy issues from the first-pass review are now fully
corrected and independently re-verified against the actual `router.go`,
`api.ts`, and every frontend call site named in Finding 1 — no remaining
discrepancies found. No code was changed anywhere, consistent with this
being a documentation-only rework. No blocking issues. Non-blocking/minor:
none noted beyond what's already on record.

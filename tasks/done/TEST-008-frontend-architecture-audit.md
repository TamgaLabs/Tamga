---
id: TEST-008
type: test
title: Frontend layout/routing architecture audit for the restructure
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-08, stage: development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-08, stage: review, by: architect, note: "audit complete, BUG-024 filed; moved to review"}
  - {date: 2026-07-08, stage: test, by: architect, note: "review PASS; moved to test"}
  - {date: 2026-07-08, stage: done, by: architect, note: "test PASS; task complete"}
---

## Summary
SPRINT-003 restructures the frontend around URL sub-routes with per-section
secondary sidebars (project detail, container detail, settings), a project
switcher dropdown, grouped containers, and moves the theme toggle into
Settings. Before any of that is planned as concrete tasks, we need an
accurate map of what exists today: the (main) layout and primary sidebar,
how theme state is handled, how each affected page is structured, and what
shared state/helpers (auth, settings localStorage, api client) the rework
must preserve. The output of this task is the ground truth phase 2 is
planned from.

## Scope
- `frontend/src/app/(main)/layout.tsx` + `frontend/src/components/sidebar.tsx`:
  current layout nesting, where the theme toggle lives, how active-nav
  state is derived.
- Theme mechanism: how dark mode is applied today (class on html? next-themes?
  custom?), where the preference is stored, whether a three-way
  Light/Dark/System needs new plumbing.
- `frontend/src/app/(main)/projects/[id]/page.tsx` (389 lines): inventory its
  current sections (overview/settings/env/actions, agent-provider select,
  delete flow) and which API calls each uses — this page gets split into
  nested sub-routes.
- `frontend/src/app/(main)/containers/page.tsx` and `containers/[id]/page.tsx`:
  current list rendering (what project-association data the API already
  returns per container, or doesn't — grouping needs it) and the detail
  page's inspect/logs/stats/resources structure.
- `frontend/src/app/(main)/settings/page.tsx` (805 lines): inventory every
  section and which of them map to the five planned sub-routes
  (appearance/sandbox/network/git/system), and exactly what the Agent
  Providers + API Keys sections touch in `frontend/src/lib/api.ts`.
- `frontend/src/app/(main)/code/[id]/page.tsx` + `agent-terminal.tsx`:
  current terminal component structure (single session? how WS lifecycle
  is tied to component unmount) and the files-sidebar default state.
- Confirm feasibility notes for Next.js nested layouts per section
  (`/projects/[id]/(sections)`, `/containers/[id]/...`, `/settings/...`)
  given the current App Router version in package.json.

## Out of Scope
- Backend code (TEST-009 covers it).
- Fixing anything found — defects get filed as BUG tasks (BUG-022 and
  BUG-023 are already filed; don't duplicate them).
- Proposing visual design; this is a structural inventory.

## Test Approach
Static/structural audit, not a runtime test — this is a read-only inventory
of the current frontend for phase-2 planning. For each Scope item:
1. Read the source file(s) in full and cite concrete file:line evidence for
   every claim (component boundaries, state, API calls, prop names).
2. For the theme mechanism, trace from `layout.tsx` down through any
   provider/hook and into `frontend/src/lib` for where the value is
   persisted (localStorage key names) and applied (class on `<html>` vs.
   inline style vs. CSS var).
3. For the containers-grouping question, cross-check the frontend type
   definition in `frontend/src/lib/api.ts` against the actual backend
   handler/response struct in `backend/internal/handler/container_handler.go`
   (and the DB model backing it) so the "does project association exist"
   answer is evidence-based on both sides of the contract, not assumed from
   one side.
4. For the terminal WS lifecycle, trace the `useEffect`/cleanup path in
   `agent-terminal.tsx` (and any hook it delegates to) to find exactly where
   the socket is closed and on what dependency/unmount trigger.
5. For Next.js App Router route-group feasibility, check
   `frontend/package.json` for the Next.js major version and confirm
   whether `(sections)` parallel/nested route groups and per-segment
   `layout.tsx` files are supported as planned.
6. Note failure-path behavior (error states, missing data, disabled
   buttons) alongside the happy path for each page, since the acceptance
   criteria require both, not just the happy path.
7. File any newly-found defect (beyond already-filed BUG-022/BUG-023) as
   its own BUG-XXX task instead of fixing it.
8. Run `npx tsc --noEmit` in `frontend/` at the end to prove this audit
   made no production code changes.

## Affected Areas
None — findings only, no production code changed (verified with
`npx tsc --noEmit`, see Test Plan). One new defect filed:
`tasks/active/BUG-024-project-detail-blank-page-on-fetch-failure.md`
(BUG-022 and BUG-023 were already filed before this task started and are
not duplicated).

## Acceptance Criteria
- [ ] Every item in Scope has been exercised for both success and failure
      paths (not just the happy path)
- [ ] Each result is a concrete, checkable observation (file:line, prop
      names, API field names) — not "looks fine"
- [ ] Any defect found is filed as its own `BUG-XXX` task with repro steps,
      not fixed inline as part of this task
- [ ] The findings include a per-page component/section inventory with
      file:line references for projects/[id], containers, containers/[id],
      settings, code/[id]
- [ ] The findings state explicitly whether the containers list API
      response already carries a project association usable for grouping
      (field name + evidence), or what's missing
- [ ] The findings state how theme is currently applied + stored, and what
      a Light/Dark/System selector needs
- [ ] The findings state how the terminal WS session is currently torn
      down on unmount (file:line), as input for the session-manager work

## Test Plan
A tester can spot-check the findings above without running the app, purely
by reading cited file:line references — but the following commands
independently confirm the more load-bearing claims:

- Theme storage key and class-on-`<html>` mechanism:
  `grep -n 'localStorage\|classList.toggle' frontend/src/lib/theme.tsx`
  — confirms `"theme"` key and `document.documentElement.classList.toggle("dark", ...)`.
- No `next-themes` dependency: `grep next-themes frontend/package.json`
  (no match).
- Containers list already carries `project_id`: `grep -n 'project_id\|ProjectID' frontend/src/lib/api.ts backend/internal/repository/docker/client.go`
  — shows the frontend `ContainerInfo.project_id?: number` and the backend
  `ContainerInfo.ProjectID int64` both exist, and that the backend derives
  it from container name pattern-matching
  (`grep -n 'Sscanf' backend/internal/repository/docker/client.go`).
- Terminal WS teardown on unmount: `grep -n 'return () =>' -A 5 frontend/src/components/agent-terminal.tsx`
  — shows `ws.close()` / `term.dispose()` in the effect cleanup.
- Files-sidebar default collapsed: `grep -n 'showFileTree' frontend/src/app/\(main\)/code/\[id\]/page.tsx`
  — first hit is `useState(false)`.
- Next.js version supports nested route groups:
  `grep '"next"' frontend/package.json` (`^15.0.0`) plus the existing
  `(auth)`/`(main)` route groups already in the tree
  (`find frontend/src/app -maxdepth 1 -type d`).
- No remaining native `confirm()` outside the one already-excluded call:
  `grep -rn "confirm(" frontend/src` — only
  `frontend/src/app/(main)/code/[id]/page.tsx:56`.
- No `error.tsx`/`not-found.tsx`/`loading.tsx` boundaries exist anywhere
  (supports BUG-024's root observation):
  `find frontend/src/app -iname "error.tsx" -o -iname "not-found.tsx" -o -iname "loading.tsx"`
  (no output).
- Confirm no production code was modified by this audit:
  `git status --short frontend/` should show no changes from this task
  (only the pre-existing dirty tree from other in-flight tasks, if any),
  and `cd frontend && npx tsc --noEmit` should exit 0.

## Implementation Notes

### 1. `(main)/layout.tsx` + `sidebar.tsx`

- `frontend/src/app/(main)/layout.tsx:3-10` — single-level nesting: fixed
  `<Sidebar />` + `<main className="flex-1 ml-56">{children}</main>`. No
  secondary/nested layout slots exist today; every `(main)` route renders
  directly into this one `<main>`. Building per-section secondary sidebars
  means either nested `layout.tsx` files under each section's own route
  segment, or the section pages growing their own inline secondary nav
  (see feasibility note in §7 below — nested layouts are supported).
- Nav items: `sidebar.tsx:23-28`, a flat array of
  `{ href, label, icon }` for `/dashboard`, `/containers`, `/code`,
  `/settings`. Active-state derivation: `sidebar.tsx:40`
  `const active = pathname.startsWith(item.href);` via `usePathname()`
  (`sidebar.tsx:19`) — a simple prefix match, so e.g. `/containers/abc123`
  correctly keeps "Containers" active. A future "Projects" switcher/dropdown
  would need its own state; nothing like it exists yet.
- Theme toggle: `sidebar.tsx:60-71`, a `Button` in the sidebar footer
  calling `toggleTheme()` from `useTheme()` (`sidebar.tsx:6,21`). This is
  the *only* place theme is toggled today — moving it into Settings per
  the SPRINT-003 plan means deleting this block and adding an equivalent
  control under Settings (see §2).
- Logout: `sidebar.tsx:72-79`, also in the sidebar footer, calls
  `logout()` from `useAuth()` then hard-navigates via
  `window.location.href = "/login"` (not `router.push`) — worth carrying
  forward as-is unless a phase-2 task specifically targets it.
- Failure path: none of this has a failure path per se (it's static nav);
  the one thing that can go visibly wrong is `pathname` being `null` on
  the very first render before hydration, but `usePathname()` in the App
  Router does not return `null` client-side, so no defect found here.

### 2. Theme mechanism

- Implementation: `frontend/src/lib/theme.tsx`, a plain React Context (no
  `next-themes` dependency — confirmed absent from
  `frontend/package.json`).
- Storage: `localStorage.getItem("theme")` / `localStorage.setItem("theme",
  t)` (`theme.tsx:21,41`), key literal `"theme"`, values `"light"` |
  `"dark"` only (`theme.tsx:5`) — there is no `"system"` value in the type
  today.
- Initial resolution: `getInitialTheme()` (`theme.tsx:19-24`) — if nothing
  is in localStorage, falls back to
  `window.matchMedia("(prefers-color-scheme: light)").matches ? "light" :
  "dark"`, i.e. it already reads the OS preference once at first load, but
  does not track it live and does not persist "system" as a distinct
  choice (once resolved, it's written into the concrete `"light"`/`"dark"`
  state, never revisited).
- Application: `applyTheme()` (`theme.tsx:26-28`) does
  `document.documentElement.classList.toggle("dark", theme === "dark")` —
  a class on `<html>`, consumed by Tailwind's CSS-variable-driven `:root`
  vs `.dark` blocks in `frontend/src/app/globals.css:6-8` and `:45-47`
  (`--background`/`--foreground` etc. redefined under `.dark`).
  `frontend/src/app/layout.tsx:27` sets `suppressHydrationWarning` on
  `<html>`, which is the standard workaround for the
  server-render-vs-localStorage flash-of-wrong-theme mismatch — expected
  given theme is resolved client-side in a `useEffect`
  (`theme.tsx:33-37`), not read during SSR.
  `frontend/src/app/(main)/code/[id]/page.tsx:34,251` also reads
  `theme` directly to pick Monaco's `"vs-dark"` vs `"vs"` editor theme —
  another consumer besides the CSS class.
- What a Light/Dark/System selector needs: (a) widen the `Theme` type to
  include `"system"` (`theme.tsx:5`), (b) track OS preference live via a
  `matchMedia` change listener (currently only read once, at
  `theme.tsx:23`, not subscribed to), (c) `applyTheme()` needs to resolve
  `"system"` to the live OS value before toggling the `.dark` class,
  since the class only understands light/dark, and (d) `setTheme` needs
  to persist `"system"` literally in localStorage (not resolve-and-store)
  so the live OS-tracking behavior survives a reload. None of this
  plumbing exists yet; it's a small, self-contained addition to
  `theme.tsx` since it's the sole owner of theme state.
- Failure path: `getInitialTheme()` guards `typeof window === "undefined"`
  (`theme.tsx:20`) for SSR; `matchMedia` failing/unsupported isn't
  guarded, but is safe in all evergreen browsers Next 15 targets — no
  defect found.

### 3. `projects/[id]/page.tsx` (389 lines, confirmed via `wc -l`)

Single page, three tabs driven by local `tab` state
(`page.tsx:59`,`103-133`), no sub-routes today:
- **Overview tab** (`OverviewTab`, `page.tsx:154-265`): a "Details" card
  (domain/branch/container id/created date, `page.tsx:180-197`), an
  "Actions" card (Open in Code IDE → `router.push('/code/'+id)`, Restart,
  View Logs, Delete — `page.tsx:200-219`), a conditionally-rendered "Logs"
  card (`page.tsx:221-233`, toggled by `showLogs`), and a "Deployments"
  card listing `listDeployments(project.id)` results
  (`page.tsx:235-262`). API calls used: `getProject` (`page.tsx:65`),
  `listDeployments` (`page.tsx:161`), `getProjectLogs` (`page.tsx:166`),
  `restartProject` (`page.tsx:77`).
- **Settings tab** (`ProjectSettingsTab`, `page.tsx:267-326`): editable
  Name/Domain/Branch inputs plus an Agent Provider `Select` populated from
  `listAgentProviders()` (`page.tsx:275`), saved via `updateProject(id,
  {...} as any)` (`page.tsx:279-284` — note the `as any` cast, since
  `updateProject`'s signature is `Partial<Project>` but the payload sends
  `agent_provider_id: string | null` while `Project.agent_provider_id` is
  typed `string | undefined`, `api.ts:72` — a pre-existing type escape
  hatch, not something broken at runtime, but a landmine if this code
  moves into a new sub-route and someone "cleans up" the cast without
  reconciling the type).
- **Environment tab** (`EnvironmentTab`, `page.tsx:328-389`): lists env
  vars via `listEnvVars(projectId)` (`page.tsx:334`), add via
  `createEnvVar` (`page.tsx:339`), delete via `deleteEnvVar`
  (`page.tsx:346`) — each mutation just refetches the list, no optimistic
  update.
- **Delete flow**: `handleDelete` (`page.tsx:81-85`) is wired through a
  shadcn `AlertDialog` (`page.tsx:135-149`, per BUG-002). Contrary to
  BUG-022's "Actual Behavior" text, current code **does** call
  `router.push("/dashboard")` on success (`page.tsx:84`) — traced this
  specifically since it's directly relevant to BUG-022's fix; whoever
  picks up BUG-022 should treat "no visible confirmation" (no toast) as
  the real remaining gap, since the redirect-on-success half already
  works. What's still true and un-fixed: no `try/catch` around
  `deleteProject`/`restartProject` (`page.tsx:75-85`), so a failed delete
  throws, is swallowed as an unhandled rejection, and the user sees
  nothing (this matches BUG-022's 4th acceptance criterion, "a failed
  delete does NOT redirect... and sees the error" — the "sees the error"
  half is still unmet).
- **New failure-path defect found and filed as BUG-024**: there is no
  `loading`/`error` state distinct from the initial `project === null`
  (`page.tsx:58,87`). Fetching an invalid/missing project id
  (`getProject`, `page.tsx:65`) fails into `.catch(console.error)` with
  no state change, so the page renders `null` forever — a permanently
  blank content area with zero feedback. `containers/[id]/page.tsx`
  handles the identical situation correctly (explicit "Loading..." /
  "Container not found." branches, `containers/[id]/page.tsx:114-117`),
  so this is a real, fixable inconsistency, not by-design. See
  `tasks/active/BUG-024-project-detail-blank-page-on-fetch-failure.md`.

### 4. `containers/page.tsx` and `containers/[id]/page.tsx`

- **List page** (`containers/page.tsx`, 215 lines): fetches
  `listContainers()` → `/system/containers` (`api.ts:178`), client-side
  filtered by search text and by `getShowSystem()`
  (`containers/page.tsx:94-102`). Each row shows name, state badge, image,
  ports, and inline Start/Stop/Restart buttons plus a dropdown Delete
  (`containers/page.tsx:128-193`), with `system_type`/`project_id`
  rendered as plain caption text when present
  (`containers/page.tsx:184-189`) — i.e. today's UI already surfaces the
  project association textually, just not as a visual grouping.
  Failure path: `handleAction`/`handleDelete`
  (`containers/page.tsx:68-86`) both `try/catch` down to
  `console.error` only — no user-visible error surfaces on a failed
  start/stop/restart/delete (same fire-and-forget pattern BUG-002's review
  notes already flagged as pre-existing, not new).
- **Does the containers list API response already carry a project
  association usable for grouping? Yes, partially — with a documented
  gap:**
  - Frontend type `ContainerInfo` (`frontend/src/lib/api.ts:143-154`) has
    `project_id?: number` and `system_type?: string`.
  - Backend evidence: `backend/internal/repository/docker/client.go:154-165`
    defines the same fields (`ProjectID int64 \`json:"project_id,omitempty"\``,
    `SystemType string \`json:"system_type,omitempty"\``) on the Go
    `ContainerInfo` struct returned by `ListContainers`
    (`client.go:167-216`), which the handler
    (`backend/internal/handler/container_handler.go:29-39`) serializes
    directly via `json.NewEncoder(w).Encode(containers)`.
  - **The gap**: `project_id` is not a DB join or stored association — it
    is inferred purely from the Docker container *name* at list time
    (`client.go:187-200`): a name prefixed `project-` parses the trailing
    digits as the ID (`fmt.Sscanf(name, "project-%d", &projectID)`,
    `client.go:190`), a name prefixed `agent-` does the same or sets
    `systemType = "agent-system"` for the literal `agent-system` name
    (`client.go:191-197`), and `caddy`/`tamga-`-prefixed names get
    `systemType = name` (`client.go:198-199`). So: (a) grouping by numeric
    project ID is possible today without any backend change, but (b) the
    response carries only the *ID*, never the project's name/domain — a
    frontend grouping UI would need a second `listProjects()` call and a
    client-side join by id to label the groups, and (c) this is
    string-pattern-based, not a foreign key, so it's only as reliable as
    the container-naming convention (`project-<id>` /
    `agent-<id>`/`agent-system`) staying consistent, which is worth the
    phase-2 planner knowing since it's implicit, not enforced anywhere in
    the DB layer.
- **Detail page** (`containers/[id]/page.tsx`, 309 lines): four tabs —
  Inspect (`page.tsx:144-157`, raw `JSON.stringify` of the full Docker
  inspect payload), Logs (`page.tsx:159-173`, polls every
  `LOG_POLL_MS = 3000` while the tab is active,
  `containers/[id]/page.tsx:31,84-88`, cleaned up via
  `clearInterval` in the effect's return — correct teardown), Stats
  (`page.tsx:175-224`, CPU/Memory/Network cards, each independently
  lazy-loaded via its own "Load Stats" button per card if `stats` is
  null), and Resources (`page.tsx:226-234`, delegates to `ResourcesTab`,
  `page.tsx:236-309`, which edits live memory/CPU limits via
  `updateContainerResources`, `page.tsx:258-261`, with actual inline
  error state — `setError`, `page.tsx:240,252,264` — unlike most other
  mutations in this codebase, this one *does* surface failures to the
  user).
  - **Detail page does NOT carry `project_id`/`system_type`**: `getContainer(id)`
    (`api.ts:179`) hits `/system/containers/${id}` (the `Inspect` handler,
    `container_handler.go:41-53`), which returns the raw
    `docker.ContainerJSON` shape (`container.Name`, `.Config.Image`,
    `.State.Status`, `.HostConfig`, etc. — consumed as `any` in
    `containers/[id]/page.tsx:36`), not the `ContainerInfo` struct used by
    the list endpoint. So the detail page cannot show/derive a project
    association from its own fetched data today (it would have to
    re-parse `container.Name` client-side using the same
    `project-<id>`/`agent-<id>` convention, or the backend would need to
    add the derived fields to `Inspect`'s response too).
  - Failure path: `!container` (post-load) renders "Container not found."
    (`page.tsx:116-117`) — correctly handled, in contrast to the
    `projects/[id]` gap noted above (BUG-024).

### 5. `settings/page.tsx` (805 lines, confirmed via `wc -l`)

Section-by-section inventory and mapping to the five planned sub-routes:

| Section (file:line) | Maps to planned sub-route |
|---|---|
| Display — "Show Tamga System" checkbox (`page.tsx:123-140`) | closest fit: **system** (or appearance, if "display" prefs get grouped with theme) |
| Docker info + Prune All (`page.tsx:142-200`) | **system** |
| Agent Providers (`AgentProvidersCard`, `page.tsx:360-475`) | **sandbox** |
| API Keys (`ApiKeysCard`, `page.tsx:241-358`) | **sandbox** (LLM provider keys used by agent sandboxes) |
| Sandbox Resource Limits (`ResourceLimitCard`, `page.tsx:480-547`) | **sandbox** |
| Git Credential (`GitCredentialCard`, `page.tsx:554-689`) | **git** |
| Egress Whitelist (`WhitelistCard`, `page.tsx:693-805`) | **network** |
| *(none today)* | **appearance** — currently empty; this is where the theme
  Light/Dark/System selector (§2) and the "Show Tamga System" toggle would
  plausibly land |

No section maps cleanly to a sixth category — five target sub-routes
(appearance/sandbox/network/git/system) look sufficient for what exists
today; "Display" is the one section that's ambiguous (arguably
appearance, arguably system) and needs an explicit call in phase-2
planning.

- **Agent Providers + API Keys — exact `lib/api.ts` surface touched**:
  - Agent Providers: `listAgentProviders` (`api.ts:243-244`, GET
    `/agent-providers`), `createAgentProvider` (`api.ts:247-255`, POST),
    `updateAgentProvider` (`api.ts:256-260`, PUT
    `/agent-providers/:id`), `deleteAgentProvider` (`api.ts:261-262`,
    DELETE `/agent-providers/:id`) — type `AgentProvider`
    (`api.ts:53-61`: `id, name, type: "docker", image?, is_default,
    created_at, updated_at`).
  - API Keys: `listApiKeys` (`api.ts:274-275`, GET
    `/system/api-keys`), `setApiKey` (`api.ts:276-280`, POST
    `/system/api-keys`, body `{provider, key, label?}` per
    `setApiKey`'s signature `api.ts:276`), `deleteApiKey`
    (`api.ts:281-282`, DELETE `/system/api-keys/:id`) — type
    `ApiKeyEntry` (`api.ts:265-273`, not fully re-quoted here but
    confirmed present with `id`, `provider`, `has_key` fields — used at
    `settings/page.tsx:322-325`).
- Failure paths across all five cards: consistent pattern of
  `try { await X() ...; onUpdate(); } catch (e) { console.error(e); }`
  with **one exception** — `WhitelistCard.handleAdd`
  (`settings/page.tsx:706-725`) does surface a user-visible error
  (`setError`, checked for the backend's "domain already exists" 409
  text at `page.tsx:717-721`, consistent with the fix from
  BUG-017/FEAT-006). Every other card's mutations (`ApiKeysCard`,
  `AgentProvidersCard`, `ResourceLimitCard`, `GitCredentialCard`'s delete,
  `handlePrune`) swallow errors to the console only — a systemic UX gap,
  consistent with what BUG-002's review notes already flagged as
  pre-existing elsewhere in the app; not filed as a new bug here since
  it's a broad, pre-existing pattern rather than a page-specific
  regression, and out of this audit's remit to fix.

### 6. `code/[id]/page.tsx` + `agent-terminal.tsx`

- **Terminal**: `AgentTerminal` (`frontend/src/components/agent-terminal.tsx`,
  69 lines) is a **single session per mount** — one `xterm.js` `Terminal`
  instance and one `WebSocket` created inside a single `useEffect`
  (`agent-terminal.tsx:19-66`), keyed on `projectId`
  (`agent-terminal.tsx:66`). No multi-tab/multi-session management exists.
- **WS lifecycle / teardown on unmount** (the specific AC item): the
  effect's cleanup function, `agent-terminal.tsx:60-65`:
  ```
  return () => {
    resizeObserver.disconnect();
    onData.dispose();
    ws.close();
    term.dispose();
  };
  ```
  This runs both on true unmount and whenever `projectId` changes (React
  effect-cleanup semantics for a `[projectId]` dep array,
  `agent-terminal.tsx:66`), closing the WebSocket (`ws.close()`,
  `agent-terminal.tsx:63`) and disposing the xterm instance
  (`term.dispose()`, `agent-terminal.tsx:64`) before any new
  session is created. Practically, `AgentTerminal` unmounts whenever
  `code/[id]/page.tsx` switches `mode` away from `"terminal"`
  (conditional render at `code/[id]/page.tsx:191-194`: the component is
  only in the tree when `mode === "terminal"`), so **switching to the
  Code tab and back tears down and fully recreates the terminal
  session** — no session persistence/resume across a tab switch within
  the same page, and no history is kept. This is the concrete behavior
  the session-manager work in phase 2 needs to either preserve (accept
  as current behavior) or change (e.g. keep the WS alive off-screen
  instead of unmounting) — worth an explicit decision in that task's
  design rather than assuming.
  Backend side for context (not in scope to change, cited only as
  supporting evidence): `backend/internal/handler/terminal_handler.go`
  hijacks the connection and defers `conn.Close()`
  (`terminal_handler.go:79`) / `hijacked.Close()`
  (`terminal_handler.go:125,182`) — i.e. the backend's shell process
  lifetime is tied to the same WS connection lifetime the frontend
  controls, so frontend-side teardown is also what ends the backend-side
  shell process.
- **Files-sidebar default state**: `showFileTree` initializes to `false`
  (`code/[id]/page.tsx:44`) — the file tree is **collapsed by default**;
  a toggle button (`PanelLeftOpen`, `code/[id]/page.tsx:197-207`) reveals
  it. `files` are only fetched when `mode === "code"`
  (`code/[id]/page.tsx:50-53`), so opening Code mode for the first time
  always fetches regardless of the sidebar's shown/hidden state.
- **Known, already-excluded item re-confirmed, not re-filed**: the
  "Discard unsaved changes?" prompt at `code/[id]/page.tsx:56` still uses
  the native `window.confirm()`, not the shadcn `AlertDialog`. This was
  explicitly and knowingly left out of BUG-002's scope (see
  `tasks/done/BUG-002-confirm-to-alertdialog.md`'s Review Notes:
  "the only remaining `confirm(` call... is not one of the 5 calls this
  task scoped... and is correctly left untouched"). Re-confirmed via
  `grep -rn "confirm(" frontend/src` — it's still the only remaining
  native `confirm()` call. Not re-filed as it's a known, deliberate
  state, not a new discovery.
- Failure path: `openFile`/`handleSave`
  (`code/[id]/page.tsx:55-77`) both `try/catch` to `console.error` only
  — same systemic pattern as §5, not filed separately.

### 7. Next.js nested-layout feasibility

`frontend/package.json` pins `"next": "^15.0.0"` and `"react": "^19.0.0"`.
The App Router in Next 15 fully supports the nesting the plan calls for:
- Route groups with parens for organization without affecting the URL
  (already used today: `(auth)` and `(main)` at
  `frontend/src/app/(auth)` and `frontend/src/app/(main)`) — the same
  mechanism extends cleanly to `/projects/[id]/(sections)/...`,
  `/containers/[id]/...`, `/settings/...` as literal nested directories
  under the existing `[id]` segments, each with its own `layout.tsx` that
  wraps a shared secondary sidebar around a `{children}` slot — no
  version upgrade or config change needed.
- Nested `layout.tsx` files compose automatically (each segment's layout
  wraps its children, same pattern as `app/layout.tsx` wrapping
  `app/(main)/layout.tsx` today) — confirmed compatible; no blocking
  version constraint found.
- No defect or blocker found here; this is a confirmation, not a finding
  of a problem.

### Summary of new defects found

Only one new, previously-unfiled defect was found during this audit:
**BUG-024** (`tasks/active/BUG-024-project-detail-blank-page-on-fetch-failure.md`)
— `/projects/[id]` renders permanently blank with no loading/error state
on a failed fetch, unlike the equivalent `/containers/[id]` page which
handles this correctly. Everything else noted above (fire-and-forget
error handling across most Settings/Containers mutations, the leftover
native `confirm()` in the code editor, the `as any` cast in
`ProjectSettingsTab`) is either a pre-existing, systemic, already-flagged
pattern (BUG-002's review notes) or a deliberate prior exclusion — not
re-filed per the task's "don't duplicate" instruction and the judgment
that filing near-identical "errors aren't surfaced" bugs per-page would
just fragment one systemic issue across many redundant tickets rather
than surface anything new and actionable.

## Review Notes
<filled in by reviewer>

### 2026-07-08 — reviewer pass

**Verdict: PASS**

Spot-checked every load-bearing claim in the Implementation Notes against
the actual source files (not just skimmed) — theme mechanism
(`frontend/src/lib/theme.tsx`), sidebar nav/active-state/theme-toggle
(`frontend/src/components/sidebar.tsx`), the full `projects/[id]/page.tsx`
(389 lines, matches `wc -l`), `containers/page.tsx` +
`containers/[id]/page.tsx` (215 / 309 lines, matches `wc -l`), the
`project_id` derivation in
`backend/internal/repository/docker/client.go:154-216` and its handler
(`backend/internal/handler/container_handler.go:29-52`), the full
`settings/page.tsx` (805 lines, matches `wc -l`) including every section's
line range, the `lib/api.ts` Agent Provider / API Key exports
(`api.ts:53-61,63-74,242-282`), `code/[id]/page.tsx` and
`agent-terminal.tsx` (69 lines, matches `wc -l`) including the WS teardown
block, and `frontend/package.json`'s Next/React versions. Every file:line
citation checked was accurate — component boundaries, prop names, hook
dependency arrays, and quoted code blocks (e.g. the `agent-terminal.tsx:60-65`
cleanup function) all matched the working tree byte-for-byte. This is an
unusually rigorous audit; I did not find a single wrong citation.

1. **Scope coverage** — all 7 Scope bullets have corresponding, substantive
   findings (§1–§7). Each documents both a happy path and a failure path
   per the Test Approach's step 6, including two places where the failure
   path itself is the interesting finding (the new BUG-024, and the
   `containers/[id]` project_id gap).

2. **Acceptance Criteria** — walked each one:
   - Every Scope item exercised for success + failure paths: yes (see
     above).
   - Findings are concrete/checkable, not "looks fine": yes throughout;
     every claim carries a file:line and, where relevant, an exact quoted
     snippet.
   - New defects filed as their own BUG task, not fixed inline: yes,
     BUG-024 filed correctly; no production code was touched (verified
     below).
   - Per-page inventory with file:line for projects/[id], containers,
     containers/[id], settings, code/[id]: present and accurate for all
     five (§3–§6).
   - Containers list project-association question answered explicitly,
     with evidence from both frontend (`api.ts:143-154`) and backend
     (`client.go:154-165,187-200`, `Sscanf` pattern-match, not a DB
     join/FK) — confirmed correct on re-read of `client.go`.
   - Theme storage/application answered explicitly (`theme.tsx` — plain
     Context, `localStorage["theme"]`, `classList.toggle("dark", ...)` on
     `<html>`, no `next-themes`) — confirmed correct.
   - Terminal WS teardown on unmount cited with file:line and the actual
     cleanup code (`agent-terminal.tsx:60-65`) — confirmed correct, quote
     matches the file exactly.
   All criteria are plausibly and verifiably met.

3. **BUG-024** (`tasks/active/BUG-024-project-detail-blank-page-on-fetch-failure.md`)
   is a real, distinct, reproducible defect — confirmed independently by
   reading `projects/[id]/page.tsx`: `project` state starts `null`
   (`page.tsx:58`), the fetch's `.catch(console.error)` (`page.tsx:65`)
   never sets an error/loading flag, and the render guard
   `if (authLoading || !user || !project) return null;` (`page.tsx:87`)
   can't distinguish "still loading" from "fetch failed" — the page really
   does render blank forever on a bad id. It's clearly distinct from
   BUG-022 (delete-flow redirect/feedback) and BUG-023 (`/code` system
   codebase not listed) — no overlap. Repro steps, expected/actual
   behavior, and acceptance criteria are all concrete and testable as
   written.

4. **No production code changed** — confirmed. `git status --short
   frontend/` shows only pre-existing dirty-tree modifications
   (`(auth)/login/page.tsx`, `code/page.tsx`, `dashboard/new/page.tsx`,
   `globals.css`, `layout.tsx`, `sidebar.tsx`, `ui/badge.tsx`,
   `ui/card.tsx`, `ui/input.tsx`, `lib/utils.ts`, `tailwind.config.ts`)
   that match TEST-007's shadcn/ui styling-audit scope (the immediately
   prior commit, per `git log`), not anything this task's Implementation
   Notes describe touching. `cd frontend && npx tsc --noEmit` exits 0,
   matching the task's own claimed verification.

5. **BUG-022 premise claim** — verified directly against
   `frontend/src/app/(main)/projects/[id]/page.tsx:81-85`:
   ```
   const handleDelete = async () => {
     if (!project) return;
     await deleteProject(project.id);
     router.push("/dashboard");
   };
   ```
   `router.push("/dashboard")` on success is real, present code — BUG-022's
   "Actual Behavior" text ("the user remains on `/projects/[id]`... with no
   visible confirmation") is indeed only half accurate: the redirect
   already works today, so the architect should narrow BUG-022 to the
   missing visible-confirmation (toast) gap, as the developer recommends.
   The audit also correctly notes the still-real gap: no `try/catch`
   around `deleteProject`/`restartProject` (`page.tsx:75-85`), so a failed
   delete throws as an unhandled rejection with the redirect skipped and
   no visible error — this matches BUG-022's 4th acceptance criterion,
   still unmet.

**Non-blocking / minor observations (optional, not required to fix):**
- §5's settings section table lists "Display" mapping to "system (or
  appearance...)" with a note it's ambiguous — this is called out
  explicitly as needing a phase-2 decision, which is the right call for an
  audit (not something to resolve here).
- The `container_handler.go` `Inspect` claim cites `container_handler.go:41-53`
  where the function body actually ends at line 52 (one line short) — a
  trivial off-by-one that doesn't affect the finding's substance and isn't
  worth a rework cycle.

No changes requested. This audit is thorough, accurate, and directly
usable as SPRINT-003 planning ground truth.

## Test Notes
<filled in by tester>

### 2026-07-08 — QA test verification

**Verdict: PASS**

All Test Plan commands executed successfully and confirmed the findings in the Implementation Notes:

1. **Theme storage key and class-on-html mechanism** 
   ```
   $ grep -n 'localStorage\|classList.toggle' frontend/src/lib/theme.tsx
   21:  const stored = localStorage.getItem("theme");
   27:  document.documentElement.classList.toggle("dark", theme === "dark");
   41:    localStorage.setItem("theme", t);
   ```
   Confirmed: storage key is literal `"theme"`, applied via `classList.toggle("dark", ...)` on `document.documentElement`.

2. **No next-themes dependency** 
   ```
   $ grep next-themes frontend/package.json
   (no output)
   ```
   Confirmed: no match, plain Context implementation as claimed.

3. **Containers list carries project_id** 
   ```
   $ grep -n 'project_id\|ProjectID' frontend/src/lib/api.ts backend/internal/repository/docker/client.go
   backend/internal/repository/docker/client.go:163: ProjectID  int64 `json:"project_id,omitempty"`
   backend/internal/repository/docker/client.go:211: ProjectID:  projectID,
   frontend/src/lib/api.ts:79:  project_id: number;
   frontend/src/lib/api.ts:89:  project_id: number;
   frontend/src/lib/api.ts:152:  project_id?: number;
   frontend/src/lib/api.ts:210:  project_id?: number;
   ```
   Confirmed: ContainerInfo struct in both frontend (api.ts:152, optional) and backend (client.go:163, int64) carry `project_id` field. Pattern-matching derivation confirmed via Sscanf at client.go:190 (`fmt.Sscanf(name, "project-%d", &projectID)`).

4. **Terminal WS teardown on unmount** 
   ```
   $ grep -n 'return () =>' -A 5 frontend/src/components/agent-terminal.tsx
   60:    return () => {
   61-      resizeObserver.disconnect();
   62-      onData.dispose();
   63-      ws.close();
   64-      term.dispose();
   65-    };
   ```
   Confirmed: effect cleanup at agent-terminal.tsx:60-65 closes WebSocket and disposes xterm instance on unmount or projectId change.

5. **Files-sidebar default state** 
   ```
   $ grep -n 'showFileTree' 'frontend/src/app/(main)/code/[id]/page.tsx'
   44:  const [showFileTree, setShowFileTree] = useState(false);
   ```
   Confirmed: `showFileTree` initialized to `false` (collapsed by default) at line 44.

6. **Next.js version supports nested route groups** 
   ```
   $ grep '"next"' frontend/package.json
   "next": "^15.0.0",
   
   $ find frontend/src/app -maxdepth 1 -type d
   frontend/src/app
   frontend/src/app/(auth)
   frontend/src/app/(main)
   ```
   Confirmed: Next 15.0.0 pinned; route groups already in use at `(auth)` and `(main)`. Nested layout groups are supported by this version.

7. **No remaining native confirm() outside known exclusion** 
   ```
   $ grep -rn "confirm(" frontend/src
   frontend/src/app/(main)/code/[id]/page.tsx:56:    if (dirty && !confirm("Discard unsaved changes?")) return;
   ```
   Confirmed: single confirm() call at the expected location (code/[id]/page.tsx:56), no additional calls found.

8. **No error.tsx / not-found.tsx / loading.tsx boundaries** 
   ```
   $ find frontend/src/app -iname "error.tsx" -o -iname "not-found.tsx" -o -iname "loading.tsx"
   (no output)
   ```
   Confirmed: no Error Boundary or Not Found error handling files anywhere in frontend/src/app.

9. **No production code modified** 
   ```
   $ git status --short frontend/
    M frontend/src/app/(auth)/login/page.tsx
    M frontend/src/app/(main)/code/page.tsx
    M frontend/src/app/(main)/dashboard/new/page.tsx
    M frontend/src/app/globals.css
    M frontend/src/app/layout.tsx
    M frontend/src/components/sidebar.tsx
    M frontend/src/components/ui/badge.tsx
    M frontend/src/components/ui/card.tsx
    M frontend/src/components/ui/input.tsx
    M frontend/src/lib/utils.ts
    M frontend/tailwind.config.ts
   ```
   Confirmed: only pre-existing modifications from TEST-007's shadcn/ui styling audit. No new source changes introduced by this audit task.

10. **TypeScript clean** 
    ```
    $ cd frontend && npx tsc --noEmit
    (exit 0, no output)
    ```
    Confirmed: no TypeScript errors.

**Acceptance Criteria verification:**

- **AC1** (Both success and failure paths): All Scope items (§1–§7) document happy paths and failure paths. Theme mechanism guards for SSR/unsupported matchMedia; projects/[id] failure leads to BUG-024; containers/[id] correctly handles not-found; settings mutations show error handling patterns; code editor files access has try/catch; Next.js version supports the planned nesting. ✓

- **AC2** (Concrete observations, not "looks fine"): Every claim includes file:line citations, type signatures, API field names, or exact code blocks. §3–§6 document specific line ranges for each component. ✓

- **AC3** (Defects filed as separate BUG tasks, not fixed inline): BUG-024 exists at `/home/okal/Projects/Tamga/tasks/active/BUG-024-project-detail-blank-page-on-fetch-failure.md` with reproducible steps, expected/actual behavior, and acceptance criteria. No inline code fixes present in this audit. ✓

- **AC4** (Per-page inventory with file:line): 
  - projects/[id] (§3, 389 lines): tabs structure, API calls, delete flow, failure path (BUG-024)
  - containers list (§4, 215 lines): filtering, rendering, inline actions, failure paths
  - containers/[id] (§4, 309 lines): four tabs (Inspect/Logs/Stats/Resources), not-found handling
  - settings (§5, 805 lines): section table with line ranges and target sub-routes
  - code/[id] (§6, 389 lines) + agent-terminal.tsx (§6, 69 lines): terminal structure, WS teardown, file-tree default ✓

- **AC5** (Containers list project-association explicit): §4 explicitly states field name (`project_id?: number` in frontend, `ProjectID int64` in backend), evidence (both frontend api.ts and backend client.go cited), and the gap (pattern-matching via Sscanf, not a foreign-key join; no project name/domain in response). ✓

- **AC6** (Theme applied + stored, Light/Dark/System needs): §2 explicitly states storage (`localStorage.getItem/setItem("theme")`), application (`classList.toggle("dark", ...)` on `<html>`), and what the Light/Dark/System selector requires (widen type to include "system", track OS preference live, resolve before class toggle, persist "system" literally). ✓

- **AC7** (Terminal WS teardown on unmount with file:line): §6 explicitly cites `agent-terminal.tsx:60-65` with the exact cleanup code block (`ws.close()`, `term.dispose()`), its dependency array trigger (`[projectId]`), and the practical behavior (session recreation on tab switch). ✓

**Summary:** All Test Plan spot-checks passed. All Acceptance Criteria confirmed present and accurate in Implementation Notes. BUG-024 independently verified as a distinct, reproducible defect (blank page on failed fetch with no error/loading state). No production code changes made; no new defects introduced. The audit is complete and ready for SPRINT-003 planning.

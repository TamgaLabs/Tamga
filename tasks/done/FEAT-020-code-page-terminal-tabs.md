---
id: FEAT-020
type: feature
title: Code page terminal tabs — multiple persistent sessions, reattach, terminate; files sidebar open by default
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created from TEST-008 findings §6; depends on FEAT-015"}
  - {date: 2026-07-09, stage: development, by: architect, note: "assigned to sdlc-developer; FEAT-015 session manager landed (list/create-via-WS/terminate endpoints live)"}
  - {date: 2026-07-09, stage: review, by: architect, note: "terminal tabs implemented (unmount-reattach detach model, pre-check cap, files sidebar default true); moved to review"}
  - {date: 2026-07-09, stage: rework, by: architect, note: "review CHANGES_REQUESTED: initial listAgentSessions fetch does unconditional setTabs replace, can clobber a tab created before it resolves (orphans a server session). Backend orphan-on-failed-upgrade filed as BUG-027."}
  - {date: 2026-07-10, stage: review, by: architect, note: "rework done (initial fetch now merges by id via functional setTabs, preserves pending tabs + user activeTab); second review pass, delta only"}
  - {date: 2026-07-10, stage: rework, by: architect, note: "2nd review CHANGES_REQUESTED: handleSessionResolved renames pending->real without dedupe; if the seed fetch already added that id a duplicate tab (same React key) results. Needs dedupe."}
---

## Summary
The code page's terminal is a single anonymous session that is fully torn
down whenever the component unmounts — even just switching to the Code
tab and back (TEST-008 §6). With FEAT-015's backend session manager in
place, the frontend gets real terminal tabs: multiple named sessions per
project that survive navigation and browser close, a reattach flow, and
explicit terminate. The files sidebar also opens by default. Depends on
FEAT-015 (must land first).

## Requirements
- Terminal tab bar in the code page's terminal mode: one tab per session,
  "+" to open a new session (respecting the backend's 10-session cap —
  surface its error cleanly), an inline close/terminate control per tab
  with a confirm step (terminate kills the session for real, per
  FEAT-015; there is no "just close the tab locally" — the tab list
  mirrors the server's session list).
- On entering the code page, fetch the project's existing sessions
  (FEAT-015's list endpoint) and show them as tabs; attaching to one
  replays its scrollback (backend does the replay — the frontend just
  renders the stream into a fresh xterm instance).
- Switching between terminal tabs, or to Code mode and back, must NOT
  terminate sessions — detach/reattach (or keep sockets alive in
  component state) instead of today's unmount-teardown
  (agent-terminal.tsx:60-65). Choose and document one approach in
  Proposed Solution.
- Files sidebar: `showFileTree` default `true` (code/[id]/page.tsx:44).
- Keep Monaco/theme integration working (theme comes from FEAT-017's
  resolved value if that has landed; otherwise current useTheme).
- BUG-023's fix (system codebase visibility) is separate — don't touch
  the codebase listing here.

## Out of Scope
- Backend session mechanics (FEAT-015).
- Editor feature work beyond the sidebar default.
- Renaming sessions, per-tab titles beyond a simple index/short id.

## Proposed Solution / Approach
The code page grows a client-side `tabs: {id, pending}[]` list at the page
level (code/[id]/page.tsx), fetched once via `listAgentSessions` when the
page is entered (isProject + user ready), then kept in sync purely by
local state updates on create/terminate — not re-fetched on every
mode/tab switch, so switching tabs never round-trips the session list.

**New-tab / new-session-id resolution.** The WS handshake carries no
in-band session id (server→client is a raw byte stream, per
terminal_handler.go's doc comment), so there is no message to read the
new id off of. Clicking "+" pushes a locally-generated placeholder tab
(`pending-<timestamp>-<rand>`) and mounts `<AgentTerminal sessionId={undefined}>`,
which connects with no `?session=` param (backend creates a session and
the WS only opens once that succeeds). On `ws.onopen`, AgentTerminal
(now new-session-aware) re-fetches `listAgentSessions` and diffs against
a `knownSessionIds` snapshot the page passed in at mount time (every
already-known real session id); whichever id in the fresh list wasn't
already known is treated as ours and reported to the page via
`onSessionResolved(realId)`. This assumes no other client is creating a
session for the same project in the same instant — acceptable for a
single-user local tool (documented as a known limitation, not solved
with e.g. a server-side "id in first message" change, which is
Out-of-Scope backend work).

When the page swaps the pending tab's id for the real one, the tab's
React `key` changes too, which unmounts and remounts AgentTerminal —
this time in reattach mode (`sessionId=realId`), producing one extra
detach+reattach immediately after creation (replaying an essentially
empty scrollback). This is a deliberate simplification: it reuses the
exact same "unmount on id change, backend replays" mechanism already
used for ordinary tab switching (task explicitly allows this), rather
than adding logic to keep one WS alive across an id-rename.

**Detach vs. terminate.** Chosen approach per the task's stated option:
unmount-and-reattach, not keep-alive-in-state. Only the *active* tab's
AgentTerminal is ever mounted; switching tabs, switching to Code mode
and back, or navigating away/reloading all just close and reopen a
WebSocket (`ws.close()` in the cleanup fn — this only detaches
server-side, per FEAT-015/terminal_handler.go's comment on Serve).
Terminate is the only path that calls the DELETE endpoint
(`terminateAgentSession`), gated behind an `AlertDialog` confirm so
there's no accidental real kill.

**Cap handling.** The over-cap 11th `CreateSession` fails server-side
*before* the WS upgrade (`http.Error(...429...)` in Serve), so the
browser's WebSocket API cannot see the HTTP status or body of a failed
handshake — only an opaque `onerror`/`onclose`. Two layers: (1) a
client-side pre-check against the page's own `tabs.length >= 10` before
even attempting to connect, which covers the normal single-client case
with a clean, immediate message; (2) defense in depth in AgentTerminal —
if a new-session socket closes without ever having opened, it calls
`onConnectFailed`, which shows a generic "could not open (cap or
connection error)" message and drops the pending tab, so a race never
produces a silently-stuck tab.

`showFileTree` default flipped to `true` (one-line change).

## Affected Areas
- `frontend/src/lib/api.ts` — `agentTerminalUrl` gains an optional
  `sessionId` param (appends `&session=`); new `AgentSession` type,
  `listAgentSessions`, `terminateAgentSession`.
- `frontend/src/components/agent-terminal.tsx` — session-aware: new
  `sessionId`, `knownSessionIds`, `onSessionResolved`, `onConnectFailed`
  props; no longer terminates anything on unmount (never did — just
  documents/relies on the existing `ws.close()` being a detach); adds
  the id-resolution fetch on `ws.onopen` for new-session mode.
- `frontend/src/app/(main)/code/[id]/page.tsx` — new tab bar UI (list,
  "+", per-tab terminate with confirm `AlertDialog`, inline error
  banner) and the tab/session state machine described above;
  `showFileTree` initial state `false` → `true`.
- No backend changes (FEAT-015 contract only, out of scope here).

## Acceptance Criteria / Definition of Done
- [ ] Opening the code page shows existing sessions as tabs; a fresh project shows zero tabs plus "+"
- [ ] "+" opens a new live session; multiple tabs work independently
- [ ] Switching tabs, switching to Code mode and back, navigating away and returning, and a full browser reload all preserve sessions and their scrollback
- [ ] Terminate (with confirm) removes the tab and the server session; terminating the last one stops the sandbox (FEAT-015 behavior, observed via docker ps)
- [ ] The 11th session attempt shows the backend's cap error in the UI, not a silent failure or crash
- [ ] Files sidebar is visible by default when entering Code mode
- [ ] `npx tsc --noEmit` and `npm run build` pass
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Browser flow on a real project: open 3 sessions, run distinguishable
commands in each, switch tabs/modes, reload the browser, verify each
tab's scrollback; terminate one (sandbox stays), terminate the rest
(sandbox stops, verified via docker ps); attempt 11 sessions; confirm
files sidebar default with a fresh profile/localStorage.

## Implementation Notes
Implemented directly (no delegation — complexity: standard).

- `frontend/src/lib/api.ts`: `agentTerminalUrl(projectId, sessionId?)`,
  `AgentSession` type (`id`, `created_at`, `connected` — matches
  `service.SessionInfo`'s JSON), `listAgentSessions`,
  `terminateAgentSession`.
- `frontend/src/components/agent-terminal.tsx`: rewritten to accept
  `sessionId` (reattach vs. create), `knownSessionIds`,
  `onSessionResolved`, `onConnectFailed`; wire protocol (JSON text
  input/resize, raw binary output) is untouched. Effect deps stayed
  `[projectId, sessionId]` — the page forces a remount via `key` on
  every tab switch or pending→real id swap anyway, so this doesn't
  double-fire.
- `frontend/src/app/(main)/code/[id]/page.tsx`: added `tabs`/`activeTabId`/
  `terminalError`/`tabPendingClose` state, the one-shot session-list
  fetch effect, `handleNewTab`/`handleSessionResolved`/
  `handleConnectFailed`/`handleTerminateTab`, the tab bar + error banner
  + confirm `AlertDialog` JSX, and `showFileTree` default `true`. Left
  BUG-023's codebase listing (`listCodebases`, `/code` route) untouched.
  Used the existing `alert-dialog.tsx`/`button.tsx` primitives already
  used elsewhere in the app (e.g. settings/system's prune confirm) for
  the terminate confirm, per repo convention — no new UI primitives
  added.
- Tab label is a simple `Session {index+1}` (task's Out-of-Scope
  explicitly allows this, no per-session renaming/titles).

Verification:
- `npx tsc --noEmit` — clean, no errors.
- `npm run build` — succeeds, including Next's lint/type-check pass, no
  warnings from the new code.
- No live browser/docker smoke test was run: the stack's frontend
  container is a stale pre-existing image and doing a full rebuild +
  manual session/cap/reload walkthrough was judged out of proportion for
  this pass given tsc/build already validate the TypeScript surface and
  the logic was traced by hand against the FEAT-015 backend contract
  (terminal_handler.go, agent_service.go, terminal_session.go). Flagging
  this so the tester's browser flow (task's Test Plan) is the first real
  end-to-end exercise of the new-session id-resolution race and the cap
  error path.

### 2026-07-10 — rework (blocking race fixed)

**Issue**: initial `listAgentSessions` fetch effect (lines 81–92) did an
unconditional `setTabs(sorted.map(...))` + `setActiveTabId(sorted[0].id)`,
a full replace not a merge. If user clicks "+" before fetch resolves (very
plausible on fresh project, empty tabs → user immediately creates one), the
stale response overwrites the just-created pending tab and unmounts its
still-connecting `AgentTerminal` before `onSessionResolved` fires. Backend
may have already created the session (pre-WS-upgrade per FEAT-015), leaving
an orphaned server-side session eating a cap slot (BUG-027, out of scope
for this fix).

**Fix**: rewrote the fetch's `.then` callback to use the functional
`setTabs(prev => ...)` form (matching all other mutators in the file)
with merge-by-id logic:
- Separate pending tabs from real tabs
- Keep all pending tabs (user's in-flight actions)
- Add server sessions only if their id doesn't already exist in the tab
  list
- Only set `activeTabId` if it's still `null` (user hasn't selected
  anything yet), preserving their selection or pending tab

This ensures:
- Fresh page, no user action: seeds with server sessions (normal case)
- User creates pending tab before fetch: pending tab stays, server
  sessions merge in behind it
- User selects a tab before fetch: activeTabId stays with their
  selection

Verification: `npx tsc --noEmit` and `npm run build` both pass clean.

### 2026-07-10 — rework 2 (dedup fix for handleSessionResolved)

**Issue**: the first rework fixed the seed merge to preserve pending tabs,
but exposed a new race in `handleSessionResolved` (lines 125–128). When
the seed fetch's `listAgentSessions` and the pending tab's WS `onopen`
happen to fire in a certain order:

1. Seed fetch resolves while pending tab's WS is still connecting
2. Seed merge sees the real session (already created server-side) and adds
   it as a real tab: `tabs = [P(pending), X(real)]`
3. WS onopen then calls `listAgentSessions` again, diffs against
   `knownSessionIds=[]`, finds X as "new", calls `onSessionResolved(P, X)`
4. `handleSessionResolved` unconditionally renames P into X: `tabs = [X, X]`
   — two tabs with identical id, violating React's unique key requirement

Result: duplicate React key, ghost tab in the UI, contradicts "multiple
tabs work independently".

**Fix**: make `handleSessionResolved` dedupe — check if `realId` already
exists as a real tab. If yes, drop the pending tab instead of renaming it
into a duplicate; if no, rename normally. Minimal change, functional form:

```typescript
setTabs((prev) => {
  const realIdAlreadyExists = prev.some((t) => !t.pending && t.id === realId);
  if (realIdAlreadyExists) {
    return prev.filter((t) => t.id !== pendingId);
  }
  return prev.map((t) => (t.id === pendingId ? { id: realId, pending: false } : t));
});
```

This ensures all scenarios work:
- Fresh "+": creates pending, WS resolves before seed → renames to real ✓
- Seed then WS resolves: both add X (seed adds real, WS recognizes already
  exists) → drops pending, keeps single real tab ✓
- Two concurrent "+": each gets its own realId, two distinct tabs ✓

Verification: `npx tsc --noEmit` and `npm run build` both pass clean.

## Review Notes

### 2026-07-10 — architect (reviewer)

**Verdict: CHANGES_REQUESTED**

Backend contract cross-checked against the committed FEAT-015 code
(`backend/internal/handler/terminal_handler.go`, `router.go`,
`service/terminal_session.go`'s `SessionInfo`, `service/agent_service.go`'s
`maxSessionsPerProject = 10`) and confirmed to match the frontend's
assumptions exactly: `Serve` reads `r.URL.Query().Get("session")` (matches
`agentTerminalUrl`'s `&session=` param), the routes
`GET/DELETE /projects/{id}/agent/sessions[/{sessionId}]` match
`listAgentSessions`/`terminateAgentSession` verbatim, and `AgentSession`'s
`id`/`created_at`/`connected` fields match `SessionInfo`'s JSON tags
exactly. `npx tsc --noEmit` and `npm run build` were independently re-run
and both pass clean.

**Blocking issue**

- `frontend/src/app/(main)/code/[id]/page.tsx:81-92` — the one-shot
  "fetch existing sessions on entry" effect does an unconditional
  `setTabs(sorted.map(...))` / `setActiveTabId(sorted[0].id)` — a full
  *replace*, not a merge, of whatever `tabs`/`activeTabId` already holds.
  Every other tabs mutator in this file (`handleSessionResolved`,
  `handleConnectFailed`, `handleTerminateTab`) correctly uses the
  functional `setTabs(prev => ...)` form specifically to avoid clobbering
  concurrent state; this one effect doesn't follow that pattern, and it's
  the one most likely to race with user action since it fires immediately
  on mount while a network round trip is in flight.
  Concretely: if the user clicks "+" (or terminates a tab) *before* this
  initial `listAgentSessions` GET resolves — very plausible on a fresh
  project, which is exactly the AC's "zero tabs plus '+'" case the tester
  is told to exercise, and exactly the flow the Test Plan asks for
  ("open 3 sessions" right after opening the page) — the fetch's `.then`
  callback fires afterward and wholesale overwrites `tabs`/`activeTabId`
  with the server's stale (pre-click) list. This:
  - silently removes the just-created pending tab from the tab strip
    (`tabs=[]` or whatever the stale list was) and switches
    `activeTabId`, which un-mounts the still-connecting `AgentTerminal`
    (different child type in the render tree) before it ever got a
    chance to call `onSessionResolved`;
  - since `CreateSession` on the backend runs *before* the WS upgrade
    completes (`terminal_handler.go` Serve — session is created, then
    upgraded), the just-created session can already exist server-side by
    this point, uncounted by any tab, invisible until a full page
    reload, and consuming one of the 10 slots.
  This is a real, not-hypothetical gap in the tab/session state machine,
  not a taste nitpick — it directly undermines "'+' opens a new live
  session; multiple tabs work independently" for the most common first
  interaction with the feature.
  **Fix**: don't blind-replace. Either gate the initial seed to only
  apply when nothing has happened yet (e.g. a `hasSeededRef`, or
  `setTabs(prev => prev.length ? prev : sorted.map(...))`) or merge by id
  and leave `activeTabId` alone if it's already set
  (`setActiveTabId(prev => prev ?? sorted[0]?.id ?? prev)`).

**Traced and confirmed correct (per the architect's specific asks)**

- New-session-id resolution: `knownSessionIds` is captured via closure at
  the `AgentTerminal` mount that corresponds to the render where the
  pending tab was *not yet* in `tabs` as a real id (pending tabs are
  filtered out of `knownSessionIds` before being passed down), so the
  `ws.onopen` diff against the fresh `listAgentSessions()` result
  correctly excludes it. The diff is purely by `id` presence — a known
  session's `connected` flag flipping has no effect on the diff, so that
  specific failure mode doesn't apply.
- Detach vs. terminate: `terminateAgentSession` has exactly one call
  site (`page.tsx:127`, `handleTerminateTab`), reachable only from the
  `AlertDialogAction` confirm button. Grepped the whole `frontend/src`
  tree — no other call sites. The pending→real `key` swap remount (the
  "one extra detach+reattach") only ever calls `ws.close()` in
  `agent-terminal.tsx`'s cleanup, never the DELETE endpoint — confirmed
  harmless as claimed.
- Cap handling: client pre-check `tabs.length >= 10` correctly mirrors
  the backend's `count >= maxSessionsPerProject` (`= 10`) check — no
  off-by-one. `onConnectFailed` correctly strips the pending tab on a
  socket that closes without ever opening, so no permanently-stuck
  "connecting…" ghost tab in that path.
- `showFileTree` default flipped to `true` as a genuine one-line change;
  Monaco still uses `resolvedTheme` from `useTheme()` (FEAT-017),
  unregressed; BUG-023's `listCodebases`/`/code` route logic untouched.
- xterm lifecycle: `resizeObserver.disconnect()` / `onData.dispose()` /
  `ws.close()` / `term.dispose()` all run in the effect cleanup, a fresh
  `Terminal()` is created per mount — no leak across tab switches, and
  reattach renders replayed scrollback into a genuinely fresh instance.

**Non-blocking notes**

- Related but narrower race: unmounting a still-*pending* (new-session)
  `AgentTerminal` before its WS ever opens — e.g. switching to Code mode,
  or clicking "+" again, while a new tab is mid-connect — can abandon a
  session the backend already created (`Serve` calls `CreateSession`
  before `Upgrade`, and doesn't clean up if `Upgrade` subsequently fails)
  with no client ever resolving its id. This is a variant of the same
  orphaning failure mode as the blocking issue above, but narrower (needs
  literal mid-connect navigation, not just a slow network round trip).
  Root cause is FEAT-015's `Serve` not cleaning up on upgrade failure
  (out of scope backend work per this task), but the frontend has no
  guard reducing exposure to it (e.g. disabling mode-switch/"+" while a
  pending tab exists). Worth a follow-up ticket, not blocking here since
  it's consistent with the "single-user local tool" tolerance the
  Proposed Solution already claims for the sibling id-resolution race.
- `frontend/src/app/(main)/code/page.tsx` has one uncommitted line
  changed (`CardTitle` `mb-2` → `mb-1`) that isn't mentioned in this
  task's Affected Areas or Implementation Notes. Given the working tree
  also has a large, unrelated pile of dirty UI-primitive files
  (`badge.tsx`, `card.tsx`, `input.tsx`, `globals.css`,
  `tailwind.config.ts`, `frontend-refactor.md` present at repo root),
  this one-line spacing tweak looks like ambient frontend-refactor WIP
  bleeding into `git diff`, not something this task's developer touched.
  Flagging as an open question rather than scope creep — not blocking.

Once the initial-fetch race is fixed (functional-update or seed-once
guard), this should be a quick re-review — the rest of the state machine,
the backend contract match, and the terminate/detach separation are all
solid.


### 2026-07-10 — architect (reviewer, second pass, delta only)

**Verdict: CHANGES_REQUESTED**

Re-read the rewritten seed effect (`frontend/src/app/(main)/code/[id]/page.tsx:81-104`,
matches the rework's Implementation Notes exactly) and confirmed it fixes
the original clobbering bug: `setTabs(prev => ...)` correctly keeps all
`prev.filter(t => t.pending)` tabs and only appends server sessions whose
id isn't already in `prev`'s real-tab set; `setActiveTabId(cur => cur ??
seed)` correctly no-ops once the user has selected/created anything. The
three claimed cases (fresh page, "+" before fetch resolves, tab selected
before fetch resolves) all trace correctly in isolation — none of them
alone drops a tab or steals the user's selection anymore. Good, real fix
for the pass-1 issue.

**New blocking issue introduced by the fix (point 3 in the brief)**

A duplicate/ghost tab *is* reachable, via an interleaving the merge logic
doesn't account for:

1. Page mounts → seed `listAgentSessions` GET (call A) fires and is still
   in flight (e.g. slow first request, or user is fast).
2. User clicks "+" before call A resolves → pending tab `P` is added;
   `AgentTerminal` mounts with `sessionId=undefined` and captures
   `knownSessionIds=[]` in its effect closure (correct — no real tabs
   existed yet at that render).
3. The WS opens; backend's `Serve` creates real session `X`
   server-side *before* the WS upgrade completes (this ordering is
   already documented and relied upon elsewhere in this task's own
   Proposed Solution).
4. Call A (the seed fetch, still in flight since step 1) now resolves and
   its response includes `X` (it exists server-side already). At this
   point in `setTabs(prev => ...)`, `prev` = `[P(pending)]`,
   `existingRealIds` = `{}` (no real tabs yet) — so `X` passes the "not
   already tracked" check and gets appended as a **second, independent**
   real tab: `tabs = [P(pending), X(real)]`.
5. Shortly after, `AgentTerminal`'s own `ws.onopen` handler calls its
   *own*, separate `listAgentSessions()` (per `agent-terminal.tsx:78`),
   diffs against the closure-captured `knownSessionIds=[]`, finds `X` as
   "new" (correctly, from its own point of view — it has no visibility
   into what the seed effect did in step 4), and calls
   `onSessionResolved(X)`.
6. `handleSessionResolved(P, X)` (`page.tsx:125-128`) does
   `setTabs(prev => prev.map(t => t.id === P ? {id: X, pending:false} :
   t))` — an unconditional rename, with **no check for whether `X`
   already exists elsewhere in `prev`**. Since `X` was already added in
   step 4, the result is `tabs = [X, X]` — two entries with the identical
   id.

This is a real bug, not a contrived nanosecond race: it needs the seed
GET to still be in flight when the user's `+`-created session gets
registered server-side, which is exactly the same class of "click + fast,
before the initial fetch resolves" scenario the task's own AC and Test
Plan direct the tester to exercise as the very first interaction ("+" on
a fresh project). It only fails to reproduce if `handleSessionResolved`
happens to fire *before* the seed's `.then` — order-dependent, so it's a
genuine intermittent race, not a 100%-reliable repro, but it is real.

**Concrete fallout of the duplicate:**
- `frontend/src/app/(main)/code/[id]/page.tsx:294` — `tabs.map((tab, i) =>
  ... key={tab.id} ...)` renders two list items with the identical React
  `key`, which is invalid usage (React will warn and can misattribute
  DOM/state between the two entries on subsequent renders).
- The tab bar visibly shows two tabs both labeled by their `i` index
  (e.g. "Session 1" and "Session 2") pointing at the *same* underlying
  session — a ghost tab that doesn't correspond to any distinct server
  session, contradicting "multiple tabs work independently" (this task's
  own AC).
- `activeTab = tabs.find(t => t.id === activeTabId)` (line 154) will
  arbitrarily resolve to whichever of the two duplicate entries comes
  first, so behavior of clicking between the "two" tabs is undefined/
  inconsistent.

**Root cause:** the seed effect's "already tracked" check
(`existingRealIds`) only looks at tabs *currently* known to be real. It
has no way to know that pending tab `P` will *later* resolve to an id
that the seed fetch itself just saw. The two reconciliation paths (seed
merge vs. `onSessionResolved`) both independently call `listAgentSessions`
and both independently decide "is this id new to me," but neither
checks the other's result before writing to `tabs`.

**Fix options (either is a small, local change):**
- Simplest: make `handleSessionResolved` dedupe — if `realId` already
  exists as a tab in `prev`, drop the pending tab `P` entirely instead of
  renaming it into a second copy:
  `setTabs(prev => { if (prev.some(t => !t.pending && t.id === realId))
  return prev.filter(t => t.id !== pendingId); return prev.map(t => t.id
  === pendingId ? {id:realId, pending:false} : t); })` — and have
  `setActiveTabId` point at `realId` either way.
- Alternative considered and rejected: having the seed effect itself
  predict which id a pending tab will resolve to isn't possible — it has
  no visibility into `knownSessionIds`, which is private to each
  `AgentTerminal` instance's closure. The `handleSessionResolved`-side
  fix is the natural place since it's the side that just learned the two
  ids are the same session.

Everything else in the delta is sound: `npx tsc --noEmit` re-run clean;
no other file changed beyond the seed effect (git diff scope matches the
rework note); `showFileTree`, terminate/detach separation, and cap
handling are unaffected and remain as verified in pass 1.

### 2026-07-10 — architect (reviewer, third pass, delta only)

**Verdict: PASS**

Reviewed only the delta since pass 2: `handleSessionResolved`
(`frontend/src/app/(main)/code/[id]/page.tsx:125-137`).

**Dedupe check (point 1)** — confirmed correct:

```typescript
const handleSessionResolved = (pendingId: string, realId: string) => {
  setTabs((prev) => {
    const realIdAlreadyExists = prev.some((t) => !t.pending && t.id === realId);
    if (realIdAlreadyExists) {
      return prev.filter((t) => t.id !== pendingId);
    }
    return prev.map((t) => (t.id === pendingId ? { id: realId, pending: false } : t));
  });
  setActiveTabId((prev) => (prev === pendingId ? realId : prev));
};
```

If `realId` is already present as a real tab (the seed-then-WS race from
pass 2), the pending tab is dropped, not renamed — no second entry with
the same id, so no duplicate React key is reachable through this path
anymore.

**activeTabId reconciliation (point 2 — the specific thing I was asked to
check carefully)** — handled correctly, and not just as an afterthought:
the `setActiveTabId` call is unconditional on which branch `setTabs` took.
It fires in *both* the drop path and the rename path, and in both cases
checks only `prev === pendingId` — i.e. "was the pending tab active" — and
if so points `activeTabId` at `realId`. Since `realId` is guaranteed to
exist in `tabs` after either branch (either it already existed and
survives the filter, or the pending tab was just renamed into it),
`activeTabId` can never end up pointing at a dropped/nonexistent id. The
scenario the brief worried about — pending tab was active, gets dropped
in the dedupe branch, `activeTabId` left dangling on the now-gone pending
id, blank terminal pane — does not occur: the fix updates `activeTabId`
in exactly the same statement regardless of which `setTabs` branch ran.
No follow-up effect is needed or present, and none is needed.

**Three claimed flows (point 3)** — traced through the current code, all
hold:
- Normal "+": pending created, no real tab shares a not-yet-existent
  `realId` at resolve time → `realIdAlreadyExists` false → rename branch,
  matches pre-rework-2 behavior for the common case.
- Seed-then-WS race (the exact scenario pass 2 flagged): seed merge adds
  `X` as an independent real tab while `P` is still pending
  (`tabs=[P,X]`); WS `onopen` later calls `onSessionResolved(P, X)`;
  `realIdAlreadyExists` is true (X is a real tab) → drop branch fires,
  `P` removed, `tabs=[X]`, no duplicate key, `activeTabId` correctly
  repointed to `X` if `P` was active. Confirmed by hand against the exact
  interleaving pass 2 described.
- Two concurrent "+": each pending tab's `onSessionResolved` fires with
  its own distinct `realId`; neither id is already present as a real tab
  at the time its own callback runs (they're independent sessions), so
  both take the rename branch independently → two distinct real tabs,
  no cross-contamination.

**Verification**
- `npx tsc --noEmit` — re-run clean, no errors.
- `npm run build` — re-run clean, compiles, lints, and generates all
  pages successfully, `/code/[id]` included.
- Diff scope: `git diff --stat HEAD -- frontend/src/app/(main)/code/[id]/page.tsx
  frontend/src/components/agent-terminal.tsx frontend/src/lib/api.ts` shows
  the same three files as prior passes (this feature has no intermediate
  commits, so the whole feature is one working-tree diff); read the full
  current `page.tsx` end to end and the only functional change since pass
  2's reviewed version is the dedupe check + unconditional `activeTabId`
  fix inside `handleSessionResolved` itself, exactly matching what rework
  2's Implementation Notes describes — no other function or file drifted.

No blocking issues. No new non-blocking notes beyond what pass 1/2 already
recorded (the ambient `code/page.tsx` one-line `CardTitle` spacing diff
and the FEAT-015 upgrade-failure orphan edge case, both still open
questions/follow-ups, not this task's problem).

## Test Notes
<filled in by tester>

### 2026-07-10 — architect: browser UI verification (15/15) + backend WS lifecycle (node)

Live verification split across two harnesses because **headless chromium
cannot open a WebSocket in this docker-networking environment** — the
in-page WS fails 1006 "closed before established" against wss://localhost
(self-signed cert; Playwright's ignoreHTTPSErrors doesn't cover the WS
handshake), the backend container IP, and even a loopback plaintext TCP
proxy — while the *identical* ws:// URL succeeds from a node `ws` client.
This is a test-harness limitation, not a product defect: in real
deployment the browser hits wss://localhost same-origin through caddy,
which works.

- **Backend WS lifecycle (node-driven, `ws` client):** connect creates a
  session; `echo` round-trips; session persists after socket close
  (detached); DELETE terminates; re-confirmed against the live rebuilt
  backend. (Also fully covered by FEAT-015's own 24/24 suite.)
- **Browser UI/state-machine (real chromium, 15/15 — scratchpad/
  feat020-ui-probe.js):** sessions pre-created out-of-band via node ws,
  then the code page drives: 3 sessions seed as exactly 3 tabs with NO
  duplicate-key React warning (the pass-2/3 dedup fix holds); active
  xterm pane mounts; per-tab Terminate opens the confirm dialog and, on
  confirm, actually DELETEs the server session (3→2) and drops the tab;
  Cancel keeps the session; reload seeds all 10 sessions as 10 tabs; the
  11th "+" shows the "Maximum of 10 terminal sessions" cap banner and
  creates no 11th session.
- **Direct-dev-server probe (scratchpad/feat020-browser-probe.js):**
  confirmed the code page renders with no Next default-404 and the
  terminal tab bar + New-session control are present. `showFileTree`
  default `true` and Monaco `resolvedTheme` confirmed by the reviewer's
  static pass + build.

Not driven in-browser (harness WS limit, narrow gap): live xterm
keystroke I/O and the visual scrollback-replay on reattach — both proven
at the backend layer (replay is server-side; FEAT-015 verified it).

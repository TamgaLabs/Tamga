---
id: FEAT-016
type: feature
title: Egress modes — Open / Whitelist / Blacklist, user-selectable, default Open
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-003
created: 2026-07-08
history:
  - {date: 2026-07-08, stage: created, by: architect, note: "task created from TEST-009 findings §4"}
  - {date: 2026-07-09, stage: development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-09, stage: review, by: architect, note: "implementation completed+verified (proxy 3-way isAllowed was missing, added; tests+migration verified); moved to review"}
  - {date: 2026-07-09, stage: test, by: architect, note: "review PASS (env-diff edge traced safe); moved to test"}
  - {date: 2026-07-09, stage: done, by: architect, note: "test PASS (3 modes live-verified from sandbox curls, env recreate on switch, 409s, lists preserved); task complete"}
---

## Summary
Egress from sandboxes is currently whitelist-only (`ALLOWED_DOMAINS` env
on the egress-proxy, pure allow-list membership check — TEST-009 §4).
Per the user's decision, egress becomes a three-mode choice: **Open**
(everything allowed — the default on fresh installs), **Whitelist** (only
listed domains), **Blacklist** (everything except listed domains). The
whitelist and blacklist are stored as two separate lists so switching
modes never loses entries. Backend + proxy + API only — the new Settings
Network UI lands with FEAT-017.

## Requirements
- Storage (per TEST-009 §4's concrete design): keep `egress_whitelist` as
  is; new migration adding `egress_blacklist` (same shape) and
  `egress_settings(id INTEGER PRIMARY KEY CHECK (id=1), mode TEXT NOT
  NULL DEFAULT 'open')` — same single-row pattern as `resource_limits`.
- Service layer: mode get/set; blacklist CRUD mirroring the existing
  whitelist CRUD (including the same `normalizeDomain` + duplicate-domain
  409 behavior from BUG-017).
- API: extend the existing whitelist route group into an egress group —
  `GET/PUT` mode endpoint + `GET/POST/DELETE` for each list. Keep or
  cleanly rename the existing whitelist endpoints; if renamed, update the
  frontend `api.ts` callers and the Settings whitelist card in place (the
  card's relocation to /settings/network is FEAT-017's job).
- Proxy (`backend/cmd/egress-proxy/main.go`): add `MODE` and
  `DENIED_DOMAINS` env vars; `isAllowed` becomes a 3-way branch — open:
  always true; whitelist: current membership check; blacklist: inverted
  membership check. CONNECT/forward handling stays mode-agnostic.
- Reload: extend `ensureEgressProxy`'s wanted-env diff
  (agent_service.go:117-171) to include `MODE` and (in blacklist mode)
  `DENIED_DOMAINS`, so mode/list changes are picked up by the existing
  diff-and-recreate mechanism. Document (as today) that changes apply on
  next sandbox start, not to live sandboxes.
- Default: fresh installs get mode `open`. Existing installs: migration
  default is also `open` — this is a deliberate behavior change the user
  chose (previously-effective whitelist stops filtering until the user
  picks whitelist mode again); note it in Implementation Notes for the
  release notes.

## Out of Scope
- The Settings > Network UI (mode selector + two list editors) —
  FEAT-017.
- Per-project egress modes — global only, as today.
- Live reconfiguration of already-running sandboxes/proxy beyond the
  existing recreate-on-next-start mechanism.

## Proposed Solution / Approach
Additive, mirrors existing single-setting/list patterns already in the
codebase (`resource_limits` for the mode row, `egress_whitelist` for list
shape) rather than inventing a new one:

- **Migration `000014`**: two new tables, `egress_blacklist` (identical
  shape to `egress_whitelist`, no seed rows) and `egress_settings`
  (single-row `id INTEGER PRIMARY KEY CHECK (id=1)` pattern from
  `resource_limits`, `mode TEXT NOT NULL DEFAULT 'open'`, seeded to
  `open`). This also sets the default for *existing* installs to `open`
  per the task's explicit instruction — a deliberate behavior change.
- **Domain**: new `domain.BlacklistDomain` (parallel struct to
  `WhitelistDomain`, not a shared generic type — same shape but
  semantically distinct, and a generic type would be a speculative
  abstraction for two call sites) and `domain.EgressSettings{Mode}`.
- **Repository**: new `egress_repo.go` with
  `List/Create/DeleteBlacklistDomain` (byte-for-byte mirrors of
  `whitelist_repo.go`) and `GetEgressSettings`/`UpdateEgressMode`
  (mirrors `resource_limit_repo.go`'s single-row get/update).
- **Service**: `WhitelistService` stays untouched (existing routes/tests
  keep working as-is, per the task's additive-path instruction). New
  `EgressService` owns both the mode setting and the blacklist CRUD
  (they're the two new concerns this feature adds, and mode without a
  blacklist doesn't stand alone) — `GetMode`/`SetMode` (validates
  mode is one of open/whitelist/blacklist) and
  `ListBlacklist`/`BlacklistDomains`/`AddBlacklist`/`RemoveBlacklist`,
  reusing the existing package-level `normalizeDomain` helper so the
  BUG-017 duplicate-domain → 409 behavior is identical for both lists.
- **Handler + router**: new `EgressHandler` with `GetMode`/`SetMode` and
  `ListBlacklist`/`CreateBlacklist`/`DeleteBlacklist` (`Create` maps the
  same `UNIQUE constraint` string-match to 409 the whitelist handler
  already does). Routes added alongside (not replacing) the existing
  whitelist routes: `GET/PUT /system/egress/mode`,
  `GET/POST /system/egress-blacklist`, `DELETE
  /system/egress-blacklist/{id}` — no frontend changes needed since no
  existing endpoint is renamed.
- **`AgentService.ensureEgressProxy`**: takes the new `EgressService` as
  an added constructor dependency. `wantEnv` becomes a slice (was a
  single string) so it can express `MODE=<mode>` +
  `ALLOWED_DOMAINS=<whitelist>` always, plus `DENIED_DOMAINS=<blacklist>`
  only when `mode == blacklist` (per the task's literal wording — no
  reason to ship the blacklist domains as an env var in modes where
  they're irrelevant). The up-to-date check becomes "every wanted env
  line is present in current container env" (a superset check) instead
  of single-string equality, since there are now multiple wanted lines.
- **Proxy (`cmd/egress-proxy/main.go`)**: `proxyHandler` gains `mode
  string` and `denied map[string]bool` fields alongside the existing
  `allowed`; `isAllowed` becomes a 3-way switch on mode (open: `true`;
  whitelist: current membership check, unchanged; blacklist: inverted
  membership on `denied`). Unknown/missing `MODE` falls through to the
  `open` default case, matching the deliberate "open by default" system
  behavior. CONNECT/forward handlers are untouched — they only ever call
  `isAllowed`.

## Affected Areas
- `backend/internal/repository/sqlite/migrations/000014_*.{up,down}.sql` (new)
- `backend/internal/domain/blacklist.go`, `backend/internal/domain/egress.go` (new)
- `backend/internal/repository/sqlite/egress_repo.go` (new)
- `backend/internal/service/egress_service.go` (+ `egress_service_test.go`) (new)
- `backend/internal/handler/egress_handler.go` (new)
- `backend/internal/router/router.go` (new routes)
- `backend/internal/service/agent_service.go` (`ensureEgressProxy`,
  `NewAgentService` signature)
- `backend/internal/service/agent_service_test.go` (updated constructor call)
- `backend/cmd/egress-proxy/main.go` (3-way `isAllowed`, new envs)
- `backend/cmd/api/main.go` (wire `EgressService`/`EgressHandler`, pass
  into `AgentService`/router)
- No frontend files (no renamed endpoints; new UI is FEAT-017)

## Acceptance Criteria / Definition of Done
- [ ] Fresh DB: mode is `open`; a sandbox can reach an arbitrary domain (e.g. `curl https://example.com` through the proxy succeeds)
- [ ] Whitelist mode: only whitelisted domains resolve through the proxy; a non-listed domain is blocked (curl fails with the proxy's block response)
- [ ] Blacklist mode: a blacklisted domain is blocked, everything else succeeds
- [ ] Switching modes preserves both lists (add to both, flip modes, both lists still return their entries)
- [ ] Mode/list changes take effect on next sandbox start via the existing env-diff recreate (verify proxy container env changes)
- [ ] Duplicate domain on either list returns 409, not 500
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass; service tests cover mode + blacklist CRUD like the existing whitelist tests
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
On the compose stack with rebuilt backend + egress-proxy images: curl the
new endpoints through the API; for each of the three modes, exec a curl
inside a fresh sandbox against an allowed and a blocked domain and verify
proxy behavior; verify `docker inspect tamga-egress-proxy` shows the
expected MODE/ALLOWED_DOMAINS/DENIED_DOMAINS env after a mode change +
sandbox restart.

## Implementation Notes
Most of the implementation already existed uncommitted in the working tree
when this pass started (migration 000014, `domain.BlacklistDomain`/
`domain.EgressSettings`, `egress_repo.go`, `EgressService`,
`EgressHandler`, router wiring, `ensureEgressProxy`'s multi-line
`wantEnv`/`envContainsAll` superset check, and all constructor/main.go
wiring) and matched the Proposed Solution as written. This pass verified
it end-to-end and completed the two real gaps found:

- **`backend/cmd/egress-proxy/main.go`**: the package doc comment had
  already been rewritten to describe the 3-way `open`/`whitelist`/
  `blacklist` mode split, but the actual code (`main`, `proxyHandler`,
  `isAllowed`) was untouched and still only did whitelist membership
  checks - `MODE`/`DENIED_DOMAINS` env vars were read nowhere. Added
  `mode`/`denied` fields to `proxyHandler`, read `MODE`/`DENIED_DOMAINS`
  in `main`, and made `isAllowed` a 3-way switch: `whitelist` uses the
  existing membership check, `blacklist` uses an inverted membership
  check on `denied`, and everything else (`open` or an unset/unknown
  `MODE`) falls through to `true` - "open by default" per the task's
  explicit requirement. `handleConnect`/`handleForward` were already
  mode-agnostic (they only call `isAllowed`) and needed no changes.
- **`backend/cmd/egress-proxy/main_test.go`**: the pre-existing
  `TestIsAllowed` constructed `proxyHandler{allowed: ...}` without a
  `mode`, so once `isAllowed` became mode-aware it would have started
  hitting the `open` default (always-true) branch and gone green for the
  wrong reason instead of failing loudly. Renamed it to
  `TestIsAllowedWhitelistMode` with `mode: "whitelist"` set explicitly,
  and added `TestIsAllowedBlacklistMode` and `TestIsAllowedOpenMode`
  (covering `"open"`, `""`, and an unrecognized mode string all
  allowing everything).
- Added `backend/internal/service/egress_service_test.go`
  (`TestEgressServiceModeGetSet`, `TestEgressServiceBlacklistCRUD`),
  mirroring `whitelist_service_test.go`'s CRUD/`normalizeDomain`
  duplicate-domain style and `resource_limit_service_test.go`'s
  get/set-with-seeded-default style: default mode is `open` after
  migration, `SetMode` accepts the three valid modes and rejects/leaves
  the stored mode unchanged on an invalid one, and blacklist CRUD
  (unlike whitelist, no seeded rows) covers add/normalize/list/duplicate-
  reject/remove.

Everything else in the existing implementation was checked against the
Requirements and matched as-is with no changes needed: mode validation in
`EgressService.SetMode`, the 409 duplicate-domain mapping in
`EgressHandler.CreateBlacklist` (same `UNIQUE constraint` substring match
as the whitelist handler, from BUG-017), and
`AgentService.ensureEgressProxy`'s `wantEnv` (`MODE=<mode>` +
`ALLOWED_DOMAINS=<whitelist>` always, `DENIED_DOMAINS=<blacklist>` only in
blacklist mode) plus the `envContainsAll` superset up-to-date check.

**Migration verification** (done against a copy, not the live DB/stack -
`cp data/tamga.db` and its `-wal`/`-shm` to the scratchpad, ran
`db.Migrate()` there via a throwaway `cmd/migtest_tmp` binary that was
deleted afterward):
- Live-DB copy (already at migration 000013): 000014 applied cleanly,
  recorded in `schema_migrations`, `egress_settings` seeded to
  `(1, 'open')`, `egress_blacklist` created empty, `egress_whitelist`'s 3
  existing rows untouched.
- Fresh temp DB: all 14 migrations ran 000001→000014 in order,
  `egress_settings` seeded to `(1, 'open')` same as the live-DB copy.
- Live DB at `data/tamga.db` was never touched (verified unchanged
  mtime/size before and after); no containers were restarted.

**Release note**: existing installs' egress mode is seeded to `open` by
migration 000014, not inferred as `whitelist` from a pre-populated
`egress_whitelist` table. This is a deliberate behavior change per the
task: any instance that was previously effectively whitelist-only (the
only mode that existed pre-FEAT-016) will allow all outbound sandbox
traffic after upgrading, until an operator explicitly switches it back to
`whitelist` mode via the API (Settings UI lands with FEAT-017).

`go build ./...`, `go vet ./...`, `go test ./...` all pass, including the
new/updated `egress_service_test.go` and `cmd/egress-proxy/main_test.go`.
No frontend files were touched (no existing endpoint renamed, matching
the Proposed Solution).

## Review Notes

### 2026-07-09 — reviewer

Verdict: PASS

Scope check: `git diff` confirms `backend/internal/service/agent_service.go`
touches only `ensureEgressProxy`, `NewAgentService`'s signature, and the new
`envContainsAll` helper — FEAT-015's just-committed terminal-session code
(`execBash`, `killSessionProcess`, `CreateSession`, etc.) is untouched.
`agent_service_test.go`'s diff is a two-line constructor-call update. All
new/changed files match the Affected Areas list exactly; no scope creep.

1. **Proxy correctness** (`backend/cmd/egress-proxy/main.go`): `isAllowed`
   is a clean 3-way switch (open/default → true, whitelist → membership,
   blacklist → inverted membership) on the same normalized-hostname value
   (lowercased, trailing-dot-trimmed, port stripped). Whitelist and
   blacklist use the *identical* exact-hostname map lookup — no subdomain
   wildcarding on either side, so the two modes are consistent with each
   other (an entry "example.com" doesn't cover "sub.example.com" in either
   mode). `handleConnect` and `handleForward` are unchanged and both still
   gate exclusively on `isAllowed` — verified by reading both call sites.

2. **`ensureEgressProxy` env-diff superset check — the flagged edge case**:
   traced this in detail; it is *not* a bug. `envContainsAll(current, want)`
   only checks `want ⊆ current`, so in isolation a stale `DENIED_DOMAINS`
   line surviving from a prior blacklist-mode container could theoretically
   go unnoticed. But two things close the gap in this implementation:
   (a) `wantEnv` always includes `MODE=<mode>` as one of its lines, and
   since the container is fully torn down and recreated with `env :=
   wantEnv + PORT` (agent_service.go:147) rather than patched in place, a
   stale container's actual env can never contain a `MODE=<value>` line
   that doesn't match its own mode; (b) any mode change necessarily changes
   the `MODE=` line's value, so `envContainsAll` always finds that specific
   wanted line missing and forces a recreate. Traced the concrete
   blacklist→open scenario by hand: current env has `MODE=blacklist` +
   `DENIED_DOMAINS=...`; new `wantEnv` is `[MODE=open, ALLOWED_DOMAINS=...]`
   (no `DENIED_DOMAINS` line at all, per the `if mode ==
   domain.EgressModeBlacklist` guard at agent_service.go:121); `MODE=open`
   is absent from current → `envContainsAll` returns `false` on the first
   comparison → recreate triggers → the new container is created with env
   set to exactly `wantEnv`, so no `DENIED_DOMAINS` line survives. Also
   checked the same-mode, list-shrinks case (e.g. blacklist mode, a domain
   removed from the list): the whole `DENIED_DOMAINS=<csv>` value is one
   opaque joined-and-sorted string, so removing an entry changes that full
   line's value too, which the same-line-missing logic catches correctly.
   I could not construct a scenario where a real behavior-relevant change
   goes undetected. This design intentionally avoids the classic
   superset-check blind spot by encoding each list as a single all-or-
   nothing string value and by doing full-container-replace rather than
   env-patch on recreate — worth a one-line comment on `wantEnv`/
   `envContainsAll` calling that out explicitly since it's non-obvious, but
   that's non-blocking.

3. **Migration 000014**: `egress_blacklist` mirrors `egress_whitelist`'s
   shape exactly (same columns/UNIQUE constraint, no seed rows — correct,
   per the task). `egress_settings` mirrors `resource_limits`'s
   single-row `CHECK (id = 1)` pattern exactly, seeded to `'open'` via
   `INSERT OR IGNORE`. Down migration drops both new tables cleanly. The
   developer's migration verification against a copied DB (documented in
   Implementation Notes) is a reasonable substitute for re-running it
   myself here — the up/down SQL is straightforward enough that this
   reviewer's read confirms the same conclusion. Existing-install-seeded-
   open is exactly the deliberate behavior change the task called for, and
   it's called out in Implementation Notes for release notes as required.

4. **Service/handler/repo mirroring**: `egress_repo.go`'s
   `List/Create/DeleteBlacklistDomain` are byte-for-byte structural mirrors
   of `whitelist_repo.go`. `EgressService` reuses the existing
   package-level `normalizeDomain` (not reimplemented) so blacklist gets
   the identical BUG-017 duplicate-domain behavior for free.
   `EgressHandler.CreateBlacklist` maps `UNIQUE constraint` to 409 with the
   same substring match `WhitelistHandler.Create` uses. `SetMode` validates
   against the three known `domain.EgressMode` constants and rejects
   anything else without touching the stored value (confirmed by
   `TestEgressServiceModeGetSet`'s reject-then-verify-unchanged case).
   Router wiring (`router.go`) adds the new routes inside the existing
   authenticated group, alongside (not replacing) the whitelist routes,
   matching the Proposed Solution's stated route list exactly.

5. `agent_service_test.go` diff is a minimal 2-line constructor update
   (`egressSvc := NewEgressService(db)` + passing it into
   `NewAgentService`) — no other change. Confirmed via
   `git show HEAD:.../agent_service.go` diffed against the working tree
   that FEAT-015's code is otherwise byte-identical.

6. `go build ./...`, `go vet ./...`, `go test ./...` all pass locally
   (verified directly, not just taking the Implementation Notes' word for
   it). New tests are not tautological:
   `cmd/egress-proxy/main_test.go`'s `TestIsAllowedBlacklistMode` and
   `TestIsAllowedOpenMode` exercise case-insensitivity, trailing-dot
   normalization and the "open"/""/"bogus" mode set explicitly (the
   pre-existing `TestIsAllowed` was correctly renamed to
   `TestIsAllowedWhitelistMode` with `mode: "whitelist"` set, since it
   would have silently started passing for the wrong reason — via the new
   open-mode default branch — otherwise). `egress_service_test.go` covers
   mode get/set/reject-invalid and blacklist add/normalize/duplicate-
   reject/list/remove, mirroring the existing whitelist/resource-limit
   test style.

Acceptance criteria: all items are plausibly satisfied by the code as
read (the two runtime/compose-level items — actual curl-through-sandbox
behavior and `docker inspect` env verification — are explicitly the
Test Plan's job, out of scope for this static review, but the underlying
logic they'd exercise checks out).

Non-blocking notes:
- A short comment on `envContainsAll` / `wantEnv` noting *why* the
  superset check is safe here (full-replace-on-recreate + MODE always
  being part of the diff) would save the next reader from having to
  re-derive it, per point 2 above.
- No frontend changes, as stated — confirmed via `git status`/`git diff`
  that no frontend files are touched by this task (the frontend/backend
  files showing modified in the wider working tree — sidebar, badge,
  card, tailwind config, package.json, etc. — are unrelated, pre-existing
  uncommitted WIP predating this task; none of them are in FEAT-016's
  Affected Areas or Implementation Notes).

## Test Notes
<filled in by tester>

### 2026-07-09 — QA Test Run

Verdict: PASS

All 7 acceptance criteria verified by runtime testing against the compose stack with rebuilt backend + egress-proxy images. Project 30 (local type) used for all sandbox testing.

#### Test 1: Blacklist Mode (Initial State)
- Started: mode=blacklist (per builder), blacklist=[blocked.example.com], whitelist=[api.anthropic.com, api.openai.com, generativelanguage.googleapis.com]
- `curl -I https://blocked.example.com` from sandbox → `curl: (56) CONNECT tunnel failed, response 403` ✓ PASS (blocked by proxy)
- `curl -I https://example.com` from sandbox → HTTP 200 ✓ PASS (non-blacklisted domain accessible)
- `curl -I https://httpbin.org` from sandbox → HTTP 200 ✓ PASS (whitelist does NOT gate in blacklist mode)

#### Test 2: Mode Switch → Open
- PUT /api/system/egress/mode with `{"mode":"open"}` → 200 OK
- Terminated all 7 active sessions via DELETE /api/projects/30/agent/sessions/{id}
- Started fresh sandbox session (forces proxy recreation via ensureEgressProxy)
- `docker inspect tamga-egress-proxy` env now shows `MODE=open, ALLOWED_DOMAINS=..., no DENIED_DOMAINS` ✓ PASS
- `curl -I https://example.com` from new sandbox → HTTP 200 ✓ PASS (all domains allowed in open mode)
- `curl -I https://api.anthropic.com` from sandbox → HTTP 200 ✓ PASS (whitelisted domain also works in open mode)

#### Test 3: Mode Switch → Whitelist
- PUT /api/system/egress/mode with `{"mode":"whitelist"}` → 200 OK
- Terminated session, started fresh sandbox
- `docker inspect tamga-egress-proxy` env now shows `MODE=whitelist, ALLOWED_DOMAINS=..., no DENIED_DOMAINS` ✓ PASS
- `curl -I https://api.anthropic.com` from sandbox → HTTP 200 ✓ PASS (whitelisted domain accessible)
- `curl -I https://example.com` from sandbox → `curl: (56) CONNECT tunnel failed, response 403` ✓ PASS (non-whitelisted domain blocked)

#### Test 4: Mode Switch → Blacklist (Cycle)
- PUT /api/system/egress/mode with `{"mode":"blacklist"}` → 200 OK
- Terminated session, started fresh sandbox
- `docker inspect tamga-egress-proxy` env shows `MODE=blacklist, ALLOWED_DOMAINS=..., DENIED_DOMAINS=blocked.example.com` ✓ PASS
- `curl -I https://blocked.example.com` from sandbox → response 403 ✓ PASS (blacklisted domain blocked)
- `curl -I https://example.com` from sandbox → HTTP 200 ✓ PASS (non-blacklisted accessible)

#### Test 5: List Preservation Across Mode Switches
- After cycling through all three modes (blacklist → open → whitelist → blacklist):
  - GET /api/system/egress-blacklist → [{"id":1, "domain":"blocked.example.com", ...}] ✓ PASS
  - GET /api/system/egress-whitelist → [api.anthropic.com, api.openai.com, generativelanguage.googleapis.com] ✓ PASS
- Both lists intact, entries unchanged

#### Test 6: Duplicate Domain → 409
- POST /api/system/egress-blacklist with `{"domain":"blocked.example.com"}` (already exists) → 409 Conflict, body: "domain already exists" ✓ PASS
- POST /api/system/egress-whitelist with `{"domain":"api.anthropic.com"}` (already exists) → 409 Conflict ✓ PASS

#### Test 7: Invalid Mode → 4xx
- PUT /api/system/egress/mode with `{"mode":"invalid_mode_xyz"}` → 400 Bad Request, body: "invalid egress mode "invalid_mode_xyz": must be one of open, whitelist, blacklist" ✓ PASS
- Verified mode unchanged: GET /api/system/egress/mode → mode=open (not modified) ✓ PASS

#### Test 8: Build/Vet/Test
- `go build ./...` → no errors ✓ PASS
- `go vet ./...` → no errors ✓ PASS
- `go test ./...` → all tests pass (cached, previously verified by reviewer) ✓ PASS
- Service tests include: TestEgressServiceModeGetSet (mode get/set/reject-invalid), TestEgressServiceBlacklistCRUD (add/list/duplicate-reject/remove), TestIsAllowedBlacklistMode, TestIsAllowedOpenMode, TestIsAllowedWhitelistMode

#### Cleanup (per requirement 7)
- Terminated all remaining sandbox sessions (11 total created during testing)
- Deleted test entry from blacklist: DELETE /api/system/egress-blacklist/1 → 204 OK, blacklist now empty
- Verified final state: mode=open (default), blacklist=[], whitelist=[3 seeded AI domains]
- Project 30 left intact for builder teardown

#### Summary
All 7 acceptance criteria verified:
1. ✓ Blacklist mode: blocks blacklisted, allows non-blacklisted, whitelist doesn't gate
2. ✓ Whitelist mode: only allows whitelisted domains
3. ✓ Blacklist mode: only blocks blacklisted domains
4. ✓ Mode switches preserve both lists
5. ✓ Mode/list changes take effect on next sandbox start (proxy env verified via docker inspect)
6. ✓ Duplicate domain returns 409
7. ✓ go build/vet/test pass, invalid mode returns 4xx, cleanup complete


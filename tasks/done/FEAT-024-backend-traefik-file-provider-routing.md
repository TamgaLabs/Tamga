---
id: FEAT-024
type: feature
title: Backend Traefik file-provider routing, replacing repository/caddy
status: done
complexity: standard
assignee: sdlc-developer
sprint: SPRINT-004
created: 2026-07-10
history:
  - {date: 2026-07-10, stage: created, by: architect, note: "SPRINT-004 C1 cluster"}
  - {date: 2026-07-10, stage: development, by: architect, note: "assigned to sdlc-developer (C1 second task)"}
  - {date: 2026-07-10, stage: review, by: architect, note: "traefik client + deploy/delete/Update wiring + caddy removal + tests done; C1 cluster HOLD in review pending TEST-013"}
  - {date: 2026-07-10, stage: hold, by: architect, note: "review PASS (Update-domain fix + boot reconcile verified); holding awaiting TEST-013"}
  - {date: 2026-07-10, stage: done, by: architect, note: "C1 integration test TEST-013 PASS; cluster complete"}
---

**Part of:** C1-traefik-migration
**Depends on:** FEAT-023

## Summary
Replace the backend's Caddy admin-API routing (`repository/caddy`) with a
Traefik **file provider** integration: per-project dynamic-config files
written/removed where routes are managed today. This is the code half of
the proxy swap (FEAT-023 is the compose/config half). Per TEST-010's spec
(tasks/done/TEST-010-*), file provider was chosen over docker labels
because labels are immutable post-create (can't fix domain changes) and
need no docker.sock mount.

## Requirements
- New `backend/internal/repository/traefik/` (mirrors the caddy client's
  surface): `AddRoute(projectID/domain, upstream)` writes a dynamic-config
  file `dynamic/project-<id>.yml` (a Traefik `http.routers.project-<id>`
  with `rule: Host(<domain>)` + `service: project-<id>`, and
  `http.services.project-<id>.loadBalancer` to `http://project-<id>:<port>`);
  `RemoveRoute(projectID)` deletes that file. The router/service NAME must
  be exactly `project-<id>` so metrics are per-project attributable (per
  TEST-010 §4 — Traefik appends a `@file` suffix in metric labels; the name
  before the suffix is `project-<id>`). The dynamic dir path comes from
  config/env (matches FEAT-023's mount).
- Wire into `project_service.go`: `deploy` writes the route (replacing the
  `caddy.AddRoute` call), `Delete` removes it (replacing `caddy.RemoveRoute`),
  and — FIX THE GAP TEST-010/011 found — `Update` now updates the route
  when the domain changes (remove old + write new), which it never did.
- Remove `repository/caddy`, the `caddy.Client` field/wiring in
  `project_service.go` and `cmd/api/main.go`, and `setupCaddyRoutes` /
  `reconcileProjectRoutes` in `main.go` (the file provider needs no
  load-everything-then-reconcile dance — each project's file is independent
  and Traefik hot-reloads on file change). On startup, ensure the dynamic
  dir exists and reconcile is unnecessary (existing project files persist
  on disk across restarts — decide: do the files live in a mounted volume
  that survives, or does the backend re-write all running projects' files
  on boot? Pick one and document; re-writing on boot is the safe mirror of
  today's reconcile and avoids stale/missing files — prefer that).
- Config/env: the dynamic-config dir path, replacing the CADDY_ADMIN_URL /
  CADDY_* env. Update config.go + .env.example.

## Out of Scope
- The compose/Traefik service + static config (FEAT-023).
- Per-project networks (C2).
- End-to-end verification (TEST-013).

## Proposed Solution / Approach
Follow TEST-010's recommended design (file provider, one file per project)
and FEAT-023's own empirical correction to it (split plain+secure routers,
not one dual-entrypoint router with `tls: {}}`) directly, rather than
re-deriving anything:

- **New `backend/internal/repository/traefik` package**, a drop-in
  replacement for `repository/caddy`'s surface but filesystem-based
  instead of HTTP-admin-API-based: `Client.AddRoute(projectID, domain,
  upstream)` marshals a small Go struct (mirroring
  `traefik/dynamic/tamga.yml`'s shape: `http.routers`/`http.services`) to
  YAML and writes `dynamic/project-<id>.yml`; `Client.RemoveRoute(projectID)`
  does a plain `os.Remove` (idempotent - a missing file is not an error).
  `gopkg.in/yaml.v3` was already resolved in `go.sum` as an indirect
  transitive dependency (never previously imported directly), so using it
  needed no new external dependency to fetch - just promoting it to
  direct via `go mod tidy`.
- **Split-router requirement, applied per-project.** Every `AddRoute` call
  writes *two* routers sharing one service: `project-<id>` (rule
  `Host(`<domain>`)`, `entryPoints: [web]`, no `tls` key) and
  `project-<id>-secure` (same rule, `entryPoints: [websecure]`, `tls: {}`).
  This is the exact pattern FEAT-023 empirically found necessary (a router
  with `tls: {}` doesn't attach to a non-TLS entrypoint even when listed)
  - copying TEST-010's single dual-entrypoint example would have silently
    broken plain HTTP for every project domain. Both routers point at one
  service, and the service is named exactly `project-<id>` (not the
  domain), so Traefik's per-router/service Prometheus metrics stay
  directly attributable to the project (TEST-010 §4).
- **Atomic writes.** `AddRoute` writes to a temp file in the same
  directory (`os.CreateTemp` + write + `os.Rename`), never a direct
  `os.WriteFile` to the destination - Traefik's file-provider watcher
  (fsnotify, `providers.file.watch: true`) could otherwise observe a
  partially-written file mid-write. A same-directory rename is a single
  atomic filesystem operation, so the watcher only ever sees the file
  complete.
- **Wire into `ProjectService`.** `deploy` calls `traefik.AddRoute`
  instead of `caddy.AddRoute` (step 4, same non-fatal `slog.Warn`
  posture). `Delete` calls `traefik.RemoveRoute(project.ID)` instead of
  `caddy.RemoveRoute(project.Domain)` - unconditionally now (no `if
  project.Domain != ""` guard needed), since `RemoveRoute` is keyed by
  project ID and a missing file is already a no-op. `Update` gets new
  logic: when `req.Domain != nil` and the resolved domain actually
  differs from what was in the DB, and the project has a running
  container, it rewrites the route (or removes it if the domain was
  cleared to `""`). Because files are keyed by project ID rather than
  domain, "moving" a route to a new domain collapses to a single
  `AddRoute` overwrite - no separate remove-old-file step is needed except
  when the domain is cleared, which is a nice structural side effect of
  the per-project-ID naming scheme TEST-010 §2 recommended for metrics
  reasons.
- **Remove `repository/caddy` and the admin-API dance entirely.**
  `setupCaddyRoutes`/`reconcileProjectRoutes` existed purely because
  Caddy's admin API replaces its *entire* running config on every
  `/load`, wiping per-project routes that then had to be restored. The
  file provider has no such all-or-nothing reload - each project's file
  is independent. So there's no reconcile-after-wipe to replicate.
  Instead, `main.go` on startup: (1) `EnsureDir()`s the dynamic-config
  directory, then (2) still calls a (renamed, repurposed)
  `reconcileProjectRoutes` that re-writes every `Running` project's route
  file. This isn't restoring anything lost - Traefik's dynamic dir is a
  bind-mounted volume that persists across backend restarts by itself -
  it's a defensive self-heal against drift (a stale port in a previously
  written file, or the dir having been cleared/tampered with outside the
  backend's control), documented as such in the function's own comment.
- **Config/env.** `config.go` drops `CaddyAdminURL`/`CaddyEmail`/
  `CaddyAutoSSL` and adds `TraefikDynamicDir` (env `TRAEFIK_DYNAMIC_DIR`,
  default `/etc/traefik/dynamic` - matching `docker-compose.yml`'s
  existing read-write bind mount into the backend container, already laid
  down by FEAT-023). `UIDomain`/`APIDomain` are also dropped: their only
  reader was `setupCaddyRoutes`, which no longer exists after this task's
  own edits, so keeping them would leave two genuinely dead config fields
  behind rather than a clean removal - not scope creep, a direct
  consequence of deleting their sole caller in this same change.

## Affected Areas
- **New:** `backend/internal/repository/traefik/client.go` - the
  file-provider client (`New`, `AddRoute`, `RemoveRoute`, `EnsureDir`).
- **New:** `backend/internal/tests/repository/traefik_client_test.go` -
  black-box unit tests (split-router shape, valid YAML, service naming,
  domain-change overwrite, idempotent removal, `EnsureDir`).
- **Removed:** `backend/internal/repository/caddy/` (whole package
  deleted).
- `backend/internal/service/project_service.go` - `ProjectService`'s
  `caddy *caddy.Client` field/constructor param replaced with `traefik
  *traefik.Client`; `deploy` step 4, `Delete`, and `Update` all rewired
  (`Update` gains new domain-change-handling logic, previously absent
  entirely).
- `backend/cmd/api/main.go` - `caddyrepo` import/client construction
  replaced with `traefikrepo`; `setupCaddyRoutes` deleted outright (no
  Traefik equivalent needed - static config is FEAT-023's file, not
  generated by the backend); `reconcileProjectRoutes` kept but repurposed
  (writes via `traefik.AddRoute` instead of `caddy.AddRoute`, doc comment
  rewritten to explain it's now a defensive self-heal, not a
  restore-after-wipe).
- `backend/internal/config/config.go` - `CaddyAdminURL`/`CaddyEmail`/
  `CaddyAutoSSL`/`UIDomain`/`APIDomain` fields removed; `TraefikDynamicDir`
  added; the now-unused `getEnvBool` helper removed (its only caller was
  `CaddyAutoSSL`).
- `.env.example` - `CADDY_EMAIL`/`CADDY_ADMIN_URL`/`CADDY_AUTO_SSL`/
  `UI_DOMAIN`/`API_DOMAIN` removed; `TRAEFIK_DYNAMIC_DIR` documented
  (commented out, matches the compose-provided default, same convention
  as `HOST_DATA_DIR`/`SYSTEM_CODE_DIR`).
- `go.mod`/`go.sum` - `gopkg.in/yaml.v3` promoted from an already-resolved
  indirect dependency to a direct one (`go mod tidy` also demoted
  `github.com/google/uuid` to indirect - pre-existing dead direct import,
  unrelated to this task, a side effect of `go mod tidy` reflecting actual
  usage accurately).
- `backend/internal/service/project_service_test.go`,
  `backend/internal/tests/service/project_service_test.go`,
  `backend/internal/tests/handler/project_handler_test.go` - all three
  test-helper `newTestProjectService` functions updated from constructing
  a `caddy.Client` pointed at an unreachable address to a `traefik.Client`
  pointed at a `t.TempDir()`.
- `Makefile` - dropped the now-fully-orphaned `CADDY_EMAIL` default (no
  longer read by `docker-compose.yml`, `.env.example`, or any Go code
  after this task; `traefik/traefik.yml`'s ACME email is hardcoded per
  FEAT-023's own Implementation Notes, not env-substituted).
- `backend/scripts/test-auth.sh`, `test-projects.sh`,
  `test-e2e-critical-path.sh`, `test-providers.sh`, `test-containers.sh` -
  each ran the real backend binary standalone with
  `CADDY_ADMIN_URL="http://127.0.0.1:1"` (an intentionally-unreachable
  address so the old Caddy client's calls were harmless no-ops); replaced
  with `TRAEFIK_DYNAMIC_DIR="${WORKDIR}/traefik-dynamic"` (an isolated
  scratch dir under the script's own temp `WORKDIR`, cleaned up on exit)
  so route-file writes during these scripts can't land in the real
  `/etc/traefik/dynamic` on the host running the script.
- **Deliberately not touched (out of scope for this task):**
  `README.md`'s prose/directory-tree Caddy references and
  `scripts/smoke-test.sh`'s `CADDY_HOST` variable name - both are
  human-facing docs/scripts, not exercised by `go build`/`vet`/`test`, and
  FEAT-023's own Affected Areas already flagged this exact class of
  reference as an expected transient inconsistency; cleaning them up reads
  as a separate doc-cleanup task, not part of "backend Traefik
  file-provider routing." `deploy/Caddyfile` (pre-existing dead/
  unreferenced file per TEST-010 §1) also left alone for the same reason.

## Acceptance Criteria / Definition of Done
- [ ] `repository/traefik` writes/removes a valid per-project dynamic file with router+service named `project-<id>` and the Host rule + loadbalancer upstream
- [ ] deploy writes the route, Delete removes it, and Update moves the route on a domain change (the old caddy gap is fixed)
- [ ] `repository/caddy`, the Caddyfile-loading `setupCaddyRoutes`/`reconcile`, and all CADDY_* config are gone; nothing imports the caddy client
- [ ] On boot the backend ensures the dynamic dir + running projects' route files exist (documented approach)
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass (update/replace any caddy-specific tests); a unit test covers the dynamic-file content generation
- [ ] Config/env updated (dynamic dir path; CADDY_* removed)
- [ ] Code follows KISS/YAGNI

## Test Plan
Unit-test the generated dynamic-file YAML for a sample project. End-to-end
reachability is the C1 integration test (TEST-013). Verify the caddy
removal with a grep (no `repository/caddy` / CADDY_ imports remain).

## Implementation Notes
Implemented directly (complexity: standard), no `opencode` delegation.

**Files changed/added/removed:** see Affected Areas above for the full
list; summary: new `backend/internal/repository/traefik/client.go` +
`backend/internal/tests/repository/traefik_client_test.go`; deleted
`backend/internal/repository/caddy/`; rewired
`backend/internal/service/project_service.go` and
`backend/cmd/api/main.go`; updated `backend/internal/config/config.go`,
`.env.example`, `Makefile`, three existing test-helper files, and five
`backend/scripts/test-*.sh` scripts.

**Traefik client shape.** `Client{dynamicDir string}`, constructed via
`traefik.New(dynamicDir)`. `AddRoute(projectID int64, domain, upstream
string) error` and `RemoveRoute(projectID int64) error` are the only two
routing methods (mirrors `caddy.Client`'s two-method surface exactly,
different signatures - `AddRoute` takes `projectID` now, not just
`domain`, since the file/router/service naming is ID-based).
`EnsureDir() error` is new (no Caddy equivalent needed one, since Caddy
never wrote to a shared directory) and is called both from
`main.go` on boot and defensively inside `writeFile` before every write.

**Split-router requirement.** `AddRoute` builds one `dynamicConfig` Go
struct with two entries in `http.routers` (`project-<id>` on `web`, no
`tls` field; `project-<id>-secure` on `websecure`, `TLS: &struct{}{}`
which marshals to `tls: {}`) and one entry in `http.services`
(`project-<id>`, `loadBalancer.servers: [{url: "http://<upstream>"}]`),
matching `traefik/dynamic/tamga.yml`'s pattern exactly. Verified this
produces the intended YAML via the new unit test
(`TestTraefikClientAddRouteWritesSplitRouters`), which round-trips the
generated file through `yaml.Unmarshal` and asserts both router names,
both `entryPoints` values, the `tls` presence/absence split, the shared
service name, and the literal `Host()` rule text.

**Atomic write.** `writeFile` does `EnsureDir` -> `os.CreateTemp(dynamicDir,
"."+name+".*.tmp")` -> write bytes -> `Close` -> `os.Chmod(0644)` ->
`os.Rename(tmpPath, destPath)`, with a `defer os.Remove(tmpPath)` as a
best-effort cleanup on any early-return error path (a no-op once the
rename has already moved the file away). The temp file is created in the
same directory as the destination specifically so the rename is
same-filesystem (no cross-device copy, which `os.Rename` can't do
atomically anyway) - this was a deliberate choice, not an accident of
using `os.CreateTemp`'s default `$TMPDIR` behavior.

**Boot-reconcile approach.** Chose "re-write every running project's
route file on boot" (the task's own stated preference) over "trust the
files already on disk survive." Reasoning, beyond just following the
suggestion: the dynamic dir is a bind-mounted volume
(`docker-compose.yml`, `./traefik/dynamic` <-> `/etc/traefik/dynamic`)
that *would* survive backend restarts unmodified on its own, so in the
common case this rewrite is a no-op (same content gets regenerated).
Where it earns its keep: if a container's exposed port ever changed
between when its route file was last written and a later restart (a
plausible drift path - e.g. a manual `docker` intervention, or a future
image change that exposes a different port), boot-time re-derivation from
the *live* container via `GetContainerPort` self-heals it, exactly
mirroring what the old `reconcileProjectRoutes` did for Caddy, just for a
different reason (defensive self-heal vs. mandatory restore-after-wipe).
Renamed the function's doc comment to make this distinction explicit so a
future reader doesn't assume Traefik has the same wipe-on-reload behavior
Caddy did.

**`Update`'s domain-change fix.** Confirmed the gap was real before
fixing it: read the pre-change `Update` body and verified zero
`s.caddy.*`/`s.traefik.*` calls existed anywhere in it (matching
TEST-010's finding). The fix captures `oldDomain := project.Domain`
before applying `req` fields, then after the DB write, if
`req.Domain != nil && project.Domain != oldDomain && project.ContainerID
!= "" && s.docker != nil`: either `RemoveRoute` (if the new domain is
`""`) or re-derive the upstream via `GetContainerPort` and call
`AddRoute` with the new domain (any other case - project not yet
deployed, or domain unchanged - does nothing, matching `deploy`'s own
non-fatal-warn-only posture on failure).

**Validation performed:**
- `go build ./...` - clean.
- `go vet ./...` - clean.
- `go test ./...` - all packages pass, including the new
  `internal/tests/repository` package (4 new tests) and the three updated
  test-helper files (`project_service_test.go` x2,
  `project_handler_test.go`), none of which needed logic changes beyond
  swapping the Caddy client construction for a Traefik one pointed at a
  `t.TempDir()`.
- `gofmt -l` clean on every file touched.
- Grepped the whole repo (excluding `tasks/` history and this session's
  own `.claude/settings.local.json` tool-permission log) for
  `repository/caddy` and `CADDY_`: the only `repository/caddy` hits left
  are two comparative comments inside the new `traefik/client.go` itself
  ("this replaces repository/caddy's admin-API client..."); the only
  `CADDY_` hits left are in `README.md` prose, `scripts/smoke-test.sh`'s
  `CADDY_HOST` variable name, `deploy/Caddyfile` (pre-existing dead file),
  and historical `sprints/`/`tasks/` records - all deliberately out of
  scope per Affected Areas above, none reachable from
  `go build`/`vet`/`test` or from `docker-compose.yml`/`config.go`.
- Did not rebuild/restart the live compose stack, per the task's own
  instruction (TEST-013 owns end-to-end verification of the full C1
  cluster).

## Review Notes
<filled in by reviewer>

### 2026-07-10 — sdlc-reviewer

**Verdict: PASS**

Scope check: `git diff`/`git status` confirms the touched files match the
task's own Affected Areas list exactly (new `repository/traefik/client.go`
+ `tests/repository/traefik_client_test.go`; deleted `repository/caddy/`;
`project_service.go`, `cmd/api/main.go`, `config.go`, `.env.example`,
`go.mod`/`go.sum`, `Makefile`, three test-helper files, five
`backend/scripts/test-*.sh`). The other uncommitted top-level churn in the
working tree (frontend files, docker-compose.yml, README/plan.md deletions,
etc.) is ambient WIP from other in-flight tasks in this SDLC session, not
this task's doing — none of it overlaps FEAT-024's stated file list.

1. **traefik/client.go** — `AddRoute` builds the split-router pair exactly
   as specified: `project-<id>` on `web` with no `tls` key, and
   `project-<id>-secure` on `websecure` with `TLS: &struct{}{}` (marshals
   to `tls: {}`), both pointing at one service literally named
   `project-<id>` (not the domain) — confirmed via
   `TestTraefikClientAddRouteWritesSplitRouters`, which round-trips the
   generated YAML and asserts entryPoints, tls presence/absence, service
   name, and the literal `Host()` rule text. `writeFile` is atomic:
   `os.CreateTemp(c.dynamicDir, ...)` (same dir as the destination, so same
   filesystem) → write → close → chmod → `os.Rename`, with a best-effort
   `defer os.Remove(tmpPath)` cleanup that's a no-op once renamed.
   `RemoveRoute` ignores `os.IsNotExist`, so double-remove and
   remove-never-existed are both no-ops — confirmed by
   `TestTraefikClientRemoveRoute`.

2. **project_service.go wiring** — `deploy` derives `upstream` from
   `s.docker.GetContainerPort(ctx, containerID)` (falling back to `"80"` on
   error) before calling `s.traefik.AddRoute`, not a hardcoded port; `Delete`
   unconditionally calls `s.traefik.RemoveRoute(project.ID)` (dropped the
   old `if project.Domain != ""` guard, correctly, since RemoveRoute is now
   ID-keyed and idempotent either way).

   **Update's domain-change fix — verified correct.** `oldDomain :=
   project.Domain` is captured before the request fields are applied. After
   the DB write, if `req.Domain != nil && project.Domain != oldDomain &&
   project.ContainerID != "" && s.docker != nil`: a cleared domain
   (`project.Domain == ""`) calls `RemoveRoute`; any other new domain
   re-derives the upstream via `GetContainerPort` and calls `AddRoute`.
   Because the route file is keyed by `project-<id>.yml`, not by domain, a
   same-project `AddRoute` overwrite is a full destination-file replace
   (temp file + rename over the existing `project-<id>.yml`) — there is no
   old-domain remnant possible, since there's no second file to leave
   behind. `TestTraefikClientAddRouteOverwritesOnDomainChange` confirms
   this directly at the client level (old domain string absent from the
   rewritten file, new domain present, exactly one file in the dir after
   both calls). The dev's "collapses to an overwrite" claim holds.

3. **Boot reconcile** — `main.go`'s `reconcileProjectRoutes` iterates
   `ps.List(ctx)`, skips anything that isn't `Status == Running`,
   `ContainerID != ""`, and `Domain != ""` (so a domain-less project can't
   produce a broken `Host()` rule at boot), derives the port fresh from
   `dc.GetContainerPort` per project, and warns-not-fails per project on
   `AddRoute` error. Returns early if `ps == nil || dc == nil`, so a
   docker-unavailable boot doesn't crash. No `setupCaddyRoutes` survives
   (grep confirms).

4. **Caddy removal** — grep across `.go` files: the only remaining
   `repository/caddy` string hits are inside `traefik/client.go`'s own
   doc-comment (comparative prose, not an import), and the only `CADDY_`
   hits in Go are zero. `repository/caddy/` directory is deleted from the
   tree. `config.go` has `TraefikDynamicDir` (default
   `/etc/traefik/dynamic`, env `TRAEFIK_DYNAMIC_DIR`) and no `Caddy*`/
   `UIDomain`/`APIDomain` fields; `getEnvBool` is gone with its only caller.
   `docker-compose.yml` confirms the backend's mount is read-write
   (`./traefik/dynamic:/etc/traefik/dynamic`, no `:ro`) matching the
   client's write access; the `traefik` service's own mount of the same
   host path is `:ro`, correctly asymmetric. Leftover `CADDY_` refs
   (`README.md`, `scripts/smoke-test.sh`'s `CADDY_HOST` var,
   `deploy/Caddyfile`) are docs/dead-script surface, not reachable from
   `go build`/`vet`/`test` or `config.go` — judged acceptable per the
   task's own out-of-scope note, consistent with FEAT-023's precedent of
   flagging this class of leftover as expected transient inconsistency.

5. **Tests** — `backend/internal/tests/repository/traefik_client_test.go`
   has four tests covering exactly what the task asked for: split-router
   shape + tls presence/absence + shared service name + valid YAML
   round-trip (`TestTraefikClientAddRouteWritesSplitRouters`), domain-change
   overwrite with no stale file left behind
   (`TestTraefikClientAddRouteOverwritesOnDomainChange`), idempotent
   removal (`TestTraefikClientRemoveRoute`), and `EnsureDir`
   (`TestTraefikClientEnsureDir`). These are meaningful, not tautological —
   they assert on the actual on-disk YAML content via `yaml.Unmarshal`, not
   just "no error returned."

6. **Build/vet/test** — reran independently: `go build ./...` clean,
   `go vet ./...` clean, `go test ./...` all pass (12 packages, including
   the new `internal/tests/repository` package). No dangling caddy test —
   `repository/caddy` package and its test file are both gone; the three
   test-helper files (`project_service_test.go` x2,
   `project_handler_test.go`) now construct `traefik.New(t.TempDir())`
   instead of `caddy.New("http://127.0.0.1:1")`, confirmed via `git diff`.

**One pre-existing, non-blocking observation (not introduced by this
task):** `deploy`'s step 4 calls `s.traefik.AddRoute(project.ID,
project.Domain, upstream)` unconditionally, even when `project.Domain ==
""` — this writes a `project-<id>.yml` with a literal `Host()`` `` rule
(empty host), which is invalid Traefik rule syntax. Checked git history:
the deleted `caddy.Client.AddRoute` call this replaced had the exact same
absence of an empty-domain guard, so this is a faithful mirror of
pre-existing behavior, not a regression this task introduced, and it's
consistent with the task's explicit charter ("mirrors the caddy client's
surface"). Worth a follow-up ticket if empty-domain deploys are a real
usage path, but out of scope here — flagging for visibility only, not
blocking.

Acceptance criteria walked item by item against the code: all met.

## Test Notes
<filled in by tester>

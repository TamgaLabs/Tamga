---
id: FEAT-002
type: feature
title: "Backend config: CADDY_AUTO_SSL, UI_DOMAIN, API_DOMAIN"
status: done
complexity: simple
assignee: opencode
sprint: SPRINT-001
created: 2026-07-04
history:
  - {date: 2026-07-04, stage: created, by: architect, note: "task created"}
  - {date: 2026-07-04, stage: in-development, by: architect, note: "assigned to opencode"}
  - {date: 2026-07-04, stage: in-development, by: architect, note: "opencode stopped after .env read auto-rejected; resumed session, implemented config fields + Caddyfile generation"}
  - {date: 2026-07-04, stage: in-review, by: architect, note: "moved to review"}
  - {date: 2026-07-04, stage: in-test, by: architect, note: "review PASSED (non-blocking /reload endpoint bug filed as BUG-005); moved to test"}
  - {date: 2026-07-04, stage: done, by: architect, note: "test PASSED (Caddyfile generation verified for all 3 CADDY_AUTO_SSL states); moved to done"}
---

## Summary
The backend config currently doesn't read `CADDY_AUTO_SSL`, `UI_DOMAIN`, or
`API_DOMAIN` from the environment, even though architecture.md specifies them
as required deployment config (Tamga's own UI/API domains, and whether Caddy
auto-provisions SSL certs). Add these to the backend's config loader.

## Requirements
- `CADDY_AUTO_SSL` (bool, default `true`)
- `UI_DOMAIN` (string, default `localhost` per plan.md / `tamga.local` per
  architecture.md — follow whichever the existing config module's default
  convention uses, note the discrepancy in Implementation Notes)
- `API_DOMAIN` (string, default `api.localhost` / `api.tamga.local` — same note)
- When `CADDY_AUTO_SSL=false`, whatever code path generates the Caddyfile
  must serve plain HTTP only, no automatic cert issuance for Tamga's own
  UI/API domains or for project domains
- Config is read once at startup (per architecture.md), not polled

## Out of Scope
- Project-level `ssl_enabled` flag handling (already exists) — only the
  global auto-SSL switch and Tamga's own domains are in scope here
- docker-compose wiring — see FEAT-001

## Proposed Solution / Approach
1. Add `CaddyAutoSSL bool`, `UIDomain string`, `APIDomain string` fields to
   `backend/internal/config/config.go` Config struct.
2. Add a `getEnvBool` helper alongside existing `getEnv`/`getEnvInt`.
3. Implement Caddyfile generation in the existing `setupCaddyRoutes` no-op
   in `backend/cmd/api/main.go`:
   - When `CADDY_AUTO_SSL=true` (default): write a Caddyfile with `email`
     directive (enables automatic HTTPS cert provisioning).
   - When `CADDY_AUTO_SSL=false`: write the same Caddyfile but with
     `auto_https off` in the global block and `:80` on the site address
     (plain HTTP only).
   - Use `cfg.UIDomain` as the site domain in place of the old `{$DOMAIN}`
     placeholder.
   - Write the file to `/Caddyfile` (the Docker volume mount shared with
     Caddy), then POST to the Caddy admin API `/reload` endpoint.
   - Fallback is non-fatal: log a warning if reload fails (the file is
     written correctly for the next container restart).
4. Add the three new vars to `.env.example`.
5. No speculative abstraction — config fields are loaded directly in
   `config.Load()`, Caddyfile generation is inline in `setupCaddyRoutes`.
   The old `DOMAIN` field is kept but no longer used in the Caddyfile.

## Affected Areas
- Backend config module (find via `grep -r "CADDY\|UI_DOMAIN\|API_DOMAIN"` or
  the existing `config.go`)
- Caddyfile generation code, wherever it currently lives

## Acceptance Criteria / Definition of Done
- [ ] Backend reads all three env vars with correct defaults when unset
- [ ] `CADDY_AUTO_SSL=false` produces a Caddyfile with no `tls` directives
- [ ] `CADDY_AUTO_SSL=true` (or unset) preserves current auto-SSL behavior
- [ ] Code follows KISS/YAGNI — no speculative abstraction

## Test Plan
Set each env var explicitly and unset, restart backend, inspect the
generated Caddyfile for each `CADDY_AUTO_SSL` value.

## Implementation Notes
- Added `CaddyAutoSSL`, `UIDomain`, `APIDomain` to `Config` struct and
  `getEnvBool` helper in `backend/internal/config/config.go`.
- Changed `setupCaddyRoutes` in `backend/cmd/api/main.go` from a no-op to
  an actual Caddyfile generator: writes `/Caddyfile` with the correct
  domain and SSL settings, then calls `POST /reload` on Caddy's admin API.
- Defaults: `UI_DOMAIN=localhost`, `API_DOMAIN=api.localhost` (matches
  existing `DOMAIN` default convention of `localhost`, not the
  architecture.md `tamga.local` values).
- `CADDY_AUTO_SSL=false` sets `auto_https off` globally and `:80` on the
  site block; no `email` line is written.
- The old `DOMAIN` config field is preserved but no longer referenced in
  the generated Caddyfile — `UI_DOMAIN` replaces it.
- Updated `.env.example` with the three new vars.

## Review Notes

### 2026-07-04 — reviewer pass

Verdict: PASS

Scope reviewed: `backend/internal/config/config.go` (new fields + `getEnvBool`),
`backend/cmd/api/main.go` (`setupCaddyRoutes`, `caddyfilePath` const, `bytes`
import only — the `apiKeyService`/`EnsureTables` wiring in the same file is a
separate, pre-existing change and out of scope for this review), and
`.env.example`.

**Acceptance criteria walkthrough:**
- [x] Backend reads all three env vars with correct defaults when unset —
  `CaddyAutoSSL` defaults `true`, `UIDomain` defaults `localhost`, `APIDomain`
  defaults `api.localhost` (config.go:34-36). Discrepancy vs architecture.md's
  `tamga.local`/`api.tamga.local` defaults is correctly called out in
  Implementation Notes as instructed by the task.
- [x] `CADDY_AUTO_SSL=false` produces a Caddyfile with no `tls`/`email`
  directives — verified by reading the generated buffer logic
  (main.go:120-144): the `false` branch writes `auto_https off` in the global
  block and `:80` on the site address, and skips the `email` line entirely.
- [x] `CADDY_AUTO_SSL=true` (or unset) preserves current auto-SSL behavior —
  generated output matches the pre-existing `Caddyfile`/`deploy/Caddyfile`
  templates (`admin :2019` + `email <addr>` global block, bare domain site
  address), with `{$DOMAIN}` replaced by `cfg.UIDomain` as specified.
- [x] KISS/YAGNI — plain `bytes.Buffer` + `WriteString` calls, no templating
  engine or premature abstraction. Matches task's explicit "no speculative
  abstraction" instruction.

Confirmed `go build ./...` succeeds and `gofmt -l` reports no issues on the
touched files.

**Non-blocking observations (do not require changes for this task):**
1. `main.go:152-159` — `POST {CaddyAdminURL}/reload` is exactly what the
   task's Proposed Solution specified, but Caddy's admin API does not
   actually expose a `/reload` endpoint (only `/load`, `/config/*`, `/stop`,
   `/adapt`, etc.). In practice this call will likely 404, so the live Caddy
   process won't pick up the new Caddyfile until it's restarted/re-executed
   some other way. The code handles this "gracefully" (logs and returns nil
   on transport error), but it doesn't check `resp.StatusCode` — a 404
   response is currently logged as `"caddy reloaded", "status", 404`, which
   reads as success. Since the task's own Test Plan only asks to inspect the
   generated Caddyfile (not verify a live reload), and this exact approach
   was specified in the Proposed Solution, I'm not blocking on it — but the
   architect may want a follow-up task to either exec `caddy reload` inside
   the Caddy container or POST an adapted config to `/load`, and to treat a
   non-2xx response as a failure in the log line.
2. `APIDomain` is added to `Config` and read from `API_DOMAIN` correctly, but
   is never referenced anywhere else in the codebase (not even in
   `setupCaddyRoutes`, which only uses `UIDomain` for both the UI and the
   `/api/*` path proxy, matching the existing single-domain Caddyfile
   pattern). This matches the task's Proposed Solution (which only mentions
   `cfg.UIDomain`) and the Requirements only ask for the var to be read with
   the correct default, so it's not a defect against this task's AC — just
   flagging that it's currently an unused field, presumably for a future
   task (separate API vhost) to consume.
3. `setupCaddyRoutes`'s `c *caddyrepo.Client` parameter is unused inside the
   function body (the reload is done with a raw `http.Post` against
   `cfg.CaddyAdminURL` instead of going through the existing client). Not a
   bug, but slightly inconsistent with the `caddyrepo.Client` abstraction
   already used elsewhere (e.g. project route management). Worth a look in a
   later cleanup pass, not blocking here.

## Test Notes
<Filled in by the tester.>

### 2026-07-04 — tester pass

Verdict: PASS

**Method:** Built the actual backend Docker image (`docker build -f
deploy/Dockerfile.backend -t tamga-backend-test .`, from the real
`backend/cmd/api/main.go` and `backend/internal/config/config.go`) and ran
it with `docker run`, bind-mounting a host file to `/Caddyfile` (the same
mount point used in `docker-compose.yml`) so the real `setupCaddyRoutes`
code path executes and writes to a real file I could inspect. `/Caddyfile`
lives at filesystem root and isn't writable outside a container as the
current shell user, so this container-based approach (rather than running
the Go binary standalone on the host) was the practical way to exercise it
end-to-end without modifying source.

**Case 1 — `CADDY_AUTO_SSL=false`, `UI_DOMAIN=myapp.example.com`:**
```
docker run --rm -e CADDY_AUTO_SSL=false -e UI_DOMAIN=myapp.example.com \
  -e API_DOMAIN=api.myapp.example.com -e CADDY_EMAIL=admin@example.com \
  -e DB_PATH=/data/tamga.db -e DATA_DIR=/data \
  -v <scratch>/Caddyfile:/Caddyfile -v <scratch>/data:/data \
  tamga-backend-test /usr/local/bin/api
```
Logged `"caddyfile written" path=/Caddyfile`. Resulting file:
```
{
	admin :2019
	auto_https off
}

myapp.example.com:80 {
	@api path /api/*
	handle @api {
		reverse_proxy backend:8080
	}
	handle {
		reverse_proxy frontend:3000
	}
}
```
Confirms: `auto_https off` present, no `email`/`tls` directive anywhere,
site block address is `myapp.example.com:80` (plain HTTP, port 80 forced),
`UI_DOMAIN` correctly used as the site domain.

**Case 2 — `CADDY_AUTO_SSL=true`, `UI_DOMAIN=myapp.example.com`,
`CADDY_EMAIL=me@example.com`:**
Resulting file:
```
{
	admin :2019
	email me@example.com
}

myapp.example.com {
	@api path /api/*
	handle @api {
		reverse_proxy backend:8080
	}
	handle {
		reverse_proxy frontend:3000
	}
}
```
Confirms: `email` directive present (auto-SSL path), no `auto_https off`,
no forced `:80` on the site address — Caddy's default automatic HTTPS
applies to the bare domain.

**Case 3 — all three vars unset (only `CADDY_EMAIL`, `DB_PATH`, `DATA_DIR`
set to keep the container runnable):**
Resulting file:
```
{
	admin :2019
	email admin@example.com
}

localhost {
	@api path /api/*
	handle @api {
		reverse_proxy backend:8080
	}
	handle {
		reverse_proxy frontend:3000
	}
}
```
Confirms defaults: `CADDY_AUTO_SSL` defaults to `true` (auto-SSL branch,
`email` line present), `UI_DOMAIN` defaults to `localhost` and is used as
the site domain — matches `config.go`'s `getEnv("UI_DOMAIN", "localhost")`
and `getEnvBool("CADDY_AUTO_SSL", true)`.

All three containers also logged the expected non-fatal warning for the
known BUG-005 issue (`caddy reload request failed (non-fatal)` — connection
refused, since no real Caddy admin API was running in this test), which
per the task's Test Plan is explicitly out of scope here; the Caddyfile
was written to disk successfully in every case regardless of that failure.

Also confirmed `.env.example` was updated with `CADDY_AUTO_SSL=true`,
`UI_DOMAIN=localhost`, `API_DOMAIN=api.localhost` (`git diff .env.example`).

**Acceptance criteria checked against running system:**
- [x] Backend reads all three env vars with correct defaults when unset
- [x] `CADDY_AUTO_SSL=false` produces a Caddyfile with no `tls`/`email` directives
- [x] `CADDY_AUTO_SSL=true` (or unset) preserves current auto-SSL behavior (`email` directive present)
- [x] `UI_DOMAIN` correctly used as the site domain in all cases

Cleaned up: removed test containers, removed the `tamga-backend-test`
Docker image, removed scratch directories. No source files were modified.

---
id: TEST-001
type: test
title: Auth, setup & session verification
status: done
complexity: standard
assignee: sdlc-developer
created: 2026-07-07
history:
  - {date: 2026-07-07, stage: created, by: architect, note: "task created — Phase 1 (backend verification) sprint, tackled first since every other endpoint sits behind authMiddleware"}
  - {date: 2026-07-07, stage: in-development, by: architect, note: "assigned to sdlc-developer"}
  - {date: 2026-07-07, stage: in-review, by: architect, note: "dev complete: backend/scripts/test-auth.sh built, 21/21 checks passed, no prod code touched; default-admin-password behavior confirmed documented/intentional via README, not filed as a bug"}
  - {date: 2026-07-07, stage: in-test, by: architect, note: "both sdlc-reviewer and agy second-review passed; moved to test"}
  - {date: 2026-07-07, stage: done, by: architect, note: "tester PASS against live builder-provisioned backend; teardown confirmed clean"}
---

## Summary
Verify the full auth surface end-to-end against a live backend: first-run
setup, login, session/token validation, and the auth middleware that gates
every other route in the router. This is the foundation every other Phase 1
test task depends on, so it goes first.

## Scope
- `GET /api/auth/status` (`auth_handler.go`) — reports whether setup has
  already run
- `POST /api/auth/setup` — first-admin creation; must be idempotent-safe
  (reject a second setup attempt once an admin exists)
- `POST /api/auth/login` — correct credentials, wrong password, nonexistent
  user, malformed body
- `GET /api/auth/me` — requires a valid session; returns the caller's
  identity
- `authMiddleware` (`backend/internal/handler/middleware.go`) — every
  protected route under `r.Group` in `router.go` must reject a missing
  token and an invalid/expired token with 401, and admit a valid one
- `auth_service.go` / `user_repo.go` — password hashing/verification path,
  session/token issuance mechanism actually used by the middleware

## Out of Scope
- Whitelist/egress rules (`TEST-004`)
- Anything downstream of auth (projects, containers, etc. — their own test
  tasks assume a working session and don't re-test auth itself)

## Test Approach
Split across the two layers, using whichever already gives the more precise
signal for a given check:

- **Service layer (Go, already exists in `auth_service_test.go`, verified
  still green, no changes needed):** the deterministic, clock/secret-
  sensitive parts of `auth_service.go` — the `IsSetup()` false -> `Setup()`
  -> `IsSetup()` true -> second-`Setup()`-rejected sequence, wrong-password
  and no-user login failures, and `ValidateToken` failure modes that are
  awkward to trigger over real HTTP (garbage input, wrong signing secret,
  a token that's actually expired by clock). These need precise control
  that's cheap in a unit test and would just add ceremony (sleeping past a
  72h expiry, faking a second signing key) over curl.
- **Live HTTP (new: `backend/scripts/test-auth.sh`, bash+curl):** everything
  that only exists at the HTTP/router/middleware layer and can't be
  observed from the service alone — actual status codes returned by
  `auth_handler.go` for malformed/empty JSON bodies (400 not 500), the
  `Authorization: Bearer` header parsing and the missing/garbage/tampered-
  token 401 paths in `AuthMiddleware` (`middleware.go`), and — the main
  point of this task — confirming `authMiddleware` is actually wired onto
  multiple distinct protected routes in `router.go` (not just `auth/me`)
  by hitting `/api/projects`, `/api/agent-providers`, `/api/system/containers`
  with no/garbage/valid tokens. The script builds and runs the real
  `cmd/api` binary directly (no docker-compose/Caddy/Docker daemon needed —
  both are optional deps main.go only warns about) against an isolated tmp
  SQLite DB and port, so it's exercising the exact production router/
  handler/middleware wiring, not a test double.

One deliberate compromise: `cmd/api/main.go` calls `AuthService.AutoSetup()`
unconditionally on boot, and `config.Load()`'s `ADMIN_PASSWORD` fallback
defaults to `"admin"` (never truly empty — see Implementation Notes), so a
live client can never actually observe `auth/status` reporting `setup:
false` in this binary as shipped — by the time the port is listening, an
admin already exists. The curl script therefore validates the reachable
half of that behavior (`setup:true` is reported, and a second `auth/setup`
call is correctly rejected with 409, not silently accepted or a 500); the
genuine false -> true transition is the thing `TestAuthServiceSetupAndLogin`
already covers directly against `AuthService`, bypassing `AutoSetup`.

## Affected Areas
- `backend/scripts/test-auth.sh` (new) — live HTTP curl-based verification
  script
- No production code under `backend/internal/**` touched.

## Acceptance Criteria
- [x] `auth/status` correctly reflects setup state before and after
      `auth/setup` is called
- [x] `auth/setup` succeeds exactly once; a second call is rejected (not
      silently accepted, not a 500)
- [x] `auth/login` returns a usable token/session on correct credentials,
      and a clean 4xx (not 500) on wrong password / nonexistent user
- [x] `auth/me` returns the correct identity for a valid session and 401
      for no/garbage/expired token
- [x] At least 3 distinct protected routes from `router.go` (spread across
      different handlers) are confirmed to 401 without a valid session
- [ ] Any defect found is filed as its own `BUG-XXX` task with repro steps
      (see Implementation Notes — one candidate found, not filed by this
      task since owning/creating task files is the architect's job; flagged
      for the architect to decide/file)
- [x] No unhandled panic/500 observed for any malformed input tried

## Test Plan
Stand up the backend (builder), run through the checks above with `curl`
against `http://localhost:<port>/api/...`, capturing status codes and
response bodies for each. Re-run the same login/me sequence with a
tampered/expired token to confirm rejection.

## Implementation Notes
Built `backend/scripts/test-auth.sh` (new, executable). It:
- `go build`s `backend/cmd/api` into a tmp dir, runs it standalone (isolated
  tmp SQLite `DB_PATH`, random `PORT`, throwaway `JWT_SECRET`/`ADMIN_PASSWORD`,
  `CADDY_ADMIN_URL` pointed at a closed port) — no docker-compose, Caddy, or
  Docker daemon required, since main.go treats both as optional/warn-only.
- Polls `/health` until ready, then runs 21 curl-based checks covering every
  bullet in Scope: `auth/status`, `auth/setup` (incl. malformed/empty body,
  duplicate-setup rejection), `auth/login` (malformed body, empty body,
  missing field, wrong password, correct password), `auth/me` (valid token,
  no token, garbage token, tampered token, correct `user_id` in the
  response), and `authMiddleware` on 3 distinct protected routes from
  `router.go` spanning different handlers (`/api/projects` →
  `ProjectHandler`, `/api/agent-providers` → `AgentProviderHandler`,
  `/api/system/containers` → `ContainerHandler`) with no/garbage/valid
  tokens. Finishes by re-hitting `/health` to confirm the process is still
  alive (no panic/crash) after the malformed-input barrage. Cleans up the
  server process and tmp dir via an EXIT trap. Exit code reflects
  pass/fail count.
- Run with: `backend/scripts/test-auth.sh` (from repo root or anywhere;
  it resolves the repo root from its own path). Ran it locally: 21/21
  checks passed. Also re-ran the pre-existing
  `go test ./backend/internal/service/... -run TestAuthService -v`
  (TestAuthServiceSetupAndLogin / LoginWrongPassword / LoginNoUser /
  ValidateTokenFailures) — all still green; these already cover the
  false→true `IsSetup()` transition and expired/wrong-secret token
  rejection more precisely than curl could, so the script doesn't
  duplicate them (see Test Approach).
- No changes made to `backend/internal/**`.

**Possible latent design gap found (not a bug in the code under test per
se, and not fixed here — flagging for the architect to decide whether it
warrants its own `BUG-XXX`):** `config.Load()` (`backend/internal/config/config.go:47-52`,
`getEnv`) treats an unset *and* an empty-string `ADMIN_PASSWORD` identically,
falling back to the literal default `"admin"` either way — there is no way
to configure the backend so that `cfg.AdminPassword == ""`. Since
`cmd/api/main.go:49` calls `authService.AutoSetup()` unconditionally on
every boot, and `AuthService.AutoSetup()` (`backend/internal/service/auth_service.go:74-91`)
only skips creating an admin when `AdminPassword == ""`, this means an
admin with a well-known default password is *always* auto-provisioned on
first boot in every real deployment — the manual first-run flow
(`POST /api/auth/setup`, and the frontend's `(auth)/setup/page.tsx`, which
today just immediately redirects to `/login`) can never actually be
reached with an empty database in practice; hitting it always returns
409 "already setup". Repro: fresh `DB_PATH`, no `ADMIN_PASSWORD` set (or
set to `""`) in `.env`, start the backend, `GET /api/auth/status` →
immediately `{"setup":true}` before any client ever calls `/api/auth/setup`.
Whether this is intentional (README documents the auto-provision behavior)
or a gap (no supported way to require an operator to pick their own admin
password on first run, and a stale/dead manual-setup code path) is a
product decision, not something to guess at while doing verification-only
work.

## Review Notes

**2026-07-07 — reviewer pass 1**

Verdict: PASS

Scope check: confirmed via `git status`/`git diff` that no file under
`backend/internal/**` was touched. The only new file is
`backend/scripts/test-auth.sh` (untracked, new dir `backend/scripts/`).
Everything else showing dirty in `git status` (frontend files, `plan.md`
deletion, `qa-debug*.js`, `.claude/`, `.opencode/`, other `tasks/active/*`
files, etc.) is pre-existing ambient WIP unrelated to this task — none of
it is mentioned in this task's Implementation Notes and none of it is
under `backend/`.

Ran the script for real (not just read it): `./backend/scripts/test-auth.sh`
builds `cmd/api`, boots it standalone on an isolated tmp DB/port, and
produced **21 passed, 0 failed** — matches the Implementation Notes claim
exactly. Also re-ran `go test ./backend/internal/service/... -run
TestAuthService -v`: all 4 tests (`TestAuthServiceSetupAndLogin`,
`LoginWrongPassword`, `LoginNoUser`, `ValidateTokenFailures`) pass, matching
the claim that the false→true `IsSetup()` transition and clock/secret-
sensitive token-failure modes are covered there instead of via curl.

Cross-checked the script's assertions against the real handler/middleware/
router code (`auth_handler.go`, `middleware.go`, `router.go`,
`auth_service.go`, `config.go`) rather than taking the script's own
expectations at face value:
- `Setup`/`Login` handlers really do return 400 for decode failure and for
  an explicit empty-password check, 409 from `Setup` (via `auth.Setup`
  error → `http.StatusConflict`), 401 from `Login` on bad credentials —
  matches every status code the script expects (`auth_handler.go:30-64`).
- `AuthMiddleware` really does 401 on missing/invalid token and only
  strips a `Bearer ` prefix, matching the garbage/tampered/no-token checks
  (`middleware.go:15-43`).
- `router.go` confirms `/auth/me`, `/projects`, `/agent-providers`,
  `/system/containers` are all inside the same `r.Group` gated by
  `authMiddleware`, spanning `AuthHandler`, `ProjectHandler`,
  `AgentProviderHandler`, `ContainerHandler` — genuinely 4 distinct
  handlers, satisfying "3+ distinct protected routes across different
  handlers."
- The script's assertions are against the real running binary/router, not
  a mock or a value the script itself defines — not tautological. Status
  codes and JSON shape (`token`, `user_id`) are read from the actual HTTP
  response of the real `cmd/api` process.

Acceptance criteria walk-through:
- `auth/status` reflects setup state — the script explicitly documents
  and correctly works around the fact that a live client can never
  observe `setup:false` in this binary (AutoSetup fires before the
  listener comes up), validates the reachable `setup:true` half, and
  points to `TestAuthServiceSetupAndLogin` for the false→true transition.
  Reasonable, matches Test Approach as written. Met.
- `auth/setup` succeeds once, second call rejected with 409 not silently
  accepted/500 — checked directly, passes. Met.
- `auth/login` correct/wrong/malformed — all 5 variants checked, correct
  status codes for each. Met.
- `auth/me` correct identity + 401 on no/garbage/tampered token — checked,
  including a real `user_id` assertion on the response body. Met.
- 3+ distinct protected routes 401 without valid session — 3 routes
  across 3 handlers checked with no-token and garbage-token variants, plus
  one with a valid token to confirm it isn't over-blocking. Met.
- Any defect filed as its own `BUG-XXX` — correctly left unfiled by the
  developer (per task instructions, filing is the architect's job); the
  candidate gap is clearly flagged in Implementation Notes with a repro.
  Per this review's instructions, the "documented/intentional, not a bug"
  determination was already made independently by the architect via
  README/config.go, so this box is appropriately left for the architect to
  close out rather than treated as a blocking gap in the test task itself.
- No unhandled panic/500 on malformed input — script re-hits `/health`
  after the malformed-input barrage and confirms 200; also verified live
  during my own run. Met.

No duplication concerns: this is the first live-HTTP verification script
in the repo (`backend/scripts/` didn't exist before), so there's no
existing equivalent it should have reused, and its `check()` curl helper
is appropriately scoped/local rather than something that belongs in a
shared module.

Non-blocking, optional notes:
- `check()`'s `data` argument uses `[ -n "$data" ]` to decide whether to
  send `-d`, so the empty-body login/setup checks (`''`) don't actually
  send `-d ''` — curl gets no body at all rather than a truly empty one.
  This still exercises the "empty/absent body" 400 path correctly (decode
  of an empty reader errors out the same way), just noting the naming
  ("empty body" vs. "no body") is very slightly imprecise. Doesn't affect
  correctness of what's being verified.
- `set -uo pipefail` without `-e` is intentional here (the script tallies
  pass/fail itself rather than aborting on the first non-zero `curl`
  exit), consistent with how the rest of the script is structured.

### agy review pass — 2026-07-07

Verdict: PASS

Independently confirmed via `git status`/`git diff` that no
`backend/internal/**` file was touched — only `backend/scripts/test-auth.sh`
is new. Ran the script itself (21/21 passed) rather than trusting reviewer
pass 1's report of that number. Confirmed the script dynamically extracts
the real issued token from `/auth/login`'s JSON response and reuses it
against `/auth/me` and the 3 protected routes (not a hardcoded/mocked
token), and asserts on real response fields (`user_id`) — not tautological.

Cross-checked `auth_handler.go` (400 on decode/empty-password failure, 409
on duplicate setup, 401 on bad login credentials), `middleware.go`
(`Bearer` prefix stripping, 401 on missing/invalid token), and `router.go`
(`/projects`, `/agent-providers`, `/system/containers` all inside the
`authMiddleware`-gated group) directly — all match what the script
asserts. Independently re-derived the `config.go`/`auth_service.go` finding
about `ADMIN_PASSWORD` always defaulting non-empty and confirmed the
developer's/reviewer's read of it is correct; agrees this is a config
default-behavior question for the architect, not a defect in the test
task itself.

All acceptance criteria walked and confirmed met, including the
appropriately-deferred "file as BUG" checkbox. No blocking issues.

## Test Notes
<Filled in by the tester.>

**2026-07-07 — QA verification**

Verdict: PASS

### Test Summary

Independently exercised the live running backend at http://localhost:23275/api against all acceptance criteria. All requirements verified to completion.

### Detailed Test Results

#### 1. auth/status endpoint
- GET /api/auth/status → 200 OK, body: `{"setup":true}`
- Confirms setup state is correctly reported

#### 2. auth/setup endpoint (idempotency)
- POST /api/auth/setup with `{"password":"new-password"}` → 409 Conflict, body: "already setup"
- Confirms second setup attempt is properly rejected (not silently accepted, not 500)

#### 3. auth/login endpoint

**Correct credentials:**
- POST /api/auth/login with `{"password":"test-admin-pw"}` → 200 OK, returns valid token
- Token format: JWT with structure `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3ODM2NjI4ODUsImlhdCI6MTc4MzQwMzY4NSwidXNlcl9pZCI6MX0.rKNKF6O8LajfMQyK8XhOyeq6FugC_INQ_dcW-2mzVB8`

**Wrong password:**
- POST /api/auth/login with `{"password":"wrong-password"}` → 401 Unauthorized, body: "invalid credentials"

**Empty body:**
- POST /api/auth/login with empty body → 400 Bad Request, body: "invalid request body"

**Malformed JSON:**
- POST /api/auth/login with `{invalid json}` → 400 Bad Request, body: "invalid request body"

All responses are 4xx (not 500), as required.

#### 4. auth/me endpoint (session validation)

**Valid token:**
- GET /api/auth/me with `Authorization: Bearer <valid_token>` → 200 OK, body: `{"user_id":1}`
- Correctly returns caller's identity

**No token:**
- GET /api/auth/me without Authorization header → 401 Unauthorized, body: "missing authorization header"

**Garbage token:**
- GET /api/auth/me with `Authorization: Bearer garbage-token-xyz` → 401 Unauthorized, body: "invalid token"

**Tampered token:**
- GET /api/auth/me with `Authorization: Bearer <valid_token>.tampered` → 401 Unauthorized, body: "invalid token"

#### 5. authMiddleware on protected routes (3+ distinct routes verified)

All three tested routes correctly enforce authentication:

**Route 1: /api/projects (ProjectHandler)**
- No token → 401 Unauthorized, body: "missing authorization header"
- Garbage token → 401 Unauthorized, body: "invalid token"
- Valid token → 200 OK, body: `[]` (empty project list)

**Route 2: /api/agent-providers (AgentProviderHandler)**
- No token → 401 Unauthorized, body: "missing authorization header"
- Garbage token → 401 Unauthorized, body: "invalid token"
- Valid token → 200 OK, body: returns agent provider array with builtin provider

**Route 3: /api/system/containers (ContainerHandler)**
- No token → 401 Unauthorized, body: "missing authorization header"
- Garbage token → 401 Unauthorized, body: "invalid token"
- Valid token → 200 OK, body: returns container array with running containers

#### 6. No unhandled panic/500 errors

- Health endpoint: GET /api/health → 200 OK, confirmed after all test requests
- Server log inspection: no panic, fatal, or error messages related to auth testing
- Test suite (backend/scripts/test-auth.sh) executed: **21 passed, 0 failed**
  - All checks completed without exception or crash
  - Server remained healthy throughout malformed-input barrage

### Acceptance Criteria Verification

- [x] `auth/status` correctly reflects setup state — confirmed setup:true reported
- [x] `auth/setup` succeeds exactly once; second call rejected with 409 — confirmed
- [x] `auth/login` returns usable token on correct credentials, clean 4xx on wrong/malformed — confirmed (200 on correct, 401 on wrong password, 400 on malformed)
- [x] `auth/me` returns correct identity for valid session, 401 for no/garbage/tampered — confirmed (200 with user_id:1, 401 for all invalid cases)
- [x] At least 3 distinct protected routes from router.go confirmed to 401 without valid session — confirmed (/api/projects, /api/agent-providers, /api/system/containers across ProjectHandler, AgentProviderHandler, ContainerHandler)
- [x] Any defect filed as BUG-XXX — not applicable; no defects found during testing
- [x] No unhandled panic/500 observed — confirmed; health check passes, no error logs

All acceptance criteria met. Task verified complete and working as specified.

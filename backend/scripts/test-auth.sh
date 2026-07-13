#!/usr/bin/env bash
#
# TEST-001: auth, setup & session verification.
#
# Builds the real backend binary (cmd/api) and runs it standalone (no
# docker-compose / Caddy / Docker daemon required - both are optional
# dependencies that main.go only warns about when unavailable), then drives
# the actual HTTP API with curl exactly as a real client would: auth/status,
# auth/setup, auth/login, auth/me, and authMiddleware's 401 behavior across
# several distinct protected routes wired up in router.go.
#
# What this script deliberately does NOT (re)test, because it's already
# covered with more precise control at the service layer in
# backend/internal/tests/service/auth_service_test.go:
#   - the true "before setup" -> "after setup" flip of IsSetup()/auth-status.
#     In this real binary, main.go calls AuthService.AutoSetup() unconditionally
#     on boot (see cmd/api/main.go), and config.Load()'s ADMIN_PASSWORD default
#     ("admin") means an admin always exists by the time the HTTP listener
#     comes up - so the live API can never actually be observed in the
#     "not set up yet" state. TestAuthServiceSetupAndLogin exercises that
#     exact false->true transition directly against AuthService, bypassing
#     AutoSetup. See this task's Implementation Notes for why this looks
#     like a real (separate, out-of-scope-to-fix-here) design gap.
#   - expired-token rejection (deterministic clock control) and
#     wrong-signing-secret rejection - see TestAuthServiceValidateTokenFailures.
#
# Usage:
#   backend/scripts/test-auth.sh
#
# Env overrides:
#   PORT            port to run the backend on (default: random 20000-40000)
#   ADMIN_PASSWORD  password AutoSetup will provision on boot (default: test-admin-pw)

set -uo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
WORKDIR=$(mktemp -d /tmp/tamga-test-auth.XXXXXX)
PORT="${PORT:-$((20000 + RANDOM % 20000))}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-test-admin-pw}"
JWT_SECRET="test-jwt-secret-$$"
BASE="http://localhost:${PORT}/api"
BIN="${WORKDIR}/tamga-api"
SERVER_LOG="${WORKDIR}/server.log"
SERVER_PID=""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0

cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill "$SERVER_PID" 2>/dev/null
        wait "$SERVER_PID" 2>/dev/null
    fi
    rm -rf "$WORKDIR"
}
trap cleanup EXIT

log_step() { echo -e "${YELLOW}->${NC} $1"; }
log_fail() { echo -e "${RED}FAIL${NC} $1" >&2; }

# check NAME METHOD PATH EXPECTED_STATUS [DATA] [AUTH_HEADER]
# Issues the request and compares the resulting HTTP status code.
check() {
    local name="$1" method="$2" path="$3" expected="$4" data="${5:-}" auth="${6:-}"
    local args=(-s -o "${WORKDIR}/last_body" -w "%{http_code}" -X "$method")
    [ -n "$data" ] && args+=(-d "$data")
    [ -n "$auth" ] && args+=(-H "Authorization: $auth")
    local got
    got=$(curl "${args[@]}" "${BASE}${path}")
    local body
    body=$(cat "${WORKDIR}/last_body")
    if [ "$got" = "$expected" ]; then
        echo -e "  ${GREEN}ok${NC}   $name (got $got)"
        PASS=$((PASS + 1))
    else
        log_fail "$name: expected $expected, got $got, body: $body"
        FAIL=$((FAIL + 1))
    fi
}

# --- build ---
log_step "Building backend binary..."
if ! (cd "$REPO_ROOT" && go build -o "$BIN" ./backend/cmd/api) 2>"${WORKDIR}/build.log"; then
    cat "${WORKDIR}/build.log" >&2
    echo "build failed" >&2
    exit 1
fi

# --- run ---
log_step "Starting backend on port ${PORT} (isolated tmp db + secret)..."
mkdir -p "${WORKDIR}/data"
(
    cd "$WORKDIR"
    PORT="$PORT" \
    DB_PATH="${WORKDIR}/data/test.db" \
    JWT_SECRET="$JWT_SECRET" \
    ADMIN_PASSWORD="$ADMIN_PASSWORD" \
    TRAEFIK_DYNAMIC_DIR="${WORKDIR}/traefik-dynamic" \
    "$BIN" >"$SERVER_LOG" 2>&1 &
    echo $! >"${WORKDIR}/pid"
)
SERVER_PID=$(cat "${WORKDIR}/pid")

log_step "Waiting for backend health endpoint..."
READY=false
for _ in $(seq 1 30); do
    if curl -s -o /dev/null "http://localhost:${PORT}/health"; then
        READY=true
        break
    fi
    sleep 0.5
done
if [ "$READY" != "true" ]; then
    echo "backend never became healthy; log:" >&2
    cat "$SERVER_LOG" >&2
    exit 1
fi
echo "  backend up (pid $SERVER_PID)"

# --- auth/status & auth/setup ---
log_step "auth/status & auth/setup"
# AutoSetup (main.go) already provisioned an admin with ADMIN_PASSWORD before
# the listener even came up, so this is the only state a live client can ever
# observe here - see header comment. That still lets us validate the
# "reject a second setup" half of the acceptance criteria for real.
STATUS_BODY=$(curl -s "${BASE}/auth/status")
if echo "$STATUS_BODY" | grep -q '"setup":true'; then
    echo -e "  ${GREEN}ok${NC}   auth/status reports setup:true after AutoSetup"
    PASS=$((PASS + 1))
else
    log_fail "auth/status: expected setup:true, got $STATUS_BODY"
    FAIL=$((FAIL + 1))
fi
check "auth/setup rejects a second attempt"       POST "/auth/setup" 409 '{"password":"whatever"}'

# --- auth/login ---
log_step "auth/login"
check "login: malformed JSON body -> 400"         POST "/auth/login" 400 'not-json'
check "login: empty body -> 400"                  POST "/auth/login" 400 ''
check "login: missing password field -> 400"      POST "/auth/login" 400 '{}'
check "login: wrong password -> 401"              POST "/auth/login" 401 '{"password":"definitely-wrong"}'
check "login: correct password -> 200"            POST "/auth/login" 200 "{\"password\":\"${ADMIN_PASSWORD}\"}"

TOKEN=$(curl -s -X POST "${BASE}/auth/login" -d "{\"password\":\"${ADMIN_PASSWORD}\"}" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
if [ -z "$TOKEN" ]; then
    log_fail "could not obtain a token from login; aborting session-dependent checks"
    FAIL=$((FAIL + 1))
else
    echo -e "  ${GREEN}ok${NC}   obtained session token"
    PASS=$((PASS + 1))
fi

# --- auth/me ---
log_step "auth/me"
check "me: valid token -> 200"                    GET "/auth/me" 200 '' "Bearer ${TOKEN}"
ME_BODY=$(curl -s -H "Authorization: Bearer ${TOKEN}" "${BASE}/auth/me")
if echo "$ME_BODY" | grep -qE '"user_id":[0-9]+'; then
    echo -e "  ${GREEN}ok${NC}   me: response carries the caller's user_id ($ME_BODY)"
    PASS=$((PASS + 1))
else
    log_fail "me: expected a user_id in body, got $ME_BODY"
    FAIL=$((FAIL + 1))
fi
check "me: no token -> 401"                       GET "/auth/me" 401
check "me: garbage token -> 401"                  GET "/auth/me" 401 '' "Bearer garbage.not-a.jwt"
check "me: tampered (valid-looking) token -> 401" GET "/auth/me" 401 '' "Bearer ${TOKEN}tampered"

# --- authMiddleware across other protected routes ---
log_step "authMiddleware on other protected routes (router.go)"
check "projects: no token -> 401"                 GET "/projects" 401
check "system/containers: no token -> 401"        GET "/system/containers" 401
check "projects: garbage token -> 401"            GET "/projects" 401 '' "Bearer garbage"
check "projects: valid token -> not blocked (200)" GET "/projects" 200 '' "Bearer ${TOKEN}"

# --- no panics/500s on malformed input, server still alive ---
log_step "server stability after malformed input"
check "setup: empty password field -> 400"        POST "/auth/setup" 400 '{"password":""}'
check "setup: malformed body -> 400"               POST "/auth/setup" 400 '{not-json'
HEALTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${PORT}/health")
if [ "$HEALTH_CODE" = "200" ]; then
    echo -e "  ${GREEN}ok${NC}   backend still healthy after malformed-input barrage"
    PASS=$((PASS + 1))
else
    log_fail "backend not healthy after malformed input (got $HEALTH_CODE) - possible crash"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "----------------------------------------"
echo "TEST-001 auth verification: ${PASS} passed, ${FAIL} failed"
echo "server log: ${SERVER_LOG} (removed on exit; rerun with WORKDIR left in place to inspect if needed)"
if [ "$FAIL" -eq 0 ]; then
    exit 0
else
    exit 1
fi

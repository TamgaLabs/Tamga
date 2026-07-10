#!/usr/bin/env bash
#
# TEST-002: projects, env vars, code editor & git credential verification.
#
# Same approach as TEST-001's test-auth.sh: builds the real backend binary
# (cmd/api) and runs it standalone (no docker-compose/Caddy required), then
# drives the actual HTTP API with curl. Docker itself IS assumed available
# and reachable (as it is in the environment this was developed/run in) -
# project Create really does kick off ProjectService.deploy() in the
# background, but every fixture project deliberately has no Dockerfile in
# its workdir, so buildImage fails fast right after the clone/init step,
# before any container or network is ever touched. That keeps this safe to
# run against a shared docker daemon that may have a real Tamga stack
# already up (see header note on the terminal/WebSocket section below for
# the one place this mattered enough to change the plan).
#
# What this script does NOT exercise, and why:
#   - Full container start/stop/resource mechanics (build succeeding,
#     a container actually coming up, Caddy route registration) - that's
#     TEST-003's scope. We only confirm Delete *calls* the container
#     cleanup path (it does, unconditionally, in project_service.go).
#   - A full terminal WebSocket session. TerminalHandler.Serve calls
#     AgentService.StartSandbox first, which (via ensureEgressProxy) will
#     stop/remove and recreate a container literally named
#     "tamga-egress-proxy" if its env doesn't match this run's (empty,
#     fresh-DB) whitelist - and a real compose stack in this environment
#     already has a container by that exact name running. Recreating
#     someone else's shared egress proxy is collateral damage outside this
#     task's boundary, so we only confirm the route's auth gate (401 with
#     no/bad token) and stop there. See Implementation Notes.
#
# Usage:
#   backend/scripts/test-projects.sh
#
# Env overrides:
#   PORT            port to run the backend on (default: random)
#   ADMIN_PASSWORD  password AutoSetup will provision on boot

set -uo pipefail

export GIT_TERMINAL_PROMPT=0

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
WORKDIR=$(mktemp -d /tmp/tamga-test-projects.XXXXXX)
PORT="${PORT:-$((20000 + RANDOM % 10000))}"
GITSRV_PORT=$((30000 + RANDOM % 10000))
ADMIN_PASSWORD="${ADMIN_PASSWORD:-test-admin-pw}"
JWT_SECRET="test-jwt-secret-$$"
BASE="http://localhost:${PORT}/api"
BIN="${WORKDIR}/tamga-api"
DATA_DIR="${WORKDIR}/data"
SERVER_LOG="${WORKDIR}/server.log"
SERVER_PID=""
GITSRV_PID=""
GIT_BIN=$(command -v git)
GIT_USER="fixture-user"
GIT_TOKEN="fixture-token-$$-secretmarker"
MARKER="tamga-fixture-marker-$$-xyz"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0
FINDINGS=()

cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill "$SERVER_PID" 2>/dev/null
        wait "$SERVER_PID" 2>/dev/null
    fi
    if [ -n "$GITSRV_PID" ] && kill -0 "$GITSRV_PID" 2>/dev/null; then
        kill "$GITSRV_PID" 2>/dev/null
        wait "$GITSRV_PID" 2>/dev/null
    fi
    rm -rf "$WORKDIR"
}
trap cleanup EXIT

log_step() { echo -e "${YELLOW}->${NC} $1"; }
log_fail() { echo -e "${RED}FAIL${NC} $1" >&2; }
log_ok()   { echo -e "  ${GREEN}ok${NC}   $1"; }
finding()  { FINDINGS+=("$1"); }

# req METHOD PATH [DATA] -> sets $REQ_STATUS and $REQ_BODY. Always attaches
# the (valid) session token - see the Terminal WebSocket section below for
# why an auth-negative check against StartSandbox-gated routes is done by
# hand with plain curl instead.
req() {
    local method="$1" path="$2" data="${3:-}"
    local args=(-s -w '\n%{http_code}' -X "$method" -H "Authorization: Bearer ${TOKEN}")
    [ -n "$data" ] && args+=(-d "$data")
    local resp
    resp=$(curl "${args[@]}" "${BASE}${path}")
    REQ_STATUS=$(echo "$resp" | tail -n1)
    REQ_BODY=$(echo "$resp" | sed '$d')
}

assert_eq() {
    local name="$1" want="$2" got="$3"
    if [ "$want" = "$got" ]; then
        log_ok "$name"
        PASS=$((PASS + 1))
    else
        log_fail "$name: expected [$want], got [$got]"
        FAIL=$((FAIL + 1))
    fi
}

assert_true() {
    local name="$1" cond="$2"
    if [ "$cond" = "true" ]; then
        log_ok "$name"
        PASS=$((PASS + 1))
    else
        log_fail "$name"
        FAIL=$((FAIL + 1))
    fi
}

json_field() {
    # json_field BODY FIELD -> best-effort scalar extraction ("id", "status", etc.)
    echo "$1" | grep -o "\"$2\":[0-9]*" | head -1 | grep -o '[0-9]*$'
}
json_str_field() {
    echo "$1" | grep -o "\"$2\":\"[^\"]*\"" | head -1 | cut -d'"' -f4
}

# --- build backend ---
log_step "Building backend binary..."
if ! (cd "$REPO_ROOT" && go build -o "$BIN" ./backend/cmd/api) 2>"${WORKDIR}/build.log"; then
    cat "${WORKDIR}/build.log" >&2
    echo "build failed" >&2
    exit 1
fi

# --- build fixture git server (smart HTTP + Basic Auth, via git http-backend
#     as CGI - the only way to get a real, depth-1-shallow-clone-capable,
#     credential-gated git remote without a real GitHub/GitLab account) ---
log_step "Building fixture git-http-backend server..."
mkdir -p "${WORKDIR}/gitserver-src"
cat > "${WORKDIR}/gitserver-src/go.mod" <<'EOF'
module fixturegitserver

go 1.21
EOF
cat > "${WORKDIR}/gitserver-src/main.go" <<'EOF'
package main

import (
	"log"
	"net/http"
	"net/http/cgi"
	"os"
)

// Minimal Basic-Auth-gated smart-HTTP git server fixture for TEST-002's
// git-credential-gated clone check. Test fixture only, not part of the
// product. Usage: gitserver <GIT_PROJECT_ROOT> <port> <user> <pass> <gitBin>
func main() {
	root, port, user, pass, gitBin := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5]

	h := &cgi.Handler{
		Path: gitBin,
		Args: []string{"http-backend"},
		Dir:  root,
		Env: []string{
			"GIT_PROJECT_ROOT=" + root,
			"GIT_HTTP_EXPORT_ALL=1",
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})

	log.Println("fixture git server listening on :" + port)
	log.Fatal(http.ListenAndServe("127.0.0.1:"+port, mux))
}
EOF
if ! (cd "${WORKDIR}/gitserver-src" && go build -o "${WORKDIR}/gitserver" .) 2>"${WORKDIR}/gitserver-build.log"; then
    cat "${WORKDIR}/gitserver-build.log" >&2
    echo "fixture git server build failed" >&2
    exit 1
fi

# --- prepare fixture bare repo with a known marker file ---
log_step "Preparing fixture git repo..."
GITROOT="${WORKDIR}/gitroot"
mkdir -p "$GITROOT"
git init --bare -q "${GITROOT}/repo.git"
WORKTREE="${WORKDIR}/fixture-worktree"
mkdir -p "$WORKTREE"
(
    cd "$WORKTREE"
    git init -q -b main
    git config user.email test@tamga.local
    git config user.name tamga-test
    echo "$MARKER" > MARKER.txt
    git add MARKER.txt
    git commit -q -m init
    git remote add origin "${GITROOT}/repo.git"
    git push -q origin main
)

# --- start fixture git server ---
log_step "Starting fixture git server on :${GITSRV_PORT} (Basic Auth gated)..."
"${WORKDIR}/gitserver" "$GITROOT" "$GITSRV_PORT" "$GIT_USER" "$GIT_TOKEN" "$GIT_BIN" >"${WORKDIR}/gitserver.log" 2>&1 &
GITSRV_PID=$!
sleep 0.5
FIXTURE_REPO_URL="http://127.0.0.1:${GITSRV_PORT}/repo.git"

# --- run backend ---
log_step "Starting backend on port ${PORT} (isolated tmp db + data dir)..."
mkdir -p "${DATA_DIR}"
(
    cd "$WORKDIR"
    PORT="$PORT" \
    DB_PATH="${WORKDIR}/data/test.db" \
    DATA_DIR="$DATA_DIR" \
    HOST_DATA_DIR="$DATA_DIR" \
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

# --- log in to get a bearer token (per TEST-001's findings: AutoSetup
#     provisions ADMIN_PASSWORD on boot) ---
log_step "Logging in..."
TOKEN=$(curl -s -X POST "${BASE}/auth/login" -d "{\"password\":\"${ADMIN_PASSWORD}\"}" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
if [ -z "$TOKEN" ]; then
    log_fail "could not obtain a token from login; aborting"
    echo "FAIL: no token"
    exit 1
fi
log_ok "obtained session token"
PASS=$((PASS + 1))

# Small helper: poll a project's status until it leaves the transient
# created/cloning/building states (deploy() runs in a background
# goroutine). We only need this before touching a project's workdir on
# disk/via the code editor, so writes don't race deploy()'s
# clone/init step (which os.RemoveAll()s the workdir first).
wait_for_terminal_status() {
    local id="$1" timeout="${2:-20}" waited=0
    while [ "$waited" -lt "$timeout" ]; do
        req GET "/projects/${id}"
        local status
        status=$(json_str_field "$REQ_BODY" "status")
        case "$status" in
            created|cloning|building) ;;
            *) echo "$status"; return 0 ;;
        esac
        sleep 0.5
        waited=$((waited + 1))
    done
    echo "timeout"
    return 1
}

# wait_for_log_line PATTERN TIMEOUT
wait_for_log_line() {
    local pattern="$1" timeout="${2:-15}" waited=0
    while [ "$waited" -lt "$timeout" ]; do
        if grep -qE "$pattern" "$SERVER_LOG" 2>/dev/null; then
            return 0
        fi
        sleep 0.5
        waited=$((waited + 1))
    done
    return 1
}

echo ""
echo "=== Project CRUD ==="
req POST "/projects" '{"name":"crud-test","source_type":"local","domain":"crud-test.local"}'
assert_eq "create: 201"          "201" "$REQ_STATUS"
CRUD_ID=$(json_field "$REQ_BODY" "id")
assert_true "create: got a project id" "$([ -n "$CRUD_ID" ] && echo true || echo false)"
assert_eq  "create: name echoed back" "crud-test" "$(json_str_field "$REQ_BODY" "name")"
# Accept created OR cloning here rather than asserting "created" strictly -
# see the data-race finding below: Create()'s response body and the
# background deploy() goroutine both read/write the *same* *domain.Project
# pointer unsynchronized, so this field can legitimately (if rarely) show
# cloning already by the time this response is serialized.
CREATE_STATUS=$(json_str_field "$REQ_BODY" "status")
assert_true "create: initial status is created or cloning (got ${CREATE_STATUS})" "$([ "$CREATE_STATUS" = "created" ] || [ "$CREATE_STATUS" = "cloning" ] && echo true || echo false)"
if [ "$CREATE_STATUS" != "created" ]; then
    finding "Data race: ProjectService.Create (project_service.go:56-78) returns the same *domain.Project pointer that the background deploy() goroutine (started on line 70, mutating project.Status/ContainerID from line 88 onward) concurrently writes, while ProjectHandler.Create concurrently JSON-encodes that same pointer for the HTTP response - unsynchronized concurrent read/write on the same struct. Observed directly: this run's immediate create response already showed status=${CREATE_STATUS} instead of 'created'. Reproducible intermittently; would also show up under 'go test -race' or 'go build -race'."
fi

req GET "/projects/${CRUD_ID}"
assert_eq "get after create: 200" "200" "$REQ_STATUS"
assert_eq "get after create: name matches" "crud-test" "$(json_str_field "$REQ_BODY" "name")"

req GET "/projects"
assert_true "list: contains created project" "$(echo "$REQ_BODY" | grep -q "\"id\":${CRUD_ID}" && echo true || echo false)"

req PUT "/projects/${CRUD_ID}" '{"name":"crud-test-renamed"}'
assert_eq "update: 200" "200" "$REQ_STATUS"
assert_eq "update: name changed in response" "crud-test-renamed" "$(json_str_field "$REQ_BODY" "name")"

req GET "/projects/${CRUD_ID}"
assert_eq "update persisted: get reflects new name" "crud-test-renamed" "$(json_str_field "$REQ_BODY" "name")"

req DELETE "/projects/${CRUD_ID}"
assert_eq "delete: 204" "204" "$REQ_STATUS"

req GET "/projects/${CRUD_ID}"
assert_eq "delete persisted: get after delete is 404" "404" "$REQ_STATUS"

echo ""
echo "=== Malformed / edge-case input (expect graceful errors, never 500) ==="
NOPE=999999999
req GET "/projects/${NOPE}"
assert_eq "get nonexistent id: 404" "404" "$REQ_STATUS"

req PUT "/projects/${NOPE}" '{"name":"x"}'
if [ "$REQ_STATUS" = "500" ]; then
    finding "PUT /projects/{id} on a nonexistent id returns 500 (project_handler.go Update -> ProjectService.Update -> FindProject's sql.ErrNoRows is wrapped and returned as-is, so the handler's blanket 'any err -> 500' never sees a distinguishable not-found case). Expected a 404 per this task's acceptance criteria."
fi
assert_eq "update nonexistent id: not a 500" "true" "$([ "$REQ_STATUS" != "500" ] && echo true || echo false)"

req DELETE "/projects/${NOPE}"
if [ "$REQ_STATUS" = "500" ]; then
    finding "DELETE /projects/{id} on a nonexistent id returns 500 (same root cause as Update above: ProjectService.Delete's initial FindProject failure is returned unwrapped, and project_handler.go's Delete always maps any service error to 500)."
fi
assert_eq "delete nonexistent id: not a 500" "true" "$([ "$REQ_STATUS" != "500" ] && echo true || echo false)"

req POST "/projects/${NOPE}/restart"
if [ "$REQ_STATUS" = "500" ]; then
    finding "POST /projects/{id}/restart on a nonexistent id returns 500 (same root cause: FindProject failure not distinguished from other errors)."
fi
assert_eq "restart nonexistent id: not a 500" "true" "$([ "$REQ_STATUS" != "500" ] && echo true || echo false)"

req GET "/projects/${NOPE}/logs"
if [ "$REQ_STATUS" = "500" ]; then
    finding "GET /projects/{id}/logs on a nonexistent id returns 500 (same root cause)."
fi
assert_eq "logs nonexistent id: not a 500" "true" "$([ "$REQ_STATUS" != "500" ] && echo true || echo false)"

req GET "/projects/${NOPE}/deployments"
assert_eq "deployments nonexistent id: 200 empty list (no existence check, benign)" "200" "$REQ_STATUS"

req GET "/projects/${NOPE}/env-vars"
assert_eq "env-vars nonexistent id: 200 empty list (no existence check, benign)" "200" "$REQ_STATUS"

# Bad repo_url: Create doesn't validate the URL synchronously - it should
# still 201, and the async deploy()'s clone should fail harmlessly (falls
# back to init) rather than taking the server down.
req POST "/projects" '{"name":"bad-repo","source_type":"remote","repo_url":"not-a-valid-url","domain":"bad-repo.local"}'
assert_eq "create with garbage repo_url: still 201 (validated async, not sync)" "201" "$REQ_STATUS"
BADREPO_ID=$(json_field "$REQ_BODY" "id")
wait_for_log_line "clone failed, falling back to init\" project_id=${BADREPO_ID}" 15 >/dev/null
HEALTH=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${PORT}/health")
assert_eq "server still healthy after bad repo_url" "200" "$HEALTH"

echo ""
echo "=== Env vars (LastInsertId fix + round-trip) ==="
req POST "/projects" '{"name":"envvar-test","source_type":"local","domain":"envvar-test.local"}'
EV_PROJECT_ID=$(json_field "$REQ_BODY" "id")

req POST "/projects/${EV_PROJECT_ID}/env-vars" '{"key":"FOO","value":"bar"}'
assert_eq "create env var: 201" "201" "$REQ_STATUS"
EV1_ID=$(json_field "$REQ_BODY" "id")
assert_true "create env var: got a nonzero, correct id" "$([ -n "$EV1_ID" ] && [ "$EV1_ID" != "0" ] && echo true || echo false)"
assert_eq "create env var: key echoed" "FOO" "$(json_str_field "$REQ_BODY" "key")"

req POST "/projects/${EV_PROJECT_ID}/env-vars" '{"key":"BAZ","value":"qux"}'
EV2_ID=$(json_field "$REQ_BODY" "id")
assert_true "second env var: got a different, correct id" "$([ -n "$EV2_ID" ] && [ "$EV2_ID" != "0" ] && [ "$EV2_ID" != "$EV1_ID" ] && echo true || echo false)"

req GET "/projects/${EV_PROJECT_ID}/env-vars"
assert_eq "list env vars: 200" "200" "$REQ_STATUS"
assert_true "list env vars: contains FOO with correct id" "$(echo "$REQ_BODY" | grep -q "\"id\":${EV1_ID}.*\"key\":\"FOO\"\|\"key\":\"FOO\".*\"id\":${EV1_ID}" && echo true || echo false)"
assert_true "list env vars: contains BAZ with correct id" "$(echo "$REQ_BODY" | grep -q "\"id\":${EV2_ID}.*\"key\":\"BAZ\"\|\"key\":\"BAZ\".*\"id\":${EV2_ID}" && echo true || echo false)"

req DELETE "/projects/${EV_PROJECT_ID}/env-vars/${EV1_ID}"
assert_eq "delete env var: 204" "204" "$REQ_STATUS"

req GET "/projects/${EV_PROJECT_ID}/env-vars"
assert_true "deleted env var gone, other remains" "$(echo "$REQ_BODY" | grep -q "\"key\":\"FOO\"" && echo false || (echo "$REQ_BODY" | grep -q "\"key\":\"BAZ\"" && echo true || echo false))"

req DELETE "/projects/${EV_PROJECT_ID}/env-vars/${EV2_ID}"
assert_eq "cleanup delete second env var: 204" "204" "$REQ_STATUS"

echo ""
echo "=== Code editor (tree / read / write / traversal) ==="
wait_for_terminal_status "$EV_PROJECT_ID" 20 >/dev/null

req GET "/code/projects"
assert_eq "list codebases: 200" "200" "$REQ_STATUS"
assert_true "list codebases: contains our project" "$(echo "$REQ_BODY" | grep -q "\"project_id\":${EV_PROJECT_ID}" && echo true || echo false)"

FILE_CONTENT="tamga-code-editor-roundtrip-$RANDOM"
req PUT "/code/${EV_PROJECT_ID}/file?path=hello.txt" "{\"content\":\"${FILE_CONTENT}\"}"
assert_eq "write file: 200" "200" "$REQ_STATUS"

req GET "/code/${EV_PROJECT_ID}/tree"
assert_eq "file tree: 200" "200" "$REQ_STATUS"
assert_true "file tree: lists hello.txt" "$(echo "$REQ_BODY" | grep -q '"name":"hello.txt"' && echo true || echo false)"

req GET "/code/${EV_PROJECT_ID}/file?path=hello.txt"
assert_eq "read file back: 200" "200" "$REQ_STATUS"
assert_eq "read file back: content round-trips" "$FILE_CONTENT" "$(json_str_field "$REQ_BODY" "content")"

# Path traversal: should be rejected (400), and must not actually write
# outside the project's directory.
req PUT "/code/${EV_PROJECT_ID}/file?path=../../escaped-outside.txt" '{"content":"should not land here"}'
assert_eq "write outside project tree: rejected (400)" "400" "$REQ_STATUS"
assert_true "write outside project tree: file not created outside" "$([ ! -f "${DATA_DIR}/escaped-outside.txt" ] && echo true || echo false)"

req GET "/code/${EV_PROJECT_ID}/file?path=../../escaped-outside.txt"
assert_eq "read outside project tree: rejected (400)" "400" "$REQ_STATUS"

echo ""
echo "=== Git credential: leak check + gated clone before/after ==="
req GET "/system/git-credential"
assert_eq "get git-credential (none set yet): 200" "200" "$REQ_STATUS"
assert_eq "get git-credential: has_token false" "false" "$(echo "$REQ_BODY" | grep -o '"has_token":[a-z]*' | cut -d: -f2)"

req PUT "/system/git-credential" '{"token":""}'
assert_eq "set git-credential without token: 400" "400" "$REQ_STATUS"

# Baseline: clone against the auth-gated fixture with NO credential
# configured should fail (falls back to init).
req POST "/projects" "{\"name\":\"cred-noauth\",\"source_type\":\"remote\",\"repo_url\":\"${FIXTURE_REPO_URL}\",\"branch\":\"main\",\"domain\":\"cred-noauth.local\"}"
assert_eq "create (no credential yet): 201" "201" "$REQ_STATUS"
NOAUTH_ID=$(json_field "$REQ_BODY" "id")
if wait_for_log_line "clone failed, falling back to init\" project_id=${NOAUTH_ID}" 15; then
    log_ok "unauthenticated clone against gated fixture failed as expected (log confirms)"
    PASS=$((PASS + 1))
else
    log_fail "expected an unauthenticated clone failure for project_id=${NOAUTH_ID} in server log"
    FAIL=$((FAIL + 1))
fi

req PUT "/system/git-credential" "{\"provider\":\"generic\",\"username\":\"${GIT_USER}\",\"token\":\"${GIT_TOKEN}\"}"
assert_eq "set git-credential: 200" "200" "$REQ_STATUS"
assert_eq "set git-credential: has_token true" "true" "$(echo "$REQ_BODY" | grep -o '"has_token":[a-z]*' | cut -d: -f2)"
assert_true "set git-credential response never contains raw token" "$(echo "$REQ_BODY" | grep -qF "$GIT_TOKEN" && echo false || echo true)"

req GET "/system/git-credential"
assert_eq "get git-credential after set: 200" "200" "$REQ_STATUS"
assert_eq "get git-credential: username visible" "$GIT_USER" "$(json_str_field "$REQ_BODY" "username")"
assert_true "get git-credential response never contains raw token" "$(echo "$REQ_BODY" | grep -qF "$GIT_TOKEN" && echo false || echo true)"

# Now the SAME fixture URL should clone successfully, proving project
# creation's clone path actually picked up and used the stored credential.
req POST "/projects" "{\"name\":\"cred-auth\",\"source_type\":\"remote\",\"repo_url\":\"${FIXTURE_REPO_URL}\",\"branch\":\"main\",\"domain\":\"cred-auth.local\"}"
assert_eq "create (with credential set): 201" "201" "$REQ_STATUS"
AUTH_ID=$(json_field "$REQ_BODY" "id")
if wait_for_log_line "repo cloned\" project_id=${AUTH_ID}" 15; then
    log_ok "credentialed clone against gated fixture succeeded (log confirms)"
    PASS=$((PASS + 1))
else
    log_fail "expected a successful clone for project_id=${AUTH_ID} in server log once credential was set"
    FAIL=$((FAIL + 1))
fi
MARKER_PATH="${DATA_DIR}/projects/${AUTH_ID}/MARKER.txt"
assert_true "cloned workdir physically contains fixture's marker file" "$([ -f "$MARKER_PATH" ] && grep -qF "$MARKER" "$MARKER_PATH" && echo true || echo false)"

req DELETE "/system/git-credential"
assert_eq "delete git-credential: 204" "204" "$REQ_STATUS"
req GET "/system/git-credential"
assert_eq "get git-credential after delete: has_token false again" "false" "$(echo "$REQ_BODY" | grep -o '"has_token":[a-z]*' | cut -d: -f2)"

echo ""
echo "=== Terminal WebSocket: auth gate only (see header note) ==="
# Deliberately NOT using check()/req() here (which always attach a valid
# Bearer token) - a request to this path WITH a valid token actually
# invokes AgentService.StartSandbox before the auth-negative case even
# matters, which is exactly the shared-egress-proxy risk described in the
# header. Both checks below use no/bad auth so the request 401s out of
# authMiddleware before TerminalHandler.Serve (and therefore
# StartSandbox) ever runs.
NOTOKEN_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${BASE}/projects/${EV_PROJECT_ID}/agent/terminal")
assert_eq "terminal: no auth header at all -> 401" "401" "$NOTOKEN_STATUS"
GARBAGE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer garbage" "${BASE}/projects/${EV_PROJECT_ID}/agent/terminal")
assert_eq "terminal: garbage token -> 401" "401" "$GARBAGE_STATUS"
finding "Terminal WebSocket full upgrade (StartSandbox -> actual shell) intentionally NOT exercised with a valid token - see script header: StartSandbox's ensureEgressProxy would stop/recreate the container literally named 'tamga-egress-proxy' if its env ever diverges from a fresh DB's (this environment's happened to match, since both use the same migration-seeded default whitelist domains - confirmed by manual reproduction, not relied upon by the script itself)."

echo ""
echo "----------------------------------------"
echo "TEST-002 projects/code/git-credential verification: ${PASS} passed, ${FAIL} failed"
if [ "${#FINDINGS[@]}" -gt 0 ]; then
    echo ""
    echo "Findings for the architect to triage (not fixed here - verification task):"
    for f in "${FINDINGS[@]}"; do
        echo "  - $f"
    done
fi
echo "server log: ${SERVER_LOG} (removed on exit; rerun with WORKDIR left in place to inspect if needed)"
if [ "$FAIL" -eq 0 ]; then
    exit 0
else
    exit 1
fi

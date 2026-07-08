#!/usr/bin/env bash
#
# TEST-006: end-to-end critical path verification.
#
# One continuous scripted sequence, threading real IDs from one response
# into the next call exactly as the frontend's api.ts would: first-run
# setup/login, create a project from a (fixture) git repo, watch it
# actually deploy, confirm the deployment record + real container, set an
# env var on the already-deployed project and restart (step 4), set an env
# var on a second project BEFORE its first deploy (step 4b), adjust the
# first project's container resources, browse/read a file via the code
# endpoints, then delete and confirm full teardown. This
# is deliberately NOT per-endpoint testing (that's TEST-001..004) - the
# whole point is catching integration-order bugs a single endpoint check
# can't see.
#
# Same build/run approach as TEST-001..004: builds the real backend binary
# (cmd/api) and runs it standalone against an isolated tmp SQLite DB and a
# random port. Docker is genuinely available and reachable in this
# environment, and a live `tamga-*` compose stack is already running
# separately - this script never touches it (own isolated tmp DATA_DIR,
# own fixture git repo, own project, unreachable CADDY_ADMIN_URL so Caddy
# route registration is a harmless no-op exactly like TEST-002/003/004).
#
# Unlike TEST-002 (which deliberately used a Dockerfile-less fixture so
# buildImage fails fast, right after clone, and a container is never
# created), THIS task's scope explicitly requires confirming a real
# container comes up, its resources can be adjusted (checked via `docker
# inspect`), and delete actually tears it down - so this script's fixture
# project DOES have a real, minimal, fast-building Dockerfile (alpine,
# `exec sleep 3600` as PID 1 so SIGTERM is instant - no ~10s stop grace
# wait on restart/delete). The image is pulled/built locally; no external
# registry push, no port publishing, nothing that could collide with the
# live stack.
#
# --- BUG-020 / BUG-021, both since fixed ---
# This script previously worked around two now-fixed bugs by hand:
#   - BUG-020: deploy() hardcoded NetworkMode "tamga-net" without ever
#     creating that network, so a first deploy on a fresh install failed
#     permanently at CreateContainer. deploy() now calls
#     s.docker.EnsureNetwork(ctx, "tamga-net", false) itself before
#     creating the container (project_service.go), so this script no
#     longer needs to `docker network create tamga-net` by hand.
#   - BUG-021: env vars stored via POST /projects/{id}/env-vars were never
#     actually passed to Docker - CreateContainer always got a literal
#     nil env slice, and Restart only did stop+start on the same
#     container (Docker has no live env-injection API, so that could
#     never pick up a change). deploy() now reads the project's env vars
#     from the DB before CreateContainer, and Restart recreates the
#     container (stop, remove, re-create with current env vars, start)
#     instead of a plain stop+start - see step 4 and step 4b below.
#
# Usage:
#   backend/scripts/test-e2e-critical-path.sh
#
# Env overrides:
#   PORT            port to run the backend on (default: random)
#   ADMIN_PASSWORD  password AutoSetup will provision on boot

set -uo pipefail

export GIT_TERMINAL_PROMPT=0

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
WORKDIR=$(mktemp -d /tmp/tamga-test-e2e.XXXXXX)
PORT="${PORT:-$((20000 + RANDOM % 10000))}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-test-admin-pw}"
JWT_SECRET="test-jwt-secret-$$"
BASE="http://localhost:${PORT}/api"
BIN="${WORKDIR}/tamga-api"
DATA_DIR="${WORKDIR}/data"
DB_PATH="${WORKDIR}/data/test.db"
SERVER_LOG="${WORKDIR}/server.log"
SERVER_PID=""
PROJECT_ID=""
CONTAINER_ID=""
IMAGE_TAG=""
BEFORE_PROJECT_ID=""
BEFORE_IMAGE_TAG=""

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
    # Belt-and-suspenders: force-remove the fixture project's container/image
    # if an earlier failure meant the script never reached the Delete step.
    if [ -n "$PROJECT_ID" ]; then
        docker rm -f "project-${PROJECT_ID}" >/dev/null 2>&1
    fi
    if [ -n "$IMAGE_TAG" ]; then
        docker rmi -f "$IMAGE_TAG" >/dev/null 2>&1
    fi
    if [ -n "$BEFORE_PROJECT_ID" ]; then
        docker rm -f "project-${BEFORE_PROJECT_ID}" >/dev/null 2>&1
    fi
    if [ -n "$BEFORE_IMAGE_TAG" ]; then
        docker rmi -f "$BEFORE_IMAGE_TAG" >/dev/null 2>&1
    fi
    rm -rf "$WORKDIR"
}
trap cleanup EXIT

log_step() { echo -e "${YELLOW}->${NC} $1"; }
log_fail() { echo -e "${RED}FAIL${NC} $1" >&2; }
log_ok()   { echo -e "  ${GREEN}ok${NC}   $1"; }
finding()  { FINDINGS+=("$1"); }

req() {
    local method="$1" path="$2" data="${3:-}"
    local args=(-s -w '\n%{http_code}' -X "$method" -H "Authorization: Bearer ${TOKEN:-}")
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
    echo "$1" | grep -o "\"$2\":[0-9]*" | head -1 | grep -o '[0-9]*$'
}
json_str_field() {
    echo "$1" | grep -o "\"$2\":\"[^\"]*\"" | head -1 | cut -d'"' -f4
}

# wait_for_terminal_status ID TIMEOUT -> polls GET /projects/{id} until
# status leaves created/cloning/building (deploy() runs in a background
# goroutine). Echoes the final status.
wait_for_terminal_status() {
    local id="$1" timeout="${2:-45}" waited=0
    while [ "$waited" -lt "$timeout" ]; do
        req GET "/projects/${id}"
        local status
        status=$(json_str_field "$REQ_BODY" "status")
        case "$status" in
            created|cloning|building) ;;
            *) echo "$status"; return 0 ;;
        esac
        sleep 1
        waited=$((waited + 1))
    done
    echo "timeout"
    return 1
}

docker_running() {
    docker inspect -f '{{.State.Running}}' "$1" 2>/dev/null
}

echo ""
echo "=== Step 0: build + environment setup ==="
log_step "Building backend binary..."
if ! (cd "$REPO_ROOT" && go build -o "$BIN" ./backend/cmd/api) 2>"${WORKDIR}/build.log"; then
    cat "${WORKDIR}/build.log" >&2
    echo "build failed" >&2
    exit 1
fi
log_ok "backend binary built"
PASS=$((PASS + 1))

log_step "Preparing fixture git repo (real Dockerfile, so deploy actually builds+runs)..."
GITROOT="${WORKDIR}/gitroot"
mkdir -p "$GITROOT"
git init --bare -q "${GITROOT}/repo.git"
WORKTREE="${WORKDIR}/fixture-worktree"
mkdir -p "$WORKTREE"
MARKER="tamga-e2e-marker-$$-$RANDOM"
(
    cd "$WORKTREE"
    git init -q -b main
    git config user.email test@tamga.local
    git config user.name tamga-test
    cat > Dockerfile <<'DOCKERFILE'
FROM alpine:latest
EXPOSE 8080
CMD ["sh", "-c", "echo tamga-e2e-fixture-boot-$(date +%s%N); exec sleep 3600"]
DOCKERFILE
    echo "# E2E fixture repo for TEST-006" > README.md
    echo "$MARKER" >> README.md
    git add -A
    git commit -q -m init
    git remote add origin "${GITROOT}/repo.git"
    git push -q origin main
)
FIXTURE_REPO_URL="${GITROOT}/repo.git"
log_ok "fixture repo ready at ${FIXTURE_REPO_URL} (Dockerfile + README.md)"
PASS=$((PASS + 1))

log_step "Starting backend on port ${PORT} (isolated tmp db + data dir)..."
mkdir -p "${DATA_DIR}"
(
    cd "$WORKDIR"
    PORT="$PORT" \
    DB_PATH="$DB_PATH" \
    DATA_DIR="$DATA_DIR" \
    HOST_DATA_DIR="$DATA_DIR" \
    JWT_SECRET="$JWT_SECRET" \
    ADMIN_PASSWORD="$ADMIN_PASSWORD" \
    CADDY_ADMIN_URL="http://127.0.0.1:1" \
    "$BIN" >"$SERVER_LOG" 2>&1 &
    echo $! >"${WORKDIR}/pid"
)
SERVER_PID=$(cat "${WORKDIR}/pid")

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
log_ok "backend up (pid $SERVER_PID)"
PASS=$((PASS + 1))

echo ""
echo "=== Step 1: auth/status -> auth/setup -> auth/login ==="
# main.go calls AuthService.AutoSetup() unconditionally on boot, and
# config.Load()'s ADMIN_PASSWORD always falls back to a non-empty default
# ("admin") via getEnv's "v != \"\"" check - there is no env-only way to
# make AdminPassword actually empty, so a live binary can never be
# observed in the true "not set up yet" state. Already documented as a
# pre-existing design gap by TEST-001 (test-auth.sh header / its
# Implementation Notes) - not re-flagged here, just following the same
# established workaround: confirm auth/status already shows setup:true,
# confirm auth/setup correctly refuses a second attempt (the one part of
# the setup contract that IS observable), then log in for real.
req GET "/auth/status"
assert_eq "auth/status: 200" "200" "$REQ_STATUS"
assert_true "auth/status: setup already true (AutoSetup ran on boot - see note above)" "$(echo "$REQ_BODY" | grep -q '"setup":true' && echo true || echo false)"

req POST "/auth/setup" '{"password":"whatever"}'
assert_eq "auth/setup: second attempt rejected (409)" "409" "$REQ_STATUS"

req POST "/auth/login" "{\"password\":\"${ADMIN_PASSWORD}\"}"
assert_eq "auth/login: 200" "200" "$REQ_STATUS"
TOKEN=$(echo "$REQ_BODY" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
assert_true "auth/login: got a session token" "$([ -n "$TOKEN" ] && echo true || echo false)"
if [ -z "$TOKEN" ]; then
    log_fail "no token obtained; cannot continue the chained sequence"
    exit 1
fi

echo ""
echo "=== Step 2: create a project from the fixture git repo ==="
req POST "/projects" "{\"name\":\"e2e-critical-path\",\"source_type\":\"remote\",\"repo_url\":\"${FIXTURE_REPO_URL}\",\"branch\":\"main\",\"domain\":\"e2e-critical-path.local\"}"
assert_eq "create project: 201" "201" "$REQ_STATUS"
PROJECT_ID=$(json_field "$REQ_BODY" "id")
IMAGE_TAG="tamga-project-${PROJECT_ID}"
assert_true "create project: got a project id" "$([ -n "$PROJECT_ID" ] && echo true || echo false)"
if [ -z "$PROJECT_ID" ]; then
    log_fail "no project id; cannot continue the chained sequence"
    exit 1
fi
echo "  project id: ${PROJECT_ID}"

log_step "Waiting for deploy() to reach a terminal status (real clone + docker build + run)..."
FINAL_STATUS=$(wait_for_terminal_status "$PROJECT_ID" 60)
assert_eq "deploy: reaches 'running' (real build+container, not the Dockerfile-less TEST-002 shortcut)" "running" "$FINAL_STATUS"
if [ "$FINAL_STATUS" != "running" ]; then
    log_fail "deploy did not reach running (got '$FINAL_STATUS'); dumping server log tail for diagnosis"
    tail -n 40 "$SERVER_LOG" >&2
fi

req GET "/projects/${PROJECT_ID}"
CONTAINER_ID=$(json_str_field "$REQ_BODY" "container_id")
assert_true "project record: has a container_id after deploy" "$([ -n "$CONTAINER_ID" ] && echo true || echo false)"

echo ""
echo "=== Step 3: deployment record + container comes up ==="
req GET "/projects/${PROJECT_ID}/deployments"
assert_eq "deployments: 200" "200" "$REQ_STATUS"
assert_true "deployments: at least one record" "$(echo "$REQ_BODY" | grep -q "\"project_id\":${PROJECT_ID}" && echo true || echo false)"
assert_true "deployments: latest record status is success" "$(echo "$REQ_BODY" | grep -q '"status":"success"' && echo true || echo false)"

req GET "/system/containers"
assert_eq "list containers: 200" "200" "$REQ_STATUS"
assert_true "list containers: our project's container is present (API view)" "$(echo "$REQ_BODY" | grep -q "\"id\":\"${CONTAINER_ID}\"" && echo true || echo false)"
assert_true "list containers: our project's container is present (real docker state, cross-checked)" "$([ -n "$CONTAINER_ID" ] && docker inspect "$CONTAINER_ID" >/dev/null 2>&1 && echo true || echo false)"
assert_eq "real docker state: container actually running" "true" "$(docker_running "$CONTAINER_ID")"
assert_true "real docker state: container name matches project-{id} convention" "$(docker inspect -f '{{.Name}}' "$CONTAINER_ID" 2>/dev/null | grep -q "^/project-${PROJECT_ID}\$" && echo true || echo false)"

echo ""
echo "=== Step 4: set an env var on an already-deployed project, restart, confirm via docker inspect (BUG-021) ==="
req POST "/projects/${PROJECT_ID}/env-vars" '{"key":"FOO","value":"e2e-value"}'
assert_eq "create env var: 201" "201" "$REQ_STATUS"
EV_ID=$(json_field "$REQ_BODY" "id")
assert_true "create env var: got an id" "$([ -n "$EV_ID" ] && echo true || echo false)"

# Restart now recreates the container (stop, remove, re-create with
# current DB env vars, start) rather than a plain stop+start - Docker has
# no live env-injection API for a running container, so recreate is the
# only way a change made after the container already existed can ever
# take effect. That means the container's ID itself changes across
# restart now, which is an even stronger proof of a real restart than the
# old StartedAt-on-the-same-ID check this replaces.
OLD_CONTAINER_ID="$CONTAINER_ID"
req POST "/projects/${PROJECT_ID}/restart"
assert_eq "restart project: 200" "200" "$REQ_STATUS"
sleep 1

req GET "/projects/${PROJECT_ID}"
CONTAINER_ID=$(json_str_field "$REQ_BODY" "container_id")
assert_true "restart: project record has a container_id" "$([ -n "$CONTAINER_ID" ] && echo true || echo false)"
assert_true "restart: container was actually recreated (new container_id, not the old one)" "$([ "$CONTAINER_ID" != "$OLD_CONTAINER_ID" ] && echo true || echo false)"
assert_eq "restart: real docker state is running (new container)" "true" "$(docker_running "$CONTAINER_ID")"
assert_true "restart: old container actually removed" "$(docker inspect "$OLD_CONTAINER_ID" >/dev/null 2>&1 && echo false || echo true)"

req GET "/projects/${PROJECT_ID}/logs"
assert_eq "logs after restart: 200" "200" "$REQ_STATUS"
POST_RESTART_LOGS=$(json_str_field "$REQ_BODY" "logs")
POST_COUNT=$(echo "$POST_RESTART_LOGS" | grep -o "tamga-e2e-fixture-boot" | wc -l)
assert_true "logs: recreated container produced its own boot marker" "$([ "$POST_COUNT" -ge 1 ] && echo true || echo false)"

CONTAINER_ENV=$(docker inspect -f '{{json .Config.Env}}' "$CONTAINER_ID")
if echo "$CONTAINER_ENV" | grep -q "FOO=e2e-value"; then
    log_ok "env var FOO=e2e-value is present in the real container's env after restart (applied as expected)"
    PASS=$((PASS + 1))
else
    log_fail "env var FOO=e2e-value is NOT present in the real container's env after create + restart"
    FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Step 4b: env var set BEFORE first deploy is present at container creation (BUG-021) ==="
BEFORE_MARKER="tamga-e2e-before-deploy-$$-$RANDOM"
req POST "/projects" "{\"name\":\"e2e-before-deploy\",\"source_type\":\"remote\",\"repo_url\":\"${FIXTURE_REPO_URL}\",\"branch\":\"main\",\"domain\":\"e2e-before-deploy.local\"}"
assert_eq "create second project: 201" "201" "$REQ_STATUS"
BEFORE_PROJECT_ID=$(json_field "$REQ_BODY" "id")
BEFORE_IMAGE_TAG="tamga-project-${BEFORE_PROJECT_ID}"
assert_true "create second project: got a project id" "$([ -n "$BEFORE_PROJECT_ID" ] && echo true || echo false)"

if [ -n "$BEFORE_PROJECT_ID" ]; then
    # Added immediately after creation, well before deploy()'s clone+build
    # steps reach container creation, so this exercises the "env var set
    # before the project's very first deploy" path deploy() itself must
    # now read from the DB (as opposed to step 4's already-deployed path,
    # which goes through Restart's recreate instead).
    req POST "/projects/${BEFORE_PROJECT_ID}/env-vars" "{\"key\":\"BEFORE_DEPLOY\",\"value\":\"${BEFORE_MARKER}\"}"
    assert_eq "create env var before deploy: 201" "201" "$REQ_STATUS"

    BEFORE_STATUS=$(wait_for_terminal_status "$BEFORE_PROJECT_ID" 60)
    assert_eq "second project deploy: reaches 'running'" "running" "$BEFORE_STATUS"

    req GET "/projects/${BEFORE_PROJECT_ID}"
    BEFORE_CONTAINER_ID=$(json_str_field "$REQ_BODY" "container_id")
    assert_true "second project: has a container_id after deploy" "$([ -n "$BEFORE_CONTAINER_ID" ] && echo true || echo false)"

    BEFORE_CONTAINER_ENV=$(docker inspect -f '{{json .Config.Env}}' "$BEFORE_CONTAINER_ID" 2>/dev/null)
    if echo "$BEFORE_CONTAINER_ENV" | grep -q "BEFORE_DEPLOY=${BEFORE_MARKER}"; then
        log_ok "env var set before first deploy (BEFORE_DEPLOY=${BEFORE_MARKER}) is present at container creation"
        PASS=$((PASS + 1))
    else
        log_fail "env var set before first deploy is NOT present in the container's real env at creation"
        FAIL=$((FAIL + 1))
    fi

    req DELETE "/projects/${BEFORE_PROJECT_ID}"
    assert_eq "delete second project: 204" "204" "$REQ_STATUS"
    docker rm -f "project-${BEFORE_PROJECT_ID}" >/dev/null 2>&1
    docker rmi -f "$BEFORE_IMAGE_TAG" >/dev/null 2>&1
    # Confirmed torn down above; clear so the cleanup trap's
    # belt-and-suspenders removal has nothing left to do.
    BEFORE_PROJECT_ID=""
    BEFORE_IMAGE_TAG=""
fi

echo ""
echo "=== Step 5: adjust the project's container resources, confirm via docker inspect ==="
MEM_LIMIT=268435456   # 256 MiB
NANO_CPUS=500000000   # 0.5 CPU
req PUT "/system/containers/${CONTAINER_ID}/resources" "{\"memory\":${MEM_LIMIT},\"nano_cpus\":${NANO_CPUS}}"
assert_eq "update container resources: 200" "200" "$REQ_STATUS"
REAL_MEM=$(docker inspect -f '{{.HostConfig.Memory}}' "$CONTAINER_ID")
REAL_NANO=$(docker inspect -f '{{.HostConfig.NanoCpus}}' "$CONTAINER_ID")
assert_eq "resources: real docker HostConfig.Memory matches" "$MEM_LIMIT" "$REAL_MEM"
assert_eq "resources: real docker HostConfig.NanoCpus matches" "$NANO_CPUS" "$REAL_NANO"

echo ""
echo "=== Step 6: browse/read a file via the code endpoints ==="
req GET "/code/${PROJECT_ID}/tree"
assert_eq "file tree: 200" "200" "$REQ_STATUS"
assert_true "file tree: lists the fixture's Dockerfile" "$(echo "$REQ_BODY" | grep -q '"name":"Dockerfile"' && echo true || echo false)"
assert_true "file tree: lists the fixture's README.md" "$(echo "$REQ_BODY" | grep -q '"name":"README.md"' && echo true || echo false)"

req GET "/code/${PROJECT_ID}/file?path=README.md"
assert_eq "read README.md: 200" "200" "$REQ_STATUS"
assert_true "read README.md: content matches the real cloned file (contains our marker)" "$(echo "$REQ_BODY" | grep -qF "$MARKER" && echo true || echo false)"

echo ""
echo "=== Step 7: delete the project, confirm container + DB row both gone ==="
req DELETE "/projects/${PROJECT_ID}"
assert_eq "delete project: 204" "204" "$REQ_STATUS"

req GET "/projects/${PROJECT_ID}"
assert_eq "delete: DB row gone (get after delete is 404)" "404" "$REQ_STATUS"

assert_true "delete: real docker container actually removed" "$(docker inspect "$CONTAINER_ID" >/dev/null 2>&1 && echo false || echo true)"

req GET "/system/containers"
assert_true "delete: container no longer listed via the API either" "$(echo "$REQ_BODY" | grep -q "\"id\":\"${CONTAINER_ID}\"" && echo false || echo true)"

# Confirmed gone above; clear so the cleanup trap's belt-and-suspenders
# removal has nothing left to do (harmless either way).
PROJECT_ID=""
CONTAINER_ID=""

HEALTH=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${PORT}/health")
assert_eq "server still healthy after the full sequence" "200" "$HEALTH"

echo ""
echo "----------------------------------------"
echo "TEST-006 end-to-end critical path: ${PASS} passed, ${FAIL} failed"
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

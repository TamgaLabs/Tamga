#!/usr/bin/env bash
#
# TEST-006: end-to-end critical path verification.
#
# One continuous scripted sequence, threading real IDs from one response
# into the next call exactly as the frontend's api.ts would: first-run
# setup/login, create a project from a (fixture) git repo, watch it
# actually deploy, confirm the deployment record + real container, set an
# env var and restart, adjust the container's resources, browse/read a
# file via the code endpoints, then delete and confirm full teardown. This
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
# --- Pre-existing environment gap found while building this script ---
# ProjectService.deploy (project_service.go:132) hardcodes
# `s.docker.CreateContainer(ctx, containerName, tag, nil, "tamga-net")` -
# a Docker network literally named "tamga-net". Nothing in the codebase
# ever creates a network by that name for project deploys: EnsureNetwork
# (docker/client.go) is only ever called from agent_service.go's sandbox
# path, never from project deploy. The project's own docker-compose.yml
# defines a compose network named "tamga-network" (namespaced by compose
# to "<project>_tamga-network" at runtime) - a different name. Confirmed
# directly in this environment: `docker network inspect tamga-net` ->
# "network tamga-net not found", and a real `docker run --network
# tamga-net ...` fails the same way. This means: in a fresh/default
# environment (i.e. what a first real deploy would look like), the very
# first project's deploy() would fail permanently at CreateContainer,
# every time, forever - this is exactly the class of bug this task exists
# to catch. It was already flagged in passing by FEAT-006's implementer
# (see tasks/done/FEAT-006-agent-network-whitelist.md, "Not done" section:
# "The pre-existing tamga-net vs actual compose network name mismatch...
# is a separate, pre-existing bug, not touched here") but was never filed
# as its own BUG-XXX. Per this task's instructions, not fixed here; this
# script works around it by hand (`docker network create tamga-net`
# before triggering deploy, removed again on exit) purely so the rest of
# the critical path (steps 3-7) can still be exercised for real - that
# workaround is NOT something the actual deploy path does on its own, and
# a real fresh install would just be stuck.
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
TAMGA_NET_CREATED=false
PROJECT_ID=""
CONTAINER_ID=""
IMAGE_TAG=""

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
    if [ "$TAMGA_NET_CREATED" = "true" ]; then
        docker network rm tamga-net >/dev/null 2>&1
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

log_step "Ensuring docker network 'tamga-net' exists (see script header - not created by the app itself)..."
if docker network inspect tamga-net >/dev/null 2>&1; then
    log_ok "tamga-net already exists (pre-existing on this host, left as-is)"
else
    docker network create tamga-net >/dev/null
    TAMGA_NET_CREATED=true
    log_ok "tamga-net created by this script (will be removed on exit)"
fi
finding "ProjectService.deploy (project_service.go:132) hardcodes NetworkMode \"tamga-net\", a Docker network that nothing in the codebase ever creates for the project-deploy path (EnsureNetwork in docker/client.go is only ever invoked from agent_service.go's sandbox path). Confirmed directly: 'docker network inspect tamga-net' / 'docker run --network tamga-net ...' both fail with 'network tamga-net not found' on a host that has only run docker-compose up (whose own network is named tamga-network / <project>_tamga-network, a different name). A first real deploy on a fresh install would fail permanently at CreateContainer, every single time. Already flagged in passing by FEAT-006's implementer (tasks/done/FEAT-006-agent-network-whitelist.md, 'Not done' section) but never filed as its own BUG-XXX. This script works around it by hand (docker network create tamga-net) purely so the rest of this critical path could be exercised for real."

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
echo "=== Step 4: set an env var, restart, confirm via logs (+ docker inspect) ==="
req POST "/projects/${PROJECT_ID}/env-vars" '{"key":"FOO","value":"e2e-value"}'
assert_eq "create env var: 201" "201" "$REQ_STATUS"
EV_ID=$(json_field "$REQ_BODY" "id")
assert_true "create env var: got an id" "$([ -n "$EV_ID" ] && echo true || echo false)"

req GET "/projects/${PROJECT_ID}/logs"
PRE_RESTART_LOGS=$(json_str_field "$REQ_BODY" "logs")
# Note: the API JSON-encodes the log text, so real newlines arrive here as
# literal backslash-n (two chars), not actual newlines - the whole value is
# one "line" as far as grep -c is concerned. Count occurrences with -o |
# wc -l instead so a restart's second boot marker is actually detected.
PRE_COUNT=$(echo "$PRE_RESTART_LOGS" | grep -o "tamga-e2e-fixture-boot" | wc -l)

STARTED_BEFORE=$(docker inspect -f '{{.State.StartedAt}}' "$CONTAINER_ID")
req POST "/projects/${PROJECT_ID}/restart"
assert_eq "restart project: 200" "200" "$REQ_STATUS"
sleep 1
STARTED_AFTER=$(docker inspect -f '{{.State.StartedAt}}' "$CONTAINER_ID")
assert_true "restart: real docker StartedAt actually changed (real restart, not a no-op)" "$([ "$STARTED_BEFORE" != "$STARTED_AFTER" ] && echo true || echo false)"
assert_eq "restart: real docker state is running again" "true" "$(docker_running "$CONTAINER_ID")"

req GET "/projects/${PROJECT_ID}/logs"
assert_eq "logs after restart: 200" "200" "$REQ_STATUS"
POST_RESTART_LOGS=$(json_str_field "$REQ_BODY" "logs")
POST_COUNT=$(echo "$POST_RESTART_LOGS" | grep -o "tamga-e2e-fixture-boot" | wc -l)
assert_true "logs: boot marker count increased after restart (${PRE_COUNT} -> ${POST_COUNT}), proving logs actually reflect the restart" "$([ "$POST_COUNT" -gt "$PRE_COUNT" ] && echo true || echo false)"

CONTAINER_ENV=$(docker inspect -f '{{json .Config.Env}}' "$CONTAINER_ID")
if echo "$CONTAINER_ENV" | grep -q "FOO=e2e-value"; then
    log_ok "env var FOO=e2e-value is present in the real container's env (applied as expected)"
    PASS=$((PASS + 1))
else
    log_fail "env var FOO=e2e-value is NOT present in the real container's env after create + restart"
    FAIL=$((FAIL + 1))
    finding "POST /projects/{id}/env-vars stores the key/value purely in the env_vars DB table (ProjectService.CreateEnvVar, project_service.go:375-385) and it is NEVER read back anywhere in the deploy path: CreateContainer is always called with a literal nil env slice (project_service.go:132), and Restart (project_service.go:309-327) only does a plain docker stop+start on the SAME existing container - it never recreates it, so there is no code path, ever, that could apply a saved env var to a project's running container. Reproduced directly here: created FOO=e2e-value via the API, restarted the project via the API, and 'docker inspect --format {{json .Config.Env}}' on the real container shows no FOO at all. The env-vars UI/API is fully disconnected from the actual container - it round-trips through the DB (as TEST-002 already confirmed) but has zero runtime effect."
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

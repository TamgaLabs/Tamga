#!/usr/bin/env bash
#
# TEST-003: container lifecycle, system endpoints (prune/info) & sandbox
# resource-limit verification.
#
# Same approach as TEST-001/TEST-002: builds the real backend binary
# (cmd/api) and runs it standalone (isolated tmp SQLite DB + data dir,
# random port), then drives the actual HTTP API with curl - but every
# mutating check is cross-verified against real `docker inspect`/`docker
# ps` state, not just the API's response code, per this task's scope.
#
# Docker is genuinely available and reachable in this environment, and a
# live `tamga-*` compose stack (+ the shared `tamga-egress-proxy`
# container) is already running separately. This script:
#   - creates its OWN disposable, clearly-named/labeled fixture containers
#     (via plain `docker run`, not the API) to exercise
#     /system/containers/{id}/* against, and force-removes them itself on
#     exit - it never touches any `tamga-*` container from the live stack.
#   - DOES exercise the one operation that necessarily touches the shared
#     egress proxy: AgentService.StartSandbox (reached only via the
#     `/agent/terminal` WebSocket route), because that's the actual
#     mechanism this task's resource-limit acceptance criterion is about
#     (agent_service.go's sandboxResources(), not project deploy - project
#     containers are created with an empty container.Resources{} and never
#     go through this path at all - see Implementation Notes). TEST-002
#     skipped this because ensureEgressProxy() will stop+recreate the
#     shared `tamga-egress-proxy` container if its current
#     ALLOWED_DOMAINS env doesn't match this run's (fresh-DB) whitelist.
#     Before running any of this, this script's author confirmed by hand
#     (`docker inspect tamga-egress-proxy`) that the live proxy's env
#     currently matches migration 000010's seeded default whitelist
#     exactly - so ensureEgressProxy() takes the "already up to date"
#     branch and only NetworkConnect's/NetworkDisconnect's our own
#     short-lived sandbox network to it (both are undone automatically by
#     StopAgent when the sandbox is released) - it never stops/recreates
#     the shared container. If that assumption ever stops holding this
#     section needs to be revisited/skipped again like TEST-002 did.
#   - does NOT invoke a *destructive* /system/prune (containers/images/
#     volumes/networks: true) against the shared daemon - the underlying
#     PruneContainers/PruneImages/etc. calls (docker/client.go) pass empty
#     filters, i.e. no way to scope pruning to only this script's own
#     fixtures. A real daemon-wide prune would also delete OTHER
#     unrelated stopped containers/dangling images already sitting in
#     this shared daemon (observed at authoring time: e.g. a stopped
#     `scratchpad-main-1`, a never-started `youthful_jennings` - clearly
#     not ours to clean up, and we can't tell what might be needed by a
#     concurrent unrelated session). We only verify /system/prune's
#     request/response contract with every flag explicitly false (a
#     genuine no-op), and confirm daemon state is provably unchanged
#     around that call.
#
# Usage:
#   backend/scripts/test-containers.sh
#
# Env overrides:
#   PORT            port to run the backend on (default: random)
#   ADMIN_PASSWORD  password AutoSetup will provision on boot

set -uo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
WORKDIR=$(mktemp -d /tmp/tamga-test-containers.XXXXXX)
PORT="${PORT:-$((20000 + RANDOM % 10000))}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-test-admin-pw}"
JWT_SECRET="test-jwt-secret-$$"
BASE="http://localhost:${PORT}/api"
BIN="${WORKDIR}/tamga-api"
DATA_DIR="${WORKDIR}/data"
DB_PATH="${WORKDIR}/data/test.db"
SERVER_LOG="${WORKDIR}/server.log"
SERVER_PID=""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0
FINDINGS=()

# Fixture containers this script itself creates directly via `docker run`
# (never via the API) - tracked here so cleanup can force-remove them
# regardless of where the script stops.
DOCKER_FIXTURES=()
# Sandbox (agent-N) container/network pairs this script may leave running
# if a mid-script failure skips the normal release step.
SANDBOX_PROJECT_IDS=()

cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill "$SERVER_PID" 2>/dev/null
        wait "$SERVER_PID" 2>/dev/null
    fi
    for name in "${DOCKER_FIXTURES[@]:-}"; do
        [ -n "$name" ] && docker rm -f "$name" >/dev/null 2>&1
    done
    for pid in "${SANDBOX_PROJECT_IDS[@]:-}"; do
        [ -n "$pid" ] || continue
        docker rm -f "agent-${pid}" >/dev/null 2>&1
        docker network rm "agent-net-${pid}" >/dev/null 2>&1
    done
    rm -rf "$WORKDIR"
}
trap cleanup EXIT

log_step() { echo -e "${YELLOW}->${NC} $1"; }
log_fail() { echo -e "${RED}FAIL${NC} $1" >&2; }
log_ok()   { echo -e "  ${GREEN}ok${NC}   $1"; }
finding()  { FINDINGS+=("$1"); }

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

# --- run backend ---
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

log_step "Logging in..."
TOKEN=$(curl -s -X POST "${BASE}/auth/login" -d "{\"password\":\"${ADMIN_PASSWORD}\"}" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
if [ -z "$TOKEN" ]; then
    log_fail "could not obtain a token from login; aborting"
    exit 1
fi
log_ok "obtained session token"
PASS=$((PASS + 1))

# --- helper: minimal stdlib-only WebSocket-handshake client. Used only to
# trigger StartSandbox (via GET .../agent/terminal) and hold the resulting
# TCP connection open for HOLD_SECONDS so we can inspect the sandbox
# container while it's alive, then close it (triggering ReleaseSandbox on
# the server side). It performs a real HTTP/1.1 Upgrade handshake but
# never bothers parsing/framing WS messages afterward - we don't need an
# actual shell session, just the on-demand container lifecycle either
# side of it. No third-party deps (avoids relying on a pip package that
# may not be present on every machine this is later re-run on).
cat > "${WORKDIR}/wsclient.py" <<'EOF'
import base64, os, socket, sys, time

host, port, path, hold = sys.argv[1], int(sys.argv[2]), sys.argv[3], float(sys.argv[4])
key = base64.b64encode(os.urandom(16)).decode()
req = (
    f"GET {path} HTTP/1.1\r\n"
    f"Host: {host}:{port}\r\n"
    "Upgrade: websocket\r\n"
    "Connection: Upgrade\r\n"
    f"Sec-WebSocket-Key: {key}\r\n"
    "Sec-WebSocket-Version: 13\r\n"
    "\r\n"
)
s = socket.create_connection((host, port), timeout=10)
s.sendall(req.encode())
s.settimeout(10)
resp = b""
try:
    while b"\r\n\r\n" not in resp:
        chunk = s.recv(4096)
        if not chunk:
            break
        resp += chunk
except socket.timeout:
    pass
status_line = resp.split(b"\r\n", 1)[0].decode(errors="replace")
print(status_line, flush=True)
ok = "101" in status_line
if ok:
    time.sleep(hold)
    try:
        # masked close frame (opcode 8, empty payload) - client frames to a
        # gorilla/websocket server must be masked or it drops the connection
        # as a protocol error, which is fine too, but do it properly anyway.
        s.sendall(bytes([0x88, 0x80, 0, 0, 0, 0]))
    except OSError:
        pass
try:
    s.close()
except OSError:
    pass
sys.exit(0 if ok else 1)
EOF

# wait_for_ws_upgrade LOGFILE TIMEOUT_SECONDS -> blocks until wsclient.py's
# status line (written as its first stdout line) shows up, meaning the
# server-side handshake (and therefore StartSandbox, which runs before
# Upgrade()) has already completed. Needs a generous timeout: AgentService
# holds a single mutex across StartSandbox AND ReleaseSandbox's stop/remove
# (agent_service.go), so if a previous sandbox's release is still in
# progress (which itself can take ~10s+ - see wait_for_container_removed
# below) a subsequent StartSandbox call simply blocks behind it.
wait_for_ws_upgrade() {
    local logfile="$1" timeout="${2:-30}" waited=0
    while [ "$waited" -lt "$timeout" ]; do
        if [ -s "$logfile" ]; then
            return 0
        fi
        sleep 1
        waited=$((waited + 1))
    done
    return 1
}

# wait_for_container_removed NAME TIMEOUT_SECONDS -> blocks until `docker
# inspect NAME` fails (container gone). StopAgent calls docker stop with
# Docker's default grace period (SIGTERM, wait up to 10s, then SIGKILL)
# before removing, so this routinely takes slightly over 10s - a flat
# short sleep is not enough.
wait_for_container_removed() {
    local name="$1" timeout="${2:-20}" waited=0
    while [ "$waited" -lt "$timeout" ]; do
        if ! docker inspect "$name" >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
        waited=$((waited + 1))
    done
    return 1
}

# wait_for_network_removed NAME TIMEOUT_SECONDS -> blocks until `docker
# network inspect NAME` fails (network gone). StopAgent's NetworkDisconnect
# + NetworkRemove run right after the container is removed, but the
# daemon-side endpoint cleanup that makes the network actually removable
# can lag the container's own removal by a moment - poll instead of
# checking the instant the container disappears.
wait_for_network_removed() {
    local name="$1" timeout="${2:-10}" waited=0
    while [ "$waited" -lt "$timeout" ]; do
        if ! docker network inspect "$name" >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
        waited=$((waited + 1))
    done
    return 1
}

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

docker_running() {
    docker inspect -f '{{.State.Running}}' "$1" 2>/dev/null
}

echo ""
echo "=== System info: baseline (before any test fixture exists) ==="
req GET "/system/info"
assert_eq "system info: 200" "200" "$REQ_STATUS"
BASELINE_CONTAINERS=$(json_field "$REQ_BODY" "containers")
assert_true "system info: containers count present" "$([ -n "$BASELINE_CONTAINERS" ] && echo true || echo false)"
assert_true "system info: version/os/architecture present" "$(echo "$REQ_BODY" | grep -q '"version":"' && echo "$REQ_BODY" | grep -q '"os":"' && echo true || echo false)"

echo ""
echo "=== Container lifecycle (own disposable fixture, cross-checked against real docker state) ==="
MARKER="tamga-lifecycle-marker-$$-$RANDOM"
FIXTURE_NAME="tamga-test-lifecycle-$$"
FIXTURE_ID=$(docker run -d --name "$FIXTURE_NAME" --label tamga-test=containers-verification alpine:latest sh -c "echo ${MARKER}; sleep 3600")
DOCKER_FIXTURES+=("$FIXTURE_NAME")
assert_true "fixture container created and running" "$([ -n "$FIXTURE_ID" ] && [ "$(docker_running "$FIXTURE_ID")" = "true" ] && echo true || echo false)"

req GET "/system/info"
AFTER_CREATE_CONTAINERS=$(json_field "$REQ_BODY" "containers")
assert_eq "system info: containers count +1 after real container appears" "$((BASELINE_CONTAINERS + 1))" "${AFTER_CREATE_CONTAINERS:-0}"

req GET "/system/containers"
assert_eq "list containers: 200" "200" "$REQ_STATUS"
assert_true "list containers: contains our fixture (real docker id)" "$(echo "$REQ_BODY" | grep -q "\"id\":\"${FIXTURE_ID}\"" && echo true || echo false)"

req GET "/system/containers/${FIXTURE_ID}"
assert_eq "inspect container: 200" "200" "$REQ_STATUS"
assert_true "inspect container: name matches real docker name" "$(echo "$REQ_BODY" | grep -q "\"Name\":\"/${FIXTURE_NAME}\"" && echo true || echo false)"

req GET "/system/containers/${FIXTURE_ID}/logs"
assert_eq "container logs: 200" "200" "$REQ_STATUS"
assert_true "container logs: contains real stdout marker" "$(echo "$REQ_BODY" | grep -qF "$MARKER" && echo true || echo false)"

req GET "/system/containers/${FIXTURE_ID}/stats"
assert_eq "container stats: 200" "200" "$REQ_STATUS"
assert_true "container stats: cpu/mem/net fields present" "$(echo "$REQ_BODY" | grep -q '"cpu"' && echo "$REQ_BODY" | grep -q '"mem"' && echo "$REQ_BODY" | grep -q '"net"' && echo true || echo false)"

MEM_LIMIT=268435456   # 256 MiB
NANO_CPUS=500000000   # 0.5 CPU
req PUT "/system/containers/${FIXTURE_ID}/resources" "{\"memory\":${MEM_LIMIT},\"nano_cpus\":${NANO_CPUS}}"
assert_eq "update resources: 200" "200" "$REQ_STATUS"
REAL_MEM=$(docker inspect -f '{{.HostConfig.Memory}}' "$FIXTURE_ID")
REAL_NANO=$(docker inspect -f '{{.HostConfig.NanoCpus}}' "$FIXTURE_ID")
assert_eq "update resources: real docker HostConfig.Memory matches" "$MEM_LIMIT" "$REAL_MEM"
assert_eq "update resources: real docker HostConfig.NanoCpus matches" "$NANO_CPUS" "$REAL_NANO"
if [ "$REQ_STATUS" != "200" ]; then
    finding "PUT /system/containers/{id}/resources reliably fails (500) for any container that was created without an explicit memory-swap limit - i.e. every container this codebase itself ever creates (CreateContainer/CreateContainerOpts in docker/client.go never set Resources.MemorySwap). Root cause: UpdateResources (container_handler.go) only ever sets resources.Memory and resources.NanoCPUs, never MemorySwap, and the Docker daemon rejects a memory-limit update whose new value exceeds the container's current (unset, effectively 0) memory-swap limit with 'Memory limit should be smaller than already set memoryswap limit, update the memoryswap at the same time'. Reproduced directly here and independently via plain docker update --memory=<N> on a fresh alpine container with no prior --memory-swap. Effectively this endpoint cannot raise a container's memory limit at all in the common case, unless memory_swap is also supplied - and the request struct doesn't even accept that field."
fi

req POST "/system/containers/${FIXTURE_ID}/stop"
assert_eq "stop container: 200" "200" "$REQ_STATUS"
assert_eq "stop container: real docker state is not running" "false" "$(docker_running "$FIXTURE_ID")"

req POST "/system/containers/${FIXTURE_ID}/start"
assert_eq "start container: 200" "200" "$REQ_STATUS"
assert_eq "start container: real docker state is running again" "true" "$(docker_running "$FIXTURE_ID")"

STARTED_BEFORE=$(docker inspect -f '{{.State.StartedAt}}' "$FIXTURE_ID")
req POST "/system/containers/${FIXTURE_ID}/restart"
assert_eq "restart container: 200" "200" "$REQ_STATUS"
STARTED_AFTER=$(docker inspect -f '{{.State.StartedAt}}' "$FIXTURE_ID")
assert_eq "restart container: real docker state is running" "true" "$(docker_running "$FIXTURE_ID")"
assert_true "restart container: StartedAt actually changed (real restart, not a no-op)" "$([ "$STARTED_BEFORE" != "$STARTED_AFTER" ] && echo true || echo false)"

req DELETE "/system/containers/${FIXTURE_ID}"
assert_eq "remove container: 204" "204" "$REQ_STATUS"
assert_true "remove container: gone from real docker state" "$(docker inspect "$FIXTURE_ID" >/dev/null 2>&1 && echo false || echo true)"
DOCKER_FIXTURES=("${DOCKER_FIXTURES[@]/$FIXTURE_NAME/}")

req GET "/system/info"
AFTER_REMOVE_CONTAINERS=$(json_field "$REQ_BODY" "containers")
assert_eq "system info: containers count back to baseline after real removal" "$BASELINE_CONTAINERS" "${AFTER_REMOVE_CONTAINERS:-0}"

echo ""
echo "=== Nonexistent container id (expect graceful errors, never a crash) ==="
NOPE="0000000000000000000000000000000000000000000000000000000000dead"

req GET "/system/containers/${NOPE}"
assert_eq "inspect nonexistent: 404" "404" "$REQ_STATUS"

req POST "/system/containers/${NOPE}/start"
if [ "$REQ_STATUS" = "500" ]; then
    finding "POST /system/containers/{id}/start on a nonexistent id returns 500 rather than 404 (container_handler.go Start: h.docker.StartContainer's not-found error from the Docker daemon is not distinguished from other errors, just mapped to a blanket 500 - same shape as Stop/Restart/Remove/Logs/Stats/UpdateResources below). Doesn't crash the server, but violates this task's 'no unhandled panic/500 for a nonexistent container id' acceptance criterion literally."
fi
assert_true "start nonexistent: not a crash (got a real HTTP status)" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"

req POST "/system/containers/${NOPE}/stop"
[ "$REQ_STATUS" = "500" ] && finding "POST /system/containers/{id}/stop on a nonexistent id returns 500 rather than 404 (same root cause as start)."
assert_true "stop nonexistent: not a crash" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"

req POST "/system/containers/${NOPE}/restart"
[ "$REQ_STATUS" = "500" ] && finding "POST /system/containers/{id}/restart on a nonexistent id returns 500 rather than 404 (same root cause)."
assert_true "restart nonexistent: not a crash" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"

req DELETE "/system/containers/${NOPE}"
[ "$REQ_STATUS" = "500" ] && finding "DELETE /system/containers/{id} on a nonexistent id returns 500 rather than 404 (same root cause)."
assert_true "remove nonexistent: not a crash" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"

req GET "/system/containers/${NOPE}/logs"
[ "$REQ_STATUS" = "500" ] && finding "GET /system/containers/{id}/logs on a nonexistent id returns 500 rather than 404 (same root cause)."
assert_true "logs nonexistent: not a crash" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"

req GET "/system/containers/${NOPE}/stats"
[ "$REQ_STATUS" = "500" ] && finding "GET /system/containers/{id}/stats on a nonexistent id returns 500 rather than 404 (same root cause)."
assert_true "stats nonexistent: not a crash" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"

req PUT "/system/containers/${NOPE}/resources" '{"memory":1073741824}'
[ "$REQ_STATUS" = "500" ] && finding "PUT /system/containers/{id}/resources on a nonexistent id returns 500 rather than 404 (same root cause)."
assert_true "update resources nonexistent: not a crash" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"

HEALTH=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${PORT}/health")
assert_eq "server still healthy after a batch of nonexistent-id operations" "200" "$HEALTH"

echo ""
echo "=== /system/prune (response contract only - see header note on why a real daemon-wide prune is not exercised) ==="
BEFORE_PS=$(docker ps -aq | sort)
BEFORE_IMAGES=$(docker images -q | sort)
req POST "/system/prune" '{"containers":false,"images":false,"volumes":false,"networks":false,"all":false}'
assert_eq "prune (all flags false, explicit no-op): 200" "200" "$REQ_STATUS"
assert_eq "prune (no-op) response: status pruned" "pruned" "$(json_str_field "$REQ_BODY" "status")"
AFTER_PS=$(docker ps -aq | sort)
AFTER_IMAGES=$(docker images -q | sort)
assert_true "prune (no-op): real container set unchanged" "$([ "$BEFORE_PS" = "$AFTER_PS" ] && echo true || echo false)"
assert_true "prune (no-op): real image set unchanged" "$([ "$BEFORE_IMAGES" = "$AFTER_IMAGES" ] && echo true || echo false)"
finding "NOT exercised here (deliberately, see script header): POST /system/prune with any flag actually true. ContainerHandler.Prune's underlying PruneContainers/PruneImages/PruneVolumes/PruneNetworks (docker/client.go) all call the Docker API with empty filters.Args{} - there is no way to scope a prune to only this task's own fixtures, so a real invocation against this shared daemon would also delete other, unrelated stopped containers/dangling images/unused volumes and networks that happen to already exist on it. Also worth the architect's attention independent of this task: Prune's malformed-JSON-body fallback in container_handler.go's Prune handler - a body that fails to json-decode sets req.All = true instead of erroring - means an EMPTY POST body (a very easy mistake for any future frontend/client code to make) silently prunes everything rather than erroring; the safe default on a decode failure should arguably be to do nothing, not everything."

echo ""
echo "=== Resource limits: get/set + validation ==="
req GET "/system/resource-limits"
assert_eq "get resource-limits: 200" "200" "$REQ_STATUS"
BASELINE_MEM=$(json_field "$REQ_BODY" "memory_bytes")
BASELINE_CPU=$(json_field "$REQ_BODY" "nano_cpus")
assert_true "get resource-limits: migration-seeded 1 GiB / 1 CPU default" "$([ "$BASELINE_MEM" = "1073741824" ] && [ "$BASELINE_CPU" = "1000000000" ] && echo true || echo false)"

req PUT "/system/resource-limits" '{"memory_bytes":0,"nano_cpus":1000000000}'
assert_eq "set resource-limits: zero memory rejected (400)" "400" "$REQ_STATUS"

req PUT "/system/resource-limits" '{"memory_bytes":1073741824,"nano_cpus":-5}'
assert_eq "set resource-limits: negative nano_cpus rejected (400)" "400" "$REQ_STATUS"

DISTINCT_MEM=536870912    # 512 MiB - deliberately different from both the
DISTINCT_CPU=1500000000   # seeded default AND the hardcoded hardcoded fallback,
                          # so a sandbox picking this up is unambiguous.
req PUT "/system/resource-limits" "{\"memory_bytes\":${DISTINCT_MEM},\"nano_cpus\":${DISTINCT_CPU}}"
assert_eq "set resource-limits: 200" "200" "$REQ_STATUS"
assert_eq "set resource-limits: memory_bytes echoed" "$DISTINCT_MEM" "$(json_field "$REQ_BODY" "memory_bytes")"
assert_eq "set resource-limits: nano_cpus echoed" "$DISTINCT_CPU" "$(json_field "$REQ_BODY" "nano_cpus")"

req GET "/system/resource-limits"
assert_eq "get resource-limits after set: persisted memory_bytes" "$DISTINCT_MEM" "$(json_field "$REQ_BODY" "memory_bytes")"
assert_eq "get resource-limits after set: persisted nano_cpus" "$DISTINCT_CPU" "$(json_field "$REQ_BODY" "nano_cpus")"

echo ""
echo "=== New sandbox actually picks up the current default limit (real docker inspect) ==="
req POST "/projects" '{"name":"sandbox-rl-test-1","source_type":"local","domain":"sandbox-rl-test-1.local"}'
assert_eq "create project for sandbox test: 201" "201" "$REQ_STATUS"
PROJ1_ID=$(json_field "$REQ_BODY" "id")
assert_true "create project for sandbox test: got an id" "$([ -n "$PROJ1_ID" ] && echo true || echo false)"
SANDBOX_PROJECT_IDS+=("$PROJ1_ID")

WS1_LOG="${WORKDIR}/ws1.log"
: > "$WS1_LOG"
python3 "${WORKDIR}/wsclient.py" 127.0.0.1 "$PORT" "/api/projects/${PROJ1_ID}/agent/terminal?token=${TOKEN}" 5 >"$WS1_LOG" 2>&1 &
WS1_PID=$!

if wait_for_ws_upgrade "$WS1_LOG" 30; then
    log_ok "sandbox 1: terminal websocket upgraded (StartSandbox ran server-side)"
    PASS=$((PASS + 1))
    SANDBOX1_MEM=$(docker inspect -f '{{.HostConfig.Memory}}' "agent-${PROJ1_ID}" 2>/dev/null)
    SANDBOX1_NANO=$(docker inspect -f '{{.HostConfig.NanoCpus}}' "agent-${PROJ1_ID}" 2>/dev/null)
    assert_eq "sandbox 1: real container HostConfig.Memory matches the just-set default" "$DISTINCT_MEM" "${SANDBOX1_MEM:-}"
    assert_eq "sandbox 1: real container HostConfig.NanoCpus matches the just-set default" "$DISTINCT_CPU" "${SANDBOX1_NANO:-}"
else
    log_fail "sandbox 1: terminal websocket never upgraded"
    cat "$WS1_LOG" >&2
    FAIL=$((FAIL + 2))
fi
wait "$WS1_PID" 2>/dev/null
# StopAgent's docker-stop grace period (SIGTERM, wait up to 10s, then
# SIGKILL) means removal routinely takes just over 10s after the
# connection closes - poll rather than assume it's instant.
wait_for_container_removed "agent-${PROJ1_ID}" 20
assert_true "sandbox 1: container removed after connection closed (ephemeral lifecycle)" "$(docker inspect "agent-${PROJ1_ID}" >/dev/null 2>&1 && echo false || echo true)"
wait_for_network_removed "agent-net-${PROJ1_ID}" 10
assert_true "sandbox 1: dedicated network removed after connection closed" "$(docker network inspect "agent-net-${PROJ1_ID}" >/dev/null 2>&1 && echo false || echo true)"

echo ""
echo "=== Hardcoded 1 GiB / 1 CPU fallback when the resource-limit DB read fails ==="
req POST "/projects" '{"name":"sandbox-rl-test-2","source_type":"local","domain":"sandbox-rl-test-2.local"}'
assert_eq "create second project for fallback test: 201" "201" "$REQ_STATUS"
PROJ2_ID=$(json_field "$REQ_BODY" "id")
SANDBOX_PROJECT_IDS+=("$PROJ2_ID")

# Simulate ResourceLimitService.Get() failing (e.g. a transient DB error) by
# deleting the single settings row directly in this run's own isolated tmp
# DB (never the production/live one) - GetResourceLimit's Scan then returns
# sql.ErrNoRows, which is exactly the failure sandboxResources() is
# documented to guard against.
sqlite3 "$DB_PATH" "DELETE FROM resource_limits WHERE id = 1;"

WS2_LOG="${WORKDIR}/ws2.log"
: > "$WS2_LOG"
python3 "${WORKDIR}/wsclient.py" 127.0.0.1 "$PORT" "/api/projects/${PROJ2_ID}/agent/terminal?token=${TOKEN}" 5 >"$WS2_LOG" 2>&1 &
WS2_PID=$!

if wait_for_ws_upgrade "$WS2_LOG" 30; then
    log_ok "sandbox 2: terminal websocket upgraded (StartSandbox ran server-side)"
    PASS=$((PASS + 1))
    SANDBOX2_MEM=$(docker inspect -f '{{.HostConfig.Memory}}' "agent-${PROJ2_ID}" 2>/dev/null)
    SANDBOX2_NANO=$(docker inspect -f '{{.HostConfig.NanoCpus}}' "agent-${PROJ2_ID}" 2>/dev/null)
    assert_eq "sandbox 2: real container HostConfig.Memory is the hardcoded 1 GiB fallback (not the deleted setting, not distinct value)" "1073741824" "${SANDBOX2_MEM:-}"
    assert_eq "sandbox 2: real container HostConfig.NanoCpus is the hardcoded 1 CPU fallback" "1000000000" "${SANDBOX2_NANO:-}"
    assert_true "sandbox 2: server log confirms the fallback path was actually taken" "$(wait_for_log_line 'using hardcoded fallback' 2 && echo true || echo false)"
else
    log_fail "sandbox 2: terminal websocket never upgraded"
    cat "$WS2_LOG" >&2
    FAIL=$((FAIL + 3))
fi
wait "$WS2_PID" 2>/dev/null
wait_for_container_removed "agent-${PROJ2_ID}" 20
assert_true "sandbox 2: container removed after connection closed" "$(docker inspect "agent-${PROJ2_ID}" >/dev/null 2>&1 && echo false || echo true)"
wait_for_network_removed "agent-net-${PROJ2_ID}" 10
assert_true "sandbox 2: dedicated network removed after connection closed" "$(docker network inspect "agent-net-${PROJ2_ID}" >/dev/null 2>&1 && echo false || echo true)"

# Both sandbox containers/networks confirmed torn down above - clear the
# tracked ids so the cleanup trap's belt-and-suspenders removal has nothing
# left to do (it's a no-op either way, docker rm/network rm on an already
# gone name just errors harmlessly, but keep it accurate).
SANDBOX_PROJECT_IDS=()

echo ""
echo "----------------------------------------"
echo "TEST-003 containers/resources/system verification: ${PASS} passed, ${FAIL} failed"
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

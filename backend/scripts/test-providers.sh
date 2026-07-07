#!/usr/bin/env bash
#
# TEST-004: agent providers, encrypted API keys & egress whitelist
# verification.
#
# Same approach as TEST-001/002/003: builds the real backend binary
# (cmd/api) and runs it standalone (isolated tmp SQLite DB + data dir,
# random port), then drives the actual HTTP API with curl. The one thing
# this script cannot take on trust from the HTTP responses alone is
# whether an API key is really encrypted at rest, so for that specific
# check it also opens this run's own isolated tmp SQLite file directly
# with `sqlite3` and inspects the raw `api_keys` row - never the
# production DB, always this script's own throwaway one.
#
# Usage:
#   backend/scripts/test-providers.sh
#
# Env overrides:
#   PORT            port to run the backend on (default: random)
#   ADMIN_PASSWORD  password AutoSetup will provision on boot

set -uo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
WORKDIR=$(mktemp -d /tmp/tamga-test-providers.XXXXXX)
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
json_bool_field() {
    echo "$1" | grep -o "\"$2\":\(true\|false\)" | head -1 | cut -d':' -f2
}
count_occurrences() {
    echo "$1" | grep -o "$2" | wc -l | tr -d ' '
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
    CADDY_ADMIN_URL="http://127.0.0.1:1" \
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

echo ""
echo "=== Agent providers: list & get seeded builtin default ==="
req GET "/agent-providers"
assert_eq "list providers: 200" "200" "$REQ_STATUS"
assert_true "list providers: contains seeded builtin-opencode default" "$(echo "$REQ_BODY" | grep -q '"id":"builtin-opencode"' && echo true || echo false)"

req GET "/agent-providers/builtin-opencode"
assert_eq "get builtin provider: 200" "200" "$REQ_STATUS"
assert_true "get builtin provider: is_default true" "$(echo "$REQ_BODY" | grep -q '"is_default":true' && echo true || echo false)"

echo ""
echo "=== Agent providers: validation on create ==="
req POST "/agent-providers" '{"type":"docker"}'
assert_eq "create provider: missing name rejected (400)" "400" "$REQ_STATUS"

req POST "/agent-providers" '{"name":"Bad Provider","type":"k8s"}'
assert_eq "create provider: non-docker type rejected (400)" "400" "$REQ_STATUS"

req POST "/agent-providers" 'not-json'
assert_eq "create provider: malformed json rejected (400)" "400" "$REQ_STATUS"

echo ""
echo "=== Agent providers: full CRUD round-trip ==="
req POST "/agent-providers" '{"name":"Custom Claude Provider","type":"docker","image":"tamga-agent-custom","env":"{\"FOO\":\"bar\"}"}'
assert_eq "create provider: 201" "201" "$REQ_STATUS"
PROVIDER_ID=$(json_str_field "$REQ_BODY" "id")
assert_true "create provider: got a generated id" "$([ -n "$PROVIDER_ID" ] && echo true || echo false)"
assert_true "create provider: is_default defaults false" "$(echo "$REQ_BODY" | grep -q '"is_default":false' && echo true || echo false)"

req GET "/agent-providers/${PROVIDER_ID}"
assert_eq "get created provider: 200" "200" "$REQ_STATUS"
assert_eq "get created provider: name matches" "Custom Claude Provider" "$(json_str_field "$REQ_BODY" "name")"
assert_eq "get created provider: image matches" "tamga-agent-custom" "$(json_str_field "$REQ_BODY" "image")"

req GET "/agent-providers"
assert_true "list providers: now contains our created provider" "$(echo "$REQ_BODY" | grep -q "\"id\":\"${PROVIDER_ID}\"" && echo true || echo false)"

req PUT "/agent-providers/${PROVIDER_ID}" '{"name":"Renamed Provider","type":"docker","image":"tamga-agent-custom-v2"}'
assert_eq "update provider: 200" "200" "$REQ_STATUS"

req GET "/agent-providers/${PROVIDER_ID}"
assert_eq "update provider: name persisted" "Renamed Provider" "$(json_str_field "$REQ_BODY" "name")"
assert_eq "update provider: image persisted" "tamga-agent-custom-v2" "$(json_str_field "$REQ_BODY" "image")"

req PUT "/agent-providers/nonexistent-id-xyz" '{"name":"whatever","type":"docker"}'
assert_true "update nonexistent provider: not a crash (real HTTP status)" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"
[ "$REQ_STATUS" = "500" ] && finding "PUT /agent-providers/{id} on a nonexistent id returns 500 rather than 404 (agent_provider_handler.go Update -> AgentProviderService.Update wraps FindAgentProvider's sql.ErrNoRows in a generic error, and the handler maps every service error to a blanket 500)."

req DELETE "/agent-providers/${PROVIDER_ID}"
assert_eq "delete provider: 200" "200" "$REQ_STATUS"

req GET "/agent-providers/${PROVIDER_ID}"
assert_eq "get deleted provider: 404" "404" "$REQ_STATUS"

req DELETE "/agent-providers/${PROVIDER_ID}"
assert_true "delete already-deleted provider: not a crash (real HTTP status)" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"
assert_eq "delete already-deleted provider: idempotent 200 (no rows affected, no error surfaced)" "200" "$REQ_STATUS"

echo ""
echo "=== Agent providers: the seeded default cannot be deleted or modified ==="
req DELETE "/agent-providers/builtin-opencode"
assert_true "delete builtin default: not a crash" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"
req GET "/agent-providers/builtin-opencode"
assert_eq "delete builtin default: silently a no-op, builtin still present after" "200" "$REQ_STATUS"

req PUT "/agent-providers/builtin-opencode" '{"name":"Hijacked","type":"docker"}'
assert_true "update builtin default: rejected (not 200)" "$([ "$REQ_STATUS" != "200" ] && echo true || echo false)"
req GET "/agent-providers/builtin-opencode"
assert_eq "update builtin default: name unchanged" "Opencode (Built-in)" "$(json_str_field "$REQ_BODY" "name")"

echo ""
echo "=== Agent providers: is_default is client-settable with no exclusivity enforced ==="
# AgentProviderHandler.Create/Update decode domain.AgentProvider directly from
# the request body, including IsDefault, and neither the handler nor
# AgentProviderService.Create/Update ever clears is_default on other rows -
# so a client can mark a second provider as default without the builtin ever
# losing its own is_default=1. Confirmed directly below by counting
# is_default:true occurrences in the list response before/after.
req GET "/agent-providers"
DEFAULT_COUNT_BEFORE=$(count_occurrences "$REQ_BODY" '"is_default":true')

req POST "/agent-providers" '{"name":"Rogue Default Provider","type":"docker","image":"tamga-agent-rogue","is_default":true}'
assert_eq "create provider with is_default:true in body: 201 (accepted, not rejected)" "201" "$REQ_STATUS"
ROGUE_ID=$(json_str_field "$REQ_BODY" "id")

req GET "/agent-providers"
DEFAULT_COUNT_AFTER=$(count_occurrences "$REQ_BODY" '"is_default":true')
if [ "$DEFAULT_COUNT_AFTER" -gt "$DEFAULT_COUNT_BEFORE" ]; then
    finding "POST/PUT /agent-providers accepts an is_default:true field straight from the client request body (agent_provider_handler.go Create/Update json-decode domain.AgentProvider wholesale, and AgentProviderService.Create/Update never clear is_default on other rows). A client can create/update a second row with is_default=1 without the original builtin-opencode ever losing its own is_default=1 - confirmed here: ${DEFAULT_COUNT_BEFORE} row(s) had is_default:true before, ${DEFAULT_COUNT_AFTER} after creating one extra provider with is_default:true in the body. Since FindDefaultProvider (agent_provider_repo.go) does 'WHERE is_default = 1 LIMIT 1' with no explicit ORDER BY, which of several is_default rows gets picked by ResolveProvider (used to pick a new sandbox's provider) is effectively undefined/DB-order-dependent."
fi
req DELETE "/agent-providers/${ROGUE_ID}"

echo ""
echo "=== API keys: validation ==="
req POST "/system/api-keys" '{"provider":"anthropic"}'
assert_eq "set api key: missing key rejected (400)" "400" "$REQ_STATUS"

req POST "/system/api-keys" '{"key":"sk-test-abc123"}'
assert_eq "set api key: missing provider rejected (400)" "400" "$REQ_STATUS"

req POST "/system/api-keys" '{"provider":"not-a-real-provider","key":"sk-test-abc123"}'
assert_eq "set api key: unsupported provider rejected (400)" "400" "$REQ_STATUS"

echo ""
echo "=== API keys: set, list, and confirm the raw secret never comes back over HTTP ==="
RAW_KEY="sk-ant-super-secret-raw-value-$$-do-not-leak"
req POST "/system/api-keys" "{\"provider\":\"anthropic\",\"key\":\"${RAW_KEY}\",\"label\":\"my anthropic key\"}"
assert_eq "set api key: 200" "200" "$REQ_STATUS"
API_KEY_ID=$(json_str_field "$REQ_BODY" "id")
assert_true "set api key: got an id" "$([ -n "$API_KEY_ID" ] && echo true || echo false)"
assert_true "set api key: has_key true" "$(echo "$REQ_BODY" | grep -q '"has_key":true' && echo true || echo false)"
assert_true "set api key response: raw secret NOT present in response body" "$(echo "$REQ_BODY" | grep -qF "$RAW_KEY" && echo false || echo true)"
assert_true "set api key response: no key_enc/key field leaked at all" "$(echo "$REQ_BODY" | grep -qE '"key(_enc)?":' && echo false || echo true)"

req GET "/system/api-keys"
assert_eq "list api keys: 200" "200" "$REQ_STATUS"
assert_true "list api keys: contains our provider" "$(echo "$REQ_BODY" | grep -q '"provider":"anthropic"' && echo true || echo false)"
assert_true "list api keys: label persisted" "$(echo "$REQ_BODY" | grep -q '"label":"my anthropic key"' && echo true || echo false)"
assert_true "list api keys: raw secret NOT present anywhere in list response" "$(echo "$REQ_BODY" | grep -qF "$RAW_KEY" && echo false || echo true)"
assert_true "list api keys: no key_enc/key field leaked at all" "$(echo "$REQ_BODY" | grep -qE '"key(_enc)?":' && echo false || echo true)"

echo ""
echo "=== API keys: confirm encryption at rest by inspecting the raw SQLite row directly ==="
RAW_ROW=$(sqlite3 "$DB_PATH" "SELECT key_enc FROM api_keys WHERE id = '${API_KEY_ID}';")
assert_true "raw sqlite row: key_enc column is non-empty" "$([ -n "$RAW_ROW" ] && echo true || echo false)"
assert_true "raw sqlite row: stored value is NOT the plaintext raw key" "$([ "$RAW_ROW" != "$RAW_KEY" ] && echo true || echo false)"
assert_true "raw sqlite row: plaintext raw key is not even a substring of the stored value" "$(echo "$RAW_ROW" | grep -qF "$RAW_KEY" && echo false || echo true)"
assert_true "raw sqlite row: looks like nonce:ciphertext hex, not plaintext" "$(echo "$RAW_ROW" | grep -qE '^[0-9a-f]+:[0-9a-f]+$' && echo true || echo false)"
echo "  (raw stored value: ${RAW_ROW})"

echo ""
echo "=== API keys: setting the same provider again upserts (same id, new ciphertext) ==="
NEW_RAW_KEY="sk-ant-replacement-secret-$$"
req POST "/system/api-keys" "{\"provider\":\"anthropic\",\"key\":\"${NEW_RAW_KEY}\",\"label\":\"replaced label\"}"
assert_eq "re-set api key for same provider: 200" "200" "$REQ_STATUS"
assert_eq "re-set api key: id unchanged (upsert, not a duplicate row)" "$API_KEY_ID" "$(json_str_field "$REQ_BODY" "id")"

req GET "/system/api-keys"
COUNT_ANTHROPIC=$(count_occurrences "$REQ_BODY" '"provider":"anthropic"')
assert_eq "re-set api key: still exactly one anthropic entry (old one replaced, not duplicated)" "1" "$COUNT_ANTHROPIC"

NEW_RAW_ROW=$(sqlite3 "$DB_PATH" "SELECT key_enc FROM api_keys WHERE id = '${API_KEY_ID}';")
assert_true "re-set api key: raw stored ciphertext actually changed" "$([ "$NEW_RAW_ROW" != "$RAW_ROW" ] && echo true || echo false)"
assert_true "re-set api key: new raw value still not the new plaintext" "$([ "$NEW_RAW_ROW" != "$NEW_RAW_KEY" ] && echo true || echo false)"

echo ""
echo "=== API keys: delete ==="
req DELETE "/system/api-keys/${API_KEY_ID}"
assert_eq "delete api key: 204" "204" "$REQ_STATUS"

req GET "/system/api-keys"
assert_true "list api keys after delete: anthropic entry gone" "$(echo "$REQ_BODY" | grep -q '"provider":"anthropic"' && echo false || echo true)"

ROW_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM api_keys WHERE id = '${API_KEY_ID}';")
assert_eq "delete api key: raw sqlite row actually gone" "0" "$ROW_COUNT"

req DELETE "/system/api-keys/${API_KEY_ID}"
assert_true "delete already-deleted api key: not a crash (real HTTP status)" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"
assert_eq "delete already-deleted api key: idempotent 204" "204" "$REQ_STATUS"

echo ""
echo "=== Egress whitelist: seeded defaults ==="
req GET "/system/egress-whitelist"
assert_eq "list whitelist: 200" "200" "$REQ_STATUS"
assert_true "list whitelist: contains migration-seeded api.anthropic.com" "$(echo "$REQ_BODY" | grep -q '"domain":"api.anthropic.com"' && echo true || echo false)"
assert_true "list whitelist: contains migration-seeded api.openai.com" "$(echo "$REQ_BODY" | grep -q '"domain":"api.openai.com"' && echo true || echo false)"
assert_true "list whitelist: contains migration-seeded generativelanguage.googleapis.com" "$(echo "$REQ_BODY" | grep -q '"domain":"generativelanguage.googleapis.com"' && echo true || echo false)"

echo ""
echo "=== Egress whitelist: validation ==="
req POST "/system/egress-whitelist" '{"domain":""}'
assert_eq "add whitelist: empty domain rejected (400)" "400" "$REQ_STATUS"

req POST "/system/egress-whitelist" 'not-json'
assert_eq "add whitelist: malformed json rejected (400)" "400" "$REQ_STATUS"

echo ""
echo "=== Egress whitelist: add, normalization, list, delete round-trip ==="
req POST "/system/egress-whitelist" '{"domain":"  Test.Example.COM.  "}'
assert_eq "add whitelist domain: 201" "201" "$REQ_STATUS"
WHITELIST_ID=$(json_field "$REQ_BODY" "id")
assert_true "add whitelist domain: got a numeric id" "$([ -n "$WHITELIST_ID" ] && echo true || echo false)"
assert_eq "add whitelist domain: normalized (trimmed, lowercased, trailing dot stripped)" "test.example.com" "$(json_str_field "$REQ_BODY" "domain")"

req GET "/system/egress-whitelist"
assert_true "list whitelist: contains our normalized domain" "$(echo "$REQ_BODY" | grep -q '"domain":"test.example.com"' && echo true || echo false)"

req POST "/system/egress-whitelist" '{"domain":"test.example.com"}'
if [ "$REQ_STATUS" = "500" ]; then
    finding "POST /system/egress-whitelist with a domain that's already on the list returns a raw 500 rather than a 400/409 (whitelist_repo.go CreateWhitelistDomain's INSERT relies solely on the egress_whitelist.domain UNIQUE constraint (migration 000010) for de-duplication; WhitelistService.Add/the handler never pre-check for an existing entry, so the SQLite UNIQUE-constraint violation error propagates up as a generic error and gets mapped to a blanket 500 by the handler)."
fi
assert_true "add duplicate whitelist domain: not a crash (real HTTP status)" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"

req DELETE "/system/egress-whitelist/${WHITELIST_ID}"
assert_eq "delete whitelist domain: 204" "204" "$REQ_STATUS"

req GET "/system/egress-whitelist"
assert_true "list whitelist after delete: our domain gone" "$(echo "$REQ_BODY" | grep -q '"domain":"test.example.com"' && echo false || echo true)"

ROW_COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM egress_whitelist WHERE id = ${WHITELIST_ID};")
assert_eq "delete whitelist domain: raw sqlite row actually gone" "0" "$ROW_COUNT"

req DELETE "/system/egress-whitelist/${WHITELIST_ID}"
assert_true "delete already-deleted whitelist domain: not a crash (real HTTP status)" "$([ -n "$REQ_STATUS" ] && echo true || echo false)"
assert_eq "delete already-deleted whitelist domain: idempotent 204" "204" "$REQ_STATUS"

req DELETE "/system/egress-whitelist/not-a-number"
assert_eq "delete whitelist domain: non-numeric id rejected (400, not a crash)" "400" "$REQ_STATUS"

HEALTH=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${PORT}/health")
assert_eq "server still healthy after full test run" "200" "$HEALTH"

echo ""
echo "----------------------------------------"
echo "TEST-004 providers/keys/whitelist verification: ${PASS} passed, ${FAIL} failed"
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

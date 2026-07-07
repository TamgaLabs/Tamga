#!/bin/bash
set -e

# Smoke test script for Tamga stack
# Verifies backend health, frontend reachability, auth flow, and basic project CRUD
# Usage: ./scripts/smoke-test.sh [--up]
# Options:
#   --up    Bring up docker-compose stack before testing

BACKEND_HOST="${BACKEND_HOST:-localhost:8080}"
CADDY_HOST="${CADDY_HOST:-https://localhost}"
API_URL="${CADDY_HOST}/api"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}"
# curl options: -k for insecure SSL, -s for silent, -f for fail on HTTP error
CURL_OPTS="-k -s"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_step() {
    echo -e "${YELLOW}→${NC} $1"
}

log_pass() {
    echo -e "${GREEN}✓${NC} $1"
}

log_fail() {
    echo -e "${RED}✗${NC} $1" >&2
}

# Check if --up flag is provided
BRING_UP_STACK=false
if [[ "${1:-}" == "--up" ]]; then
    BRING_UP_STACK=true
fi

# Bring up stack if requested
if [ "$BRING_UP_STACK" = true ]; then
    log_step "Bringing up docker-compose stack..."
    docker compose -f "$(dirname "$0")/../docker-compose.yml" up -d
    log_pass "Stack brought up"
fi

# Wait for backend to be reachable
log_step "Waiting for backend health endpoint..."
MAX_RETRIES=30
RETRY_INTERVAL=2
RETRY_COUNT=0

# Health check uses docker exec on the running backend container
# (backend already has wget available, avoiding package manager calls)
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    HEALTH_CHECK=$(docker exec tamga-backend-1 wget -O- -q http://localhost:8080/health 2>&1 || true)
    if echo "$HEALTH_CHECK" | grep -q '"status":"ok"'; then
        log_pass "Backend health check passed"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
        sleep $RETRY_INTERVAL
    fi
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    log_fail "Backend health check failed: backend not reachable after $((MAX_RETRIES * RETRY_INTERVAL)) seconds"
    exit 1
fi

# Check frontend reachability through Caddy
log_step "Checking frontend reachability through Caddy..."
if curl $CURL_OPTS -f "${CADDY_HOST}" >/dev/null 2>&1; then
    log_pass "Frontend is reachable through Caddy"
else
    log_fail "Frontend health check failed: cannot reach ${CADDY_HOST}"
    exit 1
fi

# Check auth status
log_step "Checking auth status..."
AUTH_STATUS=$(curl $CURL_OPTS "${API_URL}/auth/status" | grep -o '"setup":[^,}]*')
if [ -z "$AUTH_STATUS" ]; then
    log_fail "Auth status check failed: invalid response from auth/status endpoint"
    exit 1
fi
log_pass "Auth status check passed"

# Setup auth if needed
if echo "$AUTH_STATUS" | grep -q '"setup":false'; then
    log_step "Setting up authentication..."
    SETUP_RESPONSE=$(curl $CURL_OPTS -X POST "${API_URL}/auth/setup" \
        -H "Content-Type: application/json" \
        -d "{\"password\":\"${ADMIN_PASSWORD}\"}")

    if echo "$SETUP_RESPONSE" | grep -q '"success"'; then
        log_pass "Authentication setup completed"
    else
        log_fail "Auth setup failed: $SETUP_RESPONSE"
        exit 1
    fi
else
    log_pass "Authentication already set up"
fi

# Login to get token
log_step "Logging in to get authentication token..."
LOGIN_RESPONSE=$(curl $CURL_OPTS -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"password\":\"${ADMIN_PASSWORD}\"}")

TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
if [ -z "$TOKEN" ]; then
    log_fail "Login failed: could not extract token from response"
    log_fail "Response: $LOGIN_RESPONSE"
    exit 1
fi
log_pass "Login successful, obtained authentication token"

# Create a test project
log_step "Creating test project..."
TEST_PROJECT_NAME="smoke-test-$(date +%s)"
TEST_DOMAIN="${TEST_PROJECT_NAME}.localhost"
CREATE_RESPONSE=$(curl $CURL_OPTS -X POST "${API_URL}/projects" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "{\"name\":\"${TEST_PROJECT_NAME}\",\"domain\":\"${TEST_DOMAIN}\",\"repo_url\":\"https://github.com/example/test\"}")

PROJECT_ID=$(echo "$CREATE_RESPONSE" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)
if [ -z "$PROJECT_ID" ]; then
    log_fail "Project creation failed: could not extract project ID from response"
    log_fail "Response: $CREATE_RESPONSE"
    exit 1
fi
log_pass "Test project created (ID: $PROJECT_ID)"

# List projects and verify test project appears
log_step "Listing projects to verify test project..."
LIST_RESPONSE=$(curl $CURL_OPTS "${API_URL}/projects" \
    -H "Authorization: Bearer ${TOKEN}")

if echo "$LIST_RESPONSE" | grep -q "\"id\":${PROJECT_ID}[,}]"; then
    log_pass "Test project found in project list"
else
    log_fail "Project listing failed: test project not found in list"
    log_fail "Response: $LIST_RESPONSE"
    exit 1
fi

# Delete the test project
log_step "Deleting test project..."
DELETE_RESPONSE=$(curl $CURL_OPTS -X DELETE "${API_URL}/projects/${PROJECT_ID}" \
    -H "Authorization: Bearer ${TOKEN}")

# Give it time for deletion to complete (may take a few seconds)
sleep 3

# Verify deletion
VERIFY_RESPONSE=$(curl $CURL_OPTS "${API_URL}/projects" \
    -H "Authorization: Bearer ${TOKEN}")

if ! echo "$VERIFY_RESPONSE" | grep -q "\"id\":${PROJECT_ID}[,}]"; then
    log_pass "Test project deleted successfully"
else
    log_fail "Project deletion failed: test project still appears in list"
    exit 1
fi

# Success!
echo ""
log_pass "All smoke tests passed!"
exit 0

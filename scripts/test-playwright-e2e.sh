#!/usr/bin/env bash
# Explicit two-stage browser fixture lifecycle. Prepare is builder-only and
# hands off one private target; test only consumes that handoff. Neither mode
# can infer or select the developer's shared Compose stack.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
environment_helper="$repo_root/scripts/sdlc-environment.sh"
compose_file="$repo_root/scripts/playwright-compose.yml"
mode=${E2E_MODE:-}

require_disposable_owner() {
  [[ ${E2E_DISPOSABLE:-} == 1 ]] || {
    echo "browser E2E requires E2E_DISPOSABLE=1 for its local fixture" >&2
    exit 2
  }
  for variable in E2E_BASE_URL E2E_ADMIN_PASSWORD E2E_OWNED_STACK; do
    [[ -z ${!variable:-} ]] || {
      echo "browser E2E refuses supplied $variable; use the owned fixture handoff" >&2
      exit 2
    }
  done
}

require_builder_manifest() {
  [[ -n ${E2E_MANIFEST:-} && -f $E2E_MANIFEST ]] || {
    echo "browser E2E requires existing E2E_MANIFEST from the TEST-023 builder" >&2
    exit 2
  }
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "browser E2E requires $1 on PATH" >&2
    exit 2
  }
}

pick_port() {
  node - <<'NODE'
const net = require("net");
const server = net.createServer();
server.listen(0, "127.0.0.1", () => {
  process.stdout.write(String(server.address().port));
  server.close();
});
NODE
}

record() {
  (cd "$repo_root" && "$environment_helper" record "$E2E_MANIFEST" "$1" "$2")
}

record_containers() {
  local container
  for container in $("${compose[@]}" ps --all --quiet 2>/dev/null || true); do
    record container "$container"
  done
}

prepare() {
  require_command docker
  require_command node
  require_command curl
  docker compose version >/dev/null

  local run_id project_name fixture_root http_port https_port handoff_file
  run_id="$(date +%s)-$$"
  project_name="tamga-e2e-$run_id"
  fixture_root=$(mktemp -d "/tmp/tamga-sdlc-playwright.$run_id.XXXXXX")
  http_port=$(pick_port)
  https_port=$(pick_port)
  while [[ "$http_port" == "$https_port" ]]; do https_port=$(pick_port); done

  export E2E_HTTP_PORT="$http_port"
  export E2E_HTTPS_PORT="$https_port"
  export E2E_DATA_DIR="$fixture_root/data"
  export E2E_DYNAMIC_DIR="$fixture_root/traefik-dynamic"
  export E2E_TRAEFIK_DATA_DIR="$fixture_root/traefik-data"
  export E2E_BACKEND_ENV_FILE="$fixture_root/backend.env"
  export E2E_NETWORK_NAME="$project_name-network"
  export E2E_IMAGE_PREFIX="$project_name"
  export E2E_ADMIN_PASSWORD="tamga-e2e-local-fixture"
  export E2E_BASE_URL="https://localhost:$https_port"
  export E2E_OWNED_STACK=1
  compose=(docker compose --project-name "$project_name" --file "$compose_file")
  trap record_containers EXIT

  mkdir -p "$E2E_DATA_DIR" "$E2E_DYNAMIC_DIR" "$E2E_TRAEFIK_DATA_DIR"
  cp "$repo_root/traefik/dynamic/tamga.yml" "$E2E_DYNAMIC_DIR/tamga.yml"
  umask 077
  cat > "$E2E_BACKEND_ENV_FILE" <<EOF
ADMIN_PASSWORD=$E2E_ADMIN_PASSWORD
JWT_SECRET=tamga-e2e-fixture-jwt-secret-not-for-production
DB_PATH=/data/tamga.db
DATA_DIR=/data
PORT=8080
TRAEFIK_DYNAMIC_DIR=/etc/traefik/dynamic
TRAEFIK_METRICS_URL=http://traefik:8080/metrics
EOF

  record note "playwright-disposable-fixture:$project_name"
  record file "$fixture_root"
  record network "$E2E_NETWORK_NAME"
  record image "$E2E_IMAGE_PREFIX-backend"
  record image "$E2E_IMAGE_PREFIX-frontend"

  echo "Starting disposable Playwright fixture $project_name on HTTPS port $https_port"
  "${compose[@]}" up --detach --build
  record_containers
  for _ in $(seq 1 45); do
    curl --fail --silent --show-error --insecure --max-time 2 "$E2E_BASE_URL/api/auth/status" >/dev/null 2>&1 && break
    sleep 2
  done
  curl --fail --silent --show-error --insecure --max-time 5 "$E2E_BASE_URL/api/auth/status" >/dev/null || {
    echo "Disposable Playwright fixture did not become healthy: $E2E_BASE_URL" >&2
    exit 1
  }

  handoff_file="$fixture_root/handoff.env"
  cat > "$handoff_file" <<EOF
E2E_HANDOFF_VERSION=1
E2E_HANDOFF_MANIFEST=$E2E_MANIFEST
E2E_BASE_URL=$E2E_BASE_URL
E2E_ADMIN_PASSWORD=$E2E_ADMIN_PASSWORD
E2E_OWNED_STACK=1
EOF
  echo "E2E_HANDOFF_FILE=$handoff_file"
}

load_handoff() {
  local handoff_file=${E2E_HANDOFF_FILE:-} fixture_root key value handoff_manifest="" base_url="" password="" owned="" version=""
  case "$handoff_file" in /tmp/tamga-sdlc-playwright.*/handoff.env) ;; *)
    echo "test-e2e requires E2E_HANDOFF_FILE from the builder's private fixture" >&2
    exit 2
  esac
  [[ -f "$handoff_file" ]] || { echo "missing E2E_HANDOFF_FILE: $handoff_file" >&2; exit 2; }
  fixture_root=${handoff_file%/handoff.env}
  awk -F $'\t' -v fixture="$fixture_root" '
    $1 == "file" && $2 == fixture { found = 1 }
    END { exit(found ? 0 : 1) }
  ' "$E2E_MANIFEST" || {
    echo "browser fixture handoff is not recorded in E2E_MANIFEST" >&2
    exit 2
  }
  while IFS='=' read -r key value; do
    case "$key" in
      E2E_HANDOFF_VERSION) version=$value ;;
      E2E_HANDOFF_MANIFEST) handoff_manifest=$value ;;
      E2E_BASE_URL) base_url=$value ;;
      E2E_ADMIN_PASSWORD) password=$value ;;
      E2E_OWNED_STACK) owned=$value ;;
      *) echo "invalid browser fixture handoff" >&2; exit 2 ;;
    esac
  done < "$handoff_file"
  [[ $version == 1 && $handoff_manifest == "$E2E_MANIFEST" && $owned == 1 && $base_url =~ ^https://localhost:[0-9]+$ && -n $password ]] || {
    echo "invalid browser fixture handoff for E2E_MANIFEST" >&2
    exit 2
  }
  export E2E_BASE_URL="$base_url"
  export E2E_ADMIN_PASSWORD="$password"
  export E2E_OWNED_STACK=1
}

test_fixture() {
  require_command curl
  load_handoff
  curl --fail --silent --show-error --insecure --max-time 5 "$E2E_BASE_URL/api/auth/status" >/dev/null || {
    echo "Builder handoff fixture is not healthy: $E2E_BASE_URL" >&2
    exit 1
  }
  cd "$repo_root/frontend"
  npm run test:e2e
}

require_disposable_owner
require_builder_manifest
case "$mode" in
  prepare) prepare ;;
  test) test_fixture ;;
  *) echo "browser E2E requires E2E_MODE=prepare (builder) or E2E_MODE=test (tester)" >&2; exit 2 ;;
esac

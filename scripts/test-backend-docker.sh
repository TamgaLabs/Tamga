#!/usr/bin/env bash
set -euo pipefail

# Runs only on a fresh, job-owned Docker daemon. The integration tests use
# fixed terminal fixture names, so accepting a shared daemon would risk
# touching a developer's Tamga stack.
if [[ "${TAMGA_TEST_DOCKER_OWNED:-}" != "1" ]]; then
  echo "test-backend-docker requires TAMGA_TEST_DOCKER_OWNED=1 and a fresh, job-owned Docker daemon" >&2
  exit 2
fi

for command in docker go; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "test-backend-docker requires $command on PATH" >&2
    exit 2
  }
done

docker info >/dev/null 2>&1 || {
  echo "test-backend-docker requires a reachable Docker daemon" >&2
  exit 2
}
docker compose version >/dev/null 2>&1 || {
  echo "test-backend-docker requires the Docker Compose v2 plugin" >&2
  exit 2
}

for container in agent-1 tamga-egress-proxy; do
  if docker container inspect "$container" >/dev/null 2>&1; then
    echo "test-backend-docker refuses a non-fresh daemon: container $container already exists" >&2
    exit 2
  fi
done
if docker network inspect agent-net-1 >/dev/null 2>&1; then
  echo "test-backend-docker refuses a non-fresh daemon: network agent-net-1 already exists" >&2
  exit 2
fi
for image in tamga-agent tamga-egress-proxy; do
  if docker image inspect "$image" >/dev/null 2>&1; then
    echo "test-backend-docker refuses a non-fresh daemon: image $image already exists" >&2
    exit 2
  fi
done

# Both exact tags were absent immediately before this lane took ownership.
# Register the trap before Compose starts so a partial two-image build cannot
# leave, or cause later deletion of, an unowned tag.
images_preflighted=true
cleanup() {
  docker rm -f agent-1 >/dev/null 2>&1 || true
  docker network disconnect -f agent-net-1 tamga-egress-proxy >/dev/null 2>&1 || true
  docker network rm agent-net-1 >/dev/null 2>&1 || true
  docker rm -f tamga-egress-proxy >/dev/null 2>&1 || true
  if [[ "$images_preflighted" == true ]]; then
    docker image rm -f tamga-agent tamga-egress-proxy >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

docker compose build agent egress-proxy
for image in tamga-agent tamga-egress-proxy; do
  docker image inspect "$image" >/dev/null 2>&1 || {
    echo "test-backend-docker expected compose to build image $image" >&2
    exit 1
  }
done

TAMGA_TEST_DOCKER=1 go test -tags=integration -p 1 ./backend/...
./backend/scripts/test-e2e-critical-path.sh

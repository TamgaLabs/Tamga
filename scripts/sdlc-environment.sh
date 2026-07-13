#!/usr/bin/env bash
# Safe lifecycle helper for the Tamga SDLC builder agent.
# It never tears down the shared compose stack. Cleanup only acts on exact
# resources recorded by this test cycle in a manifest.
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
usage:
  sdlc-environment.sh prepare MANIFEST REPO_ROOT [--smoke] [--dry-run]
  sdlc-environment.sh smoke MANIFEST REPO_ROOT [--dry-run]
  sdlc-environment.sh record MANIFEST TYPE NAME
  sdlc-environment.sh show MANIFEST
  sdlc-environment.sh cleanup MANIFEST

TYPE is one of: container, network, image, volume, file, note.
EOF
  exit 2
}

[[ $# -ge 2 ]] || usage
command=$1
manifest=$2

require_manifest() {
  [[ -f "$manifest" ]] || { echo "missing manifest: $manifest" >&2; exit 1; }
}

valid_name() {
  [[ -n "$1" && "$1" != *$'\n'* && "$1" != *$'\t'* ]]
}

record() {
  local type=$1 name=$2
  case "$type" in container|network|image|volume|file|note) ;; *) usage ;; esac
  require_manifest
  valid_name "$name" || { echo "invalid resource name" >&2; exit 1; }
  printf '%s\t%s\n' "$type" "$name" >> "$manifest"
}

run() {
  if [[ ${DRY_RUN:-false} == true ]]; then
    printf '+ '
    printf '%q ' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

prepare() {
  local repo_root=$1
  shift
  local run_smoke=false
  DRY_RUN=false
  while [[ $# -gt 0 ]]; do
    case "$1" in --smoke) run_smoke=true ;; --dry-run) DRY_RUN=true ;; *) usage ;; esac
    shift
  done

  [[ -d "$repo_root" && -f "$repo_root/docker-compose.yml" ]] || {
    echo "invalid Tamga repository root: $repo_root" >&2
    exit 1
  }
  [[ ! -e "$manifest" ]] || { echo "manifest already exists: $manifest" >&2; exit 1; }
  umask 077
  : > "$manifest"
  record note "shared-compose-stack-is-never-task-owned"

  if [[ $DRY_RUN == true ]]; then
    echo "dry-run: would inspect and, if needed, start the shared Tamga stack"
    run make -C "$repo_root" up
    echo "STACK_READY=true"
    return 0
  fi

  local compose=(docker compose -f "$repo_root/docker-compose.yml")
  local backend_id traefik_id frontend_id health=""
  backend_id=$("${compose[@]}" ps -q backend)
  traefik_id=$("${compose[@]}" ps -q traefik)
  frontend_id=$("${compose[@]}" ps -q frontend)

  if [[ -z "$backend_id" || -z "$traefik_id" || -z "$frontend_id" ]]; then
    echo "Shared Tamga stack is incomplete; starting it with make up..."
    run make -C "$repo_root" up
  else
    echo "Shared Tamga stack is already running; it will not be recorded or removed."
  fi

  local attempt
  for attempt in $(seq 1 30); do
    backend_id=$("${compose[@]}" ps -q backend)
    if [[ -n "$backend_id" ]]; then
      health=$(docker exec "$backend_id" wget -O- -q http://localhost:8080/health 2>/dev/null || true)
      [[ $health == *'"status":"ok"'* ]] && break
    fi
    sleep 2
  done

  [[ $health == *'"status":"ok"'* ]] || {
    echo "backend did not become healthy within 60 seconds" >&2
    exit 1
  }

  echo "STACK_READY=true"
  echo "API_URL=https://localhost/api"
  echo "FRONTEND_URL=https://localhost"
  echo "BACKEND_CONTAINER=$backend_id"
  echo "TRAEFIK_CONTAINER=$("${compose[@]}" ps -q traefik)"
  echo "FRONTEND_CONTAINER=$("${compose[@]}" ps -q frontend)"

  [[ $run_smoke == true ]] && smoke "$repo_root" false
}

smoke() {
  local repo_root=$1 dry_run=$2
  if [[ $dry_run == true ]]; then
    printf '+ %q\n' "$repo_root/scripts/smoke-test.sh"
    return 0
  fi
  "$repo_root/scripts/smoke-test.sh"
}

cleanup() {
  require_manifest
  local failed=false type name
  while IFS=$'\t' read -r type name; do
    [[ -n "$type" && -n "$name" ]] || continue
    case "$type" in
      container) docker container inspect "$name" >/dev/null 2>&1 && docker rm -f -- "$name" || true ;;
      network) docker network inspect "$name" >/dev/null 2>&1 && docker network rm -- "$name" || true ;;
      image) docker image inspect "$name" >/dev/null 2>&1 && docker image rm -- "$name" || true ;;
      volume) docker volume inspect "$name" >/dev/null 2>&1 && docker volume rm -- "$name" || true ;;
      file)
        case "$name" in
          # A task may record one private fixture directory below /tmp.  It
          # is still an exact manifest entry; never infer a directory from a
          # compose project name or a prefix during cleanup.
          /tmp/tamga-sdlc-*|"$PWD"/.sdlc-tmp/*) [[ -e "$name" ]] && rm -rf -- "$name" ;;
          *) echo "refusing unsafe file path: $name" >&2; failed=true ;;
        esac
        ;;
      note) ;;
      *) echo "unknown manifest resource type: $type" >&2; failed=true ;;
    esac
  done < <(tac "$manifest")

  $failed && exit 1
  echo "TASK_RESOURCES_CLEANED=true"
}

case "$command" in
  prepare) [[ $# -ge 3 ]] || usage; prepare "$3" "${@:4}" ;;
  smoke)
    [[ $# -ge 3 ]] || usage
    dry_run=false
    [[ ${4:-} == --dry-run ]] && dry_run=true
    smoke "$3" "$dry_run"
    ;;
  record) [[ $# -eq 4 ]] || usage; record "$3" "$4" ;;
  show) require_manifest; sort -u "$manifest" ;;
  cleanup) [[ $# -eq 2 ]] || usage; cleanup ;;
  *) usage ;;
esac

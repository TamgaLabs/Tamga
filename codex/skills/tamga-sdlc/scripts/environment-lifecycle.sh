#!/usr/bin/env bash
# Compatibility wrapper: the project-local script owns actual build/teardown.
set -euo pipefail

repo_root=${TAMGA_REPO_ROOT:-$PWD}
exec "$repo_root/scripts/sdlc-environment.sh" "$@"

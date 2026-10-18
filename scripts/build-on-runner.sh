#!/usr/bin/env bash
# Host orchestration only: all application compilation happens inside Docker.
set -euo pipefail
repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
cd "$repo_root"
image=oshell-desktop-ci:ubuntu24.04-v2
mode=${1:-package}
container_name="oshell-build-${GITHUB_RUN_ID:-local-$$}-${GITHUB_RUN_ATTEMPT:-1}-$mode"
toolchain=$(go env GOROOT)
owner=$(stat -c '%u:%g' "$repo_root")

docker build --pull --build-arg "CACHE_EPOCH=$(date -u +%Y-%m)" \
  --tag "$image" --file .github/docker/build.Dockerfile .github/docker

# Scope caches to this trusted repository; do not mount the Docker socket or credentials.
docker volume create oshell-desktop-build-cache-v1 >/dev/null
cleanup() {
  docker rm --force "$container_name" >/dev/null 2>&1 || true
  # Also recover ownership when a cancelled job forcibly stops the build container.
  docker run --rm --mount "type=bind,source=$repo_root,target=/workspace" \
    "$image" chown -R --no-dereference "$owner" /workspace
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
docker run --rm --init --name "$container_name" \
  --mount "type=bind,source=$repo_root,target=/workspace" \
  --mount "type=bind,source=$toolchain,target=/opt/go,readonly" \
  --mount type=volume,source=oshell-desktop-build-cache-v1,target=/cache \
  --env "OSHELL_WORKSPACE_OWNER=$owner" \
  --env GITHUB_SHA --env GITHUB_RUN_ID \
  "$image" bash scripts/build-container.sh "$mode"

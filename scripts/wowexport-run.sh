#!/usr/bin/env bash
set -euo pipefail

HOST_PORT="${WOWEXPORT_PORT:-17751}"
CONTAINER_NAME="${WOWEXPORT_NAME:-wow.export}"

mkdir -p ./exports

# Remove any existing container with the same name to avoid port/name conflicts
docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true

docker run -d --name "${CONTAINER_NAME}" \
  -p "${HOST_PORT}:17751" \
  -e DISPLAY=:99 \
  -w /exports \
  -u $(id -u):$(id -g) \
  -v "$(pwd)/exports:/exports" \
  wow.export:latest
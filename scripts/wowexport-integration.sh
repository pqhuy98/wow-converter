#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="wow-export:latest"
CONTAINER_NAME="wow-export-test"
HOST_PORT="17751"

RUNTIME=docker
if ! command -v docker >/dev/null 2>&1; then
  if command -v podman >/dev/null 2>&1; then
    RUNTIME=podman
  else
    echo "ERROR: Neither docker nor podman is installed. Please install one to run the integration test." >&2
    exit 127
  fi
fi

echo "[1/4] Building image ${IMAGE_TAG} with ${RUNTIME}..."
${RUNTIME} build -f /workspace/Dockerfile.wowexport -t "${IMAGE_TAG}" /workspace | cat

echo "[2/4] Running container ${CONTAINER_NAME} with ${RUNTIME}..."
${RUNTIME} rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
${RUNTIME} run -d --name "${CONTAINER_NAME}" -p "${HOST_PORT}:17751" "${IMAGE_TAG}" | cat

echo "[3/4] Waiting for RPC to accept connections..."
for i in $(seq 1 60); do
  if python3 /workspace/scripts/wowexport-healthcheck.py 127.0.0.1 "${HOST_PORT}" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "[4/4] Running healthcheck..."
python3 /workspace/scripts/wowexport-healthcheck.py 127.0.0.1 "${HOST_PORT}"

echo "SUCCESS: wow.export container is up and responding to RCP."


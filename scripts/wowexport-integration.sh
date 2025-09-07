#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="wow-export:latest"
CONTAINER_NAME="wow-export-test"
HOST_PORT="17751"

RUNTIME=docker
SUDO=
if ! command -v docker >/dev/null 2>&1; then
  if command -v podman >/dev/null 2>&1; then
    RUNTIME=podman
    # prefer rootful podman to avoid rootless networking issues
    if command -v sudo >/dev/null 2>&1; then
      SUDO="sudo -E"
    fi
  else
    echo "ERROR: Neither docker nor podman is installed. Please install one to run the integration test." >&2
    exit 127
  fi
fi

echo "[1/4] Building image ${IMAGE_TAG} with ${RUNTIME}..."
if [ "${RUNTIME}" = "podman" ]; then
  ${SUDO} ${RUNTIME} build --network=host -f /workspace/Dockerfile.wowexport -t "${IMAGE_TAG}" /workspace | cat
else
  ${RUNTIME} build -f /workspace/Dockerfile.wowexport -t "${IMAGE_TAG}" /workspace | cat
fi

echo "[2/4] Running container ${CONTAINER_NAME} with ${RUNTIME}..."
if [ "${RUNTIME}" = "podman" ]; then
  ${SUDO} ${RUNTIME} rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
  ${SUDO} ${RUNTIME} run --network=host -d --name "${CONTAINER_NAME}" -e WOWEXPORT_PORT="${HOST_PORT}" "${IMAGE_TAG}" | cat
else
  ${RUNTIME} rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
  ${RUNTIME} run -d --name "${CONTAINER_NAME}" -p "${HOST_PORT}:17751" "${IMAGE_TAG}" | cat
fi

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


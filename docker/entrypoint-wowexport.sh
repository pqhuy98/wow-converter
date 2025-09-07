#!/usr/bin/env bash
set -euo pipefail

# Optional: allow changing RPC port via env var
WOWEXPORT_PORT="${WOWEXPORT_PORT:-17751}"
export DISPLAY=${DISPLAY:-:99}

# Start virtual framebuffer
Xvfb ${DISPLAY} -screen 0 1280x800x24 -nolisten unix -nolisten tcp &
XVFB_PID=$!
trap "kill ${XVFB_PID} >/dev/null 2>&1 || true" EXIT

# Run wow.export from its directory so bundled libs (e.g., libnw.so) are found
cd /opt/wow.export
exec ./wow.export \
  --user-data-dir=/tmp/wowexport \
  --crash-dumps-dir=/tmp/wowexport/crashes \
  --disable-dev-shm-usage \
  --disable-gpu \
  --no-sandbox \
  "$@"


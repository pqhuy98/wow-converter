#!/usr/bin/env bash
set -euo pipefail

# Optional: allow changing RPC port via env var
WOWEXPORT_PORT="${WOWEXPORT_PORT:-17751}"

# Seed user config to enable RCP on desired port in wow.export data path
DATA_DIR="/root/.config/wow.export"
mkdir -p "${DATA_DIR}"
# Merge with defaults if present; minimal config to flip RCP on
cat > "${DATA_DIR}/config.json" <<EOF
{
	"rcpEnabled": true,
	"rcpPort": ${WOWEXPORT_PORT},
	"exportDirectory": "/opt/wow.export/exports",
	"overwriteFiles": true
}
EOF

mkdir -p /opt/wow.export/exports

# Use xvfb-run to manage DISPLAY and Xauthority automatically. Pass Chromium flags for root/headless.
export NW_DISABLE_GPU=1
exec xvfb-run -a -s "-screen 0 1280x800x24" \
  /opt/wow.export/wow.export \
  --no-sandbox \
  --disable-gpu \
  --disable-software-rasterizer \
  --disable-dev-shm-usage \
  "$@"


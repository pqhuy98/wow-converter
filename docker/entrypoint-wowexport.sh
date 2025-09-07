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

# Start wow.export under Xvfb with stable flags
# Favour software rendering and X11
export NO_AT_BRIDGE=1
export LIBGL_ALWAYS_SOFTWARE=1
export MESA_LOADER_DRIVER_OVERRIDE=llvmpipe
export GDK_BACKEND=x11
export LANG=C.UTF-8

exec xvfb-run -a -s "-screen 0 1280x800x24 -nolisten tcp" \
  /opt/wow.export/wow.export \
  --no-sandbox \
  --disable-gpu \
  --disable-dev-shm-usage \
  --disable-features=UseOzonePlatform \
  --ozone-platform=x11


#!/usr/bin/env bash
set -euo pipefail

# Cold CASC load after restart can take several minutes (archives + encoding table from CDN).
CASC_READY_WAIT_SECS="${CASC_READY_WAIT_SECS:-600}"
CASC_READY_POLL_SECS=2
CASC_READY_MAX_ATTEMPTS=$((CASC_READY_WAIT_SECS / CASC_READY_POLL_SECS))

systemctl stop wow-converter.service
bash /root/clean.sh
systemctl start wow-converter.service
for ((i = 1; i <= CASC_READY_MAX_ATTEMPTS; i++)); do
  if curl -sf http://127.0.0.1:3001/ >/dev/null 2>&1 &&
    curl -sf http://127.0.0.1:3001/api/wow-config/status |
      python3 -c 'import json,sys; raise SystemExit(0 if json.load(sys.stdin).get("cascLoaded") else 1)'
  then
    break
  fi
  [[ "${i}" == "${CASC_READY_MAX_ATTEMPTS}" ]] && {
    echo "bundled wow-converter or CASC not ready after ${CASC_READY_WAIT_SECS}s" >&2
    exit 1
  }
  sleep "${CASC_READY_POLL_SECS}"
done

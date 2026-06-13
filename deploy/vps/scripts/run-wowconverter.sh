#!/usr/bin/env bash
set -euo pipefail

export PATH="/root/.bun/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

WOW_DATA_PORT="${WOW_DATA_SERVER_PORT:-17753}"

for ((i = 1; i <= 90; i++)); do
  curl -sf "http://127.0.0.1:${WOW_DATA_PORT}/rest/getConfig" >/dev/null 2>&1 && break
  [[ "${i}" == "90" ]] && { echo "wow-data-server REST not ready" >&2; exit 1; }
  sleep 2
done

cd /root/wow-converter
if [[ -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

export NODE_ENV=production
export NODE_OPTIONS="${NODE_OPTIONS:+$NODE_OPTIONS }--dns-result-order=ipv4first"

exec /root/.bun/bin/bun src/server/index.ts

#!/usr/bin/env bash
set -euo pipefail

WOW_DATA_PORT="${WOW_DATA_SERVER_PORT:-17753}"

systemctl stop wow-converter.service wow-data-server.service
bash /root/clean.sh
systemctl start wow-data-server.service

for ((i = 1; i <= 120; i++)); do
  curl -sf "http://127.0.0.1:${WOW_DATA_PORT}/rest/getConfig" >/dev/null 2>&1 && break
  [[ "${i}" == "120" ]] && { echo "wow-data-server REST not ready" >&2; exit 1; }
  sleep 2
done

systemctl start wow-converter.service
for ((i = 1; i <= 120; i++)); do
  curl -sf http://127.0.0.1:3001/ >/dev/null 2>&1 && break
  [[ "${i}" == "120" ]] && { echo "wow-converter HTTP not ready" >&2; exit 1; }
  sleep 2
done

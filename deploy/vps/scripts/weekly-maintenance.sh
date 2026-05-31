#!/usr/bin/env bash
set -euo pipefail

systemctl stop wow-converter.service wow-export.service
bash /root/clean.sh
systemctl start wow-export.service

for ((i = 1; i <= 120; i++)); do
  curl -sf http://127.0.0.1:17752/rest/getConfig >/dev/null 2>&1 && break
  [[ "${i}" == "120" ]] && { echo "wow.export REST not ready" >&2; exit 1; }
  sleep 2
done

systemctl start wow-converter.service
for ((i = 1; i <= 120; i++)); do
  curl -sf http://127.0.0.1:3001/ >/dev/null 2>&1 && break
  [[ "${i}" == "120" ]] && { echo "wow-converter HTTP not ready" >&2; exit 1; }
  sleep 2
done

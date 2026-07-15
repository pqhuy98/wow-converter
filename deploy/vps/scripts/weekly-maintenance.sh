#!/usr/bin/env bash
set -euo pipefail

systemctl stop wow-converter.service
bash /root/clean.sh
systemctl start wow-converter.service
for ((i = 1; i <= 120; i++)); do
  if curl -sf http://127.0.0.1:3001/ >/dev/null 2>&1 &&
    curl -sf http://127.0.0.1:3001/api/wow-config/status |
      python3 -c 'import json,sys; raise SystemExit(0 if json.load(sys.stdin).get("cascLoaded") else 1)'
  then
    break
  fi
  [[ "${i}" == "120" ]] && { echo "bundled wow-converter or CASC not ready" >&2; exit 1; }
  sleep 2
done

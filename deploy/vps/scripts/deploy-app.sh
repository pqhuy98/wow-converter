#!/usr/bin/env bash
set -euo pipefail

export PATH="/root/.bun/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

REPO="${WOW_CONVERTER_REPO:-/root/wow-converter}"
LOCK_FILE="/tmp/wow-converter-deploy.lock"
BUN="/root/.bun/bin/bun"
WOW_DATA_PORT="${WOW_DATA_SERVER_PORT:-17753}"

log() { echo "[$(date -Is)] deploy-app: $*"; }

exec 9>"${LOCK_FILE}"
flock -n 9 || { log "deploy already running"; exit 0; }

cd "${REPO}"
git fetch origin main

LOCAL="$(git rev-parse HEAD)"
REMOTE="$(git rev-parse origin/main)"
if [[ "${LOCAL}" == "${REMOTE}" && "${FORCE_DEPLOY:-}" != "1" ]]; then
  log "already up to date at ${LOCAL}"
  exit 0
fi

log "updating ${LOCAL} → ${REMOTE}"
git reset --hard origin/main

bash "${REPO}/deploy/vps/sync-from-repo.sh"

"${BUN}" install
"${BUN}" run build:server

systemctl restart wow-data-server.service
for ((i = 1; i <= 120; i++)); do
  curl -sf "http://127.0.0.1:${WOW_DATA_PORT}/rest/getConfig" >/dev/null 2>&1 && break
  [[ "${i}" == "120" ]] && { log "wow-data-server REST not ready" >&2; exit 1; }
  sleep 2
done

systemctl restart wow-converter.service
for ((i = 1; i <= 120; i++)); do
  curl -sf http://127.0.0.1:3001/ >/dev/null 2>&1 && break
  [[ "${i}" == "120" ]] && { log "wow-converter HTTP not ready" >&2; exit 1; }
  sleep 2
done

log "done at $(git rev-parse --short HEAD)"

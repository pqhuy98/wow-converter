#!/usr/bin/env bash
set -euo pipefail

export PATH="/root/.bun/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

REPO="${WOW_CONVERTER_REPO:-/root/wow-converter}"
LOCK_FILE="/tmp/wow-converter-deploy.lock"
BUN="/root/.bun/bin/bun"

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

WOW_EXPORT_OLD="$(git rev-parse HEAD:wow.export 2>/dev/null || true)"
log "updating ${LOCAL} → ${REMOTE}"
git reset --hard origin/main
git submodule update --init --recursive

bash "${REPO}/deploy/vps/sync-from-repo.sh"

"${BUN}" install
"${BUN}" run build:server

if [[ "$(git rev-parse HEAD:wow.export)" != "${WOW_EXPORT_OLD}" ]]; then
  log "rebuilding wow.export linux-x64"
  ( cd "${REPO}/wow.export" && "${BUN}" install && "${BUN}" run build-linux )
fi

systemctl restart wow-export.service
for ((i = 1; i <= 120; i++)); do
  curl -sf http://127.0.0.1:17752/rest/getConfig >/dev/null 2>&1 && break
  [[ "${i}" == "120" ]] && { log "wow.export REST not ready" >&2; exit 1; }
  sleep 2
done

systemctl restart wow-converter.service
for ((i = 1; i <= 120; i++)); do
  curl -sf http://127.0.0.1:3001/ >/dev/null 2>&1 && break
  [[ "${i}" == "120" ]] && { log "wow-converter HTTP not ready" >&2; exit 1; }
  sleep 2
done

log "done at $(git rev-parse --short HEAD)"

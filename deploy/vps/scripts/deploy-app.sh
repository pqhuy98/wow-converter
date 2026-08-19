#!/usr/bin/env bash
set -euo pipefail

export PATH="/root/.bun/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export HOME="${HOME:-/root}"
export GOPATH="${GOPATH:-${HOME}/go}"

REPO="${WOW_CONVERTER_REPO:-/root/wow-converter}"
BRANCH="${WOW_DEPLOY_BRANCH:-golang-port}"
LOCK_FILE="/tmp/wow-converter-deploy.lock"
BUN="/root/.bun/bin/bun"

log() { echo "[$(date -Is)] deploy-app: $*"; }

exec 9>"${LOCK_FILE}"
flock -n 9 || { log "deploy already running"; exit 0; }

cd "${REPO}"
git fetch origin "${BRANCH}"

LOCAL="$(git rev-parse HEAD)"
REMOTE="$(git rev-parse "origin/${BRANCH}")"
if [[ "${LOCAL}" == "${REMOTE}" && "${FORCE_DEPLOY:-}" != "1" ]]; then
  log "already up to date at ${LOCAL}"
  exit 0
fi

log "updating ${LOCAL} → ${REMOTE}"
git reset --hard "origin/${BRANCH}"

"${BUN}" install
"${BUN}" run build:linux

bash "${REPO}/deploy/vps/sync-from-repo.sh"

# Cold CASC load after restart can take several minutes (archives + encoding table from CDN).
CASC_READY_WAIT_SECS="${CASC_READY_WAIT_SECS:-600}"
CASC_READY_POLL_SECS=2
CASC_READY_MAX_ATTEMPTS=$((CASC_READY_WAIT_SECS / CASC_READY_POLL_SECS))

systemctl restart wow-converter.service
for ((i = 1; i <= CASC_READY_MAX_ATTEMPTS; i++)); do
  if curl -sf http://127.0.0.1:3001/ >/dev/null 2>&1 &&
    curl -sf http://127.0.0.1:3001/api/wow-config/status |
      python3 -c 'import json,sys; raise SystemExit(0 if json.load(sys.stdin).get("cascLoaded") else 1)'
  then
    break
  fi
  [[ "${i}" == "${CASC_READY_MAX_ATTEMPTS}" ]] && {
    log "bundled wow-converter or CASC not ready after ${CASC_READY_WAIT_SECS}s" >&2
    exit 1
  }
  sleep "${CASC_READY_POLL_SECS}"
done

log "done at $(git rev-parse --short HEAD)"

#!/usr/bin/env bash
set -euo pipefail

REPO="${WOW_CONVERTER_REPO:-/root/wow-converter}"
BRANCH="${WOW_DEPLOY_BRANCH:-golang-port}"
cd "${REPO}"
git fetch origin "${BRANCH}" --quiet

LOCAL="$(git rev-parse HEAD)"
REMOTE="$(git rev-parse "origin/${BRANCH}")"
if [[ "${LOCAL}" != "${REMOTE}" ]]; then
  SCRIPT="/root/wow-hosting/scripts/deploy-app.sh"
  [[ -x "${SCRIPT}" ]] || SCRIPT="${REPO}/deploy/vps/scripts/deploy-app.sh"
  bash "${SCRIPT}"
fi

#!/usr/bin/env bash
set -euo pipefail

REPO="${WOW_CONVERTER_REPO:-/root/wow-converter}"
HOSTING="/root/wow-hosting"
DEPLOY="${REPO}/deploy/vps"

[[ -d "${DEPLOY}" ]] || { echo "missing ${DEPLOY}" >&2; exit 1; }

mkdir -p "${HOSTING}/scripts" "${HOSTING}/systemd"
rm -rf "${HOSTING}/scripts/"* "${HOSTING}/systemd/"*
cp -r "${DEPLOY}/scripts/." "${HOSTING}/scripts/"
cp -r "${DEPLOY}/systemd/." "${HOSTING}/systemd/"
cp "${DEPLOY}/install.sh" "${HOSTING}/"
cp "${DEPLOY}/clean.sh" /root/clean.sh
chmod +x "${HOSTING}"/*.sh "${HOSTING}/scripts/"*.sh /root/clean.sh

bash "${HOSTING}/install.sh"

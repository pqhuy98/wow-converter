#!/usr/bin/env bash
set -euo pipefail

PROFILE="/root/.config/wow.export-huy-edition"
rm -f "${PROFILE}/SingletonLock" "${PROFILE}/SingletonSocket" "${PROFILE}/SingletonCookie"

BIN="/root/wow-converter/wow.export/bin/linux-x64/wow.export"
cd "$(dirname "${BIN}")"
exec xvfb-run -a "${BIN}"

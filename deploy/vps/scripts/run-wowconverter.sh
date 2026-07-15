#!/usr/bin/env bash
set -euo pipefail

cd /root/wow-converter/dist-go
if [[ -f /root/wow-converter/.env ]]; then
  set -a
  # shellcheck disable=SC1091
  source /root/wow-converter/.env
  set +a
fi

export NODE_ENV=production
export WOW_CONVERTER_BUNDLED=1

exec ./wow-converter

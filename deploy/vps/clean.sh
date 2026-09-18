#!/usr/bin/env bash
# Prune generated export artifacts only; keep .cache/wow (CASC).
APP_ROOT="${WOW_CONVERTER_APP_ROOT:-/root/wow-converter/dist-go}"
rm -rf \
  "${APP_ROOT}/exported-assets" \
  "${APP_ROOT}/exported-assets-browse" \
  "${APP_ROOT}/recent-exports.json" \
  "${APP_ROOT}/.cache/wow-export"

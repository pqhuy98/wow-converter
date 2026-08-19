#!/usr/bin/env bash
# Prune generated export artifacts only; keep .cache/wow (CASC) and .cache/wow-export.
rm -rf \
  ~/wow-converter/exported-assets \
  ~/wow-converter/exported-assets-browse \
  ~/wow-converter/recent-exports.json

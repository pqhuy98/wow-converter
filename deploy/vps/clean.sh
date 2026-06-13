#!/usr/bin/env bash
# Remove exported assets and wow-data-server cache output (not source code).
rm -rf \
  ~/wow-converter/.cache/wow-export \
  ~/wow-converter/exported-assets \
  ~/wow-converter/exported-assets-browse \
  ~/wow-converter/recent-exports.json

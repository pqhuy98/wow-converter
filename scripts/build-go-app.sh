#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="${ROOT}/dist-go"

cd "${ROOT}"
[[ -f webui/out/index.html ]] || {
  echo "webui/out is missing; run 'bun run build:webui' first" >&2
  exit 1
}

rm -rf "${DIST}"
mkdir -p "${DIST}"

(
  cd golang
  go build -ldflags "-s -w" -o "${DIST}/wow-converter" ./cmd/wow-converter
)

mkdir -p "${DIST}/webui" "${DIST}/bin"
cp -a webui/out "${DIST}/webui/out"
cp -a resources "${DIST}/resources"

[[ -f bin/azerothcore-world.sqlite ]] && cp bin/azerothcore-world.sqlite "${DIST}/bin/"
[[ -d bin/blp-native ]] && cp -a bin/blp-native "${DIST}/bin/"
[[ -d bin/upscayl ]] && cp -a bin/upscayl "${DIST}/bin/"

echo "Built ${DIST}/wow-converter"

#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$ROOT/node_modules/upscayl-node/dist/upscaler/sub-classes"
OUT="$ROOT/bin/upscayl"

if [[ ! -d "$SRC/model-manager/models" ]]; then
  echo "upscayl-node models not found. Run npm install first." >&2
  exit 1
fi

mkdir -p "$OUT/win" "$OUT/linux" "$OUT/models"
cp -f "$SRC/model-manager/models/"*.{bin,param} "$OUT/models/"
if [[ -f "$SRC/driver/command-upscayl/resources/win/bin/upscayl-bin.exe" ]]; then
  cp -f "$SRC/driver/command-upscayl/resources/win/bin/"* "$OUT/win/"
fi
if [[ -f "$SRC/driver/command-upscayl/resources/linux/bin/upscayl-bin" ]]; then
  cp -f "$SRC/driver/command-upscayl/resources/linux/bin/upscayl-bin" "$OUT/linux/"
  chmod +x "$OUT/linux/upscayl-bin"
fi

echo "Installed upscayl runtime to $OUT"

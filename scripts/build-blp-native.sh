#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
STANDALONE="$ROOT/node-images-blp/standalone"
THIRD="$ROOT/node-images-blp/gyp/third-party"
OUT="$ROOT/bin/blp-native"
BUILD="$STANDALONE/build-gcc"

mkdir -p "$OUT" "$BUILD"

CXXFLAGS=(
  -std=c++11 -O2 -fPIC
  -DHAVE_PNG -DHAVE_BLP -DBLPENCODE_EXPORTS
  -I"$STANDALONE"
  -I"$THIRD/libpng"
  -I"$THIRD/zlib"
)

CFLAGS=("${CXXFLAGS[@]}")

objects=()
compile() {
  local src="$1"
  local obj="$BUILD/$(basename "${src%.*}").o"
  if [[ "$src" == *.cc || "$src" == *.cpp ]]; then
    g++ "${CXXFLAGS[@]}" -c "$src" -o "$obj"
  else
    gcc "${CFLAGS[@]}" -c "$src" -o "$obj"
  fi
  objects+=("$obj")
}

for src in "$STANDALONE"/pixel_array.cc "$STANDALONE"/blp_encode_api.cc "$STANDALONE"/blp_codecs.cc; do
  compile "$src"
done

for src in "$THIRD"/zlib/*.c "$THIRD"/libpng/*.c; do
  compile "$src"
done

g++ -shared -o "$OUT/libblpencode.so" "${objects[@]}"
echo "Built $OUT/libblpencode.so"

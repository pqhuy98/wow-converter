/** PNG decode/encode for the visual snapshot sheets. Filter 0 on write. */
import { deflateSync, inflateSync } from 'zlib';

export interface RgbaImage {
  readonly width: number;
  readonly height: number;
  readonly data: Uint8Array;
}

const PNG_SIG = [137, 80, 78, 71, 13, 10, 26, 10] as const;

const CRC_TABLE = new Uint32Array(256);
for (let n = 0; n < 256; n++) {
  let c = n;
  for (let k = 0; k < 8; k++) c = (c & 1) !== 0 ? (0xedb88320 ^ (c >>> 1)) : (c >>> 1);
  CRC_TABLE[n] = c >>> 0;
}

export function decodePng(bytes: Uint8Array): RgbaImage {
  if (bytes.length < 8) throw new Error('png too small');
  for (let i = 0; i < 8; i++) {
    if (bytes[i] !== PNG_SIG[i]) throw new Error('not a png');
  }
  let width = 0;
  let height = 0;
  let bitDepth = 0;
  let colorType = -1;
  let interlace = 0;
  const idat: Uint8Array[] = [];
  let offset = 8;
  while (offset + 12 <= bytes.length) {
    const len = readU32(bytes, offset);
    const type = String.fromCharCode(bytes[offset + 4] ?? 0, bytes[offset + 5] ?? 0, bytes[offset + 6] ?? 0, bytes[offset + 7] ?? 0);
    const data = bytes.subarray(offset + 8, offset + 8 + len);
    offset += 12 + len;
    if (type === 'IHDR') {
      width = readU32(data, 0);
      height = readU32(data, 4);
      bitDepth = data[8] ?? 0;
      colorType = data[9] ?? -1;
      interlace = data[12] ?? 0;
    } else if (type === 'IDAT') {
      idat.push(data);
    } else if (type === 'IEND') {
      break;
    }
  }
  if (width <= 0 || height <= 0) throw new Error('png missing header');
  if (bitDepth !== 8 || interlace !== 0 || (colorType !== 2 && colorType !== 6)) {
    throw new Error(`unsupported png color ${colorType} depth ${bitDepth}`);
  }
  const channels = colorType === 6 ? 4 : 3;
  const raw = inflateSync(concat(idat));
  const stride = width * channels;
  if (raw.length < height * (stride + 1)) throw new Error('png data short');
  const rgba = new Uint8Array(width * height * 4);
  const prev = new Uint8Array(stride);
  const row = new Uint8Array(stride);
  let src = 0;
  for (let y = 0; y < height; y++) {
    const filter = raw[src] ?? 0;
    src += 1;
    unfilter(filter, raw.subarray(src, src + stride), prev, row, channels);
    src += stride;
    for (let x = 0; x < width; x++) {
      const i = (y * width + x) * 4;
      const j = x * channels;
      rgba[i] = row[j] ?? 0;
      rgba[i + 1] = row[j + 1] ?? 0;
      rgba[i + 2] = row[j + 2] ?? 0;
      rgba[i + 3] = channels === 4 ? (row[j + 3] ?? 0) : 255;
    }
    prev.set(row);
  }
  return { width, height, data: rgba };
}

export function encodePng(image: RgbaImage): Uint8Array {
  const stride = image.width * 4;
  const raw = new Uint8Array(image.height * (stride + 1));
  for (let y = 0; y < image.height; y++) {
    raw[y * (stride + 1)] = 0;
    raw.set(image.data.subarray(y * stride, (y + 1) * stride), y * (stride + 1) + 1);
  }
  const ihdr = new Uint8Array(13);
  const view = new DataView(ihdr.buffer);
  view.setUint32(0, image.width);
  view.setUint32(4, image.height);
  ihdr[8] = 8;
  ihdr[9] = 6;
  const parts = [
    Uint8Array.from(PNG_SIG),
    pngChunk('IHDR', ihdr),
    pngChunk('IDAT', deflateSync(raw)),
    pngChunk('IEND', new Uint8Array(0)),
  ];
  return concat(parts);
}

export function stitchSheet(
  views: ReadonlyMap<string, RgbaImage>,
  layout: readonly (readonly string[])[],
  tileW: number,
  tileH: number,
): RgbaImage {
  const rows = layout.length;
  const cols = layout[0]?.length ?? 0;
  if (rows === 0 || cols === 0) throw new Error('empty sheet layout');
  const sheet: RgbaImage = {
    width: cols * tileW,
    height: rows * tileH,
    data: new Uint8Array(cols * tileW * rows * tileH * 4),
  };
  for (let r = 0; r < rows; r++) {
    const row = layout[r];
    if (!row || row.length !== cols) throw new Error('layout rows differ');
    for (let c = 0; c < cols; c++) {
      const name = row[c] ?? '';
      const tile = views.get(name);
      if (!tile) throw new Error(`missing view ${name}`);
      if (tile.width !== tileW || tile.height !== tileH) {
        throw new Error(`view ${name} is ${tile.width}x${tile.height}`);
      }
      blit(sheet, tile, c * tileW, r * tileH);
    }
  }
  return sheet;
}

/** Counted pixels are those whose largest channel change is greater than channelDelta. */
export function diffSheet(expected: RgbaImage, actual: RgbaImage, channelDelta: number): { counted: number; heatmap: RgbaImage } {
  if (expected.width !== actual.width || expected.height !== actual.height) {
    throw new Error(`sheet size ${actual.width}x${actual.height} vs expected ${expected.width}x${expected.height}`);
  }
  const heatmap = new Uint8Array(expected.data.length);
  let counted = 0;
  for (let i = 0; i < expected.data.length; i += 4) {
    let max = 0;
    for (let c = 0; c < 4; c++) {
      const delta = Math.abs((expected.data[i + c] ?? 0) - (actual.data[i + c] ?? 0));
      if (delta > max) max = delta;
    }
    if (max > channelDelta) counted += 1;
    heatmap[i] = max;
    heatmap[i + 1] = 0;
    heatmap[i + 2] = 0;
    heatmap[i + 3] = 255;
  }
  return { counted, heatmap: { width: expected.width, height: expected.height, data: heatmap } };
}

export function assertPngRoundTrip(): void {
  const image: RgbaImage = {
    width: 2,
    height: 2,
    data: Uint8Array.of(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 255, 0, 128, 255),
  };
  const back = decodePng(encodePng(image));
  if (back.width !== 2 || back.height !== 2 || back.data.length !== image.data.length) {
    throw new Error('png roundtrip size');
  }
  for (let i = 0; i < image.data.length; i++) {
    if (back.data[i] !== image.data[i]) throw new Error('png roundtrip bytes');
  }
  const filtered = decodePng(encodeFilteredFixture());
  const expect = [10, 20, 30, 255, 11, 22, 33, 255, 15, 26, 37, 255, 16, 28, 40, 255];
  if (filtered.width !== 2 || filtered.height !== 2) throw new Error('png filter size');
  for (let i = 0; i < expect.length; i++) {
    if (filtered.data[i] !== expect[i]) throw new Error(`png filter byte ${i}`);
  }
  const same = Uint8Array.of(1, 2, 3, 255);
  const changed = Uint8Array.of(1, 2, 3, 200);
  const { counted, heatmap } = diffSheet(
    { width: 2, height: 1, data: Uint8Array.of(...same, ...same) },
    { width: 2, height: 1, data: Uint8Array.of(...same, ...changed) },
    4,
  );
  if (counted !== 1) throw new Error('diff count');
  if (heatmap.data[0] !== 0 || heatmap.data[4] !== 55) throw new Error('diff red scale');
}

function encodeFilteredFixture(): Uint8Array {
  // 2x2 RGBA. Row 0 is filter None. Row 1 is Paeth, hand-computed:
  // left/up/upLeft for pixel0 channel0 are 0/10/0 → predictor 10, stored 5 → 15.
  const raw = Uint8Array.of(
    0,
    10,
    20,
    30,
    255,
    11,
    22,
    33,
    255,
    4,
    5,
    6,
    7,
    0,
    1,
    2,
    3,
    0,
  );
  const ihdr = new Uint8Array(13);
  const view = new DataView(ihdr.buffer);
  view.setUint32(0, 2);
  view.setUint32(4, 2);
  ihdr[8] = 8;
  ihdr[9] = 6;
  return concat([
    Uint8Array.from(PNG_SIG),
    pngChunk('IHDR', ihdr),
    pngChunk('IDAT', deflateSync(raw)),
    pngChunk('IEND', new Uint8Array(0)),
  ]);
}

function unfilter(filter: number, filt: Uint8Array, prev: Uint8Array, row: Uint8Array, bpp: number): void {
  for (let i = 0; i < filt.length; i++) {
    const left = i >= bpp ? (row[i - bpp] ?? 0) : 0;
    const up = prev[i] ?? 0;
    const upLeft = i >= bpp ? (prev[i - bpp] ?? 0) : 0;
    const v = filt[i] ?? 0;
    if (filter === 0) row[i] = v;
    else if (filter === 1) row[i] = (v + left) & 255;
    else if (filter === 2) row[i] = (v + up) & 255;
    else if (filter === 3) row[i] = (v + Math.floor((left + up) / 2)) & 255;
    else if (filter === 4) row[i] = (v + paeth(left, up, upLeft)) & 255;
    else throw new Error(`png filter ${filter}`);
  }
}

function paeth(left: number, up: number, upLeft: number): number {
  const p = left + up - upLeft;
  const pa = Math.abs(p - left);
  const pb = Math.abs(p - up);
  const pc = Math.abs(p - upLeft);
  if (pa <= pb && pa <= pc) return left;
  if (pb <= pc) return up;
  return upLeft;
}

function blit(dest: RgbaImage, src: RgbaImage, x: number, y: number): void {
  for (let row = 0; row < src.height; row++) {
    const from = row * src.width * 4;
    const to = ((y + row) * dest.width + x) * 4;
    dest.data.set(src.data.subarray(from, from + src.width * 4), to);
  }
}

function pngChunk(type: string, data: Uint8Array): Uint8Array {
  const typeBytes = new TextEncoder().encode(type);
  const out = new Uint8Array(12 + data.length);
  const view = new DataView(out.buffer);
  view.setUint32(0, data.length);
  out.set(typeBytes, 4);
  out.set(data, 8);
  const crcBuf = new Uint8Array(4 + data.length);
  crcBuf.set(typeBytes, 0);
  crcBuf.set(data, 4);
  view.setUint32(8 + data.length, crc32(crcBuf));
  return out;
}

function crc32(bytes: Uint8Array): number {
  let c = 0xffffffff;
  for (let i = 0; i < bytes.length; i++) {
    c = (CRC_TABLE[(c ^ (bytes[i] ?? 0)) & 0xff] ?? 0) ^ (c >>> 8);
  }
  return (c ^ 0xffffffff) >>> 0;
}

function readU32(bytes: Uint8Array, offset: number): number {
  return new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength).getUint32(offset);
}

function concat(parts: readonly Uint8Array[]): Uint8Array {
  let n = 0;
  for (const part of parts) n += part.length;
  const out = new Uint8Array(n);
  let at = 0;
  for (const part of parts) {
    out.set(part, at);
    at += part.length;
  }
  return out;
}

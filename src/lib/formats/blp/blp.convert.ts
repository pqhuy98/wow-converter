/* eslint-disable import/no-dynamic-require */
/**
 * PNG -> BLP conversion logic, shared by the worker-thread pool (blp.worker.ts)
 * and the inline fallback path (BLP_WORKERS=0). Extracted from blp.worker.ts.
 */
import { writeFile } from 'fs/promises';
import { ensureDir } from 'fs-extra';
import * as IQ from 'image-q';
import { createRequire } from 'module';
import path from 'path';
import sharp from 'sharp';
import { fileURLToPath } from 'url';

const require = createRequire(import.meta.url);
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// eslint-disable-next-line @typescript-eslint/no-explicit-any
let binding: any;
let bindingLoadAttempted = false;

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function loadBlpNativeBinding(): any {
  if (bindingLoadAttempted) return binding;
  bindingLoadAttempted = true;

  try {
    const candidateDirs: string[] = [];

    // 1) Original behavior: resolve relative to current working directory
    candidateDirs.push(path.join(process.cwd(), 'bin/blp-preview'));

    // 2) Dist / exe layout: worker in dist root and bindings in dist/bin
    candidateDirs.push(path.join(__dirname, 'bin/blp-preview'));

    // 3) Source / node_modules layout: worker in src/lib/formats/blp and bindings in package-root/bin
    candidateDirs.push(path.resolve(__dirname, '../../../../bin/blp-preview'));

    // 4) Installed as dependency: resolve package root via its own package.json
    try {
      const pkgRoot = path.dirname(require.resolve('@pqhuy98/wow-converter/package.json'));
      candidateDirs.push(path.join(pkgRoot, 'bin/blp-preview'));
    } catch {
      // ignore if package root cannot be resolved (e.g. compiled exe)
    }

    for (const binDir of candidateDirs) {
      try {
        if (process.platform === 'win32') {
          binding = require(path.join(binDir, 'win32-x64-binding.node'));
        } else if (process.platform === 'linux' && process.arch === 'x64') {
          binding = require(path.join(binDir, 'linux-x64-binding.node'));
        }

        if (binding) {
          break;
        }
      } catch {
        // try next candidate
      }
    }
  } catch {
    // ignore, will fallback to JS
    console.log('Failed to load PNG->BLP\'s C++ native binding, will fallback to slower JavaScript implementation');
  }

  return binding;
}

/** Input for a single BLP1 encode task. Exactly one of png/blp2 must be set. */
export interface BlpEncodeInput {
  /** PNG bytes (legacy path / composited character textures). */
  png?: Buffer;
  /** Raw WoW BLP2 bytes; decoded to PNG in-process (in-memory PNG bridge). */
  blp2?: Buffer;
  /** Optional downscale applied to the PNG before BLP1 encoding. */
  resizeTo?: { width: number; height: number };
}

/**
 * Unified texture -> WC3 BLP1 conversion. Decodes raw BLP2 with the same
 * decoder + PNG writer the legacy server export used, optionally resizes with
 * the same sharp settings, then runs the existing PNG->BLP1 encoder, so the
 * output bytes are identical to the legacy PNG-file pipeline.
 */
export async function convertTextureToBlp(input: BlpEncodeInput, blpPath: string): Promise<void> {
  // Lazy imports keep worker startup light when only PNG inputs are used.
  let png = input.png;
  if (!png) {
    if (!input.blp2) throw new Error('convertTextureToBlp: either png or blp2 input is required');
    const { BLPImage } = await import('@/lib/wow/formats/blp/blp');
    const { BufferWrapper } = await import('@/lib/wow/formats/buffer');
    png = new BLPImage(new BufferWrapper(input.blp2)).toPNG(0b1111).raw;
  }
  if (input.resizeTo) {
    const { resizePng } = await import('../png');
    try {
      png = await resizePng(png, input.resizeTo.width, input.resizeTo.height);
    } catch (err) {
      // Match the legacy main-thread behavior: warn and encode at source size.
      console.warn('Failed to resize PNG, proceeding without resize:', blpPath, err);
    }
  }
  await convertPngToBlp(png, blpPath);
}

/**
 * Convert a PNG buffer to a BLP file on disk, using the native binding when
 * available and the JS implementation otherwise.
 */
export async function convertPngToBlp(pngBufferOriginal: Buffer, blpPath: string): Promise<void> {
  const { Image, TYPE_BLP } = loadBlpNativeBinding() || {};

  // Normalize PNG alpha: if the entire alpha channel is 0, force it to 255 (opaque)
  // This matches WoW/WMO opaque rendering that ignores alpha and avoids black output in WC3.
  const pngBuffer = await ensureOpaqueIfAllAlphaZero(pngBufferOriginal);

  if (!Image || TYPE_BLP === undefined) {
    await png2BlpJs(pngBuffer, blpPath);
    return;
  }

  const img = new Image();
  try {
    img.loadFromBuffer(pngBuffer, 0, pngBuffer.length);
  } catch (error: unknown) {
    await new Promise((resolve) => { setTimeout(resolve, 1000); });
    img.loadFromBuffer(pngBuffer, 0, pngBuffer.length);
  }

  const blpBuffer = img.toBuffer(TYPE_BLP);
  try {
    await ensureDir(path.dirname(blpPath));
  } catch (err) {
    // do nothing
  }
  await writeFile(blpPath, blpBuffer);
}

/**
 * Faster variant of png2BlpJs using sharp for decoding and optimized quantization path.
 * - Decodes PNG via libvips (sharp) into raw RGBA
 * - Skips quantization entirely when unique RGB colors ≤ 256 (alpha treated as a separate mask channel)
 * - Uses faster image-q settings when quantization is necessary
 * - Minimizes intermediate allocations and extra copies
 */
export async function png2BlpJs(pngBuffer: Buffer, distPath: string) {
  // Decode input with sharp to raw RGBA without resizing
  const { data, info } = await sharp(pngBuffer)
    .ensureAlpha()
    .raw()
    .toBuffer({ resolveWithObject: true });

  const width = info.width;
  const height = info.height;
  const pixelCount = width * height;

  // Pre-allocate mip0 buffers once
  const indices0 = Buffer.alloc(pixelCount);
  const alpha0 = Buffer.alloc(pixelCount);

  // Attempt fast-path: build palette directly if unique RGB colors ≤ 256 (alpha is stored separately)
  // Build mapping while scanning; abort early if unique > 256
  const rgbToIndexFast = new Map<number, number>();
  const paletteBufferFast = Buffer.alloc(256 * 4, 0);
  let paletteSizeFast = 0;
  let exceededFastPath = false;

  for (let i = 0; i < pixelCount; i++) {
    const r = data[i * 4]!;
    const g = data[i * 4 + 1]!;
    const b = data[i * 4 + 2]!;
    const a = data[i * 4 + 3]!;
    alpha0[i] = a;

    // 24-bit RGB key
    const key = rgbKey(r, g, b);
    let idx = rgbToIndexFast.get(key);
    if (idx === undefined) {
      if (paletteSizeFast === 256) {
        exceededFastPath = true;
        continue;
      }
      idx = paletteSizeFast;
      rgbToIndexFast.set(key, idx);
      // B, G, R, A in palette; palette alpha is unused for BLP1 indexed+separate-alpha, keep it opaque.
      const p = idx * 4;
      paletteBufferFast[p] = b;
      paletteBufferFast[p + 1] = g;
      paletteBufferFast[p + 2] = r;
      paletteBufferFast[p + 3] = 0xff;
      paletteSizeFast++;
    }
  }

  let paletteBuffer: Buffer;
  let paletteSize: number;

  if (!exceededFastPath) {
    // Fast path succeeded – use the directly built palette and fill outputs in a second pass
    paletteBuffer = paletteBufferFast;
    paletteSize = paletteSizeFast;

    for (let i = 0; i < pixelCount; i++) {
      const r = data[i * 4]!;
      const g = data[i * 4 + 1]!;
      const b = data[i * 4 + 2]!;
      const idx = rgbToIndexFast.get(rgbKey(r, g, b)) ?? 0;
      indices0[i] = idx;
    }
  } else {
    // Slow path: quantize RGB only (alpha treated as a separate mask channel).
    // We do this by forcing alpha to 255 for palette building + dithering, while preserving the original alpha stream.
    const dataOpaque = Buffer.from(data);
    for (let i = 0; i < pixelCount; i++) {
      dataOpaque[i * 4 + 3] = 255;
    }

    // Slow path: quantize using image-q with faster settings
    const pointContainer = IQ.utils.PointContainer.fromUint8Array(dataOpaque, width, height);

    const palette = await IQ.buildPalette([pointContainer], {
      colors: 256,
      // Favor quality similar to the original implementation
      paletteQuantization: 'wuquant',
      colorDistanceFormula: 'euclidean-bt709',
    });

    const quantised = await IQ.applyPalette(pointContainer, palette, {
      // Dithering improves gradients at a small perf cost
      imageQuantization: 'floyd-steinberg',
      colorDistanceFormula: 'euclidean-bt709',
    });

    const palettePoints = palette.getPointContainer().getPointArray();

    // Build palette buffer (BGRA), zero-initialized then filled up to palette size
    paletteBuffer = Buffer.alloc(256 * 4, 0);
    paletteSize = 0;
    const rgbToIndex = new Map<number, number>();
    for (let i = 0; i < palettePoints.length && i < 256; i++) {
      const p = palettePoints[i];
      const bufIndex = i * 4;
      paletteBuffer[bufIndex] = p.b;
      paletteBuffer[bufIndex + 1] = p.g;
      paletteBuffer[bufIndex + 2] = p.r;
      paletteBuffer[bufIndex + 3] = 0xff;

      const key = rgbKey(p.r, p.g, p.b);
      if (!rgbToIndex.has(key)) {
        rgbToIndex.set(key, i);
      }
      paletteSize++;
    }

    // Map quantized RGBA pixels to palette indices
    const quantRGBA = quantised.toUint8Array();
    for (let i = 0; i < pixelCount; i++) {
      const r = quantRGBA[i * 4]!;
      const g = quantRGBA[i * 4 + 1]!;
      const b = quantRGBA[i * 4 + 2]!;
      const idx = rgbToIndex.get(rgbKey(r, g, b)) ?? 0;
      indices0[i] = idx;
    }
  }

  // Build a 64x64x64 LUT for fast nearest-palette mapping for mip levels.
  // This matches the C++ encoder's approach (nearest in BT.709-weighted space).
  const lut64 = buildLut64FromPalette(paletteBuffer, paletteSize);

  // Plan mip chain up to 16 levels, stopping at 1x1 (avoid repeated 1x1 mips).
  const mipDims: Array<{readonly w: number; readonly h: number}> = [];
  {
    let mw = width;
    let mh = height;
    for (let level = 0; level < 16; level++) {
      mipDims.push({ w: mw, h: mh });
      if (mw === 1 && mh === 1) break;
      mw = Math.max(1, Math.ceil(mw / 2));
      mh = Math.max(1, Math.ceil(mh / 2));
    }
  }

  const mipCount = mipDims.length;

  // ----------------------------------------------------------------------------
  // Header construction (BLP1)
  // ----------------------------------------------------------------------------
  const BLP1_HEADER_SIZE = 156; // 28 + 64 + 64

  const header = Buffer.alloc(BLP1_HEADER_SIZE, 0);
  header.write('BLP1', 0, 'ascii');
  header.writeUInt32LE(1, 4); // content = 1 (Direct / Palette)
  header.writeUInt32LE(8, 8); // alphaBits = 8
  header.writeUInt32LE(width, 12);
  header.writeUInt32LE(height, 16);
  header.writeUInt32LE(0, 20); // extra (unused)
  header.writeUInt32LE(mipCount > 1 ? 1 : 0, 24); // hasMipmaps

  const offsetPos = 28;
  const sizePos = offsetPos + 64;
  const pixelDataOffset = BLP1_HEADER_SIZE + 1024; // palette follows header

  let totalMipBytes = 0;
  for (const d of mipDims) {
    totalMipBytes += d.w * d.h * 2; // indices + alpha
  }

  const out = Buffer.allocUnsafe(BLP1_HEADER_SIZE + 1024 + totalMipBytes);
  paletteBuffer.copy(out, BLP1_HEADER_SIZE);

  // Fill mip offset/size tables and write mip payloads sequentially.
  let writeOffset = pixelDataOffset;
  let currentRgba: Uint8Array = data;
  let cw = width;
  let ch = height;

  for (let level = 0; level < mipCount; level++) {
    const pixels = cw * ch;
    const mipBytes = pixels * 2;
    header.writeUInt32LE(writeOffset, offsetPos + level * 4);
    header.writeUInt32LE(mipBytes, sizePos + level * 4);

    if (level === 0) {
      indices0.copy(out, writeOffset);
      alpha0.copy(out, writeOffset + pixels);
    } else {
      const idxDst = out.subarray(writeOffset, writeOffset + pixels);
      const aDst = out.subarray(writeOffset + pixels, writeOffset + mipBytes);

      for (let i = 0; i < pixels; i++) {
        const base = i * 4;
        const r = currentRgba[base]!;
        const g = currentRgba[base + 1]!;
        const b = currentRgba[base + 2]!;
        const a = currentRgba[base + 3]!;

        const r6 = r >> 2;
        const g6 = g >> 2;
        const b6 = b >> 2;
        idxDst[i] = lut64[(r6 << 12) | (g6 << 6) | b6]!;
        aDst[i] = a;
      }
    }

    writeOffset += mipBytes;

    if (level + 1 < mipCount) {
      const next = downsample2x2SeparateAlpha(currentRgba, cw, ch);
      currentRgba = next.rgba;
      cw = next.w;
      ch = next.h;
    }
  }

  // Copy finalized header (with mip tables) into output
  header.copy(out, 0);

  const blpBuffer = out;

  try {
    await ensureDir(path.dirname(distPath));
  } catch (err) {
    // do nothing
  }
  await ensureDir(path.dirname(distPath));
  await writeFile(distPath, blpBuffer);
}

function rgbKey(r: number, g: number, b: number): number {
  return ((r & 0xff) << 16) | ((g & 0xff) << 8) | (b & 0xff);
}

function dist2Bt709(r1: number, g1: number, b1: number, r2: number, g2: number, b2: number): number {
  const wr = 0.2126;
  const wg = 0.7152;
  const wb = 0.0722;
  const dr = r1 - r2;
  const dg = g1 - g2;
  const db = b1 - b2;
  return wr * dr * dr + wg * dg * dg + wb * db * db;
}

function buildLut64FromPalette(paletteBuffer: Buffer, paletteSize: number): Uint8Array {
  const count = Math.max(1, Math.min(256, paletteSize));
  const pr = new Float32Array(count);
  const pg = new Float32Array(count);
  const pb = new Float32Array(count);
  for (let i = 0; i < count; i++) {
    const p = i * 4;
    pb[i] = paletteBuffer[p]!;
    pg[i] = paletteBuffer[p + 1]!;
    pr[i] = paletteBuffer[p + 2]!;
  }

  const lut = new Uint8Array(64 * 64 * 64);
  let idx = 0;
  for (let rr = 0; rr < 64; rr++) {
    const rSrgb = ((rr << 2) | 2);
    for (let gg = 0; gg < 64; gg++) {
      const gSrgb = ((gg << 2) | 2);
      for (let bb = 0; bb < 64; bb++) {
        const bSrgb = ((bb << 2) | 2);
        let best = 0;
        let bestD = Number.POSITIVE_INFINITY;
        for (let k = 0; k < count; k++) {
          const d = dist2Bt709(rSrgb, gSrgb, bSrgb, pr[k]!, pg[k]!, pb[k]!);
          if (d < bestD) {
            bestD = d;
            best = k;
          }
        }
        lut[idx++] = best;
      }
    }
  }
  return lut;
}

function downsample2x2SeparateAlpha(src: Uint8Array, sw: number, sh: number): { rgba: Uint8Array; w: number; h: number } {
  const dw = Math.max(1, Math.ceil(sw / 2));
  const dh = Math.max(1, Math.ceil(sh / 2));
  const dst = new Uint8Array(dw * dh * 4);

  for (let y = 0; y < dh; y++) {
    const sy0 = Math.min(sh - 1, y * 2);
    const sy1 = Math.min(sh - 1, sy0 + 1);
    for (let x = 0; x < dw; x++) {
      const sx0 = Math.min(sw - 1, x * 2);
      const sx1 = Math.min(sw - 1, sx0 + 1);

      const i00 = (sy0 * sw + sx0) * 4;
      const i10 = (sy0 * sw + sx1) * 4;
      const i01 = (sy1 * sw + sx0) * 4;
      const i11 = (sy1 * sw + sx1) * 4;

      const sumR = (src[i00]! + src[i10]! + src[i01]! + src[i11]!) >>> 0;
      const sumG = (src[i00 + 1]! + src[i10 + 1]! + src[i01 + 1]! + src[i11 + 1]!) >>> 0;
      const sumB = (src[i00 + 2]! + src[i10 + 2]! + src[i01 + 2]! + src[i11 + 2]!) >>> 0;
      const sumA = (src[i00 + 3]! + src[i10 + 3]! + src[i01 + 3]! + src[i11 + 3]!) >>> 0;

      const di = (y * dw + x) * 4;
      dst[di] = (sumR + 2) >> 2;
      dst[di + 1] = (sumG + 2) >> 2;
      dst[di + 2] = (sumB + 2) >> 2;
      dst[di + 3] = (sumA + 2) >> 2;
    }
  }

  return { rgba: dst, w: dw, h: dh };
}

/**
 * If the PNG's alpha channel is entirely zeros, force it to fully opaque (255).
 * This mirrors WoW's opaque material path (blendMode 0) which ignores alpha,
 * and prevents WC3 from rendering such textures as fully transparent/black.
 */
export async function ensureOpaqueIfAllAlphaZero(pngBuffer: Buffer): Promise<Buffer> {
  const { data, info } = await sharp(pngBuffer).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
  const total = info.width * info.height;
  let anyNonZero = false;
  for (let i = 0; i < total; i++) {
    if (data[i * 4 + 3] !== 0) {
      anyNonZero = true;
      break;
    }
  }
  if (anyNonZero) return pngBuffer;

  // Force alpha to 255 for all pixels
  for (let i = 0; i < total; i++) {
    data[i * 4 + 3] = 255;
  }

  // Re-encode to PNG for downstream path (native or JS)
  const fixed = await sharp(data, { raw: { width: info.width, height: info.height, channels: 4 } })
    .png()
    .toBuffer();
  return fixed;
}

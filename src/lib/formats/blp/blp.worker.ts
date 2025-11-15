/* eslint-disable import/no-dynamic-require */
import { mkdir, writeFile } from 'fs/promises';
import * as IQ from 'image-q';
import { createRequire } from 'module';
import path from 'path';
import sharp from 'sharp';
import { fileURLToPath } from 'url';
import { parentPort } from 'worker_threads';

const require = createRequire(import.meta.url);
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

type MessageTask = {
  type: 'task',
  id: number,
  pngArrayBuffer: ArrayBuffer,
  byteOffset: number,
  byteLength: number,
  blpPath: string,
};
type MessageShutdown = {type: 'shutdown'};
type Message = MessageTask | MessageShutdown;

const isValidMessage = (msg: unknown): msg is Message => {
  if (!msg || typeof msg !== 'object' || !('type' in msg)) {
    return false;
  }
  return true;
};

function run() {
  if (!parentPort) {
    return;
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let binding: any;
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
        } else if (process.platform === 'darwin' && process.arch === 'arm64') {
          binding = require(path.join(binDir, 'darwin-arm64-binding.node'));
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

  const { Image, TYPE_BLP } = binding || {};
  const mustUseJs = false;

  parentPort.on('message', (msg: unknown) => {
    if (!isValidMessage(msg)) return;

    if (msg.type === 'shutdown') {
      parentPort!.postMessage({ type: 'shutdown-ack' });
      process.exit(0);
    }

    if (msg.type !== 'task') return;

    const id: number = msg.id;
    const pngBufferOriginal = Buffer.from(new Uint8Array(msg.pngArrayBuffer, msg.byteOffset, msg.byteLength));
    const blpPath: string = msg.blpPath;

    void (async () => {
      try {
        // Normalize PNG alpha: if the entire alpha channel is 0, force it to 255 (opaque)
        // This matches WoW/WMO opaque rendering that ignores alpha and avoids black output in WC3.
        const pngBuffer = await ensureOpaqueIfAllAlphaZero(pngBufferOriginal);

        if (!Image || TYPE_BLP === undefined || mustUseJs) {
          await png2BlpJs(pngBuffer, blpPath);
          parentPort!.postMessage({ type: 'done', id, success: true });
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
        await mkdir(path.dirname(blpPath), { recursive: true });
        await writeFile(blpPath, blpBuffer);
        parentPort!.postMessage({ type: 'done', id, success: true });
      } catch (error: unknown) {
        parentPort!.postMessage({
          type: 'done', id, success: false, error: error instanceof Error ? error.message : String(error),
        });
      }
    })();
  });
}

void run();

/**
 * Faster variant of png2BlpJs using sharp for decoding and optimized quantization path.
 * - Decodes PNG via libvips (sharp) into raw RGBA
 * - Skips quantization entirely when unique RGBA colors ≤ 256
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

  // Pre-allocate buffers once
  const indices = Buffer.alloc(pixelCount);
  const alphaBuffer = Buffer.alloc(pixelCount);

  // Attempt fast-path: build palette directly if unique RGBA colors ≤ 256
  // Build mapping while scanning; abort early if unique > 256
  const colourToIndexFast = new Map<number, number>();
  const paletteBufferFast = Buffer.alloc(256 * 4, 0);
  let paletteSizeFast = 0;
  let exceededFastPath = false;

  for (let i = 0; i < pixelCount; i++) {
    const r = data[i * 4];
    const g = data[i * 4 + 1];
    const b = data[i * 4 + 2];
    const a = data[i * 4 + 3];

    // 32-bit RGBA key
    const key = (r << 24) | (g << 16) | (b << 8) | a;
    let idx = colourToIndexFast.get(key);
    if (idx === undefined) {
      if (paletteSizeFast === 256) {
        exceededFastPath = true;
        break;
      }
      idx = paletteSizeFast;
      colourToIndexFast.set(key, idx);
      // B, G, R, A in palette as per WC3 BLP expectation
      const p = idx * 4;
      paletteBufferFast[p] = b;
      paletteBufferFast[p + 1] = g;
      paletteBufferFast[p + 2] = r;
      paletteBufferFast[p + 3] = a;
      paletteSizeFast++;
    }
  }

  let paletteBuffer: Buffer;

  if (!exceededFastPath) {
    // Fast path succeeded – use the directly built palette and fill outputs in a second pass
    paletteBuffer = paletteBufferFast;

    for (let i = 0; i < pixelCount; i++) {
      const r = data[i * 4];
      const g = data[i * 4 + 1];
      const b = data[i * 4 + 2];
      const a = data[i * 4 + 3];
      const key = (r << 24) | (g << 16) | (b << 8) | a;
      const idx = colourToIndexFast.get(key)!;
      indices[i] = idx;
      alphaBuffer[i] = a;
    }
  } else {
    // Slow path: quantize using image-q with faster settings
    const pointContainer = IQ.utils.PointContainer.fromUint8Array(data, width, height);

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
    const colourToIndex = new Map<number, number>();
    for (let i = 0; i < palettePoints.length && i < 256; i++) {
      const p = palettePoints[i];
      const bufIndex = i * 4;
      paletteBuffer[bufIndex] = p.b;
      paletteBuffer[bufIndex + 1] = p.g;
      paletteBuffer[bufIndex + 2] = p.r;
      paletteBuffer[bufIndex + 3] = p.a;

      const key = (p.r << 24) | (p.g << 16) | (p.b << 8) | p.a;
      colourToIndex.set(key, i);
    }

    // Map quantized RGBA pixels to palette indices
    const quantRGBA = quantised.toUint8Array();
    for (let i = 0; i < pixelCount; i++) {
      const r = quantRGBA[i * 4];
      const g = quantRGBA[i * 4 + 1];
      const b = quantRGBA[i * 4 + 2];
      const a = quantRGBA[i * 4 + 3];
      const key = (r << 24) | (g << 16) | (b << 8) | a;
      const idx = colourToIndex.get(key) ?? 0;
      indices[i] = idx;
      alphaBuffer[i] = data[i * 4 + 3];
    }
  }

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
  header.writeUInt32LE(0, 24); // hasMipmaps (0)

  const offsetPos = 28;
  const sizePos = offsetPos + 64;
  const pixelDataOffset = BLP1_HEADER_SIZE + 1024; // palette follows header
  const pixelDataSize = indices.length + alphaBuffer.length;
  header.writeUInt32LE(pixelDataOffset, offsetPos); // mip0 offset
  header.writeUInt32LE(pixelDataSize, sizePos); // mip0 size

  // Assemble final file: header + palette + pixelIndices + alpha
  const blpBuffer = Buffer.concat([header, paletteBuffer, indices, alphaBuffer]);

  await mkdir(path.dirname(distPath), { recursive: true });
  await writeFile(distPath, blpBuffer);
}

/**
 * If the PNG's alpha channel is entirely zeros, force it to fully opaque (255).
 * This mirrors WoW's opaque material path (blendMode 0) which ignores alpha,
 * and prevents WC3 from rendering such textures as fully transparent/black.
 */
async function ensureOpaqueIfAllAlphaZero(pngBuffer: Buffer): Promise<Buffer> {
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

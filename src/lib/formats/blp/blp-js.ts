import fs from 'fs';
import * as IQ from 'image-q';
import path from 'path';
import { PNG } from 'pngjs';

/**
 * Encode a PNG into a Warcraft III compatible BLP v1 file (palette + 8-bit alpha).
 *
 * The produced texture contains: 256-colour palette (BGRA) + 8-bit alpha
 * channel. Only mip-level 0 is included.
 *
 * Reference spec: https://www.hiveworkshop.com/threads/blp-specifications-wc3.279306/ [[wowdev_wiki]]
 *
 * Notes:
 *  • Warcraft III ignores mip-maps for textures bigger than 512×512; providing
 *    none is fine.
 *  • If the PNG uses more than 256 unique colours the function will quantise
 *    them down using a median-cut algorithm (image-q).
 *
 * @param pngPath  Path to PNG source
 * @param distPath Destination *.blp* path
 */
export async function png2BlpJs(pngPath: string, distPath: string) {
  const pngBuffer = fs.readFileSync(pngPath);
  const png = PNG.sync.read(pngBuffer);
  const { width, height, data } = png;

  const pixelCount = width * height;

  // ----------------------------------------------------------------------------
  // Build palette (<=256 colours) and pixel indices
  // ----------------------------------------------------------------------------
  // Use image-q for quantisation if >256 colours.
  // eslint-disable-next-line @typescript-eslint/no-var-requires

  // PointContainer expects RGBA
  const pointContainer = IQ.utils.PointContainer.fromUint8Array(data, width, height);

  const palette = await IQ.buildPalette([pointContainer], { colors: 256 });
  const quantised = await IQ.applyPalette(pointContainer, palette, {});

  const palettePoints = palette.getPointContainer().getPointArray(); // Point[]

  // paletteBuffer initialised with zeros; fill up to palettePoints.length (<=256)
  const paletteBuffer = Buffer.alloc(256 * 4, 0);
  for (let i = 0; i < palettePoints.length && i < 256; i++) {
    const p = palettePoints[i];
    paletteBuffer[i * 4] = p.b;
    paletteBuffer[i * 4 + 1] = p.g;
    paletteBuffer[i * 4 + 2] = p.r;
    paletteBuffer[i * 4 + 3] = p.a; // alpha (WC3 ignores but store anyway)
  }

  // Pixel indices: map each quantised pixel colour back to its palette index.
  const quantRGBA = quantised.toUint8Array(); // length = pixelCount * 4
  const indices = Buffer.alloc(pixelCount);

  // Build lookup table colour key -> index for fast mapping.
  const colourToIndex = new Map<number, number>();
  for (let i = 0; i < palettePoints.length && i < 256; i++) {
    const p = palettePoints[i];
    const key = (p.r << 24) | (p.g << 16) | (p.b << 8) | p.a;
    colourToIndex.set(key, i);
  }

  for (let i = 0; i < pixelCount; i++) {
    const r = quantRGBA[i * 4];
    const g = quantRGBA[i * 4 + 1];
    const b = quantRGBA[i * 4 + 2];
    const a = quantRGBA[i * 4 + 3];
    const key = (r << 24) | (g << 16) | (b << 8) | a;
    const idx = colourToIndex.get(key) ?? 0;
    indices[i] = idx;
  }

  // Alpha channel separate (8-bit per pixel)
  const alphaBuffer = Buffer.alloc(pixelCount);
  for (let i = 0; i < pixelCount; i++) {
    alphaBuffer[i] = data[i * 4 + 3];
  }

  // ----------------------------------------------------------------------------
  // Header construction (BLP1)
  // ----------------------------------------------------------------------------
  const BLP1_HEADER_SIZE = 156; // 28 + 64 + 64

  const header = Buffer.alloc(BLP1_HEADER_SIZE, 0);
  header.write('BLP1', 0, 'ascii'); // 0-3 magic

  header.writeUInt32LE(1, 4); // 4-7 content = 1 (Direct / Palette)

  header.writeUInt32LE(8, 8); // 8-11 alphaBits = 8

  header.writeUInt32LE(width, 12); // 12-15 width
  header.writeUInt32LE(height, 16); // 16-19 height

  header.writeUInt32LE(0, 20); // 20-23 extra (unused)

  header.writeUInt32LE(0, 24); // 24-27 hasMipmaps (0)

  // Offsets start at byte 28
  const offsetPos = 28;
  const sizePos = offsetPos + 64;

  const pixelDataOffset = BLP1_HEADER_SIZE + 1024; // palette follows header
  const pixelDataSize = indices.length + alphaBuffer.length;

  header.writeUInt32LE(pixelDataOffset, offsetPos); // mip0 offset
  header.writeUInt32LE(pixelDataSize, sizePos); // mip0 size

  // ----------------------------------------------------------------------------
  // Assemble final file: header + palette + pixelIndices + alpha
  // ----------------------------------------------------------------------------
  const blpBuffer = Buffer.concat([header, paletteBuffer, indices, alphaBuffer]);

  fs.mkdirSync(path.dirname(distPath), { recursive: true });
  fs.writeFileSync(distPath, blpBuffer);
}

/**
 * CPU terrain bake, replacing the WebGL pipeline in wow.export's ADTExporter
 * (adt.vertex.shader + adt.fragment.shader).
 *
 * The GL pipeline rasterizes each map chunk's terrain mesh orthographically
 * onto a (quality/16)^2 canvas, blending up to 4 diffuse layers using 64x64
 * alpha maps and modulating by vertex colors. This port replicates that
 * rasterization (including GL texture filtering semantics: mipmapped
 * NEAREST_MIPMAP_LINEAR + REPEAT for diffuse layers, LINEAR + CLAMP_TO_EDGE
 * for alpha layers) on the CPU. Output is verified with pixel tolerance.
 */
import { getCasc } from '@/lib/wow/server/runtime';

import { BLPImage } from '../../formats/blp/blp';

export interface CPUMipTexture {
  /** mips[0] is the base level; each level is RGBA, row 0 = image top. */
  mips: { data: Uint8Array; width: number; height: number }[];
}

export interface BakeMaterial {
  scale: number;
  heightScale?: number;
  heightOffset?: number;
  diffuseTex?: CPUMipTexture;
  heightTex?: CPUMipTexture;
}

/** Load a texture from CASC, decode and build a mip chain (mirrors loadTexture + generateMipmap). */
export async function loadBakeTexture(fileDataID: number): Promise<CPUMipTexture> {
  const blp = new BLPImage(await getCasc().getFile(fileDataID));
  const base = blp.toUInt8Array(0);
  return { mips: buildMipChain(base, blp.scaledWidth, blp.scaledHeight) };
}

/** Build a box-filtered mip chain like GL generateMipmap. */
export function buildMipChain(data: Uint8Array, width: number, height: number): CPUMipTexture['mips'] {
  const mips: CPUMipTexture['mips'] = [{ data, width, height }];
  let cur = data;
  let w = width;
  let h = height;

  while (w > 1 || h > 1) {
    const nw = Math.max(1, w >> 1);
    const nh = Math.max(1, h >> 1);
    const next = new Uint8Array(nw * nh * 4);

    for (let y = 0; y < nh; y++) {
      const sy0 = Math.min(h - 1, y * 2);
      const sy1 = Math.min(h - 1, y * 2 + 1);
      for (let x = 0; x < nw; x++) {
        const sx0 = Math.min(w - 1, x * 2);
        const sx1 = Math.min(w - 1, x * 2 + 1);
        const i00 = (sy0 * w + sx0) * 4;
        const i10 = (sy0 * w + sx1) * 4;
        const i01 = (sy1 * w + sx0) * 4;
        const i11 = (sy1 * w + sx1) * 4;
        const o = (y * nw + x) * 4;
        for (let c = 0; c < 4; c++) next[o + c] = (cur[i00 + c] + cur[i10 + c] + cur[i01 + c] + cur[i11 + c] + 2) >> 2;
      }
    }

    mips.push({ data: next, width: nw, height: nh });
    cur = next;
    w = nw;
    h = nh;
  }

  return mips;
}

/** GLSL mod (floor-mod, always in [0, y)). */
function glslMod(x: number, y: number): number {
  return x - y * Math.floor(x / y);
}

/** Sample one mip level with NEAREST filtering and REPEAT wrap. */
function sampleNearestRepeat(mip: CPUMipTexture['mips'][number], u: number, v: number, out: Float64Array): void {
  const { data, width, height } = mip;
  let tx = Math.floor(glslMod(u, 1) * width);
  let ty = Math.floor(glslMod(v, 1) * height);
  if (tx >= width) tx = width - 1;
  if (ty >= height) ty = height - 1;
  const i = (ty * width + tx) * 4;
  out[0] = data[i] / 255;
  out[1] = data[i + 1] / 255;
  out[2] = data[i + 2] / 255;
  out[3] = data[i + 3] / 255;
}

const mipSampleA = new Float64Array(4);
const mipSampleB = new Float64Array(4);

/**
 * Sample a mipmapped texture with NEAREST_MIPMAP_LINEAR (GL default min
 * filter) + REPEAT wrap. `lod` is the level-of-detail computed per draw.
 */
export function sampleDiffuse(tex: CPUMipTexture, u: number, v: number, lod: number, out: Float64Array): void {
  const mips = tex.mips;
  if (lod <= 0) {
    // Magnification: LINEAR on base level with REPEAT.
    sampleLinearRepeat(mips[0], u, v, out);
    return;
  }

  const maxLevel = mips.length - 1;
  if (lod >= maxLevel) {
    sampleNearestRepeat(mips[maxLevel], u, v, out);
    return;
  }

  const d1 = Math.floor(lod);
  const frac = lod - d1;
  sampleNearestRepeat(mips[d1], u, v, mipSampleA);
  sampleNearestRepeat(mips[d1 + 1], u, v, mipSampleB);
  for (let c = 0; c < 4; c++) out[c] = mipSampleA[c] * (1 - frac) + mipSampleB[c] * frac;
}

/** Bilinear sample with REPEAT wrap. */
function sampleLinearRepeat(mip: CPUMipTexture['mips'][number], u: number, v: number, out: Float64Array): void {
  const { data, width, height } = mip;
  const fx = glslMod(u, 1) * width - 0.5;
  const fy = glslMod(v, 1) * height - 0.5;
  const x0 = Math.floor(fx);
  const y0 = Math.floor(fy);
  const dx = fx - x0;
  const dy = fy - y0;

  const wrap = (val: number, n: number): number => ((val % n) + n) % n;
  const cx0 = wrap(x0, width);
  const cx1 = wrap(x0 + 1, width);
  const cy0 = wrap(y0, height);
  const cy1 = wrap(y0 + 1, height);

  const i00 = (cy0 * width + cx0) * 4;
  const i10 = (cy0 * width + cx1) * 4;
  const i01 = (cy1 * width + cx0) * 4;
  const i11 = (cy1 * width + cx1) * 4;

  const w00 = (1 - dx) * (1 - dy);
  const w10 = dx * (1 - dy);
  const w01 = (1 - dx) * dy;
  const w11 = dx * dy;

  for (let c = 0; c < 4; c++) out[c] = (data[i00 + c] * w00 + data[i10 + c] * w10 + data[i01 + c] * w01 + data[i11 + c] * w11) / 255;
}

/** Bilinear sample of a single-channel 64x64 alpha layer with CLAMP_TO_EDGE. */
function sampleAlphaLinearClamp(layer: Uint8Array | number[], u: number, v: number): number {
  const size = 64;
  const fx = u * size - 0.5;
  const fy = v * size - 0.5;
  const x0 = Math.floor(fx);
  const y0 = Math.floor(fy);
  const dx = fx - x0;
  const dy = fy - y0;

  const clamp = (val: number): number => Math.min(size - 1, Math.max(0, val));
  const cx0 = clamp(x0);
  const cx1 = clamp(x0 + 1);
  const cy0 = clamp(y0);
  const cy1 = clamp(y0 + 1);

  const s00 = layer[cy0 * size + cx0] as number;
  const s10 = layer[cy0 * size + cx1] as number;
  const s01 = layer[cy1 * size + cx0] as number;
  const s11 = layer[cy1 * size + cx1] as number;

  return (s00 * (1 - dx) * (1 - dy) + s10 * dx * (1 - dy) + s01 * (1 - dx) * dy + s11 * dx * dy) / 255;
}

export interface ChunkBakeParams {
  /** Output RGBA canvas (chunkSizePx^2), row 0 = top; pre-filled black opaque. */
  canvas: Uint8ClampedArray;
  canvasSize: number;
  /** Triangle indices into the tile-global vertex arrays. */
  indices: number[];
  /** Tile-global vertex positions (x, y, z) per vertex. */
  vertices: number[];
  /** Tile-global bake UVs (uRaw, vRaw) per vertex. */
  uvsBake: number[];
  /** Tile-global vertex colors (4 floats per vertex, stored BGRA like wow.export). */
  vertexColors: number[];
  /** uTranslation for this chunk. */
  translation: [number, number];
  /** TILE_SIZE (uResolution). */
  tileSize: number;
  /** uZoom. */
  zoom: number;
  /** Per-layer materials for this chunk (index 0..3), undefined entries skipped. */
  layers: (BakeMaterial | undefined)[];
  /** Alpha layers for this chunk (index 1..3 used). */
  alphaLayers: (Uint8Array | number[] | undefined)[];
}

/**
 * Rasterize one map chunk onto its canvas, replicating the GL draw
 * (adt.vertex.shader + adt.fragment.shader semantics).
 */
export function bakeChunk(params: ChunkBakeParams): void {
  const {
    canvas, canvasSize, indices, vertices, uvsBake, vertexColors, translation, tileSize, zoom, layers, alphaLayers,
  } = params;

  const W = canvasSize;
  const H = canvasSize;

  // Pre-compute per-layer constants.
  const scales = [1, 1, 1, 1];
  for (let i = 0; i < 4; i++) {
    const mat = layers[i];
    if (mat) scales[i] = mat.scale;
  }

  // Constant LOD per draw: vTextureCoord spans 16 units across the full tile,
  // i.e. 1 unit per chunk => d(vT)/dpx = 1/W. tcK = vT * (8/scaleK).
  const lods = [0, 0, 0, 0];
  for (let i = 0; i < 4; i++) {
    const mat = layers[i];
    if (!mat?.diffuseTex) continue;
    const base = mat.diffuseTex.mips[0];
    const texelsPerPx = Math.max(base.width, base.height) * (8 / scales[i]) / W;
    lods[i] = Math.log2(Math.max(texelsPerPx, 1e-9));
  }

  // Transform a vertex into pixel space (matches adt.vertex.shader + viewport transform).
  // gl_Position = vec4(((pos.xz + uTranslation) / uResolution * 2 - 1) * (1, -1), 0, uZoom)
  // ndc = clip.xy / w; pixel = ((ndc.x + 1) / 2 * W, (1 - (ndc.y + 1) / 2) * H)
  const transform = (vi: number): [number, number] => {
    const x = vertices[vi * 3];
    const z = vertices[vi * 3 + 2];
    const cx = ((x + translation[0]) / tileSize) * 2 - 1;
    const cy = (((z + translation[1]) / tileSize) * 2 - 1) * -1;
    const ndcX = cx / zoom;
    const ndcY = cy / zoom;
    return [((ndcX + 1) / 2) * W, (1 - (ndcY + 1) / 2) * H];
  };

  const t0 = new Float64Array(4);
  const t1 = new Float64Array(4);
  const t2 = new Float64Array(4);
  const t3 = new Float64Array(4);

  for (let tri = 0; tri < indices.length; tri += 3) {
    const i0 = indices[tri];
    const i1 = indices[tri + 1];
    const i2 = indices[tri + 2];

    const p0 = transform(i0);
    const p1 = transform(i1);
    const p2 = transform(i2);

    // Signed area (CCW positive in our pixel space with y down => negative area
    // for GL CCW front faces; we rasterize regardless of winding).
    const area = (p1[0] - p0[0]) * (p2[1] - p0[1]) - (p2[0] - p0[0]) * (p1[1] - p0[1]);
    if (area === 0) continue;
    const invArea = 1 / area;

    const minX = Math.max(0, Math.floor(Math.min(p0[0], p1[0], p2[0])));
    const maxX = Math.min(W - 1, Math.ceil(Math.max(p0[0], p1[0], p2[0])));
    const minY = Math.max(0, Math.floor(Math.min(p0[1], p1[1], p2[1])));
    const maxY = Math.min(H - 1, Math.ceil(Math.max(p0[1], p1[1], p2[1])));

    for (let yPix = minY; yPix <= maxY; yPix++) {
      const sy = yPix + 0.5;
      for (let xPix = minX; xPix <= maxX; xPix++) {
        const sx = xPix + 0.5;

        // Barycentric coordinates.
        const w0 = ((p1[0] - sx) * (p2[1] - sy) - (p2[0] - sx) * (p1[1] - sy)) * invArea;
        const w1 = ((p2[0] - sx) * (p0[1] - sy) - (p0[0] - sx) * (p2[1] - sy)) * invArea;
        const w2 = 1 - w0 - w1;

        if (w0 < 0 || w1 < 0 || w2 < 0) continue;

        // Interpolated bake UV -> vTextureCoord = uvBake * (16, -16).
        const u = uvsBake[i0 * 2] * w0 + uvsBake[i1 * 2] * w1 + uvsBake[i2 * 2] * w2;
        const v = uvsBake[i0 * 2 + 1] * w0 + uvsBake[i1 * 2 + 1] * w1 + uvsBake[i2 * 2 + 1] * w2;
        const vtU = u * 16;
        const vtV = v * -16;

        // Interpolated vertex color (.rgb used by the shader).
        const vcR = vertexColors[i0 * 4] * w0 + vertexColors[i1 * 4] * w1 + vertexColors[i2 * 4] * w2;
        const vcG = vertexColors[i0 * 4 + 1] * w0 + vertexColors[i1 * 4 + 1] * w1 + vertexColors[i2 * 4 + 1] * w2;
        const vcB = vertexColors[i0 * 4 + 2] * w0 + vertexColors[i1 * 4 + 2] * w1 + vertexColors[i2 * 4 + 2] * w2;

        // Alpha blends (64x64 layers, LINEAR + CLAMP_TO_EDGE at mod(vT, 1)).
        const modU = glslMod(vtU, 1);
        const modV = glslMod(vtV, 1);
        const a0 = alphaLayers[1] ? sampleAlphaLinearClamp(alphaLayers[1], modU, modV) : 0;
        const a1 = alphaLayers[2] ? sampleAlphaLinearClamp(alphaLayers[2], modU, modV) : 0;
        const a2 = alphaLayers[3] ? sampleAlphaLinearClamp(alphaLayers[3], modU, modV) : 0;

        // Diffuse layers.
        if (layers[0]?.diffuseTex) sampleDiffuse(layers[0].diffuseTex, vtU * (8 / scales[0]), vtV * (8 / scales[0]), lods[0], t0);
        else t0.fill(0);
        if (layers[1]?.diffuseTex) sampleDiffuse(layers[1].diffuseTex, vtU * (8 / scales[1]), vtV * (8 / scales[1]), lods[1], t1);
        else t1.fill(0);
        if (layers[2]?.diffuseTex) sampleDiffuse(layers[2].diffuseTex, vtU * (8 / scales[2]), vtV * (8 / scales[2]), lods[2], t2);
        else t2.fill(0);
        if (layers[3]?.diffuseTex) sampleDiffuse(layers[3].diffuseTex, vtU * (8 / scales[3]), vtV * (8 / scales[3]), lods[3], t3);
        else t3.fill(0);

        const baseW = 1 - (a0 + a1 + a2);
        const r = (t0[0] * baseW + t1[0] * a0 + t2[0] * a1 + t3[0] * a2) * vcR * 2;
        const g = (t0[1] * baseW + t1[1] * a0 + t2[1] * a1 + t3[1] * a2) * vcG * 2;
        const b = (t0[2] * baseW + t1[2] * a0 + t2[2] * a1 + t3[2] * a2) * vcB * 2;

        const o = (yPix * W + xPix) * 4;
        canvas[o] = r * 255;
        canvas[o + 1] = g * 255;
        canvas[o + 2] = b * 255;
        canvas[o + 3] = 255;
      }
    }
  }
}

/** Rotate an RGBA image 180 degrees (replaces the rotation canvas). */
export function rotate180(src: Uint8ClampedArray, size: number): Uint8ClampedArray {
  const out = new Uint8ClampedArray(src.length);
  const n = size * size;
  for (let i = 0; i < n; i++) {
    const j = n - 1 - i;
    out[j * 4] = src[i * 4];
    out[j * 4 + 1] = src[i * 4 + 1];
    out[j * 4 + 2] = src[i * 4 + 2];
    out[j * 4 + 3] = src[i * 4 + 3];
  }
  return out;
}

/** Bilinear-resize an RGBA image (replaces canvas drawImage scaling for minimaps). */
export function resizeBilinear(src: Uint8Array | Uint8ClampedArray, srcW: number, srcH: number, dstW: number, dstH: number): Uint8ClampedArray {
  const out = new Uint8ClampedArray(dstW * dstH * 4);
  const xRatio = srcW / dstW;
  const yRatio = srcH / dstH;

  for (let y = 0; y < dstH; y++) {
    const fy = (y + 0.5) * yRatio - 0.5;
    const y0 = Math.max(0, Math.floor(fy));
    const y1 = Math.min(srcH - 1, y0 + 1);
    const dy = fy - Math.floor(fy);
    for (let x = 0; x < dstW; x++) {
      const fx = (x + 0.5) * xRatio - 0.5;
      const x0 = Math.max(0, Math.floor(fx));
      const x1 = Math.min(srcW - 1, x0 + 1);
      const dx = fx - Math.floor(fx);

      const i00 = (y0 * srcW + x0) * 4;
      const i10 = (y0 * srcW + x1) * 4;
      const i01 = (y1 * srcW + x0) * 4;
      const i11 = (y1 * srcW + x1) * 4;
      const o = (y * dstW + x) * 4;

      for (let c = 0; c < 4; c++) {
        out[o + c] = src[i00 + c] * (1 - dx) * (1 - dy)
          + src[i10 + c] * dx * (1 - dy)
          + src[i01 + c] * (1 - dx) * dy
          + src[i11 + c] * dx * dy;
      }
    }
  }

  return out;
}

export default {
  loadBakeTexture, buildMipChain, bakeChunk, rotate180, resizeBilinear, sampleDiffuse,
};

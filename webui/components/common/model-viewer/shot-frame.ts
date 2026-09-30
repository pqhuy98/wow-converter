import { vec3 } from 'gl-matrix';

/** The model fills this fraction of the limiting screen axis. */
export const SHOT_FILL = 0.92;

export interface ExtentBox {
  readonly min: readonly [number, number, number]
  readonly max: readonly [number, number, number]
}

export interface ShotExtents {
  readonly model: ExtentBox
  readonly sequences: readonly ExtentBox[]
  /** Bind-pose positions, xyz packed. The silhouette is fitted to these, not to empty extent corners. */
  readonly points: Float32Array
}

/** Camera fields the shot framer moves. Matches mdx-m3-viewer `Camera`. */
export interface ShotCamera {
  fov: number
  nearClipPlane: number
  readonly viewProjectionMatrix: Float32Array
  moveToAndFace(from: vec3, to: vec3, worldUp: vec3): void
}

/** Direction from the target to the camera, then the camera's world up. */
export type ShotOffset = readonly [number, number, number, number, number, number]

const SEQUENCE_STRIDE = 132;
const SEQUENCE_EXTENT = 104;
const MODEL_EXTENT = 340;

export function usefulExtent(box: ExtentBox): boolean {
  return box.max[0] > box.min[0] || box.max[1] > box.min[1] || box.max[2] > box.min[2];
}

/** Model extent plus one box per sequence, in file order. */
export function readMdxExtents(data: ArrayBuffer): ShotExtents {
  const view = new DataView(data);
  const bytes = new Uint8Array(data);
  let model: ExtentBox = { min: [0, 0, 0], max: [0, 0, 0] };
  const sequences: ExtentBox[] = [];
  const parts: Float32Array[] = [];
  let pointCount = 0;
  let offset = 4;
  while (offset + 8 <= data.byteLength) {
    const tag = String.fromCharCode(bytes[offset] ?? 0, bytes[offset + 1] ?? 0, bytes[offset + 2] ?? 0, bytes[offset + 3] ?? 0);
    const size = view.getUint32(offset + 4, true);
    const start = offset + 8;
    if (start + size > data.byteLength) break;
    if (tag === 'MODL' && size >= MODEL_EXTENT + 28) {
      model = readExtent(view, start + MODEL_EXTENT);
    } else if (tag === 'SEQS') {
      const count = Math.floor(size / SEQUENCE_STRIDE);
      for (let i = 0; i < count; i++) {
        sequences.push(readExtent(view, start + i * SEQUENCE_STRIDE + SEQUENCE_EXTENT));
      }
    } else if (tag === 'GEOS') {
      let at = start;
      const end = start + size;
      while (at + 12 <= end) {
        const geosetSize = view.getUint32(at, true);
        if (geosetSize < 12 || at + geosetSize > end) break;
        const vrtx = String.fromCharCode(bytes[at + 4] ?? 0, bytes[at + 5] ?? 0, bytes[at + 6] ?? 0, bytes[at + 7] ?? 0);
        if (vrtx === 'VRTX') {
          const count = view.getUint32(at + 8, true);
          const floats = count * 3;
          if (at + 12 + floats * 4 <= at + geosetSize) {
            const verts = new Float32Array(floats);
            for (let i = 0; i < floats; i++) verts[i] = view.getFloat32(at + 12 + i * 4, true);
            parts.push(verts);
            pointCount += count;
          }
        }
        at += geosetSize;
      }
    }
    offset = start + size;
  }
  const points = new Float32Array(pointCount * 3);
  let written = 0;
  for (const part of parts) {
    points.set(part, written);
    written += part.length;
  }
  return { model, sequences, points };
}

/**
 * Place the camera so the extent fills `SHOT_FILL` of the limiting axis.
 * Front/side views fit the silhouette; a top view fits the footprint.
 */
export function frameShotCamera(camera: ShotCamera, box: ExtentBox, offset: ShotOffset, points?: Float32Array): void {
  const fitted = points && points.length >= 3 ? boundsOf(points) : box;
  const samples = points && points.length >= 3 ? points : boxCorners(box);
  const target = vec3.fromValues(
    (fitted.min[0] + fitted.max[0]) / 2,
    (fitted.min[1] + fitted.max[1]) / 2,
    (fitted.min[2] + fitted.max[2]) / 2,
  );
  const along = Math.abs(offset[0]) * (fitted.max[0] - fitted.min[0])
    + Math.abs(offset[1]) * (fitted.max[1] - fitted.min[1])
    + Math.abs(offset[2]) * (fitted.max[2] - fitted.min[2]);
  const near = camera.nearClipPlane > 0 ? camera.nearClipPlane : 8;
  let lo = along / 2 + near + 1;
  let hi = Math.max(lo * 2, along * 4, 32);
  for (let i = 0; i < 24 && maxAbsNdc(camera, target, offset, samples, hi) > SHOT_FILL; i++) {
    hi *= 1.7;
  }
  for (let i = 0; i < 28; i++) {
    const mid = (lo + hi) / 2;
    if (maxAbsNdc(camera, target, offset, samples, mid) <= SHOT_FILL) hi = mid;
    else lo = mid;
  }
  place(camera, target, offset, hi);
}

function readExtent(view: DataView, offset: number): ExtentBox {
  return {
    min: [view.getFloat32(offset + 4, true), view.getFloat32(offset + 8, true), view.getFloat32(offset + 12, true)],
    max: [view.getFloat32(offset + 16, true), view.getFloat32(offset + 20, true), view.getFloat32(offset + 24, true)],
  };
}

function boundsOf(points: Float32Array): ExtentBox {
  let minX = Number.POSITIVE_INFINITY;
  let minY = Number.POSITIVE_INFINITY;
  let minZ = Number.POSITIVE_INFINITY;
  let maxX = Number.NEGATIVE_INFINITY;
  let maxY = Number.NEGATIVE_INFINITY;
  let maxZ = Number.NEGATIVE_INFINITY;
  for (let i = 0; i < points.length; i += 3) {
    const x = points[i] ?? 0;
    const y = points[i + 1] ?? 0;
    const z = points[i + 2] ?? 0;
    if (x < minX) minX = x;
    if (y < minY) minY = y;
    if (z < minZ) minZ = z;
    if (x > maxX) maxX = x;
    if (y > maxY) maxY = y;
    if (z > maxZ) maxZ = z;
  }
  return { min: [minX, minY, minZ], max: [maxX, maxY, maxZ] };
}

function boxCorners(box: ExtentBox): Float32Array {
  const corners = new Float32Array(8 * 3);
  let at = 0;
  for (const x of [box.min[0], box.max[0]]) {
    for (const y of [box.min[1], box.max[1]]) {
      for (const z of [box.min[2], box.max[2]]) {
        corners[at] = x;
        corners[at + 1] = y;
        corners[at + 2] = z;
        at += 3;
      }
    }
  }
  return corners;
}

function maxAbsNdc(
  camera: ShotCamera,
  target: vec3,
  offset: ShotOffset,
  points: Float32Array,
  distance: number,
): number {
  place(camera, target, offset, distance);
  const m = camera.viewProjectionMatrix;
  let max = 0;
  for (let i = 0; i < points.length; i += 3) {
    const x = points[i] ?? 0;
    const y = points[i + 1] ?? 0;
    const z = points[i + 2] ?? 0;
    const w = m[3] * x + m[7] * y + m[11] * z + m[15];
    if (w <= 1e-3) return Number.POSITIVE_INFINITY;
    const inv = 1 / w;
    const ndcX = Math.abs((m[0] * x + m[4] * y + m[8] * z + m[12]) * inv);
    const ndcY = Math.abs((m[1] * x + m[5] * y + m[9] * z + m[13]) * inv);
    if (ndcX > max) max = ndcX;
    if (ndcY > max) max = ndcY;
  }
  return max;
}

function place(camera: ShotCamera, target: vec3, offset: ShotOffset, distance: number): void {
  camera.moveToAndFace(
    vec3.fromValues(target[0] + distance * offset[0], target[1] + distance * offset[1], target[2] + distance * offset[2]),
    target,
    vec3.fromValues(offset[3], offset[4], offset[5]),
  );
}

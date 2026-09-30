import Camera from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/camera';
import { describe, expect, test } from 'bun:test';
import { vec3, vec4 } from 'gl-matrix';

import {
  frameShotCamera, readMdxExtents, SHOT_FILL, type ExtentBox, type ShotOffset,
} from '../webui/components/common/model-viewer/shot-frame';

const FRONT: ShotOffset = [1, 0, 0, 0, 0, 1];
const TOP: ShotOffset = [0, 0, 1, 1, 0, 0];

describe('shot framing', () => {
  test('reads model and sequence extents', () => {
    const model = chunk('MODL', payload(372, 340, [1, 2, 3], [4, 6, 9]));
    const seq = chunk('SEQS', payload(132, 104, [-2, -1, 0], [2, 1, 8]));
    const file = new Uint8Array(4 + model.length + seq.length);
    file.set(new TextEncoder().encode('MDLX'), 0);
    file.set(model, 4);
    file.set(seq, 4 + model.length);
    const extents = readMdxExtents(file.buffer);
    expect(extents.model).toEqual({ min: [1, 2, 3], max: [4, 6, 9] });
    expect(extents.sequences).toEqual([{ min: [-2, -1, 0], max: [2, 1, 8] }]);
  });

  test('a tall box fills the tile height from the front and the footprint from above', () => {
    const box: ExtentBox = { min: [-10, -10, 0], max: [10, 10, 100] };
    const front = frame(box, FRONT);
    const top = frame(box, TOP);
    expect(front.maxNdc).toBeGreaterThan(SHOT_FILL - 0.02);
    expect(front.maxNdc).toBeLessThanOrEqual(SHOT_FILL + 0.005);
    expect(top.maxNdc).toBeGreaterThan(SHOT_FILL - 0.02);
    expect(top.maxNdc).toBeLessThanOrEqual(SHOT_FILL + 0.005);
    expect(front.vertical).toBeGreaterThan(front.horizontal * 2);
    expect(top.distance).toBeLessThan(front.distance);
  });

  test('fits vertices that sit inside the extent', () => {
    const box: ExtentBox = { min: [-100, -100, 0], max: [100, 100, 10] };
    const points = new Float32Array([-5, -5, 0, 5, -5, 0, -5, 5, 0, 5, 5, 10]);
    const camera = new Camera();
    camera.perspective(Math.PI / 4, 640 / 400, 8, 1_000_000);
    frameShotCamera(camera, box, FRONT, points);
    let maxPoint = 0;
    let maxCorner = 0;
    const clip = vec4.create();
    const sample = (x: number, y: number, z: number): number => {
      vec4.set(clip, x, y, z, 1);
      vec4.transformMat4(clip, clip, camera.viewProjectionMatrix);
      return Math.max(Math.abs(clip[0] / clip[3]), Math.abs(clip[1] / clip[3]));
    };
    for (let i = 0; i < points.length; i += 3) {
      maxPoint = Math.max(maxPoint, sample(points[i] ?? 0, points[i + 1] ?? 0, points[i + 2] ?? 0));
    }
    maxCorner = sample(100, 100, 10);
    expect(maxPoint).toBeGreaterThan(SHOT_FILL - 0.02);
    expect(maxPoint).toBeLessThanOrEqual(SHOT_FILL + 0.005);
    expect(maxCorner).toBeGreaterThan(1);
  });
});

function frame(box: ExtentBox, offset: ShotOffset): { maxNdc: number; horizontal: number; vertical: number; distance: number } {
  const camera = new Camera();
  camera.perspective(Math.PI / 4, 640 / 400, 8, 1_000_000);
  frameShotCamera(camera, box, offset);
  const target = vec3.fromValues(0, 0, 50);
  let maxNdc = 0;
  let horizontal = 0;
  let vertical = 0;
  const clip = vec4.create();
  for (const x of [box.min[0], box.max[0]]) {
    for (const y of [box.min[1], box.max[1]]) {
      for (const z of [box.min[2], box.max[2]]) {
        vec4.set(clip, x, y, z, 1);
        vec4.transformMat4(clip, clip, camera.viewProjectionMatrix);
        const ndcX = Math.abs(clip[0] / clip[3]);
        const ndcY = Math.abs(clip[1] / clip[3]);
        if (ndcX > horizontal) horizontal = ndcX;
        if (ndcY > vertical) vertical = ndcY;
        if (ndcX > maxNdc) maxNdc = ndcX;
        if (ndcY > maxNdc) maxNdc = ndcY;
      }
    }
  }
  return {
    maxNdc, horizontal, vertical, distance: vec3.distance(camera.location, target),
  };
}

function payload(size: number, extentAt: number, min: readonly [number, number, number], max: readonly [number, number, number]): Uint8Array {
  const bytes = new Uint8Array(size);
  const view = new DataView(bytes.buffer);
  view.setFloat32(extentAt + 4, min[0], true);
  view.setFloat32(extentAt + 8, min[1], true);
  view.setFloat32(extentAt + 12, min[2], true);
  view.setFloat32(extentAt + 16, max[0], true);
  view.setFloat32(extentAt + 20, max[1], true);
  view.setFloat32(extentAt + 24, max[2], true);
  return bytes;
}

function chunk(tag: string, body: Uint8Array): Uint8Array {
  const out = new Uint8Array(8 + body.length);
  out.set(new TextEncoder().encode(tag), 0);
  new DataView(out.buffer).setUint32(4, body.length, true);
  out.set(body, 8);
  return out;
}

import { Vector3 } from '@/lib/math/common';

import { GeosetVertex } from '../components/geoset';
import { iterateVerticesAtTimestamp } from '../mdl-traverse';
import { MDLModify } from '.';

export function addCollisionShapes(this: MDLModify) {
  // Iterate all vertices at Stand sequence (or first sequence)
  const seq = this.mdl.sequences.find((s) => s.name === 'Stand') ?? this.mdl.sequences[0];

  const cloud: Vector3[] = [];
  const vToPos = new Map<GeosetVertex, Vector3>();
  const timestamp = seq.interval[0];
  iterateVerticesAtTimestamp(this.mdl, seq, timestamp, (v, vPos) => {
    if (!Number.isFinite(vPos[0]) || !Number.isFinite(vPos[1]) || !Number.isFinite(vPos[2])) return;
    vToPos.set(v, vPos);
  });

  // Also sample triangle interior to avoid gaps between vertices-only coverage
  this.mdl.geosets.forEach((geoset) => {
    geoset.faces.forEach((face) => {
      const v0 = vToPos.get(face.vertices[0]);
      const v1 = vToPos.get(face.vertices[1]);
      const v2 = vToPos.get(face.vertices[2]);
      if (!v0 || !v1 || !v2) return;
      const centroid: Vector3 = [
        (v0[0] + v1[0] + v2[0]) / 3,
        (v0[1] + v1[1] + v2[1]) / 3,
        (v0[2] + v1[2] + v2[2]) / 3,
      ];
      if (Number.isFinite(centroid[0]) && Number.isFinite(centroid[1]) && Number.isFinite(centroid[2])) {
        cloud.push(centroid);
      }
    });
  });

  // Fallback: if nothing sampled (e.g., unusual animation data), use static vertex positions
  if (cloud.length === 0) {
    this.mdl.geosets.forEach((geoset) => {
      geoset.vertices.forEach((v) => {
        const p = v.position;
        if (Number.isFinite(p[0]) && Number.isFinite(p[1]) && Number.isFinite(p[2])) cloud.push([p[0], p[1], p[2]]);
      });
    });
  }

  // Fallback: still empty – use model extents and produce one inflated box
  if (cloud.length === 0) {
    const BUFFER_RADIUS = 25;
    const min = [
      this.mdl.model.minimumExtent[0] - BUFFER_RADIUS,
      this.mdl.model.minimumExtent[1] - BUFFER_RADIUS,
      this.mdl.model.minimumExtent[2] - BUFFER_RADIUS,
    ] as Vector3;
    const max = [
      this.mdl.model.maximumExtent[0] + BUFFER_RADIUS,
      this.mdl.model.maximumExtent[1] + BUFFER_RADIUS,
      this.mdl.model.maximumExtent[2] + BUFFER_RADIUS,
    ] as Vector3;
    this.mdl.collisionShapes = [
      {
        name: 'Collision Box01',
        type: 'Box',
        vertices: [[...min], [...max]],
        boundRadius: 0,
        pivotPoint: [0, 0, 0],
        flags: [],
      },
    ];
    return this;
  }

  if (cloud.length === 0) return this;

  // Treat each point as a sphere of radius BUFFER_RADIUS (inflate AABBs)
  const BUFFER_RADIUS = 5;

  type Cluster = { pointsIdx: number[]; rawMin: Vector3; rawMax: Vector3; min: Vector3; max: Vector3; volume: number };

  const computeAABB = (idxs: number[]) => {
    const rawMin: Vector3 = [Infinity, Infinity, Infinity];
    const rawMax: Vector3 = [-Infinity, -Infinity, -Infinity];
    for (let i = 0; i < idxs.length; i += 1) {
      const p = cloud[idxs[i]];
      if (p[0] < rawMin[0]) rawMin[0] = p[0]; if (p[0] > rawMax[0]) rawMax[0] = p[0];
      if (p[1] < rawMin[1]) rawMin[1] = p[1]; if (p[1] > rawMax[1]) rawMax[1] = p[1];
      if (p[2] < rawMin[2]) rawMin[2] = p[2]; if (p[2] > rawMax[2]) rawMax[2] = p[2];
    }
    const min: Vector3 = [rawMin[0] - BUFFER_RADIUS, rawMin[1] - BUFFER_RADIUS, rawMin[2] - BUFFER_RADIUS];
    const max: Vector3 = [rawMax[0] + BUFFER_RADIUS, rawMax[1] + BUFFER_RADIUS, rawMax[2] + BUFFER_RADIUS];
    const dx = Math.max(0, max[0] - min[0]);
    const dy = Math.max(0, max[1] - min[1]);
    const dz = Math.max(0, max[2] - min[2]);
    const volume = dx * dy * dz;
    return {
      rawMin, rawMax, min, max, volume,
    };
  };

  const initialIdxs = cloud.map((_, i) => i);
  const initialBox = computeAABB(initialIdxs);
  const clusters: Cluster[] = [{
    pointsIdx: initialIdxs, rawMin: initialBox.rawMin, rawMax: initialBox.rawMax, min: initialBox.min, max: initialBox.max, volume: initialBox.volume,
  }];

  const MAX_SHAPES = 3;
  const NUM_BINS = 16; // candidate split thresholds per axis

  while (clusters.length < MAX_SHAPES) {
    let bestGain = 0;
    type SplitResult = { clusterIndex: number; left: Cluster; right: Cluster };
    let bestSplit: SplitResult | null = null;

    for (let ci = 0; ci < clusters.length; ci += 1) {
      const c = clusters[ci];
      const tryAxis = (axis: 0 | 1 | 2) => {
        const minVal = c.rawMin[axis];
        const maxVal = c.rawMax[axis];
        if (!(maxVal > minVal)) return;

        for (let b = 1; b < NUM_BINS; b += 1) {
          const thr = minVal + (b * (maxVal - minVal)) / NUM_BINS;
          const leftIdx: number[] = [];
          const rightIdx: number[] = [];
          for (let k = 0; k < c.pointsIdx.length; k += 1) {
            const idx = c.pointsIdx[k];
            const v = cloud[idx][axis];
            if (v <= thr) leftIdx.push(idx);
            else rightIdx.push(idx);
          }
          if (leftIdx.length === 0 || rightIdx.length === 0) continue;
          const l = computeAABB(leftIdx);
          const r = computeAABB(rightIdx);
          const gain = c.volume - (l.volume + r.volume);
          if (gain > bestGain) {
            bestGain = gain;
            bestSplit = {
              clusterIndex: ci,
              left: {
                pointsIdx: leftIdx, rawMin: l.rawMin, rawMax: l.rawMax, min: l.min, max: l.max, volume: l.volume,
              },
              right: {
                pointsIdx: rightIdx, rawMin: r.rawMin, rawMax: r.rawMax, min: r.min, max: r.max, volume: r.volume,
              },
            };
          }
        }
      };
      tryAxis(0);
      tryAxis(1);
      tryAxis(2);
    }

    if (!bestSplit || bestGain <= 0) break;

    const bs = bestSplit as SplitResult;
    clusters.splice(bs.clusterIndex, 1, bs.left, bs.right);
  }

  this.mdl.collisionShapes = clusters.map((c, i) => ({
    name: `Collision Box${String(i + 1).padStart(2, '0')}`,
    type: 'Box',
    vertices: [
      [...c.min],
      [...c.max],
    ],
    boundRadius: 0,
    pivotPoint: [0, 0, 0],
    flags: [],
  }));

  return this;
}

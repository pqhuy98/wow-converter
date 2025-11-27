import { Vector3 } from '@/lib/math/common';

import { Face, GeosetVertex } from '../components/geoset';
import { MDLModify } from '.';

export function recomputeNormals(this: MDLModify): MDLModify {
  const sub = (a: Vector3, b: Vector3): Vector3 => [a[0] - b[0], a[1] - b[1], a[2] - b[2]];
  const cross = (a: Vector3, b: Vector3): Vector3 => [
    a[1] * b[2] - a[2] * b[1],
    a[2] * b[0] - a[0] * b[2],
    a[0] * b[1] - a[1] * b[0],
  ];
  const length = (v: Vector3): number => Math.sqrt(v[0] * v[0] + v[1] * v[1] + v[2] * v[2]);

  // Use a small spatial quantization so vertices that should be shared
  // (e.g. at ADT chunk borders) are grouped even if their positions
  // differ by tiny floating-point noise.
  const POS_EPS = 1e-2; // be tolerant to tiny offsets introduced by exporters
  const q = (x: number) => Math.round(x / POS_EPS) * POS_EPS;
  const keyFromPosition = (pos: Vector3): string => `${q(pos[0])}|${q(pos[1])}|${q(pos[2])}`;

  // Build adjacency: which faces use which vertex
  const vertexToFaces = new Map<GeosetVertex, Face[]>();
  // Group vertices by (quantized) position
  const positionToVertices = new Map<string, GeosetVertex[]>();

  this.mdl.geosets.forEach((geoset) => {
    geoset.vertices.forEach((vert) => {
      // Reset normals
      vert.normal = [0, 0, 0];

      const key = keyFromPosition(vert.position);
      let list = positionToVertices.get(key);
      if (!list) {
        list = [];
        positionToVertices.set(key, list);
      }
      list.push(vert);
    });

    geoset.faces.forEach((face) => {
      face.vertices.forEach((vert) => {
        let list = vertexToFaces.get(vert);
        if (!list) {
          list = [];
          vertexToFaces.set(vert, list);
        }
        list.push(face);
      });
    });
  });

  // For each group of vertices that share (quantized) position,
  // accumulate area-weighted face normals and assign the same
  // normalized result to all duplicates. This smooths seams on chunk
  // borders inside a tile.
  positionToVertices.forEach((verticesAtPos) => {
    const sum: Vector3 = [0, 0, 0];

    verticesAtPos.forEach((vert) => {
      const faces = vertexToFaces.get(vert) ?? [];

      faces.forEach((face) => {
        const [v0, v1, v2] = face.vertices;
        const p0 = v0.position;
        const p1 = v1.position;
        const p2 = v2.position;

        // Unnormalized face normal (proportional to area)
        const e1 = sub(p0, p1);
        const e2 = sub(p1, p2);
        const perp = cross(e1, e2);

        sum[0] += perp[0];
        sum[1] += perp[1];
        sum[2] += perp[2];
      });
    });

    let sumMag = length(sum);
    if (sumMag === 0) {
      // Degenerate: fallback to +Z normal
      sum[0] = 0; sum[1] = 0; sum[2] = 1;
      sumMag = 1;
    }

    const normal: Vector3 = [sum[0] / sumMag, sum[1] / sumMag, sum[2] / sumMag];

    verticesAtPos.forEach((vert) => {
      vert.normal = [normal[0], normal[1], normal[2]];
    });
  });

  return this;
}

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
  const epsilon = 0.00001;

  // Build adjacency: which faces use which vertex
  const vertexToFaces = new Map<GeosetVertex, Face[]>();
  // Group vertices by exact position (x, y, z)
  const positionToVertices = new Map<string, GeosetVertex[]>();

  const keyFromPosition = (pos: Vector3): string => `${pos[0]}|${pos[1]}|${pos[2]}`;

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

  // For each group of vertices that share the same position,
  // reproduce GeosetVertex.createNormal(matches) from the Java code.
  positionToVertices.forEach((verticesAtPos) => {
    const sum: Vector3 = [0, 0, 0];

    verticesAtPos.forEach((vert) => {
      const faces = vertexToFaces.get(vert) ?? [];

      faces.forEach((face) => {
        const [v0, v1, v2] = face.vertices;
        const p0 = v0.position;
        const p1 = v1.position;
        const p2 = v2.position;

        // Equivalent to triangle.verts[0].delta(verts[1]).crossProduct(verts[1].delta(verts[2]))
        const e1 = sub(p0, p1);
        const e2 = sub(p1, p2);
        const perp = cross(e1, e2);

        let mag = length(perp);
        if (mag === 0) {
          mag = epsilon;
        }

        const nx = perp[0] / mag;
        const ny = perp[1] / mag;
        const nz = perp[2] / mag;

        sum[0] += nx;
        sum[1] += ny;
        sum[2] += nz;
      });
    });

    let sumMag = length(sum);
    if (sumMag === 0) {
      sumMag = epsilon;
    }

    const normal: Vector3 = [sum[0] / sumMag, sum[1] / sumMag, sum[2] / sumMag];

    verticesAtPos.forEach((vert) => {
      vert.normal = [normal[0], normal[1], normal[2]];
    });
  });

  return this;
}

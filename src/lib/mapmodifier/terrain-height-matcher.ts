import _ from 'lodash';
import { parseMDL } from 'war3-model';

import { Doodad, Terrain } from '@/vendors/wc3maptranslator/data';

import { distancePerTile } from '../constants';
import { dataHeightMax, dataHeightMin } from '../global-config';
import { Vector3 } from '../math/common';
import { calculateTriangleSlope, getZProjectionOfXyInTriangle } from '../math/geometry';
import { radians } from '../math/rotation';
import { V3 } from '../math/vector';
import {
  gameZToDataHeight,
  maxGameHeightDiff,
  nArray,
} from '../utils';

const floodBrushSize = 2;
const slopeThreshold = 45;

interface Face {
  vertices: [Vector3, Vector3, Vector3];
  geosetId: number
}

type Vertex2D = [number, number];

export function matchTerrainToDoodadHeights(terrain: Terrain, doodadModels: [Doodad, string][]) {
  const offset: Vector3 = [terrain.map.offset.x, terrain.map.offset.y, 0];

  const allFaces = doodadModels.flatMap(([doodad, mdlStr]) => {
    const mdl = parseMDL(mdlStr);
    return mdl.Geosets.flatMap(((g, geosetId) => {
      const faces: Face[] = [];
      for (let i = 0; i < g.Faces.length; i += 3) {
        const id1 = g.Faces[i] * 3;
        const v1: Vector3 = [
          g.Vertices[id1],
          g.Vertices[id1 + 1],
          g.Vertices[id1 + 2],
        ];
        const id2 = g.Faces[i + 1] * 3;
        const v2: Vector3 = [
          g.Vertices[id2],
          g.Vertices[id2 + 1],
          g.Vertices[id2 + 2],
        ];
        const id3 = g.Faces[i + 2] * 3;
        const v3: Vector3 = [
          g.Vertices[id3],
          g.Vertices[id3 + 1],
          g.Vertices[id3 + 2],
        ];
        faces.push({
          vertices: [
            V3.sum(V3.sub(doodad.position, offset), V3.rotate(V3.mul(v1, doodad.scale), [0, 0, radians(doodad.angle)])),
            V3.sum(V3.sub(doodad.position, offset), V3.rotate(V3.mul(v2, doodad.scale), [0, 0, radians(doodad.angle)])),
            V3.sum(V3.sub(doodad.position, offset), V3.rotate(V3.mul(v3, doodad.scale), [0, 0, radians(doodad.angle)])),
          ],
          geosetId,
        });
      }
      return faces;
    }));
  });

  console.log('allFaces.length', allFaces.length);
  const mapSize = [terrain.map.width * distancePerTile, terrain.map.height * distancePerTile, maxGameHeightDiff];

  console.log({ mapSize });

  const sumArray = nArray(terrain.groundHeight.length, terrain.groundHeight[0].length, 0);
  const countArray = nArray(terrain.groundHeight.length, terrain.groundHeight[0].length, 0);

  function update(i: number, j: number, newDataHeight: number) {
    countArray[i][j]++;
    if (countArray[i][j] > 1) {
    // max aggregating
      sumArray[i][j] = Math.max(sumArray[i][j], newDataHeight);
      countArray[i][j] = 1;

    /// sum aggregating
    // sumArray[i][j] += newDataHeight;
    } else {
      sumArray[i][j] = newDataHeight;
    }
    const dataHeight = Math.round(sumArray[i][j] / countArray[i][j]);
    terrain.groundHeight[i][j] = Math.max(
      terrain.groundHeight[i][j],
      Math.max(dataHeightMin, Math.min(dataHeightMax, dataHeight)),
    );
  }

  console.log('terrain.map', terrain.map);

  allFaces.forEach((f) => {
    const tileVertices: Vector3[] = f.vertices.map((v) => {
      const percentX = v[0] / mapSize[0];
      const percentY = v[1] / mapSize[1];
      const tileI = percentY * terrain.map.height + 1;
      const tileJ = percentX * terrain.map.width;
      return [tileI, tileJ, v[2]];
    });

    const points = findIntegerPointsInTriangle(
      <Vertex2D><unknown>tileVertices[0],
      <Vertex2D><unknown>tileVertices[1],
      <Vertex2D><unknown>tileVertices[2],
    );
    const slope = calculateTriangleSlope(f.vertices);

    points.forEach(([i, j]) => {
      if (i < 0 || i >= terrain.groundHeight.length || j < 0 || j >= terrain.groundHeight[i].length) {
        return;
      }
      if (slope > slopeThreshold) {
        return;
      }
      const gameZ = Math.min(
        getZProjectionOfXyInTriangle(tileVertices[0], tileVertices[1], tileVertices[2], i, j),
        _.max(tileVertices.map((v) => v[2]))!,
      );
      update(i, j, gameZToDataHeight(gameZ));
    });
  });

  for (let k = floodBrushSize * 2; k >= 1; k--) {
    for (let i = 0; i < terrain.groundHeight.length; i++) {
      for (let j = 0; j < terrain.groundHeight[i].length; j++) {
        if (countArray[i][j] === 0) {
          let sum = 0;
          let cnt = 0;
          for (let i2 = Math.max(0, i - floodBrushSize); i2 <= Math.min(terrain.groundHeight.length - 1, i + floodBrushSize); i2++) {
            for (let j2 = Math.max(0, j - floodBrushSize); j2 < Math.min(terrain.groundHeight[i].length - 1, j + floodBrushSize); j2++) {
              if (countArray[i2][j2] > 0) {
                sum += terrain.groundHeight[i2][j2];
                cnt++;
              }
            }
          }
          if (cnt >= k) {
            const dataHeight = Math.round(sum / cnt);
            terrain.groundHeight[i][j] = Math.max(
              terrain.groundHeight[i][j],
              Math.max(dataHeightMin, Math.min(dataHeightMax, dataHeight)),
            );
          }
        }
      }
    }
  }

  console.log(terrain.map);
  console.log(terrain.groundTexture.length, terrain.groundTexture[0].length);
  console.log(terrain.groundHeight.length, terrain.groundHeight[0].length);
  console.log(terrain.layerHeight.length, terrain.groundHeight[0].length);
  return terrain;
}

// Function to calculate the area of the triangle using vertices
function triangleArea(A: Vertex2D, B: Vertex2D, C: Vertex2D): number {
  return Math.abs((A[0] * (B[1] - C[1]) + B[0] * (C[1] - A[1]) + C[0] * (A[1] - B[1])) / 2.0);
}

// Function to check if a point P is inside the triangle ABC
function isInsideTriangle(A: Vertex2D, B: Vertex2D, C: Vertex2D, P: Vertex2D): boolean {
  const fullArea = triangleArea(A, B, C);
  const area1 = triangleArea(P, B, C);
  const area2 = triangleArea(A, P, C);
  const area3 = triangleArea(A, B, P);

  // Check if the sum of P's area with the sides of the triangle equals the full area
  return Math.abs(fullArea - (area1 + area2 + area3)) < 1;
}

// Function to find integer points inside the triangle
function findIntegerPointsInTriangle(A: Vertex2D, B: Vertex2D, C: Vertex2D): Vertex2D[] {
  const points: Vertex2D[] = [];
  // for (const p of [A, B, C]) {
  // points.push([Math.round(p[0]), Math.round(p[1])]);
  //   points.push([Math.floor(p[0]), Math.floor(p[1])]);
  //   points.push([Math.floor(p[0]), Math.ceil(p[1])]);
  //   points.push([Math.ceil(p[0]), Math.floor(p[1])]);
  //   points.push([Math.ceil(p[0]), Math.ceil(p[1])]);
  // }

  // Determine the bounding box of the triangle
  const minX = Math.min(A[0], B[0], C[0]);
  const maxX = Math.max(A[0], B[0], C[0]);
  const minY = Math.min(A[1], B[1], C[1]);
  const maxY = Math.max(A[1], B[1], C[1]);

  // Iterate over the bounding box and check each point
  for (let x = Math.floor(minX); x <= Math.ceil(maxX); x++) {
    for (let y = Math.floor(minY); y <= Math.ceil(maxY); y++) {
      const point: Vertex2D = [x, y];
      if (isInsideTriangle(A, B, C, point)) {
        points.push(point);
      }
    }
  }

  return points;
}

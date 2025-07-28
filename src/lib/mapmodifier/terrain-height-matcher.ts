import _ from 'lodash';
import { parseMDL } from 'war3-model';

import { Doodad, Terrain } from '@/vendors/wc3maptranslator/data';

import {
  dataHeightMax, dataHeightMin, distancePerTile, gameZToDataHeight, maxGameHeightDiff,
} from '../constants';
import { Vector2, Vector3 } from '../math/common';
import { calculateTriangleSlope, findIntegerPointsInTriangle, getZProjectionOfXyInTriangle } from '../math/geometry';
import { radians } from '../math/rotation';
import { V3 } from '../math/vector';
import { nArray } from '../utils';

const floodBrushSize = 2;
const slopeThreshold = 45;

interface Face {
  vertices: [Vector3, Vector3, Vector3];
  geosetId: number
}

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
      <Vector2><unknown>tileVertices[0],
      <Vector2><unknown>tileVertices[1],
      <Vector2><unknown>tileVertices[2],
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

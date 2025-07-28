import path from 'path';

import {
  dataHeightMax, dataHeightMin, dataHeightToGameZ, distancePerTile, maxGameHeightDiff,
} from '@/lib/constants';
import { ModificationType, Terrain } from '@/vendors/wc3maptranslator/data';
import { IDoodadType, MapManager } from '@/vendors/wc3maptranslator/extra/map-manager';

import { getInitialTerrain } from '../../mapmodifier/terrain';
import { Vector3 } from '../../math/common';
import { calculateChildAbsoluteEulerRotation, degrees } from '../../math/rotation';
import { V3 } from '../../math/vector';
import { computeAbsoluteMinMaxExtents } from '../common/model-manager';
import { WowObject } from '../common/models';
import { MapExportConfig } from './map-converter';

enum TerrainFlag {
  Unwalkable = 2,
  Unflyable = 4,
  Unbuildable = 8,
  Ramp = 16,
  Blight = 32,
  Water = 64,
  Boundary = 128,
}

export const baseDoodadType = 'YOlb'; // Lightning Bolt

export class Wc3Converter {
  constructor(private config: MapExportConfig) {
  }

  generateTerrainWithHeight(terrainObjs: WowObject[]): Terrain {
    console.log('Generating terrain from', terrainObjs.length, ' files');

    const { heightMap, height, width } = this.computeTerrainHeightMap(terrainObjs);

    console.log('Map size', { height, width });

    if (width > 480 || height > 480) {
      throw new Error('Map size is too large!');
    }

    // Ground height
    const terrain = getInitialTerrain(height, width);
    for (let i = 0; i < heightMap.length; i++) {
      for (let j = 0; j < heightMap[i].length; j++) {
        if (heightMap[i][j] === -1) {
          continue;
        } else {
          terrain.groundHeight[i][j] = Math.ceil(heightMap[i][j] * (dataHeightMax - dataHeightMin) + dataHeightMin);
        }
      }
    }

    // Water height
    // const waterHeightThreshold = waterZToDataHeight(this.config.waterZThreshold);
    // for (let i = 0; i < heightMap.length; i++) {
    //   for (let j = 0; j < heightMap[i].length; j++) {
    //     if (terrain.groundHeight[i][j] < waterHeightThreshold) {
    //       terrain.flags[i][j] |= TerrainFlag.Water;
    //       terrain.waterHeight[i][j] = waterHeightThreshold;
    //       terrain.groundHeight[i][j] = Math.min(terrain.groundHeight[i][j], waterHeightThreshold - 1);
    //       // Make sure we don't overflow the limit
    //       terrain.groundHeight[i][j] = Math.max(dataHeightMin, Math.min(dataHeightMax, terrain.groundHeight[i][j]));
    //     }
    //   }
    // }

    for (let i = 0; i < heightMap.length; i++) {
      for (let j = 0; j < heightMap[i].length; j++) {
        if (terrain.groundHeight[i][j] >= dataHeightMax) {
          terrain.flags[i][j] |= (TerrainFlag.Unflyable | TerrainFlag.Unwalkable | TerrainFlag.Unbuildable);
        }
      }
    }
    return terrain;
  }

  placeDoodads(map: MapManager, roots: WowObject[], filter: (obj: WowObject) => boolean) {
    const terrain = map.terrain;
    console.log('Placing doodads');

    const { min, max } = computeAbsoluteMinMaxExtents(roots);
    const mapMin: Vector3 = [
      terrain.map.offset.x,
      terrain.map.offset.y,
      dataHeightToGameZ(dataHeightMin),
    ];
    const mapMax: Vector3 = [
      terrain.map.offset.x + terrain.map.width * distancePerTile,
      terrain.map.offset.y + terrain.map.height * distancePerTile,
      dataHeightToGameZ(dataHeightMax),
    ];
    const mapSize = V3.sub(mapMax, mapMin);
    const modelSize = V3.sub(max, min);

    // console.log('Map', { mapMin, mapMax, mapSize });
    // console.log('Parent model', { min, max, size: modelSize });
    // roots.forEach(r => console.log(r.id, r.position))

    const terrainClampPercentDiff = this.config.terrain.clampPercent.upper - this.config.terrain.clampPercent.lower;

    const rootScale = [
      mapSize[0] / modelSize[0],
      mapSize[1] / modelSize[1],
      mapSize[2] / (modelSize[2] * terrainClampPercentDiff),
    ];
    console.log({ rootScale });

    const modelPathToDoodadType = new Map<string, IDoodadType>();

    let doodadTypesWithPitchRoll = 0;
    const placeDoodadsRecursive = (obj: WowObject, parentAbsolute: WowObject | null) => {
      // console.log('================================');
      const objAbsolute = { ...obj };
      if (parentAbsolute) {
        const relativePos = V3.rotate(obj.position, parentAbsolute.rotation);
        objAbsolute.position = V3.sum(parentAbsolute.position, relativePos);
        objAbsolute.rotation = calculateChildAbsoluteEulerRotation(parentAbsolute.rotation, objAbsolute.rotation);
        objAbsolute.scaleFactor *= parentAbsolute.scaleFactor;
        // console.log('Translating', obj.id, 'based on parent', parentAbsolute.id);
        // console.log({ childOldPos: obj.position, parentAbsRot: parentAbsolute.rotation });
        // console.log({ relativePos, parentAbsPos: parentAbsolute.position, childAbsPos: objAbsolute.position });
      }

      if (filter(obj)) {
        // WC3 pitch and roll must be negative, required by World Editor
        const wc3Roll = ((-objAbsolute.rotation[0]) % (Math.PI * 2) - Math.PI * 2) % (Math.PI * 2);
        const wc3Pitch = ((-objAbsolute.rotation[1]) % (Math.PI * 2) - Math.PI * 2) % (Math.PI * 2);

        const hasRollPitch = Math.abs(wc3Roll) > this.config.doodads.pitchRollThresholdRadians
          && Math.abs(wc3Pitch) > this.config.doodads.pitchRollThresholdRadians;

        const fileName = obj.model!.relativePath;
        const hashKey = hasRollPitch
          ? [fileName, objAbsolute.rotation[0].toFixed(2), objAbsolute.rotation[1].toFixed(2)].join(';')
          : fileName;

        // Insert new doodad type if not exists
        if (!modelPathToDoodadType.has(hashKey)) {
          const doodadType = map.addDoodadType(baseDoodadType, [
            {
              id: 'dfil', type: ModificationType.string, level: 0, column: 0, value: fileName,
            },
          ]);
          // Prefix generated doodads with ~ so that they are shown last in Object Editor.
          const doodadName = `~D ${path.basename(obj.model!.relativePath)} -- ${obj.type} -- ${doodadType.code}`;

          doodadType.data.push(
            { // model file
              id: 'dfil', type: ModificationType.string, level: 0, column: 0, value: fileName,
            },
            { // doodad name
              id: 'dnam', type: ModificationType.string, level: 0, column: 0, value: doodadName,
            },
            { // scale max
              id: 'dmas', type: ModificationType.unreal, value: objAbsolute.scaleFactor * Math.max(...rootScale) * 1.5,
            },
            { // scale min
              id: 'dmis', type: ModificationType.unreal, value: objAbsolute.scaleFactor * Math.min(...rootScale) / 1.5,
            },
            {
              id: 'danf', type: ModificationType.int, level: 0, column: 0, value: 1,
            },
            {
              id: 'dshf', type: ModificationType.int, level: 0, column: 0, value: 1,
            },
          );
          if (hasRollPitch) {
            doodadType.data.push(
              { // roll, must be unreal even though in Editor it's always negative
                id: 'dmar', type: ModificationType.unreal, level: 0, column: 0, value: wc3Roll,
              },
              { // pitch, must be unreal even though in Editor it's always negative
                id: 'dmap', type: ModificationType.unreal, level: 0, column: 0, value: wc3Pitch,
              },
            );
            doodadTypesWithPitchRoll++;
          }
          modelPathToDoodadType.set(hashKey, doodadType);
        }
        const id4Chars = modelPathToDoodadType.get(hashKey)!.code.slice(0, 4);

        // Calculate positions
        const percent = [
          (objAbsolute.position[0] - min[0]) / modelSize[0],
          (objAbsolute.position[1] - min[1]) / modelSize[1],
          (objAbsolute.position[2] - min[2]) / modelSize[2],
        ];
        const inGameX = mapMin[0] + percent[0] * mapSize[0];
        const inGameY = mapMin[1] + percent[1] * mapSize[1];

        const zDiff = (dataHeightMax - dataHeightMin)
          * (percent[2] - this.config.terrain.clampPercent.lower) / terrainClampPercentDiff;
        const inGameZ = dataHeightToGameZ(dataHeightMin + zDiff);

        let outOfBound = false;
        if (inGameX < mapMin[0] || inGameX > mapMax[0] || inGameY < mapMin[1] || inGameY > mapMax[1]) {
          outOfBound = true;
          // console.warn('Placing', objAbsolute.model?.relativePath, 'outside of map bounds.');
          // console.log(objAbsolute.id, objAbsolute.position, {
          //   percent, inGameX, inGameY, inGameZ,
          // }, { mapMin, mapMax });
        }

        // Add doodad instance
        !outOfBound && map.addDoodad(modelPathToDoodadType.get(hashKey)!, {
          id: 0,
          variation: 0,
          position: [inGameX, inGameY, inGameZ],
          angle: degrees(objAbsolute.rotation[2]),
          scale: [
            objAbsolute.scaleFactor * rootScale[0] + 0.0001,
            objAbsolute.scaleFactor * rootScale[1] + 0.0001,
            objAbsolute.scaleFactor * rootScale[2] + 0.0001,
          ],
          skinId: id4Chars,
          flags: {
            visible: true,
            solid: true,
            customHeight: true,
          },
          life: 100,
          randomItemSetPtr: -1,
          droppedItemSets: [],
        });
      }

      objAbsolute.children.forEach((child) => placeDoodadsRecursive(child, objAbsolute));
    };

    roots.forEach((p) => placeDoodadsRecursive(p, null));
    return { doodadTypesWithPitchRoll };
  }

  private computeTerrainHeightMap(terrains: WowObject[]) {
    if (terrains.length === 0) {
      const heightMap = Array.from({ length: 64 + 1 }, () => Array<number>(64 + 1).fill(-1));
      return { heightMap, height: 64, width: 64 };
    }

    console.log('Computing terrain height map...');
    const { min, max } = computeAbsoluteMinMaxExtents(terrains);
    console.log({ min, max });

    const terrainSize = V3.sub(max, min);
    console.log({ terrainSize });
    console.log(
      'Recommended terrain clamp percent difference',
      (dataHeightToGameZ(dataHeightMax) - dataHeightToGameZ(dataHeightMin)) / terrainSize[2],
    );

    const terrainClampPercentDiff = this.config.terrain.clampPercent.upper - this.config.terrain.clampPercent.lower;

    const ratioZ = maxGameHeightDiff / (terrainSize[2] * terrainClampPercentDiff);
    const ratioXY = ratioZ;
    const width = Math.ceil(terrainSize[0] / distancePerTile * ratioXY / 4) * 4;
    const height = Math.ceil(terrainSize[1] / distancePerTile * ratioXY / 4) * 4;

    console.log({ ratio: ratioZ, height, width });

    const heightMap = Array.from({ length: height + 1 }, () => Array<number>(width + 1).fill(-1));
    terrains.forEach((terrain) => {
      terrain.model!.mdl.geosets
        .forEach((geoset) => geoset.vertices.forEach((v) => {
          const rotatedV = V3.rotate(v.position, terrain.rotation);
          const position = V3.sum(terrain.position, rotatedV);

          // console.log({ position, min, max });

          const percent = [
            (position[0] - min[0]) / terrainSize[0],
            (position[1] - min[1]) / terrainSize[1],
            (position[2] - (min[2] + terrainSize[2] * this.config.terrain.clampPercent.lower))
              / (terrainSize[2] * terrainClampPercentDiff),
          ];

          if (percent[0] < 0 || percent[0] > 1 || percent[1] < 0 || percent[1] > 1) {
            console.error('Out of bounds', { percent, position });
            throw new Error('Out of bounds');
          }
          const iX = Math.round(percent[0] * width);
          const iY = Math.round(percent[1] * height);
          // [Y is height][X is width]
          heightMap[iY][iX] = Math.max(heightMap[iY][iX], Math.max(0, Math.min(1, percent[2])));
        }));
    });

    // Fill the remaining -1 cells using its neighbors
    const floodBrushSize = 3;
    for (let k = (floodBrushSize * 2 + 1) ** 2; k >= 1; k--) {
      for (let i = 0; i < heightMap.length; i++) {
        for (let j = 0; j < heightMap[i].length; j++) {
          if (heightMap[i][j] === -1) {
            let sum = 0;
            let cnt = 0;
            for (let i2 = Math.max(0, i - floodBrushSize); i2 <= Math.min(heightMap.length - 1, i + floodBrushSize); i2++) {
              for (let j2 = Math.max(0, j - floodBrushSize); j2 < Math.min(heightMap[i].length - 1, j + floodBrushSize); j2++) {
                if (heightMap[i2][j2] > 0) {
                  sum += heightMap[i2][j2];
                  cnt++;
                }
              }
            }
            if (cnt >= k) {
              const dataHeight = sum / cnt;
              heightMap[i][j] = dataHeight;
            }
          }
        }
      }
    }

    return {
      heightMap, height, width,
    };
  }
}

export function getTerrainHeight(terrain: Terrain, percentX: number, percentY: number) {
  // Clamp percent values to [0, 1]
  const u = Math.min(1, Math.max(0, percentX));
  const v = Math.min(1, Math.max(0, percentY));

  const gridHeight = terrain.groundHeight.length - 1; // Y dimension
  const gridWidth = terrain.groundHeight[0].length - 1; // X dimension

  const x = u * gridWidth;
  const y = v * gridHeight;

  const x0 = Math.floor(x);
  const x1 = Math.min(gridWidth, Math.ceil(x));
  const y0 = Math.floor(y);
  const y1 = Math.min(gridHeight, Math.ceil(y));

  const wx1 = x - x0;
  const wx0 = 1 - wx1;
  const wy1 = y - y0;
  const wy0 = 1 - wy1;

  const h00 = terrain.groundHeight[y0][x0];
  const h10 = terrain.groundHeight[y0][x1];
  const h01 = terrain.groundHeight[y1][x0];
  const h11 = terrain.groundHeight[y1][x1];

  // Bilinear interpolation
  const height = h00 * wx0 * wy0 + h10 * wx1 * wy0 + h01 * wx0 * wy1 + h11 * wx1 * wy1;

  return height;
}

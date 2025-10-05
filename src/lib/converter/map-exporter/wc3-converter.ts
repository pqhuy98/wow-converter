import path from 'path';

import {
  dataHeightMax, dataHeightMin, dataHeightToGameZ, distancePerTile, maxGameHeightDiff,
} from '@/lib/constants';
import { ModificationType, Terrain } from '@/vendors/wc3maptranslator/data';
import { IDoodadType, IUnitType, MapManager } from '@/vendors/wc3maptranslator/extra/map-manager';

import { getInitialTerrain } from '../../mapmodifier/terrain';
import { Vector3 } from '../../math/common';
import { degrees } from '../../math/rotation';
import { V3 } from '../../math/vector';
import { computeAbsoluteMinMaxExtents } from '../common/asset-manager';
import { isWowUnit, WowObject, WowUnit } from '../common/models';
import { WowObjectManager } from '../common/wow-object-manager';
import { MapExportConfig } from './map-exporter';

enum TerrainFlag {
  Unwalkable = 2,
  Unflyable = 4,
  Unbuildable = 8,
  Ramp = 16,
  Blight = 32,
  Water = 64,
  Boundary = 128,
}

export class Wc3Converter {
  constructor(private config: MapExportConfig) {
  }

  generateTerrainWithHeight(wowObjectManager: WowObjectManager): Terrain {
    const roots = wowObjectManager.roots;
    console.log('Generating terrain from', roots.length, 'objects');

    const { heightMap, height, width } = computeTerrainHeightMap(roots, this.config);

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

  placeDoodads(map: MapManager, wowObjectManager: WowObjectManager, filter: (obj: WowObject) => boolean) {
    const terrain = map.terrain;
    console.log('Placing doodads');

    const roots = wowObjectManager.roots;

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
    wowObjectManager.iterateObjects((obj, objAbsolute) => {
      // console.log('================================');
      if (filter(obj)) {
        // WC3 pitch and roll must be negative, required by World Editor
        const wc3Roll = ((-objAbsolute.rotation[0]) % (Math.PI * 2) - Math.PI * 2) % (Math.PI * 2);
        const wc3Pitch = ((-objAbsolute.rotation[1]) % (Math.PI * 2) - Math.PI * 2) % (Math.PI * 2);

        const hasRollPitch = Math.abs(wc3Roll) > this.config.doodads.pitchRollThresholdRadians
          && Math.abs(wc3Pitch) > this.config.doodads.pitchRollThresholdRadians;

        if (!obj.model) {
          console.error('Doodad has no model', obj);
          throw new Error('Doodad has no model');
        }
        const fileName = obj.model.relativePath;
        const hashKey = hasRollPitch
          ? [fileName, objAbsolute.rotation[0].toFixed(2), objAbsolute.rotation[1].toFixed(2)].join(';')
          : fileName;

        // Insert new doodad type if not exists
        if (!modelPathToDoodadType.has(hashKey)) {
          const doodadType = map.addDoodadType([
            {
              id: 'dfil', type: ModificationType.string, level: 0, column: 0, value: fileName,
            },
          ], false);
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
    });

    return { doodadTypesWithPitchRoll };
  }

  placeUnits(mapManager: MapManager, wowObjectManager: WowObjectManager) {
    const debug = false;
    const mapConfig = this.config;
    const terrain = mapManager.terrain;
    const roots = wowObjectManager.roots;

    const units: WowUnit[] = [];

    // Global map params
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

    const { min, max } = computeAbsoluteMinMaxExtents(roots);
    const modelSize = V3.sub(max, min);
    const scale = mapSize[0] / modelSize[0];

    const templateIdToUnitType = new Map<number, IUnitType>();
    const templateIdToDoodadType = new Map<number, IDoodadType>();

    // Iterate each root to position its creatures
    wowObjectManager.iterateObjects((obj, objAbsolute) => {
      if (!isWowUnit(obj)) return;
      units.push(obj);
      const c = obj.creature;

      const absPosition = objAbsolute.position;

      if (absPosition[0] < min[0] - 1 || absPosition[0] > max[0] + 1
          || absPosition[1] < min[1] - 1 || absPosition[1] > max[1] + 1) {
        console.error('Creature', c.template.name, 'is out of bounds', absPosition);
        console.log({ min, max });
        return;
      }

      const percent = [
        (absPosition[0] - min[0]) / modelSize[0],
        (absPosition[1] - min[1]) / modelSize[1],
        (absPosition[2] - min[2]) / modelSize[2],
      ];

      const inGameX = mapMin[0] + percent[0] * mapSize[0];
      const inGameY = mapMin[1] + percent[1] * mapSize[1];

      const inGameZ = dataHeightToGameZ(dataHeightMin
          + (dataHeightMax - dataHeightMin)
          / (mapConfig.terrain.clampPercent.upper - mapConfig.terrain.clampPercent.lower)
          * (percent[2] - mapConfig.terrain.clampPercent.lower));

      const terrainZ = dataHeightToGameZ(getTerrainHeight(terrain, percent[0], percent[1]));

      const creatureModel = `creature-${c.model.CreatureDisplayID}.mdx`;
      const creatureName = c.template.name || c.template.subname;
      const creatureScale = scale * c.model.DisplayScale * mapConfig.creatures.scaleUp;
      const creatureFacingRadians = objAbsolute.rotation[2];
      const position: Vector3 = [inGameX, inGameY, inGameZ];

      const withinPlayableZone = percent[2] >= mapConfig.terrain.clampPercent.lower
          && percent[2] <= mapConfig.terrain.clampPercent.upper;
      const notOnGround = inGameZ < terrainZ - 100 || inGameZ > terrainZ + 100;

      if (mapConfig.creatures.allAreDoodads || !withinPlayableZone || notOnGround) {
        // Creature is out of playable map zone or not on ground, add it as doodad
        debug && console.log('Add', c.template.name, 'as destructible because of', mapConfig.creatures.allAreDoodads ? 'overridden' : 'outside of allowed zone');

        if (!templateIdToDoodadType.has(c.template.entry)) {
          templateIdToDoodadType.set(c.template.entry, mapManager.addDoodadType([
            { id: 'bnam', type: ModificationType.string, value: `~U ${creatureName}` },
            { id: 'bfil', type: ModificationType.string, value: creatureModel },
            { id: 'bmas', type: ModificationType.unreal, value: creatureScale * 1.5 },
            { id: 'bmis', type: ModificationType.unreal, value: creatureScale / 1.5 },
          ], true));
        }
        const doodadType = templateIdToDoodadType.get(c.template.entry)!;

        mapManager.addDoodad(doodadType, {
          id: 0,
          variation: 0,
          position,
          angle: degrees(creatureFacingRadians),
          scale: [creatureScale, creatureScale, creatureScale],
          skinId: doodadType.code,
          flags: {
            visible: true,
            solid: true,
            customHeight: true,
          },
          life: 100,
          randomItemSetPtr: -1,
          droppedItemSets: [],
        });
      } else {
        // Creature is inside playable map zone, add it as unit

        if (!templateIdToUnitType.has(c.template.entry)) {
          templateIdToUnitType.set(c.template.entry, mapManager.addUnitType('unit', 'hfoo', [
            { id: 'unam', type: ModificationType.string, value: creatureName },
            // { id: 'upro', type: ModificationType.string, value: c.template.name || c.template.subname },
            { id: 'unsf', type: ModificationType.string, value: `guid=${c.creature.guid} template.entry=${c.template.entry} displayId=${c.model.CreatureDisplayID} phaseMask=${c.creature.phaseMask}` },
            { id: 'umdl', type: ModificationType.string, value: creatureModel },
            { id: 'uabi', type: ModificationType.string, value: '' },
            { id: 'usca', type: ModificationType.real, value: creatureScale },
            { id: 'uhpm', type: ModificationType.int, value: c.creature.curhealth },
            { id: 'umpm', type: ModificationType.int, value: c.creature.curmana },
            { id: 'umpi', type: ModificationType.int, value: c.creature.curmana },
            { id: 'ulev', type: ModificationType.int, value: c.template.maxlevel },
          ]));
        }
        const unitType = templateIdToUnitType.get(c.template.entry)!;

        mapManager.addUnit(unitType, {
          variation: 0,
          position,
          rotation: creatureFacingRadians,
          scale: [1, 1, 1],
          skin: unitType.code,
          player: 0,
          hitpoints: 100,
          mana: 0,
          randomItemSetPtr: -1,
          droppedItemSets: [],
          gold: 0,
          targetAcquisition: -1,
          hero: {
            level: c.template.maxlevel ?? 1, str: 0, agi: 0, int: 0,
          },
          inventory: [],
          abilities: [],
          random: {
            type: 0, level: 0, itemClass: 0, groupIndex: 0, columnIndex: 0, unitSet: [],
          },
          color: 23,
          waygate: -1,
          id: 0,
        });
      }
    });

    return units;
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

function computeTerrainHeightMap(roots: WowObject[], config: MapExportConfig) {
  if (roots.length === 0) {
    const heightMap = Array.from({ length: 64 + 1 }, () => Array<number>(64 + 1).fill(-1));
    return { heightMap, height: 64, width: 64 };
  }

  console.log('Computing terrain height map...');
  const { min, max } = computeAbsoluteMinMaxExtents(roots);
  console.log({ min, max });

  const terrainSize = V3.sub(max, min);
  console.log({ terrainSize });
  console.log(
    'Recommended terrain clamp percent difference',
    (dataHeightToGameZ(dataHeightMax) - dataHeightToGameZ(dataHeightMin)) / terrainSize[2],
  );

  const terrainClampPercentDiff = config.terrain.clampPercent.upper - config.terrain.clampPercent.lower;

  const ratioZ = maxGameHeightDiff / (terrainSize[2] * terrainClampPercentDiff);
  const ratioXY = ratioZ;
  const width = Math.ceil(terrainSize[0] / distancePerTile * ratioXY / 4) * 4;
  const height = Math.ceil(terrainSize[1] / distancePerTile * ratioXY / 4) * 4;

  console.log({ ratio: ratioZ, height, width });

  const heightMap = Array.from({ length: height + 1 }, () => Array<number>(width + 1).fill(-1));
  roots.forEach((root) => {
    root.model!.mdl.geosets
      .forEach((geoset) => geoset.vertices.forEach((v) => {
        const rotatedV = V3.rotate(v.position, root.rotation);
        const position = V3.sum(root.position, rotatedV);

        // console.log({ position, min, max });

        const percent = [
          (position[0] - min[0]) / terrainSize[0],
          (position[1] - min[1]) / terrainSize[1],
          (position[2] - (min[2] + terrainSize[2] * config.terrain.clampPercent.lower))
            / (terrainSize[2] * terrainClampPercentDiff),
        ];

        if (percent[0] < 0 || percent[0] > 1 || percent[1] < 0 || percent[1] > 1) {
          console.error('Out of bounds', { percent, position });
          throw new Error('Out of bounds');
        }
        const iX = (Math.random() > 0.5 ? Math.round : Math.floor)(percent[0] * width);
        const iY = (Math.random() > 0.5 ? Math.round : Math.floor)(percent[1] * height);
        // [Y is height][X is width]
        heightMap[iY][iX] = Math.max(heightMap[iY][iX], Math.max(0, Math.min(1, percent[2])));
      }));
  });

  // Fill the remaining -1 cells using its neighbors
  const floodBrushSize = 5;
  for (let k = (floodBrushSize * 2 + 1) ** 2; k >= 1; k--) {
    for (let i = 0; i < heightMap.length; i++) {
      for (let j = 0; j < heightMap[i].length; j++) {
        if (heightMap[i][j] === -1) {
          let sum = 0;
          let cnt = 0;
          for (let i2 = Math.max(0, i - floodBrushSize); i2 <= Math.min(heightMap.length - 1, i + floodBrushSize); i2++) {
            for (let j2 = Math.max(0, j - floodBrushSize); j2 < Math.min(heightMap[i].length - 1, j + floodBrushSize); j2++) {
              if (heightMap[i2][j2] >= 0) {
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

export function computeRecommendedTerrainClampPercent(roots: WowObject[]) {
  const { min, max } = computeAbsoluteMinMaxExtents(roots);
  const terrainSize = V3.sub(max, min);
  return {
    ratio: (dataHeightToGameZ(dataHeightMax) - dataHeightToGameZ(dataHeightMin)) / terrainSize[2],
    min,
    max,
  };
}

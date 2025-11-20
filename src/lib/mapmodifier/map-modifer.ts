import assert from 'assert';

import { Vector2 } from '@/lib/math/common';
import {
  Camera,
  DoodadList,
  ObjectModificationTable,
  Region,
  Terrain,
} from '@/vendors/wc3maptranslator/data';
import { MapManager } from '@/vendors/wc3maptranslator/extra/map-manager';
import { FourCCGenerator } from '@/vendors/wc3maptranslator/extra/war3-fourcc';

import { distancePerTile } from '../constants';
import { getInitialTerrain } from './terrain';

export interface Wc3Map {
  terrain: Terrain;
  doodads: DoodadList;
  doodadsData: ObjectModificationTable
}

export function mergeMapsLeftToRight(maps: [Wc3Map, Wc3Map], padding: number) {
  for (const map of maps) {
    assert.deepStrictEqual(map.terrain.tileset, maps[0].terrain.tileset);
    assert.deepStrictEqual(map.terrain.customTileset, maps[0].terrain.customTileset);
    assert.deepStrictEqual(map.terrain.tilePalette, maps[0].terrain.tilePalette);
    assert.deepStrictEqual(map.terrain.cliffTilePalette, maps[0].terrain.cliffTilePalette);
  }

  const newHeight = Math.max(maps[0].terrain.map.height, maps[1].terrain.map.height);
  const newWidth = Math.ceil((maps[0].terrain.map.width + maps[1].terrain.map.width + padding) / 4) * 4;
  const terrain = getInitialTerrain(newHeight, newWidth);

  const xyDelta = [
    {
      x: -maps[0].terrain.map.offset.x + terrain.map.offset.x,
      y: -maps[0].terrain.map.offset.y + terrain.map.offset.y,
    },
    {
      x: -maps[1].terrain.map.offset.x + (maps[0].terrain.map.width + 1 + padding) * distancePerTile + terrain.map.offset.x,
      y: -maps[1].terrain.map.offset.y + terrain.map.offset.y,
    },
  ];
  const tileOffset = [
    { i: 0, j: 0 },
    { i: 0, j: maps[0].terrain.map.width + 1 + padding },
  ];
  const newMap: Wc3Map = {
    terrain,
    doodads: [[], []],
    doodadsData: {
      original: {},
      custom: {},
    },
  };
  maps.forEach((map, mapIdx) => {
    // merge terrain
    for (let i = 0; i < map.terrain.groundHeight.length; i++) {
      for (let j = 0; j < map.terrain.groundHeight[i].length; j++) {
        const newI = i + tileOffset[mapIdx].i;
        const newJ = j + tileOffset[mapIdx].j;
        newMap.terrain.groundHeight[newI][newJ] = map.terrain.groundHeight[i][j];
        newMap.terrain.waterHeight[newI][newJ] = map.terrain.waterHeight[i][j];
        newMap.terrain.boundaryFlag[newI][newJ] = map.terrain.boundaryFlag[i][j];
        newMap.terrain.flags[newI][newJ] = map.terrain.flags[i][j];
        newMap.terrain.groundTexture[newI][newJ] = map.terrain.groundTexture[i][j];
        newMap.terrain.groundVariation[newI][newJ] = map.terrain.groundVariation[i][j];
        newMap.terrain.cliffVariation[newI][newJ] = map.terrain.cliffVariation[i][j];
        newMap.terrain.cliffTexture[newI][newJ] = map.terrain.cliffTexture[i][j];
        newMap.terrain.layerHeight[newI][newJ] = map.terrain.layerHeight[i][j];
      }
    }
    // merge doodads
    map.doodads.forEach((doodads, idx) => {
      doodads.forEach((doodad) => {
        newMap.doodads[idx].push({
          ...doodad,
          position: [
            doodad.position[0] + xyDelta[mapIdx].x,
            doodad.position[1] + xyDelta[mapIdx].y,
            doodad.position[2],
          ],
        });
      });
    });
    // merge doodad types
    newMap.doodadsData.original = {
      ...newMap.doodadsData.original,
      ...map.doodadsData.original,
    };
    newMap.doodadsData.custom = {
      ...newMap.doodadsData.custom,
      ...map.doodadsData.custom,
    };
  });
  return newMap;
}

export function pasteMapIntoMap(source: MapManager, target: MapManager, x: number, y: number): void {
  // Validate tileset compatibility to prevent mixing incompatible terrain palettes
  assert.deepStrictEqual(source.terrain.tileset, target.terrain.tileset);
  assert.deepStrictEqual(source.terrain.customTileset, target.terrain.customTileset);
  assert.deepStrictEqual(source.terrain.tilePalette, target.terrain.tilePalette);
  assert.deepStrictEqual(source.terrain.cliffTilePalette, target.terrain.cliffTilePalette);

  // Validate placement bounds
  const sourceHeight = source.terrain.groundHeight.length;
  const sourceWidth = sourceHeight > 0 ? source.terrain.groundHeight[0].length : 0;
  const targetHeight = target.terrain.groundHeight.length;
  const targetWidth = targetHeight > 0 ? target.terrain.groundHeight[0].length : 0;

  console.log({
    sourceHeight,
    sourceWidth,
    targetHeight,
    targetWidth,
    x,
    y,
    fitX: x + sourceWidth <= targetWidth,
    fitY: y + sourceHeight <= targetHeight,
  });

  assert(x >= 0 && y >= 0, 'placeMapIntoMap: x and y must be non-negative');
  assert(
    y + sourceHeight <= targetHeight && x + sourceWidth <= targetWidth,
    'placeMapIntoMap: source map does not fit inside target at the given coordinates',
  );

  // Helper to get the string code from a doodad's type field
  const getDoodadCode = (dType: MapManager['doodads'][number]['type']): string => (typeof dType === 'string' ? dType : dType.code);

  // Prepare doodad type collision handling at MapManager level
  const targetUsedTypeCodes = new Set<string>();
  target.doodadTypes.forEach((t) => targetUsedTypeCodes.add(t.code));
  target.destructibleTypes.forEach((t) => targetUsedTypeCodes.add(t.code));
  target.doodads.forEach((d) => targetUsedTypeCodes.add(getDoodadCode(d.type)));

  const sourceCustomTypeCodes = new Set<string>();
  source.doodadTypes.forEach((t) => sourceCustomTypeCodes.add(t.code));
  source.destructibleTypes.forEach((t) => sourceCustomTypeCodes.add(t.code));

  const sourceUsedTypeCodes = new Set<string>();
  source.doodads.forEach((d) => sourceUsedTypeCodes.add(getDoodadCode(d.type)));

  const codesNeedingRemap: string[] = [];
  sourceCustomTypeCodes.forEach((code) => {
    if (sourceUsedTypeCodes.has(code) && targetUsedTypeCodes.has(code)) {
      codesNeedingRemap.push(code);
    }
  });

  if (codesNeedingRemap.length > 0) {
    const generator = new FourCCGenerator(targetUsedTypeCodes);
    const remap = new Map<string, string>();
    for (const oldCode of codesNeedingRemap) {
      const { codeString } = generator.generate('any');
      remap.set(oldCode, codeString);
      targetUsedTypeCodes.add(codeString);
    }
    // Remap type definitions in source managers
    source.doodadTypes.forEach((t) => {
      const newCode = remap.get(t.code);
      if (newCode) t.code = newCode;
    });
    source.destructibleTypes.forEach((t) => {
      const newCode = remap.get(t.code);
      if (newCode) t.code = newCode;
    });
    // Remap doodad instances in source (update string types; object types already updated via t.code)
    source.doodads.forEach((d) => {
      if (typeof d.type === 'string') {
        const newCode = remap.get(d.type);
        if (newCode) {
          d.type = newCode;
          d.skinId = newCode;
        }
      }
    });
  }

  // Copy terrain tiles from source into target at (x, y)
  for (let i = 0; i < sourceHeight; i++) {
    for (let j = 0; j < sourceWidth; j++) {
      const newI = y + i;
      const newJ = x + j;
      target.terrain.groundHeight[newI][newJ] = source.terrain.groundHeight[i][j];
      target.terrain.waterHeight[newI][newJ] = source.terrain.waterHeight[i][j];
      target.terrain.boundaryFlag[newI][newJ] = source.terrain.boundaryFlag[i][j];
      target.terrain.flags[newI][newJ] = source.terrain.flags[i][j];
      target.terrain.groundTexture[newI][newJ] = source.terrain.groundTexture[i][j];
      target.terrain.groundVariation[newI][newJ] = source.terrain.groundVariation[i][j];
      target.terrain.cliffVariation[newI][newJ] = source.terrain.cliffVariation[i][j];
      target.terrain.cliffTexture[newI][newJ] = source.terrain.cliffTexture[i][j];
      target.terrain.layerHeight[newI][newJ] = source.terrain.layerHeight[i][j];
    }
  }

  // Compute world-space delta so that source top-left tile (0,0) aligns to target (x,y)
  const worldDeltaX = (target.terrain.map.offset.x + x * distancePerTile) - source.terrain.map.offset.x;
  const worldDeltaY = (target.terrain.map.offset.y + y * distancePerTile) - source.terrain.map.offset.y;

  // Ensure all source types exist in target after remapping (dedupe by code)
  const existingDoodadCodes = new Set(target.doodadTypes.map((t) => t.code));
  source.doodadTypes.forEach((t) => {
    if (!existingDoodadCodes.has(t.code)) {
      target.doodadTypes.push({ ...t });
      existingDoodadCodes.add(t.code);
    }
  });
  const existingDestructibleCodes = new Set(target.destructibleTypes.map((t) => t.code));
  source.destructibleTypes.forEach((t) => {
    if (!existingDestructibleCodes.has(t.code)) {
      target.destructibleTypes.push({ ...t });
      existingDestructibleCodes.add(t.code);
    }
  });

  // Merge doodads with adjusted positions into target (store type as string code for stability)
  source.doodads.forEach((doodad) => {
    const code = typeof doodad.type === 'string' ? doodad.type : doodad.type.code;
    target.doodads.push({
      ...doodad,
      type: code,
      skinId: code,
      position: [
        doodad.position[0] + worldDeltaX,
        doodad.position[1] + worldDeltaY,
        doodad.position[2],
      ],
    });
  });

  // Merge regions (now part of MapManager)
  const sourceRegions = source.regions;
  const targetRegions = target.regions;
  if (sourceRegions && targetRegions) {
    let maxId = 0;
    for (const r of targetRegions) maxId = Math.max(maxId, r.id);
    let nextId = maxId + 1;
    for (const r of sourceRegions) {
      const moved: Region = {
        ...r,
        id: nextId++,
        position: {
          left: r.position.left + worldDeltaX,
          bottom: r.position.bottom + worldDeltaY,
          right: r.position.right + worldDeltaX,
          top: r.position.top + worldDeltaY,
        },
      };
      targetRegions.push(moved);
    }
  }

  // Merge cameras
  if (source.cameras && target.cameras) {
    source.cameras.forEach((cam) => {
      const moved: Camera = {
        ...cam,
        target: {
          x: cam.target.x + worldDeltaX,
          y: cam.target.y + worldDeltaY,
        },
      };
      target.cameras.push(moved);
    });
  }
}

export function relocateMapSegment(map: MapManager, [x0, y0]: Vector2, [x1, y1]: Vector2, target: Vector2): void {
  // Normalize source rectangle
  const srcMinX = Math.min(x0, x1);
  const srcMaxX = Math.max(x0, x1);
  const srcMinY = Math.min(y0, y1);
  const srcMaxY = Math.max(y0, y1);

  const terrain = map.terrain;
  const mapHeight = terrain.groundHeight.length;
  const mapWidth = terrain.groundHeight[0].length;

  assert(srcMinX >= 0 && srcMinY >= 0 && srcMaxX < mapWidth && srcMaxY < mapHeight, 'relocateMapSegment: source rectangle out of bounds');

  const rectWidth = srcMaxX - srcMinX + 1;
  const rectHeight = srcMaxY - srcMinY + 1;

  const [dstX, dstY] = target;
  console.log({
    dstX,
    dstY,
    rectWidth,
    rectHeight,
    mapWidth,
    mapHeight,
    newX: dstX + rectWidth,
    newY: dstY + rectHeight,
    fitX: dstX + rectWidth <= mapWidth,
    fitY: dstY + rectHeight <= mapHeight,
  });
  assert(dstX >= 0 && dstY >= 0, 'relocateMapSegment: target must be non-negative');
  assert(dstX + rectWidth <= mapWidth && dstY + rectHeight <= mapHeight, 'relocateMapSegment: target rectangle does not fit inside map');

  // Snapshot terrain data
  const snap = {
    groundHeight: Array.from({ length: rectHeight }, () => new Array<number>(rectWidth)),
    waterHeight: Array.from({ length: rectHeight }, () => new Array<number>(rectWidth)),
    boundaryFlag: Array.from({ length: rectHeight }, () => new Array<boolean>(rectWidth)),
    flags: Array.from({ length: rectHeight }, () => new Array<number>(rectWidth)),
    groundTexture: Array.from({ length: rectHeight }, () => new Array<number>(rectWidth)),
    groundVariation: Array.from({ length: rectHeight }, () => new Array<number>(rectWidth)),
    cliffVariation: Array.from({ length: rectHeight }, () => new Array<number>(rectWidth)),
    cliffTexture: Array.from({ length: rectHeight }, () => new Array<number>(rectWidth)),
    layerHeight: Array.from({ length: rectHeight }, () => new Array<number>(rectWidth)),
  };
  for (let i = 0; i < rectHeight; i++) {
    for (let j = 0; j < rectWidth; j++) {
      const si = srcMinY + i;
      const sj = srcMinX + j;
      snap.groundHeight[i][j] = terrain.groundHeight[si][sj];
      snap.waterHeight[i][j] = terrain.waterHeight[si][sj];
      snap.boundaryFlag[i][j] = terrain.boundaryFlag[si][sj];
      snap.flags[i][j] = terrain.flags[si][sj];
      snap.groundTexture[i][j] = terrain.groundTexture[si][sj];
      snap.groundVariation[i][j] = terrain.groundVariation[si][sj];
      snap.cliffVariation[i][j] = terrain.cliffVariation[si][sj];
      snap.cliffTexture[i][j] = terrain.cliffTexture[si][sj];
      snap.layerHeight[i][j] = terrain.layerHeight[si][sj];
    }
  }

  // Fill the source hole with default terrain values
  const defaults = getInitialTerrain(rectHeight - 1, rectWidth - 1);
  for (let i = 0; i < rectHeight; i++) {
    for (let j = 0; j < rectWidth; j++) {
      const si = srcMinY + i;
      const sj = srcMinX + j;
      terrain.groundHeight[si][sj] = defaults.groundHeight[i][j];
      terrain.waterHeight[si][sj] = defaults.waterHeight[i][j];
      terrain.boundaryFlag[si][sj] = defaults.boundaryFlag[i][j];
      terrain.flags[si][sj] = defaults.flags[i][j];
      terrain.groundTexture[si][sj] = defaults.groundTexture[i][j];
      terrain.groundVariation[si][sj] = defaults.groundVariation[i][j];
      terrain.cliffVariation[si][sj] = defaults.cliffVariation[i][j];
      terrain.cliffTexture[si][sj] = defaults.cliffTexture[i][j];
      terrain.layerHeight[si][sj] = defaults.layerHeight[i][j];
    }
  }

  // Paste snapshot into destination
  for (let i = 0; i < rectHeight; i++) {
    for (let j = 0; j < rectWidth; j++) {
      const di = dstY + i;
      const dj = dstX + j;
      terrain.groundHeight[di][dj] = snap.groundHeight[i][j];
      terrain.waterHeight[di][dj] = snap.waterHeight[i][j];
      terrain.boundaryFlag[di][dj] = snap.boundaryFlag[i][j];
      terrain.flags[di][dj] = snap.flags[i][j];
      terrain.groundTexture[di][dj] = snap.groundTexture[i][j];
      terrain.groundVariation[di][dj] = snap.groundVariation[i][j];
      terrain.cliffVariation[di][dj] = snap.cliffVariation[i][j];
      terrain.cliffTexture[di][dj] = snap.cliffTexture[i][j];
      terrain.layerHeight[di][dj] = snap.layerHeight[i][j];
    }
  }

  // Move objects by world-space delta for those inside the source rectangle
  const worldMinX = terrain.map.offset.x + srcMinX * distancePerTile;
  const worldMinY = terrain.map.offset.y + srcMinY * distancePerTile;
  const worldMaxX = terrain.map.offset.x + (srcMaxX + 1) * distancePerTile;
  const worldMaxY = terrain.map.offset.y + (srcMaxY + 1) * distancePerTile;
  const worldDeltaX = (dstX - srcMinX) * distancePerTile;
  const worldDeltaY = (dstY - srcMinY) * distancePerTile;

  // Doodads (includes destructibles)
  map.doodads.forEach((d) => {
    const px = d.position[0];
    const py = d.position[1];
    if (px >= worldMinX && px < worldMaxX && py >= worldMinY && py < worldMaxY) {
      d.position = [px + worldDeltaX, py + worldDeltaY, d.position[2]];
    }
  });

  // Units
  map.units.forEach((u) => {
    const px = u.position[0];
    const py = u.position[1];
    if (px >= worldMinX && px < worldMaxX && py >= worldMinY && py < worldMaxY) {
      u.position[0] = px + worldDeltaX;
      u.position[1] = py + worldDeltaY;
    }
  });

  // Regions: move regions fully contained within the source rectangle
  if (map.regions) {
    map.regions.forEach((r) => {
      const {
        left, right, bottom, top,
      } = r.position;
      if (left >= worldMinX && right <= worldMaxX && bottom >= worldMinY && top <= worldMaxY) {
        r.position = {
          left: left + worldDeltaX,
          right: right + worldDeltaX,
          bottom: bottom + worldDeltaY,
          top: top + worldDeltaY,
        };
      }
    });
  }

  // Cameras: move cameras whose target lies within the source rectangle
  map.cameras.forEach((cam) => {
    const cx = cam.target.x;
    const cy = cam.target.y;
    if (cx >= worldMinX && cx < worldMaxX && cy >= worldMinY && cy < worldMaxY) {
      cam.target.x = cx + worldDeltaX;
      cam.target.y = cy + worldDeltaY;
    }
  });

  // Players' starting positions
  map.info.players.forEach((player) => {
    player.startingPos.x += worldDeltaX;
    player.startingPos.y += worldDeltaY;
  });
}

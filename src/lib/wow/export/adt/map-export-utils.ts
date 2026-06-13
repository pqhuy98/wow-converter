/**
 * Map export utilities, ported from wow.export
 * (src/js/3D/utils/map-export-utils.js).
 */
import { DB2Row, WDCReader } from '@/lib/wow/db/wdc-reader';

import { constants } from '../../formats/constants';
import { wowConfig, WowReaderConfig } from '../../server/config';

// Cache for GameObjects.db2/GameObjectDisplayInfo.db2 mapping by mapID.
let gameObjectsDB2: Map<number, Set<DB2Row>> | null = null;

/** Normalized ADT export options (overrides > base config > defaults). */
export interface ADTExportOptions {
  mapsExportRaw: boolean;
  pathFormat: string;
  enableSharedTextures: boolean;
  overwriteFiles: boolean;
  splitAlphaMaps: boolean;
  splitLargeTerrainBakes: boolean;
  mapsIncludeHoles: boolean;
  enableSharedChildren: boolean;
  enableAbsoluteCSVPaths: boolean;
  modelsExportCollision: boolean;
  mapsIncludeWMO: boolean;
  mapsIncludeM2: boolean;
  mapsIncludeWMOSets: boolean;
  exportFoliageMeta: boolean;
  mapsIncludeFoliage: boolean;
  mapsIncludeLiquid: boolean;
  mapsIncludeGameObjects: boolean;
  /** When true, ADT export writes placement CSV only for M2/WMO (no OBJ/MTL/BLP). */
  mapsDirectModels: boolean;
}

/**
 * Build a normalized ADT export options object.
 * Values from overrides take precedence, then baseConfig, then defaults.
 */
export function buildADTExportOptions(baseConfig: Partial<WowReaderConfig> = wowConfig, overrides: Record<string, unknown> = {}): ADTExportOptions {
  const base = baseConfig as Record<string, unknown>;
  const pick = (key: string, def: unknown): unknown => (overrides[key] !== undefined ? overrides[key] : (base[key] !== undefined ? base[key] : def));
  return {
    mapsExportRaw: !!pick('mapsExportRaw', false),
    pathFormat: String(pick('pathFormat', 'win32')),
    enableSharedTextures: !!pick('enableSharedTextures', false),
    overwriteFiles: !!pick('overwriteFiles', true),
    splitAlphaMaps: !!pick('splitAlphaMaps', false),
    splitLargeTerrainBakes: !!pick('splitLargeTerrainBakes', false),
    mapsIncludeHoles: !!pick('mapsIncludeHoles', true),
    enableSharedChildren: !!pick('enableSharedChildren', false),
    enableAbsoluteCSVPaths: !!pick('enableAbsoluteCSVPaths', false),
    modelsExportCollision: !!pick('modelsExportCollision', false),
    mapsIncludeWMO: !!pick('mapsIncludeWMO', true),
    mapsIncludeM2: !!pick('mapsIncludeM2', true),
    mapsIncludeWMOSets: !!pick('mapsIncludeWMOSets', true),
    exportFoliageMeta: !!pick('exportFoliageMeta', false),
    mapsIncludeFoliage: !!pick('mapsIncludeFoliage', false),
    mapsIncludeLiquid: !!pick('mapsIncludeLiquid', true),
    mapsIncludeGameObjects: !!pick('mapsIncludeGameObjects', false),
    mapsDirectModels: !!pick('mapsDirectModels', true),
  };
}

/** Compute ADT tile world bounds. */
export function getTileBounds(tileX: number, tileY: number): { startX: number; startY: number; endX: number; endY: number } {
  const TILE_SIZE = constants.GAME.TILE_SIZE;
  const MAP_OFFSET = constants.GAME.MAP_OFFSET;
  const startX = MAP_OFFSET - (tileX * TILE_SIZE) - TILE_SIZE;
  const startY = MAP_OFFSET - (tileY * TILE_SIZE) - TILE_SIZE;
  return {
    startX, startY, endX: startX + TILE_SIZE, endY: startY + TILE_SIZE,
  };
}

/**
 * Collect game objects for a specific map with optional filter.
 * Indexed by mapID and cached across calls.
 */
export async function collectGameObjects(mapID: number, filter?: (row: DB2Row) => boolean): Promise<Set<DB2Row>> {
  if (gameObjectsDB2 === null) {
    const objTable = new WDCReader('DBFilesClient/GameObjects.db2');
    await objTable.parse();

    const idTable = new WDCReader('DBFilesClient/GameObjectDisplayInfo.db2');
    await idTable.parse();

    gameObjectsDB2 = new Map();
    for (const row of objTable.getAllRows().values()) {
      const fidRow = idTable.getRow(row.DisplayID as number);
      if (fidRow !== null) {
        row.FileDataID = fidRow.FileDataID;
        let map = gameObjectsDB2.get(row.OwnerID as number);
        if (map === undefined) {
          map = new Set();
          map.add(row);
          gameObjectsDB2.set(row.OwnerID as number, map);
        } else {
          map.add(row);
        }
      }
    }
  }

  const result = new Set<DB2Row>();
  const mapObjects = gameObjectsDB2.get(mapID);
  if (mapObjects !== undefined) {
    for (const obj of mapObjects) {
      if (filter !== undefined && filter(obj)) result.add(obj);
    }
  }
  return result;
}

export default { buildADTExportOptions, getTileBounds, collectGameObjects };

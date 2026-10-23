/** Matches maps-generate API validation (alphanumeric, _, ., -, optional .w3x). */
export const MAP_SAVE_NAME_REGEX = /^[a-zA-Z0-9_.-]+(\.w3x)?$/i;

/** @deprecated Use MAP_SAVE_NAME_BASE_REGEX from map-save-name utils for form validation. */
export { MAP_SAVE_NAME_BASE_REGEX } from '@/lib/utils/map-save-name';

export interface MapGenerateTileSuccess {
  tileX: number;
  tileY: number;
  result: {
    exportType: string;
    mainFile: string | null;
  };
}

export interface MapGenerateTileFailure {
  tileX: number;
  tileY: number;
  error: string;
}

export interface MapGenerateJobResult {
  id: 'WC3_MAP_GENERATE_SUMMARY';
  map: string;
  mapID: number;
  mapSaveName: string;
  outputDir: string;
  quality: number;
  total: number;
  stepsPerTile: number;
  totalSteps: number;
  succeeded: MapGenerateTileSuccess[];
  failed: MapGenerateTileFailure[];
  warnings?: string[];
}

export interface MapGenerateProgress {
  completedSteps: number;
  totalSteps: number;
  phase: 'adt' | 'convert';
  tileIndex?: number;
  tileCount?: number;
  stepsPerTile?: number;
  currentTile?: { x: number; y: number };
  taskName?: string;
  creatureCompleted?: number;
  creatureTotal?: number;
  percent: number;
}

export interface MapGenerateJobStatus {
  id: string;
  status: 'pending' | 'processing' | 'done' | 'failed' | 'cancelled';
  position?: number;
  mapSaveName?: string;
  mapDir?: string;
  queuePending?: number;
  result?: MapGenerateJobResult;
  error?: string;
  submittedAt: number;
  startedAt?: number;
  finishedAt?: number;
  progress?: MapGenerateProgress;
}

export interface MapTextureSettings {
  terrain: number;
  unit: number;
  doodad: number;
}

function defaultMapTextureSettings(isClassic: boolean): MapTextureSettings {
  return { terrain: 4096, unit: isClassic ? 256 : 0, doodad: isClassic ? 256 : 0 };
}

export function readMapExportPreferences(raw: string | null, isClassic: boolean) {
  const settings = { ...defaultMapTextureSettings(isClassic), includeBuildingInteriors: true };
  try {
    const parsed: unknown = JSON.parse(raw ?? 'null');
    if (!parsed || typeof parsed !== 'object') return settings;
    if ('terrain' in parsed && typeof parsed.terrain === 'number' && [1024, 2048, 4096, 8192].includes(parsed.terrain)) settings.terrain = parsed.terrain;
    if ('unit' in parsed && typeof parsed.unit === 'number' && [0, 256, 512, 1024].includes(parsed.unit)) settings.unit = parsed.unit;
    if ('doodad' in parsed && typeof parsed.doodad === 'number' && [0, 256, 512, 1024].includes(parsed.doodad)) settings.doodad = parsed.doodad;
    if ('includeBuildingInteriors' in parsed && typeof parsed.includeBuildingInteriors === 'boolean') settings.includeBuildingInteriors = parsed.includeBuildingInteriors;
  } catch { /* Invalid saved settings fall back to this product's defaults. */ }
  return settings;
}

export const MAP_CLAMP_STORAGE_KEY = 'wow-converter.map-clamps';
const MAP_CLAMP_CACHE_LIMIT = 100;

interface MapClampEntry {
  key: string;
  lower: number;
  upper: number;
}

export function mapClampCacheKey(mapId: number | string, cells: { x: number; y: number }[]): string {
  return JSON.stringify([String(mapId).toLowerCase(), [...new Set(cells.map(({ x, y }) => `${x},${y}`))].sort()]);
}

/** Most recently used first; corrupt entries do not discard other saved areas. */
export function readMapClampCache(raw: string | null): MapClampEntry[] {
  const entries: MapClampEntry[] = [];
  try {
    const parsed: unknown = JSON.parse(raw ?? 'null');
    if (!Array.isArray(parsed)) return entries;
    const candidates: unknown[] = parsed;
    for (const entry of candidates) {
      if (!entry || typeof entry !== 'object' || !('key' in entry) || typeof entry.key !== 'string'
        || !('lower' in entry) || typeof entry.lower !== 'number' || !Number.isFinite(entry.lower)
        || !('upper' in entry) || typeof entry.upper !== 'number' || !Number.isFinite(entry.upper)
        || entry.lower < 0 || entry.upper > 1 || entry.lower >= entry.upper
        || entries.some((saved) => saved.key === entry.key)) continue;
      entries.push({ key: entry.key, lower: entry.lower, upper: entry.upper });
      if (entries.length === MAP_CLAMP_CACHE_LIMIT) break;
    }
  } catch { /* Corrupt cache starts empty. */ }
  return entries;
}

export function touchMapClampCache(entries: MapClampEntry[], entry: MapClampEntry): MapClampEntry[] {
  return [entry, ...entries.filter((saved) => saved.key !== entry.key)].slice(0, MAP_CLAMP_CACHE_LIMIT);
}

export interface GenerateWc3FormValues {
  /** Base map filename without .w3x extension. */
  mapSaveName: string;
  clampLower: number;
  clampUpper: number;
  mapAngleDeg: number;
  /** Unit size on the map; also drives suggested terrain height clamp. */
  unitScale: number;
  playerFaction: 'Horde' | 'Alliance';
  /** Include interior-only WMO children; exterior and unclassified children are always kept. */
  includeBuildingInteriors: boolean;
  placePathBlockers: boolean;
  terrainAsWc3Tilesets: boolean;
  freshExport: boolean;
  textures: MapTextureSettings;
  creatures: {
    enable: boolean;
    allAreDoodads: boolean;
  };
}

export const defaultGenerateWc3FormValues: GenerateWc3FormValues = {
  mapSaveName: '',
  clampLower: 0,
  clampUpper: 1,
  mapAngleDeg: 0,
  unitScale: 2,
  playerFaction: 'Alliance',
  includeBuildingInteriors: true,
  placePathBlockers: true,
  terrainAsWc3Tilesets: false,
  freshExport: false,
  textures: defaultMapTextureSettings(false),
  creatures: {
    enable: false,
    allAreDoodads: false,
  },
};

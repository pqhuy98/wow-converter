/** Matches maps-generate API validation (alphanumeric, _, ., -, optional .w3x). */
export const MAP_SAVE_NAME_REGEX = /^[a-zA-Z0-9_.-]+(\.w3x)?$/i;

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
  status: 'pending' | 'processing' | 'done' | 'failed';
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

export interface GenerateWc3FormValues {
  mapSaveName: string;
  clampLower: number;
  clampUpper: number;
  mapAngleDeg: number;
  /** Unit size on the map; also drives terrain height auto-clamp. */
  unitScale: number;
  freshExport: boolean;
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
  freshExport: false,
  creatures: {
    enable: false,
    allAreDoodads: false,
  },
};

export interface MapExportTileSuccess {
  tileX: number;
  tileY: number;
  result: {
    exportType: string;
    mainFile: string | null;
  };
}

export interface MapExportTileFailure {
  tileX: number;
  tileY: number;
  error: string;
}

export interface MapExportJobResult {
  id: 'ADT_EXPORT_SUMMARY';
  map: string;
  mapID: number;
  quality: number;
  total: number;
  stepsPerTile: number;
  totalSteps: number;
  succeeded: MapExportTileSuccess[];
  failed: MapExportTileFailure[];
}

export interface MapExportProgress {
  completedSteps: number;
  totalSteps: number;
  tileIndex: number;
  tileCount: number;
  stepsPerTile: number;
  currentTile?: { x: number; y: number };
  taskName?: string;
  taskValue?: number;
  taskMax?: number;
  percent: number;
}

export interface MapExportJobStatus {
  id: string;
  status: 'pending' | 'processing' | 'done' | 'failed';
  position?: number;
  result?: MapExportJobResult;
  error?: string;
  submittedAt: number;
  startedAt?: number;
  finishedAt?: number;
  progress?: MapExportProgress;
}

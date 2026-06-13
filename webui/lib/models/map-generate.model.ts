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
  freshExport: boolean;
  creatures: {
    enable: boolean;
    allAreDoodads: boolean;
    scaleUp: number;
  };
}

export const defaultGenerateWc3FormValues: GenerateWc3FormValues = {
  mapSaveName: '',
  clampLower: 0,
  clampUpper: 1,
  mapAngleDeg: 0,
  freshExport: false,
  creatures: {
    enable: false,
    allAreDoodads: true,
    scaleUp: 2,
  },
};

export const WC3_GENERATE_STORAGE_KEY = 'wow-converter-wc3-generate';

export interface StoredGenerateJob {
  jobId: string;
  mapSaveName: string;
  mapDir: string;
}

export function readStoredGenerateJob(): StoredGenerateJob | undefined {
  if (typeof window === 'undefined') return undefined;
  try {
    const raw = localStorage.getItem(WC3_GENERATE_STORAGE_KEY);
    if (!raw) return undefined;
    const parsed = JSON.parse(raw) as StoredGenerateJob;
    if (!parsed.jobId) return undefined;
    return parsed;
  } catch {
    return undefined;
  }
}

export function writeStoredGenerateJob(job: StoredGenerateJob): void {
  if (typeof window === 'undefined') return;
  localStorage.setItem(WC3_GENERATE_STORAGE_KEY, JSON.stringify(job));
}

export function clearStoredGenerateJob(): void {
  if (typeof window === 'undefined') return;
  localStorage.removeItem(WC3_GENERATE_STORAGE_KEY);
}

export function formatElapsedDuration(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
  }
  return `${minutes}:${String(seconds).padStart(2, '0')}`;
}

export function formatProgressLabel(job: MapGenerateJobStatus): string {
  if (job.status === 'pending') {
    if (job.position != null && job.position > 1) {
      const waiting = job.queuePending != null && job.queuePending > 0
        ? ` · ${job.queuePending} waiting in queue`
        : '';
      return `Queued (position ${job.position})${waiting}`;
    }
    if (job.queuePending != null && job.queuePending > 0) {
      return `Queued · ${job.queuePending} waiting in queue`;
    }
    return 'Queued…';
  }

  const p = job.progress;
  if (!p) return 'Starting…';

  const step = `Step ${p.completedSteps} / ${p.totalSteps}`;
  if (p.creatureTotal != null && p.creatureCompleted != null) {
    return `${step} · Exporting creatures (${p.creatureCompleted}/${p.creatureTotal})`;
  }
  if (p.phase === 'adt' && p.tileCount != null && p.tileIndex != null) {
    const tile = p.currentTile
      ? ` (${p.currentTile.x}, ${p.currentTile.y})`
      : '';
    return `${step} · tile ${p.tileIndex + 1}/${p.tileCount}${tile}`
      + (p.taskName ? ` · ${p.taskName}` : '');
  }
  return `${step}${p.taskName ? ` · ${p.taskName}` : ''}`;
}

export function persistGenerateJobFromStatus(job: MapGenerateJobStatus): void {
  if (!job.id || job.status === 'done' || job.status === 'failed') {
    clearStoredGenerateJob();
    return;
  }
  writeStoredGenerateJob({
    jobId: job.id,
    mapSaveName: job.mapSaveName ?? job.result?.mapSaveName ?? '',
    mapDir: job.mapDir ?? job.result?.map ?? '',
  });
}

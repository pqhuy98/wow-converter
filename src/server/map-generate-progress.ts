export interface MapGenerateProgressSnapshot {
  completedSteps: number;
  totalSteps: number;
  phase: 'adt' | 'convert';
  taskName?: string;
  tileIndex?: number;
  tileCount?: number;
  stepsPerTile?: number;
  currentTile?: { x: number; y: number };
  creatureCompleted?: number;
  creatureTotal?: number;
}

const progressStore = new Map<string, MapGenerateProgressSnapshot>();

export function initMapGenerateProgress(
  key: string,
  params: {
    adtTotalSteps: number;
    convertSteps: number;
    tileCount: number;
    stepsPerTile: number;
  },
): void {
  progressStore.set(key, {
    completedSteps: 0,
    totalSteps: params.adtTotalSteps + params.convertSteps,
    phase: 'adt',
    taskName: 'Exporting tiles',
    tileIndex: 0,
    tileCount: params.tileCount,
    stepsPerTile: params.stepsPerTile,
  });
}

export function updateMapGenerateTotalSteps(key: string, convertSteps: number, adtTotalSteps: number): void {
  const snap = progressStore.get(key);
  if (!snap) return;
  snap.totalSteps = adtTotalSteps + convertSteps;
}

export function setMapGeneratePhase(
  key: string,
  phase: 'adt' | 'convert',
  taskName: string,
): void {
  const snap = progressStore.get(key);
  if (!snap) return;
  snap.phase = phase;
  snap.taskName = taskName;
  if (phase === 'convert') {
    snap.tileIndex = undefined;
    snap.currentTile = undefined;
  }
}

export function syncAdtProgress(
  key: string,
  adtSnap: {
    completedSteps: number;
    tileIndex: number;
    tileCount: number;
    stepsPerTile: number;
    currentTile?: { x: number; y: number };
    taskName?: string;
  },
): void {
  const snap = progressStore.get(key);
  if (!snap) return;
  snap.phase = 'adt';
  snap.completedSteps = adtSnap.completedSteps;
  snap.tileIndex = adtSnap.tileIndex;
  snap.tileCount = adtSnap.tileCount;
  snap.stepsPerTile = adtSnap.stepsPerTile;
  snap.currentTile = adtSnap.currentTile;
  snap.taskName = adtSnap.taskName ?? 'Exporting tiles';
}

export function advanceMapGenerateProgress(
  key: string,
  adtTotalSteps: number,
  convertCompletedSteps: number,
  taskName: string,
  creatureProgress?: { completed: number; total: number },
): void {
  const snap = progressStore.get(key);
  if (!snap) return;
  snap.phase = 'convert';
  snap.completedSteps = adtTotalSteps + convertCompletedSteps;
  snap.taskName = taskName;
  snap.tileIndex = undefined;
  snap.currentTile = undefined;
  if (creatureProgress) {
    snap.creatureCompleted = creatureProgress.completed;
    snap.creatureTotal = creatureProgress.total;
  } else {
    snap.creatureCompleted = undefined;
    snap.creatureTotal = undefined;
  }
}

export function getMapGenerateProgress(key: string): MapGenerateProgressSnapshot | undefined {
  return progressStore.get(key);
}

export function clearMapGenerateProgress(key: string): void {
  progressStore.delete(key);
}

export function toProgressPercent(snap: MapGenerateProgressSnapshot): number {
  return snap.totalSteps > 0
    ? Math.min(100, Math.round((snap.completedSteps / snap.totalSteps) * 100))
    : 0;
}

/**
 * In-process export progress tracking for ADT batch exports.
 * Shared between wow-data-server (writer) and the web UI server (reader via REST).
 */
import type { ADTExportOptions } from './adt/map-export-utils';

/** Legacy OBJ export still advances per model; keep a generous cap. */
export const MODEL_STEP_BUDGET = 512;

export const FIXED_OVERHEAD_STEPS = 2;

export interface ExportProgressSnapshot {
  completedSteps: number;
  totalSteps: number;
  tileIndex: number;
  tileCount: number;
  stepsPerTile: number;
  currentTile?: { x: number; y: number };
  taskName?: string;
  taskValue?: number;
  taskMax?: number;
}

export interface ExportProgress {
  advance(steps?: number): void;
  setLabel(name: string, value?: number, max?: number): void;
  syncTileComplete(): void;
}

/**
 * Fixed phase budget per tile for a uniform tiles × stepsPerTile progress bar.
 * Direct mode uses coarse phases (typically 6 steps/tile); legacy OBJ export
 * keeps the per-model step budget.
 */
export function computeStepsPerTile(quality: number, options: ADTExportOptions): number {
  let steps = FIXED_OVERHEAD_STEPS;
  if (quality !== 0) steps += 1;
  if (options.mapsIncludeM2 || options.mapsIncludeWMO || options.mapsIncludeGameObjects) {
    steps += options.mapsDirectModels ? 1 : MODEL_STEP_BUDGET;
  }
  if (options.mapsIncludeLiquid) steps += 1;
  if (options.mapsIncludeFoliage) steps += 1;
  return steps;
}

const progressStore = new Map<string, ExportProgressSnapshot>();

export function getExportProgressSnapshot(key: string): ExportProgressSnapshot | undefined {
  return progressStore.get(key);
}

export function clearExportProgressSnapshot(key: string): void {
  progressStore.delete(key);
}

export interface BatchExportProgressParams {
  key: string;
  tileIndex: number;
  tileCount: number;
  stepsPerTile: number;
  currentTile: { x: number; y: number };
}

export function createBatchExportProgress(params: BatchExportProgressParams): ExportProgress {
  const {
    key, tileIndex, tileCount, stepsPerTile, currentTile,
  } = params;
  const totalSteps = tileCount * stepsPerTile;
  const tileBase = tileIndex * stepsPerTile;

  let tileSteps = 0;
  let taskName = '';
  let taskValue: number | undefined;
  let taskMax: number | undefined;

  const publish = (completedSteps: number): void => {
    progressStore.set(key, {
      completedSteps,
      totalSteps,
      tileIndex,
      tileCount,
      stepsPerTile,
      currentTile,
      taskName: taskName || undefined,
      taskValue,
      taskMax,
    });
  };

  publish(tileBase);

  return {
    advance(steps = 1) {
      tileSteps = Math.min(tileSteps + steps, Math.max(0, stepsPerTile - 1));
      publish(tileBase + tileSteps);
    },
    setLabel(name: string, value?: number, max?: number) {
      taskName = name;
      taskValue = value;
      taskMax = max;
      publish(tileBase + tileSteps);
    },
    syncTileComplete() {
      tileSteps = stepsPerTile;
      publish(tileBase + stepsPerTile);
    },
  };
}

export function finalizeExportProgress(key: string): void {
  const snap = progressStore.get(key);
  if (snap) {
    progressStore.set(key, {
      ...snap,
      completedSteps: snap.totalSteps,
      taskName: 'Complete',
      taskValue: undefined,
      taskMax: undefined,
    });
  }
}

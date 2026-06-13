import { randomUUID } from 'crypto';
import express from 'express';
import { z } from 'zod';

import {
  buildMapExportConfig,
  runMapGenerateConversion,
} from '@/lib/converter/map-exporter/run-map-generate';
import { getDefaultConfig } from '@/lib/global-config';
import { Vector2 } from '@/lib/math/common';
import { buildADTExportOptions } from '@/lib/wow/export/adt/map-export-utils';
import { computeStepsPerTile } from '@/lib/wow/export/export-progress';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import {
  advanceMapGenerateProgress,
  clearMapGenerateProgress,
  getMapGenerateProgress,
  initMapGenerateProgress,
  setMapGeneratePhase,
  syncAdtProgress,
  toProgressPercent,
  updateMapGenerateTotalSteps,
} from '@/server/map-generate-progress';
import { Job, JobQueue } from '@/server/utils/job-queue';

import type { MapExportTileFailure, MapExportTileSuccess } from './maps-export';

const tileSchema = z.object({
  x: z.number().int().min(0).max(63),
  y: z.number().int().min(0).max(63),
});

const qualitySchema = z.union([
  z.literal(0),
  z.literal(512),
  z.literal(1024),
  z.literal(2048),
  z.literal(4096),
  z.literal(8192),
  z.literal(16384),
]);

export const generateWc3BodySchema = z.object({
  tiles: z.array(tileSchema).min(1),
  quality: qualitySchema,
  mapSaveName: z.string().trim().min(1).max(128)
    .regex(/^[a-zA-Z0-9_.-]+(\.w3x)?$/i),
  clampLower: z.number().min(0).max(1),
  clampUpper: z.number().min(0).max(1),
  mapAngleDeg: z.number(),
  freshExport: z.boolean(),
  creatures: z.object({
    enable: z.boolean(),
    allAreDoodads: z.boolean(),
    scaleUp: z.number().positive(),
  }),
}).strict().refine((d) => d.clampUpper >= d.clampLower, {
  message: 'clampUpper must be >= clampLower',
  path: ['clampUpper'],
});

export type GenerateWc3Body = z.infer<typeof generateWc3BodySchema>;

export interface MapGenerateJobRequest {
  mapDir: string;
  mapID: number;
  body: GenerateWc3Body;
  orderedTiles: readonly { x: number; y: number }[];
  tileBounds: { min: Vector2; max: Vector2 };
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
  succeeded: MapExportTileSuccess[];
  failed: MapExportTileFailure[];
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

type MapGenerateJob = Job<MapGenerateJobRequest, MapGenerateJobResult>;

const INITIAL_CONVERT_STEPS = 3;

function tileBoundsFromTiles(tiles: readonly { x: number; y: number }[]): { min: Vector2; max: Vector2 } {
  const xs = tiles.map((t) => t.x);
  const ys = tiles.map((t) => t.y);
  return {
    min: [Math.min(...xs), Math.min(...ys)],
    max: [Math.max(...xs), Math.max(...ys)],
  };
}

async function pollAdtProgress(
  jobId: string,
  adtTotalSteps: number,
  until: () => boolean,
): Promise<void> {
  while (!until()) {
    const snap = await wowExportClient.getExportProgress(jobId);
    if (snap) {
      syncAdtProgress(jobId, {
        completedSteps: Math.min(snap.completedSteps, adtTotalSteps),
        tileIndex: snap.tileIndex,
        tileCount: snap.tileCount,
        stepsPerTile: snap.stepsPerTile,
        currentTile: snap.currentTile,
        taskName: snap.taskName,
      });
    }
    await new Promise((r) => { setTimeout(r, 250); });
  }
}

const mapGenerateQueue = new JobQueue<MapGenerateJobRequest, MapGenerateJobResult>(
  {
    concurrency: 1,
    maxPendingJobs: 10,
    jobTTL: 10 * 60 * 1000,
    jobTimeout: 2 * 60 * 60 * 1000,
  },
  async (job) => {
    const {
      mapDir, mapID, body, orderedTiles, tileBounds,
    } = job.request;
    const progressKey = job.id;

    const exportOptions = buildADTExportOptions(undefined, {
      mapsIncludeM2: true,
      mapsIncludeWMO: true,
      mapsIncludeWMOSets: true,
      mapsIncludeGameObjects: true,
      mapsIncludeLiquid: true,
      mapsIncludeFoliage: true,
      mapsIncludeHoles: true,
    });
    const stepsPerTile = computeStepsPerTile(body.quality, exportOptions);
    const tileCount = orderedTiles.length;
    const adtTotalSteps = tileCount * stepsPerTile;

    initMapGenerateProgress(progressKey, {
      adtTotalSteps,
      convertSteps: INITIAL_CONVERT_STEPS,
      tileCount,
      stepsPerTile,
    });

    await wowExportClient.waitUntilReady();

    const succeeded: MapExportTileSuccess[] = [];
    const failed: MapExportTileFailure[] = [];

    for (let tileIndex = 0; tileIndex < orderedTiles.length; tileIndex++) {
      const { x: tileX, y: tileY } = orderedTiles[tileIndex];
      let pollDone = false;
      const pollPromise = pollAdtProgress(progressKey, adtTotalSteps, () => pollDone);

      try {
        const result = await wowExportClient.exportADT({
          mapID,
          mapDir,
          tileX,
          tileY,
          quality: body.quality,
          includeM2: true,
          includeWMO: true,
          includeWMOSets: true,
          includeGameObjects: true,
          includeLiquid: true,
          includeFoliage: true,
          includeHoles: true,
          progressKey,
          tileIndex,
          tileCount,
          stepsPerTile,
        });
        succeeded.push({ tileX, tileY, result });
      } catch (e) {
        failed.push({ tileX, tileY, error: e instanceof Error ? e.message : String(e) });
      } finally {
        pollDone = true;
        await pollPromise;
        syncAdtProgress(progressKey, {
          completedSteps: Math.min((tileIndex + 1) * stepsPerTile, adtTotalSteps),
          tileIndex,
          tileCount,
          stepsPerTile,
          currentTile: { x: tileX, y: tileY },
          taskName: tileIndex + 1 >= tileCount ? 'Tiles exported' : 'Exporting tiles',
        });
      }
    }

    await wowExportClient.finalizeExportProgress(progressKey);

    if (failed.length === orderedTiles.length) {
      clearMapGenerateProgress(progressKey);
      throw new Error(failed[0]?.error ?? 'All tiles failed to export');
    }

    setMapGeneratePhase(progressKey, 'convert', 'Converting to WC3 map');
    syncAdtProgress(progressKey, {
      completedSteps: adtTotalSteps,
      tileIndex: tileCount - 1,
      tileCount,
      stepsPerTile,
      taskName: 'Tiles exported',
    });

    const config = await getDefaultConfig();
    const mapExportConfig = buildMapExportConfig({
      mapId: mapID,
      wowExportFolder: mapDir.toLowerCase(),
      min: tileBounds.min,
      max: tileBounds.max,
      mapAngleDeg: body.mapAngleDeg,
      clampLower: body.clampLower,
      clampUpper: body.clampUpper,
      creatures: body.creatures,
    });

    let convertSteps = INITIAL_CONVERT_STEPS;
    const conversion = await runMapGenerateConversion({
      config,
      mapExportConfig,
      mapSaveName: body.mapSaveName,
      freshExport: body.freshExport,
      creatureScaleUp: body.creatures.scaleUp,
      onConvertStepsKnown: (steps) => {
        convertSteps = steps;
        updateMapGenerateTotalSteps(progressKey, convertSteps, adtTotalSteps);
      },
      onProgress: (convertCompleted, taskName, creatureProgress) => {
        advanceMapGenerateProgress(
          progressKey,
          adtTotalSteps,
          convertCompleted,
          taskName,
          creatureProgress,
        );
      },
    });

    const totalSteps = adtTotalSteps + conversion.convertSteps;
    advanceMapGenerateProgress(progressKey, adtTotalSteps, conversion.convertSteps, 'Complete');

    return {
      id: 'WC3_MAP_GENERATE_SUMMARY',
      map: mapDir,
      mapID,
      mapSaveName: conversion.mapSaveName,
      outputDir: conversion.outputDir,
      quality: body.quality,
      total: orderedTiles.length,
      stepsPerTile,
      totalSteps,
      succeeded,
      failed,
    };
  },
);

function buildProgressStatus(jobId: string): MapGenerateProgress | undefined {
  const snap = getMapGenerateProgress(jobId);
  if (!snap) return undefined;
  return {
    completedSteps: snap.completedSteps,
    totalSteps: snap.totalSteps,
    phase: snap.phase,
    tileIndex: snap.tileIndex,
    tileCount: snap.tileCount,
    stepsPerTile: snap.stepsPerTile,
    currentTile: snap.currentTile,
    taskName: snap.taskName,
    creatureCompleted: snap.creatureCompleted,
    creatureTotal: snap.creatureTotal,
    percent: toProgressPercent(snap),
  };
}

function enrichJobMeta(status: MapGenerateJobStatus, job: MapGenerateJob): void {
  status.mapSaveName = job.request.body.mapSaveName;
  status.mapDir = job.request.mapDir;
  status.queuePending = mapGenerateQueue.getQueueSnapshot().pendingCount;
}

async function buildJobStatus(jobId: string): Promise<MapGenerateJobStatus | undefined> {
  const base = mapGenerateQueue.getJobStatus(jobId);
  const job = mapGenerateQueue.getJob(jobId);
  if (!base || !job) return undefined;

  const status: MapGenerateJobStatus = {
    id: base.id,
    status: base.status,
    position: base.position,
    result: base.result,
    error: base.error,
    submittedAt: base.submittedAt,
    startedAt: base.startedAt,
    finishedAt: base.finishedAt,
  };
  enrichJobMeta(status, job);

  if (base.status === 'processing') {
    if (getMapGenerateProgress(jobId)) {
      status.progress = buildProgressStatus(jobId);
    } else {
      const adtSnap = await wowExportClient.getExportProgress(jobId);
      if (adtSnap) {
        status.progress = {
          completedSteps: adtSnap.completedSteps,
          totalSteps: adtSnap.totalSteps,
          phase: 'adt',
          tileIndex: adtSnap.tileIndex,
          tileCount: adtSnap.tileCount,
          stepsPerTile: adtSnap.stepsPerTile,
          currentTile: adtSnap.currentTile,
          taskName: adtSnap.taskName,
          percent: adtSnap.totalSteps > 0
            ? Math.min(100, Math.round((adtSnap.completedSteps / adtSnap.totalSteps) * 100))
            : 0,
        };
      }
    }
  } else if (base.status === 'done' && base.result) {
    status.progress = {
      completedSteps: base.result.totalSteps,
      totalSteps: base.result.totalSteps,
      phase: 'convert',
      taskName: 'Complete',
      percent: 100,
    };
    clearMapGenerateProgress(jobId);
  } else if (base.status === 'failed') {
    clearMapGenerateProgress(jobId);
  }

  return status;
}

export function registerMapGenerateRoutes(
  router: express.Router,
  resolveMap: (key: string) => { id: number; dir: string } | undefined,
): void {
  router.post('/maps/:map/generate-wc3', (req, res) => {
    try {
      const key = String(req.params.map).toLowerCase();
      const entry = resolveMap(key);
      if (!entry) return res.status(404).json({ error: 'Unknown map' });

      const parsed = generateWc3BodySchema.safeParse(req.body);
      if (!parsed.success) {
        return res.status(400).json({ error: 'Invalid request body', issues: z.treeifyError(parsed.error) });
      }

      const unique = new Map<string, { x: number; y: number }>();
      for (const t of parsed.data.tiles) unique.set(`${t.x},${t.y}`, t);
      const orderedTiles = Array.from(unique.values()).sort((a, b) => (a.y - b.y) || (a.x - b.x));

      const job: MapGenerateJob = {
        id: randomUUID(),
        request: {
          mapDir: entry.dir,
          mapID: entry.id,
          body: parsed.data,
          orderedTiles,
          tileBounds: tileBoundsFromTiles(orderedTiles),
        },
        status: 'pending',
        submittedAt: Date.now(),
        noTimeout: true,
      };

      mapGenerateQueue.addJob(job);
      console.log('generate-wc3 queued', key, job.id, `${orderedTiles.length} tiles → ${parsed.data.mapSaveName}`);

      setImmediate(() => {
        void buildJobStatus(job.id).then((status) => {
          res.json(status);
        });
      });
      return undefined;
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });

  router.get('/maps/generate-wc3/status/:jobId', (req, res) => {
    void (async () => {
      const status = await buildJobStatus(req.params.jobId);
      if (!status) {
        res.status(404).json({ error: 'Generate job not found' });
        return;
      }
      res.json(status);
    })();
  });

  router.get('/maps/generate-wc3/active', (_req, res) => {
    void (async () => {
      const ids = mapGenerateQueue.listActiveJobIds();
      const jobs = await Promise.all(ids.map((id) => buildJobStatus(id)));
      res.json(jobs.filter((j): j is MapGenerateJobStatus => j != null));
    })();
  });
}

export { tileBoundsFromTiles };

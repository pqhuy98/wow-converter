import { randomUUID } from 'crypto';
import express from 'express';
import { z } from 'zod';

import { buildADTExportOptions } from '@/lib/wow/export/adt/map-export-utils';
import {
  computeStepsPerTile,
} from '@/lib/wow/export/export-progress';
import { ExportADTResult, wowDataClient } from '@/lib/wow-data-client/wow-data-client';
import { Job, JobQueue } from '@/server/utils/job-queue';

const tileSchema = z.object({
  x: z.number().int().min(0).max(63),
  y: z.number().int().min(0).max(63),
});

export const exportAdtBodySchema = z.object({
  tiles: z.array(tileSchema).min(1),
  quality: z.union([
    z.literal(0),
    z.literal(512),
    z.literal(1024),
    z.literal(2048),
    z.literal(4096),
    z.literal(8192),
    z.literal(16384),
  ]),
  includeM2: z.boolean(),
  includeWMO: z.boolean(),
  includeWMOSets: z.boolean(),
  includeGameObjects: z.boolean(),
  includeLiquid: z.boolean(),
  includeFoliage: z.boolean(),
  includeHoles: z.boolean(),
}).strict();

export type ExportAdtBody = z.infer<typeof exportAdtBodySchema>;

export interface MapExportJobRequest {
  mapDir: string;
  mapID: number;
  body: ExportAdtBody;
  orderedTiles: readonly { x: number; y: number }[];
}

export interface MapExportTileSuccess {
  tileX: number;
  tileY: number;
  result: ExportADTResult;
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

export interface MapExportJobStatus {
  id: string;
  status: 'pending' | 'processing' | 'done' | 'failed';
  position?: number;
  result?: MapExportJobResult;
  error?: string;
  submittedAt: number;
  startedAt?: number;
  finishedAt?: number;
  progress?: {
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
  };
}

type MapExportJob = Job<MapExportJobRequest, MapExportJobResult>;

const mapExportQueue = new JobQueue<MapExportJobRequest, MapExportJobResult>(
  {
    concurrency: 1,
    maxPendingJobs: 20,
    jobTTL: 10 * 60 * 1000,
    jobTimeout: 60 * 60 * 1000,
  },
  async (job) => {
    const {
      mapDir, mapID, body, orderedTiles,
    } = job.request;
    const exportOptions = buildADTExportOptions(undefined, {
      mapsIncludeM2: body.includeM2,
      mapsIncludeWMO: body.includeWMO,
      mapsIncludeWMOSets: body.includeWMOSets,
      mapsIncludeGameObjects: body.includeGameObjects,
      mapsIncludeLiquid: body.includeLiquid,
      mapsIncludeFoliage: body.includeFoliage,
      mapsIncludeHoles: body.includeHoles,
    });
    const stepsPerTile = computeStepsPerTile(body.quality, exportOptions);
    const tileCount = orderedTiles.length;
    const progressKey = job.id;

    await wowDataClient.waitUntilReady();

    const succeeded: MapExportTileSuccess[] = [];
    const failed: MapExportTileFailure[] = [];

    for (let tileIndex = 0; tileIndex < orderedTiles.length; tileIndex++) {
      const { x: tileX, y: tileY } = orderedTiles[tileIndex];
      try {
        const result = await wowDataClient.exportADT({
          mapID,
          mapDir,
          tileX,
          tileY,
          quality: body.quality,
          includeM2: body.includeM2,
          includeWMO: body.includeWMO,
          includeWMOSets: body.includeWMOSets,
          includeGameObjects: body.includeGameObjects,
          includeLiquid: body.includeLiquid,
          includeFoliage: body.includeFoliage,
          includeHoles: body.includeHoles,
          progressKey,
          tileIndex,
          tileCount,
          stepsPerTile,
        });
        succeeded.push({ tileX, tileY, result });
      } catch (e) {
        failed.push({ tileX, tileY, error: e instanceof Error ? e.message : String(e) });
      }
    }

    await wowDataClient.finalizeExportProgress(progressKey);

    if (failed.length === orderedTiles.length) {
      throw new Error(failed[0]?.error ?? 'All tiles failed to export');
    }

    return {
      id: 'ADT_EXPORT_SUMMARY',
      map: mapDir,
      mapID,
      quality: body.quality,
      total: orderedTiles.length,
      stepsPerTile,
      totalSteps: tileCount * stepsPerTile,
      succeeded,
      failed,
    };
  },
);

async function buildJobStatus(jobId: string): Promise<MapExportJobStatus | undefined> {
  const base = mapExportQueue.getJobStatus(jobId);
  if (!base) return undefined;

  const status: MapExportJobStatus = {
    id: base.id,
    status: base.status,
    position: base.position,
    result: base.result,
    error: base.error,
    submittedAt: base.submittedAt,
    startedAt: base.startedAt,
    finishedAt: base.finishedAt,
  };

  if (base.status === 'processing') {
    const snap = await wowDataClient.getExportProgress(jobId);
    if (snap) {
      status.progress = {
        completedSteps: snap.completedSteps,
        totalSteps: snap.totalSteps,
        tileIndex: snap.tileIndex,
        tileCount: snap.tileCount,
        stepsPerTile: snap.stepsPerTile,
        currentTile: snap.currentTile,
        taskName: snap.taskName,
        taskValue: snap.taskValue,
        taskMax: snap.taskMax,
        percent: snap.totalSteps > 0
          ? Math.min(100, Math.round((snap.completedSteps / snap.totalSteps) * 100))
          : 0,
      };
    }
  } else if (base.status === 'done' && base.result) {
    status.progress = {
      completedSteps: base.result.totalSteps,
      totalSteps: base.result.totalSteps,
      tileIndex: base.result.total - 1,
      tileCount: base.result.total,
      stepsPerTile: base.result.stepsPerTile,
      taskName: 'Complete',
      percent: 100,
    };
  }

  return status;
}

export function registerMapExportRoutes(router: express.Router, resolveMap: (key: string) => { id: number; dir: string } | undefined): void {
  router.post('/maps/:map/export-adt', (req, res) => {
    try {
      const key = String(req.params.map).toLowerCase();
      const entry = resolveMap(key);
      if (!entry) return res.status(404).json({ error: 'Unknown map' });

      const parsed = exportAdtBodySchema.safeParse(req.body);
      if (!parsed.success) {
        return res.status(400).json({ error: 'Invalid request body', issues: z.treeifyError(parsed.error) });
      }

      const unique = new Map<string, { x: number; y: number }>();
      for (const t of parsed.data.tiles) unique.set(`${t.x},${t.y}`, t);
      const orderedTiles = Array.from(unique.values()).sort((a, b) => (a.y - b.y) || (a.x - b.x));

      const job: MapExportJob = {
        id: randomUUID(),
        request: {
          mapDir: entry.dir,
          mapID: entry.id,
          body: parsed.data,
          orderedTiles,
        },
        status: 'pending',
        submittedAt: Date.now(),
        noTimeout: true,
      };

      mapExportQueue.addJob(job);
      console.log('export-adt queued', key, job.id, `${orderedTiles.length} tiles`);

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

  router.get('/maps/export-adt/status/:jobId', (req, res) => {
    void (async () => {
      const status = await buildJobStatus(req.params.jobId);
      if (!status) {
        res.status(404).json({ error: 'Export job not found' });
        return;
      }
      res.json(status);
    })();
  });
}

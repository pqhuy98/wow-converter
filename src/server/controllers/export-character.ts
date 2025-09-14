import chalk from 'chalk';
import { createHash, randomUUID } from 'crypto';
import express from 'express';
import fsExtra from 'fs-extra';
import _ from 'lodash';
import path from 'path';
import z from 'zod';

import {
  CharacterExporter, CharacterSchema, LocalRefSchema, LocalRefValueSchema,
} from '@/lib/converter/character';
import { Config, getDefaultConfig } from '@/lib/global-config';
import { stableStringify, waitUntil } from '@/lib/utils';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { Job, JobQueue, QueueConfig } from '@/server/utils/job-queue';

import {
  isSharedHosting, outputDir, outputDirBrowse, serverDeployTime,
} from '../config';
import { startupRequests } from './export-character.startup';

export const ExporCharacterRequestSchema = z.object({
  character: CharacterSchema,
  outputFileName: LocalRefValueSchema,
  optimization: z.object({
    sortSequences: z.boolean().optional(),
    removeUnusedVertices: z.boolean().optional(),
    removeUnusedNodes: z.boolean().optional(),
    removeUnusedMaterialsTextures: z.boolean().optional(),
    maxTextureSize: z.enum(['256', '512', '1024']).optional(),
  }),
  format: z.enum(['mdx', 'mdl']),
  formatVersion: z.enum(['800', '1000']).optional(),
  isBrowse: z.boolean().optional(),
});

export type ExportCharacterRequest = z.infer<typeof ExporCharacterRequestSchema>;

export type ExportCharacterResponse = {
  exportedModels: { path: string, size: number }[]
  exportedTextures: { path: string, size: number }[]
  modelStats: {
    formatVersion: number;
    vertices: number;
    faces: number;
    globalSequences: number;
    sequences: number;
    geosets: number;
    geosetAnims: number;
    materials: number;
    textures: number;
    textureAnims: number;
    bones: number;
    lights: number;
    ribbonEmitters: number;
    particles: number;
    attachments: number;
    eventObjects: number;
    helpers: number;
    collisionShapes: number;
    cameras: number;
  }
  outputDirectory?: string
  versionId: string
}

const queueConfig: QueueConfig<ExportCharacterRequest, ExportCharacterResponse> = {
  concurrency: 1,
  maxPendingJobs: 100,
  jobTTL: 5 * 60 * 1000,
  jobTimeout: 60 * 1000 + (isSharedHosting ? 60 * 1000 : 0),
};

type ExportCharacterJob = Job<ExportCharacterRequest, ExportCharacterResponse>;

let ceConfig: Config;

export function getCeConfig() {
  return ceConfig;
}

let logs: string[] = [];
const originalConsoleLog = console.log;
console.log = (...args) => {
  logs.push(args.join(' '));
  if (logs.length > 2) logs.shift();
  originalConsoleLog(...args);
};

export async function ControllerExportCharacter(router: express.Router) {
  await waitUntil(() => wowExportClient.isReady);
  ceConfig = await getDefaultConfig();
  if (isSharedHosting) {
    console.log('Shared hosting mode enabled');
  }

  /** Core export logic, extracted into its own function so the queue worker can reuse it */
  async function handleExport(job: ExportCharacterJob) {
    const start = performance.now();
    const request = job.request;
    const ce = new CharacterExporter({
      ...ceConfig,
      maxTextureSize: request.optimization.maxTextureSize
        ? parseInt(request.optimization.maxTextureSize, 10)
        : undefined,
    });
    console.log(`Start exporting ${request.outputFileName}: ${chalk.gray(JSON.stringify(request, null, 2))}`);
    logs = [];

    await wowExportClient.syncConfig();
    await ce.exportCharacter(request.character, request.outputFileName);

    ce.models.forEach(([mdl]) => {
      if (request.formatVersion === '800') {
        mdl.modify.convertToSd800();
        mdl.materials.forEach((m) => {
          m.layers.forEach((l) => {
            l.unshaded = true;
          });
        });
      }
      if (request.optimization?.sortSequences) {
        mdl.modify.sortSequences();
      }
      if (request.optimization?.removeUnusedVertices) {
        mdl.modify.removeUnusedVertices();
      }
      const particlesDensity = request.character.particlesDensity;
      if (particlesDensity != null) {
        if (particlesDensity > 0 && particlesDensity !== 1) {
          mdl.modify.scaleParticlesDensity(particlesDensity);
        } else if (particlesDensity === 0) {
          mdl.particleEmitter2s = [];
        }
      }
      if (request.optimization?.removeUnusedNodes) {
        mdl.modify.removeUnusedNodes();
      }
      if (request.optimization?.removeUnusedMaterialsTextures) {
        mdl.modify.removeUnusedMaterialsTextures();
      }
      mdl.modify.optimizeKeyFrames();
      mdl.sync();
    });

    const targetDir = request.isBrowse ? outputDirBrowse : outputDir;
    const modelPaths = ce.writeAllModels(targetDir, request.format);
    const texturePaths = await ce.writeAllTextures(targetDir);

    const exportedModels = modelPaths.map((modelPath) => ({
      path: path.relative(targetDir, `${modelPath}.${request.format}`),
      size: fsExtra.statSync(`${modelPath}.${request.format}`).size,
    }));
    const exportedTextures = texturePaths.map((texturePath) => ({
      path: path.relative(targetDir, texturePath),
      size: fsExtra.statSync(texturePath).size,
    }));
    exportedTextures.sort((a, b) => a.path.localeCompare(b.path));

    // Return the list of exported assets to the caller – zipping happens on-demand via the download API
    const resp: ExportCharacterResponse = {
      exportedModels,
      exportedTextures,
      outputDirectory: !isSharedHosting ? path.resolve(targetDir) : undefined,
      versionId: job.id,
      modelStats: {
        formatVersion: request.formatVersion === '800' ? 800 : 1000,
        globalSequences: ce.models.reduce((acc, [mdl]) => acc + mdl.globalSequences.length, 0),
        sequences: ce.models.reduce((acc, [mdl]) => acc + mdl.sequences.length, 0),
        geosets: ce.models.reduce((acc, [mdl]) => acc + mdl.geosets.length, 0),
        geosetAnims: ce.models.reduce((acc, [mdl]) => acc + mdl.geosetAnims.length, 0),
        materials: ce.models.reduce((acc, [mdl]) => acc + mdl.materials.length, 0),
        textures: ce.models.reduce((acc, [mdl]) => acc + mdl.textures.length, 0),
        textureAnims: ce.models.reduce((acc, [mdl]) => acc + mdl.textureAnims.length, 0),
        bones: ce.models.reduce((acc, [mdl]) => acc + mdl.bones.length, 0),
        lights: ce.models.reduce((acc, [mdl]) => acc + mdl.lights.length, 0),
        ribbonEmitters: ce.models.reduce((acc, [mdl]) => acc + mdl.ribbonEmitters.length, 0),
        particles: ce.models.reduce((acc, [mdl]) => acc + mdl.particleEmitter2s.length, 0),
        attachments: ce.models.reduce((acc, [mdl]) => acc + mdl.attachments.length, 0),
        eventObjects: ce.models.reduce((acc, [mdl]) => acc + mdl.eventObjects.length, 0),
        helpers: ce.models.reduce((acc, [mdl]) => acc + mdl.helpers.length, 0),
        collisionShapes: ce.models.reduce((acc, [mdl]) => acc + mdl.collisionShapes.length, 0),
        cameras: ce.models.reduce((acc, [mdl]) => acc + mdl.cameras.length, 0),
        vertices: ce.models.reduce((acc, [mdl]) => acc + mdl.geosets.reduce((acc, g) => acc + g.vertices.length, 0), 0),
        faces: ce.models.reduce((acc, [mdl]) => acc + mdl.geosets.reduce((acc, g) => acc + g.faces.length, 0), 0),
      },
    };

    console.log(
      'Job finished',
      `${chalk.yellow(`${((performance.now() - start) / 1000).toFixed(2)}s`)}`,
      chalk.gray(JSON.stringify(_.omit(resp, 'exportedTextures', 'modelStats'), null, 2)),
    );

    return resp;
  }

  const jobQueue = new JobQueue<ExportCharacterRequest, ExportCharacterResponse>(
    {
      ...queueConfig,
      jobCompletedCallback: () => {
        fsExtra.writeFileSync('recent-exports.json', JSON.stringify(jobQueue.recentCompletedJobs, null, 2));
      },
    },
    (job) => handleExport(job),
  );

  // Load recent exports from file so that it survives server restart
  try {
    const recentExports = JSON.parse(fsExtra.readFileSync('recent-exports.json', 'utf8')) as ExportCharacterJob[];
    jobQueue.recentCompletedJobs = recentExports;
  } catch (err) {
    // Ignore
  }

  router.get('/export/character/recent', (req, res) => {
    res.json(jobQueue.recentCompletedJobs);
  });

  router.post('/export/character', (req, res) => {
    if (!wowExportClient.isReady) {
      return res.status(500).json({ error: 'wow.export RCP server is not ready' });
    }

    try {
      const parsedRequest = ExporCharacterRequestSchema.parse(req.body);

      // Use a unique version suffix for the model file names to prevent collisions in multi-tenant scenarios
      // while keeping all assets in the shared exported-assets folder (avoids duplicating textures).
      // Deterministic version hash based on request content + server epoch when in shared hosting mode.
      const versionId = isSharedHosting || parsedRequest.isBrowse
        ? createHash('md5').update(stableStringify(parsedRequest) + serverDeployTime).digest('hex')
        : undefined;
      if (versionId) {
        parsedRequest.outputFileName = `${parsedRequest.outputFileName}__${versionId}`;
      }

      if (versionId && jobQueue.getJob(versionId)) {
        return res.json(jobQueue.getJobStatus(versionId));
      }

      const job: ExportCharacterJob = {
        id: versionId ?? randomUUID(),
        request: parsedRequest,
        status: 'pending',
        submittedAt: Date.now(),
      };

      jobQueue.addJob(job);
      console.log(chalk.blue(req.method), req.path, chalk.gray(`Queued job ${job.id}`));

      setImmediate(() => {
        res.json(jobQueue.getJobStatus(job.id));
      });
      return res;
    } catch (err) {
      return res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
    }
  });

  router.get('/export/character/status/:jobId', (req, res) => {
    const { jobId } = req.params;
    const job = jobQueue.getJob(jobId);

    if (!job) {
      return res.status(404).json({ error: 'Export request not found' });
    }

    const status = jobQueue.getJobStatus(jobId);

    return res.json({
      ...status,
      logs: status?.status === 'processing' ? logs : undefined,
    });
  });

  router.post('/export/character/clean', (req, res) => {
    if (outputDir !== 'exported-assets') {
      return res.status(500).json({ error: 'Cannot clean exported assets because output directory is not "exported-assets"' });
    }
    if (isSharedHosting) {
      return res.status(400).json({ error: 'Cannot clean exported assets because server is in shared hosting mode' });
    }

    fsExtra.removeSync('./recent-exports.json');
    jobQueue.recentCompletedJobs = [];
    console.log(`Removed ${path.resolve('./recent-exports.json')}`);
    fsExtra.emptyDirSync(outputDir);
    console.log(`Cleared ${path.resolve(outputDir)}`);
    fsExtra.emptyDirSync(outputDirBrowse);
    console.log(`Cleared ${path.resolve(outputDirBrowse)}`);
    wowExportClient.clearCacheFiles();
    console.log(`Cleared ${path.resolve('.cache')}`);
    return res.json({ message: 'Exported assets cleaned' });
  });

  // Serve exported assets. When running on shared hosting enable HTTP caching to reduce bandwidth
  const exportStatic = (dir: string) => (isSharedHosting
    ? express.static(dir, {
      maxAge: '1h',
      setHeaders: (res) => {
        res.setHeader('Cache-Control', 'public, max-age=3600');
      },
    })
    : express.static(dir));
  router.use('/browse-assets', exportStatic(outputDirBrowse));
  router.use('/assets', exportStatic(outputDir));
  fsExtra.emptyDirSync(outputDirBrowse);

  let jobs: ExportCharacterJob[] = [];
  if (isSharedHosting) {
    jobs = startupRequests.map((request) => {
      const job: ExportCharacterJob = {
        id: randomUUID(),
        request,
        status: 'pending',
        submittedAt: Date.now(),
        isDemo: true,
      };
      jobQueue.addJob(job);
      return job;
    });
  }

  router.get('/export/character/demos', (req, res) => {
    res.json(jobs.map((job) => jobQueue.getJob(job.id)).filter((job) => job?.status === 'done'));
  });

  router.get('/export/character/check-local-file', (req, res) => {
    try {
      const { localPath } = req.query as { localPath: string };
      const parsed = LocalRefSchema.safeParse({ type: 'local', value: localPath });
      if (!parsed.success) {
        return res.json({
          ok: false,
          similarFiles: [],
        });
      }

      let baseModelPath = path.resolve(ceConfig.wowExportAssetDir, localPath);
      // prevent traversal attacks
      if (!baseModelPath.startsWith(ceConfig.wowExportAssetDir)) {
        return res.json({
          ok: false,
          similarFiles: [],
        });
      }
      if (!baseModelPath.endsWith('.obj')) {
        baseModelPath += '.obj';
      }
      const searchPrefix = baseModelPath.replace('.obj', '');
      const dirName = path.dirname(baseModelPath);
      if (!fsExtra.existsSync(dirName)) {
        return res.json({
          ok: false,
          similarFiles: [],
        });
      }
      const allFiles = fsExtra.readdirSync(dirName).map((file) => path.join(dirName, file));
      const similarFiles = allFiles
        .filter((file) => file.startsWith(searchPrefix) && file.endsWith('.obj') && !file.endsWith('.phys.obj'))
        .map((file) => path.relative(ceConfig.wowExportAssetDir, file));

      return res.json({
        ok: fsExtra.existsSync(baseModelPath),
        similarFiles,
      });
    } catch (err) {
      return res.json({
        ok: false,
        similarFiles: [],
      });
    }
  });
}

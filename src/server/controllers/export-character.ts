import chalk from 'chalk';
import { createHash, randomUUID } from 'crypto';
import express from 'express';
import fsExtra from 'fs-extra';
import path from 'path';
import z from 'zod';

import {
  CharacterExporter, CharacterSchema, LocalRefSchema, LocalRefValueSchema,
} from '@/lib/converter/character';
import { Config, getDefaultConfig } from '@/lib/global-config';
import { stableStringify, waitUntil } from '@/lib/utils';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { Job, JobQueue, QueueConfig } from '@/server/utils/job-queue';

import { isSharedHosting, outputDir, serverDeployTime } from '../config';
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
});

export type ExportCharacterRequest = z.infer<typeof ExporCharacterRequestSchema>;

export type ExportCharacterResponse = {
  exportedModels: string[]
  exportedTextures: string[]
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

export async function ControllerExportCharacter(app: express.Application) {
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

    await wowExportClient.syncConfig();
    await ce.exportCharacter(request.character, request.outputFileName);

    ce.models.forEach(([mdl]) => {
      if (request.formatVersion === '800') {
        mdl.modify.convertToSd800();
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

    let exportedModels = ce.writeAllModels(outputDir, request.format);
    let textures = await ce.writeAllTextures(outputDir);

    exportedModels = exportedModels.map((model) => path.relative(outputDir, `${model}.${request.format}`));
    textures = textures.map((texture) => path.relative(outputDir, texture));

    // Return the list of exported assets to the caller – zipping happens on-demand via the download API
    const resp: ExportCharacterResponse = {
      exportedModels,
      exportedTextures: textures,
      outputDirectory: !isSharedHosting ? path.resolve(outputDir) : undefined,
      versionId: job.id,
    };

    console.log(
      'Job finished',
      `${chalk.yellow(`${((performance.now() - start) / 1000).toFixed(2)}s`)}`,
      chalk.gray(JSON.stringify(resp, null, 2)),
    );
    return resp;
  }

  queueConfig.jobCompletedCallback = () => {
    fsExtra.writeFileSync('recent-exports.json', JSON.stringify(jobQueue.recentCompletedJobs, null, 2));
  };

  const jobQueue = new JobQueue<ExportCharacterRequest, ExportCharacterResponse>(
    queueConfig,
    (job) => handleExport(job),
  );

  // Load recent exports from file so that it survives server restart
  try {
    const recentExports = JSON.parse(fsExtra.readFileSync('recent-exports.json', 'utf8')) as ExportCharacterJob[];
    jobQueue.recentCompletedJobs = recentExports;
  } catch (err) {
    // Ignore
  }

  app.get('/export/character/recent', (req, res) => {
    res.json(jobQueue.recentCompletedJobs);
  });

  app.post('/export/character', (req, res) => {
    if (!wowExportClient.isReady) {
      return res.status(500).json({ error: 'wow.export RCP server is not ready' });
    }

    try {
      const parsedRequest = ExporCharacterRequestSchema.parse(req.body);

      // Use a unique version suffix for the model file names to prevent collisions in multi-tenant scenarios
      // while keeping all assets in the shared exported-assets folder (avoids duplicating textures).
      // Deterministic version hash based on request content + server epoch when in shared hosting mode.
      const versionId = isSharedHosting
        ? createHash('md5').update(stableStringify(parsedRequest)).update(serverDeployTime).digest('hex')
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

  app.get('/export/character/status/:jobId', (req, res) => {
    const { jobId } = req.params;
    const job = jobQueue.getJob(jobId);

    if (!job) {
      return res.status(404).json({ error: 'Export request not found' });
    }

    return res.json(jobQueue.getJobStatus(jobId));
  });

  // Serve exported assets. When running on shared hosting enable HTTP caching to reduce bandwidth
  app.use('/assets', isSharedHosting
    ? express.static(outputDir, {
      maxAge: '1h',
      setHeaders: (res) => {
        // Explicitly set the Cache-Control header because some CDNs ignore the implicit one
        // generated by Express when maxAge is provided.
        res.setHeader('Cache-Control', 'public, max-age=3600');
      },
    })
    // When in personal mode we want always fresh content
    : express.static(outputDir));

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

  app.get('/export/character/demos', (req, res) => {
    res.json(jobs.map((job) => jobQueue.getJob(job.id)).filter((job) => job?.status === 'done'));
  });

  app.get('/export/character/config', (req, res) => {
    res.json({
      wowExportAssetDir: ceConfig.wowExportAssetDir,
      isSharedHosting,
    });
  });

  app.get('/export/character/check-local-file', (req, res) => {
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

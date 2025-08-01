import chalk from 'chalk';
import { createHash, randomUUID } from 'crypto';
import express from 'express';
import fsExtra from 'fs-extra';
import path from 'path';
import z from 'zod';

import { CharacterExporter, CharacterSchema, LocalRefValueSchema } from '@/lib/converter/character-exporter';
import { getDefaultConfig } from '@/lib/global-config';
import { stableStringify, waitUntil } from '@/lib/utils';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { Job, JobQueue, QueueConfig } from '@/server/utils/job-queue';

import { isSharedHosting, serverDeployTime } from '../config';
import { startupRequests } from './export-character.startup';

export const ExporCharacterRequestSchema = z.object({
  character: CharacterSchema,
  outputFileName: LocalRefValueSchema,
  optimization: z.object({
    sortSequences: z.boolean().optional(),
    removeUnusedVertices: z.boolean().optional(),
    removeUnusedNodes: z.boolean().optional(),
    removeUnusedMaterialsTextures: z.boolean().optional(),
  }),
  format: z.enum(['mdx', 'mdl']),
});

export type ExportCharacterRequest = z.infer<typeof ExporCharacterRequestSchema>;

export type ExportCharacterResponse = {
  exportedModels: string[]
  exportedTextures: string[]
  outputDirectory: string
  versionId: string
}

const queueConfig: QueueConfig = {
  concurrency: 1,
  maxPendingJobs: 100,
  jobTTL: 5 * 60 * 1000,
  jobTimeout: 60 * 1000,
};

type ExportCharacterJob = Job<ExportCharacterRequest, ExportCharacterResponse>;

export async function ControllerExportCharacter(app: express.Application) {
  await waitUntil(() => wowExportClient.isReady);
  const ceOutputPath = 'exported-assets';
  const ceConfig = await getDefaultConfig();

  fsExtra.ensureDirSync(ceOutputPath);
  if (isSharedHosting) {
    console.log('Shared hosting mode enabled');
  }

  /** Core export logic, extracted into its own function so the queue worker can reuse it */
  async function handleExport(job: ExportCharacterJob) {
    const request = job.request;
    const ce = new CharacterExporter(ceOutputPath, ceConfig);
    console.log(`Processing job for ${request.outputFileName}: ${chalk.gray(JSON.stringify(request, null, 2))}`);

    await ce.exportCharacter(request.character, request.outputFileName);

    let exportedModels: string[] = [];
    ce.models.forEach(([mdl, filePath]) => {
      if (request.optimization?.sortSequences) {
        mdl.modify.sortSequences();
      }
      if (request.optimization?.removeUnusedVertices) {
        mdl.modify.removeUnusedVertices();
      }
      if (request.optimization?.removeUnusedNodes) {
        mdl.modify.removeUnusedNodes();
      }
      if (request.optimization?.removeUnusedMaterialsTextures) {
        mdl.modify.removeUnusedMaterialsTextures();
      }
      mdl.modify.optimizeKeyFrames();
      mdl.sync();
      fsExtra.ensureDirSync(path.dirname(filePath));
      if (request.format === 'mdx') {
        fsExtra.writeFileSync(`${filePath}.mdx`, mdl.toMdx());
        exportedModels.push(`${filePath}.mdx`);
      } else {
        fsExtra.writeFileSync(`${filePath}.mdl`, mdl.toMdl());
        exportedModels.push(`${filePath}.mdl`);
      }
    });

    ce.assetManager.purgeTextures(ce.models.flatMap(([m]) => m.textures.map((t) => t.image)));
    let textures = await ce.assetManager.exportTextures(ce.outputPath);

    exportedModels = exportedModels.map((model) => path.relative(ceOutputPath, model));
    textures = textures.map((texture) => path.relative(ceOutputPath, texture));

    // Return the list of exported assets to the caller – zipping happens on-demand via the download API
    const resp: ExportCharacterResponse = {
      exportedModels,
      exportedTextures: textures,
      outputDirectory: path.resolve(ce.outputPath),
      versionId: job.id,
    };

    console.log('Job finished:', chalk.gray(JSON.stringify(resp, null, 2)));
    return resp;
  }

  const jobQueue = new JobQueue<ExportCharacterRequest, ExportCharacterResponse>(
    queueConfig,
    (job) => handleExport(job),
  );

  app.get('/export/character/recent', (req, res) => {
    let recentJobs = jobQueue.recentJobs;

    // if there are jobs with same outputFileName, remove all but the last one, O(N)
    const outputFileNames = new Set<string>();
    recentJobs.reverse();
    recentJobs = recentJobs.filter((job) => {
      if (outputFileNames.has(job.request.outputFileName)) {
        return false;
      }
      outputFileNames.add(job.request.outputFileName);
      return true;
    });

    res.json(recentJobs);
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

  app.use('/assets', express.static(ceOutputPath));

  let jobs: ExportCharacterJob[] = [];
  if (isSharedHosting) {
    // remove all models file with version suffix
    fsExtra.readdirSync(ceOutputPath).forEach((file) => {
      if (/__\w{32}\.(mdx|mdl)$/.test(file)) {
        fsExtra.removeSync(path.join(ceOutputPath, file));
      }
    });

    jobs = startupRequests.map((request) => {
      const job: ExportCharacterJob = {
        id: randomUUID(),
        request,
        status: 'pending',
        submittedAt: Date.now(),
      };
      jobQueue.addJob(job);
      return job;
    });
  }

  app.get('/export/character/startup', (req, res) => {
    res.json(jobs.map((job) => jobQueue.getJob(job.id)).filter((job) => job?.status === 'done'));
  });
}

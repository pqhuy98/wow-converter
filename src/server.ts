import archiver from 'archiver';
import chalk from 'chalk';
import cors from 'cors';
import { randomUUID } from 'crypto';
import express from 'express';
import fsExtra from 'fs-extra';
import path from 'path';
import z from 'zod';

import { CharacterExporter, CharacterSchema, LocalRefValueSchema } from './lib/converter/character-exporter';
import { getDefaultConfig } from './lib/global-config';
import { printLogo } from './lib/logo';
import { waitUntil } from './lib/utils';
import { wowExportClient } from './lib/wowexport-client/wowexport-client';

async function main() {
  printLogo();
  await waitUntil(() => wowExportClient.isReady);

  const app = express();
  app.use(cors());
  app.use(express.json());

  const ceOutputPath = 'exported-assets';
  const ceConfig = await getDefaultConfig();
  fsExtra.ensureDirSync(ceOutputPath);

  /**
   * Schema for incoming export requests
   */
  /**
   * Export character
   */
  const ExporCharacterRequestSchema = z.object({
    character: CharacterSchema,
    outputFileName: LocalRefValueSchema,
    optimization: z.object({
      sortSequences: z.boolean().optional(),
      removeUnusedVertices: z.boolean().optional(),
      removeUnusedNodes: z.boolean().optional(),
      removeUnusedMaterialsTextures: z.boolean().optional(),
    }).optional(),
    format: z.enum(['mdx', 'mdl']).optional(),
  });

  // ------------------------------------------------------------
  // Queue implementation
  // ------------------------------------------------------------

  type ExportCharacterRequest = z.infer<typeof ExporCharacterRequestSchema>;

  type JobStatus = 'pending' | 'processing' | 'done' | 'failed';

  interface Job {
    id: string;
    request: ExportCharacterRequest;
    status: JobStatus;
    result?: unknown;
    error?: string;
    finishedAt?: number; // timestamp when job reached a terminal state
  }

  /**
   * Optimised queue data-structures
   */
  const pendingQueue: Job[] = []; // FIFO queue of pending jobs
  let queueHead = 0; // index of the next job to process – avoids costly array.shift()
  const pendingIndexMap = new Map<string, number>(); // jobId → index in pendingQueue (O(1) position look-up)

  const jobsMap = new Map<string, Job>(); // jobId → Job (covers all statuses; O(1) access)

  // Concurrency (defaults to 1 → sequential processing). Overridable via env.
  const QUEUE_CONCURRENCY = Number(process.env.EXPORT_QUEUE_CONCURRENCY || 1);
  const MAX_PENDING_JOBS = Number(process.env.EXPORT_MAX_PENDING || 100);
  const JOB_TTL_MS = Number(process.env.EXPORT_JOB_TTL_MS || 5 * 60 * 1000); // default 5 min
  let activeJobs = 0;

  /** Core export logic, extracted into its own function so the queue worker can reuse it */
  async function handleExport(request: ExportCharacterRequest) {
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
      if (request.format === 'mdx' || !request.format) {
        fsExtra.ensureDirSync(path.dirname(filePath));
        fsExtra.writeFileSync(`${filePath}.mdx`, mdl.toMdx());
        exportedModels.push(`${filePath}.mdx`);
      } else {
        fsExtra.writeFileSync(`${filePath}.mdl`, mdl.toString());
        exportedModels.push(`${filePath}.mdl`);
      }
    });

    ce.assetManager.purgeTextures(ce.models.flatMap(([m]) => m.textures.map((t) => t.image)));
    let textures = await ce.assetManager.exportTextures(ce.outputPath);

    exportedModels = exportedModels.map((model) => path.relative(ce.outputPath, model));
    textures = textures.map((texture) => path.relative(ce.outputPath, texture));

    // Create a zip archive that contains every exported model & texture
    const randomSuffix = Math.random().toString(36).slice(2, 8);
    const zipFileName = `${request.outputFileName}-${randomSuffix}.zip`;
    const zipFilePath = path.join(ce.outputPath, zipFileName);

    const output = fsExtra.createWriteStream(zipFilePath);
    const archive = archiver('zip', { zlib: { level: 9 } });

    archive.pipe(output);

    [...exportedModels, ...textures].forEach((relativePath) => {
      archive.file(path.join(ce.outputPath, relativePath), { name: relativePath });
    });

    await new Promise<void>((resolve, reject) => {
      output.on('close', () => resolve());
      archive.on('error', (err) => reject(err));
      archive.finalize();
    });

    const resp = {
      exportedModels,
      exportedTextures: textures,
      zipFile: zipFileName,
      zipFileSize: fsExtra.statSync(zipFilePath).size,
      outputDirectory: path.resolve(ce.outputPath),
    };

    console.log('Job finished:', chalk.gray(JSON.stringify(resp, null, 2)));
    return resp;
  }

  /**
   * Queue worker – processes as many jobs as allowed by the concurrency limit.
   * Uses queueHead pointer to dequeue without costly array mutations.
   */
  function tryProcessQueue(): void {
    while (activeJobs < QUEUE_CONCURRENCY && queueHead < pendingQueue.length) {
      const job = pendingQueue[queueHead];
      queueHead++;
      pendingIndexMap.delete(job.id);

      activeJobs++;
      job.status = 'processing';

      void (async () => {
        try {
          job.result = await handleExport(job.request);
          job.status = 'done';
          job.finishedAt = Date.now();
        } catch (err) {
          job.status = 'failed';
          job.error = err instanceof Error ? err.message : String(err);
          job.finishedAt = Date.now();
          console.error(err);
        } finally {
          activeJobs--;
          // Process further jobs if capacity is available
          tryProcessQueue();
        }
      })();
    }
  }

  app.post('/export/character', (req, res) => {
    if (!wowExportClient.isReady) {
      return res.status(500).json({ error: 'wow.export RCP server is not ready' });
    }

    try {
      const parsedRequest = ExporCharacterRequestSchema.parse(req.body);

      if (pendingIndexMap.size >= MAX_PENDING_JOBS) {
        return res.status(429).json({ error: `There are already ${MAX_PENDING_JOBS} pending export requests. Please try again later.` });
      }

      const jobId = randomUUID();

      const job: Job = { id: jobId, request: parsedRequest, status: 'pending' };
      jobsMap.set(jobId, job);

      pendingQueue.push(job);
      pendingIndexMap.set(jobId, pendingQueue.length - 1);

      console.log(chalk.blue(req.method), req.path, chalk.gray(`Queued job ${jobId}`));

      tryProcessQueue();

      return res.json({ jobId });
    } catch (err) {
      return res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
    }
  });

  /**
   * Job status endpoint – returns status & queue position, and the final result once done.
   */
  app.get('/export/character/status/:jobId', (req, res) => {
    const { jobId } = req.params;
    const job = jobsMap.get(jobId);

    if (!job) {
      return res.status(404).json({ error: 'Export request not found' });
    }

    if (job.status === 'done') {
      return res.json({ status: 'done', result: job.result });
    }

    if (job.status === 'failed') {
      return res.json({ status: 'failed', error: job.error });
    }

    // pending or processing – compute position without scanning entire queue
    if (job.status === 'pending') {
      const index = pendingIndexMap.get(jobId)!;
      return res.json({ status: 'pending', position: index - queueHead + 1 });
    }

    // processing
    return res.json({ status: 'processing', position: 0 });
  });

  // ------------------------------------------------------------
  // Cleanup of old finished jobs
  // ------------------------------------------------------------

  setInterval(() => {
    const now = Date.now();
    for (const [id, job] of jobsMap) {
      if ((job.status === 'done' || job.status === 'failed') && job.finishedAt && now - job.finishedAt > JOB_TTL_MS) {
        jobsMap.delete(id);
      }
    }
  }, 60_000); // run every minute

  /**
   * Web server
   */
  const port = 3001;

  // serve the static UI
  const uiDir = path.join('webui', 'out');
  if (fsExtra.existsSync(uiDir)) {
    console.log(`Serving UI web interface at ${chalk.blue(`http://127.0.0.1:${port}/`)}`);
    app.use(express.static(uiDir));
    app.get('/', (_, res) => res.sendFile(path.join(uiDir, 'index.html')));
  } else {
    console.log(`No UI found, serving only REST API at ${chalk.blue(`http://127.0.0.1:${port}/`)}`);
  }

  // Download endpoint for zipped assets – only serves .zip files located in the export directory
  app.get('/download/:fileName', (req, res) => {
    const { fileName } = req.params;

    // Basic validation – filename must be alphanumeric/underscore/dash and end with .zip
    if (!/^[\w-]+\.zip$/.test(fileName)) {
      return res.status(400).json({ error: 'Invalid file name' });
    }

    const resolvedPath = path.resolve(ceOutputPath, fileName);

    // Prevent directory-traversal attacks – path must stay inside ceOutputPath
    if (!resolvedPath.startsWith(path.resolve(ceOutputPath))) {
      return res.status(400).json({ error: 'Invalid path' });
    }

    if (!fsExtra.existsSync(resolvedPath)) {
      return res.status(404).json({ error: 'File not found' });
    }

    return res.download(resolvedPath);
  });

  // Error-handling middleware (must be **after** all routes)
  app.use((err: Error, _req: express.Request, res: express.Response, _next: express.NextFunction) => {
    console.error(err);
    res.status(500).json({ error: err.message ?? err });
  });

  app.listen(port, () => {
    // void open(`http://127.0.0.1:${port}`);
  });
}
void main();

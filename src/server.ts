import archiver from 'archiver';
import chalk from 'chalk';
import cors from 'cors';
import express from 'express';
import fsExtra from 'fs-extra';
import path from 'path';
import z from 'zod';

import { CharacterExporter, CharacterSchema, LocalRefValueSchema } from './lib/converter/character';
import { Config } from './lib/converter/common';
import { defaultConfig } from './lib/global-config';
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
  const ceConfig: Config = {
    ...defaultConfig,
    assetPrefix: 'wow',
    rawModelScaleUp: defaultConfig.rawModelScaleUp * 2,
  };
  fsExtra.ensureDirSync(ceOutputPath);

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

  app.post('/export/character', async (req: express.Request, res: express.Response) => {
    if (!wowExportClient.isReady) {
      res.status(500).json({ error: 'wow.export RCP server is not ready' });
    }

    const ce = new CharacterExporter(ceOutputPath, ceConfig);
    console.log(chalk.blue(req.method), req.path, chalk.gray(JSON.stringify(req.body, null, 2)));
    const request = ExporCharacterRequestSchema.parse(req.body);

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
    let textures = ce.assetManager.exportTextures(ce.outputPath);

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
      outputDirectory: path.resolve(ce.outputPath),
    };
    console.log('Response:', chalk.gray(JSON.stringify(resp, null, 2)));
    res.json(resp);
  });

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

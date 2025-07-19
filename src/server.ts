import chalk from 'chalk';
import cors from 'cors';
import express from 'express';
import fsExtra from 'fs-extra';
import path from 'path';
import z from 'zod';

import { CharacterExporter, CharacterSchema } from './lib/converter/character';
import { Config } from './lib/converter/common';
import { defaultConfig } from './lib/global-config';

console.log(`
 ██╗  ██╗ ██╗   ██╗ ██╗   ██╗ █╗  ███████╗
 ██║  ██║ ██║   ██║ ╚██╗ ██╔╝ ╚╝  ██╔════╝
 ███████║ ██║   ██║  ╚████╔╝      ███████╗
 ██╔══██║ ██║   ██║   ╚██╔╝       ╚════██║
 ██║  ██║ ╚██████╔╝    ██║        ███████║
 ╚═╝  ╚═╝  ╚═════╝     ╚═╝        ╚══════╝

 ██╗    ██╗  ██████╗  ██╗    ██╗         ██████╗  ██████╗  ███╗   ██╗ ██╗   ██╗ ███████╗ ██████╗  ████████╗ ███████╗ ██████╗
 ██║    ██║ ██╔═══██╗ ██║    ██║        ██╔════╝ ██╔═══██╗ ████╗  ██║ ██║   ██║ ██╔════╝ ██╔══██╗ ╚══██╔══╝ ██╔════╝ ██╔══██╗
 ██║ █╗ ██║ ██║   ██║ ██║ █╗ ██║ █████╗ ██║      ██║   ██║ ██╔██╗ ██║ ██║   ██║ █████╗   ██████╔╝    ██║    █████╗   ██████╔╝
 ██║███╗██║ ██║   ██║ ██║███╗██║ ╚════╝ ██║      ██║   ██║ ██║╚██╗██║ ╚██╗ ██╔╝ ██╔══╝   ██╔══██╗    ██║    ██╔══╝   ██╔══██╗
 ╚███╔███╔╝ ╚██████╔╝ ╚███╔███╔╝        ╚██████╗ ╚██████╔╝ ██║ ╚████║  ╚████╔╝  ███████╗ ██║  ██║    ██║    ███████╗ ██║  ██║
  ╚══╝╚══╝   ╚═════╝   ╚══╝╚══╝          ╚═════╝  ╚═════╝  ╚═╝  ╚═══╝   ╚═══╝   ╚══════╝ ╚═╝  ╚═╝    ╚═╝    ╚══════╝ ╚═╝  ╚═

`);

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

const ExporCharacterRequestSchema = z.object({
  character: CharacterSchema,
  outputFileName: z.string(),
  optimization: z.object({
    sortSequences: z.boolean().optional(),
    removeUnusedVertices: z.boolean().optional(),
    removeUnusedNodes: z.boolean().optional(),
    removeUnusedMaterialsTextures: z.boolean().optional(),
  }).optional(),
  format: z.enum(['mdx', 'mdl']).optional(),
});

app.post('/export/character', async (req: express.Request, res: express.Response) => {
  const ce = new CharacterExporter(ceOutputPath, ceConfig);
  console.log(req.body);
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

  res.json({
    exportedModels,
    exportedTextures: textures,
    outputDirectory: path.resolve(ce.outputPath),
  });
});

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
// Error-handling middleware (must be **after** all routes)
app.use((err: Error, _req: express.Request, res: express.Response, _next: express.NextFunction) => {
  console.error(err);
  res.status(500).json({ error: err.message ?? err });
});

app.listen(port);

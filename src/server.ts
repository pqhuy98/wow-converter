import express from 'express';
import { CharacterExporter, CharacterSchema } from './lib/converter/character';
import { defaultConfig } from './lib/global-config';
import { Config } from './lib/converter/common';
import { ensureDirSync, writeFileSync } from 'fs-extra';
import z from 'zod';


const ceOutputPath = 'dist/exported-assets';
const ceConfig: Config = {
  ...defaultConfig,
  assetPrefix: 'wow',
  rawModelScaleUp: defaultConfig.rawModelScaleUp * 2,
};
ensureDirSync(ceOutputPath);

const app = express();

const ExporCharacterRequestSchema = z.object({
  character: CharacterSchema,
  outputFileName: z.string(),
  optimization: z.object({
    sortSequences: z.boolean().optional(),
    removeUnusedVertices: z.boolean().optional(),
    removeUnusedNodes: z.boolean().optional(),
    removeUnusedMaterialsTextures: z.boolean().optional(),
    removeCinematicSequences: z.boolean().optional(),
  }).optional(),
  format: z.enum(['mdx', 'mdl']).optional(),
});

app.post('/export/character', async (req: express.Request, res: express.Response) => {
  const ce = new CharacterExporter(ceOutputPath, ceConfig);
  const request = ExporCharacterRequestSchema.parse(req.body);

  await ce.exportCharacter(request.character, request.outputFileName);

  const exportedModels: string[] = [];
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
    if (request.optimization?.removeCinematicSequences) {
      mdl.modify.removeCinematicSequences();
    }
    mdl.modify.optimizeKeyFrames();
    mdl.sync();
    if (request.format === 'mdx' || !request.format) {
      writeFileSync(`${filePath}.mdx`, mdl.toMdx());
      exportedModels.push(`${filePath}.mdx`);
    } else {
      writeFileSync(`${filePath}.mdl`, mdl.toString());
      exportedModels.push(`${filePath}.mdl`);
    }
  });
  ce.assetManager.purgeTextures(ce.models.flatMap(([m]) => m.textures.map((t) => t.image)));
  const textures = ce.assetManager.exportTextures(ce.outputPath);

  res.send({
    exportedModels,
    exportedTextures: textures,
  });
});

app.listen(3000, () => {
  console.log('Server is running on port 3000');
});
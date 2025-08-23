import path from 'path';

import { Config } from '@/lib/global-config';
import { MDL } from '@/lib/objmdl/mdl/mdl';
import { ModelSkin, wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import { AssetManager } from '../common/model-manager';
import chalk from 'chalk';

export interface ExportContext {
  assetManager: AssetManager;
  config: Config;
}

export async function exportModelFileIdAsMdl(ctx: ExportContext, modelFileId: number, textureIds?: number[]): Promise<MDL> {
  const skins = await wowExportClient.getModelSkins(modelFileId);

  const countMatchingTextures = (skin: ModelSkin) => textureIds?.filter((id) => skin.textureIDs.includes(id)).length ?? 0;

  const match = skins.length > 0 ? skins.reduce((acc, s) => {
    const count = countMatchingTextures(s);
    if (count > acc.count) {
      return { skin: s, count };
    }
    return acc;
  }, { skin: skins[0], count: countMatchingTextures(skins[0]) }) : undefined;
  const skinName = match?.skin.id;

  const start = performance.now();
  const exported = (await wowExportClient.exportModels([{ fileDataID: modelFileId, skinName }]))[0];
  console.log("wow.export exportModels took", chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  const obj = exported.files.find((f) => f.type === 'OBJ')?.file;
  if (!obj) throw new Error('Failed to export model OBJ');

  const baseDir = await wowExportClient.getAssetDir();
  const relative = path.relative(baseDir, obj);
  return ctx.assetManager.parse(relative, true).mdl;
}

export async function exportTexture(textureId: number): Promise<string> {
  const tex = await wowExportClient.exportTextures([textureId]);
  return relativeToExport(tex[0].file);
}

async function relativeToExport(p: string): Promise<string> {
  const baseDir = await wowExportClient.getAssetDir();
  return path.relative(baseDir, p);
}

import chalk from 'chalk';
import {
  copyFileSync, existsSync, statSync, unlinkSync,
} from 'fs';
import path from 'path';

import { Config } from '@/lib/global-config';
import { MDL } from '@/lib/objmdl/mdl/mdl';
import { waitUntil } from '@/lib/utils';
import { ModelSkin, wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import { AssetManager } from '../common/asset-manager';

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
  console.log('wow.export exportModels took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  const obj = exported.files.find((f) => f.type === 'OBJ')?.file;
  if (!obj) {
    let msg = 'Failed to export model OBJ';
    if (wowExportClient.isClassic()) {
      msg += ', are you sure it exists in your classic wow installation?';
    }
    throw new Error(msg);
  }

  // TODO: find out why in some cases, the exported OBJ is empty for awhile even after the export is complete
  if (!existsSync(obj) || statSync(obj).size === 0) {
    await waitUntil(() => existsSync(obj) && statSync(obj).size > 0);
  }

  const baseDir = await wowExportClient.getAssetDir();
  const relative = path.relative(baseDir, obj);
  return ctx.assetManager.parse(relative, true).mdl;
}

export async function exportTexture(textureId: number): Promise<string> {
  const tex = await wowExportClient.exportTextures([textureId]);
  if (tex.length === 0) {
    let msg = `No texture with file data ID: ${textureId}`;
    if (wowExportClient.isClassic()) {
      msg += ', are you sure it exists in your classic wow installation?';
    }
    throw new Error(msg);
  }
  return relativeToExport(tex[0].file);
}

async function relativeToExport(p: string): Promise<string> {
  const baseDir = await wowExportClient.getAssetDir();
  return path.relative(baseDir, p);
}

export async function ensureLocalModelFileExists(filePath: string): Promise<void> {
  let fullPath = path.join(await wowExportClient.getAssetDir(), filePath);
  if (!fullPath.endsWith('.obj')) {
    fullPath += '.obj';
  }
  if (existsSync(fullPath)) return;

  console.log('File', fullPath, 'does not exist, try to export it...');

  // Get model file
  const fileName = filePath.replace(/\\/g, '/').replace(/\.obj$/, '');
  const file = await searchModelWithSkin(fileName);
  if (!file) {
    throw new Error(`File ${fileName} not found in wow.export assets`);
  }
  // Get skin
  const skins = await wowExportClient.getModelSkins(file.fileDataID);
  const skin = skins.find((s) => s.id === path.basename(filePath));

  // Export model
  const models = await wowExportClient.exportModels([
    { fileDataID: file.fileDataID, skinName: skin?.id }]);

  // Find the exported model
  const model = models[0].files.find((f) => f.fileDataID === file.fileDataID && f.type === 'OBJ');
  if (!model) {
    throw new Error(`Model ${fullPath} not found after wow.export assets`);
  }
  await waitUntil(() => existsSync(model.file));
  if (fullPath !== model.file) {
    await moveFile(model.file, fullPath);
    await moveFile(model.file.replace(/\.obj$/, '.mtl'), fullPath.replace(/\.obj$/, '.mtl'));
    await moveFile(model.file.replace(/\.obj$/, '.json'), fullPath.replace(/\.obj$/, '.json'));
    await moveFile(model.file.replace(/\.obj$/, '_bones.json'), fullPath.replace(/\.obj$/, '_bones.json'));
  }
  console.log('File', fullPath, 'exported');
}

async function searchModelWithSkin(fileWithSkin: string) {
  for (let i = fileWithSkin.length; i >= 0; i--) {
    const searchPhrase = fileWithSkin.slice(0, i);
    const files = await wowExportClient.searchFiles(searchPhrase);
    const file = files.find((f) => f.fileName.replace(/\.m2$/, '').replace(/\.wmo$/, '') === searchPhrase);
    if (file) {
      return file;
    }
  }
  return undefined;
}

async function moveFile(src: string, dest: string) {
  if (src === dest) return;
  await waitUntil(() => existsSync(src));
  copyFileSync(src, dest);
  unlinkSync(src);
}

import chalk from 'chalk';
import {
  copyFileSync, existsSync, mkdirSync, statSync, unlinkSync,
} from 'fs';
import path from 'path';

import { MDL } from '@/lib/formats/mdl/mdl';
import { Config } from '@/lib/global-config';
import { waitUntil } from '@/lib/utils';
import { ExportFile, wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import { AssetManager } from '../common/asset-manager';
import { Model } from '../common/models';

export interface ExportContext {
  assetManager: AssetManager;
  config: Config;
  outputFile: string;
  weaponInventoryTypes: [undefined | number, undefined | number];
  forceSheathed?: boolean;
}

export async function exportModelFileIdAsMdl(ctx: ExportContext, modelFileId: number, guessSkin: {
  textureIds?: number[]
  extraGeosets?: number[]
}): Promise<Model> {
  let skinName: string | undefined;

  if (guessSkin.textureIds?.length || guessSkin.extraGeosets?.length) {
    const skins = await wowExportClient.getModelSkins(modelFileId);

    const skinMatchScore = (extraGeosets: number[], textureIds: number[]) => {
      const textureScore = guessSkin.textureIds?.filter((id) => textureIds.includes(id)).length ?? 0;
      const geosetScore = guessSkin.extraGeosets?.filter((id) => extraGeosets.includes(id)).length ?? 0;
      const extraGeosetPenalty = extraGeosets.filter((id) => !guessSkin.extraGeosets?.includes(id)).length;
      return geosetScore * 1000000 - extraGeosetPenalty * 1000 + textureScore;
    };

    const match = skins.length > 0 ? skins.reduce((acc, s) => {
      const score = skinMatchScore(s.extraGeosets ?? [], s.textures);
      if (score > acc.score) {
        return { skin: s, score };
      }
      return acc;
    }, { skin: skins[0], score: skinMatchScore(skins[0].extraGeosets ?? [], skins[0].textures) }) : undefined;
    skinName = match?.skin.id;

    if (match) {
      const maxScore = skinMatchScore(guessSkin.extraGeosets || [], guessSkin.textureIds || []);
      const score = skinMatchScore(match?.skin.extraGeosets || [], match?.skin.textures || []);
      const confidence = score / maxScore;
      const skinIdx = skins.findIndex((s) => s === match.skin);
      console.log('Chosen skin:', skinName, 'with confidence:', `${(confidence * 100).toFixed(2)}%`, { score, maxScore, skinIdx });
    }
  }

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
  return ctx.assetManager.parse(relative, true);
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

export async function ensureLocalModelFileExists(filePath: string): Promise<string> {
  const baseDir = await wowExportClient.getAssetDir();
  let fullPath = path.resolve(path.join(baseDir, filePath));
  if (!fullPath.startsWith(baseDir)) {
    throw new Error(`File ${filePath} is outside of the wow.export assets directory`);
  }

  if (!fullPath.endsWith('.obj')) {
    fullPath += '.obj';
  }
  if (existsSync(fullPath)) return path.relative(baseDir, fullPath);

  console.log('Try exporting local file', fullPath, 'from wow.export');

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

  if (models.length === 0) {
    console.log(chalk.red(`File ${file.fileDataID} ${file.fileName} not found after wow.export assets`), models);
    throw new Error(`Model ${fullPath} not found after wow.export assets`);
  }

  // Find the exported model
  const model = models[0].files.find((f) => f.fileDataID === file.fileDataID && f.type === 'OBJ');
  if (!model) {
    throw new Error(`Model ${fullPath} not found after wow.export assets`);
  }
  await waitUntil(() => existsSync(model.file));
  // if (fullPath !== model.file) {
  //   await moveFile(model.file, fullPath);
  //   await moveFile(model.file.replace(/\.obj$/, '.mtl'), fullPath.replace(/\.obj$/, '.mtl'));
  //   await moveFile(model.file.replace(/\.obj$/, '.json'), fullPath.replace(/\.obj$/, '.json'));
  //   if (!fullPath.endsWith('wmo.obj')) {
  //     await moveFile(model.file.replace(/\.obj$/, '_bones.json'), fullPath.replace(/\.obj$/, '_bones.json'));
  //   }
  // }
  console.log('File', model.file, 'exported');
  return path.relative(baseDir, model.file);
}

async function searchModelWithSkin(fileWithSkin: string) {
  const dirName = path.dirname(fileWithSkin);
  for (let i = fileWithSkin.length; i > dirName.length; i--) {
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
  mkdirSync(path.dirname(dest), { recursive: true });
  await waitUntil(() => existsSync(src));
  copyFileSync(src, dest);
  unlinkSync(src);
}

const debug = false;

export async function applyReplaceableTextures(ctx: ExportContext, mdl: MDL, replaceableTextures: Record<string, number>) {
  debug && console.log('applyReplaceableTextrures', replaceableTextures);
  const textureMap = new Map<number, ExportFile[]>();

  for (const texture of mdl.textures) {
    const type = texture.wowData.type.toString();
    if (!replaceableTextures[type]) continue;

    debug && console.log('applyReplaceableTextrures', type, replaceableTextures[type]);

    const fileDataId = replaceableTextures[type];

    if (!textureMap.has(fileDataId)) {
      const file = await wowExportClient.exportTextures([fileDataId]);
      textureMap.set(fileDataId, file);
      debug && console.log('Replaceable texture:', path.relative(ctx.config.wowExportAssetDir, file[0].file));
    }
    const fileData = textureMap.get(fileDataId)!;

    debug && console.log('fileData', fileData);

    const fileDataPath = path.relative(ctx.config.wowExportAssetDir, fileData[0].file);
    ctx.assetManager.addPngTexture(fileDataPath);
    texture.image = path.join(ctx.config.assetPrefix, fileDataPath).replace('.png', '.blp');
    debug && console.log('texture.image', texture.image);
  }
}

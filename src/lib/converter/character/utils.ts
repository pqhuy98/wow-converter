import chalk from 'chalk';
import path from 'path';

import { convertM2CollisionToMdl } from '@/lib/converter/wow-model/direct/m2';
import {
  exportAssetExists, exportAssetStat, writeExportAsset,
} from '@/lib/export-asset-store';
import { profileScope } from '@/lib/export-profile';
import { MDL } from '@/lib/formats/mdl/mdl';
import { Config } from '@/lib/global-config';
import { waitUntil } from '@/lib/utils';
import { getFileNameByID } from '@/lib/wow/archive/client/name-client';
import { getRawWowFile } from '@/lib/wow/archive/client/raw-client';
import { replaceExtension } from '@/lib/wow/export/writers/export-helper';
import { BLPImage } from '@/lib/wow/formats/blp/blp';
import { BufferWrapper } from '@/lib/wow/formats/buffer';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import { AssetManager } from '../common/asset-manager';
import { Model } from '../common/models';
import { isDirectPipeline } from '../common/pipeline';

export interface ExportContext {
  assetManager: AssetManager;
  config: Config;
  outputFile: string;
  weaponInventoryTypes: [undefined | number, undefined | number];
  forceSheathed?: boolean;
  withCollision?: boolean;
}

export async function exportModelFileIdAsMdl(ctx: ExportContext, modelFileId: number, guessSkin: {
  textureIds?: number[]
  extraGeosets?: number[]
}): Promise<Model> {
  let skinName: string | undefined;

  if (guessSkin.textureIds?.length || guessSkin.extraGeosets?.length) {
    const skins = await profileScope('getModelSkins', () => wowExportClient.getModelSkins(modelFileId));

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

  if (isDirectPipeline()) {
    return ctx.assetManager.parseDirect({ fileDataID: modelFileId, skinName });
  }

  const exported = await profileScope(`client/rest.exportModels/${modelFileId}`, () => wowExportClient.exportModels([{ fileDataID: modelFileId, skinName }]).then((r) => r[0]));

  const obj = exported.files.find((f) => f.type === 'OBJ')?.file;
  if (!obj) {
    let msg = 'Failed to export model OBJ';
    if (wowExportClient.isClassic()) {
      msg += ', are you sure it exists in your classic wow installation?';
    }
    throw new Error(msg);
  }

  // TODO: find out why in some cases, the exported OBJ is empty for awhile even after the export is complete
  if (!(await exportAssetExists(obj)) || (await exportAssetStat(obj)).size === 0) {
    await waitUntil(async () => (await exportAssetExists(obj) && (await exportAssetStat(obj)).size > 0));
  }

  const baseDir = await wowExportClient.getAssetDir();
  const relative = path.relative(baseDir, obj);
  return ctx.assetManager.parse(relative, true);
}

export async function exportTexture(textureId: number): Promise<string> {
  if (isDirectPipeline()) {
    // Direct path: decode the raw BLP2 with the same decoder + PNG writer the
    // legacy exporter used, writing under the legacy PNG path.
    // Downstream consumers (compositing, BLP encoding) read it unchanged.
    const raw = await getRawWowFile(textureId);
    const fileName = (await getFileNameByID(textureId)) ?? `unknown/${textureId}.blp`;
    const relPath = path.normalize(replaceExtension(fileName, '.png').replace(/\s/g, ''));
    const baseDir = await wowExportClient.getAssetDir();
    const absPath = path.join(baseDir, relPath);
    if (!await exportAssetExists(absPath)) {
      const png = new BLPImage(new BufferWrapper(raw)).toPNG(0b1111).raw;
      await writeExportAsset(absPath, png);
    }
    return relPath;
  }

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

/**
 * Resolve and convert a local (listfile path based) model reference,
 * optionally with its collision mesh. Handles both pipelines.
 */
export async function exportLocalModelAsMdl(
  assetManager: AssetManager,
  config: Config,
  filePath: string,
  withCollision = false,
): Promise<{ model: Model; collision?: MDL }> {
  if (!isDirectPipeline()) {
    const relPath = await ensureLocalModelFileExists(filePath);
    const model = await assetManager.parse(relPath, true);
    let collision: MDL | undefined;
    if (withCollision) {
      const collisionRelativePath = `${relPath.replace(/\.obj$/, '')}.phys.obj`;
      const collisionFullPath = path.join(config.wowExportAssetDir, collisionRelativePath);
      console.log('collisionPath', collisionFullPath);
      if (await exportAssetExists(collisionFullPath)) {
        collision = (await assetManager.parse(collisionRelativePath, true)).mdl;
      }
    }
    return { model, collision };
  }

  // Direct path: resolve fileDataID + skin from the listfile-style reference.
  const fileName = filePath.replace(/\\/g, '/').replace(/\.obj$/, '');
  const file = await searchModelWithSkin(fileName);
  if (!file) {
    throw new Error(`File ${fileName} not found in wow.export assets`);
  }
  const skins = await wowExportClient.getModelSkins(file.fileDataID);
  const skin = skins.find((s) => s.id === path.basename(filePath));

  const model = await assetManager.parseDirect({ fileDataID: file.fileDataID, skinName: skin?.id });
  let collision: MDL | undefined;
  // WMOs have no .phys collision bundle (legacy never produced one either).
  if (withCollision && !file.fileName.toLowerCase().endsWith('.wmo')) {
    collision = (await convertM2CollisionToMdl(config, { fileDataID: file.fileDataID, skinName: skin?.id })).mdl;
  }
  return { model, collision };
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
  if (await exportAssetExists(fullPath)) return path.relative(baseDir, fullPath);

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
  await waitUntil(() => exportAssetExists(model.file));
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

const debug = false;

export async function applyReplaceableTextures(ctx: ExportContext, mdl: MDL, replaceableTextures: Record<string, number>) {
  debug && console.log('applyReplaceableTextrures', replaceableTextures);
  const textureMap = new Map<number, string>();

  for (const texture of mdl.textures) {
    const type = texture.wowData.type.toString();
    if (!replaceableTextures[type]) continue;

    debug && console.log('applyReplaceableTextrures', type, replaceableTextures[type]);

    const fileDataId = replaceableTextures[type];

    let fileDataPath = textureMap.get(fileDataId);
    if (fileDataPath === undefined) {
      fileDataPath = await profileScope(`exportTexture/${fileDataId}`, () => exportTexture(fileDataId));
      textureMap.set(fileDataId, fileDataPath);
      debug && console.log('Replaceable texture:', fileDataPath);
    }

    ctx.assetManager.addPngTexture(fileDataPath);
    texture.image = path.join(ctx.config.assetPrefix, fileDataPath).replace('.png', '.blp');
    debug && console.log('texture.image', texture.image);
  }
}

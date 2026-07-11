import path from 'path';

import { getTextureSource } from '@/lib/converter/common/texture-source';
import { convertM2CollisionToMdl } from '@/lib/converter/wow-model/direct/m2';
import {
  exportAssetExists, isCascExportCurrent, readExportAsset, writeCascExportMarker, writeExportAsset,
} from '@/lib/export-asset-store';
import { MDL } from '@/lib/formats/mdl/mdl';
import { Config } from '@/lib/global-config';
import { getFileNameByID } from '@/lib/wow/archive/client/name-client';
import { getRawWowFile } from '@/lib/wow/archive/client/raw-client';
import { replaceExtension } from '@/lib/wow/export/writers/export-helper';
import { BLPImage } from '@/lib/wow/formats/blp/blp';
import { BufferWrapper } from '@/lib/wow/formats/buffer';
import { ModelSkin, wowDataClient } from '@/lib/wow-data-client/wow-data-client';

import { AssetManager } from '../common/asset-manager';
import { Model } from '../common/models';
import { localModelBasename, normalizeLocalModelRef } from '../local-model-path';

export interface ExportContext {
  assetManager: AssetManager;
  config: Config;
  outputFile: string;
  weaponInventoryTypes: [undefined | number, undefined | number];
  forceSheathed?: boolean;
  withCollision?: boolean;
  /** Explicit DB2 skin id for local model refs (browse tab). */
  localModelSkinId?: string;
}

export async function exportModelFileIdAsMdl(ctx: ExportContext, modelFileId: number, guessSkin: {
  textureIds?: number[]
  extraGeosets?: number[]
}): Promise<Model> {
  let skinName: string | undefined;

  if (guessSkin.textureIds?.length || guessSkin.extraGeosets?.length) {
    const skins = await wowDataClient.getModelSkins(modelFileId);

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

  return ctx.assetManager.parseDirect({ fileDataID: modelFileId, skinName });
}

export async function exportTexture(textureId: number): Promise<string> {
  const { relPath } = await exportTexturePng(textureId);
  return relPath;
}

/** Export a BLP texture to PNG bytes; caches on disk when the CASC build changes. */
export async function exportTexturePng(textureId: number): Promise<{ relPath: string; png: Buffer }> {
  const raw = await getRawWowFile(textureId);
  const fileName = (await getFileNameByID(textureId)) ?? `unknown/${textureId}.blp`;
  const relPath = path.normalize(replaceExtension(fileName, '.png').replace(/\s/g, ''));
  const baseDir = await wowDataClient.getAssetDir();
  const absPath = path.join(baseDir, relPath);
  const buildKey = wowDataClient.cascInfo?.buildKey ?? '';
  if (await isCascExportCurrent(absPath, buildKey, textureId)) {
    return { relPath, png: await readExportAsset(absPath) };
  }
  const png = new BLPImage(new BufferWrapper(raw)).toPNG(0b1111).raw;
  await writeExportAsset(absPath, png);
  await writeCascExportMarker(absPath, buildKey, textureId);
  return { relPath, png };
}

/** Resolve PNG bytes from the texture-source registry or export-asset cache. */
export async function resolveTexturePngBytes(exportAssetDir: string, relPath: string): Promise<Buffer> {
  const normalized = path.normalize(relPath.replace(/\\/g, '/'));
  const source = getTextureSource(normalized);
  if (source?.kind === 'png') {
    return source.png;
  }
  if (source?.kind === 'blp') {
    const raw = await getRawWowFile(source.fileDataID);
    return new BLPImage(new BufferWrapper(raw)).toPNG(0b1111).raw;
  }
  const absPath = path.join(exportAssetDir, normalized);
  if (await exportAssetExists(absPath)) {
    return readExportAsset(absPath);
  }
  throw new Error(`texture not found: ${relPath}`);
}

export function textureRelPath(exportAssetDir: string, texturePath: string): string {
  if (path.isAbsolute(texturePath)) {
    return path.normalize(path.relative(exportAssetDir, texturePath).replace(/\\/g, '/'));
  }
  return path.normalize(texturePath.replace(/\\/g, '/'));
}

/** Pick a DB2 skin for a local model ref (explicit skin suffix or first skin). */
function pickLocalModelSkin(filePath: string, modelFileName: string, skins: ModelSkin[]): ModelSkin | undefined {
  if (skins.length === 0) return undefined;

  const refBase = localModelBasename(filePath);
  const modelBase = localModelBasename(modelFileName);

  const byId = skins.find((s) => s.id === refBase);
  if (byId) return byId;

  const bySuffix = skins.find((s) => refBase.endsWith(`_${s.id}`) || refBase.endsWith(s.id));
  if (bySuffix) return bySuffix;

  if (refBase === modelBase) return skins[0];

  return skins[0];
}

/**
 * Resolve and convert a local (listfile path based) model reference,
 * optionally with its collision mesh.
 */
export async function exportLocalModelAsMdl(
  assetManager: AssetManager,
  config: Config,
  filePath: string,
  options?: { withCollision?: boolean; skinIdOverride?: string },
): Promise<{ model: Model; collision?: MDL }> {
  const withCollision = options?.withCollision ?? false;
  const skinIdOverride = options?.skinIdOverride;
  const fileName = normalizeLocalModelRef(filePath);
  const file = await searchModelWithSkin(fileName);
  if (!file) {
    throw new Error(`File ${fileName} not found in WoW assets`);
  }
  const skins = await wowDataClient.getModelSkins(file.fileDataID);
  const skin = skinIdOverride
    ? skins.find((s) => s.id === skinIdOverride) ?? pickLocalModelSkin(filePath, file.fileName, skins)
    : pickLocalModelSkin(filePath, file.fileName, skins);

  const model = await assetManager.parseDirect({ fileDataID: file.fileDataID, skinName: skin?.id });
  let collision: MDL | undefined;
  if (withCollision && !file.fileName.toLowerCase().endsWith('.wmo')) {
    collision = (await convertM2CollisionToMdl(config, { fileDataID: file.fileDataID, skinName: skin?.id })).mdl;
  }
  return { model, collision };
}

async function searchModelWithSkin(fileWithSkin: string) {
  const dirName = path.dirname(fileWithSkin);
  for (let i = fileWithSkin.length; i > dirName.length; i--) {
    const searchPhrase = fileWithSkin.slice(0, i);
    const files = await wowDataClient.searchFiles(searchPhrase);
    const file = files.find((f) => normalizeLocalModelRef(f.fileName) === searchPhrase);
    if (file) {
      return file;
    }
  }
  return undefined;
}

function toLocalRefPath(dir: string, name: string): string {
  return path.join(dir, name).replace(/\//g, '\\');
}

export interface LocalModelSkinOption {
  id: string;
  label: string;
  localRef: string;
}

function localRefForSkin(dir: string, modelBase: string, skin: ModelSkin, isDefault: boolean): string {
  const skinBase = skin.id.split(',')[0]!;
  if (isDefault) return toLocalRefPath(dir, modelBase);
  if (skinBase.startsWith(modelBase)) return toLocalRefPath(dir, skinBase);
  if (skinBase === modelBase) return toLocalRefPath(dir, modelBase);
  return toLocalRefPath(dir, `${modelBase}_${skinBase}`);
}

/** Skin variants for a listfile model (browse UI / local ref validation). */
export async function getModelSkinOptions(
  fileDataID: number,
  listfilePath: string,
  skins?: ModelSkin[],
): Promise<LocalModelSkinOption[]> {
  await wowDataClient.waitUntilReady();
  const resolvedSkins = skins ?? await wowDataClient.getModelSkins(fileDataID);
  const baseRef = normalizeLocalModelRef(listfilePath);
  const dir = path.dirname(baseRef);
  const modelBase = path.basename(baseRef);

  if (resolvedSkins.length === 0) {
    return [{ id: '', label: 'default', localRef: toLocalRefPath(dir, modelBase) }];
  }

  const options = resolvedSkins.map((skin, index) => ({
    id: skin.id,
    label: skin.label,
    localRef: localRefForSkin(dir, modelBase, skin, index === 0),
  }));

  const byLabel = new Map<string, LocalModelSkinOption>();
  for (const option of options) {
    if (!byLabel.has(option.label)) {
      byLabel.set(option.label, option);
    }
  }
  return [...byLabel.values()];
}

function buildLocalRefVariants(listfilePath: string, skins: ModelSkin[]): string[] {
  const baseRef = normalizeLocalModelRef(listfilePath);
  const dir = path.dirname(baseRef);
  const modelBase = path.basename(baseRef);
  const refs = new Set<string>([toLocalRefPath(dir, modelBase)]);

  for (const skin of skins) {
    const skinBase = skin.id.split(',')[0]!;
    if (skinBase === modelBase) continue;
    if (skinBase.startsWith(modelBase)) {
      refs.add(toLocalRefPath(dir, skinBase));
    } else {
      refs.add(toLocalRefPath(dir, skinBase));
      refs.add(toLocalRefPath(dir, `${modelBase}_${skinBase}`));
    }
  }
  return [...refs];
}

/** Validate a local model ref against WoW listfile / skin data (not export cache files). */
export async function resolveLocalModelRef(localPath: string): Promise<{ ok: boolean; similarFiles: string[] }> {
  await wowDataClient.waitUntilReady();
  const normalized = normalizeLocalModelRef(localPath);
  const file = await searchModelWithSkin(normalized);
  if (!file) {
    return { ok: false, similarFiles: [] };
  }
  const skins = await wowDataClient.getModelSkins(file.fileDataID);
  const similarFiles = buildLocalRefVariants(file.fileName, skins);
  return { ok: true, similarFiles };
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
      fileDataPath = await exportTexture(fileDataId);
      textureMap.set(fileDataId, fileDataPath);
      debug && console.log('Replaceable texture:', fileDataPath);
    }

    ctx.assetManager.addPngTexture(fileDataPath);
    texture.image = path.join(ctx.config.assetPrefix, fileDataPath).replace('.png', '.blp');
    debug && console.log('texture.image', texture.image);
  }
}

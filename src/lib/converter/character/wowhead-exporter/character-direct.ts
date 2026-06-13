/**
 * Converter-side direct character export (WOW_PIPELINE=direct).
 *
 * Replaces the /rest/exportCharacter RPC: fetches DB2-derived metadata from
 * /rest/charMeta, builds the geoset mask and bakes customization materials
 * locally (CharMaterialRenderer over the raw-file layer), then runs the direct
 * M2 -> MDL conversion. Mirrors exportCharacterModelHeadless byte-for-byte.
 */
import { createHash } from 'crypto';
import path from 'path';

import { ExportContext } from '@/lib/converter/character/utils';
import { DirectDataTexture } from '@/lib/converter/wow-model/direct/m2/textures';
import { profileScope } from '@/lib/export-profile';
import { ensureConverterCasc } from '@/lib/wow/archive/client/remote-casc';
import { CharMaterialRenderer } from '@/lib/wow/character/char-material-renderer';
import type { GeosetMaskEntry } from '@/lib/wow/export/m2/m2-exporter';
import { replaceExtension } from '@/lib/wow/export/writers/export-helper';
import { Skin } from '@/lib/wow/formats/m2/skin';
import {
  CharacterMetaResponse, ExportCharacterParams, wowExportClient,
} from '@/lib/wowexport-client/wowexport-client';

import { Model } from '../../common/models';

/** Default-enabled geosets, mirroring the UI defaults (see M2Renderer). */
const DEFAULT_GEOSETS = new Set([0, 101, 201, 301, 401, 501, 601, 702, 801, 901, 1001, 1101, 1201, 1301, 1400, 1501, 1600, 1700, 1801, 1901, 2001, 2101, 2201, 2301, 2400, 2500, 2601, 2700, 2801, 2900, 3000, 3100, 3202, 3301, 3401, 3500, 3600, 3700, 3801, 3900, 4001, 4101, 4201, 4301, 4401, 4501, 4601, 4701, 4801, 4901, 5001, 5101]);

/** Port of the geoset mask construction in exportCharacterModelHeadless. */
function buildCharacterGeosetMask(
  skin: Skin,
  meta: CharacterMetaResponse,
  body: ExportCharacterParams,
): GeosetMaskEntry[] {
  const subMeshes = skin.subMeshes || [];
  const geosetGroup = (id: number): number => Math.floor(id / 100) * 100;

  const geosetMask = subMeshes.map((subMesh) => ({
    id: subMesh.submeshID,
    checked: DEFAULT_GEOSETS.has(subMesh.submeshID),
  }));

  const turnOnGeoset = (subMeshId: number, turnOffOthers = true): void => {
    const group = geosetGroup(subMeshId);
    const matchingGeosets = geosetMask.filter((geoset) => geoset.id === subMeshId);
    if (matchingGeosets.length > 0) {
      matchingGeosets.forEach((geoset) => { geoset.checked = true; });
      if (group === 0 || !turnOffOthers) return; // base geometry cannot be overridden
      geosetMask.forEach((geoset) => {
        if (group === geosetGroup(geoset.id) && geoset.id !== subMeshId) geoset.checked = false;
      });
    }
  };

  const hideGeosetIds = body.hideGeosetIds ?? [];

  // Turn on customization geosets
  for (const [, choiceID] of Object.entries(body.customizations || {})) {
    const geosets = meta.choices[Number(choiceID)]?.geosets ?? [];
    for (const geosetId of geosets) {
      if (!hideGeosetIds.includes(geosetId)) turnOnGeoset(geosetId, true);
    }
  }

  if (Array.isArray(body.geosetIds) && body.geosetIds.length > 0) {
    for (const geosetId of body.geosetIds) turnOnGeoset(geosetId, true);
  }

  if (Array.isArray(hideGeosetIds) && hideGeosetIds.length > 0) {
    const idsToHide = new Set(hideGeosetIds);
    for (const geoset of geosetMask) {
      if (idsToHide.has(geoset.id)) geoset.checked = false;
    }
  }

  return geosetMask;
}

/**
 * Baked materials are pure functions of (race, gender, model override,
 * customizations); cache the composited PNGs so re-exports of the same
 * character (e.g. mount + rider, warmup passes) skip the bake entirely.
 * Consumers only read the cached buffers (resolveTextures registers/writes
 * them), so sharing across requests is safe.
 */
const BAKE_CACHE_MAX = 8;
const bakeCache = new Map<string, Map<number, DirectDataTexture>>();

export function clearCharacterBakeCache(): void {
  bakeCache.clear();
}

function bakeCacheKey(body: ExportCharacterParams): string {
  return JSON.stringify({
    race: body.race,
    gender: body.gender,
    fileDataIdOverride: body.fileDataIdOverride ?? null,
    customizations: body.customizations ?? {},
  });
}

/** Port of the material bake loop in exportCharacterModelHeadless. */
async function bakeCharacterMaterials(
  meta: CharacterMetaResponse,
  body: ExportCharacterParams,
): Promise<Map<number, DirectDataTexture>> {
  CharMaterialRenderer.init();
  const chrMaterials = new Map<number, CharMaterialRenderer>();

  for (const [, choiceID] of Object.entries(body.customizations || {})) {
    const materials = meta.choices[Number(choiceID)]?.materials ?? [];
    for (const mat of materials) {
      let chrMaterial = chrMaterials.get(mat.material.TextureType);
      if (!chrMaterial) {
        chrMaterial = new CharMaterialRenderer(mat.material.TextureType, mat.material.Width, mat.material.Height, true);
        chrMaterials.set(mat.material.TextureType, chrMaterial);
        chrMaterial.init();
      }
      await chrMaterial.setTextureTarget(
        mat.custMaterial,
        mat.section!,
        mat.material,
        mat.textureLayer,
        true,
        mat.filename ?? undefined,
      );
    }
  }

  const dataTextures = new Map<number, DirectDataTexture>();
  for (const [textureType, chrMaterial] of chrMaterials) {
    let originalFilename: string | null = null;
    if (chrMaterial.textureTargets && chrMaterial.textureTargets.length > 0) {
      const target = chrMaterial.textureTargets.find((t) => t.filename);
      if (target && target.filename) originalFilename = target.filename;
    }
    dataTextures.set(Number(textureType), {
      filename: originalFilename,
      source: { kind: 'png', png: chrMaterial.getPNG() },
    });
    chrMaterial.dispose();
  }

  return dataTextures;
}

/** Mirror of the export path naming in exportCharacterModelHeadless + rest-server suffix. */
function resolveCharacterExportPath(exportRoot: string, fileName: string, exportSuffix: string): string {
  let exportPath = replaceExtension(path.normalize(path.join(exportRoot, fileName.replace(/\s/g, ''))), '.obj');
  if (exportSuffix) {
    const dir = path.dirname(exportPath);
    const base = path.basename(exportPath, path.extname(exportPath));
    const ext = path.extname(exportPath);
    exportPath = path.join(dir, `${base}_${exportSuffix}${ext}`);
  }
  return exportPath;
}

/**
 * Direct replacement for wowExportClient.exportCharacter + OBJ parse.
 * `body` must be the exact object the legacy RPC would have posted — the
 * export suffix (part of the model name) is derived from its JSON form.
 */
export async function exportCharacterDirectAsModel(ctx: ExportContext, body: ExportCharacterParams): Promise<Model> {
  ensureConverterCasc();

  const meta = await wowExportClient.getCharMeta({
    race: body.race,
    gender: body.gender,
    fileDataIdOverride: body.fileDataIdOverride,
    customizations: body.customizations,
  });

  const cacheKey = bakeCacheKey(body);
  let dataTextures = bakeCache.get(cacheKey);
  if (dataTextures) {
    // Refresh LRU recency.
    bakeCache.delete(cacheKey);
    bakeCache.set(cacheKey, dataTextures);
  } else {
    dataTextures = await profileScope('bakeMaterials', () => bakeCharacterMaterials(meta, body));
    bakeCache.set(cacheKey, dataTextures);
    if (bakeCache.size > BAKE_CACHE_MAX) bakeCache.delete(bakeCache.keys().next().value!);
  }

  // Same suffix the server derives from the RPC body (md5 of its JSON).
  const suffix = createHash('md5').update(JSON.stringify(body || {})).digest('hex').slice(0, 8);
  const exportPath = resolveCharacterExportPath(ctx.config.wowExportAssetDir, meta.fileName, suffix);

  return ctx.assetManager.parseDirect({
    fileDataID: meta.fileDataID,
    variantTextures: [],
    geosetMaskBuilder: (skin) => buildCharacterGeosetMask(skin, meta, body),
    dataTextures,
    excludeAnimIds: body.excludeAnimationIds ?? [],
    exportPathOverride: exportPath,
  });
}

/**
 * Direct M2 -> MDL conversion (no OBJ/PNG intermediates, no server export).
 *
 * Mirrors the legacy wow.export pipeline exactly:
 *   wow.export: exportFilesWithSkins -> M2Exporter.exportAsOBJ (OBJ/MTL/JSON files)
 *   converter: convertWowExportModel (parse files -> assemble MDL)
 * by constructing the same in-memory structures the file parsers would have
 * produced, then running the shared MDL assembly core. Output is
 * byte-identical to the legacy path.
 */
import path from 'path';

import { assembleWowModel } from '@/lib/converter/wow-model/assemble';
import { AnimationFile } from '@/lib/converter/wow-model/bundle/animation';
import { M2MetadataFile } from '@/lib/converter/wow-model/bundle/metadata';
import { buildCollisionObjResult, buildMeshes, buildObjResult } from '@/lib/converter/wow-model/direct/m2/geometry';
import { normalizeJsonValues } from '@/lib/converter/wow-model/direct/m2/json-normalize';
import { convertWmoToMdl } from '@/lib/converter/wow-model/direct/wmo';
import { profileScope } from '@/lib/export-profile';
import { MDL } from '@/lib/formats/mdl/mdl';
import { Config } from '@/lib/global-config';
import { getFileNameByID } from '@/lib/wow/archive/client/name-client';
import { getRawWowFile } from '@/lib/wow/archive/client/raw-client';
import { ensureConverterCasc } from '@/lib/wow/archive/client/remote-casc';
import type { GeosetMaskEntry, VariantTexture } from '@/lib/wow/export/m2/m2-exporter';
import { replaceExtension } from '@/lib/wow/export/writers/export-helper';
import { BufferWrapper } from '@/lib/wow/formats/buffer';
import { constants } from '@/lib/wow/formats/constants';
import { M2Loader } from '@/lib/wow/formats/m2/m2-loader';
import { Skin } from '@/lib/wow/formats/m2/skin';
import { ModelSkin, wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import { buildBonesData } from './bones';
import { buildMetadataObject } from './metadata';
import { DirectDataTexture, resolveTextures } from './textures';

export interface ConvertM2Options {
  fileDataID: number;
  /** Skin id (ModelSkin.id) to apply; drives variant textures + geoset mask. */
  skinName?: string;
  /** Explicit variant textures (overrides skin resolution). */
  variantTextures?: VariantTexture[];
  /** Explicit geoset mask (overrides skin-based mask; used by the character path). */
  geosetMask?: GeosetMaskEntry[];
  /** Build the geoset mask from the loaded skin (character path). */
  geosetMaskBuilder?: (skin: Skin) => GeosetMaskEntry[];
  /** Composited data textures by textureType (character path). */
  dataTextures?: Map<number, DirectDataTexture>;
  /** WoW animation IDs to strip from bones data. */
  excludeAnimIds?: Iterable<number>;
  /** Override the export naming (character path uses its own layout). */
  exportPathOverride?: string;
}

/** Mirror of getExportPath rooted at the converter's asset dir (removePathSpaces on). */
function virtualExportPath(exportRoot: string, file: string): string {
  return path.normalize(path.join(exportRoot, file.replace(/\s/g, '')));
}

/** Port of wow.export's buildGeosetMaskForSkin (tab-models). */
export function buildGeosetMaskForSkin(m2Skin: Skin, skin: Pick<ModelSkin, 'extraGeosets'> | undefined): GeosetMaskEntry[] {
  const extraSet = new Set(skin?.extraGeosets ?? []);
  const mask = new Array<GeosetMaskEntry & { id: number }>(m2Skin.subMeshes.length);

  for (let i = 0; i < m2Skin.subMeshes.length; i++) {
    const mesh = m2Skin.subMeshes[i];
    const id = mesh.submeshID;

    mask[i] = { id, checked: true };

    // If extra geosets are provided, first disable 0..900 range, then enable explicit extras.
    if (skin?.extraGeosets !== undefined) {
      if (id > 0 && id < 900) mask[i].checked = false;
      if (extraSet.has(id)) mask[i].checked = true;
    } else {
      // Default selection logic mirrors UI: enable ids ending with '0' or '01'.
      const idStr = id.toString();
      mask[i].checked = (idStr.endsWith('0') || idStr.endsWith('01'));
    }
  }

  return mask;
}

/** Mirror of the legacy export path naming in exportFilesWithSkins. */
function resolveExportPath(exportRoot: string, fileName: string, selectedSkinName: string | null): string {
  let exportPath: string;
  if (selectedSkinName !== null) {
    const baseFileName = path.basename(fileName, path.extname(fileName));
    let skinnedName: string;

    if (selectedSkinName.startsWith(baseFileName)) skinnedName = path.join(path.dirname(fileName), selectedSkinName + path.extname(fileName));
    else skinnedName = path.join(path.dirname(fileName), `${baseFileName}_${selectedSkinName}${path.extname(fileName)}`);

    exportPath = virtualExportPath(exportRoot, skinnedName);
  } else {
    exportPath = virtualExportPath(exportRoot, fileName);
  }

  return replaceExtension(exportPath, '.obj');
}

export async function convertM2ToMdl(config: Config, opts: ConvertM2Options): Promise<{ mdl: MDL; texturePaths: Set<string> }> {
  return profileScope('converter/m2ToMdx', async () => {
    ensureConverterCasc();

    const { fileDataID } = opts;
    const exportRoot = config.wowExportAssetDir;

    const raw = await profileScope('rawM2', () => getRawWowFile(fileDataID));

    // Resolve the listfile name (or synthesize an unknown/ name from magic).
    let fileName = await getFileNameByID(fileDataID);
    let isM2: boolean;
    if (fileName === undefined) {
      const magic = new BufferWrapper(raw).readUInt32LE();
      isM2 = magic === constants.MAGIC.MD20 || magic === constants.MAGIC.MD21;
      fileName = `unknown/${fileDataID}${isM2 ? '.m2' : '.wmo'}`;
    } else {
      isM2 = fileName.toLowerCase().endsWith('.m2');
    }
    if (!isM2) {
      // WMO branch (M3 still unsupported; WMOLoader rejects it like the server does).
      return convertWmoToMdl(config, { fileDataID, fileName, raw });
    }

    const m2 = new M2Loader(new BufferWrapper(raw));
    await profileScope('m2.load', () => m2.load());
    const skin0 = await profileScope('m2.getSkin', () => m2.getSkin(0));
    skin0.fileName = (await getFileNameByID(skin0.fileDataID)) ?? skin0.fileName;

    // Skin selection mirrors exportFilesWithSkins.
    let variantTextures: VariantTexture[];
    let selectedSkinName: string | null = null;
    let selectedSkin: ModelSkin | undefined;

    if (opts.variantTextures) {
      variantTextures = opts.variantTextures;
      selectedSkinName = opts.skinName ?? null;
    } else if (opts.skinName) {
      const skins = await profileScope('getModelSkins', () => wowExportClient.getModelSkins(fileDataID));
      selectedSkin = skins.find((s) => s.id === opts.skinName);
      variantTextures = selectedSkin?.textures ?? [];
      selectedSkinName = opts.skinName;
    } else {
      // Default display fallback (first skin = first display with textures).
      const skins = await profileScope('getModelSkins', () => wowExportClient.getModelSkins(fileDataID));
      variantTextures = skins[0]?.textures ?? [];
      selectedSkinName = null;
    }

    const exportPath = opts.exportPathOverride ?? resolveExportPath(exportRoot, fileName, selectedSkinName);
    const outDir = path.dirname(exportPath);

    // Geoset mask: explicit (character) or skin-based (legacy parity: only when a skin name was requested).
    const geosetMask = opts.geosetMask
      ?? (opts.geosetMaskBuilder ? opts.geosetMaskBuilder(skin0) : undefined)
      ?? (opts.skinName && !opts.variantTextures
        ? buildGeosetMaskForSkin(skin0, selectedSkin)
        : undefined);

    const dataTextures = opts.dataTextures ?? new Map<number, DirectDataTexture>();

    // Order matters for parity: textures (variant patching) -> bones -> meta -> meshes.
    const { validTextures, mtlMaterials } = await profileScope('resolveTextures', () => resolveTextures(m2, variantTextures, dataTextures, outDir, exportRoot));

    // No JSON round-trip: bones data is normalized inside buildBonesData
    // (cached per skeleton), the metadata object is normalized in place.
    const bonesData = await profileScope('buildBones', () => buildBonesData(m2, new Set(opts.excludeAnimIds ?? []), fileDataID));
    const animation = new AnimationFile(replaceExtension(exportPath, '_bones.json'), config)
      .loadFromData(bonesData as never);

    const metaObject = await profileScope('buildMeta', () => buildMetadataObject(m2, skin0, fileDataID, fileName, geosetMask, validTextures, new Set(dataTextures.keys())));
    const metadata = new M2MetadataFile(replaceExtension(exportPath, '.json'), config, animation)
      .loadFromData(normalizeJsonValues(metaObject));

    const meshes = buildMeshes(m2, skin0, geosetMask, validTextures, new Set(dataTextures.keys()));
    const obj = buildObjResult(m2, meshes, path.basename(exportPath, '.obj'), mtlMaterials.length > 0 ? path.basename(replaceExtension(exportPath, '.mtl')) : undefined);

    return assembleWowModel({
      objFilePath: exportPath,
      obj,
      mtl: { materials: mtlMaterials },
      animation,
      metadata,
    }, config);
  });
}

/**
 * Direct equivalent of parsing the legacy `.phys.obj` collision bundle:
 * collision geometry from the M2, no MTL/bones/metadata (unloaded parsers).
 */
export async function convertM2CollisionToMdl(
  config: Config,
  opts: Pick<ConvertM2Options, 'fileDataID' | 'skinName'>,
): Promise<{ mdl: MDL; texturePaths: Set<string> }> {
  return profileScope('converter/m2CollisionToMdx', async () => {
    ensureConverterCasc();
    const { fileDataID } = opts;
    const exportRoot = config.wowExportAssetDir;

    const raw = await getRawWowFile(fileDataID);
    const fileName = (await getFileNameByID(fileDataID)) ?? `unknown/${fileDataID}.m2`;

    const m2 = new M2Loader(new BufferWrapper(raw));
    await m2.load();

    const modelPath = resolveExportPath(exportRoot, fileName, opts.skinName ?? null);
    const physPath = replaceExtension(modelPath, '.phys.obj');

    const obj = buildCollisionObjResult(m2, 'Mesh');
    // Animation/metadata stay unloaded, like the legacy phys.obj parse
    // (its sibling _bones.json/.json files never exist).
    const animation = new AnimationFile(physPath.replace(/\.obj$/, '_bones.json'), config);
    return assembleWowModel({
      objFilePath: physPath,
      obj,
      mtl: { materials: [] },
      animation,
      metadata: new M2MetadataFile(physPath.replace(/\.obj$/, '.json'), config, animation),
    }, config);
  });
}

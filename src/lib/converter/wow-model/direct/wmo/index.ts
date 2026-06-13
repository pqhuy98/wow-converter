/**
 * Direct WMO -> MDL conversion (no OBJ/PNG intermediates, no server export).
 *
 * Mirrors the legacy wow.export pipeline exactly:
 *   wow.export: exportFilesWithSkins -> WMOExporter.exportAsOBJ (OBJ/MTL/JSON files)
 *   converter: convertObjBundleToMdl (parse files -> assemble MDL)
 * by constructing the same in-memory structures the file parsers would have
 * produced, then running the shared MDL assembly core. Headless behaviour
 * matches wow.export's ignoreViewerState: all groups, no doodad sets.
 */
import path from 'path';

import { registerTextureSource } from '@/lib/converter/common/texture-source';
import { assembleWowModel } from '@/lib/converter/wow-model/assemble';
import { AnimationFile } from '@/lib/converter/wow-model/bundle/animation';
import { M2MetadataFile } from '@/lib/converter/wow-model/bundle/metadata';
import type { ObjMaterial } from '@/lib/converter/wow-model/bundle/mtl';
import type { IResult } from '@/lib/converter/wow-model/bundle/obj';
import { buildRawObjResult, ObjMesh } from '@/lib/converter/wow-model/direct/m2/geometry';
import { normalizeJsonValues } from '@/lib/converter/wow-model/direct/m2/json-normalize';
import { MDL } from '@/lib/formats/mdl/mdl';
import { Config } from '@/lib/global-config';
import { getFileIDByName, getFileNameByID } from '@/lib/wow/archive/client/name-client';
import { getRawWowFile } from '@/lib/wow/archive/client/raw-client';
import { ensureConverterCasc } from '@/lib/wow/archive/client/remote-casc';
import { replaceExtension } from '@/lib/wow/export/writers/export-helper';
import { BufferWrapper } from '@/lib/wow/formats/buffer';
import { WMOLoader, WMOMaterial } from '@/lib/wow/formats/wmo/wmo-loader';

export interface ConvertWmoOptions {
  fileDataID: number;
  /** Listfile name (resolved by the caller); synthesized when unknown. */
  fileName?: string;
  /** Raw root WMO bytes, if the caller already fetched them. */
  raw?: Buffer;
  /** Mirror legacy OBJ export path (includes `_setN` for ADT-placed WMOs). */
  exportPathOverride?: string;
}

interface TextureMapEntry {
  matPathRelative: string;
  matPath: string;
  matName: string;
}

/** Mirror of getExportPath rooted at the converter's asset dir (removePathSpaces on). */
function virtualExportPath(exportRoot: string, file: string): string {
  return path.normalize(path.join(exportRoot, file.replace(/\s/g, '')));
}

/** Texture slots in WMOExporter.exportTextures order (flags3 before color3 for shader 23). */
function exportTextureSlots(material: WMOMaterial): number[] {
  const slots = [material.texture1, material.texture2, material.texture3];
  if (material.shader === 23) {
    slots.push(material.flags3, material.color3, material.runtimeData[0], material.runtimeData[1], material.runtimeData[2], material.runtimeData[3]);
  }
  return slots;
}

/** Texture slots in WMOExporter's JSON meta order (color3 before flags3 for shader 23). */
function metaTextureSlots(material: WMOMaterial): number[] {
  const slots = [material.texture1, material.texture2, material.texture3];
  if (material.shader === 23) {
    slots.push(material.color3, material.flags3, material.runtimeData[0], material.runtimeData[1], material.runtimeData[2], material.runtimeData[3]);
  }
  return slots;
}

/**
 * Direct port of WMOExporter.exportTextures naming logic, minus pixel output:
 * each texture's relative PNG path is registered in the converter-side
 * texture-source registry (raw BLP2, encoded straight to BLP1 later).
 */
async function resolveWmoTextures(
  wmo: WMOLoader,
  outDir: string,
  exportRoot: string,
): Promise<{ textureMap: Map<number, TextureMapEntry>; materialMap: Map<number, string>; mtlMaterials: ObjMaterial[] }> {
  const textureMap = new Map<number, TextureMapEntry>();
  const materialMap = new Map<number, string>();
  const mtlMaterials: ObjMaterial[] = [];

  const isClassic = !!wmo.textureNames;
  const materials = wmo.materials ?? [];

  for (let i = 0; i < materials.length; i++) {
    const material = materials[i];

    // Variable that purely exists to not handle the first texture as the main one for shader23
    let dontUseFirstTexture = material.shader === 23;

    for (const materialTexture of exportTextureSlots(material)) {
      // Skip unused material slots.
      if (materialTexture === 0) continue;

      let fileDataID = 0;
      let fileName: string | undefined;

      if (isClassic) {
        // Classic, lookup fileDataID using file name.
        fileName = wmo.textureNames![materialTexture];
        fileDataID = (await getFileIDByName(fileName)) ?? 0;

        // Remove all whitespace from exported textures due to MTL incompatibility.
        fileName = fileName.replace(/\s/g, '');
      } else {
        // Retail, use fileDataID directly.
        fileDataID = materialTexture;
      }

      // Skip unknown/missing files.
      if (fileDataID === 0) continue;

      try {
        let texFile = `${fileDataID}.png`;
        let texPath = path.join(outDir, texFile);

        // Default MTL name to the file ID (prefixed for Maya).
        let matName = `mat_${fileDataID}`;

        // Attempt to get the file name if we don't already have it.
        if (fileName === undefined) fileName = await getFileNameByID(fileDataID);

        if (fileName !== undefined) {
          matName = `mat_${path.basename(fileName.toLowerCase(), '.blp')}`;
          matName = matName.replace(/\s/g, '');
        }

        // Shared textures layout (enableSharedTextures always on).
        if (fileName !== undefined) fileName = replaceExtension(fileName, '.png');
        else fileName = `unknown/${texFile}`;

        texPath = virtualExportPath(exportRoot, fileName);
        texFile = path.relative(outDir, texPath);

        // Verify the file is fetchable (parity with the legacy path, which
        // skipped textures whose CASC read failed). The raw bytes land in the
        // shared cache and are reused for BLP encoding.
        await getRawWowFile(fileDataID);
        registerTextureSource(path.relative(exportRoot, texPath), { kind: 'blp', fileDataID });

        mtlMaterials.push({ name: matName, map_Kd: texFile });
        textureMap.set(fileDataID, { matPathRelative: texFile, matPath: texPath, matName });

        // MTL only supports one texture per material, only link the first unless we only want the second one (e.g. for shader 23).
        if (!materialMap.has(i) && dontUseFirstTexture === false) materialMap.set(i, matName);

        // Unset skip here so we always pick the next texture in line
        dontUseFirstTexture = false;
      } catch (e) {
        console.warn(`Failed to resolve texture ${fileDataID} for WMO:`, e instanceof Error ? e.message : String(e));
      }
    }
  }

  return { textureMap, materialMap, mtlMaterials };
}

/** Load all WMO group files (retail via GFID, classic via group file names). */
async function loadGroups(wmo: WMOLoader, fileName: string): Promise<WMOLoader[]> {
  const groups = new Array<WMOLoader>(wmo.groupCount!);

  for (let i = 0; i < wmo.groupCount!; i++) {
    let data: BufferWrapper;
    if (wmo.groupIDs) {
      data = new BufferWrapper(await getRawWowFile(wmo.groupIDs[i]));
    } else {
      const groupName = fileName.replace('.wmo', `_${i.toString().padStart(3, '0')}.wmo`);
      const groupID = await getFileIDByName(groupName);
      if (!groupID) throw new Error(`Unable to resolve WMO group file: ${groupName}`);
      data = new BufferWrapper(await getRawWowFile(groupID));
    }

    const group = new WMOLoader(data, undefined);
    group.load();
    groups[i] = group;
    wmo.groups![i] = group;
  }

  return groups;
}

/**
 * Mirror of WMOExporter.exportAsOBJ's geometry assembly (group/batch loop) +
 * OBJWriter/OBJFile round-trip via buildRawObjResult.
 */
function buildWmoObjResult(
  wmo: WMOLoader,
  allGroups: WMOLoader[],
  materialMap: Map<number, string>,
  modelName: string,
  mtlLib: string | undefined,
): IResult {
  const groups: WMOLoader[] = [];
  let nInd = 0;
  let maxLayerCount = 0;

  for (const group of allGroups) {
    // Skip empty groups.
    if (!group.renderBatches || group.renderBatches.length === 0) continue;

    // 3 verts per indices.
    nInd += group.vertices!.length / 3;

    // UV counts vary between groups, allocate for the max.
    maxLayerCount = Math.max(group.uvs!.length, maxLayerCount);

    groups.push(group);
  }

  // modelsExportUV2 is always on, so no clamp to a single layer here.
  const vertsArray = new Array<number>(nInd * 3);
  const normalsArray = new Array<number>(nInd * 3);
  const uvArrays = new Array<number[]>(maxLayerCount);
  for (let i = 0; i < maxLayerCount; i++) uvArrays[i] = new Array<number>(nInd * 2);

  const meshes: ObjMesh[] = [];

  let indOfs = 0;
  for (const group of groups) {
    const indCount = group.vertices!.length / 3;

    const vertOfs = indOfs * 3;
    const groupVerts = group.vertices!;
    for (let i = 0, n = groupVerts.length; i < n; i++) vertsArray[vertOfs + i] = groupVerts[i];

    // Normals and vertices should match, so re-use vertOfs here.
    const groupNormals = group.normals!;
    for (let i = 0, n = groupNormals.length; i < n; i++) normalsArray[vertOfs + i] = groupNormals[i];

    const uvsOfs = indOfs * 2;
    const groupUVs = group.uvs ?? [];
    const uvCount = indCount * 2;

    // Write to all UV layers, even if we have no data.
    for (let i = 0; i < maxLayerCount; i++) {
      const uv = groupUVs[i];
      for (let j = 0; j < uvCount; j++) uvArrays[i][uvsOfs + j] = uv?.[j] ?? 0;
    }

    const groupName = wmo.groupNames![group.nameOfs!];

    // Load all render batches into the mesh.
    for (let bI = 0, bC = group.renderBatches!.length; bI < bC; bI++) {
      const batch = group.renderBatches![bI];
      const indices = new Array<number>(batch.numFaces);

      for (let i = 0; i < batch.numFaces; i++) indices[i] = group.indices![batch.firstFace + i] + indOfs;

      const matID = ((batch.flags & 2) === 2) ? batch.possibleBox2[2] : batch.materialID;
      meshes.push({ name: groupName + bI, triangles: indices, matName: materialMap.get(matID) });
    }

    indOfs += indCount;
  }

  // The OBJ parser only understands vt/vt2; further layers (vt3+) are dropped.
  return buildRawObjResult(vertsArray, normalsArray, uvArrays.slice(0, 2), meshes, modelName, mtlLib);
}

/**
 * Build the `.json`-equivalent metadata object. Only the fields that
 * M2MetadataFile.applyRaw consumes for fileType 'wmo' are populated
 * (fileType/fileDataID/fileName/textures/materials); the rest of the legacy
 * JSON is ignored by the converter.
 */
async function buildWmoMetadataObject(
  wmo: WMOLoader,
  fileDataID: number,
  fileName: string | undefined,
  textureMap: Map<number, TextureMapEntry>,
): Promise<Record<string, unknown>> {
  const textures: unknown[] = [];
  const textureCache = new Set<number>();

  for (const material of wmo.materials ?? []) {
    for (const materialTexture of metaTextureSlots(material)) {
      if (materialTexture === 0 || textureCache.has(materialTexture)) continue;

      const textureEntry = textureMap.get(materialTexture);

      textureCache.add(materialTexture);
      textures.push({
        fileDataID: materialTexture,
        fileNameInternal: await getFileNameByID(materialTexture),
        fileNameExternal: textureEntry?.matPathRelative,
        mtlName: textureEntry?.matName,
      });
    }
  }

  return {
    fileType: 'wmo',
    fileDataID,
    fileName,
    textures,
    materials: wmo.materials,
  };
}

export async function convertWmoToMdl(config: Config, opts: ConvertWmoOptions): Promise<{ mdl: MDL; texturePaths: Set<string> }> {
  ensureConverterCasc();

  const { fileDataID } = opts;
  const exportRoot = config.exportAssetDir;

  const raw = opts.raw ?? await getRawWowFile(fileDataID);
  const listfileName = opts.fileName ?? await getFileNameByID(fileDataID);
  const fileName = listfileName ?? `unknown/${fileDataID}.wmo`;

  const exportPath = opts.exportPathOverride
    ?? virtualExportPath(exportRoot, fileName);
  const outDir = path.dirname(exportPath);

  const wmo = new WMOLoader(new BufferWrapper(raw), fileDataID);
  wmo.load();
  const allGroups = await loadGroups(wmo, fileName);

  const { textureMap, materialMap, mtlMaterials } = await resolveWmoTextures(wmo, outDir, exportRoot);

  const animation = new AnimationFile(replaceExtension(exportPath, '_bones.json'), config);

  const metaObject = await buildWmoMetadataObject(wmo, fileDataID, listfileName, textureMap);
  const metadata = new M2MetadataFile(replaceExtension(exportPath, '.json'), config, animation)
    .loadFromData(normalizeJsonValues(metaObject));

  const obj = buildWmoObjResult(
    wmo,
    allGroups,
    materialMap,
    path.basename(exportPath, path.extname(exportPath)),
    mtlMaterials.length > 0 ? path.basename(replaceExtension(exportPath, '.mtl')) : undefined,
  );

  return assembleWowModel({
    objFilePath: exportPath,
    obj,
    mtl: { materials: mtlMaterials },
    animation,
    metadata,
  }, config);
}

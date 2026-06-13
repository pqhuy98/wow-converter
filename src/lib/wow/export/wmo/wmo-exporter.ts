/**
 * WMO exporter, ported from wow.export (src/js/3D/exporters/WMOExporter.js).
 * OBJ/MTL/CSV/meta export paths only (GLTF and raw export are unused by
 * wow-converter).
 */
import path from 'path';

import { write } from '@/lib/wow/log';
import { getCasc } from '@/lib/wow/server/runtime';

import * as listfile from '../../archive/casc/listfile';
import { BLPImage } from '../../formats/blp/blp';
import { BufferWrapper } from '../../formats/buffer';
import { constants } from '../../formats/constants';
import { WMOLoader } from '../../formats/wmo/wmo-loader';
import { wowConfig } from '../../server/config';
import type { ADTExportOptions } from '../adt/map-export-utils';
import type { ExportProgress } from '../export-progress';
import { FileManifestEntry, M2Exporter } from '../m2/m2-exporter';
import { modelReferencePath, placementCsvPath } from '../model-reference-path';
import { CSVWriter } from '../writers/csv-writer';
import {
  getExportPath, replaceExtension, replaceFile, win32ToPosix,
} from '../writers/export-helper';
import { JSONWriter } from '../writers/json-writer';
import { MTLWriter } from '../writers/mtl-writer';
import { OBJWriter } from '../writers/obj-writer';
import { outputFileExists, writeOutputFile } from '../writers/output-sink';

const doodadCache = new Set<number>();

export interface WMOGroupMaskEntry {
  checked: boolean;
  groupIndex: number;
}

export interface WMODoodadSetMaskEntry {
  checked: boolean;
}

interface TextureMapEntry {
  matPathRelative: string;
  matPath: string;
  matName: string;
}

export class WMOExporter {
  wmo: WMOLoader;

  groupMask?: WMOGroupMaskEntry[];

  doodadSetMask?: WMODoodadSetMaskEntry[];

  constructor(data: BufferWrapper, fileID: string | number) {
    this.wmo = new WMOLoader(data, fileID);
  }

  /** Set the mask used for group control. */
  setGroupMask(mask: WMOGroupMaskEntry[] | undefined): void {
    this.groupMask = mask;
  }

  /** Set the mask used for doodad set control. */
  setDoodadSetMask(mask: WMODoodadSetMaskEntry[] | undefined): void {
    this.doodadSetMask = mask;
  }

  /**
   * Export textures for this WMO.
   */
  async exportTextures(
    out: string,
    mtl: MTLWriter | null = null,
    raw = false,
    progress: ExportProgress | undefined = undefined,
  ): Promise<{ textureMap: Map<number, TextureMapEntry>; materialMap: Map<number, string> }> {
    const config = wowConfig;
    const casc = getCasc();

    const textureMap = new Map<number, TextureMapEntry>();
    const materialMap = new Map<number, string>();

    if (!config.modelsExportTextures) return { textureMap, materialMap };

    // Ensure the WMO is loaded before reading materials.
    this.wmo.load();

    const useAlpha = config.modelsExportAlpha;
    const usePosix = config.pathFormat === 'posix';
    const isClassic = !!this.wmo.textureNames;
    const materialCount = this.wmo.materials!.length;
    const wmoName = path.basename(out, '.obj');
    let textureStep = 0;

    for (let i = 0; i < materialCount; i++) {
      const material = this.wmo.materials![i];

      const materialTextures = [material.texture1, material.texture2, material.texture3];

      // Variable that purely exists to not handle the first texture as the main one for shader23
      let dontUseFirstTexture = false;

      if (material.shader === 23) {
        materialTextures.push(material.flags3);
        materialTextures.push(material.color3);
        materialTextures.push(material.runtimeData[0]);
        materialTextures.push(material.runtimeData[1]);
        materialTextures.push(material.runtimeData[2]);
        materialTextures.push(material.runtimeData[3]);

        dontUseFirstTexture = true;
      }

      for (const materialTexture of materialTextures) {
        // Skip unused material slots.
        if (materialTexture === 0) continue;

        let fileDataID = 0;
        let fileName: string | undefined;

        if (isClassic) {
          // Classic, lookup fileDataID using file name.
          fileName = this.wmo.textureNames![materialTexture];
          fileDataID = listfile.getByFilename(fileName) ?? 0;

          // Remove all whitespace from exported textures due to MTL incompatibility.
          if (config.removePathSpaces) fileName = fileName.replace(/\s/g, '');
        } else {
          // Retail, use fileDataID directly.
          fileDataID = materialTexture;
        }

        // Skip unknown/missing files.
        if (fileDataID === 0) continue;

        try {
          let texFile = fileDataID + (raw ? '.blp' : '.png');
          let texPath = path.join(path.dirname(out), texFile);

          // Default MTL name to the file ID (prefixed for Maya).
          let matName = `mat_${fileDataID}`;

          // Attempt to get the file name if we don't already have it.
          if (fileName === undefined) fileName = listfile.getByID(fileDataID);

          // If we have a valid file name, use it for the material name.
          if (fileName !== undefined) {
            matName = `mat_${path.basename(fileName.toLowerCase(), '.blp')}`;

            // Remove spaces from material name for MTL compatibility.
            if (config.removePathSpaces) matName = matName.replace(/\s/g, '');
          }

          // Map texture files relative to shared directory.
          if (config.enableSharedTextures) {
            if (fileName !== undefined) {
              // Replace BLP extension with PNG.
              if (raw === false) fileName = replaceExtension(fileName, '.png');
            } else {
              // Handle unknown files.
              fileName = listfile.formatUnknownFile(fileDataID);
            }

            texPath = getExportPath(fileName);
            texFile = path.relative(path.dirname(out), texPath);
          }

          if (config.overwriteFiles || !await outputFileExists(texPath)) {
            const data = await casc.getFile(fileDataID);

            write('Exporting WMO texture %d -> %s', fileDataID, texPath);
            if (raw) {
              await writeOutputFile(texPath, data.raw);
            } else {
              const blp = new BLPImage(data);
              await writeOutputFile(texPath, blp.toPNG(useAlpha ? 0b1111 : 0b0111).raw); // material.blendMode !== 0
            }
          } else {
            write('Skipping WMO texture export %s (file exists, overwrite disabled)', texPath);
          }

          if (usePosix) texFile = win32ToPosix(texFile);

          mtl?.addMaterial(matName, texFile);
          textureMap.set(fileDataID, { matPathRelative: texFile, matPath: texPath, matName });

          textureStep++;
          progress?.setLabel(`${wmoName} WMO textures`, textureStep);
          progress?.advance(1);

          // MTL only supports one texture per material, only link the first unless we only want the second one (e.g. for shader 23).
          if (!materialMap.has(i) && dontUseFirstTexture === false) materialMap.set(i, matName);

          // Unset skip here so we always pick the next texture in line
          dontUseFirstTexture = false;
        } catch (e) {
          write('Failed to export texture %d for WMO: %s', fileDataID, (e as Error).message);
        }
      }
    }

    return { textureMap, materialMap };
  }

  /**
   * Write WMO interior doodad placement CSV. When directModels is true, skips
   * exporting M2 OBJ/MTL/BLP (converter resolves models from FileDataID).
   */
  async exportDoodadPlacementCsv(
    out: string,
    config: ADTExportOptions | typeof wowConfig,
    progress?: ExportProgress,
    directModels = false,
  ): Promise<void> {
    const wmo = this.wmo;
    wmo.load();
    const doodadSetMask = this.doodadSetMask;

    const csvPath = placementCsvPath(out);
    if (!config.overwriteFiles && await outputFileExists(csvPath)) {
      write('Skipping model placement export %s (file exists, overwrite disabled)', csvPath);
      return;
    }

    const useAbsolute = config.enableAbsoluteCSVPaths;
    const usePosix = config.pathFormat === 'posix';
    const outDir = path.dirname(out);
    const csv = new CSVWriter(csvPath);
    csv.addField('ModelFile', 'PositionX', 'PositionY', 'PositionZ', 'RotationW', 'RotationX', 'RotationY', 'RotationZ', 'ScaleFactor', 'DoodadSet', 'FileDataID');

    const wmoLabel = path.basename(out, path.extname(out));
    const doodadSets = wmo.doodadSets ?? [];
    for (let i = 0, n = doodadSets.length; i < n; i++) {
      if (!doodadSetMask?.[i]?.checked) continue;

      const set = doodadSets[i];
      const count = set.doodadCount;
      if (directModels) {
        write('Writing interior doodad placements for set %s (%d entries)...', set.name, count);
      } else {
        write('Exporting WMO doodad set %s with %d doodads...', set.name, count);
      }
      progress?.setLabel(`${wmoLabel}, ${set.name}`, 0, count);

      for (let j = 0; j < count; j++) {
        if (progress && j > 0 && j % 50 === 0) {
          progress.setLabel(`${wmoLabel}, ${set.name}`, j, count);
        }
        const doodad = wmo.doodads![set.firstInstanceIndex + j];
        let fileDataID = 0;
        let fileName: string | undefined;

        if (wmo.fileDataIDs) {
          fileDataID = wmo.fileDataIDs[doodad.offset];
          fileName = listfile.getByID(fileDataID);
        } else {
          fileName = wmo.doodadNames![doodad.offset];
          fileDataID = listfile.getByFilename(fileName) || 0;
        }

        if (fileDataID <= 0) continue;

        try {
          if (directModels) {
            fileName = modelReferencePath(fileDataID, 'm2');
          } else if (fileName !== undefined) {
            fileName = replaceExtension(fileName, '.obj');
          } else {
            fileName = listfile.formatUnknownFile(fileDataID, '.obj');
          }

          let m2Path: string;
          if (config.enableSharedChildren) m2Path = getExportPath(fileName);
          else m2Path = replaceFile(out, fileName);

          if (!directModels && !doodadCache.has(fileDataID)) {
            const data = await getCasc().getFile(fileDataID);
            const modelMagic = data.readUInt32LE();
            data.seek(0);
            if (modelMagic === constants.MAGIC.MD21) {
              const m2Export = new M2Exporter(data, undefined, fileDataID);
              await m2Export.exportAsOBJ(m2Path, undefined, config.modelsExportCollision, progress);
            } else if (modelMagic === constants.MAGIC.M3DT) {
              write('Skipping M3 doodad %d (M3 export not supported natively)', fileDataID);
            }
            doodadCache.add(fileDataID);
          }

          let modelPath = path.relative(outDir, m2Path);
          if (useAbsolute === true) modelPath = path.resolve(outDir, modelPath);
          if (usePosix) modelPath = win32ToPosix(modelPath);

          csv.addRow({
            ModelFile: modelPath,
            PositionX: doodad.position[0],
            PositionY: doodad.position[1],
            PositionZ: doodad.position[2],
            RotationW: doodad.rotation[3],
            RotationX: doodad.rotation[0],
            RotationY: doodad.rotation[1],
            RotationZ: doodad.rotation[2],
            ScaleFactor: doodad.scale,
            DoodadSet: set.name,
            FileDataID: fileDataID,
          });
        } catch (e) {
          write('Failed to load doodad %d for %s: %s', fileDataID, set.name, (e as Error).message);
        }
      }
    }

    await csv.write();
  }

  /**
   * Export the WMO model as a WaveFront OBJ.
   */
  async exportAsOBJ(out: string, fileManifest?: FileManifestEntry[], progress: ExportProgress | undefined = undefined): Promise<void> {
    const obj = new OBJWriter(out);
    const mtl = new MTLWriter(replaceExtension(out, '.mtl'));

    const config = wowConfig;

    const groupMask = this.groupMask;

    const wmoName = path.basename(out, '.obj');
    obj.setName(wmoName);

    write('Exporting WMO model %s as OBJ: %s', wmoName, out);

    const wmo = this.wmo;
    wmo.load();

    const texMaps = await this.exportTextures(out, mtl, false, progress);

    const materialMap = texMaps.materialMap;
    const textureMap = texMaps.textureMap;

    for (const [texFileDataID, texInfo] of textureMap) fileManifest?.push({ type: 'PNG', fileDataID: texFileDataID, file: texInfo.matPath });

    const groups: WMOLoader[] = [];
    let nInd = 0;
    let maxLayerCount = 0;

    let mask: Set<number> | undefined;

    // Map our user-facing group mask to a WMO mask.
    if (groupMask) {
      mask = new Set();
      for (const group of groupMask) {
        if (group.checked) {
          // Add the group index to the mask.
          mask.add(group.groupIndex);
        }
      }
    }

    // Iterate over the groups once to calculate the total size of our
    // vertex/normal/uv arrays allowing for pre-allocation.
    for (let i = 0, n = wmo.groupCount!; i < n; i++) {
      const group = await wmo.getGroup(i);

      // Skip empty groups.
      if (!group.renderBatches || group.renderBatches.length === 0) continue;

      // Skip masked groups.
      if (mask && !mask.has(i)) continue;

      // 3 verts per indices.
      nInd += group.vertices!.length / 3;

      // UV counts vary between groups, allocate for the max.
      maxLayerCount = Math.max(group.uvs!.length, maxLayerCount);

      // Store the valid groups for quicker iteration later.
      groups.push(group);
    }

    // Restrict to first UV layer if additional UV layers are not enabled.
    if (!config.modelsExportUV2) maxLayerCount = Math.min(maxLayerCount, 1);

    const vertsArray = new Array<number>(nInd * 3);
    const normalsArray = new Array<number>(nInd * 3);
    const uvArrays = new Array<number[]>(maxLayerCount);

    // Create all necessary UV layer arrays.
    for (let i = 0; i < maxLayerCount; i++) uvArrays[i] = new Array(nInd * 2);

    // Iterate over groups again and fill the allocated arrays.
    let indOfs = 0;
    let groupIndex = 0;
    for (const group of groups) {
      groupIndex++;
      progress?.setLabel(`${wmoName} WMO groups`, groupIndex, groups.length);
      progress?.advance(1);
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
        obj.addMesh(groupName + bI, indices, materialMap.get(matID));
      }

      indOfs += indCount;
    }

    obj.setVertArray(vertsArray);
    obj.setNormalArray(normalsArray);

    for (const arr of uvArrays) obj.addUVArray(arr);

    await this.exportDoodadPlacementCsv(out, config, progress, false);
    const csvPath = placementCsvPath(out);
    if (await outputFileExists(csvPath)) {
      fileManifest?.push({ type: 'PLACEMENT', fileDataID: this.wmo.fileDataID!, file: csvPath });
    }

    if (!mtl.isEmpty) obj.setMaterialLibrary(path.basename(mtl.out));

    await obj.write(config.overwriteFiles);
    fileManifest?.push({ type: 'OBJ', fileDataID: this.wmo.fileDataID!, file: obj.out });

    await mtl.write(config.overwriteFiles);
    fileManifest?.push({ type: 'MTL', fileDataID: this.wmo.fileDataID!, file: mtl.out });

    if (config.exportWMOMeta) {
      const json = new JSONWriter(replaceExtension(out, '.json'));
      json.addProperty('fileType', 'wmo');
      json.addProperty('fileDataID', wmo.fileDataID);
      json.addProperty('fileName', wmo.fileName);
      json.addProperty('version', wmo.version);
      json.addProperty('counts', {
        material: wmo.materialCount,
        group: wmo.groupCount,
        portal: wmo.portalCount,
        light: wmo.lightCount,
        model: wmo.modelCount,
        doodad: wmo.doodadCount,
        set: wmo.setCount,
        lod: wmo.lodCount,
      });

      json.addProperty('portalVertices', wmo.portalVertices);
      json.addProperty('portalInfo', wmo.portalInfo);
      json.addProperty('portalMapObjectRef', wmo.mopr);
      json.addProperty('ambientColor', wmo.ambientColor);
      json.addProperty('areaTableID', wmo.areaTableID);
      json.addProperty('boundingBox1', wmo.boundingBox1);
      json.addProperty('boundingBox2', wmo.boundingBox2);
      json.addProperty('fog', wmo.fogs);
      json.addProperty('flags', wmo.flags);

      const groupsMeta = new Array<unknown>(wmo.groups!.length);
      for (let i = 0, n = wmo.groups!.length; i < n; i++) {
        const group = wmo.groups![i];
        groupsMeta[i] = {
          groupName: wmo.groupNames![group.nameOfs!],
          groupDescription: wmo.groupNames![group.descOfs!],
          enabled: !mask || mask.has(i),
          version: group.version,
          flags: group.flags,
          ambientColor: group.ambientColor,
          boundingBox1: group.boundingBox1,
          boundingBox2: group.boundingBox2,
          numPortals: group.numPortals,
          numBatchesA: group.numBatchesA,
          numBatchesB: group.numBatchesB,
          numBatchesC: group.numBatchesC,
          liquidType: group.liquidType,
          groupID: group.groupID,
          materialInfo: group.materialInfo,
          renderBatches: group.renderBatches,
          vertexColours: group.vertexColours,
          liquid: group.liquid,
        };
      }

      // Create a textures array and push every unique fileDataID from the
      // material stack, expanded with file name/path data for external QoL.
      const textures: unknown[] = [];
      const textureCache = new Set<number>();
      for (const material of wmo.materials!) {
        const materialTextures = [material.texture1, material.texture2, material.texture3];

        if (material.shader === 23) {
          materialTextures.push(material.color3);
          materialTextures.push(material.flags3);
          materialTextures.push(material.runtimeData[0]);
          materialTextures.push(material.runtimeData[1]);
          materialTextures.push(material.runtimeData[2]);
          materialTextures.push(material.runtimeData[3]);
        }

        for (const materialTexture of materialTextures) {
          if (materialTexture === 0 || textureCache.has(materialTexture)) continue;

          const textureEntry = textureMap.get(materialTexture);

          textureCache.add(materialTexture);
          textures.push({
            fileDataID: materialTexture,
            fileNameInternal: listfile.getByID(materialTexture),
            fileNameExternal: textureEntry?.matPathRelative,
            mtlName: textureEntry?.matName,
          });
        }
      }

      json.addProperty('groups', groupsMeta);
      json.addProperty('groupNames', Object.values(wmo.groupNames!));
      json.addProperty('groupInfo', wmo.groupInfo);
      json.addProperty('textures', textures);
      json.addProperty('materials', wmo.materials);
      json.addProperty('doodadSets', wmo.doodadSets);
      json.addProperty('fileDataIDs', wmo.fileDataIDs);
      json.addProperty('doodads', wmo.doodads);
      json.addProperty('groupIDs', wmo.groupIDs);

      await json.write(config.overwriteFiles);
      fileManifest?.push({ type: 'META', fileDataID: this.wmo.fileDataID!, file: json.out });
    }
  }

  /** Clear the WMO exporting cache. */
  static clearCache(): void {
    doodadCache.clear();
  }
}

export default WMOExporter;

/**
 * M2 model exporter, ported from wow.export (src/js/3D/exporters/M2Exporter.js).
 *
 * Differences from the original:
 * - No ExportHelper cancellation (headless, exports are never cancelled).
 * - File output goes through the pluggable output sink (disk or memory).
 * - GLTF export is omitted (unused by wow-converter's pipeline).
 */
import path from 'path';

import { profileScope } from '@/lib/export-profile';
import { write } from '@/lib/wow/log';
import { getCasc } from '@/lib/wow/server/runtime';

import * as listfile from '../../archive/casc/listfile';
import { BLPImage } from '../../formats/blp/blp';
import { BufferWrapper } from '../../formats/buffer';
import type { M2Track } from '../../formats/m2/m2-generics';
import { M2Loader } from '../../formats/m2/m2-loader';
import type { M2Attachment } from '../../formats/m2/m2-types';
import { SKELLoader } from '../../formats/m2/skel-loader';
import { Texture } from '../../formats/m2/texture';
import { wowConfig } from '../../server/config';
import type { ExportProgress } from '../export-progress';
import type {
  FileManifestEntry, GeosetMaskEntry, SkeletonLike, TextureManifestEntry, VariantTexture,
} from '../types/model-export';
import {
  getExportPath, replaceExtension, win32ToPosix,
} from '../writers/export-helper';
import { JSONWriter } from '../writers/json-writer';
import { MTLWriter } from '../writers/mtl-writer';
import { OBJWriter } from '../writers/obj-writer';
import { outputFileExists, writeOutputFile } from '../writers/output-sink';
import { getGeosetName } from './geoset-mapper';

export type {
  FileManifestEntry, GeosetMaskEntry, SkeletonLike, TextureManifestEntry, VariantTexture,
} from '../types/model-export';

interface DataTextureInfo {
  dataURI: string;
  filename: string | null;
}

export class M2Exporter {
  m2: M2Loader;

  fileDataID: number;

  variantTextures: VariantTexture[];

  dataTextures = new Map<number, DataTextureInfo>();

  excludedAnimIds = new Set<number>();

  geosetMask?: GeosetMaskEntry[];

  private skel?: SKELLoader;

  private parent_skel?: SKELLoader;

  constructor(data: BufferWrapper, variantTextures: VariantTexture[] | undefined, fileDataID: number) {
    this.m2 = new M2Loader(data);
    this.fileDataID = fileDataID;
    this.variantTextures = variantTextures ?? [];
  }

  setExcludedAnimIds(excludedAnimIds: Iterable<number>): void {
    this.excludedAnimIds = new Set(excludedAnimIds);
  }

  /** Set the mask array used for geoset control. */
  setGeosetMask(mask: GeosetMaskEntry[] | undefined): void {
    this.geosetMask = mask;
  }

  /** Register an additional data texture (e.g. composited character skin). */
  addURITexture(out: number, dataURI: string, filename: string | null = null): void {
    this.dataTextures.set(out, { dataURI, filename });
  }

  resetURITextures(): void {
    this.dataTextures.clear();
  }

  /**
   * Export the textures for this M2 model.
   * @returns Map keyed by fileDataID (or data-texture key) -> texture manifest info.
   */
  async exportTextures(
    out: string,
    raw = false,
    mtl: MTLWriter | null = null,
    fullTexPaths = false,
    progress: ExportProgress | undefined = undefined,
  ): Promise<Map<VariantTexture, TextureManifestEntry>> {
    const config = wowConfig;
    const validTextures = new Map<VariantTexture, TextureManifestEntry>();

    if (!config.modelsExportTextures) return validTextures;

    await this.m2.load();

    const useAlpha = config.modelsExportAlpha;
    const usePosix = config.pathFormat === 'posix';

    let textureIndex = 0;
    let progressTextures = 0;
    const modelLabel = path.basename(out);

    // Export data textures first.
    for (const [textureName, dataTextureInfo] of this.dataTextures) {
      try {
        const { dataURI, filename } = dataTextureInfo;

        // Use original filename/path if available; otherwise fall back to generic name next to OBJ
        let texPath: string;
        let texFile: string;
        let matName: string;

        if (filename) {
          // Preserve original CASC path under export directory and update MTL to point to it relatively
          const originalPath = filename.replace(/\\/g, '/');
          const fileNamePNG = replaceExtension(originalPath, '.png');
          texPath = getExportPath(fileNamePNG);
          texFile = path.relative(out, texPath);
          matName = `mat_${path.basename(fileNamePNG, '.png')}`;
        } else {
          // Fall back to generic naming colocated with the OBJ
          texFile = `data-${textureName}.png`;
          texPath = path.join(out, texFile);
          matName = `mat_${textureName}`;
        }

        if (config.overwriteFiles || !await outputFileExists(texPath)) {
          const data = BufferWrapper.fromBase64(dataURI.replace(/^data[^,]+,/, ''));
          write('Exporting data texture %d -> %s', textureName, texPath);
          await writeOutputFile(texPath, data.raw);
        } else {
          write('Skipping data texture export %s (file exists, overwrite disabled)', texPath);
        }

        if (usePosix) texFile = win32ToPosix(texFile);

        mtl?.addMaterial(matName, texFile);
        // Use 'data-' + textureName as the key since that's what the M2 model expects
        validTextures.set(`data-${textureName}`, {
          matName: fullTexPaths ? texFile : matName,
          matPathRelative: texFile,
          matPath: texPath,
        });
        progressTextures++;
        progress?.setLabel(`${modelLabel} M2 textures`, progressTextures);
        progress?.advance(1);
      } catch (e) {
        write('Failed to export data texture %d for M2: %s', textureName, e instanceof Error ? e.message : String(e));
      }
    }

    for (const texture of this.m2.textures) {
      const textureType = this.m2.textureTypes[textureIndex];
      let texFileDataID = texture.fileDataID;

      // Skip data textures in this section as they're already processed in the data textures section
      if (this.dataTextures.has(textureType)) {
        textureIndex++;
        continue;
      }

      if (textureType > 0) {
        let targetFileDataID: VariantTexture = 0;

        if (textureType >= 11 && textureType < 14) {
          // Creature textures.
          targetFileDataID = this.variantTextures[textureType - 11];
        } else if (textureType > 1 && textureType < 5) {
          targetFileDataID = this.variantTextures[textureType - 2];
        }

        // Only override if a valid replacement is provided; otherwise keep original.
        const hasValidReplacement = (typeof targetFileDataID === 'string') || (Number.isInteger(targetFileDataID) && targetFileDataID > 0);
        if (hasValidReplacement) {
          texFileDataID = targetFileDataID;
          // Backward patch the variant texture into the M2 instance so that
          // the MTL exports with the correct texture once we swap it here.
          texture.fileDataID = targetFileDataID;
        }
      }

      if ((typeof texFileDataID === 'string' && texFileDataID.startsWith('data-')) || (typeof texFileDataID === 'number' && !Number.isNaN(texFileDataID) && texFileDataID > 0)) {
        try {
          let texFile = texFileDataID + (raw ? '.blp' : '.png');
          let texPath = path.join(out, texFile);

          // Default MTL name to the file ID (prefixed for Maya).
          let matName = `mat_${texFileDataID}`;
          let fileName = typeof texFileDataID === 'number' ? listfile.getByID(texFileDataID) : undefined;

          // For data textures, texFileDataID might be a base filename instead of a fileDataID
          if (fileName === undefined && typeof texFileDataID === 'string' && !texFileDataID.startsWith('data-')) {
            // This is likely a data texture with a proper filename
            fileName = `${texFileDataID}.blp`;
            matName = `mat_${texFileDataID}`;
          } else if (fileName !== undefined) {
            matName = `mat_${path.basename(fileName.toLowerCase(), '.blp')}`;

            // Remove spaces from material name for MTL compatibility.
            if (config.removePathSpaces) matName = matName.replace(/\s/g, '');
          }

          // Map texture files relative to its own path.
          if (config.enableSharedTextures) {
            if (fileName !== undefined) {
              // Replace BLP extension with PNG.
              if (raw === false) fileName = replaceExtension(fileName, '.png');
            } else {
              // Handle unknown files.
              fileName = `unknown/${texFile}`;
            }

            texPath = getExportPath(fileName);
            texFile = path.relative(out, texPath);
          }

          if (config.overwriteFiles || !await outputFileExists(texPath)) {
            const data = await getCasc().getFile(texFileDataID as number);
            write('Exporting M2 texture %d -> %s', texFileDataID, texPath);

            if (raw === true) {
              // Write raw BLP files.
              await writeOutputFile(texPath, data.raw);
            } else {
              // Convert BLP to PNG.
              const blp = new BLPImage(data);
              await writeOutputFile(texPath, blp.toPNG(useAlpha ? 0b1111 : 0b0111).raw);
            }
          } else {
            write('Skipping M2 texture export %s (file exists, overwrite disabled)', texPath);
          }

          if (usePosix) texFile = win32ToPosix(texFile);

          mtl?.addMaterial(matName, texFile);
          validTextures.set(texFileDataID, {
            matName: fullTexPaths ? texFile : matName,
            matPathRelative: texFile,
            matPath: texPath,
          });
          progressTextures++;
          progress?.setLabel(`${modelLabel} M2 textures`, progressTextures);
          progress?.advance(1);
        } catch (e) {
          write('Failed to export texture %d for M2: %s', texFileDataID, e instanceof Error ? e.message : String(e));
        }
      }

      textureIndex++;
    }

    return validTextures;
  }

  /**
   * Export the M2 model as a WaveFront OBJ.
   */
  async exportAsOBJ(
    out: string,
    fileManifest?: FileManifestEntry[],
    exportCollision?: boolean,
    progress: ExportProgress | undefined = undefined,
  ): Promise<void> {
    const exportCollisionMeshes = exportCollision ?? false;
    await profileScope('m2.load', async () => {
      await this.m2.load();
    });
    const skin = await profileScope('m2.getSkin', () => this.m2.getSkin(0));

    const config = wowConfig;
    const exportMeta = config.exportM2Meta;
    const exportBones = config.exportM2Bones;

    const obj = new OBJWriter(out);
    const mtl = new MTLWriter(replaceExtension(out, '.mtl'));

    const outDir = path.dirname(out);

    const model_name = path.basename(out, '.obj');
    obj.setName(model_name);

    write('Exporting M2 model %s as OBJ: %s', model_name, out);

    obj.setVertArray(this.m2.vertices);
    obj.setNormalArray(this.m2.normals);
    obj.addUVArray(this.m2.uv);

    if (config.modelsExportUV2) obj.addUVArray(this.m2.uv2);

    const validTextures = await profileScope('m2.exportTextures', () => this.exportTextures(outDir, false, mtl, false, progress));
    for (const [texFileDataID, texInfo] of validTextures) fileManifest?.push({ type: 'PNG', fileDataID: texFileDataID, file: texInfo.matPath });

    if (exportBones) {
      await profileScope('writeBones', async () => {
        const json = new JSONWriter(replaceExtension(out, '_bones.json'));

        if (this.m2.skeletonFileID) {
          if (!this.skel) {
            const skel_file = await getCasc().getFile(this.m2.skeletonFileID);
            this.skel = new SKELLoader(skel_file);
            this.skel.load();
            await this.skel.loadAnims();
          }

          const skel = this.skel;

          if (skel.parent_skel_file_id && skel.parent_skel_file_id > 0) {
            if (!this.parent_skel) {
              const parent_skel_file = await getCasc().getFile(skel.parent_skel_file_id);
              this.parent_skel = new SKELLoader(parent_skel_file);
              this.parent_skel.load();
              await this.parent_skel.loadAnims();
            }

            const parent_skel = this.parent_skel;
            if (!skel.bones || !parent_skel.bones) {
              throw new Error('Skeleton merge requires loaded bones on child and parent SKEL');
            }

            // This section is similar to M2Exporter.exportAsGLTF
            // Map of animation indices from child to parent.
            const animIndexMap = new Map<number, number>();

            for (let i = 0; i < skel.animations.length; i++) {
              const anim = skel.animations[i];
              for (let j = 0; j < parent_skel.animations.length; j++) {
                const parent_anim = parent_skel.animations[j];
                if (parent_anim.id === anim.id && parent_anim.variationIndex === anim.variationIndex) {
                  animIndexMap.set(i, j);
                  break;
                }
              }
            }

            // Override parent bone animation data with child skeleton animation data if animation is present on both.
            for (let i = 0; i < skel.bones.length; i++) {
              if (i >= parent_skel.bones.length) break;

              const bone = skel.bones[i];
              const parentBone = parent_skel.bones[i];

              for (const anim of animIndexMap) {
                if (bone.translation.timestamps.length > anim[0] && parentBone.translation.timestamps.length > anim[1]
                && bone.translation.values.length > anim[0] && parentBone.translation.values.length > anim[1]) {
                  parent_skel.bones[i].translation.timestamps[anim[1]] = bone.translation.timestamps[anim[0]];
                  parent_skel.bones[i].translation.values[anim[1]] = bone.translation.values[anim[0]];
                }

                if (bone.rotation.timestamps.length > anim[0] && parentBone.rotation.timestamps.length > anim[1]
                && bone.rotation.values.length > anim[0] && parentBone.rotation.values.length > anim[1]) {
                  parent_skel.bones[i].rotation.timestamps[anim[1]] = bone.rotation.timestamps[anim[0]];
                  parent_skel.bones[i].rotation.values[anim[1]] = bone.rotation.values[anim[0]];
                }

                if (bone.scale.timestamps.length > anim[0] && parentBone.scale.timestamps.length > anim[1]
                && bone.scale.values.length > anim[0] && parentBone.scale.values.length > anim[1]) {
                  parent_skel.bones[i].scale.timestamps[anim[1]] = bone.scale.timestamps[anim[0]];
                  parent_skel.bones[i].scale.values[anim[1]] = bone.scale.values[anim[0]];
                }
              }
            }

            json.addProperty('bones', bonesExcludeAnimations(parent_skel, this.excludedAnimIds));
            if (parent_skel.animations) json.addProperty('animations', parent_skel.animations);
          } else {
            json.addProperty('bones', bonesExcludeAnimations(skel, this.excludedAnimIds));
            if (skel.animations) json.addProperty('animations', skel.animations);
          }
        } else {
          await this.m2.loadAnims();
          json.addProperty('bones', bonesExcludeAnimations(this.m2, this.excludedAnimIds));
          json.addProperty('animations', this.m2.animations);
        }

        json.addProperty('boneWeights', this.m2.boneWeights);
        json.addProperty('boneIndicies', this.m2.boneIndices);

        // wow.export leaves M2Loader.attachments undefined when the MD21 chunk is
        // empty, so its `m2.attachments || skel.attachments` falls through to the
        // SKEL. Our loader initialises it to [] (truthy), so check length instead.
        const attachments: M2Attachment[] = (this.m2.attachments?.length ? this.m2.attachments : undefined)
        ?? this.skel?.attachments ?? [];
        json.addProperty('attachments', attachments);

        await json.write(config.overwriteFiles, true);
        fileManifest?.push({ type: 'BONE_META', fileDataID: this.fileDataID, file: json.out });
      });
    }

    if (exportMeta) {
      await profileScope('writeMeta', async () => {
        const json = new JSONWriter(replaceExtension(out, '.json'));

        // Clone the submesh array and add a custom 'enabled' property
        // to indicate to external readers which submeshes are not included
        // in the actual geometry file.
        const subMeshes = new Array<unknown>(skin.subMeshes.length);
        for (let i = 0, n = subMeshes.length; i < n; i++) {
          const subMeshEnabled = !this.geosetMask || this.geosetMask[i].checked;
          subMeshes[i] = { enabled: subMeshEnabled, ...skin.subMeshes[i] };
        }

        // Clone M2 textures array and expand the entries to include internal
        // and external paths/names for external convenience. GH-208
        const textures = new Array<unknown>(this.m2.textures.length);
        for (let i = 0, n = textures.length; i < n; i++) {
          const texture = this.m2.textures[i];
          const texType = this.m2.textureTypes[i];
          let textureEntry = validTextures.get(texture.fileDataID);

          // Fallback for data textures whose fileDataID may be 0 or not mapped yet
          if (!textureEntry && this.dataTextures.has(texType)) textureEntry = validTextures.get(`data-${texType}`);

          textures[i] = {
            fileNameInternal: typeof texture.fileDataID === 'number' ? listfile.getByID(texture.fileDataID) : undefined,
            fileNameExternal: textureEntry?.matPathRelative,
            mtlName: textureEntry?.matName,
            ...texture,
          };
        }

        json.addProperty('fileType', 'm2');
        json.addProperty('fileDataID', this.fileDataID);
        json.addProperty('fileName', listfile.getByID(this.fileDataID));
        json.addProperty('internalName', this.m2.name);
        json.addProperty('textures', textures);
        json.addProperty('textureTypes', this.m2.textureTypes);
        json.addProperty('materials', this.m2.materials);
        json.addProperty('textureCombos', this.m2.textureCombos);
        json.addProperty('skeletonFileID', this.m2.skeletonFileID);
        json.addProperty('boneFileIDs', this.m2.boneFileIDs);
        json.addProperty('animFileIDs', this.m2.animFileIDs);
        json.addProperty('m2Animations', this.m2.animations);
        json.addProperty('colors', this.m2.colors);
        json.addProperty('textureWeights', this.m2.textureWeights);
        json.addProperty('transparencyLookup', this.m2.transparencyLookup);
        json.addProperty('textureTransforms', this.m2.textureTransforms);
        json.addProperty('textureTransformsLookup', this.m2.textureTransformsLookup);
        json.addProperty('boundingBox', this.m2.boundingBox);
        json.addProperty('boundingSphereRadius', this.m2.boundingSphereRadius);
        json.addProperty('collisionBox', this.m2.collisionBox);
        json.addProperty('collisionSphereRadius', this.m2.collisionSphereRadius);
        json.addProperty('lights', this.m2.lights || []);
        json.addProperty('skin', {
          subMeshes,
          textureUnits: skin.textureUnits,
          fileName: skin.fileName,
          fileDataID: skin.fileDataID,
        });
        json.addProperty('cameras', this.m2.cameras || []);
        json.addProperty('cameraLookup', this.m2.cameraLookup || []);
        json.addProperty('ribbonEmitters', this.m2.ribbonEmitters || []);
        json.addProperty('particleEmitters', this.m2.particleEmitters || []);

        await json.write(config.overwriteFiles);
        fileManifest?.push({ type: 'META', fileDataID: this.fileDataID, file: json.out });
      });
    }

    await profileScope('writeOBJ', async () => {
      for (let mI = 0, mC = skin.subMeshes.length; mI < mC; mI++) {
        if (this.geosetMask && !this.geosetMask[mI].checked) continue;

        const mesh = skin.subMeshes[mI];
        const verts = new Array<number>(mesh.triangleCount);
        for (let vI = 0; vI < mesh.triangleCount; vI++) verts[vI] = skin.indices[skin.triangles[mesh.triangleStart + vI]];

        let texture: Texture | null = null;
        const texUnit = skin.textureUnits.find((tex) => tex.skinSectionIndex === mI);
        if (texUnit) {
          texture = this.m2.textures[this.m2.textureCombos[texUnit.textureComboIndex]];
          // Patch for data textures
          const texType = this.m2.textureTypes[this.m2.textureCombos[texUnit.textureComboIndex]];
          if (this.dataTextures.has(texType)) texture.fileDataID = `data-${texType}`;
        }

        let matName: string | undefined;
        if (texture?.fileDataID && validTextures.has(texture.fileDataID)) matName = validTextures.get(texture.fileDataID)!.matName;

        obj.addMesh(getGeosetName(mI, mesh.submeshID), verts, matName);
      }

      if (!mtl.isEmpty) obj.setMaterialLibrary(path.basename(mtl.out));

      await obj.write(config.overwriteFiles);
      fileManifest?.push({ type: 'OBJ', fileDataID: this.fileDataID, file: obj.out });

      await mtl.write(config.overwriteFiles);
      fileManifest?.push({ type: 'MTL', fileDataID: this.fileDataID, file: mtl.out });

      if (exportCollisionMeshes) {
        const phys = new OBJWriter(replaceExtension(out, '.phys.obj'));
        phys.setVertArray(this.m2.collisionPositions);
        phys.setNormalArray(this.m2.collisionNormals);
        phys.addMesh('Collision', this.m2.collisionIndices);

        await phys.write(config.overwriteFiles);
        fileManifest?.push({ type: 'PHYS_OBJ', fileDataID: this.fileDataID, file: phys.out });
      }
    });
  }
}

/**
 * Strip animation keyframes belonging to excluded animation IDs from bones.
 * Ported from wow.export's bonesExcludeAnimations.
 * (Exported for reuse by the direct M2->MDL path in src/lib/m2mdl.)
 */
export function bonesExcludeAnimations(skel: SkeletonLike, excludedAnimIds: Set<number>): SkeletonLike['bones'] {
  // Fast-path: nothing to exclude
  if (!excludedAnimIds || excludedAnimIds.size === 0) return skel.bones;
  if (!skel.bones) return skel.bones;

  const animations = skel.animations;
  // Map animID -> index into timestamp/value arrays
  const animIdxMap = new Map(animations.map((anim, idx) => [anim.id, idx]));

  // Pre-compute the indices we actually need to strip out and turn into a Set for O(1) look-ups
  const excludedIndices = new Set(
    [...excludedAnimIds]
      .map((id) => animIdxMap.get(id))
      .filter((idx): idx is number => idx !== undefined),
  );

  // If none of the requested IDs exist on this skeleton, bail early
  if (excludedIndices.size === 0) return skel.bones;

  // If the bone belongs to some global sequence, we can't remove the timestamps/values
  const noGlobalSeq = 65535;
  const modifyArr = <T>(arr: T[][], globalSeq: number): T[][] => arr.map((a, idx) => (globalSeq === noGlobalSeq && excludedIndices.has(idx) ? [] : a));

  const modifyTrack = <V>(track: M2Track<V>): M2Track<V> => ({
    ...track,
    timestamps: modifyArr(track.timestamps, track.globalSeq),
    values: modifyArr(track.values, track.globalSeq),
  });

  return skel.bones.map((bone) => ({
    ...bone,
    translation: modifyTrack(bone.translation),
    rotation: modifyTrack(bone.rotation),
    scale: modifyTrack(bone.scale),
  }));
}

export default M2Exporter;

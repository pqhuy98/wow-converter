import { readFileSync } from 'fs';
import { dirname, join, relative } from 'path';

import { BlizzardNull } from '../../constants';
import { Config } from '../../converter/common';
import { QuaternionRotation, Vector3 } from '../../math/common';
import { AnimationFile } from '../animation/animation';
import {
  Geoset, GlobalSequence, m2BlendModeToWc3FilterMode, Material, MDL, Texture, TextureAnim, wowToWc3Interpolation,
} from '../mdl/mdl';
import { MetadataFile } from './interface';

namespace Data {
  export interface Texture {
    fileNameInternal: string
    fileNameExternal: string
    mtlName: string
    flags: number
    fileDataID: number
  }

  export interface Material {
    flags: number
    blendingMode: number
  }

  export interface AnimFileId {
    animID: number
    subAnimID: number
    fileDataID: number
  }

  export interface Color {
    color: {
      globalSeq: number
      interpolation: number
      timestamps: number[][]
      values: Vector3[][]
    }
    alpha: {
      globalSeq: number
      interpolation: number
      timestamps: number[][]
      values: number[][]
    }
  }

  export interface TextureWeight {
    globalSeq: number
    interpolation: number
    timestamps: number[][]
    values: number[][]
  }

  export interface TextureTransform {
    translation: Translation
    rotation: Rotation
    scaling: Scaling
  }

  export interface Translation {
    globalSeq: number
    interpolation: number
    timestamps: number[][]
    values: Vector3[][]
  }

  export interface Rotation {
    globalSeq: number
    interpolation: number
    timestamps: number[][]
    values: QuaternionRotation[][]
  }

  export interface Scaling {
    globalSeq: number
    interpolation: number
    timestamps: number[][]
    values: Vector3[][]
  }

  export interface BoundingBox {
    min: Vector3
    max: Vector3
  }

  export interface CollisionBox {
    min: Vector3
    max: Vector3
  }

  export interface Skin {
    subMeshes: SubMesh[]
    textureUnits: TextureUnit[]
    fileName: string
    fileDataID: number
  }

  export interface SubMesh {
    enabled: boolean
    submeshID: number
    level: number
    vertexStart: number
    vertexCount: number
    triangleStart: number
    triangleCount: number
    boneCount: number
    boneStart: number
    boneInfluences: number
    centerBoneIndex: number
    centerPosition: number[]
    sortCenterPosition: number[]
    sortRadius: number
  }

  export interface TextureUnit {
    flags: number
    priority: number
    shaderID: number
    skinSectionIndex: number
    flags2: number
    colorIndex: number
    materialIndex: number
    materialLayer: number
    textureCount: number
    textureComboIndex: number
    textureCoordComboIndex: number
    textureWeightComboIndex: number
    textureTransformComboIndex: number
  }
}

export class M2MetadataFile implements MetadataFile {
  fileType: string;

  fileDataID: number;

  fileName: string;

  internalName: string;

  textures: Data.Texture[];

  textureTypes: number[];

  materials: Data.Material[];

  textureCombos: number[];

  animFileIDs: Data.AnimFileId[];

  colors: Data.Color[];

  textureWeights: Data.TextureWeight[];

  transparencyLookup: number[];

  textureTransforms: Data.TextureTransform[];

  textureTransformsLookup: number[];

  boundingBox: Data.BoundingBox;

  boundingSphereRadius: number;

  collisionBox: Data.CollisionBox;

  collisionSphereRadius: number;

  skin: Data.Skin;

  isLoaded = false;

  constructor(private filePath: string, private options: Config) {
    try {
      Object.assign(this, JSON.parse(readFileSync(this.filePath, 'utf-8')));
      if (this.fileType === 'm2' && this.textures.every((tex) => tex.fileNameExternal && tex.fileNameInternal)) {
        // ADT files (terrain) won't have metadata JSON.
        // WMO files (world object)'s metadata file is not yet supported.
        // Therefore fallback to heuristic OBJ textures/materials decoding.
        // Heuristic OBJ textures/materials uses `guessFilterMode` which is not always correct.
        this.isLoaded = true;
      } else {
        // Metadata of other files (WMO) are not supported.
        this.isLoaded = false;
      }
    } catch (e) {
      if (e.code === 'ENOENT') {
        // file not exist, do not throw.
        return;
      }
      throw e;
    }
  }

  extractMDLGeosetAnim(animFile: AnimationFile, geosets: Geoset[]): MDL['geosetAnims'] {
    if (!this.isLoaded) {
      throw new Error(`Metadata file is not loaded: ${this.filePath}`);
    }

    const textureUnits = this.skin.textureUnits.filter((t) => t.colorIndex !== 2 ** 16 - 1);
    const result: MDL['geosetAnims'] = [];
    textureUnits.forEach((tu) => {
      const wowColor = this.colors[tu.colorIndex];
      if (!geosets[tu.skinSectionIndex]) {
        console.log('geoset not found', tu.skinSectionIndex, geosets.length);
        return;
      }

      const geosetAnim: MDL['geosetAnims'][number] = {
        id: 0,
        geoset: geosets[tu.skinSectionIndex],
        color: {
          static: true,
          value: wowColor.color.values[0][0],
        },
        alpha: {
          static: true,
          value: wowColor.alpha.values[0][0],
        },
      };

      // Color
      if (wowColor.color.timestamps.length > 1 || wowColor.color.timestamps[0].length > 1) {
        geosetAnim.color = {
          interpolation: wowToWc3Interpolation(wowColor.alpha.interpolation),
          keyFrames: new Map(),
        };
        let accumTime = 0;

        animFile.animations!.forEach((anim, animId) => {
          const timestamps = wowColor.color.timestamps[animId] ?? wowColor.color.timestamps[0];
          const values = wowColor.color.values[animId] ?? wowColor.color.values[0];

          let maxTimestamp = -Infinity;
          values.forEach((value, i) => {
            if (!('keyFrames' in geosetAnim.color!)) throw new Error('Field keyframes is missing in geosetAnim.color. This should never happen.');
            geosetAnim.color.keyFrames.set(timestamps[i] + accumTime, value);
            maxTimestamp = Math.max(maxTimestamp, timestamps[i] + accumTime);
          });
          if (maxTimestamp >= -1) {
            if (!('keyFrames' in geosetAnim.color!)) throw new Error('Field keyframes is missing in geosetAnim.color. This should never happen.');
            geosetAnim.color.keyFrames.set(accumTime + anim.duration, [...geosetAnim!.color!.keyFrames.get(maxTimestamp)!]);
          }
          accumTime += anim.duration + 1;
        });
      }

      // Alpha
      if (wowColor.alpha.timestamps.length > 1 || wowColor.alpha.timestamps[0].length > 1) {
        geosetAnim.alpha = {
          interpolation: wowToWc3Interpolation(wowColor.alpha.interpolation),
          keyFrames: new Map(),
        };
        let accumTime = 0;

        animFile.animations!.forEach((anim, animId) => {
          const timestamps = wowColor.alpha.timestamps[animId] ?? wowColor.alpha.timestamps[0];
          const values = wowColor.alpha.values[animId] ?? wowColor.alpha.values[0];

          let maxTimestamp = -Infinity;
          values.forEach((value, i) => {
            if (!('keyFrames' in geosetAnim.alpha!)) throw new Error('Field keyframes is missing in geosetAnim.alpha. This should never happen.');
            geosetAnim.alpha.keyFrames.set(timestamps[i] + accumTime, value);
            maxTimestamp = Math.max(maxTimestamp, timestamps[i] + accumTime);
          });
          if (maxTimestamp >= -1) {
            if (!('keyFrames' in geosetAnim.alpha!)) throw new Error('Field keyframes is missing in geosetAnim.alpha. This should never happen.');
            geosetAnim.alpha.keyFrames.set(accumTime + anim.duration, geosetAnim.alpha!.keyFrames.get(maxTimestamp)!);
          }
          accumTime += anim.duration + 1;
        });
      }

      result.push(geosetAnim);
    });
    return result;
  }

  extractMDLTexturesMaterials(
    texturePrefix: string,
    numGeosets: number,
    animFile: AnimationFile,
    globalSequences: GlobalSequence[],
  ): Pick<MDL, 'textures' | 'materials' | 'textureAnims'> & {geosetToMat: Map<number, Material>} {
    if (!this.isLoaded) {
      throw new Error(`Metadata file is not loaded: ${this.filePath}`);
    }

    // Textures
    const textures: Texture[] = this.textures.map((tex) => ({
      id: 0,
      image: tex.fileNameExternal
        ? join(texturePrefix, relative(this.options.wowExportPath, join(dirname(this.filePath), tex.fileNameExternal.replace('.png', '.blp'))))
        : '',
      wrapHeight: (tex.flags & 1) > 0,
      wrapWidth: (tex.flags & 2) > 0,
    }));

    // Texture anims
    const globalSequenceMap = new Map<number, GlobalSequence>(globalSequences.map((gs) => [gs.id, gs]));
    function getGlobalSeq(id: number) {
      if (!globalSequenceMap.has(id)) {
        const newGs: GlobalSequence = {
          id, duration: 1,
        };
        globalSequenceMap.set(id, newGs);
        globalSequences.push(newGs);
      }
      return globalSequenceMap.get(id);
    }

    const textureAnims: MDL['textureAnims'] = this.textureTransforms.map((transform) => {
      const mdlTxAnim: TextureAnim = {
        id: 0,
        translation: {
          interpolation: wowToWc3Interpolation(transform.translation.interpolation),
          globalSeq: transform.translation.globalSeq !== BlizzardNull ? getGlobalSeq(transform.translation.globalSeq) : undefined,
          keyFrames: new Map(),
        },
        rotation: {
          interpolation: wowToWc3Interpolation(transform.rotation.interpolation),
          globalSeq: transform.rotation.globalSeq !== BlizzardNull ? getGlobalSeq(transform.rotation.globalSeq) : undefined,
          keyFrames: new Map(),
        },
        scaling: {
          interpolation: wowToWc3Interpolation(transform.scaling.interpolation),
          globalSeq: transform.scaling.globalSeq !== BlizzardNull ? getGlobalSeq(transform.scaling.globalSeq) : undefined,
          keyFrames: new Map(),
        },
      };

      // Translation
      let accumTime = 0;
      transform.translation.timestamps.forEach((timestamps, animId) => {
        let maxTimestamp = -Infinity;
        timestamps.forEach((timestamp, timestampI) => {
          const [x, y, z] = transform.translation.values[animId][timestampI];
          mdlTxAnim.translation!.keyFrames.set(timestamp + accumTime, [x, y, z]);
          maxTimestamp = Math.max(maxTimestamp, timestamp + accumTime);
        });
        if (maxTimestamp >= -1 && !mdlTxAnim.translation?.globalSeq) {
          mdlTxAnim.translation!.keyFrames.set(accumTime + animFile.animations![animId].duration, mdlTxAnim.translation!.keyFrames.get(maxTimestamp)!);
        }
        accumTime += animFile.animations![animId].duration + 1;
      });
      // Rotation
      accumTime = 0;
      transform.rotation.timestamps.forEach((timestamps, animId) => {
        let maxTimestamp = -Infinity;
        timestamps.forEach((timestamp, timestampI) => {
          const [w, x, y, z] = transform.rotation.values[animId][timestampI];
          mdlTxAnim.rotation!.keyFrames.set(timestamp + accumTime, [w, -y, x, z]);
          maxTimestamp = Math.max(maxTimestamp, timestamp + accumTime);
        });
        if (maxTimestamp >= -1 && !mdlTxAnim.rotation?.globalSeq) {
          mdlTxAnim.rotation!.keyFrames.set(accumTime + animFile.animations![animId].duration, mdlTxAnim.rotation!.keyFrames.get(maxTimestamp)!);
        }
        accumTime += animFile.animations![animId].duration + 1;
      });

      // Scaling
      accumTime = 0;
      transform.scaling.timestamps.forEach((timestamps, animId) => {
        let maxTimestamp = -Infinity;
        timestamps.forEach((timestamp, timestampI) => {
          const [x, y, z] = transform.scaling.values[animId][timestampI];
          mdlTxAnim.scaling!.keyFrames.set(timestamp + accumTime, [x, z, y]);
          maxTimestamp = Math.max(maxTimestamp, timestamp + accumTime);
        });
        if (maxTimestamp >= -1 && !mdlTxAnim.scaling?.globalSeq) {
          mdlTxAnim.scaling!.keyFrames.set(accumTime + animFile.animations![animId].duration, mdlTxAnim.scaling!.keyFrames.get(maxTimestamp)!);
        }
        accumTime += animFile.animations![animId].duration + 1;
      });
      if (!mdlTxAnim.translation?.keyFrames.size) mdlTxAnim.translation = undefined;
      if (!mdlTxAnim.rotation?.keyFrames.size) mdlTxAnim.rotation = undefined;
      if (!mdlTxAnim.scaling?.keyFrames.size) mdlTxAnim.scaling = undefined;
      return mdlTxAnim;
    });

    // Materials
    const geosetMaterials: Material[] = Array(numGeosets).fill(0).map(() => ({
      id: 0,
      constantColor: true,
      layers: [],
    }));

    this.skin.textureUnits.forEach((tu) => {
      const geosetId = tu.skinSectionIndex;
      const textureId = this.textureCombos[tu.textureComboIndex];
      const material = this.materials[tu.materialIndex];
      const textAnimId = this.textureTransformsLookup[tu.textureTransformComboIndex];

      geosetMaterials[geosetId] && geosetMaterials[geosetId].layers.push({
        texture: textures[textureId],
        filterMode: m2BlendModeToWc3FilterMode(material.blendingMode),
        twoSided: (material.flags & 0x04) > 0, // https://wowdev.wiki/M2#Render_flags_and_blending_modes
        tvertexAnim: textAnimId !== BlizzardNull ? textureAnims[textAnimId] : undefined,
      });
    });

    // deduplicate materials
    function hashMaterial(mat: MDL['materials'][number]) {
      return JSON.stringify(mat);
    }

    const dedupedMaterials: Material[] = [];
    const geosetToMat = new Map(geosetMaterials.map((mat, i) => [i, mat]));
    const hashToMat = new Map<string, Material>();

    geosetMaterials.forEach((mat, geosetId) => {
      const hashKey = hashMaterial(mat);
      if (hashToMat.has(hashKey)) {
        // reuse existing material
        geosetToMat[geosetId] = hashToMat.get(hashKey)!;
      } else {
        // add new material
        dedupedMaterials.push(mat);
        geosetToMat[geosetId] = mat;
        hashToMat.set(hashKey, mat);
      }
    });

    return {
      textures,
      materials: dedupedMaterials,
      geosetToMat,
      textureAnims,
    };
  }

  objToSubmesh = new Map<number, number>();

  getSkinWeightIndex(geosetVertexIndex: number) {
    if (this.objToSubmesh.size === 0) {
      let idx = 0;
      this.skin.subMeshes.forEach((submesh) => {
        if (!submesh.enabled) return;
        for (let v = submesh.vertexStart; v < submesh.vertexStart + submesh.vertexCount; v++) {
          this.objToSubmesh.set(idx, v);
          idx++;
        }
      });
    }
    return this.objToSubmesh.get(geosetVertexIndex);
  }
}

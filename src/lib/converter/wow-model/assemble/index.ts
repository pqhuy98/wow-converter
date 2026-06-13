import chalk from 'chalk';
import _ from 'lodash';
import path from 'path';

import { hasTextureSource } from '@/lib/converter/common/texture-source';
import { exportAssetExists } from '@/lib/export-asset-store';
import { stripModelReferenceExt } from '@/lib/wow/export/model-reference-path';
import {
  Geoset, GeosetVertex, Matrix, SkinWeight,
} from '@/lib/formats/mdl/components/geoset';
import { Material } from '@/lib/formats/mdl/components/material';
import { Texture } from '@/lib/formats/mdl/components/texture';
import { MDL } from '@/lib/formats/mdl/mdl';
import { Config } from '@/lib/global-config';

import { AnimationFile } from '../bundle/animation';
import { M2MetadataFile } from '../bundle/metadata';
import { ObjMaterial } from '../bundle/mtl';
import { IFace, IGroup, IResult } from '../bundle/obj';
import { guessFilterMode } from '../bundle/utils';

const debug = false;

const ADT_UV_PADDING = 1 / 512;
const ADT_UV_EDGE_EPS = 1e-6;
function padUvEdgeAware(u: number, v: number): [number, number] {
  const uu = (u <= ADT_UV_EDGE_EPS) ? 0 : (u >= 1 - ADT_UV_EDGE_EPS) ? 1 : (u * (1 - 2 * ADT_UV_PADDING) + ADT_UV_PADDING);
  const vv = (v <= ADT_UV_EDGE_EPS) ? 0 : (v >= 1 - ADT_UV_EDGE_EPS) ? 1 : (v * (1 - 2 * ADT_UV_PADDING) + ADT_UV_PADDING);
  return [uu, vv];
}

/** Parsed inputs for MDL assembly, decoupled from where they came from (files or direct M2 parsing). */
export interface WowModelInputs {
  objFilePath: string;
  obj: IResult;
  mtl: { materials: ObjMaterial[] };
  animation: AnimationFile;
  metadata: M2MetadataFile;
}

/** Shared assembly core: parsed structs -> MDL. */
export async function assembleWowModel(inputs: WowModelInputs, config: Config): Promise<{ mdl: MDL; texturePaths: Set<string> }> {
  const {
    objFilePath, obj, mtl, animation, metadata,
  } = inputs;
  const isAdtModel = objFilePath.includes('adt_');

  const mdl = new MDL({
    formatVersion: 1000,
    name: path.join(
      config.assetPrefix,
      path.relative(config.exportAssetDir, stripModelReferenceExt(objFilePath)),
    ),
  });
  metadata.bindMdl(mdl);

  if (obj.models.length === 0) {
    console.error(chalk.red('No models found in', objFilePath));
    return { mdl, texturePaths: new Set<string>() };
  }

  const groups = new Map<IGroup, IFace[]>();
  const geosetsGroups: [Geoset, IGroup][] = [];
  obj.models[0].faces.forEach((f) => {
    if (!groups.has(f.group)) {
      groups.set(f.group, []);
      mdl.geosets.push({
        id: 0,
        name: f.group.name,
        vertices: [],
        faces: [],
        matrices: mdl.bones.map((b) => ({ id: 0, bones: [b] })),
        minimumExtent: [0, 0, 0],
        maximumExtent: [0, 0, 0],
        boundsRadius: 0,
        material: undefined!,
        selectionGroup: 0,
        wowData: {
          submeshId: -1,
        },
      });
      geosetsGroups.push([mdl.geosets[mdl.geosets.length - 1], f.group]);
    }
    groups.get(f.group)!.push(f);
  });
  if (metadata.fileType === 'm2') {
    metadata.mapSubMeshesToMdlGeosets(mdl);
  }

  const parentDir = path.dirname(path.normalize(objFilePath.replaceAll('\\', '/')));

  const texturePaths = new Set<string>();

  const {
    submeshIdToMat, textures,
  } = metadata.extractMDLTexturesMaterials();
  mdl.textures = [];
  mdl.materials = [];
  for (const tex of metadata.textures) {
    if (!tex.fileNameExternal) continue;
    const absPath = path.join(parentDir, tex.fileNameExternal);
    if (!await exportAssetExists(absPath) && !hasTextureSource(path.relative(config.exportAssetDir, absPath))) {
      console.warn('Skipping texture not found', absPath, 'for model', objFilePath);
      continue;
    }
    const textureRelativePath = path.relative(config.exportAssetDir, absPath);
    texturePaths.add(textureRelativePath);
  }

  const mtlNameMap = new Map<string, Material>();

  const resolveGeosetMaterial = (submeshId: number, matName: string): MDL['materials'][number] => {
    const mtlMaterial = mtl.materials.find((m) => m.name === matName);
    const textureRelativePath = mtlMaterial ? path.relative(config.exportAssetDir, path.join(parentDir, mtlMaterial.map_Kd!)) : undefined;
    textureRelativePath && texturePaths.add(textureRelativePath);

    const protoMat = submeshIdToMat.get(submeshId);
    let mat = _.cloneDeep(protoMat);

    if (!mat && metadata.fileType === 'wmo') {
      const wmoMat = metadata.getWmoMaterialByMtlName(matName);
      if (wmoMat) mat = _.cloneDeep(wmoMat);
    }

    if (mat) {
      mat.layers.forEach((l, i) => {
        l.tvertexAnim = protoMat?.layers[i].tvertexAnim;
      });

      mat.layers.forEach((l) => {
        const blpPath = l.texture.image || (textureRelativePath ? path.join(config.assetPrefix, textureRelativePath.replace('.png', '.blp')) : '');
        l.texture = {
          ...l.texture,
          image: blpPath,
        };
        if (blpPath) {
          texturePaths.add(blpPath.replace('.blp', '.png').replace(new RegExp(`^${config.assetPrefix}\\\\`), ''));
        }
      });
    } else {
      if (mtlNameMap.has(matName)) {
        return mtlNameMap.get(matName)!;
      }

      if (!isAdtModel) {
        console.log(chalk.red('Warning: no material found for matName:', matName, 'submeshId:', submeshId));
      }

      textureRelativePath && texturePaths.add(textureRelativePath);
      const texture: Texture = {
        id: 0,
        image: textureRelativePath ? path.join(config.assetPrefix, textureRelativePath.replace('.png', '.blp')) : '',
        wrapHeight: !isAdtModel,
        wrapWidth: !isAdtModel,
        wowData: {
          type: 0,
          pngPath: textureRelativePath || '',
        },
      };
      const material: Material = {
        id: 0,
        constantColor: false,
        twoSided: false,
        layers: [
          {
            texture,
            filterMode: textureRelativePath && !isAdtModel ? guessFilterMode(textureRelativePath) : 'None',
            unshaded: false,
            sphereEnvMap: false,
            twoSided: false,
            unfogged: false,
            unlit: false,
            noDepthTest: false,
            noDepthSet: false,
            alpha: { static: true, value: 1 },
          },
        ],
      };
      mdl.textures.push(texture);
      mdl.materials.push(material);
      mtlNameMap.set(matName, material);
      return material;
    }

    return mat;
  };

  let mdlAnim: ReturnType<typeof animation.toMdl>;
  if (animation.isLoaded) {
    mdlAnim = animation.toMdl(mdl.globalSequences);
    mdl.bones = mdlAnim.bones;
    mdl.sequences = mdlAnim.sequences;
    mdl.wowAttachments = mdlAnim.wowAttachments;
  } else {
    mdl.bones = [{
      type: 'Bone',
      name: 'bone_default',
      flags: [],
      pivotPoint: [0, 0, 0],
    }];
  }

  if (mdl.sequences.length === 0) {
    mdl.sequences.push({
      name: 'Stand',
      data: {
        wowName: '', attackTag: '', wc3Name: 'Stand', wowVariant: 0, wowFrequency: 0,
      },
      interval: [0, 1000],
      moveSpeed: 0,
      nonLooping: false,
      minimumExtent: [-1, -1, -1],
      maximumExtent: [1, 1, 1],
      boundsRadius: 1,
    });
  }

  const isM2Meta = metadata.fileType === 'm2' && Array.isArray(metadata.skin?.subMeshes);
  const submeshToId = isM2Meta ? new Map(metadata.skin.subMeshes.map((s, i) => [s, i])) : undefined;
  const enabledSubmeshes = isM2Meta ? metadata.skin.subMeshes.filter((s) => s.enabled) : [];

  let idx = 0;
  groups.forEach((faces) => {
    const i = idx;
    idx++;
    const [geoset] = geosetsGroups.find(([_geoset, group]) => group === faces[0].group)!;

    let submeshId = -1;
    if (isM2Meta) {
      const submesh = enabledSubmeshes[i];
      submeshId = submeshToId!.get(submesh)!;
    }

    geoset.material = resolveGeosetMaterial(submeshId, faces[0].material);

    mdl.textures.push(...geoset.material.layers.map((l) => l.texture));
    mdl.materials.push(geoset.material);

    const vMap = new Map<number, GeosetVertex>();

    faces.forEach((face) => {
      const vertices = face.vertices.map((v) => {
        const objV = obj.models[0].vertices[v.vertexIndex - 1];
        if (!vMap.has(v.vertexIndex)) {
          const objN = obj.models[0].vertexNormals[v.vertexNormalIndex - 1];
          let objT = obj.models[0].textureCoords[v.textureCoordsIndex - 1];
          const objT2 = obj.models[0].textureCoords2
            ? obj.models[0].textureCoords2[v.textureCoordsIndex - 1]
            : undefined;
          if (!objT) {
            objT = { u: 0, v: 0, w: 0 };
          }

          const baseTexPos: [number, number] = isAdtModel
            ? padUvEdgeAware(objT.u, 1 - objT.v)
            : [objT.u, 1 - objT.v];
          let baseTexPos2: [number, number] | undefined;
          if (objT2) {
            baseTexPos2 = isAdtModel ? padUvEdgeAware(objT2.u, 1 - objT2.v) : [objT2.u, 1 - objT2.v];
          }

          let skinWeights: SkinWeight[] | undefined;
          let matrix: Matrix | undefined;
          if (animation.isLoaded) {
            const realSkinWeightIndex = metadata.getSkinWeightIndex(v.vertexIndex - 1)!;
            skinWeights = mdlAnim.skinWeights[realSkinWeightIndex]!;
          } else {
            if (geoset.matrices.length === 0) {
              geoset.matrices.push({ id: 0, bones: [mdl.bones[0]] });
            }
            matrix = geoset.matrices[0];
          }

          geoset.vertices.push({
            id: 0,
            position: [objV.x, -objV.z, objV.y],
            normal: [objN.x, -objN.z, objN.y],
            texPosition: baseTexPos,
            texPosition2: baseTexPos2,
            matrix,
            skinWeights,
          });
          vMap.set(v.vertexIndex, geoset.vertices[geoset.vertices.length - 1]);
          return geoset.vertices[geoset.vertices.length - 1];
        }
        return vMap.get(v.vertexIndex);
      }) as [GeosetVertex, GeosetVertex, GeosetVertex];
      geoset.faces.push({ vertices });
    });
  });

  if (metadata.isLoaded && metadata.fileType === 'm2') {
    metadata.extractMDLGeosetAnim();

    debug && console.log('Geoset count:', mdl.geosets.length, 'Submesh count:', enabledSubmeshes.length);
    mdl.geosets.forEach((geoset, i) => {
      const subMesh = enabledSubmeshes[i];
      if (!subMesh || subMesh.vertexCount !== geoset.vertices.length) {
        console.error(chalk.red('Submesh mismatch'), {
          subMesh,
          geoset: geoset.name,
          geosetVertices: geoset.vertices.length,
        });
      }
      debug && console.log(geoset.name, metadata.skin.subMeshes.findIndex((s) => s === subMesh), geoset.material.layers[0].texture.image);
    });
  }

  if (metadata.isLoaded && metadata.fileType === 'm2') {
    metadata.extractMDLParticlesEmitters(textures);
    metadata.extractMDLLights();
    metadata.extractMDLRibbonEmitters(textures);
    metadata.extractMDLCameras();
  }

  mdl.modify.addDoodadDeathAnimation();
  renameEffectWowAnimations(mdl);

  if (isAdtModel) mdl.modify.recomputeNormals();
  mdl.modify.optimizeKeyFrames();
  mdl.modify.computeWalkMovespeed();
  mdl.modify.scale(config.rawModelScaleUp);
  mdl.accumScale = 1;
  mdl.modify.addCollisionShapes();
  mdl.sync();
  mdl.modify.addWc3AttachmentPoint();

  !config.isBulkExport && console.log(chalk.green('Converted:'), objFilePath, '\n');

  return { mdl, texturePaths };
}

function renameEffectWowAnimations(mdl: MDL) {
  const hold = mdl.sequences.find((s) => s.data.wowName === 'Hold');
  const decay = mdl.sequences.find((s) => s.data.wowName === 'Decay');
  const stand = mdl.sequences.find((s) => s.data.wowName === 'Stand');
  if (hold && stand && decay) {
    stand.name = 'Birth';
    stand.nonLooping = true;
    decay.nonLooping = true;
  }
}

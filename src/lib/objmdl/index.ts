import chalk from 'chalk';
import { existsSync } from 'fs';
import _ from 'lodash';
import path from 'path';

import {
  Geoset, GeosetVertex, Matrix, SkinWeight,
} from '@/lib/formats/mdl/components/geoset';
import { Material } from '@/lib/formats/mdl/components/material';
import { Texture } from '@/lib/formats/mdl/components/texture';
import { MDL } from '@/lib/formats/mdl/mdl';
import { M2MetadataFile } from '@/lib/objmdl/metadata/m2_metadata';
import { MTLFile } from '@/lib/objmdl/mtl';
import { IFace, IGroup, OBJFile } from '@/lib/objmdl/obj';

import { Config } from '../global-config';
import { AnimationFile } from './animation/animation';
import { guessFilterMode } from './utils';

const debug = false;

export async function convertWowExportModel(objFilePath: string, config: Config): Promise<{mdl: MDL, texturePaths: Set<string>}> {
  !config.isBulkExport && console.log('Converting OBJ model:', chalk.blue(objFilePath));
  const start0 = performance.now();
  let start = start0;
  const obj = await new OBJFile(objFilePath, config).parse();
  const mtl = await new MTLFile(objFilePath.replace(/\.obj$/, '.mtl'), config).parse();

  const mdl = new MDL({
    formatVersion: 1000,
    name: path.join(config.assetPrefix, path.relative(config.wowExportAssetDir, objFilePath).replace('.obj', '')),
  });

  const animation = await new AnimationFile(objFilePath.replace(/\.obj$/, '_bones.json'), config).parse();
  const metadata = await new M2MetadataFile(objFilePath.replace(/\.obj$/, '.json'), config, animation, mdl).parse();

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
  metadata.mapSubMeshesToMdlGeosets(mdl);

  const parentDir = path.dirname(path.normalize(objFilePath.replaceAll('\\', '/')));

  // Extract material data

  const texturePaths = new Set<string>();

  const {
    submeshIdToMat, textures,
  } = metadata.extractMDLTexturesMaterials();
  mdl.textures = [];
  mdl.materials = [];
  metadata.textures.forEach((tex) => {
    if (!tex.fileNameExternal) return;
    const absPath = path.join(parentDir, tex.fileNameExternal);
    if (!existsSync(absPath)) {
      console.warn('Skipping texture not found', absPath, 'for model', objFilePath);
      return;
    }
    const textureRelativePath = path.relative(config.wowExportAssetDir, absPath);
    texturePaths.add(textureRelativePath);
  });

  const mtlNameMap = new Map<string, Material>();

  const resolveGeosetMaterial = (submeshId: number, matName: string): MDL['materials'][number] => {
    const mtlMaterial = mtl.materials.find((m) => m.name === matName);
    const textureRelativePath = mtlMaterial ? path.relative(config.wowExportAssetDir, path.join(parentDir, mtlMaterial.map_Kd!)) : undefined;
    textureRelativePath && texturePaths.add(textureRelativePath);

    const protoMat = submeshIdToMat.get(submeshId);
    const mat = _.cloneDeep(protoMat);

    if (mat) {
      // do not clone tvertexAnim
      mat.layers.forEach((l, i) => {
        l.tvertexAnim = protoMat?.layers[i].tvertexAnim;
      });

      // if missing texture path, use value from .MTL file
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
      // metadata does not have this material, fallback to resolve material from mtl file

      if (mtlNameMap.has(matName)) {
        return mtlNameMap.get(matName)!;
      }

      debug && console.log('no submeshIdToMat for submeshId', submeshId, 'matName', matName);
      debug && console.log('Fallback to mtl file');
      textureRelativePath && texturePaths.add(textureRelativePath);
      const texture: Texture = {
        id: 0,
        image: textureRelativePath ? path.join(config.assetPrefix, textureRelativePath.replace('.png', '.blp')) : '',
        wrapHeight: true,
        wrapWidth: true,
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
            filterMode: textureRelativePath ? guessFilterMode(textureRelativePath) : 'None',
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

  // Construct mdl

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
    // Model without sequence will crash Wc3
    mdl.sequences.push({
      name: 'Stand',
      data: {
        wowName: '', attackTag: '', wc3Name: 'Stand', wowVariant: 0, wowFrequency: 0,
      },
      interval: [0, 1000],
      moveSpeed: 0,
      nonLooping: false,

      // bounds will be computed later in mdl.sync()
      minimumExtent: [-1, -1, -1],
      maximumExtent: [1, 1, 1],
      boundsRadius: 1,
    });
  }

  const submeshToId = new Map(metadata.skin.subMeshes.map((s, i) => [s, i]));
  const enabledSubmeshes = metadata.skin.subMeshes.filter((s) => s.enabled);

  let idx = 0;
  groups.forEach((faces) => {
    const i = idx;
    idx++;
    const [geoset] = geosetsGroups.find(([_geoset, group]) => group === faces[0].group)!;

    const submesh = enabledSubmeshes[i];
    const submeshId = submeshToId.get(submesh)!;

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
          const objT2 = obj.models[0].textureCoords2 ? obj.models[0].textureCoords2[v.textureCoordsIndex - 1] : undefined;
          if (!objT) {
            // console.error('No texture coords found for vertex', v.vertexIndex, 'in', objFilePath);
            // console.error('obj.models[0].textureCoords.length', obj.models[0].textureCoords.length);
            // console.error('obj.models[0].textureCoords - 1', v.textureCoordsIndex - 1);
            objT = { u: 0, v: 0, w: 0 };
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
            texPosition: [objT.u, 1 - objT.v],
            texPosition2: objT2 ? [objT2.u, 1 - objT2.v] : undefined,
            matrix,
            skinWeights,
          });
          vMap.set(v.vertexIndex, geoset.vertices[geoset.vertices.length - 1]);
          return geoset.vertices[geoset.vertices.length - 1];
        }
        return vMap.get(v.vertexIndex);
      }) as [GeosetVertex, GeosetVertex, GeosetVertex]; // TODO: handle when there are 4+ vertices in OBJ face
      geoset.faces.push({ vertices });
    });
  });

  // Assign materials to geosets

  if (metadata.isLoaded) {
    metadata.extractMDLGeosetAnim();

    // Validate submeshes equals to geosets
    debug && console.log('Geoset count:', mdl.geosets.length, 'Submesh count:', enabledSubmeshes.length);
    mdl.geosets.forEach((geoset, i) => {
      const subMesh = enabledSubmeshes[i];
      if (!subMesh || subMesh.vertexCount !== geoset.vertices.length) {
        console.error(chalk.red('Submesh mismatch'), {
          subMesh,
          geoset: geoset.name,
          geosetVertices: geoset.vertices.length,
        });
        // throw new Error('Submesh mismatch');
      }
      debug && console.log(geoset.name, metadata.skin.subMeshes.findIndex((s) => s === subMesh), geoset.material.layers[0].texture.image);
    });
  }

  // New: particles emitters
  if (metadata.isLoaded) {
    metadata.extractMDLParticlesEmitters(textures);
    metadata.extractMDLLights();
    metadata.extractMDLRibbonEmitters(textures);
    metadata.extractMDLCameras();
  }

  debug && console.log('basic parse took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.modify.optimizeKeyFrames();
  debug && console.log('optimizeKeyFrames took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.modify.computeWalkMovespeed();
  debug && console.log('computeWalkMovespeed took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.modify.scale(config.rawModelScaleUp);
  mdl.accumScale = 1;
  debug && console.log('scale took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.modify.addCollisionShapes();
  debug && console.log('computeCollisionShape took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.sync();
  debug && console.log('sync took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.modify.addWc3AttachmentPoint();
  debug && console.log('addWc3AttachmentPoint took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  const totalTimeS = (performance.now() - start0) / 1000;
  !config.isBulkExport && console.log(chalk.green('Converted:'), objFilePath, '-', chalk.yellow(totalTimeS.toFixed(2)), 's\n');

  return { mdl, texturePaths };
}

import chalk from 'chalk';
import { existsSync } from 'fs';
import _ from 'lodash';
import path from 'path';

import { Config } from '../global-config';
import { AnimationFile } from './animation/animation';
import { GeosetVertex, Matrix, SkinWeight } from './mdl/components/geoset';
import { Material } from './mdl/components/material';
import { Texture } from './mdl/components/texture';
import { MDL } from './mdl/mdl';
import { M2MetadataFile } from './metadata/m2_metadata';
import { MTLFile } from './mtl';
import { IFace, IGroup, OBJFile } from './obj';
import { guessFilterMode } from './utils';

const debug = false;

export function convertWowExportModel(objFilePath: string, config: Config): {mdl: MDL, texturePaths: Set<string>} {
  console.log('Converting OBJ model:', objFilePath);
  let start0 = performance.now();
  let start = start0;
  const obj = new OBJFile(objFilePath).parse();
  const mtl = new MTLFile(objFilePath.replace(/\.obj$/, '.mtl'));

  const mdl = new MDL({
    formatVersion: 1000,
    name: path.relative(config.wowExportAssetDir, objFilePath).replace('.obj', ''),
  });

  const animation = new AnimationFile(objFilePath.replace(/\.obj$/, '_bones.json'));
  const metadata = new M2MetadataFile(objFilePath.replace(/\.obj$/, '.json'), config, animation, mdl);

  if (obj.models.length === 0) {
    console.error(chalk.red('No models found in', objFilePath));
    return { mdl, texturePaths: new Set<string>() };
  }

  const groups = new Map<IGroup, IFace[]>();
  obj.models[0].faces.forEach((f) => {
    if (!groups.has(f.group)) {
      groups.set(f.group, []);
    }
    groups.get(f.group)!.push(f);
  });

  const parentDir = path.dirname(objFilePath);

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
    const textureRelativePath = mtlMaterial ? path.relative(config.wowExportAssetDir, path.join(parentDir, mtlMaterial.map_Kd!)) : '';
    texturePaths.add(textureRelativePath);

    const protoMat = submeshIdToMat.get(submeshId);
    const mat = _.cloneDeep(protoMat);

    if (mat) {
      // do not clone tvertexAnim
      mat.layers.forEach((l, i) => {
        l.tvertexAnim = protoMat?.layers[i].tvertexAnim;
      });

      // if missing texture path, use value from .MTL file
      mat.layers.forEach((l) => {
        const blpPath = l.texture.image || path.join(config.assetPrefix, textureRelativePath.replace('.png', '.blp'));
        l.texture = {
          ...l.texture,
          image: blpPath,
        };
        if (blpPath) {
          texturePaths.add(blpPath);
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

  groups.forEach((faces) => {
    const i = mdl.geosets.length;
    const submesh = enabledSubmeshes[i];
    const submeshId = submeshToId.get(submesh)!;
    mdl.geosets.push({
      id: 0,
      name: faces[0].group.name,
      vertices: [],
      faces: [],
      matrices: mdl.bones.map((b) => ({ id: 0, bones: [b] })),
      minimumExtent: [0, 0, 0],
      maximumExtent: [0, 0, 0],
      boundsRadius: 0,
      material: resolveGeosetMaterial(submeshId, faces[0].material),
      selectionGroup: 0,
    });
    const geoset = mdl.geosets[mdl.geosets.length - 1];

    mdl.textures.push(...geoset.material.layers.map((l) => l.texture));
    mdl.materials.push(geoset.material);

    const vMap = new Map<number, GeosetVertex>();

    faces.forEach((face) => {
      const vertices = face.vertices.map((v) => {
        const objV = obj.models[0].vertices[v.vertexIndex - 1];
        if (!vMap.has(v.vertexIndex)) {
          const objN = obj.models[0].vertexNormals[v.vertexNormalIndex - 1];
          const objT = obj.models[0].textureCoords[v.textureCoordsIndex - 1];
          if (!objT) {
            console.error('No texture coords found for vertex', v.vertexIndex, 'in', objFilePath);
            console.error('obj.models[0].textureCoords.length', obj.models[0].textureCoords.length);
            console.error('obj.models[0].textureCoords - 1', v.textureCoordsIndex - 1);
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
        console.error('Submesh mismatch', {
          subMesh,
          geoset: geoset.name,
        });
        throw new Error('Submesh mismatch');
      }
      debug && console.log(geoset.name, metadata.skin.subMeshes.findIndex((s) => s === subMesh), geoset.material.layers[0].texture.image);
    });
  }

  // New: particles emitters
  if (metadata.isLoaded) {
    metadata.extractMDLParticlesEmitters(textures);
  }

  debug && console.log('basic parse took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.modify.optimizeKeyFrames();
  debug && console.log('optimizeKeyFrames took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.modify.scale(config.rawModelScaleUp);
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

  start = performance.now();
  mdl.modify.computeWalkMovespeed();
  debug && console.log('computeWalkMovespeed took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  const totalTimeS = (performance.now() - start0) / 1000;
  console.log(chalk.green('Successfully converted:'), objFilePath, "-", chalk.yellow(totalTimeS.toFixed(2)), 's\n');

  return { mdl, texturePaths };
}

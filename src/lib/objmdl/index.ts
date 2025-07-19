import chalk from 'chalk';
import { existsSync, readFileSync } from 'fs';
import path from 'path';

import { Config } from '../converter/common';
import { guessFilterMode } from '../global-config';
import { AnimationFile } from './animation/animation';
import {
  GeosetVertex, Material, Matrix, MDL, SkinWeight, Texture,
} from './mdl/mdl';
import { M2MetadataFile } from './metadata/m2_metadata';
import { MTLFile } from './mtl';
import { IFace, OBJFile } from './obj';

const debug = false;

export function convertObjMdl(objFilePath: string, assetRoot: string, texturePrefix: string, config: Config) {
  let start = performance.now();
  const obj = new OBJFile(objFilePath).parse();
  const mtl = new MTLFile(objFilePath.replace(/\.obj$/, '.mtl'));
  const animation = new AnimationFile(objFilePath.replace(/\.obj$/, '_bones.json'));
  const metadata = new M2MetadataFile(objFilePath.replace(/\.obj$/, '.json'), config);

  const mdl = new MDL({
    formatVersion: 900,
    name: path.relative(assetRoot, objFilePath).replace('.obj', '.mdl'),
  });

  if (obj.models.length === 0) {
    return { mdl, texturePaths: new Set<string>() };
  }

  const groups = new Map<string, IFace[]>();
  obj.models[0].faces.forEach((f) => {
    if (!groups.has(f.group)) {
      groups.set(f.group, []);
    }
    groups.get(f.group)!.push(f);
  });

  const parentDir = path.dirname(objFilePath);

  // Extract material data
  // eslint-disable-next-line
  let resolveGeosetMaterial = (_geosetId: number, _matName: string): MDL['materials'][number] => mdl.materials[0];
  const texturePaths = new Set<string>();

  if (!metadata.isLoaded) {
    const matMap = new Map<string, Material>();
    mtl.materials.forEach((mtlMaterial) => {
      const materialRelativePath = path.relative(assetRoot, path.join(parentDir, mtlMaterial.map_Kd!));
      if (!existsSync(path.join(config.wowExportPath, materialRelativePath))) {
        console.warn('Material not found', materialRelativePath, 'for model', objFilePath);
      }
      texturePaths.add(materialRelativePath);
      const texture: Texture = {
        id: 0,
        image: path.join(texturePrefix, materialRelativePath.replace('.png', '.blp')),
        wrapHeight: true,
        wrapWidth: true,
      };
      const material: Material = {
        id: 0,
        constantColor: true,
        layers: [
          { texture, filterMode: guessFilterMode(materialRelativePath), twoSided: false },
        ],
      };
      mdl.textures.push(texture);
      mdl.materials.push(material);
      matMap.set(mtlMaterial.name, material);
    });
    resolveGeosetMaterial = (_geosetId, matName) => matMap.get(matName)!;
  } else {
    const {
      textures, materials, textureAnims, geosetToMat,
    } = metadata.extractMDLTexturesMaterials(texturePrefix, groups.size, animation, mdl.globalSequences);
    mdl.textures = textures;
    mdl.materials = materials;
    mdl.textureAnims = textureAnims;
    // eslint-disable-next-line
    resolveGeosetMaterial = (geosetId, _matName) => geosetToMat[geosetId]!;
    metadata.textures.forEach((tex) => {
      const absPath = path.join(parentDir, tex.fileNameExternal);
      if (!tex.fileNameExternal || !existsSync(absPath)) {
        console.warn('Skipping texture not found', absPath, 'for model', objFilePath);
        return;
      }
      const materialRelativePath = path.relative(assetRoot, path.join(parentDir, tex.fileNameExternal));
      texturePaths.add(materialRelativePath);
    });
  }

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

  groups.forEach((faces) => {
    mdl.geosets.push({
      id: 0,
      name: faces[0].group,
      vertices: [],
      faces: [],
      matrices: mdl.bones.map((b) => ({ id: 0, bones: [b] })),
      minimumExtent: [0, 0, 0],
      maximumExtent: [0, 0, 0],
      boundsRadius: 0,
      material: resolveGeosetMaterial(mdl.geosets.length, faces[0].material),
      selectionGroup: 0,
    });

    const geoset = mdl.geosets[mdl.geosets.length - 1];

    const vMap = new Map<number, GeosetVertex>();

    faces.forEach((face) => {
      const vertices = face.vertices.map((v) => {
        const objV = obj.models[0].vertices[v.vertexIndex - 1];
        if (!vMap.has(v.vertexIndex)) {
          const objN = obj.models[0].vertexNormals[v.vertexNormalIndex - 1];
          const objT = obj.models[0].textureCoords[v.textureCoordsIndex - 1];

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

  if (metadata.isLoaded) {
    mdl.geosetAnims = metadata.extractMDLGeosetAnim(animation, mdl.geosets);
  }

  debug && console.log('basic parse took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.modify.optimizeKeyFrames();
  debug && console.log('optimizeKeyFrames took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.modify.scale(config.rawModelScaleUp);
  debug && console.log('scale took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.sync();
  debug && console.log('sync took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  start = performance.now();
  mdl.modify.addWc3AttachmentPoint();
  debug && console.log('addWc3AttachmentPoint took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  return { mdl, texturePaths };
}

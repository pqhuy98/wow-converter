import path from 'path';

import { V3 } from '@/lib/math/vector';

import { Bone } from '../components/node/node';
import { MDL } from '../mdl';
import { MDLModify } from '.';

const debug = true;

export function addMdlItemToBone(this: MDLModify, item: MDL, bone: Bone) {
  debug && console.log(`Attaching item "${path.basename(item.model.name)}" to bone "${bone.name}"...`);

  item.sequences = [item.sequences[0]];
  item.modify.optimizeKeyFrames();

  item.getNodes().forEach((b) => {
    if (!b.parent) {
      b.parent = bone;
    }
    b.pivotPoint = V3.sum(b.pivotPoint, bone.pivotPoint);
  });
  item.geosets.forEach((geoset) => geoset.vertices.forEach((v) => {
    v.position = V3.sum(v.position, bone.pivotPoint);
  }));

  mergeItemObjects(this.mdl, item);
  return this;
}

export function addItemPathToBone(this: MDLModify, itemPath: string, bone: Bone) {
  this.mdl.attachments.push({
    type: 'AttachmentPoint',
    name: `Item_${itemPath}`,
    path: itemPath,
    parent: bone,
    pivotPoint: [...bone.pivotPoint],
    flags: [],
    attachmentId: 0,
    scaling: {
      interpolation: 'DontInterp',
      keyFrames: new Map(this.mdl.sequences.map((seq) => [seq.interval[0], [2, 2, 2]])),
      type: 'scaling',
    },
  });
  return this;
}

export function addMdlCollectionItemToModel(this: MDLModify, item: MDL) {
  debug && console.log(`Attaching item "${path.basename(item.model.name)}" as collection...`);

  item.sequences = [item.sequences[0]];
  item.modify.optimizeKeyFrames();

  const boneMap = new Map<string, Bone>(this.mdl.bones.map((b) => [b.name, b]));
  const getMainBone = (bone: Bone) => {
    const mainBone = boneMap.get(bone.name);
    if (!mainBone) {
      throw new Error(`Cannot merge item "${path.basename(item.model.name)}" to model because bone "${bone.name}" is missing.`);
    }
    return mainBone;
  };

  // Replace item bones with main model bones
  item.geosets.forEach((geoset) => {
    geoset.matrices.forEach((matrix) => {
      matrix.bones.forEach((bone, i) => {
        // const oldBone = bone;
        matrix.bones[i] = getMainBone(bone);
        // console.log(`Replace bone "${oldBone.name}" of item with "${matrix.bones[i].name}" in main model, include=${this.mdl.bones.includes(matrix.bones[i])}`);
      });
    });
    geoset.vertices.forEach((vertex) => {
      if (vertex.skinWeights) {
        vertex.skinWeights.forEach((weight) => {
          // const oldBone = weight.bone;
          weight.bone = getMainBone(weight.bone);
          // console.log(`Replace bone "${oldBone.name}" of item with "${weight.bone.name}" in main model, include=${this.mdl.bones.includes(weight.bone)}`);
        });
      }
    });
  });
  item.bones = [];

  mergeItemObjects(this.mdl, item);
  return this;
}

export function canAddMdlCollectionItemToModel(main: MDL, item: MDL) {
  if (item.bones.some((b) => !b.parent && b.name.includes('bone_'))) {
    return false;
  }

  const boneMap = new Map<string, Bone>(main.bones.map((b) => [b.name, b]));
  return item.bones.every((b) => boneMap.has(b.name));
}

function mergeItemObjects(main: MDL, item: MDL) {
  item.geosets.forEach((geoset) => {
    geoset.name = `item_${geoset.name}`;
  });

  main.globalSequences.push(...item.globalSequences);

  main.textures.push(...item.textures);
  main.textureAnims.push(...item.textureAnims);
  main.materials.push(...item.materials);

  main.geosets.push(...item.geosets);
  main.geosetAnims.push(...item.geosetAnims);

  main.bones.push(...item.bones);
  // main.attachments.push(...extra.attachments);
  main.eventObjects.push(...item.eventObjects);
  // main.collisionShapes.push(...extra.collisionShapes);
  main.particleEmitter2s.push(...item.particleEmitter2s);
}

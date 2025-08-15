import chalk from "chalk";
import { MDL } from "../mdl";
import { MDLModify } from ".";
import { V3 } from "@/lib/math/vector";
import path from "path";

export function addMdlItemToBone(this: MDLModify, item: MDL, boneName: string) {
  const attachmentBone = this.mdl.bones.find((b) => b.name === boneName);
  if (!attachmentBone) {
    console.error(chalk.red(`Cannot find bone "${boneName}" to attach item "${path.basename(item.model.name)}".`));
    return this;
  }
  console.log(`Attach item "${path.basename(item.model.name)}" to bone "${attachmentBone.name}"`);

  item.getNodes().forEach((b) => {
    if (!b.parent) {
      b.parent = attachmentBone;
      b.pivotPoint = V3.sum(b.pivotPoint, attachmentBone.pivotPoint);
    }
  });
  item.geosets.forEach((geoset) => geoset.vertices.forEach((v) => {
    v.position = V3.sum(v.position, attachmentBone.pivotPoint);
  }));

  this.mdl.globalSequences.push(...item.globalSequences);

  this.mdl.textures.push(...item.textures);
  this.mdl.textureAnims.push(...item.textureAnims);
  this.mdl.materials.push(...item.materials);

  this.mdl.geosets.push(...item.geosets);
  this.mdl.geosetAnims.push(...item.geosetAnims);

  this.mdl.bones.push(...item.bones);
  // this.mdl.attachments.push(...item.attachments);
  this.mdl.eventObjects.push(...item.eventObjects);
  // this.mdl.collisionShapes.push(...item.collisionShapes);
  return this;
}

export function addItemPathToBone(this: MDLModify, itemPath: string, boneName: string) {
  const attachmentBone = this.mdl.bones.find((b) => b.name === boneName);
  if (!attachmentBone) {
    console.error(chalk.red(`Cannot find bone "${boneName}" to attach item path "${itemPath}".`));
    return this;
  }
  this.mdl.attachments.push({
    type: 'AttachmentPoint',
    name: `Item_${itemPath}`,
    path: itemPath,
    parent: attachmentBone,
    pivotPoint: [...attachmentBone.pivotPoint],
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
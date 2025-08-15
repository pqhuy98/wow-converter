import chalk from 'chalk';
import _ from 'lodash';

import { Vector3 } from '../../../math/common';
import { WowAnimName } from '../../animation/animation_mapper';
import { WoWAttachmentID, WoWToWC3AttachmentMap } from '../../animation/bones_mapper';
import { Bound } from '../components/extent';
import { Sequence } from '../components/sequence';
import { MDL } from '../mdl';
import {
  iterateVerticesAtTimestamp,
} from '../mdl-traverse';
import { addCollisionShapes } from './compute-collision-shapes';
import { optimizeKeyFrames, removeCinematicSequences, removeUnusedMaterialsTextures, removeUnusedNodes, removeUnusedVertices } from './optimizations';
import { scale, scaleSequenceDuration, translate } from './translate-scale';
import { addPortraitCamera } from './add-portrait-camera';
import { computeWalkMovespeed } from './compute-walk-speed';
import { deleteVerticesIf, deleteVerticesOutsideBox, deleteVerticesInsideBox, cut1DimOutside, cutInsidePercent, cutOutsidePercent, cropVerticesOneDimension, deleteFacesIf } from './delete-cut-crop';
import { convertToSd800 } from './convert-to-sd800';
import { addDecayAnimation, addEventObjectBySequenceName, debugSequence, removeWowSequence, renameSequencesByWowName, sortSequences, useWalkSequenceByWowName } from './sequences';
import { addItemPathToBone, addMdlItemToBone } from './add-item-to-bone';
import { addWc3AttachmentPoint, setWowAttachmentScale } from './attachments';

export class MDLModify {
  constructor(public mdl: MDL) {
  }

  // General
  convertToSd800 = convertToSd800;
  computeWalkMovespeed = computeWalkMovespeed;

  // Basic transformations
  scale = scale;
  translate = translate;
  scaleSequenceDuration = scaleSequenceDuration;

  // Add extras
  addPortraitCamera = addPortraitCamera;
  addDecayAnimation = addDecayAnimation;
  addMdlItemToBone = addMdlItemToBone;
  addItemPathToBone = addItemPathToBone;
  addCollisionShapes = addCollisionShapes;

  // Sequence manipulation
  sortSequences = sortSequences;
  removeWowSequence = removeWowSequence;
  useWalkSequenceByWowName = useWalkSequenceByWowName;
  renameSequencesByWowName = renameSequencesByWowName;
  debugSequence = debugSequence;
  addEventObjectBySequenceName = addEventObjectBySequenceName;

  // Attachments
  addWc3AttachmentPoint = addWc3AttachmentPoint;
  setWowAttachmentScale = setWowAttachmentScale;

  // Optimizations
  removeUnusedMaterialsTextures = removeUnusedMaterialsTextures;
  removeUnusedNodes = removeUnusedNodes;
  removeUnusedVertices = removeUnusedVertices;
  removeCinematicSequences = removeCinematicSequences;
  optimizeKeyFrames = optimizeKeyFrames;

  // Delete/cut/crop
  deleteVerticesIf = deleteVerticesIf;
  deleteVerticesOutsideBox = deleteVerticesOutsideBox;
  deleteVerticesInsideBox = deleteVerticesInsideBox;
  cut1DimOutside = cut1DimOutside;
  cutInsidePercent = cutInsidePercent;
  cutOutsidePercent = cutOutsidePercent;
  cropVerticesOneDimension = cropVerticesOneDimension;
  deleteFacesIf = deleteFacesIf;

  setLargeBounds() {
    this.mdl.boundsOverriden = (obj: Bound) => {
      const min = obj.minimumExtent;
      const max = obj.maximumExtent;
      for (let i = 0; i < 3; i++) {
        const abs = Math.max(Math.abs(min[i]), Math.abs(max[i]));
        min[i] = -abs * 3;
        max[i] = abs * 3;
      }
      obj.boundsRadius = _.max(max)!;
    };
    return this;
  }

  setInfiniteBounds() {
    this.mdl.boundsOverriden = (obj: Bound) => {
      const min = obj.minimumExtent;
      const max = obj.maximumExtent;
      for (let i = 0; i < 3; i++) {
        min[i] = -99999;
        max[i] = 99999;
      }
      obj.boundsRadius = 99999;
    };
    return this;
  }

  getMaxZAtTimestamp(sequence: Sequence, offset: number) {
    // Find the highest vertex in the animation
    let maxZ = -Infinity;
    iterateVerticesAtTimestamp(this.mdl, sequence, offset, (v, vPos) => {
      maxZ = Math.max(maxZ, vPos[2]);
    });
    return maxZ;
  }
}


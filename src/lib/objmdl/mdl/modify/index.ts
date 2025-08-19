import _ from 'lodash';

import { Bound } from '../components/extent';
import { Sequence } from '../components/sequence';
import { MDL } from '../mdl';
import {
  iterateVerticesAtTimestamp,
} from '../mdl-traverse';
import { addItemPathToBone, addMdlItemToBone } from './add-item-to-bone';
import { addPortraitCamera } from './add-portrait-camera';
import { addWc3AttachmentPoint, setWowAttachmentScale } from './attachments';
import { addCollisionShapes } from './compute-collision-shapes';
import { computeWalkMovespeed } from './compute-walk-speed';
import { convertToSd800 } from './convert-to-sd800';
import {
  cropVerticesOneDimension, cut1DimOutside, cutInsidePercent, cutOutsidePercent, deleteFacesIf,
  deleteVerticesIf, deleteVerticesInsideBox, deleteVerticesOutsideBox,
} from './delete-cut-crop';
import {
  optimizeKeyFrames, removeCinematicSequences, removeUnusedMaterialsTextures, removeUnusedNodes, removeUnusedVertices,
} from './optimizations';
import {
  addDecayAnimation, addEventObjectBySequenceName, debugSequence, removeWowSequence, renameSequencesByWowName, sortSequences, useWalkSequenceByWowName,
} from './sequences';
import { scale, scaleSequenceDuration, translate } from './translate-scale';

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

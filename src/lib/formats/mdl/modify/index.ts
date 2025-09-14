import _ from 'lodash';

import { Bound } from '../components/extent';
import { Sequence } from '../components/sequence';
import { MDL } from '../mdl';
import {
  iterateVerticesAtTimestamp,
} from '../mdl-traverse';
import {
  addItemPathToBone, addMdlCollectionItemToModel, addMdlItemToBone,
} from './add-item-to-model';
import { addPortraitCamera } from './add-portrait-camera';
import { addWc3AttachmentPoint } from './attachments';
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
  addDecayAnimation, addEventObjectBySequenceName, cloneSequence, concatenateSequences, debugSequence, removeWowSequence, renameSequencesByWowName, sortSequences, useWalkSequenceByWowName,
} from './sequences';
import {
  flipY,
  rotate, scale, scaleSequenceDuration, translate,
} from './translate-scale-rotate';

export class MDLModify {
  constructor(public mdl: MDL) {
  }

  // General
  convertToSd800 = convertToSd800;

  computeWalkMovespeed = computeWalkMovespeed;

  // Basic transformations
  scale = scale;

  flipY = flipY;

  translate = translate;

  rotate = rotate;

  scaleSequenceDuration = scaleSequenceDuration;

  // Add extras
  addPortraitCamera = addPortraitCamera;

  addDecayAnimation = addDecayAnimation;

  addMdlItemToBone = addMdlItemToBone;

  addItemPathToBone = addItemPathToBone;

  addMdlCollectionItemToModel = addMdlCollectionItemToModel;

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

  // Optimizations
  removeUnusedMaterialsTextures = removeUnusedMaterialsTextures;

  removeUnusedNodes = removeUnusedNodes;

  removeUnusedVertices = removeUnusedVertices;

  removeCinematicSequences = removeCinematicSequences;

  optimizeKeyFrames = optimizeKeyFrames;

  optimizeAll() {
    this.sortSequences();
    this.removeUnusedVertices();
    this.removeUnusedNodes();
    this.removeUnusedMaterialsTextures();
    this.optimizeKeyFrames();
    this.mdl.sync();
    return this;
  }

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

  keepCinematicSequences(patterns: (string | RegExp)[]) {
    this.mdl.sequences.forEach((s) => {
      const name = s.data.wowName;
      if (patterns.some((p) => (typeof p === 'string' ? name.includes(p) : p.test(name)))) {
        s.keep = true;
      }
    });
    return this;
  }

  scaleParticlesDensity(factor: number) {
    this.mdl.particleEmitter2s.forEach((p) => {
      if ('static' in p.emissionRate) {
        p.emissionRate.value *= factor;
      } else {
        const keyFrames = p.emissionRate.keyFrames;
        keyFrames.forEach((v, k) => {
          keyFrames[k] = v * factor;
        });
      }
    });
    return this;
  }

  cloneSequence = cloneSequence;

  concatenateSequences = concatenateSequences;
}

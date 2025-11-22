import _ from 'lodash';

import { Vector3 } from '../../../math/common';
import { Bound } from '../components/extent';
import { Face, GeosetVertex } from '../components/geoset';
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
  addDecayAnimation, addDoodadDeathAnimation, addEventObjectBySequenceName, cloneSequence, concatenateSequences,
  debugSequence, removeWowSequence, renameSequencesByWowName, sortSequences, useWalkSequenceByWowName,
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

  addDoodadDeathAnimation = addDoodadDeathAnimation;

  recomputeNormals() {
    const sub = (a: Vector3, b: Vector3): Vector3 => [a[0] - b[0], a[1] - b[1], a[2] - b[2]];
    const cross = (a: Vector3, b: Vector3): Vector3 => [
      a[1] * b[2] - a[2] * b[1],
      a[2] * b[0] - a[0] * b[2],
      a[0] * b[1] - a[1] * b[0],
    ];
    const length = (v: Vector3): number => Math.sqrt(v[0] * v[0] + v[1] * v[1] + v[2] * v[2]);
    const epsilon = 0.00001;

    // Build adjacency: which faces use which vertex
    const vertexToFaces = new Map<GeosetVertex, Face[]>();
    // Group vertices by exact position (x, y, z)
    const positionToVertices = new Map<string, GeosetVertex[]>();

    const keyFromPosition = (pos: Vector3): string => `${pos[0]}|${pos[1]}|${pos[2]}`;

    this.mdl.geosets.forEach((geoset) => {
      geoset.vertices.forEach((vert) => {
        // Reset normals
        vert.normal = [0, 0, 0];

        const key = keyFromPosition(vert.position);
        let list = positionToVertices.get(key);
        if (!list) {
          list = [];
          positionToVertices.set(key, list);
        }
        list.push(vert);
      });

      geoset.faces.forEach((face) => {
        face.vertices.forEach((vert) => {
          let list = vertexToFaces.get(vert);
          if (!list) {
            list = [];
            vertexToFaces.set(vert, list);
          }
          list.push(face);
        });
      });
    });

    // For each group of vertices that share the same position,
    // reproduce GeosetVertex.createNormal(matches) from the Java code.
    positionToVertices.forEach((verticesAtPos) => {
      const sum: Vector3 = [0, 0, 0];

      verticesAtPos.forEach((vert) => {
        const faces = vertexToFaces.get(vert) ?? [];

        faces.forEach((face) => {
          const [v0, v1, v2] = face.vertices;
          const p0 = v0.position;
          const p1 = v1.position;
          const p2 = v2.position;

          // Equivalent to triangle.verts[0].delta(verts[1]).crossProduct(verts[1].delta(verts[2]))
          const e1 = sub(p0, p1);
          const e2 = sub(p1, p2);
          const perp = cross(e1, e2);

          let mag = length(perp);
          if (mag === 0) {
            mag = epsilon;
          }

          const nx = perp[0] / mag;
          const ny = perp[1] / mag;
          const nz = perp[2] / mag;

          sum[0] += nx;
          sum[1] += ny;
          sum[2] += nz;
        });
      });

      let sumMag = length(sum);
      if (sumMag === 0) {
        sumMag = epsilon;
      }

      const normal: Vector3 = [sum[0] / sumMag, sum[1] / sumMag, sum[2] / sumMag];

      verticesAtPos.forEach((vert) => {
        vert.normal = [normal[0], normal[1], normal[2]];
      });
    });

    return this;
  }
}

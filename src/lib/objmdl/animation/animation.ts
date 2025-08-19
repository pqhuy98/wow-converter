import assert from 'assert';
import chalk from 'chalk';
import { readFileSync } from 'fs';
import _ from 'lodash';

import { Vector3 } from '@/lib/math/common';

import { BlizzardNull } from '../../constants';
import { SkinWeight } from '../mdl/components/geoset';
import { GlobalSequence } from '../mdl/components/global-sequence';
import { Bone, NodeFlag } from '../mdl/components/node/node';
import { MDL, WowAttachment } from '../mdl/mdl';
import { wowToWc3Interpolation } from '../utils';
import { getWacraftSequenceData, getWowAnimName, isLoopAnimation } from './animation_mapper';
import { getBoneName } from './bones_mapper';

export namespace Data {
  export interface Bone {
    boneID: number
    flags: number
    parentBone: number
    subMeshID: number
    boneNameCRC: number
    translation: Translation
    rotation: Rotation
    scale: Scale
    pivot: [number, number, number]
  }

  export interface Translation {
    globalSeq: number
    interpolation: number
    timestamps: (number[] | null)[]
    values: [number, number, number][][]
  }

  export interface Rotation {
    globalSeq: number
    interpolation: number
    timestamps: (number[] | null)[]
    values: [number, number, number, number][][]
  }

  export interface Scale {
    globalSeq: number
    interpolation: number
    timestamps: (number[] | null)[]
    values: [number, number, number][][]
  }

  export interface Animation {
    id: number
    variationIndex: number
    duration: number
    movespeed: number
    flags: number
    frequency: number
    padding: number
    replayMin: number
    replayMax: number
    blendTimeIn: number
    blendTimeOut: number
    boxPosMin: [number, number, number]
    boxPosMax: [number, number, number]
    boxRadius: number
    variationNext: number
    aliasNext: number
  }

  export interface Attachment {
    id: number
    bone: number
    unknown: number
    position: Vector3
    animateAttached: {
      globalSeq: number
      interpolation: number
      timestamps: number[][]
      values: number[][]
    }
  }
}

export interface AnimationData {
  bones: Data.Bone[]
  animations?: Data.Animation[]
  boneWeights: number[]
  boneIndicies: number[]
}

const debug = false;

export class AnimationFile implements AnimationData {
  bones: Data.Bone[];

  animations?: Data.Animation[];

  boneWeights: number[];

  boneIndicies: number[];

  attachments: Data.Attachment[];

  isLoaded = false;

  constructor(public filePath: string) {
    try {
      console.log("Loading animation file", this.filePath);
      const start = performance.now();
      Object.assign(this, JSON.parse(readFileSync(filePath, 'utf-8')));
      debug && console.log('AnimationFile load took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');
      this.isLoaded = true;
    } catch (e) {
      if (e.code === 'ENOENT') {
        // file not exist, do not load.
        return;
      }
      throw e;
    }
  }

  /**
   * `boneIndices[vertexId]` is the list of bones attached to the vertex vertexId
   */
  toMdl(globalSequences: GlobalSequence[]): Pick<MDL, 'bones' | 'sequences'> & {
    skinWeights: SkinWeight[][],
    wowAttachments: WowAttachment[],
  } {
    const start = performance.now();
    if (!this.isLoaded) {
      throw new Error('Animation file is not loaded');
    }

    const excludedAnimation = new Set<number>();

    const globalSequenceMap = new Map<number, GlobalSequence>(globalSequences.map((gs) => [gs.id, gs]));
    function getGlobalSeq(id: number) {
      if (!globalSequenceMap.has(id)) {
        const newGs = {
          id, duration: 1,
        };
        globalSequences.push(newGs);
        globalSequenceMap.set(id, newGs);
      }
      return globalSequenceMap.get(id);
    }

    const bones: MDL['bones'] = this.bones.map((bone, boneI) => {
      const mdlBone: Bone = {
        type: 'Bone',
        name: getBoneName(bone.boneID, boneI, bone.boneNameCRC),
        flags: [],
        translation: {
          interpolation: wowToWc3Interpolation(bone.translation.interpolation),
          globalSeq: bone.translation.globalSeq !== BlizzardNull ? getGlobalSeq(bone.translation.globalSeq) : undefined,
          keyFrames: new Map(),
          type: 'translation',
        },
        rotation: {
          interpolation: wowToWc3Interpolation(bone.rotation.interpolation),
          globalSeq: bone.rotation.globalSeq !== BlizzardNull ? getGlobalSeq(bone.rotation.globalSeq) : undefined,
          keyFrames: new Map(),
          type: 'rotation',
        },
        scaling: {
          interpolation: wowToWc3Interpolation(bone.scale.interpolation),
          globalSeq: bone.scale.globalSeq !== BlizzardNull ? getGlobalSeq(bone.scale.globalSeq) : undefined,
          keyFrames: new Map(),
          type: 'scaling',
        },
        pivotPoint: [bone.pivot[0], -bone.pivot[2], bone.pivot[1]],
      };

      // https://wowdev.wiki/M2#Bones bone flags
      if (bone.flags & 0x1) mdlBone.flags.push(NodeFlag.DONTINHERIT_TRANSLATION);
      if (bone.flags & 0x2) mdlBone.flags.push(NodeFlag.DONTINHERIT_SCALING);
      if (bone.flags & 0x4) mdlBone.flags.push(NodeFlag.DONTINHERIT_ROTATION);
      if (bone.flags & 0x8) mdlBone.flags.push(NodeFlag.BILLBOARDED);
      if (bone.flags & 0x10) mdlBone.flags.push(NodeFlag.BILLBOARD_LOCK_Y); // X and Z are swapped?
      if (bone.flags & 0x20) mdlBone.flags.push(NodeFlag.BILLBOARD_LOCK_X);
      if (bone.flags & 0x40) mdlBone.flags.push(NodeFlag.BILLBOARD_LOCK_Z); // X and Z are swapped?

      const isValidTimestamps = (timestamps: number[] | null, animId: number): timestamps is number[] => {
        const isNull = timestamps == null;
        const isNotIncreasing = timestamps != null
          && timestamps.some((_, i) => i > 0 && timestamps[i] < timestamps[i - 1] || timestamps[i] > 9999999);
        const isInvalid = isNull || isNotIncreasing;
        if (isInvalid) {
          console.warn(`Invalid timestamps ${timestamps} for bone ${bone.boneNameCRC} in animation ${this.animations?.[animId]?.id}`, { isNull, isNotIncreasing});
        }
        return !isInvalid;
      }

      // Translation
      let accumTime = 0;
      bone.translation.timestamps.forEach((timestamps, animId) => {
        const animation = this.animations?.[animId];
        if (animation == null) {
          return;
        }

        const startTime = accumTime;
        accumTime += animation.duration + 1;

        if (excludedAnimation.has(animId)) {
          return;
        }

        if (!isValidTimestamps(timestamps, animId)) {
          if (timestamps != null) {
            excludedAnimation.add(animId);
          }
          return;
        }

        let maxTimestamp = -Infinity;
        timestamps.forEach((timestamp, timestampI) => {
          const values = bone.translation.values[animId][timestampI];
          if (values.length !== 3) {
            throw new Error(`Invalid Vector3 ${values.toString()} for bone ${bone.boneNameCRC} in animation ${this.animations?.[animId]?.id}`);
          }

          const [x, y, z] = values;
          if (x == null || y == null || z == null) return;
          if (Math.abs(x) > 999999 || Math.abs(y) > 999999 || Math.abs(z) > 999999) return;
          mdlBone.translation!.keyFrames.set(timestamp + startTime, [x, -z, y]);
          maxTimestamp = Math.max(maxTimestamp, timestamp + startTime);
        });
        if (maxTimestamp >= -1 && !mdlBone.translation!.globalSeq) {
          mdlBone.translation!.keyFrames.set(startTime + animation.duration, [...mdlBone.translation!.keyFrames.get(maxTimestamp)!]);
        }
      });
      // Rotation
      accumTime = 0;
      bone.rotation.timestamps.forEach((timestamps, animId) => {
        const animation = this.animations?.[animId];
        if (animation == null) {
          return;
        }
        const startTime = accumTime;
        accumTime += animation.duration + 1;

        if (excludedAnimation.has(animId)) {
          return;
        }
        if (!isValidTimestamps(timestamps, animId)) {
          if (timestamps != null) {
            excludedAnimation.add(animId);
          }
          return;
        }
        let maxTimestamp = -Infinity;
        timestamps.forEach((timestamp, timestampI) => {
          const [w, x, y, z] = bone.rotation.values[animId][timestampI];
          if (x == null || y == null || z == null) return;
          if (Math.abs(x) > 999999 || Math.abs(y) > 999999 || Math.abs(z) > 999999) return;
          mdlBone.rotation!.keyFrames.set(timestamp + startTime, [w, -y, x, z]);
          maxTimestamp = Math.max(maxTimestamp, timestamp + startTime);
        });
        if (maxTimestamp >= -1 && !mdlBone.rotation!.globalSeq) {
          mdlBone.rotation!.keyFrames.set(startTime + animation.duration, [...mdlBone.rotation!.keyFrames.get(maxTimestamp)!]);
        }
      });

      // Scaling
      accumTime = 0;
      bone.scale.timestamps.forEach((timestamps, animId) => {
        const animation = this.animations?.[animId];
        if (animation == null) {
          return;
        }
        const startTime = accumTime;
        accumTime += animation.duration + 1;

        if (excludedAnimation.has(animId)) {
          return;
        }
        if (!isValidTimestamps(timestamps, animId)) {
          if (timestamps != null) {
            excludedAnimation.add(animId);
          }
          return;
        }
        let maxTimestamp = -Infinity;
        timestamps.forEach((timestamp, timestampI) => {
          const [x, y, z] = bone.scale.values[animId][timestampI];
          if (x == null || y == null || z == null) return;
          if (Math.abs(x) > 999999 || Math.abs(y) > 999999 || Math.abs(z) > 999999) return;
          mdlBone.scaling!.keyFrames.set(timestamp + startTime, [x, z, y]);
          maxTimestamp = Math.max(maxTimestamp, timestamp + startTime);
        });
        if (maxTimestamp >= -1 && !mdlBone.scaling!.globalSeq) {
          mdlBone.scaling!.keyFrames.set(startTime + animation.duration, [...mdlBone.scaling!.keyFrames.get(maxTimestamp)!]);
        }
      });
      return mdlBone;
    });

    // Fill in parent and safeguard
    bones.forEach((mdlBone, i) => {
      const dataBone = this.bones[i];
      mdlBone.parent = dataBone.parentBone > -1 ? bones[dataBone.parentBone] : undefined;
      if (mdlBone.translation!.keyFrames.size + mdlBone.rotation!.keyFrames.size + mdlBone.scaling!.keyFrames.size === 0 && mdlBone.parent != null) {
        // This bone has parent, it must have some transformation, else WC3 will render crazy
        mdlBone.translation!.keyFrames.set(0, [0, 0, 0]); // no translation
      }
      if (!mdlBone.translation?.keyFrames.size) mdlBone.translation = undefined;
      if (!mdlBone.rotation?.keyFrames.size) mdlBone.rotation = undefined;
      if (!mdlBone.scaling?.keyFrames.size) mdlBone.scaling = undefined;
    });

    const skinWeights: SkinWeight[][] = [];
    for (let i = 0; i < (this.boneIndicies?.length ?? 0); i += 4) {
      const weights = this.boneWeights.slice(i, i + 4);
      assert.ok(_.sum(weights) > 0);
      const indices = this.boneIndicies.slice(i, i + 4);
      const skinWeight: SkinWeight[] = [];
      weights.forEach((w, j) => {
        if (w > 0) {
          skinWeight.push({
            bone: bones[indices[j]],
            weight: weights[j],
          });
        }
      });
      skinWeight.sort((w1, w2) => w2.weight - w1.weight);
      skinWeights.push(skinWeight);
    }

    // Compute `Sequences`
    let seqAccumTime = 0;
    const sequences: MDL['sequences'] = (this.animations ?? []).map((animation, animId) => {
      const wowAnimName = getWowAnimName(animation.id);
      const seqData = getWacraftSequenceData(animation);
      if (excludedAnimation.has(animId)) {
        seqAccumTime += animation.duration + 1;
        // console.log("Skip animation", seqData.wc3Name, "of", this.filePath)
        return null;
      }

      const sequence: typeof sequences[number] = {
        name: seqData.wc3Name,
        data: seqData,
        interval: [seqAccumTime, seqAccumTime + animation.duration],
        moveSpeed: animation.movespeed,
        minimumExtent: [animation.boxPosMin[0], -animation.boxPosMax[2], animation.boxPosMin[1]],
        maximumExtent: [animation.boxPosMax[0], -animation.boxPosMin[2], animation.boxPosMax[1]],
        boundsRadius: animation.boxRadius,
        nonLooping: !isLoopAnimation(wowAnimName),
      };
      seqAccumTime += animation.duration + 1;
      return sequence;
    }).filter((seq) => !!seq);

    // Find secondary Stand animations and set their rarity

    const standSeqs = sequences.filter((seq) => seq.data.wc3Name === 'Stand');
    if (standSeqs.length > 1) {
      const mainStandSeq = standSeqs.reduce((best, seq) => (seq.data.wowFrequency > best.data.wowFrequency ? seq : best));
      standSeqs.filter((seq) => seq !== mainStandSeq).forEach((seq) => seq.rarity = 4);
    }

    if (sequences.length === 0) {
      // Model with sequence will crash Wc3
      sequences.push({
        name: 'Stand',
        data: {
          wowName: '', attackTag: '', wc3Name: 'Stand', wowVariant: 0, wowFrequency: 0,
        },
        interval: [0, 1000],
        moveSpeed: 0,
        minimumExtent: [-1, -1, -1],
        maximumExtent: [1, 1, 1],
        boundsRadius: 1,
        nonLooping: false,
      });
    }

    const wowAttachments = this.extractWowAttachments(bones);
    debug && console.log('AnimationFile toMdl took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

    return {
      sequences,
      bones,
      skinWeights,
      wowAttachments,
    };
  }

  private extractWowAttachments(bones: Bone[]): WowAttachment[] {
    if (!this.isLoaded || !this.attachments) {
      return [];
    }

    const result: WowAttachment[] = [];

    this.attachments.forEach((attachment) => {
      const parent = bones[attachment.bone];
      if (!parent) {
        console.warn(`Attachment ${attachment.id} references non-existent bone ${attachment.bone}`);
        return;
      }

      const [x, y, z] = attachment.position;
      result.push({
        wowAttachmentId: attachment.id,
        bone: parent,
        pivotPoint: [x, -z, y],
      });
    });

    return result;
  }
}

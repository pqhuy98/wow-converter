import _ from 'lodash';

import { Vector3 } from '@/lib/math/common';

import { WowAnimName } from '../../animation/animation_mapper';
import { Animation } from '../components/animation';
import { Sequence } from '../components/sequence';
import { interpolateTransformQuat } from '../mdl-traverse';
import { MDLModify } from '.';

const animPrefixOrder = ['Stand', 'Walk', 'Attack', 'Spell', 'Death', 'Decay'];

function getPrefixIndex(str: string): number {
  for (let i = 0; i < animPrefixOrder.length; i++) {
    if (str.startsWith(animPrefixOrder[i])) {
      return i;
    }
  }
  return animPrefixOrder.length; // Return a large index for strings without any of the prefixes
}

function animNameAsc(a: Sequence, b: Sequence): number {
  if (a.name === b.name) {
    return (a.rarity ?? 0) - (b.rarity ?? 0);
  }
  const indexA = getPrefixIndex(a.name);
  const indexB = getPrefixIndex(b.name);
  if (indexA !== indexB) {
    return indexA - indexB; // Sort by prefix order
  }

  return a.name.localeCompare(b.name); // Sort alphabetically if prefixes are the same
}

export function sortSequences(this: MDLModify) {
  this.mdl.sequences.sort(animNameAsc);
  return this;
}

export function removeWowSequence(this: MDLModify, wowName: string, variant?: number) {
  this.mdl.sequences = this.mdl.sequences.filter((s) => !(s.data.wowName === wowName && (variant == null || s.data.wowVariant === variant)));
  return this;
}

export function useWalkSequenceByWowName(this: MDLModify, wowName: WowAnimName) {
  const seq = this.mdl.sequences.find((s) => s.data.wowName === wowName);
  if (seq) {
    const walkSeq = this.mdl.sequences.find((s) => s.name === 'Walk');
    if (walkSeq) {
      walkSeq.name = `Cinematic ${walkSeq.data.wowName}`;
    }
    seq.name = 'Walk';
  }
  return this;
}

export function renameSequencesByWowName(this: MDLModify, wowName: WowAnimName, wc3Name: string) {
  const matchingSequences = this.mdl.sequences.filter((s) => s.data.wowName === wowName);
  matchingSequences.forEach((seq) => seq.name = seq.data.wc3Name = wc3Name);
  return this;
}

export function debugSequence(this: MDLModify) {
  this.mdl.sequences.forEach((s) => s.name += ` ${s.data.wowName} ${s.data.wowVariant}`);
  return this;
}

export function addEventObjectBySequenceName(this: MDLModify, name: string, sequenceName: string, offset: number) {
  let event = this.mdl.eventObjects.find((e) => e.name === name);
  this.mdl.sequences.filter((s) => s.name === sequenceName).forEach((sequence) => {
    if (!event) {
      event = {
        name, track: [], pivotPoint: [0, 0, 0], flags: [], type: 'EventObject',
      };
      this.mdl.eventObjects.push(event);
    }
    event.track.push({ sequence, offset });
  });
  return this;
}

export function addDecayAnimation(this: MDLModify) {
  const decayDuration = 60000;
  const offsetDuration = 2 * (decayDuration + 1);

  const deathSequence = this.mdl.sequences.find((seq) => seq.name === 'Death');
  if (!deathSequence) return this;
  const deathTimestamp = deathSequence.interval[1];

  // Move all keyFrame up by `offsetDuration`
  const updateKeyFrame = <T>(keyFrame: Map<number, T>) => {
    const newKeyFrame = new Map<number, T>();
    [...keyFrame.keys()].forEach((timestamp) => {
      if (timestamp <= deathTimestamp) {
        newKeyFrame.set(timestamp, keyFrame.get(timestamp)!);
        return;
      }
      const newTimestamp = timestamp + offsetDuration;
      newKeyFrame.set(newTimestamp, keyFrame.get(timestamp)!);
    });
    return newKeyFrame;
  };

  this.mdl.bones.forEach((bone) => {
    if (bone.translation && !bone.translation.globalSeq) bone.translation.keyFrames = updateKeyFrame(bone.translation.keyFrames);
    if (bone.scaling && !bone.scaling.globalSeq) bone.scaling.keyFrames = updateKeyFrame(bone.scaling.keyFrames);
    if (bone.rotation && !bone.rotation.globalSeq) bone.rotation.keyFrames = updateKeyFrame(bone.rotation.keyFrames);
  });
  this.mdl.textureAnims.forEach((texAnim) => {
    if (texAnim.translation && !texAnim.translation.globalSeq) texAnim.translation.keyFrames = updateKeyFrame(texAnim.translation.keyFrames);
    if (texAnim.scaling && !texAnim.scaling.globalSeq) texAnim.scaling.keyFrames = updateKeyFrame(texAnim.scaling.keyFrames);
    if (texAnim.rotation && !texAnim.rotation.globalSeq) texAnim.rotation.keyFrames = updateKeyFrame(texAnim.rotation.keyFrames);
  });
  this.mdl.geosetAnims.forEach((geosetAnim) => {
    if (geosetAnim.alpha) {
      if ('keyFrames' in geosetAnim.alpha) {
        geosetAnim.alpha.keyFrames = updateKeyFrame(geosetAnim.alpha.keyFrames);
      }
    }
  });
  this.mdl.sequences.filter((seq) => seq.interval[0] > deathTimestamp).forEach((seq) => {
    seq.interval[0] += offsetDuration;
    seq.interval[1] += offsetDuration;
  });

  const decayFleshSequence: Sequence = {
    name: 'Decay Flesh',
    data: {
      wc3Name: 'Decay Flesh',
      wowName: '',
      wowVariant: 0,
      attackTag: '',
      wowFrequency: 1,
    },
    interval: [deathTimestamp + 1, deathTimestamp + 1 + decayDuration],
    nonLooping: true,
    moveSpeed: 0,
    minimumExtent: deathSequence.minimumExtent,
    maximumExtent: deathSequence.maximumExtent,
    boundsRadius: deathSequence.boundsRadius,
  };
  const decayBoneSequence: Sequence = {
    name: 'Decay Bone',
    data: {
      wc3Name: 'Decay Bone',
      wowName: '',
      wowVariant: 0,
      attackTag: '',
      wowFrequency: 1,
    },
    interval: [decayFleshSequence.interval[1] + 1, decayFleshSequence.interval[1] + 1 + decayDuration],
    nonLooping: true,
    moveSpeed: 0,
    minimumExtent: deathSequence.minimumExtent,
    maximumExtent: deathSequence.maximumExtent,
    boundsRadius: deathSequence.boundsRadius,
  };
  this.mdl.sequences.push(decayFleshSequence);
  this.mdl.sequences.push(decayBoneSequence);

  // Copy geoset animation
  const updateAnimKeyFrames = <T>(anim: Animation<T>, timestampFrom: number, timestampTo: number) => {
    const value = anim.keyFrames.get(timestampFrom);
    if (value) {
      anim.keyFrames.set(timestampTo, _.cloneDeep(value));
    }
  };
  const copyAnimKeyFrames = <T>(anim: Animation<T>) => {
    if (anim.globalSeq) return;
    updateAnimKeyFrames(anim, deathTimestamp, decayFleshSequence.interval[0]);
    updateAnimKeyFrames(anim, deathTimestamp, decayFleshSequence.interval[1]);
    updateAnimKeyFrames(anim, deathTimestamp, decayBoneSequence.interval[0]);
    updateAnimKeyFrames(anim, deathTimestamp, decayBoneSequence.interval[1]);
  };

  this.mdl.geosetAnims.forEach((geosetAnim) => {
    if (geosetAnim.alpha && 'keyFrames' in geosetAnim.alpha && !geosetAnim.alpha.globalSeq) {
      copyAnimKeyFrames(geosetAnim.alpha);
    }
    if (geosetAnim.color && 'keyFrames' in geosetAnim.color && !geosetAnim.color.globalSeq) {
      copyAnimKeyFrames(geosetAnim.color);
    }
  });

  // Copy texture anim
  this.mdl.textureAnims.forEach((texAnim) => {
    if (texAnim.translation && !texAnim.translation.globalSeq) {
      copyAnimKeyFrames(texAnim.translation);
    }
    if (texAnim.rotation && !texAnim.rotation.globalSeq) {
      copyAnimKeyFrames(texAnim.rotation);
    }
    if (texAnim.scaling && !texAnim.scaling.globalSeq) {
      copyAnimKeyFrames(texAnim.scaling);
    }
  });

  // Disable particle emitters
  this.mdl.particleEmitter2s.forEach((p) => {
    if (p.visibility && p.visibility.globalSeq) return;
    if (!p.visibility) {
      p.visibility = {
        interpolation: 'Linear',
        keyFrames: new Map(),
        type: 'others',
      };
      this.mdl.sequences.filter((s) => ![decayFleshSequence, decayBoneSequence].includes(s)).forEach((s) => {
        p.visibility!.keyFrames.set(s.interval[0], 1);
        p.visibility!.keyFrames.set(s.interval[1], 1);
      });
    }
    p.visibility.keyFrames.set(decayFleshSequence.interval[0], 0);
    p.visibility.keyFrames.set(decayFleshSequence.interval[1], 0);
    p.visibility.keyFrames.set(decayBoneSequence.interval[0], 0);
    p.visibility.keyFrames.set(decayBoneSequence.interval[1], 0);
  });

  // Find the highest vertex in Death animation
  const maxZ = Math.max(0, this.getMaxZAtTimestamp(deathSequence, deathSequence.interval[1] - deathSequence.interval[0]));

  // Translate bones linearly downward to the ground
  this.mdl.bones.forEach((bone) => {
    // if (bone.name === "Main") return
    const value = interpolateTransformQuat(bone, deathSequence, deathTimestamp);
    if (bone.translation && !bone.translation.globalSeq) {
      bone.translation.keyFrames.set(decayFleshSequence.interval[0], [...value.position]);
      bone.translation.keyFrames.set(decayFleshSequence.interval[1], [...value.position]);
      bone.translation.keyFrames.set(decayBoneSequence.interval[0], [...value.position]);
      bone.translation.keyFrames.set(decayBoneSequence.interval[1], [...value.position]);
    }
    if (bone.rotation && !bone.rotation.globalSeq) {
      bone.rotation.keyFrames.set(decayFleshSequence.interval[0], [...value.rotation]);
      bone.rotation.keyFrames.set(decayFleshSequence.interval[1], [...value.rotation]);
      bone.rotation.keyFrames.set(decayBoneSequence.interval[0], [...value.rotation]);
      bone.rotation.keyFrames.set(decayBoneSequence.interval[1], [...value.rotation]);
    }
    if (bone.scaling && !bone.scaling.globalSeq) {
      bone.scaling.keyFrames.set(decayFleshSequence.interval[0], [...value.scaling]);
      bone.scaling.keyFrames.set(decayFleshSequence.interval[1], [...value.scaling]);
      bone.scaling.keyFrames.set(decayBoneSequence.interval[0], [...value.scaling]);
      bone.scaling.keyFrames.set(decayBoneSequence.interval[1], [...value.scaling]);
    }
    // Root bones will sink to the ground
    if (!bone.parent && (!bone.translation || !bone.translation.globalSeq)) {
      if (!bone.translation) {
        bone.translation = {
          interpolation: 'Linear',
          keyFrames: new Map(),
          type: 'translation',
        };
      }
      bone.translation.keyFrames.set(decayFleshSequence.interval[0], [...value.position]);
      bone.translation.keyFrames.set(decayFleshSequence.interval[1], [...value.position]);
      bone.translation.keyFrames.set(decayBoneSequence.interval[0], [...value.position]);
      const translation: Vector3 = [...value.position];
      translation[2] -= maxZ;
      bone.translation.keyFrames.set(decayBoneSequence.interval[1], translation);
    }
  });
  return this;
}

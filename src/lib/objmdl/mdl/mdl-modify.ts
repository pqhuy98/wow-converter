import chalk from 'chalk';
import _ from 'lodash';
import path from 'path';

import { EulerRotation, Vector2, Vector3 } from '../../math/common';
import { V3 } from '../../math/vector';
import { WowAnimName } from '../animation/animation_mapper';
import { WoWAttachmentID, WoWToWC3AttachmentMap } from '../animation/bones_mapper';
import {
  Bone, Extents, Face, GeosetVertex, GlobalSequence, Material, MDL, Node, Sequence, SkinWeight, TransformAnimation,
} from './mdl';
import {
  buildNodesChildrenList, interpolateTransformQuat, iterateNodesAtTimestamp, Value,
} from './mdl-traverse';

export class MDLModify {
  constructor(public mdl: MDL) {
  }

  setLargeExtents() {
    this.mdl.extendsOverriden = (obj: Extents) => {
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

  setInfiniteExtents() {
    this.mdl.extendsOverriden = (obj: Extents) => {
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

  scale(value: number) {
    this.mdl.geosets.forEach((geoset) => {
      geoset.vertices.forEach((vertex) => {
        vertex.position[0] *= value;
        vertex.position[1] *= value;
        vertex.position[2] *= value;
      });
    });
    this.mdl.bones.forEach((bone) => {
      if (!bone.translation) return;
      [...bone.translation.keyFrames.values()].forEach((translation) => {
        translation[0] *= value;
        translation[1] *= value;
        translation[2] *= value;
      });
    });
    [
      ...this.mdl.bones,
      ...this.mdl.attachmentPoints,
      ...this.mdl.wowAttachments,
    ].forEach(({ pivotPoint: p }) => {
      p[0] *= value;
      p[1] *= value;
      p[2] *= value;
    });
    this.mdl.cameras.forEach((cam) => {
      cam.position = V3.scale(cam.position, value);
      cam.target.position = V3.scale(cam.target.position, value);
    });
    this.mdl.sequences.forEach((seq) => seq.movementSpeed *= value);
    return this;
  }

  translate(delta: Vector3) {
    this.mdl.geosets.forEach((geoset) => {
      geoset.vertices.forEach((vertex) => {
        vertex.position = V3.sum(vertex.position, delta);
      });
    });
    [...this.mdl.bones, ...this.mdl.attachmentPoints].forEach((node) => {
      node.pivotPoint = V3.sum(node.pivotPoint, delta);
    });
    this.mdl.cameras.forEach((cam) => {
      cam.position = V3.sum(cam.position, delta);
      cam.target.position = V3.sum(cam.target.position, delta);
    });
    this.mdl.sync();
    return this;
  }

  rotate(eulerRotation: EulerRotation) {
    this.mdl.geosets.forEach((geoset) => {
      geoset.vertices.forEach((vertex) => {
        vertex.position = V3.rotate(vertex.position, eulerRotation);
      });
    });
    [...this.mdl.bones, ...this.mdl.attachmentPoints].forEach((node) => {
      node.pivotPoint = V3.rotate(node.pivotPoint, eulerRotation);
    });
    this.mdl.cameras.forEach((cam) => {
      cam.position = V3.rotate(cam.position, eulerRotation);
      cam.target.position = V3.rotate(cam.target.position, eulerRotation);
    });
    this.mdl.sync();
    return this;
  }

  updateGlobalSequenceDuration(globalSeq: GlobalSequence, ...values: number[]) {
    globalSeq.duration = Math.max(globalSeq.duration, ...values);
    return this;
  }

  addWc3AttachmentPoint() {
    // Only map WoW attachment points that have a valid WC3 equivalent.
    // E.g. WoW ArmL/ArmR do not exist in WC3, so we use Medium/Large as proxies.
    this.mdl.wowAttachments.forEach((wowAttachment) => {
      const bone = wowAttachment.bone;
      const wowAttachmentId = wowAttachment.wowAttachmentId as WoWAttachmentID;
      const wc3Key = WoWToWC3AttachmentMap[wowAttachmentId];
      if (wc3Key) {
        this.mdl.attachmentPoints.push({
          attachmentId: 0,
          type: 'AttachmentPoint',
          name: `${wc3Key} Ref`,
          parent: bone,
          pivotPoint: wowAttachment.pivotPoint,
          flags: [],
        });
      }
    });
    return this;
  }

  addPortraitCamera(standSequenceName: string = 'Stand') {
    const cameraBone = this.mdl.bones.find((b) => b.name === 'Head')
      || this.mdl.bones.find((b) => b.name === 'Chest')
      || this.mdl.bones.find((b) => b.name === 'Root');
    if (!cameraBone) {
      // Generate default camera
      this.mdl.cameras.push({
        name: 'Portrait_Camera',
        fieldOfView: 1,
        nearClip: 0.1,
        farClip: 10000,
        target: {
          position: V3.mean(this.mdl.model.minimumExtent, this.mdl.model.maximumExtent),
        },
        position: [...V3.scale([
          this.mdl.model.minimumExtent[0],
          this.mdl.model.maximumExtent[1],
          this.mdl.model.maximumExtent[2],
        ], 1.1)],
      });

      return this;
    }

    let nodePos: Vector3 = cameraBone.pivotPoint;
    const standSequence = this.mdl.sequences.find((seq) => seq.name === standSequenceName);

    // Find actual position of cameraBone during Stand animation
    if (standSequence) {
      iterateNodesAtTimestamp(this.mdl, standSequence, standSequence.interval[0], (node, value) => {
        if (node === cameraBone) {
          nodePos = value.position;
        }
      });
    }

    const distanceScale = {
      Head: 3,
      Chest: 2,
      Root: 1.5,
    }[cameraBone.name]!;

    const cameraPosition = V3.sum(nodePos, [
      distanceScale * this.mdl.model.maximumExtent[0],
      0.5 * (Math.random() - 0.5) * this.mdl.model.maximumExtent[1],
      (Math.random() * 0.2 - 0.1) * this.mdl.model.maximumExtent[2],
    ]);
    // console.log('Add portrait camera looking at bone', cameraBone.name);
    // console.log('Target position', nodePos);
    // console.log('Camera position', cameraPosition);

    this.mdl.cameras.push({
      name: 'Portrait_Camera',
      fieldOfView: 1,
      nearClip: 0.1,
      farClip: 10000,
      target: {
        position: nodePos,
      },
      position: cameraPosition,
    });
    return this;
  }

  setWowAttachmentScale(wowAttachmentId: WoWAttachmentID, scale: number) {
    const attachment = this.mdl.wowAttachments.find((a) => a.wowAttachmentId === wowAttachmentId);
    if (!attachment) {
      console.error(chalk.red(`Cannot find wow attachment ${wowAttachmentId}`));
      return this;
    }
    attachment.bone.scaling = {
      interpolation: 'DontInterp',
      keyFrames: new Map(this.mdl.sequences.map((s) => <[number, Vector3]>[s.interval[0], [scale, scale, scale]])),
    };
    return this;
  }

  sortSequences() {
    this.mdl.sequences.sort(animNameAsc);
    return this;
  }

  optimizeKeyFrames() {
    // Pre-compute sequence intervals once so every key-frame test is O(1)
    const seqIntervals = this.mdl.sequences
      .map((s) => [s.interval[0], s.interval[1]] as const)
      .sort((a, b) => a[0] - b[0]);

    // Cursor-based helper – much cheaper than Array.some/Array.find each time.
    const inSequence = (timestamp: number, cursor: { idx: number }): boolean => {
      let i = cursor.idx;
      while (i < seqIntervals.length && seqIntervals[i][1] < timestamp) i++;
      cursor.idx = i;
      return i < seqIntervals.length && seqIntervals[i][0] <= timestamp;
    };

    const optimiseAnim = <T extends number[]>(anim: TransformAnimation<T>, threshold: number) => {
      if (!anim || anim.keyFrames.size <= 2) return; // nothing to prune

      const times = Array.from(anim.keyFrames.keys()).sort((a, b) => a - b);
      let prevT = times[0];
      const cursor = { idx: 0 };

      for (let k = 1; k < times.length; k++) {
        const t = times[k];
        const v1 = anim.keyFrames.get(t)!;
        const v0 = anim.keyFrames.get(prevT)!;

        // Early-exit diff calculation
        let diff = 0;
        for (let j = 0; j < v1.length && diff < threshold; j++) diff += Math.abs(v1[j] - v0[j]);

        if (diff >= threshold) {
          prevT = t;
          continue; // keep – movement above threshold
        }

        const inside = inSequence(t, cursor);

        let firstFrame = false;
        for (let sIdx = 0; sIdx < seqIntervals.length; sIdx++) {
          const sStart = seqIntervals[sIdx][0];
          if (prevT < sStart && sStart <= t) { firstFrame = true; break; }
        }

        const nextT = k + 1 < times.length ? times[k + 1] : Number.POSITIVE_INFINITY;
        let lastFrame = k === times.length - 1;
        if (!lastFrame) {
          for (let sIdx = 0; sIdx < seqIntervals.length; sIdx++) {
            const sEnd = seqIntervals[sIdx][1];
            if (t <= sEnd && sEnd < nextT) { lastFrame = true; break; }
          }
        }

        if (!inside || (!firstFrame && !lastFrame)) {
          // Prune – insignificant change inside idle segment
          anim.keyFrames.delete(t);
        } else {
          prevT = t; // keep – important boundary frame
        }
      }
    };

    for (const bone of this.mdl.bones) {
      if (bone.translation && !bone.translation.globalSeq) optimiseAnim(bone.translation, 0.005);
      if (bone.rotation && !bone.rotation.globalSeq) optimiseAnim(bone.rotation, 0.001);
      if (bone.scaling && !bone.scaling.globalSeq) optimiseAnim(bone.scaling, 0.01);
    }

    return this;
  }

  removeUnusedMaterialsTextures() {
    const usedMaterials = new Set<Material>(
      this.mdl.geosets.filter((geoset) => geoset.vertices.length > 0)
        .map((geoset) => geoset.material),
    );

    const usedTextures = new Set([...usedMaterials].flatMap((mat) => mat.layers.map((layer) => layer.texture)));

    this.mdl.materials = this.mdl.materials.filter((mat) => usedMaterials.has(mat));
    this.mdl.textures = this.mdl.textures.filter((tex) => usedTextures.has(tex));
    return this;
  }

  removeUnusedNodes() {
    const usedNodes = new Set<Node>([...this.mdl.attachmentPoints]);
    this.mdl.geosets.forEach((geoset) => geoset.vertices.forEach((v) => {
      v.skinWeights?.forEach((sw) => usedNodes.add(sw.bone));
      v.matrix?.bones.forEach((b) => usedNodes.add(b));
    }));

    const childrenList = buildNodesChildrenList(this.mdl);
    const dfs = (cur: Node) => {
      let isUsed = usedNodes.has(cur);
      // console.log(cur.name, "children:", childrenList.get(cur)!.map(n => n.name).join(", "))
      for (const child of childrenList.get(cur)!) {
        const childUsed = dfs(child);
        isUsed = childUsed || isUsed;
      }
      if (isUsed) usedNodes.add(cur);
      return isUsed;
    };
    this.mdl.bones.forEach((b) => {
      if (!b.parent) dfs(b);
    });
    this.mdl.bones = this.mdl.bones.filter((b) => usedNodes.has(b));
    return this;
  }

  removeUnusedVertices() {
    this.mdl.geosets.forEach((geoset) => {
      const usedNodes = new Set(geoset.faces.flatMap((face) => face.vertices));
      geoset.vertices = geoset.vertices.filter((v) => usedNodes.has(v));
    });
    this.mdl.syncExtends();
    return this;
  }

  scaleSequenceDuration(sequence: Sequence, scalingFactor: number) {
    const durationOffset = Math.floor((sequence.interval[1] - sequence.interval[0]) * (scalingFactor - 1));

    const updateKeyFrame = <T>(keyFrame: Map<number, T>) => {
      const newKeyFrame = new Map<number, T>();
      [...keyFrame.keys()].forEach((timestamp) => {
        if (timestamp <= sequence.interval[0]) {
          newKeyFrame.set(timestamp, keyFrame.get(timestamp)!);
          return;
        }
        let newTimestamp = timestamp;
        if (timestamp <= sequence.interval[1]) {
          newTimestamp = (timestamp - sequence.interval[0]) * scalingFactor + sequence.interval[0];
        } else {
          newTimestamp = timestamp + durationOffset;
        }
        newTimestamp = Math.floor(newTimestamp);
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
      if (texAnim.rotation && !texAnim.rotation.globalSeq) texAnim.rotation.keyFrames = updateKeyFrame(texAnim.rotation.keyFrames);
      if (texAnim.scaling && !texAnim.scaling.globalSeq) texAnim.scaling.keyFrames = updateKeyFrame(texAnim.scaling.keyFrames);
    });
    this.mdl.geosetAnims.forEach((geosetAnim) => {
      if (geosetAnim.alpha) {
        if ('keyFrames' in geosetAnim.alpha) {
          geosetAnim.alpha.keyFrames = updateKeyFrame(geosetAnim.alpha.keyFrames);
        }
      }
    });
    this.mdl.sequences.filter((seq) => seq.interval[0] > sequence.interval[1]).forEach((seq) => {
      seq.interval[0] += durationOffset;
      seq.interval[1] += durationOffset;
    });
    sequence.interval[1] += durationOffset;
    return this;
  }

  addMdlItemToBone(item: MDL, boneName: string) {
    const attachmentBone = this.mdl.bones.find((b) => b.name === boneName);
    if (!attachmentBone) {
      console.error(chalk.red(`Cannot find bone "${boneName}" for attachment.`));
      return this;
    }
    console.log('Attach item', path.basename(item.model.name), 'to bone', attachmentBone.name);

    item.bones.forEach((b) => {
      if (!b.parent) {
        b.parent = attachmentBone;
        b.pivotPoint = V3.sum(b.pivotPoint, attachmentBone.pivotPoint);
        b.translation = {
          interpolation: 'DontInterp',
          keyFrames: new Map([[0, [0, 0, 0]]]),
        };
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
    return this;
  }

  addDecayAnimation() {
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
      movementSpeed: 0,
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
      movementSpeed: 0,
      minimumExtent: deathSequence.minimumExtent,
      maximumExtent: deathSequence.maximumExtent,
      boundsRadius: deathSequence.boundsRadius,
    };
    this.mdl.sequences.push(decayFleshSequence);
    this.mdl.sequences.push(decayBoneSequence);
    this.mdl.sequences.sort(animNameAsc);

    // Copy geoset animation
    this.mdl.geosetAnims.forEach((geosetAnim) => {
      if (geosetAnim.alpha && 'keyFrames' in geosetAnim.alpha) {
        geosetAnim.alpha.keyFrames.set(decayFleshSequence.interval[0], geosetAnim.alpha.keyFrames.get(deathTimestamp)!);
        geosetAnim.alpha.keyFrames.set(decayFleshSequence.interval[1], geosetAnim.alpha.keyFrames.get(deathTimestamp)!);
        geosetAnim.alpha.keyFrames.set(decayBoneSequence.interval[0], geosetAnim.alpha.keyFrames.get(deathTimestamp)!);
        geosetAnim.alpha.keyFrames.set(decayBoneSequence.interval[1], geosetAnim.alpha.keyFrames.get(deathTimestamp)!);
      }
      if (geosetAnim.color && 'keyFrames' in geosetAnim.color) {
        geosetAnim.color.keyFrames.set(decayFleshSequence.interval[0], [...geosetAnim.color.keyFrames.get(deathTimestamp)!]);
        geosetAnim.color.keyFrames.set(decayFleshSequence.interval[1], [...geosetAnim.color.keyFrames.get(deathTimestamp)!]);
        geosetAnim.color.keyFrames.set(decayBoneSequence.interval[0], [...geosetAnim.color.keyFrames.get(deathTimestamp)!]);
        geosetAnim.color.keyFrames.set(decayBoneSequence.interval[1], [...geosetAnim.color.keyFrames.get(deathTimestamp)!]);
      }
    });

    // Copy texture anim
    this.mdl.textureAnims.forEach((texAnim) => {
      if (texAnim.translation && !texAnim.translation.globalSeq && texAnim.translation.keyFrames.get(deathTimestamp)) {
        texAnim.translation.keyFrames.set(decayFleshSequence.interval[0], [...texAnim.translation.keyFrames.get(deathTimestamp)!]);
        texAnim.translation.keyFrames.set(decayFleshSequence.interval[1], [...texAnim.translation.keyFrames.get(deathTimestamp)!]);
        texAnim.translation.keyFrames.set(decayBoneSequence.interval[0], [...texAnim.translation.keyFrames.get(deathTimestamp)!]);
        texAnim.translation.keyFrames.set(decayBoneSequence.interval[1], [...texAnim.translation.keyFrames.get(deathTimestamp)!]);
      }
      if (texAnim.rotation && !texAnim.rotation.globalSeq && texAnim.rotation.keyFrames.get(deathTimestamp)) {
        texAnim.rotation.keyFrames.set(decayFleshSequence.interval[0], [...texAnim.rotation.keyFrames.get(deathTimestamp)!]);
        texAnim.rotation.keyFrames.set(decayFleshSequence.interval[1], [...texAnim.rotation.keyFrames.get(deathTimestamp)!]);
      }
      if (texAnim.scaling && !texAnim.scaling.globalSeq && texAnim.scaling.keyFrames.get(deathTimestamp)) {
        texAnim.scaling.keyFrames.set(decayFleshSequence.interval[0], [...texAnim.scaling.keyFrames.get(deathTimestamp)!]);
        texAnim.scaling.keyFrames.set(decayFleshSequence.interval[1], [...texAnim.scaling.keyFrames.get(deathTimestamp)!]);
        texAnim.scaling.keyFrames.set(decayBoneSequence.interval[0], [...texAnim.scaling.keyFrames.get(deathTimestamp)!]);
        texAnim.scaling.keyFrames.set(decayBoneSequence.interval[1], [...texAnim.scaling.keyFrames.get(deathTimestamp)!]);
      }
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

    // const k = JSON.stringify
    // this.mdl.bones.forEach(b => {
    //   if (k(b.translation?.keyFrames.get(deathTimestamp)) !== k(b.translation?.keyFrames.get(decaySequence.interval[0]))) {
    //     console.log("bone", b.name, "translation", b.translation?.keyFrames.get(deathTimestamp), b.translation?.keyFrames.get(decaySequence.interval[0]))
    //   }
    //   if (k(b.rotation?.keyFrames.get(deathTimestamp)) !== k(b.rotation?.keyFrames.get(decaySequence.interval[0]))) {
    //     console.log("bone", b.name, "rotation", b.rotation?.keyFrames.get(deathTimestamp), b.rotation?.keyFrames.get(decaySequence.interval[0]))
    //   }
    //   if (k(b.scaling?.keyFrames.get(deathTimestamp)) !== k(b.scaling?.keyFrames.get(decaySequence.interval[0]))) {
    //     console.log("bone", b.name, "scaling", b.scaling?.keyFrames.get(deathTimestamp), b.scaling?.keyFrames.get(decaySequence.interval[0]))
    //   }
    // })
    return this;
  }

  getMaxZAtTimestamp(sequence: Sequence, offset: number) {
    // Find the highest vertex in the animation
    let maxZ = -Infinity;
    const nodeValues = new Map<Node, Value>();
    iterateNodesAtTimestamp(this.mdl, sequence, sequence.interval[0] + offset, (node, value) => {
      nodeValues.set(node, value);
    });
    this.mdl.geosets.forEach((geoset) => geoset.vertices.forEach((v) => {
      let translation: Vector3 = [0, 0, 0];
      v.skinWeights?.forEach(({ bone, weight }) => {
        const boneValue = nodeValues.get(bone)!;
        const vPosDeltaToBone = V3.mul(V3.rotate(V3.sub(v.position, bone.pivotPoint), boneValue.rotation), boneValue.scaling);
        const vPos = V3.sum(boneValue.position, vPosDeltaToBone);
        const vPosDelta = V3.sub(vPos, v.position);
        translation = V3.sum(translation, V3.scale(vPosDelta, weight / 255));
      });
      const vPos = V3.sum(v.position, translation);
      maxZ = Math.max(maxZ, vPos[2]);
    }));
    return maxZ;
  }

  estimateAttackDamagePoint() {
    // loop through all Attack sequences, get timestamp when max X of all vertices at timestamp 0.1, 0.2,... of the sequence
    // interval.
    // return the average of the timestamps
    const timestamps: number[] = [];
    this.mdl.sequences.filter((s) => /$Attack [0-9]+^/.test(s.name)).forEach((s) => {
      console.log(s.name);
      let maxX = -Infinity;
      let maxXTimestamp = 0;
      const tstamps = [0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9];
      tstamps.forEach((t) => {
        const timestamp = t * (s.interval[1] - s.interval[0]);
        iterateNodesAtTimestamp(this.mdl, s, timestamp, (node, value) => {
          if (value.position[0] > maxX) {
            maxX = value.position[0];
            maxXTimestamp = timestamp;
          }
        });
      });
      timestamps.push(maxXTimestamp);
    });
    return timestamps.reduce((a, b) => a + b, 0) / timestamps.length;
  }

  removeWowSequence(wowName: string, variant?: number) {
    this.mdl.sequences = this.mdl.sequences.filter((s) => !(s.data.wowName === wowName && (variant == null || s.data.wowVariant === variant)));
    return this;
  }

  useWalkSequenceByWowName(wowName: WowAnimName) {
    const walkSeq = this.mdl.sequences.find((s) => s.name === 'Walk');
    if (walkSeq) {
      walkSeq.name = `Cinematic ${walkSeq.data.wowName}`;
    }
    const seq = this.mdl.sequences.find((s) => s.data.wowName === wowName);
    if (seq) {
      seq.name = 'Walk';
    }
    return this;
  }

  renameSequencesByWowName(wowName: WowAnimName, wc3Name: string) {
    const matchingSequences = this.mdl.sequences.filter((s) => s.data.wowName === wowName);
    matchingSequences.forEach((seq) => seq.name = seq.data.wc3Name = wc3Name);
    return this;
  }

  removeCinematicSequences() {
    this.mdl.sequences = this.mdl.sequences.filter((seq) => !seq.name.includes('Cinematic') || seq.keep);
    return this;
  }

  debugSequence() {
    this.mdl.sequences.forEach((s) => s.name += ` ${s.data.wowName} ${s.data.wowVariant}`);
    return this;
  }

  addEventObjectBySequenceName(name: string, sequenceName: string, offset: number) {
    let event = this.mdl.eventObjects.find((e) => e.name === name);
    this.mdl.sequences.filter((s) => s.name === sequenceName).forEach((sequence) => {
      if (!event) {
        event = { name, track: [], pivotPoint: [0, 0, 0] };
        this.mdl.eventObjects.push(event);
      }
      event.track.push({ sequence, offset });
    });
    return this;
  }

  deleteVerticesIf(
    shouldDeleteVert: (v: GeosetVertex) => boolean,
    resolvePartialFace?: (f: Face) => Face[],
  ) {
    this.mdl.geosets.forEach((geoset) => {
      let verts = new Set<GeosetVertex>(geoset.vertices);
      const faces = new Set<Face>(geoset.faces);
      geoset.vertices.forEach((vert) => {
        if (shouldDeleteVert?.(vert)) verts.delete(vert);
      });
      geoset.faces.forEach((face) => {
        if (face.vertices.some((v) => !verts.has(v))) {
          faces.delete(face);
          const newFaces = resolvePartialFace?.(face);
          newFaces?.forEach((newFace) => {
            faces.add(newFace);
            newFace.vertices.forEach((v) => verts.add(v));
          });
        }
      });
      verts = new Set([...faces].flatMap((face) => face.vertices));

      geoset.vertices = [...verts];
      geoset.faces = [...faces];
    });
    return this;
  }

  deleteVerticesOutsideBox(low: Vector3, high: Vector3) {
    const shouldDeleteVert = (vert: GeosetVertex) => vert.position[0] < low[0]
      || vert.position[1] < low[1]
      || vert.position[2] < low[2]
      || vert.position[0] > high[0]
      || vert.position[1] > high[1]
      || vert.position[2] > high[2];

    const resolvePartialFace = (face: Face): Face[] => {
      const input = [...face.vertices];
      const planes: Plane[] = [
        { axis: 0, min: true, value: low[0] },
        { axis: 0, min: false, value: high[0] },
        { axis: 1, min: true, value: low[1] },
        { axis: 1, min: false, value: high[1] },
        { axis: 2, min: true, value: low[2] },
        { axis: 2, min: false, value: high[2] },
      ];
      let poly = input;
      for (const plane of planes) {
        poly = clipPolygon(poly, plane);
        if (poly.length === 0) return [];
      }
      const outFaces: Face[] = [];
      for (let i = 1; i < poly.length - 1; i++) {
        outFaces.push({ vertices: [poly[0], poly[i], poly[i + 1]] });
      }
      return outFaces;
    };

    this.deleteVerticesIf(shouldDeleteVert, resolvePartialFace);
    return this;
  }

  deleteVerticesInsideBox(low: Vector3, high: Vector3) {
    const shouldDeleteVertInside = (vert: GeosetVertex) => vert.position[0] >= low[0]
      && vert.position[1] >= low[1]
      && vert.position[2] >= low[2]
      && vert.position[0] <= high[0]
      && vert.position[1] <= high[1]
      && vert.position[2] <= high[2];

    const resolvePartialFaceInside = (face: Face): Face[] => {
      const planes: Plane[] = [
        { axis: 0, min: false, value: low[0] }, // keep x < low[0]
        { axis: 0, min: true, value: high[0] }, // keep x > high[0]
        { axis: 1, min: false, value: low[1] }, // keep y < low[1]
        { axis: 1, min: true, value: high[1] }, // keep y > high[1]
        { axis: 2, min: false, value: low[2] }, // keep z < low[2]
        { axis: 2, min: true, value: high[2] }, // keep z > high[2]
      ];
      const outFaces: Face[] = [];

      for (const plane of planes) {
        const poly = clipPolygon(face.vertices, plane);
        if (poly.length < 3) continue;
        for (let i = 1; i < poly.length - 1; i++) {
          outFaces.push({ vertices: [poly[0], poly[i], poly[i + 1]] });
        }
      }

      return outFaces;
    };

    this.deleteVerticesIf(shouldDeleteVertInside, resolvePartialFaceInside);
    return this;
  }

  cut1DimOutside(dimension: number, lowPercent: number, highPercent: number) {
    const diff = this.mdl.model.maximumExtent[dimension] - this.mdl.model.minimumExtent[dimension];
    const low = this.mdl.model.minimumExtent[dimension] + diff * lowPercent;
    const high = this.mdl.model.minimumExtent[dimension] + diff * highPercent;
    const vLow: Vector3 = [-Infinity, -Infinity, -Infinity];
    const vHigh: Vector3 = [Infinity, Infinity, Infinity];
    vLow[dimension] = low;
    vHigh[dimension] = high;
    return this.deleteVerticesOutsideBox(vLow, vHigh);
  }

  cutInsidePercent([[x0, x1], [y0, y1], [z0, z1]]: [[number, number], [number, number], [number, number]]) {
    const vLow: Vector3 = [
      V3.lerpScalar(this.mdl.model.minimumExtent[0], this.mdl.model.maximumExtent[0], x0),
      V3.lerpScalar(this.mdl.model.minimumExtent[1], this.mdl.model.maximumExtent[1], y0),
      V3.lerpScalar(this.mdl.model.minimumExtent[2], this.mdl.model.maximumExtent[2], z0),
    ];
    const vHigh: Vector3 = [
      V3.lerpScalar(this.mdl.model.minimumExtent[0], this.mdl.model.maximumExtent[0], x1),
      V3.lerpScalar(this.mdl.model.minimumExtent[1], this.mdl.model.maximumExtent[1], y1),
      V3.lerpScalar(this.mdl.model.minimumExtent[2], this.mdl.model.maximumExtent[2], z1),
    ];
    return this.deleteVerticesInsideBox(vLow, vHigh);
  }

  cutOutsidePercent([[x0, x1], [y0, y1], [z0, z1]]: [[number, number], [number, number], [number, number]]) {
    this.mdl.sync();
    const vLow: Vector3 = [
      V3.lerpScalar(this.mdl.model.minimumExtent[0], this.mdl.model.maximumExtent[0], x0),
      V3.lerpScalar(this.mdl.model.minimumExtent[1], this.mdl.model.maximumExtent[1], y0),
      V3.lerpScalar(this.mdl.model.minimumExtent[2], this.mdl.model.maximumExtent[2], z0),
    ];
    const vHigh: Vector3 = [
      V3.lerpScalar(this.mdl.model.minimumExtent[0], this.mdl.model.maximumExtent[0], x1),
      V3.lerpScalar(this.mdl.model.minimumExtent[1], this.mdl.model.maximumExtent[1], y1),
      V3.lerpScalar(this.mdl.model.minimumExtent[2], this.mdl.model.maximumExtent[2], z1),
    ];
    return this.deleteVerticesOutsideBox(vLow, vHigh);
  }

  cropVerticesOneDimension(dimension: number, low: number, high: number) {
    this.mdl.sync();
    const vLow: Vector3 = [-Infinity, -Infinity, -Infinity];
    const vHigh: Vector3 = [Infinity, Infinity, Infinity];
    vLow[dimension] = low;
    vHigh[dimension] = high;
    return this.deleteVerticesOutsideBox(vLow, vHigh);
  }

  deleteFacesIf(shouldDeleteFace: (face: Face) => boolean) {
    this.mdl.geosets.forEach((geoset) => {
      let verts = new Set<GeosetVertex>(geoset.vertices);
      const faces = new Set<Face>(geoset.faces);
      geoset.faces.forEach((face) => {
        if (shouldDeleteFace(face)) {
          faces.delete(face);
        }
      });
      verts = new Set([...faces].flatMap((face) => face.vertices));

      geoset.vertices = [...verts];
      geoset.faces = [...faces];
    });
    return this;
  }
}

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

function interpolateVertex(v1: GeosetVertex, v2: GeosetVertex, t: number): GeosetVertex {
  const lerp = (a: number, b: number) => a + (b - a) * t;
  const interpVec3 = (a: Vector3, b: Vector3) => [lerp(a[0], b[0]), lerp(a[1], b[1]), lerp(a[2], b[2])] as Vector3;
  const interpVec2 = (a: Vector2, b: Vector2): Vector2 => [lerp(a[0], b[0]), lerp(a[1], b[1])];

  const skinWeights: SkinWeight[] = [];
  if (v1.skinWeights || v2.skinWeights) {
    const map = new Map<Bone, number>();
    (v1.skinWeights || []).forEach((sw) => map.set(sw.bone, sw.weight * (1 - t)));
    (v2.skinWeights || []).forEach((sw) => map.set(sw.bone, (map.get(sw.bone) || 0) + sw.weight * t));
    map.forEach((weight, bone) => {
      if (weight > 0) skinWeights.push({ bone, weight });
    });
  }

  return {
    id: -1,
    position: interpVec3(v1.position, v2.position),
    normal: interpVec3(v1.normal, v2.normal),
    texPosition: interpVec2(v1.texPosition, v2.texPosition),
    matrix: v1.matrix && v2.matrix
      ? /* simple lerp or choose one */ v1.matrix
      : v1.matrix || v2.matrix,
    skinWeights: skinWeights.length ? skinWeights : undefined,
  };
}

type Plane = { axis: 0 | 1 | 2; min: boolean; value: number };

function clipPolygon(
  verts: GeosetVertex[],
  plane: Plane,
): GeosetVertex[] {
  const inside = (v: GeosetVertex) => (plane.min
    ? v.position[plane.axis] >= plane.value
    : v.position[plane.axis] <= plane.value);

  const out: GeosetVertex[] = [];
  for (let i = 0; i < verts.length; i++) {
    const curr = verts[i];
    const next = verts[(i + 1) % verts.length];
    const currIn = inside(curr);
    const nextIn = inside(next);
    if (currIn) out.push(curr);
    if (currIn !== nextIn) {
      const delta = (plane.value - curr.position[plane.axis])
        / (next.position[plane.axis] - curr.position[plane.axis]);
      out.push(interpolateVertex(curr, next, delta));
    }
  }
  return out;
}

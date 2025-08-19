import { Vector3 } from '@/lib/math/common';
import { V3 } from '@/lib/math/vector';

import { AnimationOrStatic } from '../components/animation';
import { Sequence } from '../components/sequence';
import { MDLModify } from '.';

export function scale(this: MDLModify, value: number) {
  this.mdl.geosets.forEach((geoset) => {
    geoset.vertices.forEach((vertex) => {
      vertex.position[0] *= value;
      vertex.position[1] *= value;
      vertex.position[2] *= value;
    });
  });
  this.mdl.getNodes().forEach((node) => {
    if (node.translation) {
      [...node.translation.keyFrames.values()].forEach((translation) => {
        translation[0] *= value;
        translation[1] *= value;
        translation[2] *= value;
      });
    }
    node.pivotPoint = V3.scale(node.pivotPoint, value);
  });
  this.mdl.cameras.forEach((cam) => {
    cam.position = V3.scale(cam.position, value);
    cam.target.position = V3.scale(cam.target.position, value);
  });
  this.mdl.collisionShapes.forEach((shape) => {
    shape.vertices.forEach((v) => {
      v[0] *= value;
      v[1] *= value;
      v[2] *= value;
    });
    shape.boundRadius *= value;
    shape.pivotPoint = V3.scale(shape.pivotPoint, value);
  });
  this.mdl.sequences.forEach((seq) => seq.moveSpeed *= value);

  // Scale particle emitters
  const scaleAnimOrStatic = (a?: AnimationOrStatic<number>) => {
    if (!a) return;
    if ('static' in a) a.value *= value;
    else if ('keyFrames' in a) a.keyFrames.forEach((v: number, k: number) => a.keyFrames.set(k, v * value));
  };

  this.mdl.particleEmitter2s.forEach((e) => {
    scaleAnimOrStatic(e.width);
    scaleAnimOrStatic(e.length);
    scaleAnimOrStatic(e.speed);
    scaleAnimOrStatic(e.gravity);
    e.segmentScaling = [
      e.segmentScaling[0] * value,
      e.segmentScaling[1] * value,
      e.segmentScaling[2] * value,
    ];
  });

  return this;
}

export function translate(this: MDLModify, delta: Vector3) {
  this.mdl.geosets.forEach((geoset) => {
    geoset.vertices.forEach((vertex) => {
      vertex.position = V3.sum(vertex.position, delta);
    });
  });
  this.mdl.getNodes().forEach((node) => {
    node.pivotPoint = V3.sum(node.pivotPoint, delta);
  });
  this.mdl.cameras.forEach((cam) => {
    cam.position = V3.sum(cam.position, delta);
    cam.target.position = V3.sum(cam.target.position, delta);
  });
  this.mdl.sync();
  return this;
}

export function scaleSequenceDuration(this: MDLModify, sequence: Sequence, scalingFactor: number) {
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

  const animated = this.mdl.getAnimated();
  animated.forEach((anim) => {
    if (anim.globalSeq) return;
    anim.keyFrames = updateKeyFrame(anim.keyFrames);
  });

  this.mdl.sequences.filter((seq) => seq.interval[0] > sequence.interval[1]).forEach((seq) => {
    seq.interval[0] += durationOffset;
    seq.interval[1] += durationOffset;
  });
  sequence.interval[1] += durationOffset;
  return this;
}

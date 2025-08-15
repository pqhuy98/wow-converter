import { QuaternionRotation, Vector3 } from '../../math/common';
import { calculateChildAbsoluteEulerRotation, quaternionToEuler, quatNoRotation } from '../../math/rotation';
import { V3 } from '../../math/vector';
import { Geoset, GeosetVertex } from './components/geoset';
import { Node } from './components/node';
import { Sequence } from './components/sequence';
import { MDL } from './mdl';

export interface Value {
  position: Vector3
  rotation: Vector3
  scaling: Vector3
}

export function buildChildrenLists(mdl: MDL) {
  const childrenList = new Map<Node, Node[]>(); // { parent -> children[] }
  [...mdl.bones, ...mdl.attachments].forEach((node) => {
    if (!childrenList.has(node)) {
      childrenList.set(node, []);
    }
    if (node.parent) {
      if (!childrenList.has(node.parent)) {
        childrenList.set(node.parent, []);
      }
      childrenList.get(node.parent)!.push(node);
    }
  });
  return childrenList;
}

export function iterateNodesAtTimestamp(mdl: MDL, sequence: Sequence, timestamp: number, callback: (node: Node, value: Value) => unknown) {
  const childrenList = buildChildrenLists(mdl);

  const dfs = (current: Node, currentValue: Value) => {
    callback(current, currentValue);
    for (const child of childrenList.get(current)!) {
      const childValue: Value = {
        position: [...currentValue.position],
        rotation: [...currentValue.rotation],
        scaling: [...currentValue.scaling],
      };

      const transform = interpolateTransformEuler(child, sequence, timestamp);
      let deltaPosition = V3.sum(transform.position, V3.sub(child.pivotPoint, current.pivotPoint));
      deltaPosition = V3.mul(deltaPosition, currentValue.scaling);
      deltaPosition = V3.rotate(deltaPosition, currentValue.rotation);

      childValue.position = V3.sum(childValue.position, deltaPosition);
      childValue.rotation = calculateChildAbsoluteEulerRotation(currentValue.rotation, transform.rotation);
      childValue.scaling = V3.mul(currentValue.scaling, transform.scaling);

      dfs(child, childValue);
    }
  };
  mdl.bones.forEach((b) => {
    if (!b.parent) {
      const transform = interpolateTransformEuler(b, sequence, timestamp);
      dfs(b, {
        position: V3.sum(transform.position, b.pivotPoint),
        rotation: transform.rotation,
        scaling: transform.scaling,
      });
    }
  });
}

export function iterateVerticesAtTimestamp(mdl: MDL, sequence: Sequence, timestamp: number, callback: (v: GeosetVertex, vPos: Vector3, geoset: Geoset) => unknown) {
  const nodeValues = new Map<Node, Value>();
  iterateNodesAtTimestamp(mdl, sequence, timestamp, (node, value) => {
    nodeValues.set(node, value);
  });

  const vertices = new Set<GeosetVertex>();
  const geosets = new Map<GeosetVertex, Geoset>();
  mdl.geosets.forEach((geoset) => {
    geoset.faces.forEach((f) => {
      f.vertices.forEach((v) => {
        vertices.add(v);
        geosets.set(v, geoset);
      });
    });
  });

  vertices.forEach((v) => {
    let translation: Vector3 = [0, 0, 0];
    v.skinWeights?.forEach(({ bone, weight }) => {
      const boneValue = nodeValues.get(bone)!;
      const vPosDeltaToBone = V3.mul(V3.rotate(V3.sub(v.position, bone.pivotPoint), boneValue.rotation), boneValue.scaling);
      const vPos = V3.sum(boneValue.position, vPosDeltaToBone);
      const vPosDelta = V3.sub(vPos, v.position);
      translation = V3.sum(translation, V3.scale(vPosDelta, weight / 255));
    });
    const vPos = V3.sum(v.position, translation);
    callback(v, vPos, geosets.get(v)!);
  });
}

function interpolateTransformEuler(node: Node, sequence: Sequence, timestamp: number): Value {
  const value = interpolateTransformQuat(node, sequence, timestamp);
  return {
    position: value.position,
    rotation: quaternionToEuler(value.rotation),
    scaling: value.scaling,
  };
}

export function interpolateTransformQuat(node: Node, sequence: Sequence, timestamp: number): Omit<Value, 'rotation'> & {rotation: QuaternionRotation} {
  return {
    // eslint-disable-next-line @typescript-eslint/unbound-method
    position: node.translation ? interpolateKeyFrames(sequence, node.translation.keyFrames, timestamp, node.translation.interpolation === 'Linear' ? V3.lerp : V3.noInterp, [0, 0, 0]) : [0, 0, 0],
    // eslint-disable-next-line max-len, @typescript-eslint/unbound-method
    rotation: node.rotation ? interpolateKeyFrames(sequence, node.rotation.keyFrames, timestamp, node.rotation.interpolation === 'Linear' ? V3.slerp : V3.noInterp, quatNoRotation()) : quatNoRotation(),
    // eslint-disable-next-line @typescript-eslint/unbound-method
    scaling: node.scaling ? interpolateKeyFrames(sequence, node.scaling.keyFrames, timestamp, node.scaling.interpolation === 'Linear' ? V3.lerp : V3.noInterp, [1, 1, 1]) : [1, 1, 1],
  };
}

export function interpolateKeyFrames<T>(sequence: Sequence, keyFrames: Map<number, T>, timestamp: number, intepolationFunction: (low: T, high: T, time: number) => T, defaultValue: T) {
  let low: T | undefined;
  let lowTs = -1;
  let high: T | undefined;
  let highTs = -1;
  keyFrames.forEach((value, ts) => {
    if (ts < sequence.interval[0] || sequence.interval[1] < ts) return;
    if (ts <= timestamp && (lowTs === -1 || ts > lowTs)) {
      lowTs = ts;
      low = value;
    }
    if (ts >= timestamp && (highTs === -1 || ts < highTs)) {
      highTs = ts;
      high = value;
    }
  });

  const time = highTs === lowTs ? 0 : (timestamp - lowTs) / (highTs - lowTs);
  return low && high
    ? intepolationFunction(low, high, time)
    : low ?? high ?? defaultValue;
}

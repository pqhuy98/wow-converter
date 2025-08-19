import { QuaternionRotation, Vector3 } from '@/lib/math/common';

import { f, fVector } from './formatter';
import { GlobalSequence } from './global-sequence';

export type Interpolation = 'Linear' | 'DontInterp' | 'Hermite' | 'Bezier'

export type AnimationType = 'translation' | 'rotation' | 'scaling' | 'alpha' | 'color' | 'tvertex' | 'tvertexAnim' | 'others';

export type Animation<T> = {
  globalSeq?: GlobalSequence;
  interpolation: Interpolation;
  keyFrames: Map<number, T>;
  // inOutTans?: Map<number, {inTan: Vector3, outTan: Vector3}>;
  type: AnimationType;
};

export type AnimationOrStatic<T> = {
  static: true;
  value: T;
} | Animation<T>;

export function animationToString<T extends number[] | number>(type: string, animation?: Animation<T>): string {
  if (animation == null) return '';
  if (animation.keyFrames.size === 0) return '';
  return `
  ${type} ${[...animation.keyFrames.keys()].length} {

    ${animation.interpolation},

    ${animation.globalSeq != null ? `GlobalSeqId ${animation.globalSeq.id},` : ''}

    ${[...sortMapByKeyAsc(animation.keyFrames).entries()].map(([timestamp, value]) => `
      ${timestamp}: ${Array.isArray(value) ? `{ ${fVector(value)} }` : f(value)},
      ${/* ${animation.inOutTans?.get(timestamp) != null ? `
        InTan { ${fVector(animation.inOutTans.get(timestamp)!.inTan)} },
        OutTan { ${fVector(animation.inOutTans.get(timestamp)!.outTan)} },` : ''} */''}
      `).join('\n')}
  }`;
}

export function animatedValueToString<T extends number[] | number>(type: string, animatedValue?: AnimationOrStatic<T>): string {
  if (animatedValue == null) return '';
  if ('static' in animatedValue) {
    return `static ${type} ${Array.isArray(animatedValue.value) ? `{ ${fVector(animatedValue.value)} }` : f(animatedValue.value)},`;
  }
  return animationToString(type, animatedValue);
}

export function staticTranslation(): Animation<Vector3> {
  return {
    interpolation: 'DontInterp',
    keyFrames: new Map([[0, [0, 0, 0]]]),
    type: 'translation',
  };
}

export function staticRotation(): Animation<QuaternionRotation> {
  return {
    interpolation: 'DontInterp',
    keyFrames: new Map([[0, [0, 0, 0, 1]]]),
    type: 'rotation',
  };
}

export function staticScaling(): Animation<Vector3> {
  return {
    interpolation: 'DontInterp',
    keyFrames: new Map([[0, [1, 1, 1]]]),
    type: 'scaling',
  };
}

function sortMapByKeyAsc<K, V>(map: Map<K, V>): Map<K, V> {
  return new Map(
    Array.from(map.entries()).sort(([keyA], [keyB]) => {
      if (keyA > keyB) return 1;
      if (keyA < keyB) return -1;
      return 0;
    }),
  );
}

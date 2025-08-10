import { f, fVector } from './formatter';
import { GlobalSequence } from './global-sequence';

export type Interpolation = 'Linear' | 'DontInterp'

export function wowToWc3Interpolation(wowInterpolation: number): Interpolation {
  return wowInterpolation === 1 ? 'Linear' : 'DontInterp';
}

export interface Animation<T> {
  interpolation: Interpolation;
  globalSeq?: GlobalSequence;
  keyFrames: Map<number, T>;
}

export function animationToString<T extends number[] | number>(type: string, animation?: Animation<T>): string {
  if (animation == null) return '';
  if (animation.keyFrames.size === 0) return '';
  return `
  ${type} ${[...animation.keyFrames.keys()].length} {

    ${animation.interpolation},

    ${animation.globalSeq != null ? `GlobalSeqId ${animation.globalSeq.id},` : ''}

    ${[...sortMapByKeyAsc(animation.keyFrames).entries()].map(([timestamp, value]) => `
    ${timestamp}: ${Array.isArray(value) ? `{ ${fVector(value)} }` : f(value)},`).join('\n')}
  }`;
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

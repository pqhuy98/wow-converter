import { Vector3 } from '@/lib/math/common';

import {
  animatedValueToString, Animation, AnimationOrStatic, animationToString,
} from '../animation';
import { Node, nodeAnimations, nodeHeaders } from './node';

export enum ParticleEmitterPopcornFlag {
  Unshaded = 'Unshaded',
  SortPrimsFarZ = 'SortPrimsFarZ',
  Unfogged = 'Unfogged',
}

export interface ParticleEmitterPopcorn extends Node {
  type: 'ParticleEmitterPopcorn'
  flagsPop: ParticleEmitterPopcornFlag[];

  lifeSpan?: AnimationOrStatic<number>;
  emissionRate?: AnimationOrStatic<number>;
  speed?: AnimationOrStatic<number>;
  color?: AnimationOrStatic<Vector3>;
  alpha?: AnimationOrStatic<number>;
  visibility?: Animation<number>;
  replaceableId?: number;
  path?: string;
  animationVisiblityGuide?: string;
}

export function particleEmitterPopcornsToString(emitters: ParticleEmitterPopcorn[]): string {
  return emitters.map((e) => `
    ParticleEmitterPopcorn "${e.name}" {
      ${nodeHeaders(e)}
      ${e.flagsPop.map((f) => `${f},`).join('\n')}

      ${animatedValueToString('LifeSpan', e.lifeSpan)}
      ${animatedValueToString('EmissionRate', e.emissionRate)}
      ${animatedValueToString('Speed', e.speed)}
      ${animatedValueToString('Color', e.color)}
      ${animatedValueToString('Alpha', e.alpha)}
      ${animationToString('Visibility', e.visibility)}
      ${e.replaceableId ? `ReplaceableId ${e.replaceableId},` : ''}
      ${e.path ? `Path "${e.path}",` : ''}
      ${e.animationVisiblityGuide ? `AnimVisibilityGuide "${e.animationVisiblityGuide}",` : ''}

      ${nodeAnimations(e)}
    }`).join('\n');
}

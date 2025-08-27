import { Vector3 } from '@/lib/math/common';

import {
  animatedValueToString, Animation, AnimationOrStatic, animationToString,
} from '../animation';
import { f } from '../formatter';
import { Node, nodeAnimations, nodeHeaders } from './node';

export interface RibbonEmitter extends Node {
  type: 'RibbonEmitter'

  // Animatable tracks
  heightAbove?: AnimationOrStatic<number>;
  heightBelow?: AnimationOrStatic<number>;
  alpha?: AnimationOrStatic<number>;
  color?: AnimationOrStatic<Vector3>;
  textureSlot?: AnimationOrStatic<number>;
  visibility?: Animation<number>;

  // Static properties
  emissionRate: number;
  lifeSpan: number;
  rows: number;
  columns: number;
  materialId: number;
  gravity: number;
}

export function ribbonEmittersToString(ribbons: RibbonEmitter[]): string {
  if (ribbons.length === 0) return '';
  return ribbons.map((e) => `
    RibbonEmitter "${e.name}" {
      ${nodeHeaders(e)}

      ${animatedValueToString('HeightAbove', e.heightAbove)}
      ${animatedValueToString('HeightBelow', e.heightBelow)}
      ${animatedValueToString('Alpha', e.alpha)}
      ${animatedValueToString('Color', e.color)}
      ${animatedValueToString('TextureSlot', e.textureSlot)}
      ${animationToString('Visibility', e.visibility)}

      EmissionRate ${f(e.emissionRate)},
      LifeSpan ${f(e.lifeSpan)},
      ${e.gravity !== 0 ? `Gravity ${f(e.gravity)},` : ''}
      Rows ${e.rows},
      Columns ${e.columns},
      MaterialID ${e.materialId},

      ${nodeAnimations(e)}
    }`).join('\n');
}

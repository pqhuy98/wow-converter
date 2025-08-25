import { Vector3 } from '@/lib/math/common';

import { Animation, AnimationOrStatic, animationToString } from '../animation';
import { f, fVector } from '../formatter';
import { Node, nodeAnimations, nodeHeaders } from './node';

export enum LightType {
  Omnidirectional = 'Omnidirectional',
  Directional = 'Directional',
  Ambient = 'Ambient',
}

export interface Light extends Node {
  type: 'Light'
  lightType: LightType;
  attenuationStart: AnimationOrStatic<number>;
  attenuationEnd: AnimationOrStatic<number>;
  intensity: AnimationOrStatic<number>;
  color: AnimationOrStatic<Vector3>;
  ambientIntensity: AnimationOrStatic<number>;
  ambientColor: AnimationOrStatic<Vector3>;
  visibility?: Animation<number>;
}

export function lightsToString(lights: Light[]): string {
  return lights.map((l) => `
    Light "${l.name}" {
      ${nodeHeaders(l)}
      ${l.lightType},

      ${animationOrStaticNumber('AttenuationStart', l.attenuationStart)}
      ${animationOrStaticNumber('AttenuationEnd', l.attenuationEnd)}
      ${animationOrStaticNumber('Intensity', l.intensity)}
      ${animationOrStaticColor('Color', l.color)}
      ${animationOrStaticNumber('AmbIntensity', l.ambientIntensity)}
      ${animationOrStaticColor('AmbColor', l.ambientColor)}
      ${animationToString('Visibility', l.visibility)}

      ${nodeAnimations(l)}
    }`).join('\n');
}

function animationOrStaticNumber(type: string, value: AnimationOrStatic<number>): string {
  return 'static' in value ? `static ${type} ${f(value.value)},` : animationToString(type, value);
}

function animationOrStaticColor(type: string, value: AnimationOrStatic<Vector3>): string {
  return 'static' in value ? `static ${type} { ${fVector(value.value)} },` : animationToString(type, value);
}

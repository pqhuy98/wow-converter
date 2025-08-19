import {
  animatedValueToString, Animation, AnimationOrStatic, animationToString,
} from '../animation';
import { Node, nodeAnimations, nodeHeaders } from './node';

export enum ParticleEmitterFlag {
  EmitterUsesMDL = 'EmitterUsesMDL',
  EmitterUsesTGA = 'EmitterUsesTGA',
}

export interface ParticleEmitter extends Node {
  type: 'ParticleEmitter'
  emitterFlags: ParticleEmitterFlag[];
  emissionRate?: AnimationOrStatic<number>;
  gravity?: AnimationOrStatic<number>;
  longitude?: AnimationOrStatic<number>;
  latitude?: AnimationOrStatic<number>;
  visibility?: Animation<number>;
  lifeSpan?: AnimationOrStatic<number>;
  speed?: AnimationOrStatic<number>; // InitVelocity
  path?: string;
}

export function particleEmittersToString(particleEmitters: ParticleEmitter[]): string {
  return particleEmitters.map((emitter) => `
    ParticleEmitter "${emitter.name}" {
      ${nodeHeaders(emitter)}
      ${emitter.emitterFlags.map((flag) => `${flag},`).join('\n')}

      ${animatedValueToString('EmissionRate', emitter.emissionRate)}
      ${animatedValueToString('Gravity', emitter.gravity)}
      ${animatedValueToString('Longitude', emitter.longitude)}
      ${animatedValueToString('Latitude', emitter.latitude)}
      ${animationToString('Visibility', emitter.visibility)}

      Particle {
        ${animatedValueToString('LifeSpan', emitter.lifeSpan)}
        ${animatedValueToString('InitVelocity', emitter.speed)}
        ${needsPath(emitter) && emitter.path ? `Path "${emitter.path}",` : ''}
      }

      ${nodeAnimations(emitter)}
    }`).join('\n');
}

function needsPath(emitter: ParticleEmitter): boolean {
  return emitter.emitterFlags.includes(ParticleEmitterFlag.EmitterUsesMDL) || emitter.emitterFlags.includes(ParticleEmitterFlag.EmitterUsesTGA);
}

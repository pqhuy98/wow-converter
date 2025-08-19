import { Vector3 } from '@/lib/math/common';

import {
  animatedValueToString, Animation, AnimationOrStatic, animationToString,
} from '../animation';
import { f, fVector } from '../formatter';
import { Texture } from '../texture';
import { Node, nodeAnimations, nodeHeaders } from './node';

export enum ParticleEmitter2Flag {
  Unshaded = 'Unshaded',
  SortPrimsFarZ = 'SortPrimsFarZ',
  LineEmitter = 'LineEmitter',
  Unfogged = 'Unfogged',
  ModelSpace = 'ModelSpace',
  XYQuad = 'XYQuad',
}

export enum FilterMode {
  Blend = 'Blend',
  Additive = 'Additive',
  Modulate = 'Modulate',
  Modulate2x = 'Modulate2x',
  AlphaKey = 'AlphaKey',
}

export enum HeadOrTail {
  Head = 'Head',
  Tail = 'Tail',
  Both = 'Both',
}

export interface ParticleEmitter2 extends Node {
  type: 'ParticleEmitter2'
  flags2: ParticleEmitter2Flag[];

  // Gating. If visibility ≤ 0, the emitter does not update/spawn for that frame.
  // Default visibility is 1 if track absent.
  // wow: ???
  visibility?: Animation<number>;

  // Width and Length determine the rectangular field in which particles are spawned.
  // The spawn point is uniformly random in the rectangle [−Width/2..Width/2] × [−Length/2..Length/2]
  // around the node pivot.
  // Bug: Wc3 when render MDL file, it will swap these 2 values compared to same model in MDX
  // WIDTH - Along the X Axis (frontward/backward)
  // LENGTH - Along the Y Axis (left/right)
  // wow: emissionAreaWidth/emissionAreaLength - as is
  width: AnimationOrStatic<number>;
  length: AnimationOrStatic<number>;

  // The amount of particles generated per second. The interval between emissions is capped at ~0.05s (?). Going faster than this threshold will emit those excess particles in bulk.
  // Emission rates below 1 will only create particles if the sequence is longer than 1 second.
  // wow: emissionRate - as is
  emissionRate: AnimationOrStatic<number>;

  // wc3: By default, causes the particle to move towards its Z. This direction can be changed
  // through emitter or parent rotation. Animated changes in speed will only affect newly
  // emitted particles; thus gravity is the only way to accelerate.
  // wow: emissionSpeed - copy as is
  speed: AnimationOrStatic<number>;

  // The percentage of randomness with which the particle can deviate from its base speed.
  // 1 means speed can go down to zero, or double; and 2 means it can go -1x negative,
  // or triple.
  // wow: speedVariation - as is
  variation: AnimationOrStatic<number>;

  // Angles to which the particles will fly, deviating from the emitter's facing direction.
  // A value of 90 means the particles will go towards a semi-spherical area,
  // while 180 removes angular restrictions.
  // wow: verticalRange (rad) - convert to wc3 degrees
  latitude: AnimationOrStatic<number>;

  /*
  * === Head/Tail ===
  */

  // Head = quad at the particle; Tail = velocity‑aligned camera‑facing ribbon; Both emits both.
  // wow: ???
  headOrTail: HeadOrTail;
  // Length of a tail segment in world units; combined with velocity to form the tail.
  // wow: ???
  tailLength: number;

  /*
  * === Life Span ===
  */

  // Duration each particle lives before expiring (seconds).
  // wow: ???
  lifeSpan: number;

  // Segment = life stage; particles traverse 3 segments (1→2→3), with Time splitting 1→2 and 2→3.
  // Fraction of life (0..1) where segment 1→2 ends and 2→3 begins.
  // wow: ???
  timeMiddle: number;

  // RGB colors for the three life segments (interpolated 1→2→3).
  // wow: ???
  segmentColors: [Vector3, Vector3, Vector3];

  // Alpha (0–255) for the three life segments (interpolated 1→2→3).
  // wow: ???
  segmentAlphas: [number, number, number];

  // Scale factors for the three life segments; controls per‑particle size.
  // wow: ???
  segmentScaling: [number, number, number];

  /*
  * === Texture ===
  */

  // Texture used when not using replaceable IDs; ignored if replaceableId selects a built‑in.
  // wow: ???
  texture: Texture;

  // Selects team color/glow (1/2) or other replaceable texture (>2); 0 uses model textureId.
  // wow: ???
  replaceableId?: number;

  // Number of sprite rows in the texture atlas.
  // wow: ???
  rows: number;

  // Number of sprite columns in the texture atlas.
  // wow: ???
  columns: number;

  // Sprite range and repeat for head during segment 1: [start, end, repeat].
  // wow: ???
  headIntervals: [number, number, number];

  // Sprite range and repeat for head during segment 2 (decay = second half of life): [start, end, repeat].
  // wow: ???
  decayIntervals: [number, number, number];

  // Sprite range and repeat for tail during segment 1: [start, end, repeat].
  // wow: ???
  tailIntervals: [number, number, number];

  // Sprite range and repeat for tail during segment 2 (decay): [start, end, repeat].
  // wow: ???
  tailDecayIntervals: [number, number, number];

  /*
  * === Other ===
  */

  // Spawns all particles for a key instantly on key change instead of distributing over time.
  // wow: ???
  squirt: boolean;

  // Render sorting layer for particles within the transparent queue.
  // wow: ???
  priorityPlane?: number;

  // How particles blend with the framebuffer (Blend/Additive/Modulate/Modulate2x/AlphaKey).
  // wow: ???
  filterMode: FilterMode;

  // An accelerating force that points downwards in the absence of Model Space rotations. Negative values will make the particles go up.
  // wow: gravity - as is (maybe), wowdev has "Compressed Particle Gravity" that turn float to Vector3. So maybe we need to get its Z value?
  gravity: AnimationOrStatic<number>;
}

export function particleEmitter2sToString(emitters: ParticleEmitter2[]): string {
  return emitters.map((e) => `
    ParticleEmitter2 "${e.name}" {
      ${nodeHeaders(e)}
      ${e.flags2.map((f) => `${f},`).join('\n')}

      ${animatedValueToString('Speed', e.speed)}
      ${animatedValueToString('Variation', e.variation)}
      ${animatedValueToString('Latitude', e.latitude)}
      ${animatedValueToString('Gravity', e.gravity)}
      ${animationToString('Visibility', e.visibility)}
      ${e.squirt ? 'Squirt,' : ''}
      LifeSpan ${f(e.lifeSpan)},
      ${animatedValueToString('EmissionRate', e.emissionRate)}
      ${animatedValueToString('Width', e.width)}
      ${animatedValueToString('Length', e.length)}

      ${e.filterMode},
      Rows ${e.rows},
      Columns ${e.columns},
      ${e.headOrTail},
      TailLength ${f(e.tailLength)},
      Time ${f(e.timeMiddle)},

      SegmentColor {
        Color { ${fVector(e.segmentColors[0])} },
        Color { ${fVector(e.segmentColors[1])} },
        Color { ${fVector(e.segmentColors[2])} },
      },
      Alpha { ${fVector(e.segmentAlphas)} },
      ParticleScaling { ${fVector(e.segmentScaling)} },
      LifeSpanUVAnim { ${fVector(e.headIntervals)} },
      DecayUVAnim { ${fVector(e.decayIntervals)} },
      TailUVAnim { ${fVector(e.tailIntervals)} },
      TailDecayUVAnim { ${fVector(e.tailDecayIntervals)} },
      TextureID ${e.texture.id},
      ${e.replaceableId ? `ReplaceableId ${e.replaceableId},` : ''}
      ${e.priorityPlane ? `PriorityPlane ${e.priorityPlane},` : ''}

      ${nodeAnimations(e)}
    }`).join('\n');
}

import {
  animatedValueToString, Animation, AnimationOrStatic, animationToString,
} from './animation';
import { Texture } from './texture';
import { TextureAnim } from './texture-anim';

export type BlendMode = 'None' | 'Transparent' | 'Blend' | 'Additive' | 'AddAlpha' | 'Modulate' | 'Modulate2x'

export interface Layer {
  filterMode: BlendMode;
  texture: Texture;
  textureIDAnim?: Animation<Texture>;
  tvertexAnim?: TextureAnim;
  alpha: AnimationOrStatic<number>;
  coordId?: number;

  // flags
  unshaded: boolean;
  sphereEnvMap: boolean;
  twoSided: boolean;
  unfogged: boolean;
  unlit: boolean;
  noDepthTest: boolean;
  noDepthSet: boolean;
}

export interface Material {
  id: number
  priorityPlane?: number;
  constantColor: boolean;
  twoSided: boolean;
  layers: Layer[];
}

export function materialsToString(version: number, materials: Material[]) {
  if (materials.length === 0) return '';
  return `Materials ${materials.length} {
    ${materials.map((material) => `
      Material {
        ${material.priorityPlane ? `PriorityPlane ${material.priorityPlane},` : ''}
        ${material.constantColor ? 'ConstantColor,' : ''}
        ${material.layers.map((layer) => `
        Layer {
          FilterMode ${layer.filterMode},
          ${layer.textureIDAnim ? animationToString('TextureID', { ...layer.textureIDAnim, keyFrames: new Map([...layer.textureIDAnim.keyFrames].map(([t, tex]) => [t, tex.id])) }) : `static TextureID ${layer.texture.id},`}
          ${layer.unshaded ? 'Unshaded,' : ''}
          ${layer.sphereEnvMap ? 'SphereEnvMap,' : ''}
          ${layer.twoSided ? 'TwoSided,' : ''}
          ${layer.unfogged ? 'Unfogged,' : ''}
          ${layer.noDepthTest ? 'NoDepthTest,' : ''}
          ${layer.noDepthSet ? 'NoDepthSet,' : ''}
          ${version > 800 && layer.unlit ? 'Unlit,' : ''}
          ${layer.coordId && layer.coordId !== 0 ? `CoordId ${layer.coordId},` : ''}
          ${layer.tvertexAnim != null ? `TVertexAnimId ${layer.tvertexAnim.id},` : ''}
          ${animatedValueToString('Alpha', layer.alpha)}
        }`).join('\n')}
      }`).join('\n')}
  }`;
}

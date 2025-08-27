import { AnimationOrStatic } from './animation';
import { Texture } from './texture';
import { TextureAnim } from './texture-anim';

export type BlendMode = 'None' | 'Transparent' | 'Blend' | 'Additive' | 'AddAlpha' | 'Modulate' | 'Modulate2x'

export interface Material {
  id: number
  constantColor: boolean;
  twoSided: boolean;
  layers: {
    filterMode: BlendMode;
    texture: Texture;
    tvertexAnim?: TextureAnim;
    alpha: AnimationOrStatic<number>;

    // flags
    unshaded: boolean;
    sphereEnvMap: boolean;
    twoSided: boolean;
    unfogged: boolean;
    unlit: boolean;
    noDepthTest: boolean;
    noDepthSet: boolean;
  }[];
}

export function materialsToString(version: number, materials: Material[]) {
  if (materials.length === 0) return '';
  return `Materials ${materials.length} {
    ${materials.map((material) => `
      Material {
        ${material.constantColor ? 'ConstantColor,' : ''}
        ${version > 800 && material.twoSided ? 'TwoSided,' : ''}
        ${material.layers.map((layer) => `
        Layer {
          FilterMode ${layer.filterMode},
          static TextureID ${layer.texture.id},
          ${layer.unshaded ? 'Unshaded,' : ''}
          ${layer.sphereEnvMap ? 'SphereEnvMap,' : ''}
          ${layer.twoSided ? 'TwoSided,' : ''}
          ${layer.unfogged ? 'Unfogged,' : ''}
          ${layer.noDepthTest ? 'NoDepthTest,' : ''}
          ${layer.noDepthSet ? 'NoDepthSet,' : ''}
          ${layer.unlit ? 'Unlit,' : ''}
          ${layer.tvertexAnim != null ? `TVertexAnimId ${layer.tvertexAnim.id},` : ''}
        }`).join('\n')}
      }`).join('\n')}
  }`;
}

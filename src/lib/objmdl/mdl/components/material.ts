import { Texture } from './texture';
import { TextureAnim } from './texture-anim';

export type BlendMode = 'None' | 'Transparent' | 'Blend' | 'Additive' | 'AddAlpha' | 'Modulate' | 'Modulate2x'
export function m2BlendModeToWc3FilterMode(m2BlendMode: number): BlendMode {
  switch (m2BlendMode) {
    // https://wowdev.wiki/M2/Rendering#M2BLEND
    case 0: // GxBlend_Opaque
      return 'None';
    case 1: // GxBlend_AlphaKey
      return 'Transparent';
    case 2: // GxBlend_Alpha
      return 'Blend';
    case 3: // GxBlend_NoAlphaAdd
      return 'Blend';
    case 4: // GxBlend_Add
      return 'Additive';
    case 5: // GxBlend_Mod
      return 'Modulate';
    case 6: // GxBlend_Mod2x
      return 'Modulate2x';
    case 7: // GxBlend_BlendAdd
      return 'Additive';
    default:
      throw new Error('Unknown blend mode');
  }
}

export interface Material {
  id: number
  constantColor: boolean;
  layers: {
    filterMode: BlendMode;
    texture: Texture;
    twoSided: boolean;
    unfogged: boolean;
    unlit: boolean;
    noDepthTest: boolean;
    noDepthSet: boolean;
    tvertexAnim?: TextureAnim;
  }[];
}

export function materialsToString(materials: Material[]) {
  if (materials.length === 0) return '';
  return `Materials ${materials.length} {
    ${materials.map((material) => `
      Material {
        ${material.constantColor ? 'ConstantColor,' : ''}
        ${material.layers.map((layer) => `
        Layer {
          FilterMode ${layer.filterMode},
          static TextureID ${layer.texture.id},
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

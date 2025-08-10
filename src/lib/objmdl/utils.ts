import { Interpolation } from './mdl/components/animation';
import { BlendMode } from './mdl/components/material';

export function wowToWc3Interpolation(wowInterpolation: number): Interpolation {
  switch (wowInterpolation) {
    case 0:
      return 'DontInterp';
    case 1:
      return 'Linear';
    default:
      throw new Error(`Unknown interpolation ${wowInterpolation}`);
  }
}

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

const noneFilterPatterns = [
  'textures\\walls',
  'textures\\trim',
  'textures\\floor',
];
const transparentFilterPatterns = [
  '\\bush',
  '_bush',
  '\\branch',
  '_branch',
  '\\tree',
  '_tree',
  'treetall',
  '_vfx_fire_',
  'vines',
  'treebranch',
  'floornets',
  'spells\\',
  'environment\\doodad\\',
  '\\gate10.',
  'interface\\glues',
  'fence',
  'haypiles',
  // 'passivedoodads', -- too wide
  'plant',
  'alpha',
  'ash04',
  '\\glow',
  'elwynnmiscrope03',
  'textures\\decoration',
  '_glow',
  'jlo_worc_chainsr',
  '\\hay\\',
  '\\sc_brazier',
  'hangnets',
  'flare05',
  'lightbeam',
  'jlo_worc_grate',
  'sc_chain',
];
const additiveFilterPatterns = [
  'genericglow',
  'swordinice',
  '_fog_',
  'icecrown_rays',
  'blueglow',
  'treeweb01',
  '_web',
];

export function guessFilterMode(filePath: string): BlendMode {
  if (noneFilterPatterns.some((pattern) => filePath.includes(pattern))) {
    return 'None';
  }
  if (additiveFilterPatterns.some((pattern) => filePath.includes(pattern))) {
    return 'Additive';
  }
  if (transparentFilterPatterns.some((pattern) => filePath.includes(pattern))) {
    return 'Transparent';
  }
  return 'None';
}

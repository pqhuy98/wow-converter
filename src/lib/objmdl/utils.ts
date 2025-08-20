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

function m2BlendModeToWc3FilterMode(m2BlendMode: number): BlendMode {
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

const debug = false;

// Map M2 shader combiner (from shaderId) to WC3 per-layer filter
export function getLayerFilterMode(blendingMode: number, shaderId: number, layerIndex: number): BlendMode | undefined {
  if (layerIndex === 0) return m2BlendModeToWc3FilterMode(blendingMode);
  const opaquePath = (shaderId & 0x70) === 0;
  const op = (shaderId & 7);
  // Aligns with WebWowViewer combiner groupings; simplified to WC3 filter modes
  if (opaquePath) {
    debug && console.log('opaquePath', op);
    if (op === 0) return undefined; // Opaque_Opaque
    if (op === 3) return undefined; // Opaque_AddAlpha / Opaque_AddAlpha_Alpha
    return 'Additive';
    // https://www.wowhead.com/mop-classic/npc=71953/xuen op=[2,5,6]
    // https://www.wowhead.com/mop-classic/npc=56762/yulon op=[6]
    // https://www.wowhead.com/wotlk/npc=36612/lord-marrowgar op=[7]
    // switch (op) {
    //   case 0: // Opaque_Opaque (best-effort)
    //   case 1: // Opaque_Mod
    //   case 2: // Opaque_Mod
    //   case 3: // Opaque_AddAlpha / Opaque_AddAlpha_Alpha
    //   case 4: // Opaque_Mod2x
    //   case 5: // Opaque_Mod
    //   case 6: // Opaque_Mod2xNA
    //   case 7: // ??
  } // Mod path
  return undefined;
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

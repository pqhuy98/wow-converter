import chalk from 'chalk';

import { Interpolation } from '@/lib/formats/mdl/components/animation';
import { BlendMode } from '@/lib/formats/mdl/components/material';
import { Texture } from '@/lib/formats/mdl/components/texture';

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

// Map M2 shader combiner (from shaderId) to WC3 per-layer filter. This is all hacks and reverse-engineering.
export function getLayerFilterMode(blendingMode: number, shaderId: number, layerIndex: number, texture: Texture): BlendMode | undefined {
  if (layerIndex === 0) return m2BlendModeToWc3FilterMode(blendingMode);
  const opaquePath = (shaderId & 0x70) === 0;
  const op = (shaderId & 7);

  const debug = false;
  debug && console.log('opaquePath', {
    opaquePath, op, blendingMode, shaderId, layerIndex, img: texture.image,
  });

  // Aligns with WebWowViewer combiner groupings; simplified to WC3 filter modes
  if (opaquePath) {
    if (texture.image.includes('reflect') || texture.image.includes('shine')) {
      return undefined;
    }
    if (op === 0) return undefined; // Opaque_Opaque
    if (op === 3) {
      // Opaque_AddAlpha / Opaque_AddAlpha_Alpha
      const texturePath = texture.image.replace('.png', '').replace('.blp', '');
      // A hack to skip textures like "armorreflect" that has same op as glow textures but should be skipped
      // We need to be aggressive in skipping secondary textures, only add when absolutely certain
      if (texturePath.includes('glow')) {
        return 'Additive';
      }
      return undefined;
    }
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

// Map WMO blend mode (EGxBlendEnum) to WC3 filter mode
// Source of truth: WebWowViewerCpp EGxBlendEnum and blending factors, mdx-m3-viewer filter modes
// 0 Opaque, 1 AlphaKey, 2 Alpha, 3 Add, 4 Mod, 5 Mod2x, 6 ModAdd, 7 InvSrcAlphaAdd, 8 InvSrcAlphaOpaque,
// 9 SrcAlphaOpaque, 10 NoAlphaAdd, 11 ConstantAlpha, 12 Screen, 13 BlendAdd
export function wmoBlendModeToWc3FilterMode(wmoBlendMode: number): BlendMode {
  switch (wmoBlendMode) {
    case 0: // Opaque
      return 'None';
    case 1: // AlphaKey (1-bit alpha)
      return 'Transparent';
    case 2: // Alpha (srcAlpha, oneMinusSrcAlpha)
      return 'Blend';
    case 3: // Add (srcAlpha, one)
      return 'Additive';
    case 4: // Mod (dstColor, 0)
      return 'Modulate';
    case 5: // Mod2x (dstColor, srcColor)
      return 'Modulate2x';
    case 6: // ModAdd (dstColor, one)
      return 'Additive';
    case 7: // InvSrcAlphaAdd (1-srcAlpha, one)
      return 'Additive';
    case 8: // InvSrcAlphaOpaque (1-srcAlpha, 0)
      return 'Blend';
    case 9: // SrcAlphaOpaque (srcAlpha, 0)
      return 'Blend';
    case 10: // NoAlphaAdd (one, one)
      return 'Additive';
    case 11: // ConstantAlpha
      return 'Blend';
    case 12: // Screen (1-dstColor, one)
      return 'Additive';
    case 13: // BlendAdd
      return 'Additive';
    default:
      return 'Blend';
  }
}

export function guessFilterMode(filePath: string): BlendMode {
  console.log(chalk.red('Warning: guessFilterMode', filePath));

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

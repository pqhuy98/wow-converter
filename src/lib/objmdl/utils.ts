import { BlendMode } from './mdl/mdl';

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

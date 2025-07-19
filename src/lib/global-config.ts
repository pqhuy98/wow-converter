import chalk from 'chalk';

import { Config } from './converter/common';
import { radians } from './math/rotation';
import { BlendMode } from './objmdl/mdl/mdl';
import { wowExportClient } from './wowexport-client/wowexport-client';

// eslint-disable-next-line import/no-mutable-exports
export const wowExportPath = {
  value: '',
};
export const assetPrefix = 'wow';

wowExportClient.getConfig().then((config) => {
  wowExportPath.value = config.exportDirectory;
  console.log(chalk.green('Connected successfully to wow.export server'));
  console.log('wow.export assets path:', chalk.gray(wowExportPath.value));
}).catch((error) => {
  console.error(chalk.red('Cannot connect to wow.export server, did you run it?'));
});

export const defaultConfig: Config = {
  assetPrefix,
  release: true,

  terrainHeightClampPercent: {
    upper: 1,
    lower: 0,
  },
  waterZThreshold: -2000,
  verticalHorizontalRatio: 1, // reducing this makes the map bigger, but doodads' position Z will become more wrong.
  pitchRollThresholdRadians: radians(5),
  rawModelScaleUp: 30,
  infiniteExtentBoundRadiusThreshold: 2000 / 28,
  overrideModels: true,
  placeCreatures: true,
  exportCreatureModels: true,
};

// Fine-tuned parameters
export const defaultFilterMode = 'None';
// export const defaultFilterMode = 'Transparent';

// Map generation configs
// export const dataHeightMin = 0;
// export const dataHeightMax = 16383; // 2^14 - 1
export const dataHeightMin = 128;
export const dataHeightMax = 16383; // Blizzard magic number

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
  return defaultFilterMode;
}

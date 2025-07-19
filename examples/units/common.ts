import { rmSync } from 'fs';
import fsExtra from 'fs-extra';

import { CharacterExporter } from '@/lib/converter/character';
import { Config } from '@/lib/converter/common';

import { defaultConfig } from '../../src/lib/global-config';

export const ceConfig: Config = {
  ...defaultConfig,
  assetPrefix: 'wow',
  rawModelScaleUp: defaultConfig.rawModelScaleUp * 2,
};
export const ceOutputPath = 'dist/exported-assets';
fsExtra.ensureDirSync(ceOutputPath);

export const ce = new CharacterExporter(ceOutputPath, ceConfig);

export function clearOutput() {
  rmSync(ce.outputPath, { recursive: true });
  fsExtra.ensureDirSync(ce.outputPath);
}

const clear = false;
if (clear) {
  clearOutput();
}

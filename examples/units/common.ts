import { rmSync } from 'fs';
import fsExtra from 'fs-extra';

import { CharacterExporter } from '@/lib/converter/character-exporter';
import { getDefaultConfig } from '@/lib/global-config';

export const ceOutputPath = 'exported-assets';
export const ceConfig = await getDefaultConfig();
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

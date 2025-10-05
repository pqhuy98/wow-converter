import { rmSync } from 'fs';
import { ensureDir } from 'fs-extra';

import { CharacterExporter } from '@/lib/converter/character';
import { getDefaultConfig } from '@/lib/global-config';

export const ceOutputPath = 'exported-assets';
export const ceConfig = await getDefaultConfig();
await ensureDir(ceOutputPath);

export const ce = new CharacterExporter(ceConfig);

const clear = false;
if (clear) {
  rmSync(ceOutputPath, { recursive: true });
  await ensureDir(ceOutputPath);
}

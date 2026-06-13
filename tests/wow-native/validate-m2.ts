/**
 * Phase 3 validation: direct M2 -> MDL conversion from CASC.
 *
 * Usage: bun tests/wow-native/validate-m2.ts [--wow-dir <path>]
 */
import path from 'path';

import { convertM2ToMdl } from '@/lib/converter/wow-model/direct/m2';
import { CASCLocal } from '../../src/lib/wow/archive/casc/casc-source-local';
import * as listfile from '../../src/lib/wow/archive/casc/listfile';
import { load as loadTactKeys } from '../../src/lib/wow/archive/casc/tact-keys';
import { Config } from '@/lib/global-config';
import { wowConfig } from '../../src/lib/wow/server/config';
import { runtimeState } from '../../src/lib/wow/server/runtime';
import { write } from '../../src/lib/wow/log';

const WOW_DIR = getArg('--wow-dir') ?? process.env.CASC_LOCAL_WOW ?? 'D:\\Programs\\Blizzard Games\\World of Warcraft';
const PRODUCT = getArg('--product') ?? 'wow';

const TEST_MODELS = [
  'world/azeroth/elwynn/passivedoodads/farmwindmill/farmwindmill.m2',
  'world/expansion02/doodads/generic/scourge/sc_plaguevat_01.m2',
  'world/generic/human/passive doodads/barrels/barrel01.m2',
];

function getArg(name: string): string | undefined {
  const idx = process.argv.indexOf(name);
  return idx > -1 ? process.argv[idx + 1] : undefined;
}

let failures = 0;

function check(name: string, condition: boolean, detail?: string): void {
  if (condition) {
    console.log(`  PASS ${name}${detail ? ` (${detail})` : ''}`);
  } else {
    failures++;
    console.error(`  FAIL ${name}${detail ? ` (${detail})` : ''}`);
  }
}

async function main() {
  const t0 = Date.now();
  const preloadPromise = listfile.preload();
  await loadTactKeys();
  const casc = new CASCLocal(WOW_DIR);
  await casc.init();
  const buildIndex = casc.builds.findIndex((b) => b.Product === PRODUCT);
  if (buildIndex === -1) throw new Error(`Product ${PRODUCT} not found`);
  await preloadPromise;
  await casc.load(buildIndex);
  runtimeState.casc = casc;

  const exportDir = path.resolve(__dirname, 'out', 'm2-direct');
  const config = {
    exportAssetDir: exportDir,
    assetPrefix: 'wow',
    isBulkExport: true,
    mdx: true,
  } satisfies Pick<Config, 'exportAssetDir' | 'assetPrefix' | 'isBulkExport' | 'mdx'> as Config;

  wowConfig.exportDirectory = exportDir;

  for (const fileName of TEST_MODELS) {
    console.log(`=== ${fileName} ===`);
    const fdid = listfile.getByFilename(fileName);
    if (fdid === undefined) {
      check('fileDataID resolved', false, 'not in listfile');
      continue;
    }

    const { mdl, texturePaths } = await convertM2ToMdl(config, { fileDataID: fdid });
    check('MDL produced', mdl.geosets.length > 0, `${mdl.geosets.length} geosets`);
    check('textures resolved', texturePaths.size > 0, `${texturePaths.size} textures`);
    console.log('');
  }

  console.log(`${failures === 0 ? 'ALL CHECKS PASSED' : `${failures} CHECK(S) FAILED`} in ${((Date.now() - t0) / 1000).toFixed(1)}s`);
  process.exit(failures === 0 ? 0 : 1);
}

main().catch((e) => {
  write('Validation failed: %s', (e as Error).stack);
  process.exit(1);
});

/**
 * Phase 1 validation: native CASC reader vs live wow.export instance.
 *
 * Loads the local WoW installation through src/lib/wow (no wow.export),
 * verifies build info against the running wow.export REST server, and
 * sanity-checks raw file reads (magic numbers, listfile lookups).
 *
 * Usage: bun tests/wow-native/validate-casc.ts [--wow-dir <path>] [--product wow]
 */
import { CASCLocal } from '../../src/lib/wow/archive/casc/casc-source-local';
import * as listfile from '../../src/lib/wow/archive/casc/listfile';
import { load as loadTactKeys } from '../../src/lib/wow/archive/casc/tact-keys';
import { write } from '../../src/lib/wow/log';

// Note: the install root is the directory containing .build.info and Data/.
const WOW_DIR = getArg('--wow-dir') ?? process.env.CASC_LOCAL_WOW ?? 'D:\\Programs\\Blizzard Games\\World of Warcraft';
const PRODUCT = getArg('--product') ?? 'wow';
const WOWEXPORT_URL = process.env.WOWEXPORT_URL ?? 'http://127.0.0.1:17752';

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

  // Kick off listfile preload + tact keys in parallel with CASC init.
  const preloadPromise = listfile.preload();
  await loadTactKeys();

  const casc = new CASCLocal(WOW_DIR);
  await casc.init();

  console.log('Available products:');
  for (const p of casc.getProductList()) console.log(`  - ${p}`);

  const buildIndex = casc.builds.findIndex((b) => b.Product === PRODUCT);
  if (buildIndex === -1) throw new Error(`Product ${PRODUCT} not found in ${WOW_DIR}`);

  await preloadPromise;
  await casc.load(buildIndex);

  console.log(`\nNative CASC loaded in ${((Date.now() - t0) / 1000).toFixed(1)}s`);
  console.log(`Build: ${casc.getBuildName()} (${casc.getBuildKey()})`);

  // --- Compare build info with live wow.export ---
  try {
    const res = await fetch(`${WOWEXPORT_URL}/rest/getCascInfo`);
    if (res.ok) {
      const info = await res.json() as { buildName: string; buildKey: string };
      console.log('\n[vs wow.export]');
      check('buildName matches', info.buildName === casc.getBuildName(), `${info.buildName} vs ${casc.getBuildName()}`);
      check('buildKey matches', info.buildKey === casc.getBuildKey(), `${info.buildKey} vs ${casc.getBuildKey()}`);
    } else {
      console.log(`\nwow.export REST not ready (HTTP ${res.status}), skipping build comparison.`);
    }
  } catch (e) {
    console.log('\nwow.export REST unreachable, skipping build comparison.');
  }

  // --- Listfile sanity ---
  console.log('\n[listfile]');
  const murlocId = listfile.getByFilename('creature/murloc/murloc.m2');
  check('murloc.m2 fdid resolved', murlocId !== undefined, `fdid=${murlocId}`);
  const searchResults = listfile.getFilteredEntries('creature/murloc/');
  check('search creature/murloc/ has results', searchResults.length > 0, `${searchResults.length} entries`);

  // --- Raw file reads ---
  console.log('\n[file reads]');

  // M2 model (MD21 magic)
  const m2 = await casc.getFileByName('creature/murloc/murloc.m2');
  m2.processAllBlocks();
  const m2Magic = m2.readUInt32LE();
  check('murloc.m2 MD21 magic', m2Magic === 0x3132444D, `0x${m2Magic.toString(16)} size=${m2.byteLength}`);

  // BLP texture (BLP2 magic)
  const blp = await casc.getFileByName('interface/icons/inv_misc_questionmark.blp');
  blp.processAllBlocks();
  blp.seek(0);
  const blpMagic = blp.readString(4);
  check('inv_misc_questionmark.blp BLP2 magic', blpMagic === 'BLP2', `magic=${blpMagic} size=${blp.byteLength}`);

  // DB2 (WDC magic starts with 'WDC')
  const db2 = await casc.getFileByName('dbfilesclient/map.db2');
  db2.processAllBlocks();
  db2.seek(0);
  const db2Magic = db2.readString(4);
  check('map.db2 WDC magic', db2Magic.startsWith('WDC'), `magic=${db2Magic} size=${db2.byteLength}`);

  // WDT map tile
  const wdt = await casc.getFileByName('world/maps/azeroth/azeroth.wdt');
  wdt.processAllBlocks();
  check('azeroth.wdt readable', wdt.byteLength > 0, `size=${wdt.byteLength}`);

  // getFile by fdid path
  if (murlocId !== undefined) {
    const m2ById = await casc.getFile(murlocId);
    m2ById.processAllBlocks();
    check('getFile(fdid) == getFileByName md5', m2ById.calculateHash() === (m2.seek(0), m2.calculateHash()));
  }

  // fileExists checks
  console.log('\n[fileExists]');
  check('existing file', casc.fileExists(murlocId!));
  check('bogus fdid', !casc.fileExists(0xFFFFFF7));

  console.log(`\n${failures === 0 ? 'ALL CHECKS PASSED' : `${failures} CHECK(S) FAILED`} in ${((Date.now() - t0) / 1000).toFixed(1)}s`);
  process.exit(failures === 0 ? 0 : 1);
}

main().catch((e) => {
  write('Validation failed: %s', (e as Error).stack);
  process.exit(1);
});

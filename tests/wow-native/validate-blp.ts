/**
 * Phase 2 validation: native BLP decoder vs live wow.export PNG exports.
 *
 * Exports a set of BLP textures through the running wow.export instance
 * (REST /rest/exportTextures), decodes the produced PNGs with sharp, and
 * compares them pixel-for-pixel with the native decoder's output.
 *
 * Pixels must match exactly (both are CPU decoders of the same algorithm);
 * PNG container bytes are also compared and reported (zlib builds may differ).
 *
 * Usage: bun tests/wow-native/validate-blp.ts [--wow-dir <path>] [--product wow]
 */
import path from 'path';
import sharp from 'sharp';

import { BLPImage } from '@/lib/wow/formats/blp/blp';
import { CASCLocal } from '../../src/lib/wow/archive/casc/casc-source-local';
import * as listfile from '../../src/lib/wow/archive/casc/listfile';
import { load as loadTactKeys } from '../../src/lib/wow/archive/casc/tact-keys';
import { write } from '../../src/lib/wow/log';

const WOW_DIR = getArg('--wow-dir') ?? process.env.CASC_LOCAL_WOW ?? 'D:\\Programs\\Blizzard Games\\World of Warcraft';
const PRODUCT = getArg('--product') ?? 'wow';
const WOWEXPORT_URL = process.env.WOWEXPORT_URL ?? 'http://127.0.0.1:17752';

// Mix of encodings: DXT1/3/5 (encoding 2), palette (1), BGRA (3).
const TEST_TEXTURES = [
  'interface/icons/inv_misc_questionmark.blp',
  'creature/murloc/murlocskinblue.blp',
  'tileset/elwynn/elwynngrassbase.blp',
  'character/human/female/humanfemaleskin00_00.blp',
  'interface/glues/loadingscreens/loadscreendeathknight.blp',
  'spells/holy_circle.blp',
  'environments/stars/hellfireskynebula03.blp',
];

function getArg(name: string): string | undefined {
  const idx = process.argv.indexOf(name);
  return idx > -1 ? process.argv[idx + 1] : undefined;
}

let failures = 0;

function check(name: string, condition: boolean, detail?: string): void {
  if (condition) console.log(`  PASS ${name}${detail ? ` (${detail})` : ''}`);
  else {
    failures++;
    console.error(`  FAIL ${name}${detail ? ` (${detail})` : ''}`);
  }
}

async function decodePNG(input: Buffer | string): Promise<{ data: Buffer; width: number; height: number }> {
  const { data, info } = await sharp(input).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
  return { data, width: info.width, height: info.height };
}

async function main() {
  // --- Get wow.export config (export directory) ---
  const cfgRes = await fetch(`${WOWEXPORT_URL}/rest/getConfig`);
  if (!cfgRes.ok) throw new Error(`wow.export REST unavailable: HTTP ${cfgRes.status}`);
  const cfg = (await cfgRes.json() as { config: Record<string, unknown> }).config;
  const exportDir = String(cfg.exportDirectory);
  console.log(`wow.export exportDirectory: ${exportDir}`);

  // --- Load native CASC ---
  const preloadPromise = listfile.preload();
  await loadTactKeys();

  const casc = new CASCLocal(WOW_DIR);
  await casc.init();
  const buildIndex = casc.builds.findIndex((b) => b.Product === PRODUCT);
  if (buildIndex === -1) throw new Error(`Product ${PRODUCT} not found`);
  await preloadPromise;
  await casc.load(buildIndex);
  console.log(`Native CASC loaded: ${casc.getBuildName()}`);

  for (const fileName of TEST_TEXTURES) {
    const fdid = listfile.getByFilename(fileName);
    console.log(`\n[${fileName}] fdid=${fdid}`);
    if (fdid === undefined) {
      check('fdid resolved', false);
      continue;
    }

    // wow.export REST export
    const expRes = await fetch(`${WOWEXPORT_URL}/rest/exportTextures`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ fileDataID: [fdid] }),
    });
    const expJson = await expRes.json() as { succeeded?: { fileDataID: number; file: string }[] };
    const exported = expJson.succeeded?.[0];
    if (!exported) {
      check('wow.export export succeeded', false, JSON.stringify(expJson).slice(0, 200));
      continue;
    }

    const refPath = path.isAbsolute(exported.file) ? exported.file : path.join(exportDir, exported.file);
    const ref = await decodePNG(refPath);

    // Native decode
    const blpData = await casc.getFile(fdid, false, true);
    blpData.processAllBlocks();
    blpData.seek(0);
    const blp = new BLPImage(blpData);
    const pngBuf = blp.toPNG(0b1111, 0);
    const native = await decodePNG(pngBuf.raw);

    console.log(`  encoding=${blp.encoding} alphaDepth=${blp.alphaDepth} alphaEnc=${blp.alphaEncoding} ${blp.width}x${blp.height}`);
    check('dimensions match', native.width === ref.width && native.height === ref.height, `${native.width}x${native.height} vs ${ref.width}x${ref.height}`);

    if (native.width === ref.width && native.height === ref.height) {
      let maxDelta = 0;
      let diffCount = 0;
      for (let i = 0; i < native.data.length; i++) {
        const d = Math.abs(native.data[i] - ref.data[i]);
        if (d > 0) {
          diffCount++;
          if (d > maxDelta) maxDelta = d;
        }
      }
      check('pixels identical', diffCount === 0, diffCount === 0 ? 'exact' : `${diffCount} bytes differ, maxDelta=${maxDelta}`);
    }

    // PNG container bytes (informational; zlib builds may differ)
    const fs = await import('fs/promises');
    const refBytes = await fs.readFile(refPath);
    const bytesEqual = refBytes.length === pngBuf.byteLength && refBytes.equals(pngBuf.raw);
    console.log(`  PNG container bytes: ${bytesEqual ? 'identical' : `differ (${pngBuf.byteLength} vs ${refBytes.length})`}`);
  }

  console.log(`\n${failures === 0 ? 'ALL CHECKS PASSED' : `${failures} CHECK(S) FAILED`}`);
  process.exit(failures === 0 ? 0 : 1);
}

main().catch((e) => {
  write('Validation failed: %s', (e as Error).stack);
  process.exit(1);
});

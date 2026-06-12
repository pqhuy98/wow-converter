/**
 * Phase 3 validation: native M2 export vs live wow.export instance.
 *
 * Exports M2 models (doodads, no creature skins -- displays are Phase 4)
 * through the live wow.export REST API and through the native M2Exporter,
 * then byte-compares every artifact (.obj, .mtl, .json, _bones.json,
 * .phys.obj, .png textures).
 *
 * Usage: bun tests/wow-native/validate-m2.ts [--wow-dir <path>]
 */
import crypto from 'crypto';
import fs from 'fs';
import path from 'path';

import { CASCLocal } from '../../src/lib/wow/archive/casc/casc-source-local';
import * as listfile from '../../src/lib/wow/archive/casc/listfile';
import { load as loadTactKeys } from '../../src/lib/wow/archive/casc/tact-keys';
import { wowConfig } from '../../src/lib/wow/server/config';
import { getExportPath, replaceExtension } from '@/lib/wow/export/writers/export-helper';
import { M2Exporter } from '../../src/lib/wow/export/m2/m2-exporter';
import { write } from '../../src/lib/wow/log';
import { runtimeState } from '../../src/lib/wow/server/runtime';

const WOW_DIR = getArg('--wow-dir') ?? process.env.CASC_LOCAL_WOW ?? 'D:\\Programs\\Blizzard Games\\World of Warcraft';
const PRODUCT = getArg('--product') ?? 'wow';
const WOWEXPORT_URL = process.env.WOWEXPORT_URL ?? 'http://127.0.0.1:17752';
const NATIVE_OUT = path.resolve(__dirname, 'out', 'm2-native');

// Doodad M2s without creature displays (variantTextures = []).
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

function sha256(buf: Buffer): string {
  return crypto.createHash('sha256').update(buf).digest('hex');
}

async function comparePixelsExact(a: Buffer, b: Buffer): Promise<{ equal: boolean; detail: string }> {
  const sharp = (await import('sharp')).default;
  const [ia, ib] = await Promise.all([
    sharp(a).ensureAlpha().raw().toBuffer({ resolveWithObject: true }),
    sharp(b).ensureAlpha().raw().toBuffer({ resolveWithObject: true }),
  ]);
  if (ia.info.width !== ib.info.width || ia.info.height !== ib.info.height) {
    return { equal: false, detail: `dims ${ia.info.width}x${ia.info.height} vs ${ib.info.width}x${ib.info.height}` };
  }
  let diff = 0;
  let maxDelta = 0;
  for (let i = 0; i < ia.data.length; i++) {
    const d = Math.abs(ia.data[i] - ib.data[i]);
    if (d > 0) { diff++; if (d > maxDelta) maxDelta = d; }
  }
  return { equal: diff === 0, detail: diff === 0 ? 'pixels exact' : `${diff} bytes differ, maxDelta=${maxDelta}` };
}

async function main() {
  const t0 = Date.now();

  // --- Mirror the live wow.export export-shaping config ---
  const cfgRes = await fetch(`${WOWEXPORT_URL}/rest/getConfig`);
  if (!cfgRes.ok) throw new Error(`wow.export REST unreachable (HTTP ${cfgRes.status})`);
  const remoteConfig = ((await cfgRes.json()) as { config: Record<string, unknown> }).config;

  const mirrorKeys = [
    'modelsExportCollision', 'modelsExportUV2', 'modelsExportTextures', 'modelsExportAlpha',
    'exportM2Meta', 'exportM2Bones', 'enableSharedTextures', 'enableAbsoluteMTLPaths',
    'removePathSpaces', 'pathFormat', 'overwriteFiles',
  ] as const;
  for (const key of mirrorKeys) {
    if (remoteConfig[key] !== undefined) (wowConfig as unknown as Record<string, unknown>)[key] = remoteConfig[key];
  }
  const refExportDir = String(remoteConfig.exportDirectory).replace(/\//g, path.sep);
  console.log('wow.export exportDirectory:', refExportDir);

  // --- Load native CASC ---
  const preloadPromise = listfile.preload();
  await loadTactKeys();
  const casc = new CASCLocal(WOW_DIR);
  await casc.init();
  const buildIndex = casc.builds.findIndex((b) => b.Product === PRODUCT);
  if (buildIndex === -1) throw new Error(`Product ${PRODUCT} not found`);
  await preloadPromise;
  await casc.load(buildIndex);
  runtimeState.casc = casc;
  console.log(`Native CASC loaded in ${((Date.now() - t0) / 1000).toFixed(1)}s\n`);

  fs.rmSync(NATIVE_OUT, { recursive: true, force: true });
  wowConfig.exportDirectory = NATIVE_OUT;

  for (const fileName of TEST_MODELS) {
    console.log(`=== ${fileName} ===`);
    const fdid = listfile.getByFilename(fileName);
    if (fdid === undefined) {
      check('fileDataID resolved', false, 'not in listfile');
      continue;
    }

    // --- REST export (reference) ---
    const restRes = await fetch(`${WOWEXPORT_URL}/rest/exportModels`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ fileDataID: fdid }),
    });
    const restJson = await restRes.json() as {
      succeeded?: { fileDataID: number; files: { type: string; file: string }[] }[];
    };
    const refFiles = restJson.succeeded?.[0]?.files ?? [];
    check('REST export succeeded', restRes.ok && refFiles.length > 0, `${refFiles.length} files`);
    if (refFiles.length === 0) continue;

    // --- Native export ---
    const data = await casc.getFile(fdid);
    const exporter = new M2Exporter(data, [], fdid);
    const exportPath = replaceExtension(getExportPath(fileName), '.obj');
    await exporter.exportAsOBJ(exportPath, undefined, wowConfig.modelsExportCollision);

    // --- Compare artifacts ---
    for (const ref of refFiles) {
      const rel = path.relative(refExportDir, ref.file);
      const nativePath = path.join(NATIVE_OUT, rel);
      const label = `${ref.type} ${rel}`;

      if (!fs.existsSync(nativePath)) {
        check(label, false, 'missing in native output');
        continue;
      }

      const refBuf = fs.readFileSync(ref.file);
      const nativeBuf = fs.readFileSync(nativePath);

      if (refBuf.equals(nativeBuf)) {
        check(label, true, `byte-identical (${nativeBuf.length} bytes)`);
      } else if (rel.toLowerCase().endsWith('.png')) {
        // PNG containers may differ across zlib builds; require pixel-exact.
        const result = await comparePixelsExact(refBuf, nativeBuf);
        check(label, result.equal, result.detail);
      } else {
        check(label, false, `bytes differ (${refBuf.length} vs ${nativeBuf.length}, sha ${sha256(refBuf).slice(0, 8)} vs ${sha256(nativeBuf).slice(0, 8)})`);
      }
    }
    console.log('');
  }

  console.log(`${failures === 0 ? 'ALL CHECKS PASSED' : `${failures} CHECK(S) FAILED`} in ${((Date.now() - t0) / 1000).toFixed(1)}s`);
  process.exit(failures === 0 ? 0 : 1);
}

main().catch((e) => {
  write('Validation failed: %s', (e as Error).stack);
  process.exit(1);
});

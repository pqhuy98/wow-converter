/**
 * Phase 7 validation: native ADT export vs live wow-data-server instance.
 *
 * Exports a map tile through the live wow-data-server REST API and through the
 * native ADTExporter, then compares artifacts:
 *  - OBJ/MTL/CSV/JSON: byte-identical
 *  - tex_*.png terrain bakes: pixel tolerance (GL vs CPU rasterizer)
 *
 * Usage: bun tests/wow-native/validate-adt.ts [--wow-dir <path>] [--map azeroth]
 *        [--map-id 0] [--tile-x 31] [--tile-y 28] [--quality 1024]
 */
import crypto from 'crypto';
import fs from 'fs';
import path from 'path';

import { ADTExporter } from '@/lib/wow/export/adt/adt-exporter';
import { buildADTExportOptions } from '@/lib/wow/export/adt/map-export-utils';
import { getExportPath } from '@/lib/wow/export/writers/export-helper';

import { CASCLocal } from '../../src/lib/wow/archive/casc/casc-source-local';
import * as listfile from '../../src/lib/wow/archive/casc/listfile';
import { load as loadTactKeys } from '../../src/lib/wow/archive/casc/tact-keys';
import { write } from '../../src/lib/wow/log';
import { wowConfig } from '../../src/lib/wow/server/config';
import { runtimeState } from '../../src/lib/wow/server/runtime';

const WOW_DIR = getArg('--wow-dir') ?? process.env.CASC_LOCAL_WOW ?? 'D:\\Programs\\Blizzard Games\\World of Warcraft';
const PRODUCT = getArg('--product') ?? 'wow';
const WOW_DATA_SERVER_URL = process.env.WOW_DATA_SERVER_URL ?? 'http://127.0.0.1:17753';
const NATIVE_OUT = path.resolve(__dirname, 'out', 'adt-native');

const MAP_DIR = getArg('--map') ?? 'azeroth';
const MAP_ID = Number(getArg('--map-id') ?? 0);
const TILE_X = Number(getArg('--tile-x') ?? 31);
const TILE_Y = Number(getArg('--tile-y') ?? 28);
const QUALITY = Number(getArg('--quality') ?? 1024);
const INCLUDE_MODELS = process.argv.includes('--models');

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

/** Tolerance pixel compare for GPU-vs-CPU bakes. */
async function comparePixelsTolerant(a: Buffer, b: Buffer): Promise<{ equal: boolean; detail: string }> {
  const sharp = (await import('sharp')).default;
  const [ia, ib] = await Promise.all([
    sharp(a).ensureAlpha().raw().toBuffer({ resolveWithObject: true }),
    sharp(b).ensureAlpha().raw().toBuffer({ resolveWithObject: true }),
  ]);
  if (ia.info.width !== ib.info.width || ia.info.height !== ib.info.height) {
    return { equal: false, detail: `dims ${ia.info.width}x${ia.info.height} vs ${ib.info.width}x${ib.info.height}` };
  }

  let sumDelta = 0;
  let maxDelta = 0;
  let over8 = 0;
  const n = ia.data.length;
  for (let i = 0; i < n; i++) {
    const d = Math.abs(ia.data[i] - ib.data[i]);
    sumDelta += d;
    if (d > maxDelta) maxDelta = d;
    if (d > 8) over8++;
  }
  const meanDelta = sumDelta / n;
  const over8Pct = (over8 / n) * 100;
  const detail = `meanDelta=${meanDelta.toFixed(3)} maxDelta=${maxDelta} over8=${over8Pct.toFixed(3)}%`;

  // Tolerances: average error under 2/255 and <1% of samples off by more than 8.
  return { equal: meanDelta < 2 && over8Pct < 1, detail };
}

async function main() {
  const t0 = Date.now();
  // REST tileIndex = tileX * 64 + tileY; ADTExporter then derives
  // tileID = floor(tileIndex / 64) + '_' + (tileIndex % 64) = TILE_X_TILE_Y.
  const tileID = `${TILE_X}_${TILE_Y}`;

  // --- Mirror the live wow-data-server export-shaping config ---
  const cfgRes = await fetch(`${WOW_DATA_SERVER_URL}/rest/getConfig`);
  if (!cfgRes.ok) throw new Error(`wow-data-server REST unreachable (HTTP ${cfgRes.status})`);
  const remoteConfig = ((await cfgRes.json()) as { config: Record<string, unknown> }).config;

  const mirrorKeys = [
    'modelsExportCollision', 'modelsExportUV2', 'modelsExportTextures', 'modelsExportAlpha',
    'exportM2Meta', 'exportM2Bones', 'exportWMOMeta', 'enableSharedTextures', 'enableSharedChildren',
    'enableAbsoluteMTLPaths', 'enableAbsoluteCSVPaths', 'removePathSpaces', 'pathFormat',
    'overwriteFiles', 'splitAlphaMaps', 'splitLargeTerrainBakes', 'mapsIncludeHoles', 'exportFoliageMeta',
  ] as const;
  for (const key of mirrorKeys) {
    if (remoteConfig[key] !== undefined) (wowConfig as unknown as Record<string, unknown>)[key] = remoteConfig[key];
  }
  const refExportDir = String(remoteConfig.exportDirectory).replace(/\//g, path.sep);
  console.log('wow-data-server exportDirectory:', refExportDir);

  const requestBody = {
    mapID: MAP_ID,
    mapDir: MAP_DIR,
    tileX: TILE_X,
    tileY: TILE_Y,
    quality: QUALITY,
    includeM2: INCLUDE_MODELS,
    includeWMO: INCLUDE_MODELS,
    includeWMOSets: INCLUDE_MODELS,
    includeGameObjects: false,
    includeLiquid: true,
    includeFoliage: false,
    includeHoles: true,
  };

  // --- REST export (reference) ---
  console.log(`=== REST exportADT ${MAP_DIR} ${tileID} q=${QUALITY} models=${INCLUDE_MODELS} ===`);
  const tRest = Date.now();
  const restRes = await fetch(`${WOW_DATA_SERVER_URL}/rest/exportADT`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(requestBody),
    signal: AbortSignal.timeout(30 * 60 * 1000),
  });
  const restJson = await restRes.json() as { id: string; exportPath?: string; mainFile?: string; message?: string };
  check('REST export succeeded', restRes.ok && restJson.id === 'EXPORT_RESULT', restJson.message ?? restJson.mainFile ?? '');
  if (!restRes.ok) process.exit(1);
  console.log(`REST export took ${((Date.now() - tRest) / 1000).toFixed(1)}s`);

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

  // --- Native export ---
  const tNative = Date.now();
  const options = buildADTExportOptions(wowConfig, {
    mapsIncludeM2: requestBody.includeM2,
    mapsIncludeWMO: requestBody.includeWMO,
    mapsIncludeWMOSets: requestBody.includeWMOSets,
    mapsIncludeGameObjects: requestBody.includeGameObjects,
    mapsIncludeLiquid: requestBody.includeLiquid,
    mapsIncludeFoliage: requestBody.includeFoliage,
    mapsIncludeHoles: requestBody.includeHoles,
  });
  const baseDir = getExportPath(path.join('maps', MAP_DIR));
  const tileIndex = TILE_X * 64 + TILE_Y;
  const exporter = new ADTExporter(MAP_ID, MAP_DIR, tileIndex);
  await exporter.export(baseDir, QUALITY, undefined, options);
  console.log(`Native export took ${((Date.now() - tNative) / 1000).toFixed(1)}s\n`);

  // --- Compare artifacts ---
  const refDir = path.join(refExportDir, 'maps', MAP_DIR);
  const nativeDir = baseDir;

  // Collect all reference files belonging to this tile (plus shared textures when enabled).
  const collect = (dir: string, base = dir): string[] => {
    if (!fs.existsSync(dir)) return [];
    const out: string[] = [];
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) out.push(...collect(full, base));
      else out.push(path.relative(base, full));
    }
    return out;
  };

  const tileFiles = collect(nativeDir).filter((rel) => rel.includes(tileID));
  check('native produced tile artifacts', tileFiles.length > 0, `${tileFiles.length} files`);

  for (const rel of tileFiles.sort()) {
    const refPath = path.join(refDir, rel);
    const nativePath = path.join(nativeDir, rel);
    const label = rel;

    if (!fs.existsSync(refPath)) {
      check(label, false, 'missing in REST output');
      continue;
    }

    const refBuf = fs.readFileSync(refPath);
    const nativeBuf = fs.readFileSync(nativePath);

    if (refBuf.equals(nativeBuf)) {
      check(label, true, `byte-identical (${nativeBuf.length} bytes)`);
    } else if (rel.toLowerCase().endsWith('.png')) {
      const result = await comparePixelsTolerant(refBuf, nativeBuf);
      check(label, result.equal, result.detail);
    } else {
      check(label, false, `bytes differ (${refBuf.length} vs ${nativeBuf.length}, sha ${sha256(refBuf).slice(0, 8)} vs ${sha256(nativeBuf).slice(0, 8)})`);
    }
  }

  // Also flag reference tile files the native export did not produce.
  const refTileFiles = collect(refDir).filter((rel) => rel.includes(tileID));
  for (const rel of refTileFiles) {
    if (!tileFiles.includes(rel)) check(rel, false, 'missing in native output');
  }

  console.log(`\n${failures === 0 ? 'ALL CHECKS PASSED' : `${failures} CHECK(S) FAILED`} in ${((Date.now() - t0) / 1000).toFixed(1)}s`);
  process.exit(failures === 0 ? 0 : 1);
}

main().catch((e) => {
  write('Validation failed: %s', (e as Error).stack);
  console.error(e);
  process.exit(1);
});

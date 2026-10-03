import { beforeAll, expect, setDefaultTimeout, test } from 'bun:test';

import {
  api, ensureCascLoaded, isRecord, jsonInit, readBody,
} from './client';

setDefaultTimeout(120_000);

beforeAll(async () => {
  await ensureCascLoaded();
}, 180_000);

test('config, wow status, cache size, and openapi', async () => {
  const config = record(await readBody(await api('/api/get-config')), 'config');
  expect(typeof config.exportAssetDir).toBe('string');
  expect(config.exportAssetDir).not.toBe('');
  expect(typeof config.buildKey).toBe('string');
  expect(config.buildKey).not.toBe('');
  expect(config.isSharedHosting).toBe(false);
  expect(typeof config.isDev).toBe('boolean');
  expect(typeof config.isClassic).toBe('boolean');
  expect(typeof config.mapGenerateHalt).toBe('boolean');

  const status = record(await readBody(await api('/api/wow-config/status')), 'status');
  expect(status.cascLoaded).toBe(true);
  expect(status.cascLoading).toBe(false);
  expect(status.wowDataServerReachable).toBe(true);
  expect(status.needsSetup).toBe(false);
  expect(status.error).toBeNull();
  const wow = record(status.config, 'status.config');
  expect(typeof wow.mode).toBe('string');
  expect(typeof wow.product).toBe('string');
  expect(items(status.products, 'products').length).toBeGreaterThan(0);
  expect(items(status.regions, 'regions').length).toBeGreaterThan(0);
  expect(typeof status.outputDirectory).toBe('string');
  expect(status.outputDirectory).not.toBe('');

  const cache = record(await readBody(await api('/api/wow-config/cache-size')), 'cache');
  expect(numberOf(cache.bytes, 'cache.bytes')).toBeGreaterThanOrEqual(0);

  const docs = await api('/docs/openapi.yaml');
  expect(docs.status).toBe(200);
  const yaml = await docs.text();
  expect(yaml.includes('/api/get-config')).toBe(true);
});

test('browse, model skins, and local file check', async () => {
  const missing = await api('/api/browse');
  expect(missing.status).toBe(400);
  expect(errorOf(await missing.json()).error).toBe('q is required');

  const badKind = await api('/api/browse?q=nope');
  expect(badKind.status).toBe(400);
  expect(errorOf(await badKind.json()).error).toBe('q must be "model" or "texture"');

  const model = await firstFile('/api/browse?q=model&search=', [
    'creature_spellportal_purple.m2',
    'fireball_missile.m2',
  ]);
  const skinsResponse = await api(`/api/browse/model-skins?fileDataID=${model.fileDataID}`);
  const skins = record(await readBody(skinsResponse), 'skins');
  expect(skins.fileDataID).toBe(model.fileDataID);
  expect(skins.fileName).toBe(model.fileName);
  expect(Array.isArray(skins.skins)).toBe(true);

  const missingSkin = await api('/api/browse/model-skins');
  expect(missingSkin.status).toBe(400);
  expect(errorOf(await missingSkin.json()).error).toBe('fileDataID is required');
  const unknownSkin = await api('/api/browse/model-skins?fileDataID=999999999');
  expect(unknownSkin.status).toBe(404);
  expect(errorOf(await unknownSkin.json()).error).toBe('Model not found');

  const empty = record(await readBody(await api('/api/export/character/check-local-file')), 'check');
  expect(empty.ok).toBe(false);
  expect(Array.isArray(empty.similarFiles)).toBe(true);

  const localPath = model.fileName.replace(/\.m2$/i, '');
  const found = record(await readBody(await api(`/api/export/character/check-local-file?localPath=${encodeURIComponent(localPath)}`)), 'check-found');
  expect(found.ok).toBe(true);
  expect(items(found.similarFiles, 'similarFiles').length).toBeGreaterThan(0);

  const texture = await firstFile('/api/browse?q=texture&search=', ['inv_misc_questionmark.blp']);
  expect(texture.fileName.toLowerCase().endsWith('.blp')).toBe(true);
});

test('sounds, transcripts, and a sound zip', async () => {
  const sound = await firstFile('/api/sound?search=', [
    'bloodelfmale_err_alreadyingroup01.ogg',
    'err_alreadyingroup01.ogg',
  ]);
  expect(sound.fileName.toLowerCase().includes('.ogg')).toBe(true);

  const bytes = await api(`/api/sound/${sound.fileDataID}`);
  expect(bytes.status).toBe(200);
  expect(bytes.headers.get('content-type')).toBe('audio/ogg');
  const ogg = new Uint8Array(await bytes.arrayBuffer());
  expect(ogg.length).toBeGreaterThan(0);
  expect(ogg[0]).toBe(79);
  expect(ogg[1]).toBe(103);
  expect(ogg[2]).toBe(103);
  expect(ogg[3]).toBe(83);

  const badId = await api('/api/sound/0');
  expect(badId.status).toBe(400);
  expect(errorOf(await badId.json()).error).toBe('Invalid fileDataID');
  const missing = await api('/api/sound/999999999');
  expect(missing.status).toBe(404);
  expect(errorOf(await missing.json()).error).toBe('Sound not found');

  const emptyZip = await api('/api/sound/zip', jsonInit({ fileDataIDs: [] }));
  expect(emptyZip.status).toBe(400);
  expect(errorOf(await emptyZip.json()).error).toBe('fileDataIDs must contain 1 to 1000 ids');

  const zip = await api('/api/sound/zip', jsonInit({ fileDataIDs: [sound.fileDataID] }));
  expect(zip.status).toBe(200);
  expect(zip.headers.get('content-type')).toBe('application/zip');
  const soundZip = new Uint8Array(await zip.arrayBuffer());
  expect(isZip(soundZip)).toBe(true);
  expect(zipEntryNames(soundZip)).toContain(sound.fileName);

  const transcripts = await readBody(await api('/api/sound/transcripts'));
  const entries = record(transcripts, 'transcripts');
  const keys = Object.keys(entries);
  expect(keys.length).toBeGreaterThan(0);
  expect(typeof entries[keys[0]]).toBe('string');
}, 60_000);

test('maps, mask, creatures, and a minimap tile', async () => {
  const list = items(await readBody(await api('/api/maps')), 'maps');
  expect(list.length).toBeGreaterThan(0);
  const first = record(list[0], 'map');
  expect(typeof first.id).toBe('number');
  expect(typeof first.name).toBe('string');
  expect(typeof first.dir).toBe('string');
  expect(typeof first.expansionID).toBe('number');

  const chosen = await firstTexturedMap(list);
  const mask = record(await readBody(await api(`/api/maps/${encodeURIComponent(chosen.dir)}/wdt-mask`)), 'mask');
  expect(mask.map).toBe(chosen.dir);
  expect(mask.size).toBe(64);
  const tile = record(items(mask.tiles, 'tiles')[0], 'tile');
  expect(typeof tile.x).toBe('number');
  expect(typeof tile.y).toBe('number');
  expect(tile.hasTexture).toBe(true);

  const unknown = record(await readBody(await api('/api/maps/not-a-map/wdt-mask')), 'unknown-mask');
  expect(unknown.map).toBe('not-a-map');
  expect(unknown.size).toBe(64);
  expect(unknown.tiles).toBeNull();

  const creatures = record(await readBody(await api(
    `/api/maps/${encodeURIComponent(chosen.dir)}/creatures-check`,
    jsonInit({ tiles: [{ x: tile.x, y: tile.y }] }),
  )), 'creatures');
  expect(typeof creatures.hasCreatures).toBe('boolean');
  expect(typeof creatures.creatureCount).toBe('number');
  expect(Array.isArray(creatures.checkedTiles)).toBe(true);

  const unknownCreatures = await api('/api/maps/not-a-map/creatures-check', jsonInit({ tiles: [{ x: 1, y: 1 }] }));
  expect(unknownCreatures.status).toBe(404);
  expect(errorOf(await unknownCreatures.json()).error).toBe('Unknown map');
  const badTiles = await api(`/api/maps/${encodeURIComponent(chosen.dir)}/creatures-check`, jsonInit({}));
  expect(badTiles.status).toBe(400);
  expect(errorOf(await badTiles.json()).error).toBe('Invalid request body');

  const minimap = await api(`/api/maps/${encodeURIComponent(chosen.dir)}/minimap/${tile.x}/${tile.y}`);
  expect(minimap.status).toBe(200);
  expect(minimap.headers.get('content-type')).toBe('image/png');
  expect(isPng(new Uint8Array(await minimap.arrayBuffer()))).toBe(true);
  const outOfRange = await api(`/api/maps/${encodeURIComponent(chosen.dir)}/minimap/99/0`);
  expect(outOfRange.status).toBe(400);
  expect(errorOf(await outOfRange.json()).error).toBe('x and y must be within 0..63');

  const active = await readBody(await api('/api/maps/generate-wc3/active'));
  expect(Array.isArray(active)).toBe(true);
  const missingJob = await api('/api/maps/generate-wc3/status/missing-job');
  expect(missingJob.status).toBe(404);
  expect(errorOf(await missingJob.json()).error).toBe('Generate job not found');
});

test('texture png and blp export', async () => {
  const texturePath = 'interface/icons/inv_misc_questionmark.blp';
  const png = await api(`/api/texture/png/${texturePath}`);
  expect(png.status).toBe(200);
  expect(png.headers.get('content-type')).toBe('image/png');
  expect(isPng(new Uint8Array(await png.arrayBuffer()))).toBe(true);
  const etag = png.headers.get('etag');
  if (typeof etag !== 'string' || etag === '') throw new Error('texture png has no etag');
  const cached = await api(`/api/texture/png/${texturePath}`, { headers: { 'If-None-Match': etag } });
  expect(cached.status).toBe(304);
  expect(await cached.text()).toBe('');

  const iconMissingSize = await api(`/api/texture/png/${texturePath}?mode=icon`);
  expect(iconMissingSize.status).toBe(400);
  expect(errorOf(await iconMissingSize.json()).error).toBe('size is required for icon mode');
  const icon = await api(`/api/texture/png/${texturePath}?mode=icon&size=64x64`);
  expect(icon.status).toBe(200);
  expect(icon.headers.get('content-type')).toBe('image/png');
  expect(isPng(new Uint8Array(await icon.arrayBuffer()))).toBe(true);

  const missing = await api('/api/texture/png/interface/icons/does-not-exist.blp');
  expect(missing.status).toBe(404);
  expect(errorOf(await missing.json()).error).toBe('Texture not found');

  const empty = await api('/api/texture/blp', jsonInit({}));
  expect(empty.status).toBe(400);
  expect(errorOf(await empty.json()).error).toBe('No items provided');

  const exported = record(await readBody(await api('/api/texture/blp', jsonInit({
    items: [{ texturePath }],
  }))), 'blp');
  expect(numberOf(exported.count, 'count')).toBeGreaterThan(0);
  const paths = items(exported.paths, 'paths');
  expect(paths.length).toBeGreaterThan(0);
  expect(typeof exported.outputDirectory).toBe('string');
  const written = textOf(paths[0], 'blp path');
  const blp = await api(`/api/assets/${encodePath(written)}`);
  expect(blp.status).toBe(200);
  expect(isBlp1(new Uint8Array(await blp.arrayBuffer()))).toBe(true);
});

test('character export rejects a bad body and a missing job', async () => {
  const demos = await readBody(await api('/api/export/character/demos'));
  expect(Array.isArray(demos)).toBe(true);
  const recentList = items(await readBody(await api('/api/export/character/recent')), 'recent');
  if (recentList.length > 0) {
    const job = record(recentList[0], 'recent');
    expect(typeof job.id).toBe('string');
    expect(typeof job.status).toBe('string');
  }

  const invalid = await api('/api/export/character', jsonInit({}));
  expect(invalid.status).toBe(400);
  const body = errorOf(await invalid.json());
  expect(body.error).toBe('Invalid request body');
  expect(body.issues).toContain('character is required');

  const missing = await api('/api/export/character/status/missing-job');
  expect(missing.status).toBe(404);
  expect(errorOf(await missing.json()).error).toBe('Export request not found');

  const download = await api('/api/download', jsonInit({}));
  expect(download.status).toBe(400);
  expect(errorOf(await download.json()).error).toBe('files is required');
  const absent = await api('/api/download', jsonInit({
    files: ['api-sanity/does-not-exist.mdx'],
    source: 'browse',
  }));
  expect(absent.status).toBe(400);
  expect(absent.headers.get('content-type')).toBe('application/json');
  expect(errorOf(await absent.json()).error).toBe('File not found');
});

test('character export writes an mdx and a zip', async () => {
  const model = await firstFile('/api/browse?q=model&search=', [
    'creature_spellportal_purple.m2',
    'fireball_missile.m2',
  ]);
  const localPath = model.fileName.replace(/\.m2$/i, '');
  const queued = record(await readBody(await api('/api/export/character', jsonInit({
    character: {
      base: { type: 'local', value: localPath },
      inGameMovespeed: 270,
      keepCinematic: true,
      noDecay: true,
    },
    outputFileName: `api-sanity/${localPath.split('/').pop()}`,
    optimization: {},
    format: 'mdx',
    formatVersion: '1000',
    isBrowse: true,
  }))), 'export');
  expect(typeof queued.id).toBe('string');
  const done = await waitUntilDone(textOf(queued.id, 'job id'));
  expect(done.status).toBe('done');
  const result = record(done.result, 'export result');
  const models = assets(result.exportedModels);
  expect(models.length).toBeGreaterThan(0);
  expect(models[0].size).toBeGreaterThan(0);
  expect(models[0].path.toLowerCase().endsWith('.mdx')).toBe(true);
  expect(typeof result.versionId).toBe('string');
  expect(typeof result.outputDirectory).toBe('string');

  const asset = await api(`/api/browse-assets/${encodePath(models[0].path)}`);
  expect(asset.status).toBe(200);
  const mdx = new Uint8Array(await asset.arrayBuffer());
  expect(mdx.length).toBe(models[0].size);
  expect(String.fromCharCode(mdx[0], mdx[1], mdx[2], mdx[3])).toBe('MDLX');

  const files = [...models, ...assets(result.exportedTextures)].map((file) => file.path);
  const zip = await api('/api/download', jsonInit({ files, source: 'browse' }));
  expect(zip.status).toBe(200);
  expect(zip.headers.get('content-type')).toBe('application/zip');
  const archive = new Uint8Array(await zip.arrayBuffer());
  expect(isZip(archive)).toBe(true);
  const names = zipEntryNames(archive);
  for (const file of files) expect(names).toContain(downloadEntryName(file));
}, 180_000);

async function waitUntilDone(id: string): Promise<Record<string, unknown>> {
  const deadline = Date.now() + 150_000;
  while (Date.now() < deadline) {
    const job = record(await readBody(await api(`/api/export/character/status/${id}`)), 'job');
    if (job.status === 'failed') {
      throw new Error(`export failed: ${typeof job.error === 'string' ? job.error : JSON.stringify(job)}`);
    }
    if (job.status === 'done') return job;
    await Bun.sleep(500);
  }
  throw new Error(`export ${id} did not finish`);
}

async function firstFile(prefix: string, queries: readonly string[]): Promise<{ fileDataID: number; fileName: string }> {
  for (const query of queries) {
    const body = await readBody(await api(`${prefix}${encodeURIComponent(query)}`));
    if (!Array.isArray(body)) continue;
    for (const item of body) {
      if (!isRecord(item) || typeof item.fileDataID !== 'number' || typeof item.fileName !== 'string') continue;
      if (item.fileName.toLowerCase().includes(query.toLowerCase())) {
        return { fileDataID: item.fileDataID, fileName: item.fileName };
      }
    }
  }
  throw new Error(`none of these files were listed: ${queries.join(', ')}`);
}

async function firstTexturedMap(maps: readonly unknown[]): Promise<{ dir: string }> {
  for (const item of maps.slice(0, 12)) {
    if (!isRecord(item) || typeof item.dir !== 'string') continue;
    const mask = record(await readBody(await api(`/api/maps/${encodeURIComponent(item.dir)}/wdt-mask`)), 'mask');
    if (!Array.isArray(mask.tiles) || mask.tiles.length === 0) continue;
    const tile = mask.tiles[0];
    if (isRecord(tile) && tile.hasTexture === true) return { dir: item.dir };
  }
  throw new Error('no map with a textured tile in the first maps');
}

function record(value: unknown, label: string): Record<string, unknown> {
  if (!isRecord(value)) throw new Error(`${label} is not an object`);
  return value;
}

function items(value: unknown, label: string): unknown[] {
  if (!Array.isArray(value)) throw new Error(`${label} is not an array`);
  const out: unknown[] = [];
  for (const item of value) out.push(item);
  return out;
}

function numberOf(value: unknown, label: string): number {
  if (typeof value !== 'number') throw new Error(`${label} is not a number`);
  return value;
}

function textOf(value: unknown, label: string): string {
  if (typeof value !== 'string') throw new Error(`${label} is not a string`);
  return value;
}

function errorOf(value: unknown): { error: string; issues: string[] } {
  const body = record(value, 'error');
  if (typeof body.error !== 'string') throw new Error('error message missing');
  const issues = Array.isArray(body.issues)
    ? body.issues.filter((item): item is string => typeof item === 'string')
    : [];
  return { error: body.error, issues };
}

function assets(value: unknown): { path: string; size: number }[] {
  if (!Array.isArray(value)) return [];
  const out: { path: string; size: number }[] = [];
  for (const item of value) {
    if (!isRecord(item) || typeof item.path !== 'string' || typeof item.size !== 'number') continue;
    out.push({ path: item.path, size: item.size });
  }
  return out;
}

function encodePath(path: string): string {
  return path.split('/').map((part) => encodeURIComponent(part)).join('/');
}

function isPng(bytes: Uint8Array): boolean {
  return bytes.length > 8 && bytes[0] === 137 && bytes[1] === 80 && bytes[2] === 78 && bytes[3] === 71;
}

function isZip(bytes: Uint8Array): boolean {
  return bytes.length > 4 && bytes[0] === 80 && bytes[1] === 75;
}

function isBlp1(bytes: Uint8Array): boolean {
  return bytes.length > 4 && bytes[0] === 66 && bytes[1] === 76 && bytes[2] === 80 && bytes[3] === 49;
}

function downloadEntryName(path: string): string {
  return path.replace(/__([0-9a-fA-F]{32})\.(mdx|mdl)$/i, '.$2').replaceAll('\\', '/');
}

function zipEntryNames(bytes: Uint8Array): string[] {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  let eocd = -1;
  for (let i = bytes.length - 22; i >= 0; i--) {
    if (view.getUint32(i, true) === 0x06054b50) {
      eocd = i;
      break;
    }
  }
  if (eocd < 0) throw new Error('zip has no central directory');
  const count = view.getUint16(eocd + 10, true);
  let offset = view.getUint32(eocd + 16, true);
  const names: string[] = [];
  for (let i = 0; i < count; i++) {
    if (offset + 46 > bytes.length || view.getUint32(offset, true) !== 0x02014b50) {
      throw new Error('zip central directory is truncated');
    }
    const nameLen = view.getUint16(offset + 28, true);
    const extraLen = view.getUint16(offset + 30, true);
    const commentLen = view.getUint16(offset + 32, true);
    const start = offset + 46;
    names.push(new TextDecoder().decode(bytes.subarray(start, start + nameLen)));
    offset = start + nameLen + extraLen + commentLen;
  }
  return names;
}

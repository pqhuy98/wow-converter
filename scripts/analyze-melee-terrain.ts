import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import {
  mkdirSync, readdirSync, readFileSync, writeFileSync,
} from 'node:fs';
import path from 'node:path';

import { parsers } from '@pqhuy98/mdx-m3-viewer';

type Counts = { [id: string]: number };
type SlkRow = { [field: string]: string };

// Read only the stable W3I prefix: old RoC/TFT maps have no build-version words.
function mapInfo(raw: Buffer) {
  const version = raw.readUInt32LE(0);
  assert(version >= 18, `Unsupported W3I version ${version}`);
  let at = version > 27 ? 28 : 12;
  const strings: string[] = [];
  for (let i = 0; i < 4; i++) {
    const end = raw.indexOf(0, at);
    assert(end >= at, 'Unterminated W3I string');
    strings.push(raw.toString('utf8', at, end));
    at = end + 1;
  }
  at += 32; // Eight camera-bound floats.
  const margins = Array.from({ length: 4 }, (_, i) => raw.readInt32LE(at + i * 4));
  at += 24; // Four complements and two playable dimensions.
  return {
    version, name: strings[0], melee: (raw.readUInt32LE(at) & 4) !== 0, margins,
  };
}

function terrainCounts(raw: Buffer, margins: number[]) {
  assert.equal(raw.toString('ascii', 0, 4), 'W3E!');
  const version = raw.readUInt32LE(4);
  assert(version === 11 || version === 12, `Unsupported W3E version ${version}`);
  const count = raw.readUInt32LE(13);
  assert(count > 0 && count <= (version === 12 ? 64 : 16), 'Invalid ground palette');
  let at = 17;
  const palette = Array.from({ length: count }, () => {
    const id = raw.toString('ascii', at, at + 4);
    at += 4;
    return id;
  });
  const cliffs = raw.readUInt32LE(at);
  at += 4 + cliffs * 4;
  const width = raw.readUInt32LE(at); const
    height = raw.readUInt32LE(at + 4);
  at += 16;
  const stride = version === 12 ? 8 : 7;
  assert(width > 1 && height > 1 && width <= 2049 && height <= 2049, 'Invalid terrain dimensions');
  assert.equal(raw.length, at + width * height * stride, 'W3E corner size/version mismatch');
  const [left, right, bottom, top] = margins;
  assert(margins.every((v) => Number.isInteger(v) && v >= 0) && left + right < width - 1 && bottom + top < height - 1, 'Invalid playable bounds');
  const painted: Counts = {}; const
    playable: Counts = {};
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const corner = at + (y * width + x) * stride;
      const index = version === 12 ? raw.readUInt16LE(corner + 4) & 63 : raw[corner + 4] & 15;
      const id = palette[index];
      assert(id, `Ground index ${index} outside palette`);
      painted[id] = (painted[id] ?? 0) + 1;
      if (x >= left && x <= width - 1 - right && y >= bottom && y <= height - 1 - top) {
        playable[id] = (playable[id] ?? 0) + 1;
      }
    }
  }
  return {
    version, width, height, palette, painted, playable,
  };
}

function slk(file: string): SlkRow[] {
  const rows = new Map<number, Map<number, string>>();
  let x = 0; let
    y = 0;
  for (const line of readFileSync(file, 'utf8').split(/\r?\n/)) {
    if (!line.startsWith('C;')) continue;
    for (const field of line.split(/;(?=(?:[^"]*"[^"]*")*[^"]*$)/).slice(1)) {
      if (field[0] === 'X') x = Number(field.slice(1));
      if (field[0] === 'Y') y = Number(field.slice(1));
      if (field[0] === 'K') {
        if (!rows.has(y)) rows.set(y, new Map());
        rows.get(y)!.set(x, field.slice(1).replace(/^"|"$/g, '').replace(/""/g, '"'));
      }
    }
  }
  const headers = rows.get(1)!;
  return [...rows].filter(([row]) => row > 1).map(([, cells]) => Object.fromEntries([...cells].map(([column, value]) => [headers.get(column)!, value])));
}

function archiveFiles(root: string): string[] {
  const result: string[] = [];
  for (const e of readdirSync(root, { withFileTypes: true })) {
    const file = path.join(root, e.name);
    if (e.isDirectory()) result.push(...archiveFiles(file));
    else if (/\.w3[xm]$/i.test(e.name)) result.push(file);
  }
  return result.sort();
}

function selfTest() {
  // The same paint pattern, with flag bits set, in both corner layouts.
  for (const version of [11, 12]) {
    const stride = version === 12 ? 8 : 7; const
      raw = Buffer.alloc(45 + 4 * stride);
    raw.write('W3E!'); raw.writeUInt32LE(version, 4); raw.write('L', 8);
    raw.writeUInt32LE(2, 13); raw.write('LdroLgrd', 17);
    raw.writeUInt32LE(2, 29); raw.writeUInt32LE(2, 33);
    for (let i = 0; i < 4; i++) {
      if (version === 12) raw.writeUInt16LE(0x140 | (i % 2), 45 + i * stride + 4);
      else raw[45 + i * stride + 4] = 0x50 | (i % 2);
    }
    assert.deepEqual(terrainCounts(raw, [0, 0, 0, 0]).painted, { Ldro: 2, Lgrd: 2 });
    assert.throws(() => terrainCounts(raw.subarray(0, -1), [0, 0, 0, 0]));
  }
  for (const version of [18, 25, 31]) {
    const at = version > 27 ? 28 : 12; const
      raw = Buffer.alloc(at + 4 + 60);
    raw.writeUInt32LE(version); raw.writeUInt32LE(4, at + 4 + 56);
    assert.equal(mapInfo(raw).melee, true);
  }
  console.log('Legacy/modern W3E and W3I prefix checks passed');
}

async function main() {
  if (process.argv[2] === '--self-test') { selfTest(); return; }
  const [root, output, allowAll = '40', nonBuildable = '8'] = process.argv.slice(2);
  const allowAllCount = Number(allowAll); const
    nonBuildableCount = Number(nonBuildable);
  assert(root && output, 'Usage: bun scripts/analyze-melee-terrain.ts <extracted-mpq> <output-dir> [unrestricted=40] [non-buildable=8]');
  assert([allowAllCount, nonBuildableCount].every((n) => Number.isInteger(n) && n >= 0) && allowAllCount + nonBuildableCount <= 63, 'Paint budget must leave a slot for native Maph');
  const terrain = slk(path.join(root, 'TerrainArt/Terrain.slk'));
  const cliffs = slk(path.join(root, 'TerrainArt/CliffTypes.slk'));
  const cliffGround = new Set(cliffs.flatMap((row) => [row.groundTile, row.upperTile]).filter((id) => id && id !== '_'));
  const files = archiveFiles(root); const
    seen = new Map<string, string>();
  const painted: Counts = {}; const playable: Counts = {}; const mapsPainted: Counts = {}; const
    paletteMaps: Counts = {};
  const maps: { path: string; name: string; terrainVersion: number; infoVersion: number; width: number; height: number; painted: Counts; playable: Counts }[] = [];
  const skipped: { path: string; reason: string }[] = []; const
    errors: { path: string; error: string }[] = [];
  for (const file of files) {
    const relative = path.relative(root, file).replaceAll('\\', '/');
    try {
      const archive = new parsers.mpq.Archive();
      // Bun.file returns an independent ArrayBuffer, avoiding pooled Buffer offsets.
      archive.load(await Bun.file(file).arrayBuffer(), true);
      const get = (name: string) => {
        const bytes = archive.get(name)?.arrayBuffer();
        assert(bytes, `Missing ${name}`);
        return Buffer.from(bytes);
      };
      const info = mapInfo(get('war3map.w3i'));
      if (!info.melee) { skipped.push({ path: relative, reason: 'W3I melee flag is off' }); continue; }
      const raw = get('war3map.w3e'); const
        t = terrainCounts(raw, info.margins);
      const hash = createHash('sha256').update(raw).digest('hex');
      if (seen.has(hash)) { skipped.push({ path: relative, reason: `Identical W3E: ${seen.get(hash)}` }); continue; }
      seen.set(hash, relative);
      // Stock IDs can be repainted by map imports; those maps cannot establish stock texture usage.
      const overrides = terrain.filter((row) => t.painted[row.tileID] && ['.blp', '.dds', '_diffuse.dds'].some((suffix) => archive.has(`${row.dir}\\${row.file}${suffix}`)));
      if (overrides.length) { skipped.push({ path: relative, reason: `Overrides stock terrain: ${overrides.map((row) => row.tileID).join(',')}` }); continue; }
      for (const id of new Set(t.palette)) paletteMaps[id] = (paletteMaps[id] ?? 0) + 1;
      for (const [id, count] of Object.entries(t.painted)) { painted[id] = (painted[id] ?? 0) + count; mapsPainted[id] = (mapsPainted[id] ?? 0) + 1; }
      for (const [id, count] of Object.entries(t.playable)) playable[id] = (playable[id] ?? 0) + count;
      maps.push({
        path: relative, name: info.name, terrainVersion: t.version, infoVersion: info.version, width: t.width, height: t.height, painted: t.painted, playable: t.playable,
      });
    } catch (error) { errors.push({ path: relative, error: String(error) }); }
  }
  assert(maps.length > 0, 'No readable melee maps found');
  const ranked = terrain.map((row) => {
    const excluded: string[] = [];
    if (row.cliffSet !== '-1') excluded.push('cliffSet');
    if (cliffGround.has(row.tileID)) excluded.push('cliff ground/upper tile');
    for (const property of ['walkable', 'flyable']) if (row[property] !== '1') excluded.push(`${property}=0`);
    return {
      id: row.tileID, dir: row.dir, file: row.file, name: row.comment, buildable: row.buildable === '1', walkable: row.walkable === '1', flyable: row.flyable === '1', cliff: row.cliffSet !== '-1' || cliffGround.has(row.tileID), paintedCorners: painted[row.tileID] ?? 0, playablePaintedCorners: playable[row.tileID] ?? 0, mapsPainted: mapsPainted[row.tileID] ?? 0, paletteMaps: paletteMaps[row.tileID] ?? 0, excluded,
    };
  }).sort((a, b) => a.playablePaintedCorners - b.playablePaintedCorners || a.paintedCorners - b.paintedCorners || a.id.localeCompare(b.id));
  const eligible = ranked.filter((row) => !row.excluded.length);
  const unrestricted = eligible.filter((row) => row.buildable); const
    unbuildable = eligible.filter((row) => !row.buildable);
  assert(allowAllCount <= unrestricted.length && nonBuildableCount <= unbuildable.length, 'Not enough eligible stock tiles');
  const selected = [...unrestricted.slice(0, allowAllCount), ...unbuildable.slice(0, nonBuildableCount)];
  const report = {
    paintSlots: selected.length, nativeAlphaId: 'Maph', unrestrictedCount: unrestricted.length, nonBuildableCount: unbuildable.length, archivesFound: files.length, meleeTerrainsCounted: maps.length, skipped, errors, terrainVersions: [...new Set(maps.map((m) => m.terrainVersion))].sort(), infoVersions: [...new Set(maps.map((m) => m.infoVersion))].sort(), method: 'Count actual W3E ground-paint assignments per corner. Rank by playable-region count, then full-map count, then ID. Palette-only presence contributes zero. Identical W3E terrain is counted once; W3I must mark the map melee. Exclude imported terrain overrides. Counts measure paint assignments, not rendered alpha-weighted pixel area.', selected, ranked, maps,
  };
  mkdirSync(output, { recursive: true });
  writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
  const fields = ['id', 'name', 'paintedCorners', 'playablePaintedCorners', 'mapsPainted', 'paletteMaps', 'buildable', 'walkable', 'flyable', 'cliff', 'excluded'];
  const csv = [fields, ...ranked.map((row) => fields.map((key) => String(Reflect.get(row, key))))].map((row) => row.map((value) => `"${value.replaceAll('"', '""')}"`).join(',')).join('\n');
  writeFileSync(path.join(output, 'tiles.csv'), `${csv}\n`);
  writeFileSync(path.join(output, 'go-slots.txt'), `${selected.map((row) => `\t{"${row.id}", \`${row.dir}\`, "${row.file}", ${row.buildable}}, // ${row.playablePaintedCorners} playable paint corners`).join('\n')}\n`);
  console.log(JSON.stringify({
    archivesFound: files.length, meleeTerrainsCounted: maps.length, skipped: skipped.length, errors, terrainVersions: report.terrainVersions, infoVersions: report.infoVersions, unrestrictedTiles: unrestricted.length, nonBuildableTiles: unbuildable.length, selectedPaintTiles: selected.length, neverPainted: selected.filter((row) => !row.paintedCorners).length,
  }, null, 2));
}

await main();

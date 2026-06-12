/**
 * Second-pass import fixes after reorganize-lib.ts moves.
 * Run: bun scripts/fix-imports.ts
 */
import { readFileSync, readdirSync, rmSync, statSync, writeFileSync, existsSync } from 'fs';
import path from 'path';

const ROOT = path.resolve(import.meta.dir, '..');

const DUPLICATE_DIRS = [
  'src/lib/wow/casc',
  'src/lib/wow/m2',
  'src/lib/wow/map',
  'src/lib/wow-raw',
  'src/lib/objmdl',
  'src/lib/m2mdl',
];

const DUPLICATE_FILES = [
  'src/lib/wow/buffer.ts',
  'src/lib/wow/constants.ts',
  'src/lib/wow/crc32.ts',
  'src/lib/wow/generics.ts',
  'src/lib/wow/png-writer.ts',
  'src/lib/wow/config.ts',
  'src/lib/wow/runtime.ts',
  'src/lib/wow/memory-diagnostics.ts',
  'src/lib/wow/export/adt-exporter.ts',
  'src/lib/wow/export/m2-exporter.ts',
  'src/lib/wow/export/model-export-service.ts',
  'src/lib/wow/export/geoset-mapper.ts',
  'src/lib/wow/export/wmo-exporter.ts',
  'src/lib/wow/export/export-helper.ts',
  'src/lib/wow/export/csv-writer.ts',
  'src/lib/wow/export/json-writer.ts',
  'src/lib/wow/export/mtl-writer.ts',
  'src/lib/wow/export/obj-writer.ts',
  'src/lib/wow/export/output-sink.ts',
];

/** Global replacements (longest first). */
const GLOBAL: [string, string][] = [
  ['@/lib/wow/archive/casc/blp', '@/lib/wow/formats/blp/blp'],
  ['@/lib/wow/export/m2/export-helper', '@/lib/wow/export/writers/export-helper'],
  ['../wow/casc/casc-source', '@/lib/wow/archive/casc/casc-source'],
  ['../wow/map/wmo-loader', '@/lib/wow/formats/wmo/wmo-loader'],
  ['../../archive/casc/blp', '../../formats/blp/blp'],
  ['../../generics', '../../formats/generics'],
  ["from '../writers/output-sink'", "from './output-sink'"],
  ["from './geometry'", "from '@/lib/converter/wow-model/direct/m2/geometry'"],
  ["from './json-normalize'", "from '@/lib/converter/wow-model/direct/m2/json-normalize'"],
  ["'../../wow/casc/blp'", "'@/lib/wow/formats/blp/blp'"],
  ["'../../wow/buffer'", "'@/lib/wow/formats/buffer'"],
  ["'../src/lib/wow/casc/blp'", "'@/lib/wow/formats/blp/blp'"],
  ["'../src/lib/wow/buffer'", "'@/lib/wow/formats/buffer'"],
  ['../../src/lib/wow/archive/casc/blp', '@/lib/wow/formats/blp/blp'],
  ['../lib/wow-raw/raw-cache', '@/lib/wow/archive/client/raw-cache'],
];

/** Per-file replacements keyed by path substring. */
const BY_PATH: [string, [string, string][]][] = [
  ['mapmodifier/', [
    ["from '../../formats/constants'", "from '@/lib/constants'"],
  ]],
  ['wow/db/', [
    ["from '../../formats/constants'", "from '../formats/constants'"],
    ["from '../../formats/buffer'", "from '../formats/buffer'"],
    ["from '../../formats/generics'", "from '../formats/generics'"],
    ["from '../writers/export-helper'", "from '@/lib/wow/export/writers/export-helper'"],
  ]],
  ['converter/icon/', [
    ["from '@/lib/wow/formats/constants'", "from '@/lib/converter/icon/constants'"],
  ]],
  ['converter/wow-model/bundle/animation.ts', [
    ["from '@/lib/wow/formats/constants'", "from '@/lib/constants'"],
  ]],
  ['converter/wow-model/bundle/metadata.ts', [
    ["from '@/lib/wow/formats/constants'", "from '@/lib/constants'"],
  ]],
  ['formats/mdl/components/camera.ts', [
    ["from '@/lib/converter/wow-model/bundle/animation'", "from './animation'"],
  ]],
  ['formats/mdl/components/texture-anim.ts', [
    ["from '@/lib/converter/wow-model/bundle/animation'", "from './animation'"],
  ]],
  ['formats/mdl/components/geoset.ts', [
    ["from '@/lib/converter/wow-model/bundle/animation'", "from './animation'"],
  ]],
  ['formats/mdl/components/material.ts', [
    ["from '@/lib/converter/wow-model/bundle/animation'", "from './animation'"],
  ]],
  ['wowexport-client/wowexport-client.ts', [
    ["from '@/lib/converter/wow-model/bundle/utils'", "from '@/lib/utils'"],
  ]],
  ['mapmodifier/terrain.ts', [
    ["from '@/lib/converter/wow-model/bundle/utils'", "from '@/lib/utils'"],
  ]],
  ['mapmodifier/terrain-height-matcher.ts', [
    ["from '@/lib/converter/wow-model/bundle/utils'", "from '@/lib/utils'"],
  ]],
  ['azerothcore-client/creatures.ts', [
    ["from '@/lib/converter/wow-model/bundle/utils'", "from '@/lib/utils'"],
  ]],
  ['wowhead-exporter/character-direct.ts', [
    ["from '@/lib/converter/wow-model/bundle/utils'", "from '@/lib/converter/character/utils'"],
  ]],
  ['wowhead-exporter/character-model.ts', [
    ["from '@/lib/converter/wow-model/bundle/utils'", "from '@/lib/converter/character/utils'"],
  ]],
  ['wowhead-exporter/creature-model.ts', [
    ["from '@/lib/converter/wow-model/bundle/utils'", "from '@/lib/converter/character/utils'"],
  ]],
  ['wowhead-exporter/item-model.ts', [
    ["from '@/lib/converter/wow-model/bundle/utils'", "from '@/lib/converter/character/utils'"],
  ]],
];

function walkTs(dir: string, out: string[] = []): string[] {
  if (!existsSync(dir)) return out;
  for (const name of readdirSync(dir)) {
    const p = path.join(dir, name);
    if (statSync(p).isDirectory()) {
      if (name === 'node_modules' || name === '.next' || name === 'webui') continue;
      walkTs(p, out);
    } else if (name.endsWith('.ts')) {
      out.push(p);
    }
  }
  return out;
}

function applyReplacements(text: string, reps: [string, string][]): string {
  const sorted = [...reps].sort((a, b) => b[0].length - a[0].length);
  let out = text;
  for (const [from, to] of sorted) {
    if (out.includes(from)) out = out.split(from).join(to);
  }
  return out;
}

function rewriteFile(filePath: string) {
  const rel = path.relative(ROOT, filePath).replace(/\\/g, '/');
  let text = readFileSync(filePath, 'utf-8');
  const original = text;
  text = applyReplacements(text, GLOBAL);
  for (const [pattern, reps] of BY_PATH) {
    if (rel.includes(pattern)) text = applyReplacements(text, reps);
  }
  if (text !== original) writeFileSync(filePath, text);
}

console.log('=== removing duplicate old paths ===');
for (const d of DUPLICATE_DIRS) {
  const full = path.join(ROOT, d);
  if (existsSync(full)) {
    rmSync(full, { recursive: true, force: true });
    console.log('removed', d);
  }
}
for (const f of DUPLICATE_FILES) {
  const full = path.join(ROOT, f);
  if (existsSync(full)) {
    rmSync(full, { force: true });
    console.log('removed', f);
  }
}

console.log('=== rewriting imports ===');
const files = walkTs(path.join(ROOT, 'src'))
  .concat(walkTs(path.join(ROOT, 'tests')))
  .concat(walkTs(path.join(ROOT, 'examples')));
for (const file of files) rewriteFile(file);

console.log('done');

/**
 * One-shot library reorganization: moves files and rewrites import paths.
 * Run: bun scripts/reorganize-lib.ts
 */
import { existsSync, mkdirSync, readdirSync, readFileSync, renameSync, rmSync, statSync, writeFileSync } from 'fs';
import path from 'path';

const ROOT = path.resolve(import.meta.dir, '..');
const SRC = path.join(ROOT, 'src');

function ensureDir(filePath: string) {
  mkdirSync(path.dirname(filePath), { recursive: true });
}

function move(fromRel: string, toRel: string) {
  const from = path.join(ROOT, fromRel);
  const to = path.join(ROOT, toRel);
  if (!existsSync(from)) {
    console.warn('skip missing', fromRel);
    return;
  }
  ensureDir(to);
  const stat = statSync(from);
  if (stat.isDirectory()) {
    for (const name of readdirSync(from)) {
      move(path.join(fromRel, name).replace(/\\/g, '/'), path.join(toRel, name).replace(/\\/g, '/'));
    }
    try { rmSync(from, { recursive: true }); } catch { /* may be locked */ }
  } else {
    try {
      renameSync(from, to);
    } catch {
      // Fallback: copy + delete when rename fails (Windows file locks).
      writeFileSync(to, readFileSync(from));
      try { rmSync(from); } catch { /* ignore */ }
    }
  }
  console.log('mv', fromRel, '->', toRel);
}

function removeEmptyDirs(dirRel: string) {
  const dir = path.join(ROOT, dirRel);
  if (!existsSync(dir)) return;
  for (const name of readdirSync(dir)) {
    const p = path.join(dir, name);
    if (statSync(p).isDirectory()) removeEmptyDirs(path.relative(ROOT, p));
  }
  if (readdirSync(dir).length === 0) rmSync(dir, { recursive: true });
}

/** Longest-match-first import path replacements. */
const REPLACEMENTS: [string, string][] = [
  // wow-raw → archive/client
  ['@/lib/wow-raw/raw-client', '@/lib/wow/archive/client/raw-client'],
  ['@/lib/wow-raw/raw-cache', '@/lib/wow/archive/client/raw-cache'],
  ['@/lib/wow-raw/name-client', '@/lib/wow/archive/client/name-client'],
  ['@/lib/wow-raw/remote-casc', '@/lib/wow/archive/client/remote-casc'],
  ['@/lib/wow-raw/', '@/lib/wow/archive/client/'],

  // export types (before m2-exporter)
  ['@/lib/wow/export/m2-exporter', '@/lib/wow/export/m2/m2-exporter'],
  ['@/lib/wow/export/types/model-export', '@/lib/wow/export/types/model-export'],

  // export paths
  ['@/lib/wow/export/adt-exporter', '@/lib/wow/export/adt/adt-exporter'],
  ['@/lib/wow/export/model-export-service', '@/lib/wow/export/m2/model-export-service'],
  ['@/lib/wow/export/geoset-mapper', '@/lib/wow/export/m2/geoset-mapper'],
  ['@/lib/wow/export/wmo-exporter', '@/lib/wow/export/wmo/wmo-exporter'],
  ['@/lib/wow/export/export-helper', '@/lib/wow/export/writers/export-helper'],
  ['@/lib/wow/export/csv-writer', '@/lib/wow/export/writers/csv-writer'],
  ['@/lib/wow/export/json-writer', '@/lib/wow/export/writers/json-writer'],
  ['@/lib/wow/export/mtl-writer', '@/lib/wow/export/writers/mtl-writer'],
  ['@/lib/wow/export/obj-writer', '@/lib/wow/export/writers/obj-writer'],
  ['@/lib/wow/export/output-sink', '@/lib/wow/export/writers/output-sink'],

  // map → formats/export
  ['@/lib/wow/map/terrain-baker', '@/lib/wow/export/adt/terrain-baker'],
  ['@/lib/wow/map/map-export-utils', '@/lib/wow/export/adt/map-export-utils'],
  ['@/lib/wow/map/wmo-loader', '@/lib/wow/formats/wmo/wmo-loader'],
  ['@/lib/wow/map/adt-loader', '@/lib/wow/formats/adt/adt-loader'],
  ['@/lib/wow/map/wdt-loader', '@/lib/wow/formats/adt/wdt-loader'],
  ['@/lib/wow/map/loader-generics', '@/lib/wow/formats/adt/loader-generics'],

  // casc → archive/casc
  ['@/lib/wow/casc/', '@/lib/wow/archive/casc/'],
  ['@/lib/wow/casc/blp', '@/lib/wow/formats/blp/blp'],

  // formats
  ['@/lib/wow/m2/', '@/lib/wow/formats/m2/'],
  ['@/lib/wow/buffer', '@/lib/wow/formats/buffer'],
  ['@/lib/wow/constants', '@/lib/wow/formats/constants'],
  ['@/lib/wow/crc32', '@/lib/wow/formats/crc32'],
  ['@/lib/wow/generics', '@/lib/wow/formats/generics'],
  ['@/lib/wow/png-writer', '@/lib/wow/formats/png-writer'],

  // server
  ['@/lib/wow/memory-diagnostics', '@/lib/wow/server/memory-diagnostics'],
  ['@/lib/wow/runtime', '@/lib/wow/server/runtime'],
  ['@/lib/wow/config', '@/lib/wow/server/config'],

  // converter wow-model (objmdl/m2mdl)
  ['@/lib/objmdl/animation/animation_mapper', '@/lib/converter/wow-model/animation/animation-mapper'],
  ['@/lib/objmdl/animation/bones_mapper', '@/lib/converter/wow-model/animation/bones-mapper'],
  ['@/lib/objmdl/metadata/m2_metadata', '@/lib/converter/wow-model/bundle/metadata'],
  ['@/lib/objmdl/animation/animation', '@/lib/converter/wow-model/bundle/animation'],
  ['@/lib/objmdl/obj', '@/lib/converter/wow-model/bundle/obj'],
  ['@/lib/objmdl/mtl', '@/lib/converter/wow-model/bundle/mtl'],
  ['@/lib/m2mdl/textures', '@/lib/converter/wow-model/direct/m2/textures'],
  ['@/lib/m2mdl/', '@/lib/converter/wow-model/direct/m2/'],
  ['@/lib/m2mdl', '@/lib/converter/wow-model/direct/m2'],
  ['@/lib/objmdl', '@/lib/converter/wow-model/legacy'],
  ['../../m2mdl', '../../converter/wow-model/direct/m2'],
  ['../../objmdl', '../../converter/wow-model/legacy'],
  ['../m2mdl', '../converter/wow-model/direct/m2'],
  ['../objmdl', '../converter/wow-model/legacy'],

  // relative paths used in tests
  ['../../src/lib/wow/casc/', '../../src/lib/wow/archive/casc/'],
  ['../../src/lib/wow/m2/', '../../src/lib/wow/formats/m2/'],
  ['../../src/lib/wow/export/', '../../src/lib/wow/export/m2/'],
  ['../../src/lib/wow/config', '../../src/lib/wow/server/config'],
  ['../../src/lib/wow/runtime', '../../src/lib/wow/server/runtime'],
  ['../../src/lib/wow/buffer', '../../src/lib/wow/formats/buffer'],
  ['../../src/lib/wow/constants', '../../src/lib/wow/formats/constants'],
  ['../../src/lib/wow/map/', '../../src/lib/wow/formats/adt/'],
  ['../../src/lib/wow/export/adt-exporter', '../../src/lib/wow/export/adt/adt-exporter'],
  ['../../src/lib/wow/export/m2-exporter', '../../src/lib/wow/export/m2/m2-exporter'],
  ['../../src/lib/wow/export/export-helper', '../../src/lib/wow/export/writers/export-helper'],
  ['../../src/lib/wow/export/model-export-service', '../../src/lib/wow/export/m2/model-export-service'],
  ['../../src/lib/wow/export/wmo-exporter', '../../src/lib/wow/export/wmo/wmo-exporter'],
  ['../../src/lib/wow/export/output-sink', '../../src/lib/wow/export/writers/output-sink'],
  ['../../src/lib/wow/export/obj-writer', '../../src/lib/wow/export/writers/obj-writer'],
  ['../../src/lib/wow/export/mtl-writer', '../../src/lib/wow/export/writers/mtl-writer'],
  ['../../src/lib/wow/export/json-writer', '../../src/lib/wow/export/writers/json-writer'],
  ['../../src/lib/wow/export/csv-writer', '../../src/lib/wow/export/writers/csv-writer'],
  ['../../src/lib/wow/map/wmo-loader', '../../src/lib/wow/formats/wmo/wmo-loader'],
  ['../../src/lib/wow/map/terrain-baker', '../../src/lib/wow/export/adt/terrain-baker'],
  ['../../src/lib/wow/map/map-export-utils', '../../src/lib/wow/export/adt/map-export-utils'],
  ['../../src/lib/wow/casc/blp', '../../src/lib/wow/formats/blp/blp'],
  ['../../src/lib/wow/character/', '../../src/lib/wow/character/'],
  ['../../src/lib/wow/db/', '../../src/lib/wow/db/'],
  ['../../src/lib/wow/log', '../../src/lib/wow/log'],
  ['../../src/lib/wow-raw/', '../../src/lib/wow/archive/client/'],
];

/** Relative import fixes inside moved wow/ files. */
const RELATIVE_REPLACEMENTS: [RegExp, string][] = [
  [/(from ['"])\.\.\/casc\//g, '$1../../archive/casc/'],
  [/(from ['"])\.\.\/\.\.\/casc\//g, '$1../../../archive/casc/'],
  [/(from ['"])\.\.\/m2\//g, '$1../../formats/m2/'],
  [/(from ['"])\.\.\/\.\.\/m2\//g, '$1../../../formats/m2/'],
  [/(from ['"])\.\.\/map\/wmo-loader/g, '$1../../formats/wmo/wmo-loader'],
  [/(from ['"])\.\.\/map\/adt-loader/g, '$1../../formats/adt/adt-loader'],
  [/(from ['"])\.\.\/map\/wdt-loader/g, '$1../../formats/adt/wdt-loader'],
  [/(from ['"])\.\.\/map\/loader-generics/g, '$1../../formats/adt/loader-generics'],
  [/(from ['"])\.\.\/map\/terrain-baker/g, '$1../adt/terrain-baker'],
  [/(from ['"])\.\.\/map\/map-export-utils/g, '$1../adt/map-export-utils'],
  [/(from ['"])\.\.\/buffer/g, '$1../../formats/buffer'],
  [/(from ['"])\.\.\/\.\.\/buffer/g, '$1../../../formats/buffer'],
  [/(from ['"])\.\.\/constants/g, '$1../../formats/constants'],
  [/(from ['"])\.\.\/\.\.\/constants/g, '$1../../../formats/constants'],
  [/(from ['"])\.\.\/crc32/g, '$1../../formats/crc32'],
  [/(from ['"])\.\.\/generics/g, '$1../../formats/generics'],
  [/(from ['"])\.\.\/png-writer/g, '$1../../formats/png-writer'],
  [/(from ['"])\.\.\/config/g, '$1../../server/config'],
  [/(from ['"])\.\.\/\.\.\/config/g, '$1../../../server/config'],
  [/(from ['"])\.\.\/runtime/g, '$1../../server/runtime'],
  [/(from ['"])\.\.\/\.\.\/runtime/g, '$1../../../server/runtime'],
  [/(from ['"])\.\.\/log/g, '$1../../log'],
  [/(from ['"])\.\.\/\.\.\/log/g, '$1../../../log'],
  [/(from ['"])\.\.\/casc\/blp/g, '$1../../formats/blp/blp'],
  [/(from ['"])\.\.\/\.\.\/casc\/blp/g, '$1../../../formats/blp/blp'],
  [/(from ['"])\.\.\/export\/export-helper/g, '$1../writers/export-helper'],
  [/(from ['"])\.\.\/export-helper/g, '$1../writers/export-helper'],
  [/(from ['"])\.\/geoset-mapper/g, '$1./geoset-mapper'],
  [/(from ['"])\.\/json-writer/g, '$1../writers/json-writer'],
  [/(from ['"])\.\/mtl-writer/g, '$1../writers/mtl-writer'],
  [/(from ['"])\.\/obj-writer/g, '$1../writers/obj-writer'],
  [/(from ['"])\.\/output-sink/g, '$1../writers/output-sink'],
  [/(from ['"])\.\/csv-writer/g, '$1../writers/csv-writer'],
  [/(from ['"])\.\.\/export\/m2-exporter/g, '$1../m2/m2-exporter'],
  [/(from ['"])\.\.\/export\/wmo-exporter/g, '$1../wmo/wmo-exporter'],
  [/(from ['"])\.\.\/export\/geoset-mapper/g, '$1../m2/geoset-mapper'],
  [/(from ['"])\.\.\/wow\/export\//g, '$1../../wow/export/m2/'],
  [/(from ['"])\.\.\/wow\/m2\//g, '$1../../wow/formats/m2/'],
  [/(from ['"])\.\.\/wow\/buffer/g, '$1../../wow/formats/buffer'],
  [/(from ['"])\.\.\/wow\/constants/g, '$1../../wow/formats/constants'],
  [/(from ['"])\.\.\/wow\/archive\/casc\//g, '$1../../wow/archive/casc/'],
  [/(from ['"])\.\.\/wow-raw\//g, '$1../../wow/archive/client/'],
  [/(from ['"])\.\.\/objmdl/g, '$1../wow-model/bundle'],
  [/(from ['"])\.\.\/\.\.\/objmdl/g, '$1../../wow-model/bundle'],
  [/(from ['"])\.\.\/m2mdl/g, '$1./direct/m2'],
  [/(from ['"])\.\.\/\.\.\/m2mdl/g, '$1../direct/m2'],
  [/(from ['"])\.\.\/objmdl\/index/g, '$1../assemble'],
  [/(from ['"])\.\.\/objmdl['"]/g, '$1../assemble\''],
  [/(from ['"])\.\.\/\.\.\/objmdl['"]/g, '$1../../assemble\''],
  [/(from ['"])\.\.\/\.\.\/export-profile/g, '$1../../../export-profile'],
  [/(from ['"])\.\.\/\.\.\/\.\.\/export-profile/g, '$1../../../../export-profile'],
  [/(from ['"])\.\.\/\.\.\/global-config/g, '$1../../../global-config'],
  [/(from ['"])\.\.\/global-config/g, '$1../../global-config'],
  [/(from ['"])\.\.\/\.\.\/export-asset-store/g, '$1../../../export-asset-store'],
  [/(from ['"])\.\.\/\.\.\/formats\/mdl/g, '$1../../../formats/mdl'],
];

function walkTs(dir: string, out: string[] = []): string[] {
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

function rewriteFile(filePath: string) {
  let text = readFileSync(filePath, 'utf-8');
  let changed = false;
  for (const [from, to] of REPLACEMENTS) {
    if (text.includes(from)) {
      text = text.split(from).join(to);
      changed = true;
    }
  }
  for (const [re, to] of RELATIVE_REPLACEMENTS) {
    if (re.test(text)) {
      text = text.replace(re, to);
      changed = true;
    }
  }
  if (changed) writeFileSync(filePath, text);
}

// --- moves ---
const moves: [string, string][] = [
  // archive
  ['src/lib/wow/casc', 'src/lib/wow/archive/casc'],
  ['src/lib/wow-raw/name-client.ts', 'src/lib/wow/archive/client/name-client.ts'],
  ['src/lib/wow-raw/raw-cache.ts', 'src/lib/wow/archive/client/raw-cache.ts'],
  ['src/lib/wow-raw/raw-client.ts', 'src/lib/wow/archive/client/raw-client.ts'],
  ['src/lib/wow-raw/remote-casc.ts', 'src/lib/wow/archive/client/remote-casc.ts'],

  // formats (after casc move, blp extracted)
  ['src/lib/wow/buffer.ts', 'src/lib/wow/formats/buffer.ts'],
  ['src/lib/wow/constants.ts', 'src/lib/wow/formats/constants.ts'],
  ['src/lib/wow/crc32.ts', 'src/lib/wow/formats/crc32.ts'],
  ['src/lib/wow/generics.ts', 'src/lib/wow/formats/generics.ts'],
  ['src/lib/wow/png-writer.ts', 'src/lib/wow/formats/png-writer.ts'],
  ['src/lib/wow/m2', 'src/lib/wow/formats/m2'],
  ['src/lib/wow/archive/casc/blp.ts', 'src/lib/wow/formats/blp/blp.ts'],
  ['src/lib/wow/map/wmo-loader.ts', 'src/lib/wow/formats/wmo/wmo-loader.ts'],
  ['src/lib/wow/map/adt-loader.ts', 'src/lib/wow/formats/adt/adt-loader.ts'],
  ['src/lib/wow/map/wdt-loader.ts', 'src/lib/wow/formats/adt/wdt-loader.ts'],
  ['src/lib/wow/map/loader-generics.ts', 'src/lib/wow/formats/adt/loader-generics.ts'],

  // export
  ['src/lib/wow/export/adt-exporter.ts', 'src/lib/wow/export/adt/adt-exporter.ts'],
  ['src/lib/wow/map/terrain-baker.ts', 'src/lib/wow/export/adt/terrain-baker.ts'],
  ['src/lib/wow/map/map-export-utils.ts', 'src/lib/wow/export/adt/map-export-utils.ts'],
  ['src/lib/wow/export/m2-exporter.ts', 'src/lib/wow/export/m2/m2-exporter.ts'],
  ['src/lib/wow/export/model-export-service.ts', 'src/lib/wow/export/m2/model-export-service.ts'],
  ['src/lib/wow/export/geoset-mapper.ts', 'src/lib/wow/export/m2/geoset-mapper.ts'],
  ['src/lib/wow/export/wmo-exporter.ts', 'src/lib/wow/export/wmo/wmo-exporter.ts'],
  ['src/lib/wow/export/csv-writer.ts', 'src/lib/wow/export/writers/csv-writer.ts'],
  ['src/lib/wow/export/export-helper.ts', 'src/lib/wow/export/writers/export-helper.ts'],
  ['src/lib/wow/export/json-writer.ts', 'src/lib/wow/export/writers/json-writer.ts'],
  ['src/lib/wow/export/mtl-writer.ts', 'src/lib/wow/export/writers/mtl-writer.ts'],
  ['src/lib/wow/export/obj-writer.ts', 'src/lib/wow/export/writers/obj-writer.ts'],
  ['src/lib/wow/export/output-sink.ts', 'src/lib/wow/export/writers/output-sink.ts'],

  // server
  ['src/lib/wow/config.ts', 'src/lib/wow/server/config.ts'],
  ['src/lib/wow/runtime.ts', 'src/lib/wow/server/runtime.ts'],
  ['src/lib/wow/memory-diagnostics.ts', 'src/lib/wow/server/memory-diagnostics.ts'],

  // converter wow-model bundle
  ['src/lib/objmdl/obj.ts', 'src/lib/converter/wow-model/bundle/obj.ts'],
  ['src/lib/objmdl/mtl.ts', 'src/lib/converter/wow-model/bundle/mtl.ts'],
  ['src/lib/objmdl/utils.ts', 'src/lib/converter/wow-model/bundle/utils.ts'],
  ['src/lib/objmdl/animation/animation.ts', 'src/lib/converter/wow-model/bundle/animation.ts'],
  ['src/lib/objmdl/metadata/m2_metadata.ts', 'src/lib/converter/wow-model/bundle/metadata.ts'],
  ['src/lib/objmdl/animation/animation_mapper.ts', 'src/lib/converter/wow-model/animation/animation-mapper.ts'],
  ['src/lib/objmdl/animation/bones_mapper.ts', 'src/lib/converter/wow-model/animation/bones-mapper.ts'],

  // converter wow-model direct
  ['src/lib/m2mdl/index.ts', 'src/lib/converter/wow-model/direct/m2/index.ts'],
  ['src/lib/m2mdl/geometry.ts', 'src/lib/converter/wow-model/direct/m2/geometry.ts'],
  ['src/lib/m2mdl/bones.ts', 'src/lib/converter/wow-model/direct/m2/bones.ts'],
  ['src/lib/m2mdl/textures.ts', 'src/lib/converter/wow-model/direct/m2/textures.ts'],
  ['src/lib/m2mdl/metadata.ts', 'src/lib/converter/wow-model/direct/m2/metadata.ts'],
  ['src/lib/m2mdl/json-normalize.ts', 'src/lib/converter/wow-model/direct/m2/json-normalize.ts'],
  ['src/lib/m2mdl/wmo.ts', 'src/lib/converter/wow-model/direct/wmo/index.ts'],
];

console.log('=== moving files ===');
for (const [from, to] of moves) move(from, to);

console.log('=== rewriting imports ===');
for (const file of walkTs(SRC).concat(walkTs(path.join(ROOT, 'tests')))) {
  rewriteFile(file);
}
for (const file of walkTs(path.join(ROOT, 'examples'))) rewriteFile(file);

console.log('=== cleanup empty dirs ===');
if (existsSync(path.join(ROOT, 'src/lib/objmdl/index.ts'))) {
  rmSync(path.join(ROOT, 'src/lib/objmdl/index.ts'));
  console.log('removed src/lib/objmdl/index.ts');
}
for (const d of ['src/lib/wow-raw', 'src/lib/wow/map', 'src/lib/wow/m2', 'src/lib/wow/casc', 'src/lib/m2mdl', 'src/lib/objmdl']) {
  removeEmptyDirs(d);
}

console.log('done — run tsc and fix assemble/legacy split manually if needed');

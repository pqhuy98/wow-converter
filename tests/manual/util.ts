import { parsers } from '@pqhuy98/mdx-m3-viewer';
import {
  copyFileSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, unlinkSync,
} from 'fs';
import path, { join } from 'path';

import { ModificationType } from '@/vendors/wc3maptranslator/data';
import { MapManager } from '@/vendors/wc3maptranslator/extra/map-manager';

import { converterUrl, exportCharacter, exportedAssetsDir, repoRoot } from '../helpers';
import { type SnapshotCase, type SnapshotSuite } from '../snapshot-tests/model/catalog';

const distancePerTile = 128;

interface PlacedModel {
  readonly name: string;
  readonly model: string;
}

export function ensureRegressionMap(name: SnapshotSuite): string {
  const mapDir = path.join(repoRoot, 'maps', `test-regression-${name}.w3x`);
  if (!existsSync(join(mapDir, 'war3map.w3i'))) {
    throw new Error(`missing map shell ${join(mapDir, 'war3map.w3i')}`);
  }
  return mapDir;
}

export function limitedCases<T>(cases: readonly T[]): readonly T[] {
  const limit = Number(process.env.TEST_LIMIT || 0);
  return limit > 0 ? cases.slice(0, limit) : cases;
}

export function isFresh(): boolean {
  return process.env.fresh === '1';
}

/** Copies the suite models and the textures they reference into the map. */
export async function copyRegressionModels(
  mapDir: string,
  suite: SnapshotSuite,
  cases: readonly SnapshotCase[],
): Promise<PlacedModel[]> {
  if (suite === 'mount') {
    for (const testCase of cases) {
      if (!testCase.mount) throw new Error(`${testCase.slug} has no mount`);
    }
  }
  await exportCases(mapDir, suite, cases);
  return cases.map((testCase) => ({
    name: testCase.slug,
    model: unitModel(suite, testCase.slug),
  }));
}

export function placeCharacters(mapDir: string, placed: readonly PlacedModel[]): void {
  const map = new MapManager();
  map.load(mapDir);
  map.units = map.units.filter((unit) => typeof unit.type === 'string');
  map.unitTypes = [];

  for (const [i, npc] of placed.entries()) {
    const unitType = map.addUnitType('hero', 'Hpal', [
      { id: 'unam', type: ModificationType.string, value: npc.name },
      { id: 'upro', type: ModificationType.string, value: npc.name },
      { id: 'umdl', type: ModificationType.string, value: `${npc.model}.mdx` },
      { id: 'usca', type: ModificationType.real, value: 1 },
      { id: 'ussc', type: ModificationType.real, value: 2 },
      { id: 'ua1b', type: ModificationType.int, value: 500 },
      { id: 'uabi', type: ModificationType.string, value: 'A003,A001,A002,A000' },
      { id: 'udtm', type: ModificationType.unreal, value: 6 },
    ]);
    map.addUnit(unitType, unitAt(map, i * 500, unitType.code));
    console.log(npc.name, 'placed');
  }
  saveRegressionMap(map, mapDir);
}

export function placeMounts(mapDir: string, placed: readonly PlacedModel[]): void {
  const map = new MapManager();
  map.load(mapDir);
  map.units = map.units.filter((unit) => typeof unit.type === 'string');
  map.unitTypes = [];
  map.abilities = [];

  let offset = 0;
  for (const npc of placed) {
    const grounded = map.addUnitType('hero', 'Hpal', heroFields(npc.name, npc.model));
    const flying = map.addUnitType('hero', 'Hpal', [
      ...heroFields(`${npc.name} Alt`, npc.model),
      { id: 'uani', type: ModificationType.string, value: 'alternate' },
      { id: 'umvh', type: ModificationType.unreal, value: 300 },
      { id: 'umvt', type: ModificationType.string, value: 'fly' },
    ]);
    const ability = map.addAbility('Arav', [
      {
        id: 'Emeu', type: ModificationType.string, value: flying.code, level: 1, column: 0,
      },
      {
        id: 'Eme1', type: ModificationType.string, value: grounded.code, level: 1, column: 1,
      },
      {
        id: 'Eme4', type: ModificationType.unreal, value: 0, level: 1, column: 4,
      },
      {
        id: 'areq', type: ModificationType.string, value: '', level: 0, column: 0,
      },
      {
        id: 'amcs', type: ModificationType.int, value: 0, level: 1, column: 0,
      },
      {
        id: 'acas', type: ModificationType.unreal, value: 0, level: 1, column: 0,
      },
      {
        id: 'adur', type: ModificationType.unreal, value: 1, level: 1, column: 0,
      },
      { id: 'ahky', type: ModificationType.string, value: 'R' },
      { id: 'auhk', type: ModificationType.string, value: 'R' },
    ]);
    grounded.data.push({ id: 'uabi', type: ModificationType.string, value: ability.code });
    flying.data.push({ id: 'uabi', type: ModificationType.string, value: ability.code });

    for (const [index, unitType] of [grounded, flying].entries()) {
      map.addUnit(unitType, unitAt(map, offset, unitType.code));
      offset += index === 0 ? 250 : 500;
    }
    console.log(npc.name, 'placed');
  }
  saveRegressionMap(map, mapDir);
}

/** Fresh exports write `{suite}/{slug}` into the server's exported-assets. Lazy copies that same layout. */
async function exportCases(mapDir: string, suite: SnapshotSuite, cases: readonly SnapshotCase[]): Promise<void> {
  const fresh = isFresh();
  const base = converterUrl();
  console.log(`${suite}: fresh=${fresh ? '1' : '0'} (${cases.length} cases)`);
  if (fresh) {
    for (const testCase of cases) {
      const outputFileName = `${suite}/${testCase.slug}`;
      console.log(`fresh, exporting ${outputFileName}`);
      await exportCharacter(base, outputFileName, testCase);
    }
  }
  const exportRoot = await exportedAssetsDir(base);
  copyIntoMap(mapDir, exportRoot, cases.flatMap((testCase) => modelFiles(suite, testCase.slug)));
}

function unitModel(suite: SnapshotSuite, slug: string): string {
  const stem = `${suite}/${slug}`;
  return suite === 'mount' ? `${stem}_mount` : stem;
}

function modelFiles(suite: SnapshotSuite, slug: string): string[] {
  const stem = `${suite}/${slug}`;
  return suite === 'mount' ? [`${stem}.mdx`, `${stem}_mount.mdx`] : [`${stem}.mdx`];
}

function copyIntoMap(mapDir: string, exportRoot: string, models: readonly string[]): void {
  for (const name of readdirSync(mapDir)) {
    if (name.endsWith('.mdx')) unlinkSync(join(mapDir, name));
  }
  rmSync(join(mapDir, 'wow'), { recursive: true, force: true });
  for (const dir of ['visual-snapshot', 'map-regression', 'retail', 'classic', 'mount']) {
    rmSync(join(mapDir, dir), { recursive: true, force: true });
  }
  const textures = new Set<string>();
  for (const rel of models) {
    const src = join(exportRoot, rel);
    if (!existsSync(src)) throw new Error(`missing ${src}`);
    copyFile(src, join(mapDir, rel));
    for (const texture of modelTextures(readFileSync(src))) textures.add(texture);
  }
  for (const texture of textures) {
    const src = join(exportRoot, texture);
    if (!existsSync(src)) throw new Error(`missing ${src}`);
    copyFile(src, join(mapDir, texture));
  }
  console.log(`Copied ${models.length} models and ${textures.size} textures from ${exportRoot} into ${mapDir}`);
}

function modelTextures(buf: Buffer): string[] {
  const model = new parsers.mdlx.Model();
  model.load(buf);
  const out: string[] = [];
  for (const texture of model.textures) {
    if (texture.replaceableId !== 0) continue;
    const rel = texture.path.replace(/\\/g, '/');
    if (rel === '') continue;
    if (!rel.startsWith('wow/')) throw new Error(`texture is not under wow/: ${texture.path}`);
    out.push(rel);
  }
  return out;
}

function copyFile(src: string, dest: string): void {
  mkdirSync(path.dirname(dest), { recursive: true });
  copyFileSync(src, dest);
}

function heroFields(name: string, model: string) {
  return [
    { id: 'unam', type: ModificationType.string, value: name },
    { id: 'upro', type: ModificationType.string, value: name },
    { id: 'umdl', type: ModificationType.string, value: `${model}.mdx` },
    { id: 'usca', type: ModificationType.real, value: 1 },
    { id: 'ussc', type: ModificationType.real, value: 2 },
    { id: 'ua1b', type: ModificationType.int, value: 500 },
    { id: 'udtm', type: ModificationType.unreal, value: 6 },
    { id: 'usnd', type: ModificationType.string, value: '' },
    { id: 'uhhd', type: ModificationType.int, value: 1 },
  ];
}

function unitAt(map: MapManager, slot: number, skin: string) {
  const mapSize = map.terrain.map;
  const padding = 10 * distancePerTile;
  const width = mapSize.width * distancePerTile - 2 * padding;
  const position = [
    (slot % width) + padding + mapSize.offset.x,
    -(Math.floor(slot / width) * 1000 + padding + mapSize.offset.y),
    0,
  ];
  return {
    variation: 0,
    position,
    rotation: 270,
    scale: [1, 1, 1],
    skin,
    player: 0,
    hitpoints: 100,
    mana: 0,
    randomItemSetPtr: -1,
    droppedItemSets: [],
    gold: 0,
    targetAcquisition: -1,
    hero: {
      level: 10, str: 0, agi: 0, int: 0,
    },
    inventory: [],
    abilities: [],
    random: {
      type: 0, level: 0, itemClass: 0, groupIndex: 0, columnIndex: 0, unitSet: [],
    },
    color: 23,
    waygate: -1,
    id: 0,
  };
}

function saveRegressionMap(map: MapManager, mapDir: string): void {
  map.save(mapDir);
  for (const file of ['war3map.j', 'war3map.imp', 'war3map.wts', 'war3mapSkin.w3u']) {
    const full = join(mapDir, file);
    if (existsSync(full)) unlinkSync(full);
  }
  console.log('Map saved to', mapDir);
}

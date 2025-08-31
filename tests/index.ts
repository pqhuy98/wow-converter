import chalk from 'chalk';
import esMain from 'es-main';
import { existsSync, unlinkSync } from 'fs';
import path, { join } from 'path';

import { distancePerTile } from '@/lib/constants';
import {
  AttachItem, CharacterExporter, local, Size, wowhead,
} from '@/lib/converter/character';
import { MDL } from '@/lib/formats/mdl/mdl';
import { Config, getDefaultConfig } from '@/lib/global-config';
import { Vector3 } from '@/lib/math/common';
import { AttackTag } from '@/lib/objmdl/animation/animation_mapper';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { ModificationType } from '@/vendors/wc3maptranslator/data';
import { MapManager } from '@/vendors/wc3maptranslator/extra/map-manager';

import { testConfigClassic } from './classic';
import { testConfigRetail } from './retail';

await wowExportClient.waitUntilReady();

const testConfig = wowExportClient.isClassic() ? testConfigClassic : testConfigRetail;
const mapDir = testConfig.map;
const testCases = testConfig.testCases;

console.log('--------------------------------');
console.log('|Test mode:', chalk.yellow(testConfig.name));
console.log('--------------------------------');

const ceConfig: Config = {
  ...(await getDefaultConfig()),
  // overrideModels: true,
  overrideModels: false,
};

async function exportTestCases() {
  const npcs: {name: string, mdl?: MDL}[] = [];

  for (let i = 0; i < testCases.length; i++) {
    const url = testCases[i];
    const base = Array.isArray(url) ? url[0] : url;
    const weaponR = Array.isArray(url) ? url[1] : undefined;
    const weaponL = Array.isArray(url) ? url[2] : undefined;
    const sizeStr = Array.isArray(url) ? url[3] : '';
    const size: Size = sizeStr === '' ? undefined : sizeStr;

    const attachItems: Record<string, AttachItem> = {};
    if (weaponR) {
      attachItems[WoWAttachmentID.HandRight] = { path: wowhead(weaponR), scale: 1 };
    }
    if (weaponL) {
      attachItems[WoWAttachmentID.HandLeft] = { path: wowhead(weaponL), scale: 1 };
    }

    let attackTag: AttackTag = 'Unarmed';
    if (weaponR && !weaponL || !weaponR && weaponL) {
      attackTag = '2H';
    }
    if (weaponR && weaponL) {
      attackTag = '1H';
    }

    let name = '';
    if (base.startsWith('local::')) {
      name = `${base.split('\\').pop()!.replace('.obj', '')}`;
    } else if (base.includes('npc=')) {
      const npcId = base.split('npc=').pop()?.split('/').shift();
      const npcName = base.split('/').pop()!.split('#')[0];
      name = `${npcName}-${npcId}`;
    } else if (base.includes('dressing-room')) {
      name = base.split('?').at(-1)!.split('#')[0];
    }
    npcs.push({ name });
    if (existsSync(join(mapDir, `${name}.mdx`)) && !ceConfig.overrideModels) {
      console.log('Skipping file already exists', chalk.yellow(`${name}.mdx`));
      continue;
    }

    const ce = new CharacterExporter(ceConfig);
    npcs.at(-1)!.mdl = await ce.exportCharacter({
      base: base.startsWith('local::') ? local(base.replace('local::', '')) : wowhead(base),
      attachItems,
      attackTag,
      inGameMovespeed: 270,
      size,
      scale: 1.5,
      particlesDensity: 0.5,
    }, name);

    ce.optimizeModelsTextures();
    ce.writeAllModels(mapDir, 'mdx');
    await ce.writeAllTextures(mapDir);
  }

  return npcs;
}

export async function main() {
  const npcs = await exportTestCases();

  const map = new MapManager();
  map.load(mapDir);
  console.log('Unit types', map.unitTypes.map((t) => t.code), map.unitTypes.length);

  map.units = map.units.filter((unit) => typeof unit.type === 'string'); // all melee units
  map.unitTypes = [];

  for (let i = 0; i < npcs.length; i++) {
    const npc = npcs[i];
    const name = npc.name;
    const mdl = npc.mdl;
    const deathSequence = mdl?.sequences.find((s) => s.name === 'Death');
    const unitType = map.addUnitType('hero', 'Hpal', [
      { id: 'unam', type: ModificationType.string, value: name },
      { id: 'upro', type: ModificationType.string, value: name },
      { id: 'umdl', type: ModificationType.string, value: `${name}.mdx` },
      { id: 'usca', type: ModificationType.real, value: 1 },
      { id: 'ussc', type: ModificationType.real, value: 2 },
      { id: 'ua1b', type: ModificationType.int, value: 500 },
      { id: 'uabi', type: ModificationType.string, value: 'A003,A001,A002,A000' },
      {
        id: 'udtm',
        type: ModificationType.real,
        value: deathSequence ? (deathSequence.interval[1] - deathSequence.interval[0]) / 1000 : 6,
      },
    ]);

    const mapSize = map.terrain.map;
    const padding = 10 * distancePerTile;
    const width = mapSize.width * distancePerTile - 2 * padding;
    const i2 = i * 500;
    const position: Vector3 = [
      (i2 % width) + padding + mapSize.offset.x,
      -(Math.floor(i2 / width) * 1000 + padding + mapSize.offset.y),
      0,
    ];

    console.log(name, 'at location', position.slice(0, 2));

    map.addUnit(unitType, {
      variation: 0,
      position,
      rotation: 270,
      scale: [1, 1, 1],
      skin: unitType.code,
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
    });
  }

  console.log('Unit counts:', map.units.length);
  console.log('Unit types counts:', map.unitTypes.length);

  map.save(mapDir);

  const filesToDelete = [
    'war3map.j',
    'war3map.imp',
    'war3map.wts',
    'war3mapSkin.w3u',
  ];
  for (const file of filesToDelete) {
    if (existsSync(join(mapDir, file))) unlinkSync(join(mapDir, file));
  }
  console.log('Map saved to', chalk.blue(path.resolve(mapDir)));
}

if (esMain(import.meta)) {
  void main().then(() => process.exit(0));
}

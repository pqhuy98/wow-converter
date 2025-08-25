import chalk from 'chalk';
import esMain from 'es-main';
import { existsSync } from 'fs';
import { join } from 'path';

import { distancePerTile } from '@/lib/constants';
import {
  AttachItem, CharacterExporter, local, Size, wowhead,
} from '@/lib/converter/character';
import { Config, getDefaultConfig } from '@/lib/global-config';
import { Vector3 } from '@/lib/math/common';
import { AttackTag } from '@/lib/objmdl/animation/animation_mapper';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { ModificationType } from '@/vendors/wc3maptranslator/data';
import { MapManager } from '@/vendors/wc3maptranslator/extra/map-manager';

export const outputDir = './maps/test-regression-classic.w3x';
export const ceConfig: Config = {
  ...(await getDefaultConfig()),
  overrideModels: true,
  // overrideModels: false,
};
export const ce = new CharacterExporter(ceConfig);

async function exportTestCases() {
  if (!wowExportClient.isClassic()) {
    throw new Error('This test is only for WoW classic');
  }

  const testCases: [string, string, string, Size | ''][] = [
    ['https://www.wowhead.com/mop-classic/npc=28714/ildine-sorrowspear', '', '', ''],
    ['https://www.wowhead.com/mop-classic/npc=28674/aludane-whitecloud', '', '', ''],
    ['https://www.wowhead.com/mop-classic/npc=30115/vereesa-windrunner', '', '', ''],
    ['https://www.wowhead.com/mop-classic/npc=32678/emeline-fizzlefry', '', '', ''],
    ['https://www.wowhead.com/mop-classic/npc=32677/whirt-the-all-knowing', '', '', ''],
  ];

  const names: string[] = [];

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
      name = `${i}-${base.split('\\').pop()!.replace('.obj', '')}`;
    } else {
      const npcId = base.split('npc=').pop()?.split('/').shift();
      const npcName = base.split('/').pop()!.split('#')[0];
      name = `${i}-${npcName}-${npcId}`;
    }
    names.push(name);
    if (existsSync(join(outputDir, `${name}.mdx`)) && !ceConfig.overrideModels) {
      console.log('Skipping file already exists', chalk.yellow(`${name}.mdx`));
      continue;
    }

    await ce.exportCharacter({
      base: base.startsWith('local::') ? local(base.replace('local::', '')) : wowhead(base),
      attachItems,
      attackTag,
      inGameMovespeed: 270,
      size,
      scale: 1.5,
      particlesDensity: 0.5,
    }, name);
  }

  return names;
}

export async function main() {
  const names = await exportTestCases();

  ce.optimizeModelsTextures();
  ce.writeAllModels(outputDir, 'mdx');
  await ce.writeAllTextures(outputDir);

  const map = new MapManager();
  map.load(outputDir);
  map.units = map.units.filter((unit) => typeof unit.type === 'string'); // all melee units
  map.unitTypes = [];

  for (let i = 0; i < names.length; i++) {
    const name = names[i];
    const unitType = map.addUnitType('hero', 'Hpal', [
      { id: 'unam', type: ModificationType.string, value: name },
      { id: 'upro', type: ModificationType.string, value: name },
      { id: 'umdl', type: ModificationType.string, value: `${name}.mdx` },
      { id: 'usca', type: ModificationType.real, value: 1 },
      { id: 'ussc', type: ModificationType.real, value: 2 },
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

  map.save(outputDir);
}

if (esMain(import.meta)) {
  void main().then(() => process.exit(0));
}

import chalk from 'chalk';
import esMain from 'es-main';
import { existsSync, unlinkSync } from 'fs';
import path, { join } from 'path';

import { distancePerTile } from '@/lib/constants';
import {
  AttachItem, CharacterExporter, local, wowhead,
} from '@/lib/converter/character';
import { MDL } from '@/lib/formats/mdl/mdl';
import { Config, getDefaultConfig } from '@/lib/global-config';
import { Vector3 } from '@/lib/math/common';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { ModificationType } from '@/vendors/wc3maptranslator/data';
import { MapManager } from '@/vendors/wc3maptranslator/extra/map-manager';

const mapDir = 'maps/test-regression-mount.w3x';
console.log('--------------------------------');
console.log('|Test mode: Mount');
console.log('--------------------------------');

const ceConfig: Config = {
  ...(await getDefaultConfig()),
  // overrideModels: true,
  overrideModels: false,
  maxTextureSize: 512,
};

const testCases: {
  base: string;
  weaponR?: string;
  weaponL?: string;
  mount: string;
  animation?: string;
  seatOffset?: Vector3;
  scale?: number;
}[] = [
  {
    base: 'https://www.wowhead.com/wotlk/npc=36597/the-lich-king',
    weaponR: 'https://www.wowhead.com/classic/item=231885/frostmourne',
    mount: 'https://www.wowhead.com/wotlk/item=50818/invincibles-reins',
    seatOffset: [-15, 0, 15],
  },
  {
    base: 'https://www.wowhead.com/wotlk/npc=36597/the-lich-king',
    weaponR: 'https://www.wowhead.com/classic/item=231885/frostmourne',
    weaponL: '',
    mount: 'https://www.wowhead.com/wotlk/npc=28531/frost-wyrm-mount',
    scale: 0.75,
  },
  {
    base: 'https://www.wowhead.com/wotlk/npc=29173/highlord-darion-mograine',
    weaponR: 'https://www.wowhead.com/item=39344/slayer-of-the-lifeless',
    weaponL: 'https://www.wowhead.com/item=39344/slayer-of-the-lifeless',
    mount: 'https://www.wowhead.com/item=52200/reins-of-the-crimson-deathcharger',
  },
  {
    base: 'https://www.wowhead.com/wotlk/npc=37119/highlord-tirion-fordring#modelviewer',
    weaponR: 'https://www.wowhead.com/item=120978/ashbringer',
    mount: 'https://www.wowhead.com/item=47179/argent-charger',
  },
  {
    base: 'https://www.wowhead.com/dressing-room?human-paladin#fz80z0zN89c8d8zA8k58zdn8fG8M3m8a8mvR8sc8zyb8b8zxA8q8zno8fh8M2Y8fu8M3q8s8zjP877LWjs8zpa87MWT48zpa87MPie808OQL808PYC808X2U808Uio808X31808JzI87M1kr87o',
    mount: 'https://www.wowhead.com/item=87777/reins-of-the-astral-cloud-serpent#modelviewer',
    scale: 0.5,
  },
  {
    base: 'https://www.wowhead.com/npc=35222/trade-prince-gallywix',
    mount: 'https://www.wowhead.com/mop-classic/item=95416/sky-golem#screenshots',
    scale: 0.6,
  },
  {
    base: 'https://www.wowhead.com/dressing-room?aladdin#fz80o0zN89c8a8G8s8z8q8O8b8zc8fh8M2Q8d8zh8fu8M218sc8zya8MEt8wUK8fG8M238k58zGa8zYw8dAG8Mx2877iYsv808z4V808rMd808M2r87VM2u808mfH808MVv808M2Y808Y808CF87Mb3187o',
    mount: 'https://www.wowhead.com/item=44554/flying-carpet',
    animation: 'MountCrouch',
  },
];

function getName(value: string) {
  let name = '';
  if (value.startsWith('local::')) {
    name = `local-${value.split('\\').pop()!.replace('.obj', '')}`;
  } else if (value.includes('npc=')) {
    const npcId = value.split('npc=').pop()?.split('/').shift();
    const npcName = value.split('/').pop()!.split('#')[0];
    name = `npc-${npcName}-${npcId}`;
  } else if (value.includes('item=')) {
    const itemId = value.split('item=').pop()?.split('/').shift();
    const itemName = value.split('/').pop()!.split('#')[0];
    name = `item-${itemName}-${itemId}`;
  } else if (value.includes('spell=')) {
    const spellId = value.split('spell=').pop()?.split('/').shift();
    const spellName = value.split('/').pop()!.split('#')[0];
    name = `spell-${spellName}-${spellId}`;
  } else if (value.includes('dressing-room')) {
    name = `dressing-room-${value.split('?').at(-1)!.split('#')[0]}`;
  }
  return name;
}

async function exportTestCases() {
  const result: {name: string, mdl?: MDL}[] = [];

  for (let i = 0; i < testCases.length; i++) {
    const test = testCases[i];
    const base = test.base;
    const weaponR = test.weaponR;
    const weaponL = test.weaponL;
    const mountStr = test.mount;
    const animation = test.animation;
    const seatOffset = test.seatOffset;
    const scale = test.scale;

    const attachItems: Record<string, AttachItem> = {};
    if (weaponR) {
      attachItems[WoWAttachmentID.HandRight] = { path: wowhead(weaponR), scale: 1 };
    }
    if (weaponL) {
      attachItems[WoWAttachmentID.HandLeft] = { path: wowhead(weaponL), scale: 1 };
    }

    const name = `${getName(base)}_${getName(mountStr)}`;
    result.push({ name: `${name}_mount` });
    if (existsSync(join(mapDir, `${name}.mdx`)) && !ceConfig.overrideModels) {
      console.log('Skipping file already exists', chalk.yellow(`${name}.mdx`));
      continue;
    }

    const ce = new CharacterExporter(ceConfig);
    result.at(-1)!.mdl = await ce.exportCharacter({
      base: base.startsWith('local::') ? local(base.replace('local::', '')) : wowhead(base),
      attachItems,
      inGameMovespeed: 270,
      size: 'hero',
      mount: {
        path: mountStr.startsWith('local::') ? local(mountStr.replace('local::', '')) : wowhead(mountStr),
        animation,
        scale,
        seatOffset,
      },
    }, name);

    ce.optimizeModelsTextures();
    await ce.writeAllTextures(mapDir);
    await ce.writeAllModels(mapDir, 'mdx');
  }

  return result;
}
export async function main() {
  const npcs = await exportTestCases();

  const map = new MapManager();
  map.load(mapDir);

  console.log(JSON.stringify(map.abilities, null, 2));
  console.log('Unit types', map.unitTypes.map((t) => t.code), map.unitTypes.length);

  map.units = map.units.filter((unit) => typeof unit.type === 'string'); // all melee units
  map.unitTypes = [];
  map.abilities = [];

  let offset = 0;

  for (let i = 0; i < npcs.length; i++) {
    const npc = npcs[i];
    const name = npc.name;
    const mdl = npc.mdl;
    const deathSequence = mdl?.sequences.find((s) => s.name.startsWith('Death'));
    const deathTime = deathSequence ? (deathSequence.interval[1] - deathSequence.interval[0]) / 1000 : 6;
    const unitType0 = map.addUnitType('hero', 'Hpal', [
      { id: 'unam', type: ModificationType.string, value: name },
      { id: 'upro', type: ModificationType.string, value: name },
      { id: 'umdl', type: ModificationType.string, value: `${name}.mdx` },
      { id: 'usca', type: ModificationType.real, value: 1 },
      { id: 'ussc', type: ModificationType.real, value: 2 },
      { id: 'ua1b', type: ModificationType.int, value: 500 },
      { id: 'udtm', type: ModificationType.unreal, value: deathTime },
      { id: 'usnd', type: ModificationType.string, value: '' },
      { id: 'uhhd', type: ModificationType.int, value: 1 },
    ]);

    const unitTypes = [unitType0];

    const unitType1 = map.addUnitType('hero', 'Hpal', [
      { id: 'unam', type: ModificationType.string, value: `${name} Alt` },
      { id: 'upro', type: ModificationType.string, value: `${name} Alt` },
      { id: 'umdl', type: ModificationType.string, value: `${name}.mdx` },
      { id: 'usca', type: ModificationType.real, value: 1 },
      { id: 'ussc', type: ModificationType.real, value: 2 },
      { id: 'ua1b', type: ModificationType.int, value: 500 },
      { id: 'udtm', type: ModificationType.unreal, value: deathTime },
      { id: 'usnd', type: ModificationType.string, value: '' },
      { id: 'uhhd', type: ModificationType.int, value: 1 },
      // Fly
      { id: 'uani', type: ModificationType.string, value: 'alternate' },
      { id: 'umvh', type: ModificationType.unreal, value: 300 },
      { id: 'umvt', type: ModificationType.string, value: 'fly' },
    ]);
    unitTypes.push(unitType1);

    const ability = map.addAbility('Arav', [ // Storm Crow Form
      {
        id: 'Emeu', type: ModificationType.string, value: unitType1.code, level: 1, column: 0,
      },
      {
        id: 'Eme1', type: ModificationType.string, value: unitType0.code, level: 1, column: 1,
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

    unitType0.data.push({ id: 'uabi', type: ModificationType.string, value: ability.code });
    unitType1.data.push({ id: 'uabi', type: ModificationType.string, value: ability.code });

    const mapSize = map.terrain.map;
    const padding = 10 * distancePerTile;
    const width = mapSize.width * distancePerTile - 2 * padding;

    unitTypes.forEach((unitType, i) => {
      const i2 = offset;
      offset += i === 0 ? 250 : 500;
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

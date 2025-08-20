import { MapTranslator } from './../src/vendors/wc3maptranslator/translators/MapTranslator';
import esMain from 'es-main';
import { writeFileSync } from 'fs';

import { AttachItem, CharacterExporter, Size, wowhead } from '@/lib/converter/character-exporter';

import { getDefaultConfig } from '@/lib/global-config';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { AttackTag } from '@/lib/objmdl/animation/animation_mapper';
import { MapManager } from '@/vendors/wc3maptranslator/extra/map-manager';
import { ModificationType } from '@/vendors/wc3maptranslator/data';
import { distancePerTile } from '@/lib/constants';
import { Vector3 } from '@/lib/math/common';

export const testMapDir = './maps/test-regression.w3x';
export const ceConfig = await getDefaultConfig();
export const ce = new CharacterExporter(testMapDir, ceConfig);

async function exportTestCases() {
  const testCases: [string, string, string, Size | ""][] = [
    ['https://www.wowhead.com/wotlk/npc=36855/lady-deathwhisper', "", "", ""],
    ['https://www.wowhead.com/wotlk/npc=36612/lord-marrowgar', "", "", ""],
    ['https://www.wowhead.com/mop-classic/npc=71953/xuen', "", "", ""],
    ['https://www.wowhead.com/npc=154515/yulon', "", "", ""],
    ['https://www.wowhead.com/npc=56439/sha-of-doubt', "", "", "giant"],
    [
      'https://www.wowhead.com/wotlk/npc=37187/high-overlord-saurfang',
      "https://www.wowhead.com/wotlk/item=49623/shadowmourne",
      "https://www.wowhead.com/wotlk/item=49623/shadowmourne",
      ""
    ],
    [
      'https://www.wowhead.com/wotlk/npc=37119/highlord-tirion-fordring',
      "https://www.wowhead.com/item=120978/ashbringer",
      "",
      ""
    ],
    [
      "https://www.wowhead.com/wotlk/npc=36597/the-lich-king",
      "https://www.wowhead.com/classic/item=231885/frostmourne",
      "",
      ""
    ],
    ["https://www.wowhead.com/npc=102672/nythendra", "", "", "giant"],
    ["https://www.wowhead.com/npc=211664/elisande", "", "", ""],
    ["https://www.wowhead.com/npc=113201/thicket-manahunter", "", "", ""],
    ["https://www.wowhead.com/npc=68397/lei-shen", "", "", ""],
    [
      "https://www.wowhead.com/npc=22917/illidan-stormrage",
      "https://www.wowhead.com/item=32837/warglaive-of-azzinoth",
      "https://www.wowhead.com/item=32838/warglaive-of-azzinoth",
      ""
    ],
    ["https://www.wowhead.com/npc=114895/nightbane#modelviewer", "", "", "giant"],
    ["https://www.wowhead.com/mop-classic/npc=64986/heavenly-onyx-cloud-serpent", "", "", ""],
  ];

  const names: string[] = [];

  for (let i = 0; i < testCases.length; i++) {
    const url = testCases[i];
    const base = Array.isArray(url) ? url[0] : url;
    const weaponR = Array.isArray(url) ? url[1] : undefined;
    const weaponL = Array.isArray(url) ? url[2] : undefined;
    const sizeStr = Array.isArray(url) ? url[3] : "";
    const size: Size = sizeStr === "" ? undefined : sizeStr;

    const attachItems: Record<string, AttachItem> = {};
    if (weaponR) {
      attachItems[WoWAttachmentID.HandRight] = { path: wowhead(weaponR), scale: 1 };
    }
    if (weaponL) {
      attachItems[WoWAttachmentID.HandLeft] = { path: wowhead(weaponL), scale: 1 };
    }

    let attackTag: AttackTag = "Unarmed";
    if (weaponR && !weaponL || !weaponR && weaponL) {
      attackTag = "2H";
    }
    if (weaponR && weaponL) {
      attackTag = "1H";
    }

    const npcId = base.split("npc=").pop()?.split("/").shift();
    const npcName = base.split("/").pop();
    const name = `${i}-${npcName}-${npcId}`;
    // await ce.exportCharacter({
    //   base: wowhead(base),
    //   attachItems,
    //   attackTag,
    //   inGameMovespeed: 270,
    //   size,
    //   scale: 1.5
    // }, name);
    names.push(name);
    console.log(name, attackTag, size);
  }

  return names;
}

export async function main() {
  const names = await exportTestCases();

  ce.models.forEach(([model, path]) => {
    model.modify
      .sortSequences()
      .removeUnusedVertices()
      .removeUnusedNodes()
      .removeUnusedMaterialsTextures()
      .optimizeKeyFrames();
    model.sync();
    writeFileSync(`${path}.mdx`, model.toMdx());
    console.log('Wrote character model to', path);
  });
  ce.assetManager.purgeTextures(ce.models.flatMap(([m]) => m.textures.map((t) => t.image)));
  await ce.assetManager.exportTextures(ce.outputPath);

  const map = new MapManager(testMapDir);
  map.load();
  map.units = [];
  map.unitTypes = []

  for(let i = 0; i < names.length; i++) {
    const name = names[i % names.length];
    const unitType = map.addUnitType("hero", "Hpal", [
      { id: 'unam', type: ModificationType.string, value: name },
      { id: 'upro', type: ModificationType.string, value: name },
      { id: 'umdl', type: ModificationType.string, value: `${name}.mdx` },
      { id: 'usca', type: ModificationType.real, value: 1 },
      { id: 'ussc', type: ModificationType.real, value: 2 },
    ])

    const mapSize = map.terrain.map;
    const padding = 10 * distancePerTile
    const width = mapSize.width * distancePerTile - 2 * padding
    const i2 = i * 500
    const position: Vector3 = [
      (i2 % width) + padding + mapSize.offset.x,
      -(Math.floor(i2 / width) * 1000 + padding + mapSize.offset.y),
      0,
    ]

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
    })
  }

  map.save();
}

if (esMain(import.meta)) {
  void main().then(() => process.exit(0));
}

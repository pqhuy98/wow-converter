import {
  creature, creature_template, creature_template_model, item_template, PrismaClient,
} from '@prisma/client';
import chalk from 'chalk';
import { existsSync, writeFileSync } from 'fs';
import { join } from 'path';

import {
  Character, CharacterExporter, displayID, wowhead,
} from '../converter/character';
import { Config } from '../converter/common';
import { AttackTag } from '../objmdl/animation/animation_mapper';
import { WoWAttachmentID } from '../objmdl/animation/bones_mapper';
import { toMap } from '../utils';

const prismaClient = new PrismaClient();

export interface Equipment {
  item1?: item_template;
  item2?: item_template;
  item3?: item_template;
}

export interface Creature {
  creature: creature;
  template: creature_template;
  equipment: Equipment | null;
  model: creature_template_model;
}

export async function getCreaturesInTile(mapId: number, tileXy: [number, number]): Promise<Creature[]> {
  const [tileX, tileY] = tileXy;

  const tileSize = 533.3333333; // yards per ADT tile

  // WoW coordinates: origin at center, x east, y north
  const worldMinX = (32 - tileY - 1) * tileSize;
  const worldMaxX = (32 - tileY) * tileSize;
  const worldMinY = (32 - tileX - 1) * tileSize;
  const worldMaxY = (32 - tileX) * tileSize;

  const [minX, maxX] = worldMinX < worldMaxX ? [worldMinX, worldMaxX] : [worldMaxX, worldMinX];
  const [minY, maxY] = worldMinY < worldMaxY ? [worldMinY, worldMaxY] : [worldMaxY, worldMinY];

  const creatures = await prismaClient.creature.findMany({
    where: {
      map: mapId,
      position_x: { gte: minX, lte: maxX },
      position_y: { gte: minY, lte: maxY },
    },
  });
  const templateIds = new Set(creatures.map((c) => c.id1));
  const templatesMap = toMap(await prismaClient.creature_template.findMany({
    where: {
      entry: { in: Array.from(templateIds) },
    },
  }), 'entry');

  const equipments = await prismaClient.creature_equip_template.findMany({
    where: {
      CreatureID: { in: Array.from(templateIds) },
    },
  });

  const equipmentsMap = toMap(equipments, 'CreatureID');

  const modelsMap = toMap(await prismaClient.creature_template_model.findMany({
    where: {
      CreatureID: { in: Array.from(templateIds) },
    },
  }), 'CreatureID');

  const itemsMap = toMap(await prismaClient.item_template.findMany({
    where: {
      entry: {
        in: Array.from(new Set([
          ...equipments.map((e) => e.ItemID1),
          ...equipments.map((e) => e.ItemID2),
          ...equipments.map((e) => e.ItemID3),
        ].filter((i) => i > 0))),
      },
    },
  }), 'entry');

  return creatures.map((c) => {
    const template = templatesMap.get(c.id1)!;
    const model = modelsMap.get(template.entry)!;
    const equipment = equipmentsMap.get(template.entry);
    const item1 = equipment ? itemsMap.get(equipment.ItemID1) : undefined;
    const item2 = equipment ? itemsMap.get(equipment.ItemID2) : undefined;
    const item3 = equipment ? itemsMap.get(equipment.ItemID3) : undefined;
    return {
      creature: c,
      template,
      equipment: { item1, item2, item3 },
      model,
    };
  });
}

export async function exportCreatureModels(
  creatures: Creature[],
  outputPath: string,
  config: Config,
) {
  const debug = false;
  let cnt = 0;
  const exportedDisplayIds = new Set<number>();

  const batchSize = 5;
  for (let i = 0; i < creatures.length; i += batchSize) {
    const batch = creatures.slice(i, i + batchSize);
    await Promise.all(batch.map(async (c) => {
      cnt++;
      const displayId = c.model.CreatureDisplayID;
      if (!displayId) {
        throw new Error(`No display id found for creature template ${c.template.entry}`);
      }

      if (existsSync(join(outputPath, `creature-${displayId}.mdx`)) && !config.overrideModels) {
        debug && console.log('Skipping file already exists', chalk.yellow(`creature-${displayId}.mdx`));
        return;
      }

      if (exportedDisplayIds.has(displayId)) {
        return;
      }
      exportedDisplayIds.add(displayId);

      console.log('\n');
      console.log(`==== Exporting creature ${chalk.blue(c.template.name)} (${cnt}/${creatures.length}, guid=${c.creature.guid}, displayId=${displayId})`);

      const start0 = performance.now();
      let start = performance.now();

      const attachItems: Character['attachItems'] = {};

      if (c.equipment?.item1) {
        const slot = inventoryTypeToEquipmentSlot(c.equipment.item1.InventoryType, 0);
        if (slot !== undefined) {
          attachItems[slot] = { path: wowhead(`https://www.wowhead.com/wotlk/item=${c.equipment.item1.entry}`), scale: 1 };
        } else {
          console.log(`Unmapped item 1: ${c.equipment.item1.entry} ${c.equipment.item1.entry}`);
        }
      }
      if (c.equipment?.item2) {
        const slot = inventoryTypeToEquipmentSlot(c.equipment.item2.InventoryType, 1);
        if (slot !== undefined) {
          attachItems[slot] = { path: wowhead(`https://www.wowhead.com/wotlk/item=${c.equipment.item2.entry}`), scale: 1 };
        } else {
          console.log(`Unmapped item 2: ${c.equipment.item2.entry} ${c.equipment.item2.entry}`);
        }
      }

      const ex = new CharacterExporter(outputPath, config);
      const attackTag = c.equipment ? getAttackTag(c.equipment) : undefined;
      console.log('Attack tag:', attackTag);
      await ex.exportCharacter({
        base: displayID(displayId),
        inGameMovespeed: 270,
        attachItems,
        attackTag,
      }, `creature-${displayId}`);
      console.log('initial export character took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

      start = performance.now();
      ex.models.forEach(([model, path]) => {
        model.modify
          .sortSequences()
          .removeUnusedVertices()
          .removeUnusedNodes()
          .removeUnusedMaterialsTextures()
          .optimizeKeyFrames();
        model.sync();
        writeFileSync(`${path}.mdx`, model.toMdx());
        console.log('Wrote character model to', chalk.blue(path));
      });
      console.log('optimize and write took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

      start = performance.now();
      ex.assetManager.purgeTextures(ex.models.flatMap(([m]) => m.textures.map((t) => t.image)));
      await ex.assetManager.exportTextures(ex.outputPath);
      console.log('export materials took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

      const end = performance.now();
      console.log(chalk.green(`=> Exported creature ${c.template.name} in ${chalk.yellow(((end - start0) / 1000).toFixed(2))}s`));
    }));
  }
}

enum INVENTORY_TYPE {
  WEAPON = 13,
  SHIELD = 14,
  RANGED = 15,
  RANGEDRIGHT = 26,
  TWO_HANDED_WEAPON = 17,
  WEAPONMAINHAND = 21,
  WEAPONOFFHAND = 22,
  HOLDABLE = 23,
  THROWN = 25,
  RELIC = 28,
}

function inventoryTypeToEquipmentSlot(inventoryType: number, idx: number): WoWAttachmentID | undefined {
  switch (inventoryType) {
    case INVENTORY_TYPE.WEAPON:
      return idx === 0 ? WoWAttachmentID.HandRight : WoWAttachmentID.HandLeft;
    case INVENTORY_TYPE.WEAPONMAINHAND:
      return WoWAttachmentID.HandRight;
    case INVENTORY_TYPE.SHIELD:
      return WoWAttachmentID.Shield;
    case INVENTORY_TYPE.RANGED:
    case INVENTORY_TYPE.RANGEDRIGHT:
      return WoWAttachmentID.HandRight;
    case INVENTORY_TYPE.TWO_HANDED_WEAPON:
      return WoWAttachmentID.HandRight;
    case INVENTORY_TYPE.HOLDABLE:
      return WoWAttachmentID.HandLeft;
    case INVENTORY_TYPE.WEAPONOFFHAND:
    case INVENTORY_TYPE.THROWN:
      return WoWAttachmentID.HandRight;
    case INVENTORY_TYPE.RELIC:
      return WoWAttachmentID.HandRight;
    default:
      return undefined;
  }
}

function getAttackTag(equipment: Equipment): AttackTag {
  const inventoryType1 = equipment.item1?.InventoryType ?? 0;
  const inventoryType2 = equipment.item2?.InventoryType ?? 0;
  console.log({ inventoryType1, inventoryType2 });

  if (inventoryType1 === INVENTORY_TYPE.TWO_HANDED_WEAPON && !inventoryType2) {
    return '2H';
  }

  const rightCanMelee = inventoryType1 === INVENTORY_TYPE.WEAPON
    || inventoryType1 === INVENTORY_TYPE.WEAPONMAINHAND
    || inventoryType1 === INVENTORY_TYPE.WEAPONOFFHAND
    || inventoryType1 === INVENTORY_TYPE.TWO_HANDED_WEAPON;
  const leftCanMelee = inventoryType2 === INVENTORY_TYPE.WEAPON
    || inventoryType2 === INVENTORY_TYPE.WEAPONMAINHAND
    || inventoryType2 === INVENTORY_TYPE.WEAPONOFFHAND
    || inventoryType2 === INVENTORY_TYPE.TWO_HANDED_WEAPON;

  if (rightCanMelee || leftCanMelee) {
    return '1H';
  }

  if (inventoryType1 === INVENTORY_TYPE.RANGED || inventoryType1 === INVENTORY_TYPE.RANGEDRIGHT) {
    return 'Bow';
  }

  if (inventoryType1 === INVENTORY_TYPE.THROWN) {
    return 'Thrown';
  }

  return 'Unarmed';
}

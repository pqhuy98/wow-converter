import {
  creature, creature_template, creature_template_model, item_template, Prisma, PrismaClient,
} from '@prisma/client';
import chalk from 'chalk';
import { existsSync } from 'fs';
import _ from 'lodash';
import { join } from 'path';

import {
  Character, CharacterExporter, displayID, wowhead,
} from '../converter/character';
import { guessAttackTag, inventoryTypeToEquipmentSlot } from '../converter/character/item-mapper';
import { Config } from '../global-config';
import { toMap, workerPool } from '../utils';

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

export async function getCreaturesInTile(
  mapId: number,
  tileXy: [number, number],
  extraConditions: Prisma.creatureWhereInput = {},
): Promise<Creature[]> {
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
      ...extraConditions,
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

  // Visibility filter based on server logic
  const CREATURE_FLAG_EXTRA_TRIGGER = 0x00000080;
  const CREATURE_FLAG_EXTRA_GHOST_VISIBILITY = 0x00000400;
  const INVISIBLE_DISPLAY_ID = 11686; // CreatureModel::DefaultInvisibleModel

  const visibleEntryIds = new Set(Array.from(templateIds).filter((entry) => {
    const t = templatesMap.get(entry);
    const m = modelsMap.get(entry);
    if (!t || !m) return false;
    const flagsExtra = t.flags_extra ?? 0;
    if ((flagsExtra & CREATURE_FLAG_EXTRA_TRIGGER) !== 0) return false;
    if ((flagsExtra & CREATURE_FLAG_EXTRA_GHOST_VISIBILITY) !== 0) return false;
    if (m.CreatureDisplayID === INVISIBLE_DISPLAY_ID) return false;
    return true;
  }));

  const visibleCreatures = creatures.filter((c) => visibleEntryIds.has(c.id1));

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

  return visibleCreatures.map((c) => {
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
  allCreatures: Creature[],
  outputPath: string,
  config: Config,
) {
  const debug = false;
  let cnt = 0;

  // Filter out creatures with the same display id
  const displayIds = new Set<number>();
  const creatures = _.shuffle(allCreatures.filter((c) => {
    const displayId = c.model.CreatureDisplayID;
    if (!displayId) {
      throw new Error(`No display id found for creature template ${c.template.entry}`);
    }
    if (displayIds.has(displayId)) {
      return false;
    }
    displayIds.add(displayId);
    return true;
  }));

  const batchSize = 5;
  await workerPool(
    batchSize,
    creatures.map((c) => async () => {
      cnt++;
      const displayId = c.model.CreatureDisplayID;
      if (!displayId) {
        throw new Error(`No display id found for creature template ${c.template.entry}`);
      }

      if (existsSync(join(outputPath, `creature-${displayId}.mdx`)) && !config.overrideModels) {
        debug && console.log('Skipping file already exists', chalk.yellow(`creature-${displayId}.mdx`));
        return;
      }

      console.log('\n');
      console.log(`==== Exporting creature ${chalk.blue(c.template.name)} (${cnt}/${allCreatures.length}, guid=${c.creature.guid}, displayId=${displayId})`);

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

      const ex = new CharacterExporter(config);
      const attackTag = c.equipment ? guessAttackTag(
        c.equipment.item1?.InventoryType ?? 0,
        c.equipment.item2?.InventoryType ?? 0,
      ) : undefined;
      console.log('Attack tag:', attackTag);
      await ex.exportCharacter({
        base: displayID(displayId),
        inGameMovespeed: 270,
        attachItems,
        attackTag,
      }, `creature-${displayId}`);

      start = performance.now();
      ex.optimizeModelsTextures();
      ex.writeAllModels(outputPath, config.mdx ? 'mdx' : 'mdl');
      console.log('optimize and write took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

      start = performance.now();
      await ex.writeAllTextures(outputPath);
      console.log('export materials took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

      const end = performance.now();
      console.log(chalk.green(`=> Exported creature ${c.template.name} in ${chalk.yellow(((end - start0) / 1000).toFixed(2))}s`));
    }),
  );
}

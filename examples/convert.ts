import assert from 'assert';
import chalk from 'chalk';
import { existsSync, rmSync, writeFileSync } from 'fs';
import fsExtra from 'fs-extra';
import _ from 'lodash';
import path from 'path';

import { dataHeightToGameZ, maxGameHeightDiff } from '@/lib/utils';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import {
  DoodadsTranslator, ObjectsTranslator, TerrainTranslator, UnitsTranslator,
} from '@/vendors/wc3maptranslator';
import { ObjectType, Terrain } from '@/vendors/wc3maptranslator/data';
import { ModificationType, ObjectModificationTable } from '@/vendors/wc3maptranslator/data/ObjectModificationTable';
import { Unit } from '@/vendors/wc3maptranslator/data/Unit';

import { Creature, exportCreatureModels, getCreaturesInTile } from '../src/lib/azerothcore-client/creatures';
import { distancePerTile } from '../src/lib/constants';
import { Config, WowObject, WowObjectType } from '../src/lib/converter/common';
import { computeAbsoluteMinMaxExtents } from '../src/lib/converter/model-manager';
import { Wc3Converter } from '../src/lib/converter/wc3-converter';
import { WowObjectManager } from '../src/lib/converter/wow-object-manager';
import {
  dataHeightMax, dataHeightMin, defaultConfig,
} from '../src/lib/global-config';
import { Vector3 } from '../src/lib/math/common';
import { radians } from '../src/lib/math/rotation';
import { V3 } from '../src/lib/math/vector';
import { generateFourCC } from '../src/lib/utils';

const mapIds = {
  Azeroth: 0,
  Northrend: 571,
  Outland: 530,
};

const paths = [
  // '**/northrend/adt_*_*.obj',
  // ...buildPaths('**/azeroth', [22, 26], [39, 43]),
  ...buildPaths('**/northrend', [33, 34], [24, 25]),
];
const mapId = mapIds.Northrend;
// const mapPath = 'maps/andorhal.w3x';
const mapPath = 'maps/naxxramas.w3x';

const assetPrefix = 'wow';

const config: Config = {
  ...defaultConfig,
  assetPrefix,
  terrainHeightClampPercent: {
    // lower: 0,
    // upper: 1,

    lower: gameZToPercent(-600),
    upper: gameZToPercent(800),
  },
  pitchRollThresholdRadians: radians(99),
  waterZThreshold: -2000,
  overrideModels: false,
  placeCreatures: true,
  exportCreatureModels: true,
};

const mapAngle: 0 | 90 | 180 | 270 = 0;

const skipCreatures = [
  'DND', 'Invisible Stalker',
];

const filter = (_file: string, type: WowObjectType) => true;
// const filter = (_file: string, type: WowObjectType) => type === 'adt';

/**
 * ======================================================================
 * ======================================================================
 * ======================================================================
 * ======================================================================
 */

config.terrainHeightClampPercent.lower = Math.max(0, config.terrainHeightClampPercent.lower);
config.terrainHeightClampPercent.upper = Math.min(1, config.terrainHeightClampPercent.upper);
console.log(config.terrainHeightClampPercent);

function buildPaths(prefix: string, x: [number, number], y: [number, number]) {
  const res: string[] = [];
  for (let i = x[0]; i <= x[1]; i++) {
    for (let j = y[0]; j <= y[1]; j++) {
      res.push(path.join(prefix, `adt_${i}_${j}.obj`));
    }
  }
  return res;
}

function gameZToPercent(z: number) {
  return (z - dataHeightToGameZ(dataHeightMin)) / maxGameHeightDiff;
}

async function main() {
  await wowExportClient.waitUntilReady();
  const { write } = await generate(paths);
  await write({ mapPath });
}

export async function generate(adtPatterns: string[]) {
  const wowObjectManager = new WowObjectManager(config);
  await wowObjectManager.parse(adtPatterns, filter);

  console.log('Total objects:', wowObjectManager.objects.size);
  const typeCountMap = _([...wowObjectManager.objects.values()])
    .map((o) => o.type)
    .countBy();
  console.log('Type count:', typeCountMap.entries().toJSON());

  const roots = wowObjectManager.roots;

  console.log(`Rotating roots at center by ${mapAngle} degrees`);
  wowObjectManager.rotateRootsAtCenter([0, 0, radians(-90 + mapAngle)]);

  const war3Exporter = new Wc3Converter();

  const wc3Terrain = war3Exporter.generateTerrainWithHeight(roots, config);
  const { doodadsData, doodads, doodadTypesWithPitchRoll } = war3Exporter.placeDoodads(roots, wc3Terrain, config);

  console.log('Total doodads:', doodads[0].length);
  console.log('Total doodad types:', Object.keys(doodadsData.custom).length);

  const ddMap = new Map(Object.keys(doodadsData.custom).map((k) => [k.slice(0, 4), doodadsData.custom[k]]));

  if (doodads[0].length > 30_000) {
    console.log('Eliminating doodads to fit Wc3 30000 doodads limit, current count:', doodads[0].length);
    const modelCountMap = _(doodads[0].map((d) => ddMap.get(d.type)!).map((d) => d[0].value))
      .countBy(); // { value: count, ... }

    const excessDoodads = doodads[0].length - 30_000;

    let removed = doodads[0].filter((d) => {
      const model = ddMap.get(d.type)![0].value as string;
      return modelCountMap[model] > 100;
    });

    removed = _.sampleSize(removed, excessDoodads);
    const removedSet = new Set(removed);
    doodads[0] = doodads[0].filter((d) => !removedSet.has(d));

    console.log('Eliminated', removed.length, 'doodads');
    console.log('Remaining doodads:', doodads[0].length);
  }

  console.log('Created', Object.keys(doodadsData.custom).length, `custom doodad types (${doodadTypesWithPitchRoll} with pitch&roll)`);
  console.log('Placed', doodads[0].length, 'doodad instances');

  const { unitObjectsData, unitPlacements, creatures } = await generateUnitsData(mapId, roots, wc3Terrain, config);
  console.log('Created', Object.keys(unitObjectsData.custom).length, 'custom unit types');
  console.log('Placed', unitPlacements.length, 'unit instances');

  return {
    wowObjectManager,
    war3Exporter,
    write: async ({ mapPath }: {mapPath: string}) => {
      assert.ok(mapPath.startsWith('maps/'));
      if (!existsSync(mapPath)) {
        fsExtra.copySync('maps/template-empty.w3x', mapPath);
      }
      wowObjectManager.assetManager.exportModels(mapPath);
      wowObjectManager.assetManager.exportTextures(mapPath);
      writeFileSync(
        path.join(mapPath, 'war3map.w3e'),
        TerrainTranslator.jsonToWar(wc3Terrain).buffer,
      );
      writeFileSync(
        path.join(mapPath, 'war3map.w3d'),
        ObjectsTranslator.jsonToWar(ObjectType.Doodads, doodadsData).buffer,
      );
      writeFileSync(
        path.join(mapPath, 'war3map.doo'),
        DoodadsTranslator.jsonToWar(doodads).buffer,
      );
      writeFileSync(
        path.join(mapPath, 'war3map.w3u'),
        ObjectsTranslator.jsonToWar(ObjectType.Units, unitObjectsData).buffer,
      );
      writeFileSync(
        path.join(mapPath, 'war3mapUnits.doo'),
        UnitsTranslator.jsonToWar(unitPlacements).buffer,
      );
      try {
        rmSync(path.join(mapPath, 'war3map.shd'));
      } catch (e) {
        // ignore
      }
      const start = performance.now();
      creatures.sort((a, b) => a.model.CreatureDisplayID - b.model.CreatureDisplayID);
      if (config.placeCreatures) {
        await exportCreatureModels(creatures, mapPath, {
          ...config,
          rawModelScaleUp: config.rawModelScaleUp * 2,
        });
      }
      console.log('Exported all creatures in', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');
      console.log('Done');
    },
  };
}

export async function generateUnitsData(
  mapId: number,
  roots: WowObject[],
  terrain: Terrain,
  config: Config,
) {
  const unitObjectsData: ObjectModificationTable = { original: {}, custom: {} };
  const unitPlacements: Unit[] = [];
  const creatures: Creature[] = [];

  // Global map params
  const mapMin: Vector3 = [
    terrain.map.offset.x,
    terrain.map.offset.y,
    dataHeightToGameZ(dataHeightMin),
  ];
  const mapMax: Vector3 = [
    terrain.map.offset.x + terrain.map.width * distancePerTile,
    terrain.map.offset.y + terrain.map.height * distancePerTile,
    dataHeightToGameZ(dataHeightMax),
  ];
  const mapSize = V3.sub(mapMax, mapMin);

  const { min, max } = computeAbsoluteMinMaxExtents(roots);
  const modelSize = V3.sub(max, min);
  const center = V3.mean(min, max);
  const scale = mapSize[0] / modelSize[0];
  console.log('Unit scale:', scale);

  const makeUnitType = (c: Creature): string => {
    const typeId = generateFourCC('lower').codeString;
    unitObjectsData.custom[`${typeId}:hfoo`] = [
      { id: 'unam', type: ModificationType.string, value: c.template.name || c.template.subname },
      // { id: 'upro', type: ModificationType.string, value: c.template.name || c.template.subname },
      { id: 'umdl', type: ModificationType.string, value: `creature-${c.model.CreatureDisplayID}.mdx` },
      { id: 'uabi', type: ModificationType.string, value: '' },
      { id: 'usca', type: ModificationType.real, value: scale * c.model.DisplayScale },
      { id: 'uhpm', type: ModificationType.int, value: c.creature.curhealth },
      { id: 'umpm', type: ModificationType.int, value: c.creature.curmana },
      { id: 'umpi', type: ModificationType.int, value: c.creature.curmana },
      { id: 'ulev', type: ModificationType.int, value: c.template.maxlevel },
    ];
    console.log(c.template.name, 'life', c.creature.curhealth, 'lvl', c.template.maxlevel);
    return typeId;
  };

  const templateIdToTypeId = new Map<number, string>();

  // Iterate each root to position its creatures
  for (const adtTile of roots) {
    if (!adtTile.id.includes('adt_')) continue;
    const m = adtTile.id.match(/adt_(\d+)_(\d+)/);
    if (!m) continue;
    const tileX = parseInt(m[1], 10);
    const tileY = parseInt(m[2], 10);

    const creaturesInTile = (await getCreaturesInTile(mapId, [tileX, tileY]))
      .filter((c) => !skipCreatures.some((s) => c.template.name.includes(s)));

    if (creaturesInTile.length === 0) continue;

    const { min: rootMin, max: rootMax } = computeAbsoluteMinMaxExtents([adtTile]);

    creaturesInTile.forEach((c) => {
      let pos = V3.scale([
        -c.creature.position_x,
        -c.creature.position_y,
        c.creature.position_z,
      ], config.rawModelScaleUp);
      pos = V3.sub(pos, center);
      pos = V3.rotate(pos, [0, 0, radians(-90 + mapAngle)]);
      const absPosition = V3.sum(pos, center);

      if (absPosition[0] < rootMin[0] - 1 || absPosition[0] > rootMax[0] + 1
        || absPosition[1] < rootMin[1] - 1 || absPosition[1] > rootMax[1] + 1) {
        console.error(c.template.name, 'is out of bounds', absPosition);
        console.log({ rootMin, rootMax });
        return;
      }

      const percent = [
        (absPosition[0] - min[0]) / modelSize[0],
        (absPosition[1] - min[1]) / modelSize[1],
        (absPosition[2] - min[2]) / modelSize[2],
      ];

      const inGameX = mapMin[0] + percent[0] * mapSize[0];
      const inGameY = mapMin[1] + percent[1] * mapSize[1];

      const inGameZ = dataHeightToGameZ(dataHeightMin
        + (dataHeightMax - dataHeightMin)
        / (config.terrainHeightClampPercent.upper - config.terrainHeightClampPercent.lower)
        * (percent[2] - config.terrainHeightClampPercent.lower));

      if (percent[2] < config.terrainHeightClampPercent.lower
        || percent[2] > config.terrainHeightClampPercent.upper) {
        console.log('Skip', c.template.name, 'because outside of allowed height range', percent[2], config.terrainHeightClampPercent.lower, config.terrainHeightClampPercent.upper);
        return;
      }

      if (!templateIdToTypeId.has(c.template.entry)) {
        templateIdToTypeId.set(c.template.entry, makeUnitType(c));
      }
      const typeId = templateIdToTypeId.get(c.template.entry)!;

      unitPlacements.push({
        type: typeId,
        variation: 0,
        position: [inGameX, inGameY, inGameZ],
        rotation: c.creature.orientation + adtTile.rotation[2] + radians(-90),
        scale: [1, 1, 1],
        skin: typeId,
        player: 0,
        hitpoints: 100,
        mana: 0,
        randomItemSetPtr: -1,
        droppedItemSets: [],
        gold: 0,
        targetAcquisition: -1,
        hero: {
          level: c.template.maxlevel ?? 1, str: 0, agi: 0, int: 0,
        },
        inventory: [],
        abilities: [],
        random: {
          type: 0, level: 0, itemClass: 0, groupIndex: 0, columnIndex: 0, unitSet: [],
        },
        color: 23,
        waygate: -1,
        id: unitPlacements.length,
      });
      creatures.push(c);
    });
  }

  return { unitObjectsData, unitPlacements, creatures };
}

void main().then(() => process.exit(0));

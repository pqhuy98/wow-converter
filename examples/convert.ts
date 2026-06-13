import 'dotenv/config';

import chalk from 'chalk';

import {
  defaultMapExportConfig, gameZToPercent, MapExportConfig, MapExporter,
} from '@/lib/converter/map-exporter/map-exporter';
import {
  autoChooseClampPercent,
  pruneDepth,
} from '@/lib/converter/map-exporter/map-generate-utils';
import { Vector2 } from '@/lib/math/common';
import { assertWowCascReady } from '@/lib/wow/wow-config-service';

import { Config, getDefaultConfig } from '../src/lib/global-config';

type WowMap = {
  id: number;
  folder: string;
};

const WowMap = {
  Azeroth: { id: 0, folder: 'azeroth' },
  Kalimdor: { id: 1, folder: 'kalimdor' },
  Northrend: { id: 571, folder: 'northrend' },
  Outland: { id: 530, folder: 'outland' },
  DeathKnightStart: { id: 609, folder: 'deathknightstart' },
  IcecrownCitadel: { id: 631, folder: 'icecrowncitadel' },
  TheMaw: { id: 2456, folder: '2456' },
  Durnhole: { id: 560, folder: 'HillsbradPast' },
  StratholmeRaid: { id: 533, folder: 'stratholmeraid' },
};

const maps: ([WowMap,
  Vector2, // low x, low y
  Vector2, // high x, high y
  string, // output map file name
  number, // lower percent
  number, // upper percent
  number, // map angle degrees
] | [string, number, number, string, number])[] = [
  // [WowMap.Northrend, [29, 22], [29, 23], 'wrathgate.w3x', 0.05, 0.3, 0],
  [WowMap.Northrend, [21, 27], [22, 28], 'valiancekeep.w3x', 0, 1, 0],
  // [WowMap.Northrend, [29, 15], [30, 18], 'icecrown.w3x', 0.63, 0.75, 180],
  // [WowMap.Northrend, [27, 20], [28, 21], 'icecrown.w3x', 0.65, 0.77, 180],
  // [WowMap.Northrend, [18, 24], [19, 25], 'nexus.w3x', 0, 1, 0],
  // [WowMap.DeathKnightStart, [41, 27], [43, 29], 'deathknightstart.w3x', 0, 1, 90],
  // [WowMap.IcecrownCitadel, [27, 32], [29, 33], 'icc-floor12.w3x'],
  // [WowMap.IcecrownCitadel, [25, 23], [27, 24], 'icc-floor34.w3x', 1 - 2 * 0.095, 1, 0],
  // [
  //   'world\\wmo\\dungeon\\icecrownraid\\icecrownraid_middle_section_set0.obj',
  //   0.55, 0.65, 'icc-floor34-wmo.w3x', 90,
  // ],
  // [WowMap.IcecrownCitadel, [35, 30], [36, 31], 'frozen-throne.w3x', 0.5, 0.7, 180],
  // [WowMap.Azeroth, [32, 48], [32, 4  8], 'northshire-abbey.w3x', 0, 1, 0],
  // [WowMap.Azeroth, [29, 26], [33, 29], 'undercity.w3x', 0, 1, 0],
  // [
  //   'world\\wmo\\lorderon\\undercity\\undercity_set0.obj',
  //   0, 0.20, 'undercity-indoor.w3x', -90,
  // ],
  // [WowMap.Azeroth, [34, 28], [35, 29], 'andorhal.w3x', 0, 1, 0],
  // [WowMap.StratholmeRaid, [37, 25], [39, 27], 'stratholmeraid.w3x', 0, 1, 0],
  // [
  //   'world\\wmo\\dungeon\\ld_stratholme\\stratholme_raid_set0.obj',
  //   0, 1, 'stratholmeraid-indoor.w3x', 90,
  // ],
  // [WowMap.Kalimdor, [28, 33], [29, 34], 'kalimdor-forest.w3x', 0, 1, 0],
  // [WowMap.Kalimdor, [31, 33], [33, 36], 'taurent-city.w3x', 0, 1, 0],
  // [WowMap.Durnhole, [27, 25], [32, 30], 'durnhole.w3x', 0, 1, 0],
  // [WowMap.TheMaw, [17, 18], [24, 24], 'themaw.w3x'],
  // [WowMap.TheMaw, [17, 19], [22, 23], 'themaw2.w3x'],
  // [WowMap.TheMaw, [19, 21], [22, 25], 'themaw3.w3x'],
  // [WowMap.TheMaw, [17, 21], [22, 25], 'themaw4.w3x'],
];

const autoChoseClampPercent = true;

const chosenMap = maps[0];

const config: Config = {
  ...await getDefaultConfig(),
  isBulkExport: true,
  overrideModels: false,
  mdx: true,
};

const creatureScaleUp = 2;
const mapOutputDir = `maps/${chosenMap[3]}`;

const mapExportConfig: MapExportConfig = {
  ...(chosenMap.length === 5 ? {
    ...defaultMapExportConfig,
    mapId: 0,
    wowExportFolder: '',
    min: [0, 0],
    max: [1, 1],
    mapAngleDeg: chosenMap[4],
    wmoSet: [chosenMap[0]],
    terrain: {
      clampPercent: {
        lower: chosenMap[1],
        upper: chosenMap[2],
      },
    },
  } : {
    ...defaultMapExportConfig,
    mapId: chosenMap[0].id,
    wowExportFolder: chosenMap[0].folder,
    min: chosenMap[1],
    max: chosenMap[2],
    mapAngleDeg: chosenMap[6],
    terrain: {
      clampPercent: {
        lower: chosenMap[4],
        upper: chosenMap[5],
      // lower: gameZToPercent(1400),
      // upper: gameZToPercent(2600),
      },
    },
  }),
  creatures: {
    enable: true,
    allAreDoodads: true,
  },
  unitScale: creatureScaleUp,
};

const depth = 3; // 1: adt only, 2: adt + wmo + top m2, 3: adt + wmo + top m2 + wmo interiors

(async function main() {
  await assertWowCascReady();
  const start = performance.now();
  const mapExporter = new MapExporter(config, mapExportConfig);

  await mapExporter.parseObjects();

  pruneDepth(mapExporter, depth);

  if (autoChoseClampPercent) {
    autoChooseClampPercent(mapExporter, mapExportConfig, creatureScaleUp);
  }

  await mapExporter.exportTerrainsDoodads(mapOutputDir);
  await mapExporter.exportCreatures(mapOutputDir);
  mapExporter.saveWar3mapFiles(mapOutputDir);
  console.log(`Total map export time: ${chalk.yellow(((performance.now() - start) / 1000).toFixed(2))} s`);
}())
  .then(() => process.exit(0))
  .catch((e) => {
    console.error(e);
    process.exit(1);
  });

gameZToPercent;

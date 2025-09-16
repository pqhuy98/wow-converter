import {
  defaultMapExportConfig, gameZToPercent, MapExportConfig, MapExporter,
} from '@/lib/converter/map-exporter/map-exporter';

import { Config, getDefaultConfig } from '../src/lib/global-config';

gameZToPercent;

type WowMap = {
  id: number;
  folder: string;
};

const WowMap = {
  Azeroth: { id: 0, folder: 'azeroth' },
  Northrend: { id: 571, folder: 'northrend' },
  Outland: { id: 530, folder: 'outland' },
  DeathKnightStart: { id: 609, folder: 'deathknightstart' },
  IcecrownCitadel: { id: 631, folder: 'icecrowncitadel' },
  TheMaw: { id: 2456, folder: '2456' },
};

const maps: [WowMap, [number, number], [number, number], string, number, number, number][] = [
  // [WowMap.Northrend, [28, 29], [22, 23], 'wrathgate.w3x'],
  // [WowMap.Northrend, [29, 15], [30, 18], 'icecrown.w3x', 0.63, 0.75, 180],
  // [WowMap.Northrend, [32, 21], [33, 22], 'icecrown.w3x', 0, 0.4, 0],
  [WowMap.DeathKnightStart, [41, 27], [43, 29], 'deathknightstart.w3x', 0.55, 0.77, 0],
  // [WowMap.IcecrownCitadel, [27, 32], [29, 33], 'icc-floor12.w3x'],
  // [WowMap.IcecrownCitadel, [25, 28], [21, 24], 'icc-floor34.w3x'],
  // [WowMap.IcecrownCitadel, [35, 30], [36, 31], 'frozen-throne.w3x', 0.9, 1, 180],
  // [WowMap.Azeroth, [32, 32], [48, 48], 'northshire-abbey.w3x'],
  // [WowMap.Azeroth, [30, 31], [27, 28], 'undercity.w3x'],
  // [WowMap.TheMaw, [17, 18], [24, 24], 'themaw.w3x'],
  // [WowMap.TheMaw, [17, 19], [22, 23], 'themaw2.w3x'],
  // [WowMap.TheMaw, [19, 21], [22, 25], 'themaw3.w3x'],
  // [WowMap.TheMaw, [17, 21], [22, 25], 'themaw4.w3x'],
];

const chosenMap = maps[0];
const mapAngleDeg = chosenMap[6];

const config: Config = {
  ...await getDefaultConfig(),
  isBulkExport: true,
  overrideModels: false,
  mdx: true,
};

const creatureScaleUp = 2;
const mapOutputDir = `maps/${chosenMap[3]}`;

const mapExportConfig: MapExportConfig = {
  ...defaultMapExportConfig,
  mapId: chosenMap[0].id,
  wowExportFolder: chosenMap[0].folder,
  min: chosenMap[1],
  max: chosenMap[2],
  mapAngleDeg,
  terrain: {
    clampPercent: {
      lower: chosenMap[4],
      upper: chosenMap[5],
      // lower: gameZToPercent(1400),
      // upper: gameZToPercent(2600),
    },
  },
  creatures: {
    enable: true,
    allAreDoodads: false,
    scaleUp: creatureScaleUp,
  },
};

(async function main() {
  const mapConverter = new MapExporter(config, mapExportConfig);

  await mapConverter.exportDoodadsAssets(mapOutputDir);
  await mapConverter.exportCreatures(mapOutputDir);
  mapConverter.saveWar3mapFiles(mapOutputDir);
}())
  .then(() => process.exit(0))
  .catch((e) => {
    console.error(e);
    process.exit(1);
  });

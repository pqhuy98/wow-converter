import chalk from 'chalk';

import { distancePerTile, maxGameHeightDiff } from '@/lib/constants';
import { isWowUnit } from '@/lib/converter/common/models';
import {
  defaultMapExportConfig, gameZToPercent, MapExportConfig, MapExporter,
} from '@/lib/converter/map-exporter/map-exporter';
import { computeRecommendedTerrainClampPercent } from '@/lib/converter/map-exporter/wc3-converter';
import { Vector3 } from '@/lib/math/common';
import { V3 } from '@/lib/math/vector';

import { Config, getDefaultConfig } from '../src/lib/global-config';

gameZToPercent;

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
};

const maps: [WowMap,
  [number, number], // low x, low y
  [number, number], // high x, high y
  string, // output map file name
  number, // lower percent
  number, // upper percent
  number, // map angle degrees
][] = [
  // [WowMap.Northrend, [28, 29], [22, 23], 'wrathgate.w3x'],
  // [WowMap.Northrend, [29, 15], [30, 18], 'icecrown.w3x', 0.63, 0.75, 180],
  // [WowMap.Northrend, [32, 21], [33, 22], 'icecrown.w3x', 0, 0.4, 0],
  // [WowMap.Northrend, [18, 24], [19, 25], 'nexus.w3x', 0, 1, 0],
  // [WowMap.DeathKnightStart, [41, 27], [43, 29], 'deathknightstart.w3x', 0, 1, 90],
  // [WowMap.IcecrownCitadel, [27, 32], [29, 33], 'icc-floor12.w3x'],
  // [WowMap.IcecrownCitadel, [25, 23], [27, 24], 'icc-floor34.w3x', 0, 1, 0],
  [WowMap.IcecrownCitadel, [35, 30], [36, 31], 'frozen-throne.w3x', 0.5, 0.7, 180],
  // [WowMap.Azeroth, [32, 48], [32, 48], 'northshire-abbey.w3x', 0, 1, 0],
  // [WowMap.Azeroth, [30, 31], [27, 28], 'undercity.w3x'],
  // [WowMap.Azeroth, [34, 28], [35, 29], 'andorhal.w3x', 0, 1, 0],
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
const mapAngleDeg = chosenMap[6];

const config: Config = {
  ...await getDefaultConfig(),
  isBulkExport: true,
  overrideModels: false,
  mdx: true,
};

const creatureScaleUp = 1;
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
    allAreDoodads: true,
    scaleUp: creatureScaleUp,
  },
};

(async function main() {
  const start = performance.now();
  const mapExporter = new MapExporter(config, mapExportConfig);

  await mapExporter.parseObjects();

  if (autoChoseClampPercent) {
    autoChooseClampPercent(mapExporter, mapExportConfig);
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

function autoChooseClampPercent(mapConverter: MapExporter, mapExportConfig: MapExportConfig) {
  const unitPos: Vector3[] = [];
  mapConverter.wowObjectManager.iterateObjects((obj, abs) => {
    if (!isWowUnit(obj)) return;
    unitPos.push(abs.position);
  });
  if (unitPos.length === 0) {
    console.log('No units found, cannot auto choose clamp percent. Defaulting to', mapExportConfig.terrain.clampPercent.lower, mapExportConfig.terrain.clampPercent.upper);
    return;
  }
  unitPos.sort((a, b) => a[2] - b[2]);
  const { ratio, min, max } = computeRecommendedTerrainClampPercent(mapConverter.wowObjectManager.roots);
  let clampDiff = ratio / creatureScaleUp;

  const size = V3.sub(max, min);
  const ratioZ = maxGameHeightDiff / (size[2] * clampDiff);
  const width = size[0] * ratioZ / distancePerTile;
  const height = size[1] * ratioZ / distancePerTile;

  const w4 = Math.ceil(width / 4) * 4;
  const h4 = Math.ceil(height / 4) * 4;
  clampDiff *= Math.max(1, w4 / 480, h4 / 480);

  const unitPosRatio = unitPos.map((pos) => (pos[2] - min[2]) / (max[2] - min[2]));

  // find [lower percent, upper percent = lower percent + ratio) so that maximize the number of unitPosRatio that are within the range
  let bestLowerPercent = 0;
  let bestUpperPercent = ratio;
  let maxCount = 0;
  const lower = mapExportConfig.terrain.clampPercent.lower;
  const upper = mapExportConfig.terrain.clampPercent.upper;
  if (upper - lower <= clampDiff) {
    console.log('Map terrain clamp is already within the recommended range, skipping auto choose.');
    return;
  }

  for (let lowerPercent = lower; lowerPercent <= upper - clampDiff; lowerPercent += 0.01) {
    const upperPercent = lowerPercent + clampDiff;
    const count = unitPosRatio.filter((ratio) => ratio >= lowerPercent && ratio <= upperPercent).length;
    if (count > maxCount) {
      maxCount = count;
      bestLowerPercent = lowerPercent;
      bestUpperPercent = upperPercent;
    }
  }
  mapExportConfig.terrain.clampPercent.lower = bestLowerPercent;
  mapExportConfig.terrain.clampPercent.upper = bestUpperPercent;
  const leftOutBelow = unitPosRatio.filter((ratio) => ratio < bestLowerPercent).length;
  const leftOutAbove = unitPosRatio.filter((ratio) => ratio > bestUpperPercent).length;
  const leftOut = leftOutBelow + leftOutAbove;
  const remaining = unitPosRatio.length - leftOut;
  console.log(`Chosen clamp percent: ${bestLowerPercent} - ${bestUpperPercent} (${remaining} units remaining)`);
  console.log(`Left out units: ${leftOut} (${leftOutBelow} below, ${leftOutAbove} above)`);
}

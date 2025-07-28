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
  IcecrownCitadel: { id: 631, folder: 'icecrowncitadel' },
};

const maps: [WowMap, [number, number], [number, number], string][] = [
  // [WowMap.Northrend, [28, 29], [22, 23], 'wrathgate.w3x'],
  [WowMap.Northrend, [29, 30], [18, 19], 'icecrown.w3x'],
  // [WowMap.IcecrownCitadel, [27, 28], [31, 33], 'icc-floor12.w3x'],
  // [WowMap.IcecrownCitadel, [25, 28], [21, 24], 'icc-floor34.w3x'],
  // [WowMap.IcecrownCitadel, [35, 36], [30, 31], 'frozen-throne.w3x'],
  // [WowMap.Azeroth, [32, 32], [48, 48], 'northshire-abbey.w3x'],
];

const chosenMap = maps[0];
const mapAngleDeg = 0;

const config: Config = {
  ...await getDefaultConfig(),
  overrideModels: false,
};

const creatureScaleUp = 2;

const mapExportConfig: MapExportConfig = {
  ...defaultMapExportConfig,
  mapId: chosenMap[0].id,
  wowExportFolder: chosenMap[0].folder,
  outputPath: `maps/${chosenMap[3]}`,
  min: chosenMap[1],
  max: chosenMap[2],
  mapAngleDeg,
  terrain: {
    clampPercent: {
      // lower: 0,
      // upper: 1,
      lower: gameZToPercent(800),
      upper: gameZToPercent(800) + 0.11459549564525706 * creatureScaleUp,
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

  await mapConverter.convert();
}())
  .then(() => process.exit(0))
  .catch((e) => {
    console.error(e);
    process.exit(1);
  });

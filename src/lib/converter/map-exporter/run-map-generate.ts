import { rmSync } from 'fs';
import path from 'path';

import { Config } from '@/lib/global-config';
import { assertWowCascReady } from '@/lib/wow/wow-config-service';

import {
  defaultMapExportConfig,
  MapExportConfig,
  MapExporter,
} from './map-exporter';
import {
  autoChooseClampPercent,
  computeCreatureExportSteps,
  countUniqueUnitExportsFromManager,
  logMapGeneratePhase,
  normalizeMapSaveName,
  pruneDepth,
} from './map-generate-utils';

const DEFAULT_PRUNE_DEPTH = 3;

export interface MapGenerateConversionOptions {
  config: Config;
  mapExportConfig: MapExportConfig;
  mapSaveName: string;
  freshExport: boolean;
  autoClampPercent?: boolean;
  creatureScaleUp: number;
  onConvertStepsKnown?: (convertSteps: number) => void;
  onProgress?: (
    convertCompletedSteps: number,
    taskName: string,
    creatureProgress?: { completed: number; total: number },
  ) => void;
}

export interface MapGenerateConversionResult {
  outputDir: string;
  mapSaveName: string;
  convertSteps: number;
}

export async function runMapGenerateConversion(
  options: MapGenerateConversionOptions,
): Promise<MapGenerateConversionResult> {
  const {
    config,
    mapExportConfig,
    mapSaveName: rawSaveName,
    freshExport,
    autoClampPercent = false,
    creatureScaleUp,
    onConvertStepsKnown,
    onProgress,
  } = options;

  const mapSaveName = normalizeMapSaveName(rawSaveName);
  const outputDir = path.join('maps', mapSaveName);

  await assertWowCascReady();

  if (freshExport) {
    try {
      rmSync(outputDir, { recursive: true, force: true });
      logMapGeneratePhase(`Removed existing map folder ${outputDir}`);
    } catch {
      // ignore
    }
  }

  const conversionConfig: Config = {
    ...config,
    isBulkExport: true,
    overrideModels: freshExport,
    mdx: true,
  };

  let convertCompleted = 0;
  const report = (
    taskName: string,
    creatureProgress?: { completed: number; total: number },
  ) => {
    onProgress?.(convertCompleted, taskName, creatureProgress);
  };

  const mapExporter = new MapExporter(conversionConfig, mapExportConfig);

  logMapGeneratePhase('Parsing map objects');
  report('Parsing map data');
  await mapExporter.parseObjects();
  pruneDepth(mapExporter, DEFAULT_PRUNE_DEPTH);

  if (autoClampPercent) {
    autoChooseClampPercent(mapExporter, mapExportConfig, creatureScaleUp);
  }

  convertCompleted = 1;
  report('Parsed map data');

  const uniqueCreatureCount = mapExportConfig.creatures.enable
    ? countUniqueUnitExportsFromManager(mapExporter.wowObjectManager)
    : 0;
  const creatureExportSteps = computeCreatureExportSteps(uniqueCreatureCount);
  const convertSteps = 1 + 1 + creatureExportSteps + 1;
  onConvertStepsKnown?.(convertSteps);

  logMapGeneratePhase('Saving terrain and doodads');
  report('Saving terrain and doodads');
  await mapExporter.exportTerrainsDoodads(outputDir);
  convertCompleted = 2;
  report('Saved terrain and doodads');

  if (mapExportConfig.creatures.enable && creatureExportSteps > 0) {
    logMapGeneratePhase('Exporting creatures');
    report('Exporting creatures', { completed: 0, total: uniqueCreatureCount });
    await mapExporter.exportCreatures(outputDir, {
      onCreatureProgress: (completed, total) => {
        convertCompleted = 2 + completed;
        report('Exporting creatures', { completed, total });
      },
    });
    convertCompleted = 2 + creatureExportSteps;
    report('Exported creatures');
  }

  logMapGeneratePhase('Saving war3map files');
  report('Saving war3map files');
  mapExporter.saveWar3mapFiles(outputDir);
  convertCompleted = convertSteps;
  report('Complete');

  return { outputDir, mapSaveName, convertSteps };
}

export function buildMapExportConfig(params: {
  mapId: number;
  wowExportFolder: string;
  min: [number, number];
  max: [number, number];
  mapAngleDeg: number;
  clampLower: number;
  clampUpper: number;
  creatures: MapExportConfig['creatures'];
}): MapExportConfig {
  return {
    ...defaultMapExportConfig,
    mapId: params.mapId,
    wowExportFolder: params.wowExportFolder,
    min: params.min,
    max: params.max,
    mapAngleDeg: params.mapAngleDeg,
    terrain: {
      clampPercent: {
        lower: params.clampLower,
        upper: params.clampUpper,
      },
    },
    creatures: params.creatures,
  };
}

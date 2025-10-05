import chalk from 'chalk';
import { existsSync, readdirSync, rmSync } from 'fs';
import fsExtra from 'fs-extra';
import _ from 'lodash';
import path from 'path';

import { exportCreatureModels } from '@/lib/azerothcore-client/creatures';
import { dataHeightMin, dataHeightToGameZ, maxGameHeightDiff } from '@/lib/constants';
import { WowObjectType } from '@/lib/converter/common/models';
import { WowObjectManager } from '@/lib/converter/common/wow-object-manager';
import { Wc3Converter } from '@/lib/converter/map-exporter/wc3-converter';
import { Config } from '@/lib/global-config';
import { Vector2 } from '@/lib/math/common';
import { radians } from '@/lib/math/rotation';
import { MapManager } from '@/vendors/wc3maptranslator/extra/map-manager';

export interface MapExportConfig {
  mapId: number;
  wowExportFolder: string;
  min: Vector2;
  max: Vector2;
  mapAngleDeg: number;

  terrain: {
    clampPercent: {
      upper: number,
      lower: number;
    },
  },

  doodads: {
    enable: Record<WowObjectType | 'others', boolean>;
    pitchRollThresholdRadians: number
  },

  creatures: {
    enable: boolean
    allAreDoodads: boolean;
    scaleUp: number
  },

}

export const defaultMapExportConfig: Omit<MapExportConfig, 'mapId' | 'wowExportFolder' | 'outputPath' | 'min' | 'max' | 'mapAngleDeg' > = {
  terrain: {
    clampPercent: {
      upper: 1,
      lower: 0,
    },
  },
  doodads: {
    enable: {
      adt: true,
      wmo: true,
      m2: true,
      gobj: true,
      others: true,
      unit: true,
    },
    pitchRollThresholdRadians: radians(5 /* degrees */),
  },
  creatures: {
    enable: true,
    allAreDoodads: false,
    scaleUp: 1,
  },
};

export class MapExporter {
  mapManager: MapManager;

  wowObjectManager: WowObjectManager;

  private filterDoodads: (id: string, type: WowObjectType) => boolean;

  constructor(public config: Config, public mapExportConfig: MapExportConfig) {
    this.mapManager = new MapManager();
  }

  public async parseObjects() {
    const {
      wowExportFolder, min, max, mapAngleDeg, mapId,
    } = this.mapExportConfig;

    this.wowObjectManager = new WowObjectManager(this.config);
    this.filterDoodads = (_id, type) => (this.mapExportConfig.doodads.enable[type] ?? this.mapExportConfig.doodads.enable.others) && type !== 'unit';
    await this.wowObjectManager.readTerrainsDoodads(
      buildPaths(`**/${wowExportFolder}`, min, max),
      this.filterDoodads,
    );

    if (this.mapExportConfig.creatures.enable) {
      await this.wowObjectManager.readCreatures(mapId);
    }

    console.log('Total objects:', this.wowObjectManager.objects.size);
    const typeCountMap = _([...this.wowObjectManager.objects.values()])
      .map((o) => o.type)
      .countBy();
    console.log('Object type count:', typeCountMap.entries().toJSON());

    // Rotate all root objects
    console.log(`Rotating roots at center by ${mapAngleDeg} degrees`);
    this.wowObjectManager.rotateRootsAtCenter([0, 0, radians(-90 + mapAngleDeg)]);
  }

  public async exportTerrainsDoodads(outputDir: string) {
    const mapConfig = this.mapExportConfig;

    const wc3Converter = new Wc3Converter(mapConfig);
    this.mapManager.terrain = wc3Converter.generateTerrainWithHeight(this.wowObjectManager);

    // Place doodads
    const { doodadTypesWithPitchRoll } = wc3Converter.placeDoodads(
      this.mapManager,
      this.wowObjectManager,
      (obj) => this.filterDoodads(obj.id, obj.type),
    );

    console.log('Total doodads:', this.mapManager.doodads.length);
    console.log('Total doodad types:', this.mapManager.doodadTypes.length);
    console.log('Doodad types with custom pitch/roll:', doodadTypesWithPitchRoll);
    if (this.mapManager.doodads.length > 130_000) {
      throw new Error(`Too many doodads: ${this.mapManager.doodads.length}, limit is 130_000`);
    }

    // Export doodad assets
    await this.wowObjectManager.assetManager.exportTextures(outputDir);
    await this.wowObjectManager.assetManager.exportModels(outputDir);
  }

  public async exportCreatures(outputDir: string) {
    const mapConfig = this.mapExportConfig;

    // Place creatures as Units or Doodads
    if (mapConfig.creatures.enable) {
      const wc3Converter = new Wc3Converter(mapConfig);
      const units = wc3Converter.placeUnits(this.mapManager, this.wowObjectManager);
      console.log('Created', this.mapManager.unitTypes.length, 'custom unit types');
      console.log('Placed', this.mapManager.units.length, 'unit instances');

      // Export creature assets
      const start = performance.now();
      units.sort((a, b) => a.creature.model.CreatureDisplayID - b.creature.model.CreatureDisplayID);
      await exportCreatureModels(units.map((u) => u.creature), outputDir, this.config);
      console.log('Exported all unit assets in', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');
      console.log('Done');
    }
  }

  public saveWar3mapFiles(outputDir: string) {
    // Save map
    const templateEmptyDir = 'maps/template-empty.w3x';
    if (!existsSync(outputDir)) {
      fsExtra.copySync(templateEmptyDir, outputDir);
    } else {
      // copy all files in template-empty.w3x to outputDir if not already exists
      const allFiles = readdirSync(templateEmptyDir);
      for (const file of allFiles) {
        if (!existsSync(path.join(outputDir, file))) {
          fsExtra.copySync(path.join(templateEmptyDir, file), path.join(outputDir, file));
        }
      }
    }
    this.mapManager.save(outputDir);
    try {
      // Remove precomputed shadow file if it exists, since it no longers match objects on the map
      rmSync(path.join(outputDir, 'war3map.shd'));
    } catch (e) {
      // ignore
    }
  }
}

function buildPaths(prefix: string, min: Vector2, max: Vector2) {
  const res: string[] = [];
  for (let i = min[0]; i <= max[0]; i++) {
    for (let j = min[1]; j <= max[1]; j++) {
      res.push(path.join(prefix, `adt_${i}_${j}.obj`));
    }
  }
  return res;
}

export function gameZToPercent(z: number) {
  return (z - dataHeightToGameZ(dataHeightMin)) / maxGameHeightDiff;
}

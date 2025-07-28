import assert from 'assert';
import chalk from 'chalk';
import { existsSync, rmSync } from 'fs';
import fsExtra from 'fs-extra';
import _ from 'lodash';
import path from 'path';

import { Creature, exportCreatureModels, getCreaturesInTile } from '@/lib/azerothcore-client/creatures';
import {
  dataHeightMax, dataHeightMin, dataHeightToGameZ, distancePerTile, maxGameHeightDiff,
} from '@/lib/constants';
import { computeAbsoluteMinMaxExtents } from '@/lib/converter/common/model-manager';
import { WowObject, WowObjectType } from '@/lib/converter/common/models';
import { WowObjectManager } from '@/lib/converter/common/wow-object-manager';
import { getTerrainHeight, Wc3Converter } from '@/lib/converter/map-exporter/wc3-converter';
import { Config } from '@/lib/global-config';
import { EulerRotation, Vector2, Vector3 } from '@/lib/math/common';
import { degrees, radians } from '@/lib/math/rotation';
import { V3 } from '@/lib/math/vector';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { ModificationType } from '@/vendors/wc3maptranslator/data/ObjectModificationTable';
import { IDoodadType, IUnitType, MapManager } from '@/vendors/wc3maptranslator/extra/map-manager';

export interface MapExportConfig {
  mapId: number;
  wowExportFolder: string;
  outputPath: string;
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
  private globalRotation: EulerRotation;

  constructor(public config: Config, public mapExportConfig: MapExportConfig) {
    this.globalRotation = [0, 0, radians(-90 + mapExportConfig.mapAngleDeg)];
  }

  public async convert() {
    const {
      mapId, wowExportFolder, outputPath, min, max, mapAngleDeg,
    } = this.mapExportConfig;
    const mapConfig = this.mapExportConfig;
    assert.ok(outputPath.startsWith('maps/'));

    await wowExportClient.waitUntilReady();

    const doodadFilter = (_file: string, type: WowObjectType) => mapConfig.doodads.enable[type] ?? mapConfig.doodads.enable.others;

    // Parse all objects
    const wowObjectManager = new WowObjectManager(this.config);
    await wowObjectManager.parse(
      buildPaths(`**/${wowExportFolder}`, min, max),
      doodadFilter,
    );

    console.log('Total objects:', wowObjectManager.objects.size);
    const typeCountMap = _([...wowObjectManager.objects.values()])
      .map((o) => o.type)
      .countBy();
    console.log('Type count:', typeCountMap.entries().toJSON());

    // Rotate all root objects
    console.log(`Rotating roots at center by ${mapAngleDeg} degrees`);
    wowObjectManager.rotateRootsAtCenter(this.globalRotation);

    // Generate terrain
    const roots = wowObjectManager.roots;
    const mapManager = new MapManager(outputPath);
    const war3Exporter = new Wc3Converter(mapConfig);
    mapManager.terrain = war3Exporter.generateTerrainWithHeight(roots);

    // Place doodads
    const { doodadTypesWithPitchRoll } = war3Exporter.placeDoodads(
      mapManager,
      roots,
      (doodad) => doodadFilter(doodad.id, doodad.type),
    );

    console.log('Total doodads:', mapManager.doodads.length);
    console.log('Total doodad types:', mapManager.doodadTypes.length);
    console.log('Doodad types with custom pitch/roll:', doodadTypesWithPitchRoll);
    if (mapManager.doodads.length > 130_000) {
      throw new Error(`Too many doodads: ${mapManager.doodads.length}, limit is 130_000`);
    }

    // Place creatures as Units or Doodads
    let creatures: Creature[] = [];
    if (mapConfig.creatures.enable) {
      creatures = await this.generateUnitsData(mapManager, mapId, roots);
      console.log('Created', mapManager.unitTypes.length, 'custom unit types');
      console.log('Placed', mapManager.units.length, 'unit instances');
    }

    // Save map
    if (!existsSync(outputPath)) {
      fsExtra.copySync('maps/template-empty.w3x', outputPath);
    }
    mapManager.save();
    try {
      // Remove precomputed shadow file if it exists, since it no longers match objects on the map
      rmSync(path.join(outputPath, 'war3map.shd'));
    } catch (e) {
      // ignore
    }

    // Export assets
    wowObjectManager.assetManager.exportModels(outputPath);
    await wowObjectManager.assetManager.exportTextures(outputPath);
    const start = performance.now();
    creatures.sort((a, b) => a.model.CreatureDisplayID - b.model.CreatureDisplayID);
    if (mapConfig.creatures.enable) {
      await exportCreatureModels(creatures, outputPath, this.config);
    }
    console.log('Exported all creatures in', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');
    console.log('Done');
  }

  private async generateUnitsData(
    mapManager: MapManager,
    wowExportMapId: number,
    roots: WowObject[],
  ) {
    const debug = false;
    const mapConfig = this.mapExportConfig;
    const terrain = mapManager.terrain;

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

    const templateIdToUnitType = new Map<number, IUnitType>();
    const templateIdToDoodadType = new Map<number, IDoodadType>();

    // Iterate each root to position its creatures
    for (const adtTile of roots) {
      if (!adtTile.id.includes('adt_')) continue;
      const m = adtTile.id.match(/adt_(\d+)_(\d+)/);
      if (!m) continue;
      const tileX = parseInt(m[1], 10);
      const tileY = parseInt(m[2], 10);

      const creaturesInTile = (await getCreaturesInTile(
        wowExportMapId,
        [tileX, tileY],
        {
          phaseMask: 1,
        },
      ));

      if (creaturesInTile.length === 0) {
        console.log('No creatures in tile', adtTile.id);
        continue;
      } else {
        console.log('Found', creaturesInTile.length, 'creatures in tile', adtTile.id);
      }

      creaturesInTile.forEach((c) => {
        let pos = V3.scale([
          -c.creature.position_x,
          -c.creature.position_y,
          c.creature.position_z,
        ], this.config.rawModelScaleUp);
        pos = V3.sub(pos, center);
        pos = V3.rotate(pos, this.globalRotation);
        const absPosition = V3.sum(pos, center);

        if (absPosition[0] < min[0] - 1 || absPosition[0] > max[0] + 1
          || absPosition[1] < min[1] - 1 || absPosition[1] > max[1] + 1) {
          console.error('Creature', c.template.name, 'is out of bounds', absPosition);
          console.log({ min, max });
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
          / (mapConfig.terrain.clampPercent.upper - mapConfig.terrain.clampPercent.lower)
          * (percent[2] - mapConfig.terrain.clampPercent.lower));

        const terrainZ = dataHeightToGameZ(getTerrainHeight(terrain, percent[0], percent[1]));

        const creatureModel = `creature-${c.model.CreatureDisplayID}.mdx`;
        const creatureName = c.template.name || c.template.subname;
        const creatureScale = scale * c.model.DisplayScale * mapConfig.creatures.scaleUp;
        const creatureFacingRadians = c.creature.orientation + adtTile.rotation[2] + radians(-90);
        const position: Vector3 = [inGameX, inGameY, inGameZ];

        const withinPlayableZone = percent[2] >= mapConfig.terrain.clampPercent.lower
          && percent[2] <= mapConfig.terrain.clampPercent.upper;
        const notOnGround = inGameZ < terrainZ - 100 || inGameZ > terrainZ + 100;

        if (mapConfig.creatures.allAreDoodads || !withinPlayableZone || notOnGround) {
          // Creature is out of playable map zone or not on ground, add it as doodad

          debug && console.log('Add', c.template.name, 'as destructible because of', mapConfig.creatures.allAreDoodads ? 'overridden' : 'outside of allowed zone');

          if (!templateIdToDoodadType.has(c.template.entry)) {
            templateIdToDoodadType.set(c.template.entry, mapManager.addDoodadType([
              { id: 'bnam', type: ModificationType.string, value: `~U ${creatureName}` },
              { id: 'bfil', type: ModificationType.string, value: creatureModel },
              { id: 'bmas', type: ModificationType.unreal, value: creatureScale * 1.5 },
              { id: 'bmis', type: ModificationType.unreal, value: creatureScale / 1.5 },
            ], true));
          }
          const doodadType = templateIdToDoodadType.get(c.template.entry)!;

          mapManager.addDoodad(doodadType, {
            id: 0,
            variation: 0,
            position,
            angle: degrees(creatureFacingRadians),
            scale: [creatureScale, creatureScale, creatureScale],
            skinId: doodadType.code,
            flags: {
              visible: true,
              solid: true,
              customHeight: true,
            },
            life: 100,
            randomItemSetPtr: -1,
            droppedItemSets: [],
          });
        } else {
          // Creature is inside playable map zone, add it as unit

          if (!templateIdToUnitType.has(c.template.entry)) {
            templateIdToUnitType.set(c.template.entry, mapManager.addUnitType('unit', 'hfoo', [
              { id: 'unam', type: ModificationType.string, value: creatureName },
              // { id: 'upro', type: ModificationType.string, value: c.template.name || c.template.subname },
              { id: 'umdl', type: ModificationType.string, value: creatureModel },
              { id: 'uabi', type: ModificationType.string, value: '' },
              { id: 'usca', type: ModificationType.real, value: creatureScale },
              { id: 'uhpm', type: ModificationType.int, value: c.creature.curhealth },
              { id: 'umpm', type: ModificationType.int, value: c.creature.curmana },
              { id: 'umpi', type: ModificationType.int, value: c.creature.curmana },
              { id: 'ulev', type: ModificationType.int, value: c.template.maxlevel },
            ]));
          }
          const unitType = templateIdToUnitType.get(c.template.entry)!;

          mapManager.addUnit(unitType, {
            variation: 0,
            position,
            rotation: creatureFacingRadians,
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
              level: c.template.maxlevel ?? 1, str: 0, agi: 0, int: 0,
            },
            inventory: [],
            abilities: [],
            random: {
              type: 0, level: 0, itemClass: 0, groupIndex: 0, columnIndex: 0, unitSet: [],
            },
            color: 23,
            waygate: -1,
            id: 0,
          });
        }
        creatures.push(c);
      });
    }

    return creatures;
  }
}

function buildPaths(prefix: string, x: [number, number], y: [number, number]) {
  const res: string[] = [];
  for (let i = x[0]; i <= x[1]; i++) {
    for (let j = y[0]; j <= y[1]; j++) {
      res.push(path.join(prefix, `adt_${i}_${j}.obj`));
    }
  }
  return res;
}

export function gameZToPercent(z: number) {
  return (z - dataHeightToGameZ(dataHeightMin)) / maxGameHeightDiff;
}

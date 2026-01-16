import { readFileSync, writeFileSync } from 'fs';
import { cloneDeep } from 'lodash';
import { join } from 'path';

import { dataHeightMin, setDataHeightLimit } from '@/lib/constants';
import { AssetManager } from '@/lib/converter/common/asset-manager';
import { defaultMapExportConfig, MapExportConfig, MapExporter } from '@/lib/converter/map-exporter/map-exporter';
import { MDL } from '@/lib/formats/mdl/mdl';
import { Config, getDefaultConfig } from '@/lib/global-config';
import { matchTerrainToDoodadHeights } from '@/lib/mapmodifier/terrain-height-matcher';
import { calculateTriangleSlope } from '@/lib/math/geometry';

(async function main() {
  const config: Config = {
    ...await getDefaultConfig(),
    isBulkExport: true,
    overrideModels: true,
    mdx: true,
  };

  const mapExportConfig: MapExportConfig = {
    ...defaultMapExportConfig,
    mapId: 0,
    wowExportFolder: '',
    min: [0, 0],
    max: [1, 1],
    mapAngleDeg: 90,
    wmoSet: ['world\\wmo\\dungeon\\ld_stratholme\\stratholme_raid_set0.obj'],
    terrain: {
      clampPercent: {
        lower: 0.2,
        upper: 1,
      },
    },
    doodads: defaultMapExportConfig.doodads,
    creatures: {
      enable: false,
      allAreDoodads: false,
      scaleUp: 2,
    },
  };

  const mapDir = './maps/naxxramas.w3x';

  setDataHeightLimit(512, 8192 * 2 - 1512);

  // 2) Parse Wrathgate into a separate MapManager (source) and export assets into the target map directory
  const exporter = new MapExporter(config, mapExportConfig);
  await exporter.parseObjects();

  // export assets into the existing map folder
  await exporter.exportTerrainsDoodads(mapDir);
  // await floor3Exporter.exportCreatures(targetMapAssetDir);

  // Set terrain height
  const sourceManager = exporter.mapManager;
  const tmap = sourceManager.mapData.terrain.groundHeight;
  for (const row of tmap) {
    for (let i = 0; i < row.length; i++) {
      row[i] = dataHeightMin;
    }
  }
  const raidType = sourceManager.doodadTypes.find((dt) => dt.data.some((d) => d.id === 'dfil' && (d.value as string).includes('stratholme_raid_set0')))!;

  const raid = sourceManager.doodads.find((d) => d.type === raidType)!;

  // undercityType.data.find((d) => d.id === 'dfil')!.value = 'model.mdl';

  const ground = readFileSync('./examples/experiments/resources/naxxramas-floor.mdl', 'utf8');
  matchTerrainToDoodadHeights(sourceManager.mapData.terrain, [
    [raid, ground],
  ], {
    floodBrushSize: 10,
    slopeThreshold: 45,
  });

  // 4) Save the updated map in place
  exporter.saveWar3mapFiles(mapDir);
  sourceManager.mapData.save('doodads');
  sourceManager.mapData.save('terrain');

  process.exit(0);
}()).catch((e) => {
  // eslint-disable-next-line no-console
  console.error(e);
  process.exit(1);
});

async function buildUndercityLower(assetManager: AssetManager, outputPath: string) {
  const model = (await assetManager.parse('world\\wmo\\dungeon\\ld_stratholme\\stratholme_raid_set0.obj', true)).mdl;
  model.modify.deleteVerticesInsideBox([-99999, -99999, -791.26], [99999, 18062.16, 99999]);

  const floor = cloneDeep(model).modify.deleteFacesIf((f) => {
    const slope = calculateTriangleSlope([f.vertices[0].position, f.vertices[1].position, f.vertices[2].position]);
    return slope > 20;
  })
    .deleteVerticesInsideBox([-99999, -99999, -6748.07], [99999, 99999, 99999]);

  // dragon entry
  model.modify.setInfiniteBounds();
  model.modify.optimizeAll();

  write({ model });
  write({ floor: floor.mdl });
  await assetManager.exportTextures(outputPath);

  function write(x: {[name: string]: MDL}) {
    const name = Object.keys(x)[0];
    const mdl = x[name];
    mdl.sync();
    // writeFileSync(join(outputPath, `${name}.mdx`), mdl.toMdx());
    writeFileSync(join(outputPath, `${name}.mdl`), mdl.toMdl());
  }

  return { model, floor };
}

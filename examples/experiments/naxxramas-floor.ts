import { writeFileSync } from 'fs';
import { join } from 'path';

import { AssetManager } from '@/lib/converter/common/asset-manager';
import { MDLModify } from '@/lib/formats/mdl/modify';
import { getDefaultConfig } from '@/lib/global-config';
import { calculateTriangleSlope } from '@/lib/math/geometry';

const outputPath = '../wow-converter/exported-assets';
const assetManager = new AssetManager(await getDefaultConfig());

const model = (await assetManager.parse('world\\wmo\\dungeon\\ld_stratholme\\stratholme_raid_set0.obj', true)).mdl;
write({ naxxramas_full: model.modify });

const floor = model.modify.deleteFacesIf((f, geoset) => {
  if (geoset.material.layers[0].texture.image.includes('jlo_udercity_floor')) {
    return false;
  }
  const slope = calculateTriangleSlope([f.vertices[0].position, f.vertices[1].position, f.vertices[2].position]);
  // if (
  //   geoset.name.includes('scourgehallintersect')
  //   || geoset.name.includes('scourgehallstair')
  //   || geoset.name.includes('princehallRbottom')
  //   || geoset.name.includes('princehallLbottom')
  // ) return slope > 40;
  return slope > 20;
});

floor.deleteVerticesInsideBox([-33289.264, -31737.97, 621.715], [33833.426, -8984.699, 33556.385]);
floor.deleteVerticesInsideBox([-9502.622, -18892.66, -79.063], [72643.274, 39834.499, 33556.385]);
floor.deleteVerticesInsideBox([-55785.485, -18892.66, -79.063], [-13463.136, 39834.499, 33556.385]);

write({ floor });

// Find and delete connected components of faces that has area less than 1000
floor.removeSmallFaceComponents(10000);

// Anub'Rekhan
// const anubRekhan = cloneDeep(model).modify.deleteVerticesOutsideBox([-21033.869, -1510.559, -33837.778], [-10951.421, 6773.778, 48690.147]);
// write({ anubRekhan });

// // Noth
// const noth = cloneDeep(model).modify.deleteVerticesOutsideBox([14682.666, 442.53, -33837.778], [21735.966, 7297.515, -26.088]);
// write({ noth });

// // Heigan
// const heigan = cloneDeep(model).modify.deleteVerticesOutsideBox([10069.134, 11243.487, -33837.778], [16001.382, 17108.816, 1032.755]);
// write({ heigan });

// const corner3way = cloneDeep(model).modify.deleteVerticesOutsideBox([21357.956, 2044.728, -33837.778], [28859.658, 8761.768, 1032.755]);
// write({ corner3way });

await assetManager.exportTextures(outputPath);

function write(x: {[name: string]: MDLModify}) {
  const name = Object.keys(x)[0];
  const mdl = x[name];
  mdl.mdl.modify.optimizeAll();
  mdl.mdl.sync();
  // mdl.mdl.modify.centerModelMinZ();
  writeFileSync(join(outputPath, `${name}.mdl`), mdl.mdl.toMdl());
  // writeFileSync(join(outputPath, name+".mdl"), model.toString())
}

process.exit(0);

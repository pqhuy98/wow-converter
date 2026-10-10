import assert from 'assert';
import { existsSync, readFileSync, writeFileSync } from 'fs';
import path from 'path';

import { duplicateMapSouthRotated, duplicateRasterSouthRotated } from '../src/lib/mapmodifier/duplicate-south';
import { MapTranslator } from '../src/vendors/wc3maptranslator/translators';

const mapDir = process.argv[2];
assert(mapDir, 'Usage: bun scripts/duplicate-map-south.ts <unpacked-map-directory>');
const map = new MapTranslator();
map.load(mapDir);
const { width, height } = map.terrain.map;
// Build every output before replacing any file. Back up the map before running.
const rasters = ['war3map.wpm', 'war3map.shd'].filter(name => existsSync(path.join(mapDir, name))).map(name => ({
  name,
  buffer: duplicateRasterSouthRotated(readFileSync(path.join(mapDir, name)), width, height, name.endsWith('.wpm')),
}));
duplicateMapSouthRotated(map);
// Validate serializers before the first write; object data and imports are untouched.
const { TerrainTranslator, InfoTranslator, UnitsTranslator, DoodadsTranslator, RegionsTranslator, CamerasTranslator } = await import('../src/vendors/wc3maptranslator/translators');
const outputs = [
  { name: 'war3map.w3e', buffer: TerrainTranslator.jsonToWar(map.terrain).buffer },
  { name: 'war3map.w3i', buffer: InfoTranslator.jsonToWar(map.info).buffer },
  { name: 'war3mapUnits.doo', buffer: UnitsTranslator.jsonToWar(map.units, map.formatVersions.units).buffer },
  { name: 'war3map.doo', buffer: DoodadsTranslator.jsonToWar([map.doodads, map.specialDoodads], map.formatVersions.doodads).buffer },
  { name: 'war3map.w3r', buffer: RegionsTranslator.jsonToWar(map.regions, map.formatVersions.regions).buffer },
  { name: 'war3map.w3c', buffer: CamerasTranslator.jsonToWar(map.cameras, map.formatVersions.cameras).buffer },
  ...rasters,
];
for (const { name, buffer } of outputs) writeFileSync(path.join(mapDir, name), buffer);
console.log(`Expanded ${width}x${height} to ${width}x${2 * height}: ${map.doodads.length} doodads/destructibles, ${map.units.length} units, ${map.regions.length} regions. Reopen and save in World Editor to regenerate the script and minimap.`);
process.exit(0);

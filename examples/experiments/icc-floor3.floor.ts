import { readFileSync, writeFileSync } from 'fs';

import { matchTerrainToDoodadHeights } from '@/lib/mapmodifier/terrain-height-matcher';
import { DoodadsTranslator, TerrainTranslator } from '@/vendors/wc3maptranslator';

function main() {
  const terrain = TerrainTranslator.warToJson(readFileSync('./maps/icc-floor34-wmo.w3x/war3map copy.w3e')).json;
  const doodads = DoodadsTranslator.warToJson(readFileSync('./maps/icc-floor34-wmo.w3x/war3map.doo')).json;

  const icc = doodads[0].find((d) => d.type === 'aaaa')!;
  console.log(doodads[0]);
  const ground = readFileSync('./exported-assets/floor.mdl', 'utf-8');

  matchTerrainToDoodadHeights(terrain, [
    [icc, ground],
  ]);

  writeFileSync('./maps/icc-floor34-wmo.w3x/war3map.w3e', TerrainTranslator.jsonToWar(terrain).buffer);
}
main();

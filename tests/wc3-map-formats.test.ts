import assert from 'assert';
import { test } from 'bun:test';
import { readFileSync } from 'fs';
import path from 'path';

import { duplicateMapSouthRotated, duplicateRasterSouthRotated } from '../src/lib/mapmodifier/duplicate-south';
import { getInitialTerrain } from '../src/lib/mapmodifier/terrain';
import { defaultInfo } from '../src/vendors/wc3maptranslator/extra/default-info';
import {
  CamerasTranslator, DoodadsTranslator, InfoTranslator, MapTranslator, RegionsTranslator, TerrainTranslator, UnitsTranslator,
} from '../src/vendors/wc3maptranslator/translators';

test('modern placements preserve nonzero extra fields and zero-valued overrides', () => {
  const doodads = DoodadsTranslator.jsonToWar([[{
    type: 'LTlt', skinId: 'LTlt', variation: 0, position: [1, 2, 3], angle: 37.5, scale: [1, 2, 3], flags: { visible: true, solid: true, customHeight: false }, life: 0, randomItemSetPtr: -1, droppedItemSets: [], id: 17,
    groupId: 41, unknown1: -7, roll: 0.25, pitch: -0.75, state: 5,
    lights: [{ index: 2, shadowCasting: 1, color: -1, intensity: 3.5, shadowCastingStart: 1, shadowCastingEnd: 2, quadraticFalloff: 4, linearFalloff: 5, damping: 6 }],
  }], []], 13).buffer;
  const parsed = DoodadsTranslator.warToJson(doodads);
  assert.equal(parsed.json[0][0].lights?.[0].damping, 6);
  assert.deepStrictEqual(doodads, DoodadsTranslator.jsonToWar(parsed.json, parsed.formatVersion).buffer);
  const cameras = CamerasTranslator.jsonToWar([{
    target: { x: 1, y: 2 }, offsetZ: 3, rotation: 90, aoa: 45, distance: 100, roll: 0, fov: 70, farClipping: 5000, nearClipping: 16, localPitch: 1, localYaw: 2, localRoll: 3, name: 'Camera', dofDistance: 123.456, dofScale: 0.25, posAbsoluteZ: 37.5, cameraType: 2,
  }], 3).buffer;
  assert.deepStrictEqual(cameras, CamerasTranslator.jsonToWar(CamerasTranslator.warToJson(cameras).json, 3).buffer);
});

test('southern rotation preserves north, rotates asymmetric geometry and remaps waygates', () => {
  const map = new MapTranslator();
  map.terrain = getInitialTerrain(4, 4);
  map.terrain.groundHeight[1][2] = 1234;
  map.info = defaultInfo();
  map.info.camera.complements = [0, 0, 0, 0];
  map.info.camera.bounds = [-128, -128, 128, 128, -128, 128, 128, -128];
  map.regions = [{ name: 'Base', id: 9, position: { left: -100, bottom: -90, right: 50, top: 80 }, color: [1, 2, 3], weatherEffect: '\0\0\0\0', ambientSound: '', cameraBlocker: 2, alphaTileMinimapColor: -1 }];
  map.units = UnitsTranslator.warToJson(UnitsTranslator.jsonToWar([{
    type: 'hfoo', skin: 'hfoo', variation: 0, position: [100, -90, 12], rotation: 0.25, scale: [1, 1, 1], player: 1, hitpoints: -1, mana: -1, randomItemSetPtr: -1, droppedItemSets: [], gold: 0, targetAcquisition: -1, hero: { level: 0, str: 0, agi: 0, int: 0 }, inventory: [], abilities: [], random: { type: -1, level: undefined, itemClass: undefined, groupIndex: undefined, columnIndex: undefined, unitSet: undefined }, color: 0, waygate: 9, id: 17, groupId: 3, flags: 9, unknownBytes: [1, 2], unknownTail: [3, 4, 5],
  }], 13).buffer).json;
  const original = structuredClone(map.units[0]);
  duplicateMapSouthRotated(map);
  assert.equal(map.terrain.map.height, 8);
  assert.equal(map.terrain.map.offset.y, -768);
  assert.equal(map.terrain.groundHeight[5][2], 1234);
  assert.equal(map.terrain.groundHeight[3][2], 1234);
  assert.deepStrictEqual(map.units[0], original);
  assert.deepStrictEqual(map.units[1].position, [-100, -422, 12]);
  assert(Math.abs(map.units[1].rotation - (0.25 + Math.PI)) < 1e-6);
  assert.equal(map.units[1].waygate, 10);
  assert.equal(map.units[1].id, 18);
  assert.deepStrictEqual(map.regions[1].position, { left: -50, bottom: -592, right: 100, top: -422 });
  assert.equal(map.regions[1].cameraBlocker, 2);
  const encoded = UnitsTranslator.jsonToWar(map.units, 13).buffer;
  assert.deepStrictEqual(encoded, UnitsTranslator.jsonToWar(UnitsTranslator.warToJson(encoded).json, 13).buffer);
});

test('pathing and shadows rotate cells and preserve the north byte-for-byte', () => {
  const pixels = Buffer.from(Array.from({ length: 16 }, (_, index) => index));
  const header = Buffer.alloc(16);
  header.write('MP3W'); header.writeInt32LE(4, 8); header.writeInt32LE(4, 12);
  const result = duplicateRasterSouthRotated(Buffer.concat([header, pixels]), 1, 1, true);
  assert.equal(result.readInt32LE(12), 8);
  assert.deepStrictEqual(result.subarray(16, 32), Buffer.from(pixels).reverse());
  assert.deepStrictEqual(result.subarray(32), pixels);
  assert.deepStrictEqual(duplicateRasterSouthRotated(pixels, 1, 1, false), Buffer.concat([Buffer.from(pixels).reverse(), pixels]));
});

test('optional real map round-trips every relevant file without byte changes', () => {
  const dir = process.env.WC3_MAP_DIR;
  if (!dir) return;
  for (const [name, translator] of [
    ['war3map.w3e', TerrainTranslator], ['war3map.w3i', InfoTranslator], ['war3map.doo', DoodadsTranslator], ['war3mapUnits.doo', UnitsTranslator], ['war3map.w3r', RegionsTranslator], ['war3map.w3c', CamerasTranslator],
  ] as const) {
    const input = readFileSync(path.join(dir, name));
    // Keep each union member's typed input paired with its own writer.
    const roundTrip = (t: typeof translator) => {
      switch (t) {
        case TerrainTranslator: return TerrainTranslator.jsonToWar(TerrainTranslator.warToJson(input).json).buffer;
        case InfoTranslator: return InfoTranslator.jsonToWar(InfoTranslator.warToJson(input).json).buffer;
        case DoodadsTranslator: { const r = DoodadsTranslator.warToJson(input); return DoodadsTranslator.jsonToWar(r.json, r.formatVersion).buffer; }
        case UnitsTranslator: { const r = UnitsTranslator.warToJson(input); return UnitsTranslator.jsonToWar(r.json, r.formatVersion).buffer; }
        case RegionsTranslator: { const r = RegionsTranslator.warToJson(input); return RegionsTranslator.jsonToWar(r.json, r.formatVersion).buffer; }
        case CamerasTranslator: { const r = CamerasTranslator.warToJson(input); return CamerasTranslator.jsonToWar(r.json, r.formatVersion).buffer; }
        default: throw new Error('Unknown translator');
      }
    };
    assert.deepStrictEqual(roundTrip(translator), input, name);
  }
});

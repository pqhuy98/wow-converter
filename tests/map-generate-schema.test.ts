import { expect, test } from 'bun:test';
import { generateWc3BodySchema } from '../src/server/controllers/maps-generate';

const body = {
  tiles: [{ x: 32, y: 48 }], quality: 8192, mapSaveName: 'compatibility.w3x',
  clampLower: 0, clampUpper: 1, mapAngleDeg: 0, unitScale: 2,
  placePathBlockers: false, terrainAsWc3Tilesets: false, freshExport: false,
};

test('legacy server accepts the shared faction field in each NPC mode', () => {
  for (const creatures of [
    { enable: false, allAreDoodads: false },
    { enable: true, allAreDoodads: true },
    { enable: true, allAreDoodads: false },
  ]) {
    expect(generateWc3BodySchema.safeParse({ ...body, creatures, playerFaction: 'Alliance' }).success).toBe(true);
    expect(generateWc3BodySchema.safeParse({ ...body, creatures, playerFaction: 'Horde' }).success).toBe(true);
    expect(generateWc3BodySchema.safeParse({ ...body, creatures, playerFaction: 'Invalid' }).success).toBe(false);
  }
});

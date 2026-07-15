import {
  afterEach, describe, expect, spyOn, test,
} from 'bun:test';

import { resolveNpcMetaFromDB } from '@/lib/converter/character/wowhead-exporter/creature-model';
import { getWowDataClient } from '@/lib/wow-data-client/wow-data-client';

describe('NPC display resolution', () => {
  afterEach(() => {
    spyOn(getWowDataClient(), 'resolveNpcDisplayMeta').mockRestore();
  });

  test('uses wow-data-client metadata before converter-side DB2 caches', async () => {
    const resolve = spyOn(getWowDataClient(), 'resolveNpcDisplayMeta').mockResolvedValue({
      found: true,
      model: 456,
      textures: { 1: 789 },
      geosets: [{ geosetIndex: 2, geosetValue: 3 }],
    });

    const result = await resolveNpcMetaFromDB(123);
    expect(result).toEqual({
      Model: 456,
      Textures: { 1: 789 },
      Creature: {
        CreatureCustomizations: [],
        CreatureGeosetData: [{ GeosetIndex: 2, GeosetValue: 3 }],
      },
    });
    expect(resolve).toHaveBeenCalledWith(123);
  });
});

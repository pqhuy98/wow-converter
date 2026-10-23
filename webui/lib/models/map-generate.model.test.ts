import { expect, test } from 'bun:test';

import {
  mapClampCacheKey, readMapClampCache, readMapExportPreferences, touchMapClampCache,
} from './map-generate.model';

test('map texture defaults follow the product family', () => {
  expect(readMapExportPreferences(null, true)).toEqual({
    terrain: 4096, unit: 256, doodad: 256, includeBuildingInteriors: true,
  });
  expect(readMapExportPreferences(null, false)).toEqual({
    terrain: 4096, unit: 0, doodad: 0, includeBuildingInteriors: true,
  });
});

test('saved texture settings preserve explicit unlimited and independent caps', () => {
  expect(readMapExportPreferences('{"terrain":1024,"unit":0,"doodad":512}', true)).toEqual({
    terrain: 1024, unit: 0, doodad: 512, includeBuildingInteriors: true,
  });
});

test('invalid storage falls back per field without losing valid selections', () => {
  for (const raw of ['{', 'null', '[]', '"invalid"']) {
    expect(readMapExportPreferences(raw, true)).toEqual({
      terrain: 4096, unit: 256, doodad: 256, includeBuildingInteriors: true,
    });
  }
  expect(readMapExportPreferences('{"terrain":16384,"unit":"512","doodad":1024}', false)).toEqual({
    terrain: 4096, unit: 0, doodad: 1024, includeBuildingInteriors: true,
  });
});

test('8k terrain resolution survives saved preferences for both products', () => {
  for (const classic of [true, false]) {
    expect(readMapExportPreferences('{"terrain":8192}', classic).terrain).toBe(8192);
  }
});

test('interior preferences share the texture object, retaining explicit false', () => {
  expect(readMapExportPreferences('{"terrain":2048,"includeBuildingInteriors":false}', true)).toEqual({
    terrain: 2048, unit: 256, doodad: 256, includeBuildingInteriors: false,
  });
  expect(readMapExportPreferences('{"includeBuildingInteriors":"false"}', false).includeBuildingInteriors).toBe(true);
});

test('area clamp identity is independent of cell ordering, duplicates and product', () => {
  const key = mapClampCacheKey(0, [{ x: 32, y: 31 }, { x: 33, y: 32 }]);
  expect(mapClampCacheKey('0', [{ x: 33, y: 32 }, { x: 32, y: 31 }, { x: 32, y: 31 }])).toBe(key);
  expect(mapClampCacheKey(1, [{ x: 32, y: 31 }, { x: 33, y: 32 }])).not.toBe(key);
  expect(mapClampCacheKey(0, [{ x: 32, y: 31 }])).not.toBe(key);
});

test('clamp cache retains the 100 most recently used areas and refreshes hits', () => {
  let entries = readMapClampCache(null);
  for (let i = 0; i < 100; i += 1) entries = touchMapClampCache(entries, { key: String(i), lower: 0, upper: 0.8 });
  const oldest = entries[99];
  entries = touchMapClampCache(entries, { ...oldest, lower: 0.1 });
  entries = touchMapClampCache(entries, { key: 'new', lower: 0.2, upper: 1 });
  expect(entries).toHaveLength(100);
  expect(entries[0].key).toBe('new');
  expect(entries[1]).toEqual({ key: '0', lower: 0.1, upper: 0.8 });
  expect(entries.some((entry) => entry.key === '1')).toBe(false);
  expect(readMapClampCache(JSON.stringify(entries))).toEqual(entries);
});

test('invalid clamp cache entries are skipped without discarding valid areas', () => {
  for (const raw of [null, '{', '{}', 'null']) expect(readMapClampCache(raw)).toEqual([]);
  const good = { key: 'area', lower: 0.1, upper: 0.9 };
  const invalid = [null, {}, { ...good, lower: '0.1' }, { ...good, lower: -0.1 }, { ...good, upper: 1.1 }, { ...good, lower: 0.9 }];
  expect(readMapClampCache(JSON.stringify([...invalid, good, { ...good, lower: 0.2 }]))).toEqual([good]);
});

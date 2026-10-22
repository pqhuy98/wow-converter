import { describe, expect, test } from 'bun:test';

import { defaultGenerateWc3FormValues } from './map-generate.model';
import type { MapPreviewData, MapPreviewNPC } from './map-preview.model';
import {
  fitTerrainClampRange, minimumTerrainClampWidth, previewFactionCounts, previewIsWater, previewMapSize, previewNPCIsUnit, previewNPCPlacement, previewWaterLevels, previewWaterRange, readMapPreviewStream,
} from './map-preview.model';

const scene: MapPreviewData = {
  assets: [],
  instances: [],
  npcs: [],
  warnings: [],
  min: [0, 0, 0],
  max: [100, 100, 100],
  rawModelScale: 56,
  groundHeightRange: 3840,
  groundMinZ: -1280,
  waterHeightRange: [-2137.6, 1958.15],
  liquidTypes: { 1: 0, 2: 1, 3: 2 },
};
const npc: MapPreviewNPC = {
  guid: 1, name: 'Guard', position: [50, 50, 50], groundZ: 50, terrainZ: 10,
};
const enabled = { ...defaultGenerateWc3FormValues, creatures: { enable: true, allAreDoodads: false } };

describe('terrain clamp map size limit', () => {
  test('reports output width and height as the clamp changes, rounded to four cells', () => {
    const data: MapPreviewData = { ...scene, min: [-100, -200, -10], max: [900, 300, 990] };
    expect(previewMapSize(data, { ...enabled, clampLower: 0, clampUpper: 1 })).toEqual({ width: 32, height: 16 });
    expect(previewMapSize(data, { ...enabled, clampLower: 0.1, clampUpper: 0.6 })).toEqual({ width: 60, height: 32 });
    expect(previewMapSize(data, {
      ...enabled, clampLower: 0.1, clampUpper: 0.6, unitScale: 99,
    })).toEqual({ width: 60, height: 32 });
    expect(previewMapSize({ ...data, max: [900, -199, 990] }, { ...enabled, clampLower: 0, clampUpper: 1 })).toEqual({ width: 32, height: 4 });
    expect(previewMapSize(data, { ...enabled, clampLower: 0.5, clampUpper: 0.5 })).toBeNull();
    expect(previewMapSize({ ...data, max: [900, 300, -10] }, enabled)).toBeNull();
    const limit: MapPreviewData = { ...scene, max: [122881, 100, 10000] };
    expect(previewMapSize(limit, { ...enabled, clampLower: 0, clampUpper: minimumTerrainClampWidth(limit) })).toEqual({ width: 480, height: 4 });
  });

  test('rounds the gap upwards and keeps either horizontal axis within 480 tiles', () => {
    for (const x of [100, 10000, 122881]) {
      for (const y of [100, 10000, 122881]) {
        const data = { ...scene, max: [x, y, 10000] satisfies [number, number, number] };
        const width = minimumTerrainClampWidth(data);
        for (const center of [0, 0.5, 1]) {
          const [lower, upper] = fitTerrainClampRange(center, center, width);
          expect(lower).toBeGreaterThanOrEqual(0);
          expect(upper).toBeLessThanOrEqual(1);
          expect(upper - lower).toBeCloseTo(width, 12);
          for (const horizontal of [x, y]) {
            const tiles = horizontal * data.groundHeightRange / (10000 * (upper - lower) * 128);
            expect(Math.ceil(tiles / 4 - 1e-10) * 4).toBeLessThanOrEqual(480);
          }
        }
      }
    }
    expect(minimumTerrainClampWidth({ ...scene, max: [122881, 100, 10000] })).toBe(0.769);
    expect(minimumTerrainClampWidth({ ...scene, max: [100, 122881, 10000] })).toBe(0.769);
  });

  test('preserves valid manual ranges and rejects impossible selections', () => {
    expect(fitTerrainClampRange(0.12345, 0.98765, 0.2)).toEqual([0.12345, 0.98765]);
    expect(fitTerrainClampRange(0.49, 0.51, 0.2)).toEqual([0.4, 0.6]);
    expect(minimumTerrainClampWidth({ ...scene, max: [2000, 100, 1] })).toBeGreaterThan(1);
    expect(minimumTerrainClampWidth({ ...scene, max: [100, 100, 0] })).toBe(Infinity);
    expect(fitTerrainClampRange(0.5, 0.5, Infinity)).toEqual([0, 1]);
  });
});

describe('map preview progress stream', () => {
  test('reads split UTF-8 progress messages and the final result', async () => {
    const encoded = new TextEncoder().encode(`${JSON.stringify({ progress: { message: 'Preparing terrain —', completed: 1, total: 2 } })}\n${JSON.stringify({ result: scene })}`);
    const stream = new ReadableStream<Uint8Array>({ start(controller) { for (const byte of encoded) controller.enqueue(new Uint8Array([byte])); controller.close(); } });
    const progress: string[] = [];
    expect(await readMapPreviewStream(new Response(stream), (step) => progress.push(`${step.message} ${step.completed}/${step.total}`))).toEqual(scene);
    expect(progress).toEqual(['Preparing terrain — 1/2']);
  });
  test('rejects server errors, incomplete and aborted streams', async () => {
    const failure = (response: Response) => readMapPreviewStream(response, () => {}).then(() => '', (error: Error) => error.message);
    expect(await failure(new Response('Internal Server Error', { status: 500 }))).toContain('Could not prepare');
    expect(await failure(new Response('{"error":"missing tile"}\n'))).toContain('missing tile');
    expect(await failure(new Response('{"progress":{"message":"Loading","completed":0,"total":1}}\n'))).toContain('before completion');
    const stream = new ReadableStream<Uint8Array>({ start(controller) { controller.error(new DOMException('Cancelled', 'AbortError')); } });
    expect(await failure(new Response(stream))).toContain('Cancelled');
  });
});

describe('map preview NPC placement', () => {
  test('green for a supported floor, red outside clamp or XY bounds', () => {
    expect(previewNPCIsUnit(npc, scene, enabled)).toBe(true);
    expect(previewNPCIsUnit(npc, scene, { ...enabled, clampUpper: 0.4 })).toBe(false);
    expect(previewNPCIsUnit({ ...npc, position: [-1, 50, 50] }, scene, enabled)).toBe(false);
  });
  test('honors NPC decoration mode and the export checkbox', () => {
    expect(previewNPCIsUnit(npc, scene, { ...enabled, creatures: { enable: true, allAreDoodads: true } })).toBe(false);
    expect(previewNPCIsUnit(npc, scene, defaultGenerateWc3FormValues)).toBe(false);
  });
  test('uses WMO floors only when collision terrain is enabled', () => {
    expect(previewNPCIsUnit(npc, scene, { ...enabled, placePathBlockers: false })).toBe(false);
    expect(previewNPCIsUnit({ ...npc, groundZ: null }, scene, enabled)).toBe(false);
  });
  test('anchors ground distance to the recommended scale across manual clamp edits', () => {
    const referenceScene: MapPreviewData = { ...scene, max: [100, 100, 200] };
    const onFloor: MapPreviewNPC = { ...npc, position: [50, 50, 100], groundZ: 100 };
    // Recommended width = 3840/(200*56)*2. A 3-source-unit gap is 84 WC3
    // units at that scale; a 4-unit gap is 112 and must always stay excluded.
    for (const [clampLower, clampUpper] of [[0, 1], [0, 3840 / 11200 * 2], [0, 0.9], [0.25, 0.75], [0.49, 0.54]]) {
      const form = { ...enabled, clampLower, clampUpper };
      for (const offset of [-3, 3]) {
        expect(previewNPCIsUnit({ ...onFloor, position: [50, 50, 100 + offset] }, referenceScene, form)).toBe(offset > 0 || clampLower < 0.49);
      }
      expect(previewNPCIsUnit({ ...onFloor, position: [50, 50, 104] }, referenceScene, form)).toBe(false);
    }
    expect(previewNPCIsUnit({ ...onFloor, position: [50, 50, 103] }, referenceScene, { ...enabled, unitScale: 1 })).toBe(false);
  });
  test('honors recommendation map-size limits and full-range saturation', () => {
    // Same raw dimensions as the Go recommendation tests: 122881 x 100 x 10000.
    const wide: MapPreviewData = { ...scene, min: [0, 0, 0], max: [122881 / 56, 100 / 56, 10000 / 56] };
    const spawn: MapPreviewNPC = {
      guid: 2, name: 'Guard', position: [50 / 56, 50 / 56, 2150 / 56], groundZ: 2000 / 56, terrainZ: 2000 / 56,
    };
    for (const clampUpper of [1, 0.5, 0.25]) {
      expect(previewNPCIsUnit(spawn, wide, { ...enabled, unitScale: 1, clampUpper })).toBe(true);
      expect(previewNPCIsUnit({ ...spawn, position: [50 / 56, 50 / 56, 2250 / 56] }, wide, { ...enabled, unitScale: 1, clampUpper })).toBe(false);
    }
    const short: MapPreviewData = { ...scene, max: [100, 100, 10] };
    const close: MapPreviewNPC = {
      ...npc, position: [50, 50, 5.2], groundZ: 5, terrainZ: 5,
    };
    expect(previewNPCIsUnit(close, short, enabled)).toBe(true);
    expect(previewNPCIsUnit(close, short, { ...enabled, clampLower: 0.25, clampUpper: 0.75 })).toBe(true);
    expect(previewNPCIsUnit({ ...close, position: [50, 50, 5.3] }, short, enabled)).toBe(false);
  });
});

describe('map preview water', () => {
  const water: MapPreviewData = {
    ...scene,
    min: [0, 0, -20],
    max: [100, 100, 80],
    assets: [{ id: 'sea', parts: [{ mesh: 'sea.bin', vertices: 6, liquid: { typeId: 2, min: [0, 0, 0], max: [10, 10, 0] } }] }],
    instances: [{ asset: 'sea', kind: 'adt', transform: [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1] }],
  };
  test('sea remains visible with NPC export off, but a high clamp excludes it', () => {
    expect(previewWaterLevels(water, defaultGenerateWc3FormValues)).toEqual({ min: 0, max: 0, clipped: false });
    expect(previewWaterLevels(water, { ...enabled, clampLower: 0.3 })).toEqual({ min: 0, max: 0, clipped: true });
    expect(previewWaterLevels(water, { ...enabled, clampLower: 0.1 })).toEqual({ min: 0, max: 0, clipped: false });
  });
  test('respects native water ceiling even inside the terrain clamp', () => {
    const range = previewWaterRange(scene, enabled);
    expect(range[0]).toBeGreaterThan(0);
    expect(range[1]).toBeCloseTo((1958.15 + 1280) / 3840 * 100);
    expect(range[1]).toBeLessThan(100);
    expect(previewWaterRange(scene, { ...enabled, clampLower: 0.1, clampUpper: 0.9 })[1]).toBeLessThan(range[1]);
  });
  test('applies WMO placement Z and scale exactly once', () => {
    const placed: MapPreviewData = { ...water, instances: [{ asset: 'sea', kind: 'wmo', transform: [2, 0, 0, 0, 0, 2, 0, 0, 0, 0, 2, 0, 0, 0, 40, 1] }] };
    expect(previewWaterLevels(placed, enabled)).toEqual({ min: 40, max: 40, clipped: false });
    expect(previewWaterLevels(placed, { ...enabled, clampUpper: 0.5 })?.clipped).toBe(true);
  });
  test('does not depict magma, unknown liquids or dry geometry as native water', () => {
    expect(previewIsWater({ mesh: '', vertices: 0, liquid: { typeId: 3, min: [0, 0, 0], max: [1, 1, 1] } }, scene)).toBe(false);
    expect(previewIsWater({ mesh: '', vertices: 0, liquid: { typeId: 999, min: [0, 0, 0], max: [1, 1, 1] } }, scene)).toBe(false);
    expect(previewIsWater({ mesh: '', vertices: 0 }, scene)).toBe(false);
    expect(previewWaterLevels(scene, enabled)).toBeNull();
  });
});

describe('preview placement', () => {
  test('counts factions only for NPCs exported as units', () => {
    const data = {
      ...scene,
      npcs: [
        { ...npc, faction: 10 }, { ...npc, guid: 2, faction: 10 }, { ...npc, guid: 3, faction: 20 },
        {
          ...npc, guid: 4, faction: 30, groundZ: null,
        }, {
          ...npc, guid: 5, faction: 40, position: [50, 50, 90] satisfies [number, number, number],
        },
      ],
    };
    const form = { ...enabled, clampLower: 0.25, clampUpper: 0.75 };
    expect(previewFactionCounts(data, form)).toEqual({ 10: 2, 20: 1 });
    expect(previewFactionCounts(data, { ...form, creatures: { enable: true, allAreDoodads: true } })).toEqual({});
    expect(previewFactionCounts(data, { ...form, creatures: { enable: false, allAreDoodads: false } })).toEqual({});
    expect(previewFactionCounts(data, { ...form, clampLower: 0.6 })).toEqual({});
  });

  test('distinguishes units, destructibles and excluded NPCs using export rules', () => {
    expect(previewNPCPlacement(npc, scene, enabled)).toBe('unit');
    expect(previewNPCPlacement(npc, scene, { ...enabled, clampUpper: 0.4 })).toBe('excluded');
    const decorations = { ...enabled, creatures: { enable: true, allAreDoodads: true } };
    expect(previewNPCPlacement(npc, scene, { ...decorations, clampUpper: 0.4 })).toBe('destructible');
    expect(previewNPCPlacement({ ...npc, groundZ: null }, scene, decorations)).toBe('destructible');
    expect(previewNPCPlacement({ ...npc, position: [-10, 50, 50] }, scene, decorations)).toBe('excluded');
    expect(previewNPCPlacement(npc, scene, defaultGenerateWc3FormValues)).toBe('excluded');
  });
});

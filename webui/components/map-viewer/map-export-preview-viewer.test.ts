import { afterEach, expect, test } from 'bun:test';

import { defaultGenerateWc3FormValues } from '../../lib/models/map-generate.model';
import type { MapPreviewData } from '../../lib/models/map-preview.model';
import { MapExportPreviewViewer } from './map-export-preview-viewer';

const cleanups: (() => void)[] = [];
afterEach(() => { while (cleanups.length) cleanups.pop()?.(); });

// Record WebGL commands without opening a browser. This covers pass isolation
// and cache reuse; rasterization and driver GLSL linking need visual verification.
function fixture(rootWMO = false, withWaterAndNPC = false) {
  const draws: { x: number; clamped: boolean; wireframe: boolean; textured: boolean; water: number; markers: boolean; depth: boolean }[] = [];
  const uniforms = new Map<string, number>();
  const matrices = new Map<string, number[]>();
  const locations = new Map<WebGLUniformLocation, string>();
  const shaders: string[] = [];
  const fetches: string[] = [];
  const frames = new Map<number, FrameRequestCallback>();
  let frameID = 0; let modelX = 0; let depth = true; let pickID = 0;
  const markerUploads: number[][] = [];
  const pixelReads: number[][] = [];
  const constants = {
    VERTEX_SHADER: 0x8b31, FRAGMENT_SHADER: 0x8b30, DEPTH_TEST: 0x0b71, BLEND: 0x0be2, FRAMEBUFFER_COMPLETE: 0x8cd5, DYNAMIC_DRAW: 0x88e8,
  };
  const record = () => draws.push({
    x: modelX,
    clamped: uniforms.get('u_terrain') === 1,
    wireframe: uniforms.get('u_wireframe') === 1,
    textured: uniforms.get('u_textured') === 1,
    water: uniforms.get('u_water') ?? 0,
    markers: uniforms.get('u_markers') === 1,
    depth,
  });
  const methods: Partial<WebGL2RenderingContext> = {
    createProgram: () => ({}),
    createFramebuffer: () => ({}),
    createRenderbuffer: () => ({}),
    checkFramebufferStatus: () => constants.FRAMEBUFFER_COMPLETE,
    readPixels: (x, y, width, height, _format, _type, pixels) => {
      pixelReads.push([x, y, width, height]);
      if (pixels instanceof Uint8Array) pixels.set([pickID & 255, (pickID >> 8) & 255, (pickID >> 16) & 255, 255]);
    },
    bufferData: (_target, data, usage) => {
      if (usage === constants.DYNAMIC_DRAW && data instanceof Float32Array) markerUploads.push(Array.from(data));
    },
    createShader: () => ({}),
    createTexture: () => ({}),
    createBuffer: () => ({}),
    createVertexArray: () => ({}),
    shaderSource: (_shader, source) => { shaders.push(source); },
    getShaderParameter: () => true,
    getProgramParameter: () => true,
    getExtension: () => null,
    getUniformLocation: (_program, name) => { const location = {}; locations.set(location, name); return location; },
    uniform1i: (location, value) => { uniforms.set(locations.get(location ?? {}) ?? '', value); },
    uniformMatrix4fv: (location, _transpose, data) => {
      const name = locations.get(location ?? {}) ?? '';
      matrices.set(name, Array.from(data));
      if (name === 'u_model') modelX = Array.from(data)[12];
    },
    uniform1f: (location, value) => { uniforms.set(locations.get(location ?? {}) ?? '', value); },
    enable: (cap) => { if (cap === constants.DEPTH_TEST) depth = true; },
    disable: (cap) => { if (cap === constants.DEPTH_TEST) depth = false; },
    drawArrays: record,
    drawArraysInstanced: record,
  };
  // The only casts are the partial DOM/WebGL boundaries of this offline fixture.
  const gl = new Proxy(methods, { get: (target, key) => Reflect.get(target, key) ?? Reflect.get(constants, key) ?? (typeof key === 'string' && /^[A-Z_0-9]+$/.test(key) ? 0 : () => {}) }) as WebGL2RenderingContext;
  const canvas = {
    getContext: () => gl, clientWidth: 800, clientHeight: 600, width: 800, height: 600, addEventListener: () => {}, removeEventListener: () => {},
  } as unknown as HTMLCanvasElement;
  const replaceGlobal = (key: string, value: unknown) => {
    const original = Object.getOwnPropertyDescriptor(globalThis, key);
    Object.defineProperty(globalThis, key, { configurable: true, value });
    cleanups.push(() => { if (original) Object.defineProperty(globalThis, key, original); else Reflect.deleteProperty(globalThis, key); });
  };
  replaceGlobal('window', { devicePixelRatio: 1, addEventListener: () => {}, removeEventListener: () => {} });
  replaceGlobal('ResizeObserver', class {
    observe() {}

    disconnect() {}
  });
  replaceGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => { frames.set(++frameID, callback); return frameID; });
  replaceGlobal('cancelAnimationFrame', (id: number) => { frames.delete(id); });
  replaceGlobal('fetch', (url: string) => { fetches.push(url); return Promise.resolve(new Response(new Float32Array(24).buffer)); });
  replaceGlobal('createImageBitmap', () => Promise.resolve({ close: () => {} }));
  const viewer = new MapExportPreviewViewer(canvas);
  cleanups.push(() => viewer.dispose());
  const transform = (x: number) => [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, x, 0, 0, 1];
  const scene: MapPreviewData = {
    min: [0, 0, 0],
    max: [100, 100, 100],
    rawModelScale: 56,
    groundHeightRange: 3840,
    groundMinZ: -1280,
    waterHeightRange: [-2137.6, 1958.15],
    liquidTypes: { 1: 0 },
    warnings: [],
    npcs: withWaterAndNPC ? [{
      guid: 1, name: 'Guard', position: [50, 50, 50], groundZ: 50, terrainZ: 50,
    }] : [],
    assets: [
      { id: 'root', parts: [{ mesh: 'root.bin', vertices: 3 }] },
      { id: 'child', parts: [{ mesh: 'child.bin', texture: 'child.png', vertices: 3 }] },
    ],
    instances: [
      {
        asset: 'root', kind: rootWMO ? 'wmo' : 'adt', root: rootWMO, transform: transform(11),
      },
      { asset: 'child', kind: 'wmo', transform: transform(22) },
    ],
  };
  if (withWaterAndNPC) scene.assets[0].parts.push({ mesh: 'water.bin', vertices: 3, liquid: { typeId: 1, min: [0, 0, 50], max: [10, 10, 50] } });
  const flush = () => { for (const [id, callback] of frames) { frames.delete(id); callback(0); } };
  return {
    viewer, scene, draws, fetches, shaders, flush, matrices, uniforms, markerUploads, pixelReads, setPickID: (id: number) => { pickID = id; },
  };
}

test('default camera and Fit show WC3 axes while applying the chosen angle once', async () => {
  const {
    viewer, scene, matrices, uniforms, flush,
  } = fixture();
  viewer.setOptions(defaultGenerateWc3FormValues);
  await viewer.setScene(scene); flush();
  const view = matrices.get('u_viewProjection');
  expect(view?.[0]).toBeGreaterThan(0);
  expect(Math.abs(view?.[4] ?? Infinity)).toBeLessThan(1e-6);
  expect(uniforms.get('u_angle')).toBe(0);
  viewer.setOptions({ ...defaultGenerateWc3FormValues, mapAngleDeg: 90 });
  viewer.fitCamera(); flush();
  expect(uniforms.get('u_angle')).toBeCloseTo(Math.PI / 2);
  expect(matrices.get('u_viewProjection')).toEqual(view);
});

test('asset progress includes completed cached assets', async () => {
  const { viewer, scene, fetches } = fixture();
  const progress: number[] = [];
  await viewer.setScene(scene, (step) => { progress.push(step.completed); expect(step.total).toBe(2); });
  expect(progress[0]).toBe(0); expect(progress.at(-1)).toBe(2);
  const loaded = fetches.length;
  const warm: number[] = [];
  await viewer.setScene(scene, (step) => { warm.push(step.completed); });
  expect(warm).toEqual([0, 1, 2]); expect(fetches.length).toBe(loaded);
});

test('clamp draws a root-only wireframe guide and preserves child WMO materials without reloading assets', async () => {
  const {
    viewer, scene, draws, fetches, flush,
  } = fixture();
  viewer.setOptions(defaultGenerateWc3FormValues);
  await viewer.setScene(scene); flush();
  expect(draws.map((d) => [d.x, d.clamped, d.wireframe, d.textured])).toEqual([[11, true, false, false], [22, false, false, true]]);
  const loaded = fetches.length;
  draws.length = 0;
  viewer.setOptions({ ...defaultGenerateWc3FormValues, clampLower: 0.2, clampUpper: 0.8 }); flush();
  expect(draws.map((d) => [d.x, d.clamped, d.wireframe, d.textured, d.depth])).toEqual([
    [11, true, false, false, true], [11, true, true, false, false], [22, false, false, true, true],
  ]);
  expect(fetches.length).toBe(loaded);
});

test('explicit root WMOs follow the clamp while water and NPC passes never inherit wireframe state', async () => {
  const {
    viewer, scene, draws, shaders, flush,
  } = fixture(true, true);
  viewer.setOptions({
    ...defaultGenerateWc3FormValues, clampLower: 0.2, clampUpper: 0.8, creatures: { enable: true, allAreDoodads: false },
  });
  await viewer.setScene(scene); flush();
  expect(draws.filter((d) => d.wireframe).map((d) => d.x)).toEqual([11]);
  expect(draws.filter((d) => d.water > 0 || d.markers).map((d) => [d.clamped, d.wireframe])).toEqual([[false, false], [false, false], [false, false]]);
  // Shared integer uniforms require explicit precision: the two GLSL stages
  // otherwise have different defaults even when their declarations look alike.
  for (const source of shaders) expect(source).toMatch(/uniform highp int u_water;/);
});

test('cursor picking resolves visible terrain and NPC names with a single pixel and resets draw state', async () => {
  const {
    viewer, scene, flush, setPickID, pixelReads, uniforms, fetches,
  } = fixture(false, true);
  viewer.setOptions({ ...defaultGenerateWc3FormValues, creatures: { enable: true, allAreDoodads: false } });
  await viewer.setScene(scene); flush();
  const loaded = fetches.length;
  setPickID(1); expect(viewer.pick(100, 120)).toEqual({ kind: 'terrain', playable: true });
  setPickID(2); expect(viewer.pick(110, 130)).toEqual({ kind: 'terrain', playable: false });
  setPickID(3); expect(viewer.pick(120, 140)).toEqual({ kind: 'npc', name: 'Guard', placement: 'unit' });
  setPickID(0); expect(viewer.pick(130, 150)).toBeNull();
  expect(viewer.pick(-1, 0)).toBeNull();
  expect(viewer.pick(800, 100)).toBeNull();
  expect(pixelReads).toEqual(Array(4).fill([0, 0, 1, 1]));
  expect(uniforms.get('u_picking')).toBe(0);
  expect(uniforms.get('u_markers')).toBe(0);
  expect(fetches.length).toBe(loaded);
  // ID 259 exercises decoding across both low bytes, not just small maps.
  scene.npcs = Array.from({ length: 257 }, (_, i) => ({ ...scene.npcs[0], guid: i + 1, name: `Guard ${i}` }));
  viewer.setOptions({ ...defaultGenerateWc3FormValues, creatures: { enable: true, allAreDoodads: false } }); flush();
  setPickID(259); expect(viewer.pick(140, 160)).toEqual({ kind: 'npc', name: 'Guard 256', placement: 'unit' });
});

test('destructible NPC markers are yellow and excluded NPC markers stay red', async () => {
  const {
    viewer, scene, flush, markerUploads, setPickID,
  } = fixture(false, true);
  viewer.setOptions({ ...defaultGenerateWc3FormValues, creatures: { enable: true, allAreDoodads: true } });
  await viewer.setScene(scene); flush();
  const color = () => markerUploads.at(-1)?.slice(4, 7).map((value) => Number(value.toFixed(2)));
  expect(color()).toEqual([1, 0.85, 0.1]);
  setPickID(3); expect(viewer.pick(100, 120)).toEqual({ kind: 'npc', name: 'Guard', placement: 'destructible' });
  viewer.setOptions({ ...defaultGenerateWc3FormValues, clampUpper: 0.4, creatures: { enable: true, allAreDoodads: false } }); flush();
  expect(color()).toEqual([1, 0.12, 0.15]);
  expect(viewer.pick(100, 120)).toEqual({ kind: 'npc', name: 'Guard', placement: 'excluded' });
});

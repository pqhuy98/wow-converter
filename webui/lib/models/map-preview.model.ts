import type { GenerateWc3FormValues } from './map-generate.model';

export type PreviewVector = [number, number, number];

export interface MapPreviewProgress {
  message: string;
  completed: number;
  total: number;
}

/** A request-owned stream: aborting the dialog fetch also cancels preparation. */
export async function readMapPreviewStream(response: Response, onProgress: (progress: MapPreviewProgress) => void): Promise<MapPreviewData> {
  if (!response.ok) {
    const failure: { error?: string } = await response.json().catch(() => ({}));
    throw new Error(failure.error ?? 'Could not prepare the map preview.');
  }
  if (!response.body) throw new Error('Map preview response is empty.');
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let pending = ''; let result: MapPreviewData | undefined;
  const consume = (line: string) => {
    if (!line.trim()) return;
    const event: { progress?: MapPreviewProgress; result?: MapPreviewData; error?: string } = JSON.parse(line);
    if (event.error) throw new Error(event.error);
    if (event.progress) onProgress(event.progress);
    if (event.result) result = event.result;
  };
  try {
    let ended = false;
    while (!ended) {
      const { value, done } = await reader.read();
      pending += decoder.decode(value, { stream: !done });
      let newline = pending.indexOf('\n');
      while (newline !== -1) {
        consume(pending.slice(0, newline)); pending = pending.slice(newline + 1);
        newline = pending.indexOf('\n');
      }
      ended = done;
    }
    consume(pending);
    if (!result) throw new Error('Map preview preparation ended before completion.');
    return result;
  } finally {
    await reader.cancel().catch(() => {});
    reader.releaseLock();
  }
}

export interface MapPreviewPart {
  mesh: string;
  texture?: string;
  vertices: number;
  liquid?: { typeId: number; min: PreviewVector; max: PreviewVector };
}

export interface MapPreviewAsset {
  id: string;
  parts: MapPreviewPart[];
}

export interface MapPreviewInstance {
  asset: string;
  kind: 'adt' | 'wmo';
  root?: boolean; // ADTs are implicit roots; root WMO previews can opt in.
  transform: number[];
}

export interface MapPreviewNPC {
  faction?: number;
  guid: number;
  name: string;
  position: PreviewVector;
  groundZ: number | null;
  terrainZ: number | null;
}

export interface MapPreviewFactions {
  allies: { name: string; units: number }[];
  enemies: { name: string; units: number }[];
  neutral: { name: string; units: number }[];
  neutralHostile: number;
  neutralPassive: number;
}

export function previewFactionCounts(scene: MapPreviewData, form: GenerateWc3FormValues): Record<number, number> {
  const counts: Record<number, number> = {};
  for (const npc of scene.npcs) {
    if (npc.faction && previewNPCPlacement(npc, scene, form) === 'unit') counts[npc.faction] = (counts[npc.faction] ?? 0) + 1;
  }
  return counts;
}

export interface MapPreviewData {
  assets: MapPreviewAsset[];
  instances: MapPreviewInstance[];
  npcs: MapPreviewNPC[];
  min: PreviewVector;
  max: PreviewVector;
  rawModelScale: number;
  groundHeightRange: number;
  groundMinZ: number;
  waterHeightRange: [number, number];
  liquidTypes: Record<number, number>;
  warnings: string[];
}

/** Mirrors terrainDimensions: XY and Z scale together, so raw model scale and
 * unit scale cancel out. Round up to the slider's 0.1 percentage-point step. */
export function minimumTerrainClampWidth(scene: MapPreviewData): number {
  const span = scene.max[2] - scene.min[2];
  if (span <= 0) return Infinity;
  const horizontal = Math.max(scene.max[0] - scene.min[0], scene.max[1] - scene.min[1]);
  const width = scene.groundHeightRange * horizontal / (span * 128 * 480);
  if (!Number.isFinite(width) || width <= 0) return Infinity;
  return Math.max(0.001, Math.ceil(width * 1000 - 1e-9) / 1000);
}

/** Output terrain cells, matching Go TerrainDimensions' four-tile rounding. */
export function previewMapSize(scene: MapPreviewData, form: GenerateWc3FormValues): { width: number; height: number } | null {
  const spans = scene.max.map((value, axis) => value - scene.min[axis]);
  const clampWidth = form.clampUpper - form.clampLower;
  if (spans.some((span) => !Number.isFinite(span) || span <= 0) || !Number.isFinite(clampWidth) || clampWidth <= 0) return null;
  const ratio = scene.groundHeightRange / (spans[2] * clampWidth) / 128;
  const tiles = (span: number) => Math.max(4, Math.ceil(span * ratio / 4 - 1e-10) * 4);
  return { width: tiles(spans[0]), height: tiles(spans[1]) };
}

/** Widen stale settings or a suggestion that falls below the safe minimum. */
export function fitTerrainClampRange(lower: number, upper: number, minimumWidth: number): [number, number] {
  const width = Math.min(1, minimumWidth);
  if (upper - lower >= width - 1e-12) return [lower, upper];
  const nextLower = Math.max(0, Math.min(1 - width, Math.floor((lower + upper - width) * 500) / 1000));
  return [nextLower, Math.round((nextLower + width) * 1000) / 1000];
}

export function previewIsWater(part: MapPreviewPart, scene: MapPreviewData): boolean {
  const bank = part.liquid ? scene.liquidTypes[part.liquid.typeId] : -1;
  return bank === 0 || bank === 1;
}

/** WC3 water has a smaller upper height limit than terrain. Water must also
 * clear the clamped floor by 0.125 WC3 units, as in terrain_water.go. */
export function previewWaterRange(scene: MapPreviewData, form: GenerateWc3FormValues): [number, number] {
  const span = scene.max[2] - scene.min[2];
  const lower = scene.min[2] + span * form.clampLower;
  const upper = scene.min[2] + span * form.clampUpper;
  const inverseScale = Math.max(0.000001, span * (form.clampUpper - form.clampLower)) / scene.groundHeightRange;
  return [Math.max(lower + 0.125 * inverseScale, lower + (scene.waterHeightRange[0] - scene.groundMinZ) * inverseScale),
    Math.min(upper, lower + (scene.waterHeightRange[1] - scene.groundMinZ) * inverseScale)];
}

export function previewWaterLevels(scene: MapPreviewData, form: GenerateWc3FormValues): { min: number; max: number; clipped: boolean } | null {
  const assets = new Map(scene.assets.map((asset) => [asset.id, asset]));
  let min = Infinity; let max = -Infinity;
  for (const instance of scene.instances) {
    for (const part of assets.get(instance.asset)?.parts ?? []) {
      if (!part.liquid || !previewIsWater(part, scene)) continue;
      // Transform the local bounds, including tilted/scaled WMO placements.
      const m = instance.transform;
      for (const x of [part.liquid.min[0], part.liquid.max[0]]) {
        for (const y of [part.liquid.min[1], part.liquid.max[1]]) {
          for (const z of [part.liquid.min[2], part.liquid.max[2]]) {
            const height = m[2] * x + m[6] * y + m[10] * z + m[14];
            min = Math.min(min, height); max = Math.max(max, height);
          }
        }
      }
    }
  }
  if (!Number.isFinite(min)) return null;
  const range = previewWaterRange(scene, form);
  return { min, max, clipped: min < range[0] || max > range[1] };
}

/** Same recommended-clamp ground tolerance as PlaceUnits, compared in source
 * space so slider edits cannot change ground status for an in-range NPC.
 * The source floor precedes the full collision terrain generated on export. */
export function previewNPCIsUnit(npc: MapPreviewNPC, scene: MapPreviewData, form: GenerateWc3FormValues): boolean {
  return previewNPCPlacement(npc, scene, form) === 'unit';
}

export function previewNPCPlacement(npc: MapPreviewNPC, scene: MapPreviewData, form: GenerateWc3FormValues): 'unit' | 'destructible' | 'excluded' {
  if (!form.creatures.enable) return 'excluded';
  const tolerance = 1 / scene.rawModelScale;
  if (npc.position[0] < scene.min[0] - tolerance || npc.position[0] > scene.max[0] + tolerance
    || npc.position[1] < scene.min[1] - tolerance || npc.position[1] > scene.max[1] + tolerance) return 'excluded';
  if (form.creatures.allAreDoodads) return 'destructible';
  const groundZ = form.placePathBlockers ? npc.groundZ : npc.terrainZ;
  if (groundZ === null) return 'excluded';
  const span = scene.max[2] - scene.min[2];
  const ratio = span > 0 ? (npc.position[2] - scene.min[2]) / span : 0;
  if (ratio < form.clampLower || ratio > form.clampUpper) return 'excluded';
  const referenceWidth = recommendedTerrainClampWidth(scene, form.unitScale);
  return Math.abs(npc.position[2] - groundZ) * scene.groundHeightRange <= 100 * span * referenceWidth ? 'unit' : 'excluded';
}

/** Mirrors mapexporter's recommendation width, including four-tile rounding and
 * the 480-tile limit. NPC density affects the interval position only. */
function recommendedTerrainClampWidth(scene: MapPreviewData, unitScale: number): number {
  const span = (scene.max[2] - scene.min[2]) * scene.rawModelScale;
  if (span <= 0 || unitScale <= 0) return 1;
  const tiles = (axis: number) => (scene.max[axis] - scene.min[axis]) * scene.rawModelScale / unitScale / 128;
  const ceilDiv4 = (value: number) => Math.ceil(Math.floor(value) / 4) * 4;
  const minimumWidth = scene.groundHeightRange * Math.max(scene.max[0] - scene.min[0], scene.max[1] - scene.min[1])
    / ((scene.max[2] - scene.min[2]) * 128 * 480);
  return Math.min(1, Math.max(minimumWidth, scene.groundHeightRange / span * unitScale
    * Math.max(1, ceilDiv4(tiles(0)) / 480, ceilDiv4(tiles(1)) / 480)));
}

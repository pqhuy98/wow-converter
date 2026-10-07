import { createHash } from 'crypto';

export type ShotCameras = Readonly<Record<string, Readonly<Record<string, unknown>>>>;

/** Keep the PNG and camera map paired, and invalidate maps from older capture protocols. */
export function wowheadPairHash(
  url: string,
  source: { readonly wowName: string; readonly wowVariant: number },
  cameras: ShotCameras,
  png: Uint8Array,
): string {
  const ordered = Object.keys(cameras).sort().map((view) => [view, cameras[view]]);
  return createHash('sha256')
    .update(JSON.stringify(['source-camera-units-v1', url, source.wowName, source.wowVariant, ordered]))
    .update(png)
    .digest('hex');
}

export function readCameras(value: unknown, views: readonly string[]): ShotCameras | undefined {
  if (!isRecord(value)) return undefined;
  const cameras: Record<string, Readonly<Record<string, unknown>>> = {};
  for (const view of views) {
    const camera = value[view];
    if (!isRecord(camera)
      || !vector(camera.eye, 3) || !vector(camera.target, 3) || !vector(camera.up, 3)
      || !vector(camera.modelMatrix, 16)
      || typeof camera.modelScale !== 'number' || !Number.isFinite(camera.modelScale) || camera.modelScale <= 0) {
      return undefined;
    }
    cameras[view] = camera;
  }
  return cameras;
}

/** A new export can have a different size while using the same source camera. */
export function scaleCameras(cameras: ShotCameras, modelScale: number): ShotCameras {
  if (!Number.isFinite(modelScale) || modelScale <= 0) throw new Error('export has no valid model scale');
  return Object.fromEntries(Object.entries(cameras).map(([view, camera]) => [view, { ...camera, modelScale }]));
}

function vector(value: unknown, length: number): boolean {
  return Array.isArray(value) && value.length === length
    && value.every((n: unknown) => typeof n === 'number' && Number.isFinite(n));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

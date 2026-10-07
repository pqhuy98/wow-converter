/**
 * Cases for the model snapshot live in each suite folder as `<slug>/<slug>.manifest.json`.
 * Retail and classic are filled by the snapshot test. Mount is the mount regression map.
 */
import { existsSync, readdirSync, readFileSync } from 'fs';
import path from 'path';

export type SnapshotSuite = 'retail' | 'classic' | 'mount';

export interface TextureBakingOptions {
  readonly enabled?: boolean;
  readonly animate?: boolean;
  readonly fps?: number;
  readonly windowMS?: number;
  readonly resolutionScale?: 1 | 0.5;
}

export interface ModelCase {
  readonly base: string;
  readonly weaponR: string;
  readonly weaponL: string;
  readonly size: string;
  readonly mount?: string;
  readonly mountScale?: number;
  readonly seatOffset?: readonly [number, number, number];
  readonly animation?: string;
  readonly textureBaking?: TextureBakingOptions;
}

export interface SnapshotCase extends ModelCase {
  readonly slug: string;
}

export function isWowheadUrl(base: string): boolean {
  return /^https?:\/\//i.test(base) && /wowhead\.com/i.test(base);
}

/** Wowhead cannot reproduce externally attached weapons or mounts. */
export function wantsWowheadShot(suite: SnapshotSuite, testCase: ModelCase): boolean {
  if (suite === 'mount') return false;
  if (!isWowheadUrl(testCase.base)) return false;
  return testCase.weaponR === '' && testCase.weaponL === '' && !testCase.mount;
}

export function readSnapshotCases(suite: SnapshotSuite): SnapshotCase[] {
  const root = path.join('tests', 'snapshot-tests', 'model', suite);
  if (!existsSync(root)) throw new Error(`missing snapshot catalog ${root}`);
  const slugs = readdirSync(root, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort();
  const cases: SnapshotCase[] = [];
  for (const slug of slugs) {
    const file = path.join(root, slug, `${slug}.manifest.json`);
    if (!existsSync(file)) continue;
    const parsed: unknown = JSON.parse(readFileSync(file, 'utf8'));
    if (!isRecord(parsed) || parsed.suite !== suite || parsed.slug !== slug) {
      throw new Error(`${file} slug does not match the folder`);
    }
    const fields = modelCase(parsed.case, file);
    if (suite === 'mount' && fields.mount === undefined) throw new Error(`${file} mount case has no mount`);
    cases.push({ slug, ...fields });
  }
  if (cases.length === 0) throw new Error(`no snapshot cases in ${root}`);
  return cases;
}

export function modelCase(value: unknown, where: string): ModelCase {
  if (!isRecord(value) || typeof value.base !== 'string' || typeof value.weaponR !== 'string' || typeof value.weaponL !== 'string' || typeof value.size !== 'string') {
    throw new Error(`${where} case fields must be strings`);
  }
  const parsed: ModelCase = {
    base: value.base,
    weaponR: value.weaponR,
    weaponL: value.weaponL,
    size: value.size,
    ...(value.textureBaking !== undefined ? { textureBaking: readTextureBaking(value.textureBaking, where) } : {}),
  };
  if (value.mount === undefined || value.mount === '') return parsed;
  if (typeof value.mount !== 'string') throw new Error(`${where} mount must be a string`);
  const seatOffset = readSeat(value.seatOffset, where);
  if (value.mountScale !== undefined && typeof value.mountScale !== 'number') {
    throw new Error(`${where} mountScale must be a number`);
  }
  if (value.animation !== undefined && value.animation !== '' && typeof value.animation !== 'string') {
    throw new Error(`${where} animation must be a string`);
  }
  return {
    ...parsed,
    mount: value.mount,
    ...(typeof value.mountScale === 'number' ? { mountScale: value.mountScale } : {}),
    ...(seatOffset ? { seatOffset } : {}),
    ...(typeof value.animation === 'string' && value.animation !== '' ? { animation: value.animation } : {}),
  };
}

function readTextureBaking(value: unknown, where: string): TextureBakingOptions {
  if (!isRecord(value)) throw new Error(`${where} textureBaking must be an object`);
  if (value.enabled !== undefined && typeof value.enabled !== 'boolean') {
    throw new Error(`${where} textureBaking.enabled must be a boolean`);
  }
  if (value.animate !== undefined && typeof value.animate !== 'boolean') {
    throw new Error(`${where} textureBaking.animate must be a boolean`);
  }
  if (value.fps !== undefined && (typeof value.fps !== 'number' || !Number.isInteger(value.fps) || value.fps < 1 || value.fps > 60)) {
    throw new Error(`${where} textureBaking.fps must be an integer from 1 to 60`);
  }
  if (value.windowMS !== undefined && (typeof value.windowMS !== 'number' || !Number.isInteger(value.windowMS) || value.windowMS < 1 || value.windowMS > 60000)) {
    throw new Error(`${where} textureBaking.windowMS must be an integer from 1 to 60000`);
  }
  if (value.resolutionScale !== undefined && value.resolutionScale !== 1 && value.resolutionScale !== 0.5) {
    throw new Error(`${where} textureBaking.resolutionScale must be 1 or 0.5`);
  }
  return {
    ...(typeof value.enabled === 'boolean' ? { enabled: value.enabled } : {}),
    ...(typeof value.animate === 'boolean' ? { animate: value.animate } : {}),
    ...(typeof value.fps === 'number' ? { fps: value.fps } : {}),
    ...(typeof value.windowMS === 'number' ? { windowMS: value.windowMS } : {}),
    ...(value.resolutionScale === 1 || value.resolutionScale === 0.5 ? { resolutionScale: value.resolutionScale } : {}),
  };
}

function readSeat(value: unknown, where: string): readonly [number, number, number] | undefined {
  if (value === undefined) return undefined;
  if (!Array.isArray(value) || value.length !== 3) throw new Error(`${where} seatOffset must be three numbers`);
  const [x, y, z] = value;
  if (typeof x !== 'number' || typeof y !== 'number' || typeof z !== 'number') {
    throw new Error(`${where} seatOffset must be three numbers`);
  }
  return [x, y, z];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

/**
 * Cases for the model snapshot live in each suite folder as `<slug>/<slug>.manifest.json`.
 * Retail and classic are filled by the snapshot test. Mount is the mount regression map.
 */
import { existsSync, readdirSync, readFileSync } from 'fs';
import path from 'path';

export type SnapshotSuite = 'retail' | 'classic' | 'mount';

export interface ModelCase {
  readonly base: string;
  readonly weaponR: string;
  readonly weaponL: string;
  readonly size: string;
  readonly mount?: string;
  readonly mountScale?: number;
  readonly seatOffset?: readonly [number, number, number];
  readonly animation?: string;
}

export interface SnapshotCase extends ModelCase {
  readonly slug: string;
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

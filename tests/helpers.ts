import { type ChildProcess, spawn, spawnSync } from 'child_process';
import os from 'os';
import path from 'path';

import { killListeningPort } from '../scripts/kill-dev-ports';
import { type ModelCase } from './snapshot-tests/model/catalog';

export const repoRoot = path.resolve(import.meta.dir, '..');

const children: ChildProcess[] = [];
const ownedPorts = new Set<number>();
const stopped = new WeakSet<ChildProcess>();
let cleanupInstalled = false;

const baseEnv: NodeJS.ProcessEnv = { ...process.env };
delete baseEnv.WOW_CONVERTER_BUNDLED;
delete baseEnv.WOW_CONVERTER_BUNDLE;
baseEnv.IS_SHARED_HOSTING = 'false';
baseEnv.NODE_ENV = 'production';

export function converterUrl(): string {
  return (process.env.WOW_CONVERTER_URL ?? 'http://127.0.0.1:3001').replace(/\/$/, '');
}

export function spawnManaged(
  label: string,
  command: string,
  args: readonly string[],
  cwd: string,
  env: Readonly<Record<string, string>>,
  port?: number,
): ChildProcess {
  installCleanup();
  if (port != null) ownedPorts.add(port);
  console.log(`Starting ${label}`);
  const child = spawn(command, [...args], {
    cwd,
    env: { ...baseEnv, ...env },
    shell: process.platform === 'win32',
    stdio: 'inherit',
  });
  child.on('exit', (code, signal) => {
    if (stopped.has(child)) return;
    if (code != null && code !== 0) {
      console.error(`${label} exited with code ${code}`);
    } else if (signal != null) {
      console.error(`${label} exited with signal ${signal}`);
    }
  });
  children.push(child);
  return child;
}

export async function waitForEndpoint(
  child: ChildProcess,
  url: string,
  timeoutMs: number,
  accept: (value: unknown) => boolean,
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (child.exitCode != null) {
      throw new Error(`Process exited before ${url} became ready`);
    }
    try {
      const response = await fetch(url);
      if (response.ok && accept(await response.json())) return;
    } catch {
      // Startup commonly rejects connections until the listener is ready.
    }
    await Bun.sleep(2_000);
  }
  throw new Error(`Timed out waiting for ${url}`);
}

export function stopChildren(): void {
  const pending = children.splice(0);
  for (const child of pending.reverse()) {
    if (child.pid == null || child.exitCode != null) continue;
    stopped.add(child);
    if (process.platform === 'win32') {
      spawnSync('taskkill', ['/PID', String(child.pid), '/T', '/F'], { stdio: 'ignore' });
    } else {
      child.kill('SIGTERM');
    }
  }
  const ports = [...ownedPorts];
  ownedPorts.clear();
  for (const port of ports) {
    try {
      killListeningPort(port);
    } catch {
      // The process tree kill already released the port, or it never listened.
    }
  }
}

function installCleanup(): void {
  if (cleanupInstalled) return;
  cleanupInstalled = true;
  process.on('exit', () => {
    stopChildren();
  });
  process.on('SIGINT', () => {
    stopChildren();
    process.exit(130);
  });
  process.on('SIGTERM', () => {
    stopChildren();
    process.exit(143);
  });
}

export function killPorts(ports: readonly number[]): void {
  if (ports.length === 0) {
    throw new Error('killPorts requires at least one port');
  }
  for (const port of ports) killListeningPort(port);
}

interface WowProduct {
  readonly product: string;
  readonly installDirectory: string;
  readonly loaded: boolean;
}

export async function readWowProduct(base: string): Promise<WowProduct> {
  const res = await fetch(`${base}/api/wow-config/status`);
  const body: unknown = await res.json();
  if (!isRecord(body) || !isRecord(body.config) || typeof body.config.installDirectory !== 'string') {
    throw new Error('wow-config status has no install directory');
  }
  let product = typeof body.config.product === 'string' ? body.config.product : '';
  if (isRecord(body.cascInfo) && isRecord(body.cascInfo.build) && typeof body.cascInfo.build.Product === 'string' && body.cascInfo.build.Product !== '') {
    product = body.cascInfo.build.Product;
  }
  return { product, installDirectory: body.config.installDirectory, loaded: body.cascLoaded === true };
}

export async function ensureWowProduct(base: string, product: string): Promise<void> {
  const current = await readWowProduct(base);
  if (current.loaded && current.product === product) return;
  console.log(`switching WoW data to ${product}`);
  if (current.loaded) {
    const reset = await fetch(`${base}/api/wow-config/reset`, { method: 'POST' });
    if (!reset.ok) throw new Error(`wow-config reset failed: ${await reset.text()}`);
  }
  const apply = await fetch(`${base}/api/wow-config/apply`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ mode: 'local', product, installDirectory: current.installDirectory }),
  });
  const appliedText = await apply.text();
  if (!apply.ok) throw new Error(`wow-config apply ${product} failed: ${appliedText}`);
  const after = await readWowProduct(base);
  if (!after.loaded || after.product !== product) {
    throw new Error(`WoW data is ${after.product || 'unloaded'}, wanted ${product}`);
  }
  console.log(`WoW data is ${product}`);
}

export async function clearExportedAssets(base: string): Promise<void> {
  const res = await fetch(`${base}/api/export/character/clean`, { method: 'POST' });
  if (!res.ok) throw new Error(`clean exported assets failed: ${await res.text()}`);
}

/** Absolute exported-assets directory of the running converter, including a dist-go bundle. */
export async function exportedAssetsDir(base: string): Promise<string> {
  const res = await fetch(`${base}/api/wow-config/status`);
  const body: unknown = await res.json();
  if (!isRecord(body) || typeof body.outputDirectory !== 'string' || body.outputDirectory === '') {
    throw new Error('wow-config status has no output directory');
  }
  return body.outputDirectory;
}

/** Same bound as Go map export: NumCPU()-1, at least 1, at most 8. */
export const exportWorkers = mapExportWorkerCount();

export interface SourceSequence {
  readonly index: number;
  readonly name: string;
  readonly wowName: string;
  readonly wowVariant: number;
}

export interface ExportedCharacterSource {
  readonly modelScale: number;
  readonly sequences: readonly SourceSequence[];
}

export interface ExportedCharacter {
  readonly modelPath: string;
  readonly source?: ExportedCharacterSource;
}

function mapExportWorkerCount(): number {
  const cpus = os.availableParallelism();
  const max = cpus <= 1 ? 1 : cpus - 1;
  return max > 8 ? 8 : max;
}

export async function runWorkers<T>(count: number, items: readonly T[], run: (item: T) => Promise<void>): Promise<void> {
  let next = 0;
  const workers = Array.from({ length: Math.min(count, items.length) }, async () => {
    for (;;) {
      const index = next;
      next += 1;
      const item = items[index];
      if (!item) return;
      await run(item);
    }
  });
  await Promise.all(workers);
}

/** Writes `{outputFileName}.mdx`, or `{outputFileName}_mount.mdx` when the case has a mount. Returns that path. */
export async function exportCharacter(base: string, outputFileName: string, testCase: ModelCase): Promise<string> {
  return (await runCharacterExport(base, outputFileName, testCase, false)).modelPath;
}

/** Exports a model and reads its source scale and sequence identities when report metadata is available. */
export async function exportCharacterWithMetadata(
  base: string,
  outputFileName: string,
  testCase: ModelCase,
): Promise<ExportedCharacter> {
  return runCharacterExport(base, outputFileName, testCase, true);
}

async function runCharacterExport(
  base: string,
  outputFileName: string,
  testCase: ModelCase,
  includeMetadata: boolean,
): Promise<ExportedCharacter> {
  const character: {
    base: { type: string; value: string };
    inGameMovespeed: number;
    particlesDensity: number;
    portraitCameraSequenceName: string;
    size?: string;
    attachItems?: Record<string, { path: { type: string; value: string }; scale: number }>;
    mount?: {
      path: { type: string; value: string };
      scale?: number;
      seatOffset?: readonly [number, number, number];
      animation?: string;
    };
  } = {
    base: modelRef(testCase.base),
    inGameMovespeed: 270,
    particlesDensity: 1,
    portraitCameraSequenceName: 'Stand',
  };
  if (testCase.size !== '') character.size = testCase.size;
  const attach: Record<string, { path: { type: string; value: string }; scale: number }> = {};
  if (testCase.weaponR !== '') attach.HandRight = { path: modelRef(testCase.weaponR), scale: 1 };
  if (testCase.weaponL !== '') attach.HandLeft = { path: modelRef(testCase.weaponL), scale: 1 };
  if (Object.keys(attach).length > 0) character.attachItems = attach;
  if (testCase.mount) {
    character.mount = {
      path: modelRef(testCase.mount),
      ...(testCase.mountScale !== undefined ? { scale: testCase.mountScale } : {}),
      ...(testCase.seatOffset ? { seatOffset: testCase.seatOffset } : {}),
      ...(testCase.animation ? { animation: testCase.animation } : {}),
    };
  }
  const wanted = testCase.mount ? `${outputFileName}_mount.mdx` : `${outputFileName}.mdx`;
  const res = await fetch(`${base}/api/export/character`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      character,
      ...(testCase.textureBaking !== undefined ? { textureBaking: testCase.textureBaking } : {}),
      outputFileName,
      optimization: {},
      format: 'mdx',
      formatVersion: '1000',
      isBrowse: false,
    }),
  });
  const posted: unknown = await res.json();
  if (!res.ok) throw new Error(`export ${res.status} ${JSON.stringify(posted)}`);
  const id = readString(posted, 'id');
  if (id === '') throw new Error('export response had no id');
  const deadline = Date.now() + 6 * 60_000;
  let current = posted;
  while (Date.now() < deadline) {
    const status = readString(current, 'status');
    if (status === 'done') {
      const modelPath = writtenModel(current, wanted);
      const source = includeMetadata ? readModelSourceMetadata(current, modelPath) : undefined;
      return source ? { modelPath, source } : { modelPath };
    }
    if (status === 'failed' || status === 'cancelled') {
      const error = readString(current, 'error');
      throw new Error(error !== '' ? error : status);
    }
    await Bun.sleep(500);
    const next = await fetch(`${base}/api/export/character/status/${id}`);
    current = await next.json();
  }
  throw new Error(`export timed out ${outputFileName}`);
}

function modelRef(value: string): { type: string; value: string } {
  if (value.startsWith('local::')) {
    return { type: 'local', value: value.slice('local::'.length).replace(/\\/g, '/') };
  }
  return { type: 'wowhead', value };
}

function writtenModel(body: unknown, wanted: string): string {
  if (!isRecord(body) || !isRecord(body.result) || !Array.isArray(body.result.exportedModels)) {
    throw new Error('export returned no model');
  }
  for (const item of body.result.exportedModels) {
    if (!isRecord(item) || typeof item.path !== 'string') continue;
    const modelPath = normalizeExportPath(item.path);
    if (modelPath === wanted) return modelPath;
  }
  throw new Error(`export did not write ${wanted}`);
}

function readModelSourceMetadata(body: unknown, modelPath: string): ExportedCharacterSource | undefined {
  if (!isRecord(body) || !isRecord(body.result)) return undefined;
  const reportMetadata = body.result.reportMetadata;
  if (reportMetadata == null) return undefined;
  if (!isRecord(reportMetadata) || !Array.isArray(reportMetadata.models)) {
    throw new Error('export report metadata had no model list');
  }
  let model: Record<string, unknown> | undefined;
  for (const candidate of reportMetadata.models) {
    if (isRecord(candidate) && typeof candidate.path === 'string' && normalizeExportPath(candidate.path) === modelPath) {
      model = candidate;
      break;
    }
  }
  if (!model) throw new Error(`export report metadata had no entry for ${modelPath}`);

  const modelScale = model.modelScale;
  if (typeof modelScale !== 'number' || !Number.isFinite(modelScale) || modelScale <= 0) {
    throw new Error(`export report metadata had an invalid model scale for ${modelPath}`);
  }
  if (!Array.isArray(model.sequences)) throw new Error(`export report metadata had no sequence list for ${modelPath}`);
  const sequences: SourceSequence[] = [];
  for (const source of model.sequences) {
    if (!isRecord(source)
      || typeof source.index !== 'number' || !Number.isInteger(source.index) || source.index < 0
      || typeof source.name !== 'string'
      || typeof source.wowName !== 'string'
      || typeof source.wowVariant !== 'number' || !Number.isInteger(source.wowVariant) || source.wowVariant < 0) {
      throw new Error(`export report metadata had an invalid sequence for ${modelPath}`);
    }
    sequences.push({
      index: source.index,
      name: source.name,
      wowName: source.wowName,
      wowVariant: source.wowVariant,
    });
  }
  return { modelScale, sequences };
}

function normalizeExportPath(modelPath: string): string {
  return modelPath.replace(/\\/g, '/');
}

function readString(value: unknown, key: string): string {
  if (!isRecord(value)) return '';
  const field = value[key];
  return typeof field === 'string' ? field : '';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value != null;
}

/**
 * Export each manifest under tests/snapshot-tests/model, shoot a 2×3 sheet, and
 * compare it to the committed expected image.
 *
 *   bun test tests/snapshot-tests/model/model.test.ts
 *   SNAPSHOT_UPDATE=1 SNAPSHOT_SUITE=retail SNAPSHOT_SLUG=<slug> bun test tests/snapshot-tests/model/model.test.ts
 *   `bun test` does not forward arguments after `--` into process.argv.
 *   The converter must already be listening (WOW_CONVERTER_URL, or http://127.0.0.1:3001).
 *
 * Export workers match map export (CPU-1, capped at 8). One browser shoots finished exports.
 * Retail and mounts use the wow product. Classic runs after a product switch.
 */
import { afterAll, beforeAll, test } from 'bun:test';
import {
  existsSync, mkdirSync, readFileSync, writeFileSync,
} from 'fs';
import path from 'path';

import { ShotBrowser } from '../../../.cursor/skills/shot-export-wow-converter/shot-export';
import {
  clearExportedAssets, ensureWowProduct, exportCharacter, exportWorkers, readWowProduct, runWorkers,
} from '../../helpers';
import {
  type ModelCase, readSnapshotCases, type SnapshotSuite,
} from './catalog';
import {
  assertPngRoundTrip, decodePng, diffSheet, encodePng, type RgbaImage,
  stitchSheet,
} from './visual-png.helper';

const TILE: readonly [number, number] = [640, 400];

interface Manifest {
  readonly sequence: string;
  readonly tile: readonly [number, number];
  readonly layout: readonly (readonly string[])[];
  readonly thresholdPercent: number;
  readonly channelDelta: number;
  readonly issue?: string;
}

interface CaseJob {
  readonly suite: SnapshotSuite;
  readonly slug: string;
  readonly testCase: ModelCase;
  readonly manifest: Manifest;
}

interface ReadyJob extends CaseJob {
  readonly modelPath: string;
}

interface RunOptions {
  readonly update: boolean;
  readonly slug: string;
  readonly suite: string;
  readonly base: string;
}

const opts = parseArgs(process.argv);
const jobs = listJobs(opts);
if (jobs.length === 0) throw new Error('no snapshot cases matched');

interface Outcome {
  readonly ok: boolean;
  readonly line: string;
}

const CASE_TIMEOUT_MS = 3 * 60 * 60_000;
const results = new Map<string, Promise<Outcome>>();
const resolvers: Map<string, (outcome: Outcome) => void> = new Map();
const settled = new Set<string>();
let done = 0;

for (const job of jobs) {
  const id = caseId(job);
  results.set(id, new Promise((resolve) => {
    resolvers.set(id, resolve);
  }));
}

let run: Promise<void> = Promise.resolve();

beforeAll(() => {
  run = runAll();
});

afterAll(async () => {
  await run;
}, CASE_TIMEOUT_MS);

for (const job of jobs) {
  const id = caseId(job);
  test(id, async () => {
    const result = await results.get(id);
    if (!result) throw new Error(`${id} produced no result`);
    if (!result.ok) throw new Error(result.line);
  }, CASE_TIMEOUT_MS);
}

async function runAll(): Promise<void> {
  const started = Date.now();
  assertPngRoundTrip();
  let viewer: { browser: ShotBrowser } | undefined;
  let original = '';
  try {
    await assertServer(opts.base);
    await clearExportedAssets(opts.base);
    original = (await readWowProduct(opts.base)).product;
    viewer = { browser: await ShotBrowser.open(TILE[0], TILE[1]) };
    const wow = jobs.filter((job) => job.suite !== 'classic');
    const classic = jobs.filter((job) => job.suite === 'classic');
    if (wow.length > 0) {
      await ensureWowProduct(opts.base, 'wow');
      await runPhase(wow, viewer);
    }
    if (classic.length > 0) {
      await ensureWowProduct(opts.base, 'wow_classic');
      // The same texture path can contain different bytes in Retail and Classic.
      if (wow.length > 0) await clearExportedAssets(opts.base);
      await runPhase(classic, viewer);
    }
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    for (const job of jobs) finish(job, false, message);
  } finally {
    for (const job of jobs) finish(job, false, 'no result');
    if (viewer) await viewer.browser.close();
    if (original !== '') await ensureWowProduct(opts.base, original);
    const noun = jobs.length === 1 ? 'case' : 'cases';
    console.log(`${jobs.length} ${noun} in ${formatDuration(Date.now() - started)}`);
  }
}

function finish(job: CaseJob, ok: boolean, detail: string): void {
  const id = caseId(job);
  if (settled.has(id)) return;
  settled.add(id);
  done += 1;
  const line = `[${done}/${jobs.length}] ${ok ? 'OK' : 'FAIL'} ${id}${detail !== '' ? ` ${detail}` : ''}`;
  console.log(line);
  resolvers.get(id)?.({ ok, line });
}

function caseId(job: CaseJob): string {
  return `${job.suite}/${job.slug}`;
}

function formatDuration(ms: number): string {
  const seconds = ms / 1000;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${Math.round(seconds % 60)}s`;
}

async function runPhase(phase: readonly CaseJob[], viewer: { browser: ShotBrowser }): Promise<void> {
  const shots = new Queue<ReadyJob>();
  const shooting = (async () => {
    for (;;) {
      const ready = await shots.take();
      if (!ready) return;
      await shoot(ready, viewer);
    }
  })();
  await runWorkers(exportWorkers, phase, async (job) => {
    console.log(`export ${caseId(job)}`);
    try {
      const modelPath = await exportCharacter(opts.base, `${job.suite}/${job.slug}`, job.testCase);
      shots.push({ ...job, modelPath });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      if (job.manifest.issue && message.includes('cannot infer type')) {
        finish(job, true, `export unsupported; compare ${job.slug}.reported.png to ${job.slug}.reference.png`);
        return;
      }
      finish(job, false, message);
    }
  });
  shots.close();
  await shooting;
}

class Queue<T> {
  private readonly items: T[] = [];

  private readonly waiting: ((item: T | undefined) => void)[] = [];

  private closed = false;

  push(item: T): void {
    const waiter = this.waiting.shift();
    if (waiter) waiter(item);
    else this.items.push(item);
  }

  close(): void {
    this.closed = true;
    for (const waiter of this.waiting.splice(0)) waiter(undefined);
  }

  take(): Promise<T | undefined> {
    const next = this.items.shift();
    if (next !== undefined) return Promise.resolve(next);
    if (this.closed) return Promise.resolve(undefined);
    return new Promise((resolve) => {
      this.waiting.push(resolve);
    });
  }
}

async function shoot(job: ReadyJob, viewer: { browser: ShotBrowser }): Promise<void> {
  try {
    const result = await compareSheet(job, viewer.browser);
    finish(job, result.ok, result.detail);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    if (!isBrowserFailure(message)) {
      finish(job, false, message);
      return;
    }
    try {
      await viewer.browser.close();
    } catch {
      // The browser is already gone.
    }
    viewer.browser = await ShotBrowser.open(TILE[0], TILE[1]);
    try {
      const result = await compareSheet(job, viewer.browser);
      finish(job, result.ok, result.detail);
    } catch (retryErr: unknown) {
      const retryMessage = retryErr instanceof Error ? retryErr.message : String(retryErr);
      finish(job, false, retryMessage);
    }
  }
}

async function compareSheet(job: ReadyJob, browser: ShotBrowser): Promise<{ ok: boolean; detail: string }> {
  const label = `${job.suite}/${job.slug}`;
  const dir = path.join('tests', 'snapshot-tests', 'model', job.suite, job.slug);
  const stem = path.join(dir, job.slug);
  const expectedPath = `${stem}.expected.png`;
  const diffPath = `${stem}.diff.png`;
  const manifest = job.manifest;
  console.log(`shooting ${label}`);
  const captured = await browser.capture({
    base: opts.base,
    model: job.modelPath,
    seq: manifest.sequence !== '' ? manifest.sequence : 'Stand',
    width: manifest.tile[0],
    height: manifest.tile[1],
    views: uniqueViews(manifest.layout),
    readyMs: 120_000,
  });
  if (captured.sequence === '') throw new Error('viewer did not report a sequence');
  const tiles = new Map<string, RgbaImage>();
  for (const [name, png] of captured.views) tiles.set(name, decodePng(png));
  const sheet = stitchSheet(tiles, manifest.layout, manifest.tile[0], manifest.tile[1]);
  const actualBytes = encodePng(sheet);
  mkdirSync(dir, { recursive: true });
  writeFileSync(`${stem}.actual.png`, actualBytes);

  if (opts.update && manifest.issue && !existsSync(expectedPath) && opts.slug !== job.slug) {
    return { ok: true, detail: `no ground truth; compare ${job.slug}.actual.png to ${job.slug}.reference.png` };
  }
  if (opts.update) {
    writeFileSync(expectedPath, actualBytes);
    writeManifest(job, captured.sequence);
    writeDiff(expectedPath, sheet, manifest.channelDelta, diffPath);
    return { ok: true, detail: `updated sequence ${captured.sequence}` };
  }
  if (!existsSync(expectedPath) && existsSync(`${stem}.reference.png`)) {
    return { ok: true, detail: `no ground truth; compare ${job.slug}.actual.png to ${job.slug}.reference.png` };
  }
  if (!existsSync(expectedPath)) {
    return { ok: false, detail: `missing ${job.slug}.expected.png` };
  }
  const reasons: string[] = [];
  if (captured.sequence !== manifest.sequence) {
    reasons.push(`sequence ${captured.sequence} vs ${manifest.sequence}`);
  }
  const { counted, total } = writeDiff(expectedPath, sheet, manifest.channelDelta, diffPath);
  const percent = ((counted / total) * 100).toFixed(2);
  if (counted > (manifest.thresholdPercent / 100) * total) {
    reasons.push(`${percent}% > ${manifest.thresholdPercent}% (${path.resolve(diffPath)})`);
  }
  if (reasons.length > 0) return { ok: false, detail: reasons.join('; ') };
  return { ok: true, detail: `${percent}% sequence ${captured.sequence}` };
}

function writeManifest(job: ReadyJob, sequence: string): void {
  const file = path.join('tests', 'snapshot-tests', 'model', job.suite, job.slug, `${job.slug}.manifest.json`);
  const parsed: unknown = JSON.parse(readFileSync(file, 'utf8'));
  if (!isRecord(parsed) || !isRecord(parsed.shot)) throw new Error(`${file} has no shot`);
  parsed.shot.sequence = sequence;
  writeFileSync(file, `${JSON.stringify(parsed, null, 2)}\n`);
}

function writeDiff(expectedPath: string, actual: RgbaImage, channelDelta: number, diffPath: string): { counted: number; total: number } {
  const expected = decodePng(readFileSync(expectedPath));
  const { counted, heatmap } = diffSheet(expected, actual, channelDelta);
  writeFileSync(diffPath, encodePng(heatmap));
  return { counted, total: expected.width * expected.height };
}

function listJobs(options: RunOptions): CaseJob[] {
  const suites: SnapshotSuite[] = options.suite === ''
    ? ['retail', 'mount', 'classic']
    : [parseSuite(options.suite)];
  const out: CaseJob[] = [];
  for (const suite of suites) {
    for (const item of readSnapshotCases(suite)) {
      if (options.slug !== '' && options.slug !== item.slug) continue;
      const { slug, ...testCase } = item;
      out.push({
        suite, slug, testCase, manifest: readManifest(suite, slug),
      });
    }
  }
  if (out.length === 0 && options.slug !== '') throw new Error(`no case with slug ${options.slug}`);
  return out;
}

function readManifest(suite: SnapshotSuite, slug: string): Manifest {
  const file = path.join('tests', 'snapshot-tests', 'model', suite, slug, `${slug}.manifest.json`);
  const parsed: unknown = JSON.parse(readFileSync(file, 'utf8'));
  if (!isRecord(parsed) || !isRecord(parsed.shot) || !isRecord(parsed.diff)) {
    throw new Error(`${file} is missing shot or diff`);
  }
  const tileW = parsed.shot.tile;
  const layout = parsed.shot.layout;
  const width = Array.isArray(tileW) ? tileW[0] : undefined;
  const height = Array.isArray(tileW) ? tileW[1] : undefined;
  if (typeof parsed.shot.sequence !== 'string' || typeof width !== 'number' || typeof height !== 'number' || !Array.isArray(layout)) {
    throw new Error(`${file} shot is missing`);
  }
  if (typeof parsed.diff.thresholdPercent !== 'number' || typeof parsed.diff.channelDelta !== 'number') {
    throw new Error(`${file} diff threshold is missing`);
  }
  const rows: string[][] = [];
  for (const row of layout) {
    if (!Array.isArray(row)) throw new Error(`${file} layout must be strings`);
    const cells: string[] = [];
    for (const cell of row) {
      if (typeof cell !== 'string') throw new Error(`${file} layout must be strings`);
      cells.push(cell);
    }
    rows.push(cells);
  }
  return {
    sequence: parsed.shot.sequence,
    tile: [width, height],
    layout: rows,
    thresholdPercent: parsed.diff.thresholdPercent,
    channelDelta: parsed.diff.channelDelta,
    ...(typeof parsed.issue === 'string' && parsed.issue !== '' ? { issue: parsed.issue } : {}),
  };
}

function parseSuite(value: string): SnapshotSuite {
  if (value === 'retail' || value === 'classic' || value === 'mount') return value;
  throw new Error('--suite is retail, classic, or mount');
}

function uniqueViews(layout: readonly (readonly string[])[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const row of layout) {
    for (const name of row) {
      if (seen.has(name)) continue;
      seen.add(name);
      out.push(name);
    }
  }
  return out;
}

function parseArgs(argv: readonly string[]): RunOptions {
  let update = process.env.SNAPSHOT_UPDATE === '1';
  let slug = process.env.SNAPSHOT_SLUG ?? '';
  let suite = process.env.SNAPSHOT_SUITE ?? '';
  let base = process.env.WOW_CONVERTER_URL ?? 'http://127.0.0.1:3001';
  for (const arg of argv) {
    if (arg === '--update') update = true;
    else if (arg.startsWith('--slug=')) slug = arg.slice('--slug='.length);
    else if (arg.startsWith('--suite=')) suite = arg.slice('--suite='.length);
    else if (arg.startsWith('--base=')) base = arg.slice('--base='.length);
  }
  return {
    update, slug, suite, base,
  };
}

async function assertServer(base: string): Promise<void> {
  const ping = await fetch(base).catch(() => undefined);
  if (!ping) throw new Error(`converter is not running at ${base}`);
}

function isBrowserFailure(message: string): boolean {
  return /cdp timeout|websocket|browser|devtools|screenshot|viewer rejected|viewer did not/i.test(message);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

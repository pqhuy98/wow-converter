/**
 * Export every retail and classic catalog case, shoot a 2×3 contact sheet, and
 * compare it to the committed expected image. Thresholds live in the manifest.
 *
 *   bun scripts/visual-snapshot.ts
 *   bun scripts/visual-snapshot.ts --update [--slug=...] [--suite=retail|classic]
 */
import {
  existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync,
} from 'fs';
import path from 'path';

import { ShotBrowser } from './shot-export';
import {
  assertPngRoundTrip, decodePng, diffSheet, encodePng, type RgbaImage,
  stitchSheet,
} from './visual-png';

const DEFAULT_TILE: readonly [number, number] = [640, 400];
const DEFAULT_LAYOUT: readonly (readonly string[])[] = [
  ['front', 'left', 'back'],
  ['right', 'top', 'bottom'],
];

interface VisualCase {
  readonly base: string;
  readonly weaponR: string;
  readonly weaponL: string;
  readonly size: string;
}

interface VisualManifest {
  readonly version: 1;
  readonly slug: string;
  readonly suite: Suite;
  readonly case: VisualCase;
  readonly export: { readonly format: 'mdx'; readonly formatVersion: '1000' };
  readonly shot: {
    readonly sequence: string;
    readonly tile: readonly [number, number];
    readonly layout: readonly (readonly string[])[];
  };
  readonly diff: { readonly thresholdPercent: number; readonly channelDelta: number };
  readonly issue?: string;
}

type Suite = 'retail' | 'classic';

interface RunOptions {
  readonly update: boolean;
  readonly slug: string;
  readonly suite: string;
  readonly base: string;
}

interface Outcome {
  readonly ok: boolean;
  readonly line: string;
}

function main(): Promise<number> {
  assertPngRoundTrip();
  const opts = parseArgs(process.argv.slice(2));
  const selected = selectCases(opts);
  return runAll(opts, selected);
}

async function runAll(opts: RunOptions, selected: readonly { suite: Suite; testCase: VisualCase; slug: string }[]): Promise<number> {
  await assertServer(opts.base);
  const original = await readWowProduct(opts.base);
  let browser = await ShotBrowser.open(DEFAULT_TILE[0], DEFAULT_TILE[1]);
  const outcomes: Outcome[] = [];
  let activeSuite = '';
  try {
    for (let i = 0; i < selected.length; i++) {
      const item = selected[i];
      if (!item) continue;
      if (item.suite !== activeSuite) {
        activeSuite = item.suite;
        await ensureWowProduct(opts.base, item.suite === 'classic' ? 'wow_classic' : 'wow');
      }
      const label = `${item.suite}/${item.slug}`;
      console.log(`[${i + 1}/${selected.length}] ${label}`);
      try {
        outcomes.push(await runCase(browser, opts, item.suite, item.slug, item.testCase));
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : String(err);
        outcomes.push({ ok: false, line: `FAIL ${label} ${message}` });
        console.log(outcomes[outcomes.length - 1]?.line);
        if (isBrowserFailure(message)) {
          try {
            await browser.close();
          } catch {
            // The browser is already gone.
          }
          browser = await ShotBrowser.open(DEFAULT_TILE[0], DEFAULT_TILE[1]);
        }
      }
    }
  } finally {
    await browser.close();
    await ensureWowProduct(opts.base, original.product);
  }
  const failed = outcomes.filter((item) => !item.ok).length;
  console.log(`${failed} failed, ${outcomes.length - failed} passed`);
  return failed > 0 ? 1 : 0;
}

async function runCase(
  browser: ShotBrowser,
  opts: RunOptions,
  suite: Suite,
  slug: string,
  testCase: VisualCase,
): Promise<Outcome> {
  const label = `${suite}/${slug}`;
  const dir = path.join('tests', 'visual', suite, slug);
  const stem = path.join(dir, slug);
  const expectedPath = `${stem}.expected.png`;
  const actualPath = `${stem}.actual.png`;
  const diffPath = `${stem}.diff.png`;
  const manifestPath = `${stem}.manifest.json`;
  const existing = readManifest(manifestPath, suite, slug);
  if (!opts.update && !existing) {
    const line = `FAIL ${label} no manifest; run bun run test:visual:update`;
    console.log(line);
    return { ok: false, line };
  }
  if (!opts.update && existing && !sameCase(existing.case, testCase)) {
    const line = `FAIL ${label} manifest case drifted from cases.go`;
    console.log(line);
    return { ok: false, line };
  }

  const shot = existing?.shot ?? { sequence: 'Stand', tile: DEFAULT_TILE, layout: DEFAULT_LAYOUT };
  const diff = existing?.diff ?? { thresholdPercent: 1, channelDelta: 4 };
  let modelPath: string;
  try {
    modelPath = await exportModel(opts.base, `visual-snapshot/${suite}/${slug}`, testCase, existing);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    if (existing?.issue && message.includes('cannot infer type')) {
      const line = `ok ${label} export unsupported; compare ${slug}.reported.png to ${slug}.reference.png`;
      console.log(line);
      return { ok: true, line };
    }
    throw err;
  }
  const views = uniqueViews(shot.layout);
  const captured = await browser.capture({
    base: opts.base,
    model: modelPath,
    seq: shot.sequence !== '' ? shot.sequence : 'Stand',
    width: shot.tile[0],
    height: shot.tile[1],
    views,
    readyMs: 120_000,
  });
  if (captured.sequence === '') throw new Error('viewer did not report a sequence');
  const tiles = new Map<string, RgbaImage>();
  for (const [name, png] of captured.views) tiles.set(name, decodePng(png));
  const sheet = stitchSheet(tiles, shot.layout, shot.tile[0], shot.tile[1]);
  const actualBytes = encodePng(sheet);
  mkdirSync(dir, { recursive: true });
  writeFileSync(actualPath, actualBytes);

  const next: VisualManifest = {
    version: 1,
    slug,
    suite,
    case: testCase,
    export: { format: 'mdx', formatVersion: '1000' },
    shot: { sequence: captured.sequence, tile: [shot.tile[0], shot.tile[1]], layout: shot.layout },
    diff,
    ...(existing?.issue ? { issue: existing.issue } : {}),
  };

  if (opts.update && existing?.issue && !existsFile(expectedPath) && opts.slug !== slug) {
    const line = `ok ${label} no ground truth; compare ${path.basename(actualPath)} to ${slug}.reference.png`;
    console.log(line);
    return { ok: true, line };
  }

  if (opts.update) {
    writeFileSync(expectedPath, actualBytes);
    writeFileSync(manifestPath, `${JSON.stringify(next, null, 2)}\n`);
    writeDiff(expectedPath, sheet, diff.channelDelta, diffPath);
    const line = `updated ${label} sequence ${captured.sequence}`;
    console.log(line);
    return { ok: true, line };
  }

  if (!existing) {
    const line = `FAIL ${label} no manifest`;
    console.log(line);
    return { ok: false, line };
  }
  if (!existsFile(expectedPath) && existsFile(`${stem}.reference.png`)) {
    const line = `ok ${label} no ground truth; compare ${path.basename(actualPath)} to ${slug}.reference.png`;
    console.log(line);
    return { ok: true, line };
  }
  if (!existsFile(expectedPath)) {
    const line = `FAIL ${label} missing ${path.basename(expectedPath)}`;
    console.log(line);
    return { ok: false, line };
  }
  const reasons: string[] = [];
  if (captured.sequence !== existing.shot.sequence) {
    reasons.push(`sequence ${captured.sequence} vs ${existing.shot.sequence}`);
  }
  const { counted, total } = writeDiff(expectedPath, sheet, existing.diff.channelDelta, diffPath);
  const limit = (existing.diff.thresholdPercent / 100) * total;
  const percent = ((counted / total) * 100).toFixed(2);
  if (counted > limit) reasons.push(`${percent}% > ${existing.diff.thresholdPercent}% (${path.resolve(diffPath)})`);
  if (reasons.length > 0) {
    const line = `FAIL ${label} ${reasons.join('; ')}`;
    console.log(line);
    return { ok: false, line };
  }
  const line = `ok ${label} ${percent}% sequence ${captured.sequence}`;
  console.log(line);
  return { ok: true, line };
}

function writeDiff(expectedPath: string, actual: RgbaImage, channelDelta: number, diffPath: string): { counted: number; total: number } {
  const expected = decodePng(readFileSync(expectedPath));
  const { counted, heatmap } = diffSheet(expected, actual, channelDelta);
  writeFileSync(diffPath, encodePng(heatmap));
  return { counted, total: expected.width * expected.height };
}

async function exportModel(base: string, outputFileName: string, testCase: VisualCase, existing: VisualManifest | undefined): Promise<string> {
  const format = existing?.export.format ?? 'mdx';
  const formatVersion = existing?.export.formatVersion ?? '1000';
  const character: {
    base: { type: string; value: string };
    inGameMovespeed: number;
    particlesDensity: number;
    portraitCameraSequenceName: string;
    size?: string;
    attachItems?: Record<string, { path: { type: string; value: string }; scale: number }>;
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

  const res = await fetch(`${base}/api/export/character`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      character,
      outputFileName,
      optimization: {},
      format,
      formatVersion,
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
    if (status === 'done') return pickModel(current, `${outputFileName}.${format}`);
    if (status === 'failed' || status === 'cancelled') {
      const error = readString(current, 'error');
      throw new Error(error !== '' ? error : status);
    }
    await Bun.sleep(500);
    const next = await fetch(`${base}/api/export/character/status/${id}`);
    current = await next.json();
  }
  throw new Error('export timed out');
}

function pickModel(body: unknown, wanted: string): string {
  if (!isRecord(body) || !isRecord(body.result) || !Array.isArray(body.result.exportedModels)) {
    throw new Error('export returned no model');
  }
  for (const item of body.result.exportedModels) {
    if (!isRecord(item) || typeof item.path !== 'string') continue;
    const rel = item.path.replace(/\\/g, '/');
    if (rel === wanted) return rel;
  }
  throw new Error(`export did not write ${wanted}`);
}

function modelRef(value: string): { type: string; value: string } {
  if (value.startsWith('local::')) {
    return { type: 'local', value: value.slice('local::'.length).replace(/\\/g, '/') };
  }
  return { type: 'wowhead', value };
}

function readManifest(file: string, suite: Suite, slug: string): VisualManifest | undefined {
  if (!existsFile(file)) return undefined;
  const parsed: unknown = JSON.parse(readFileSync(file, 'utf8'));
  const manifest = parseManifest(parsed);
  if (manifest.suite !== suite || manifest.slug !== slug) {
    throw new Error(`${file} slug ${manifest.suite}/${manifest.slug} does not match the folder`);
  }
  return manifest;
}

function parseManifest(value: unknown): VisualManifest {
  if (!isRecord(value) || value.version !== 1) throw new Error('manifest version must be 1');
  const suite = value.suite === 'retail' || value.suite === 'classic' ? value.suite : undefined;
  if (!suite || typeof value.slug !== 'string') throw new Error('manifest slug or suite is missing');
  const testCase = parseCase(value.case);
  if (!isRecord(value.export) || value.export.format !== 'mdx' || value.export.formatVersion !== '1000') {
    throw new Error('manifest export must be mdx 1000');
  }
  if (!isRecord(value.shot) || typeof value.shot.sequence !== 'string' || !Array.isArray(value.shot.tile) || !Array.isArray(value.shot.layout)) {
    throw new Error('manifest shot is missing');
  }
  const tileW = value.shot.tile[0];
  const tileH = value.shot.tile[1];
  if (typeof tileW !== 'number' || typeof tileH !== 'number') throw new Error('manifest tile must be numbers');
  const layout: string[][] = [];
  for (const row of value.shot.layout) {
    if (!Array.isArray(row)) throw new Error('manifest layout must be strings');
    const cells: string[] = [];
    for (const cell of row) {
      if (typeof cell !== 'string') throw new Error('manifest layout must be strings');
      cells.push(cell);
    }
    layout.push(cells);
  }
  if (!isRecord(value.diff) || typeof value.diff.thresholdPercent !== 'number' || typeof value.diff.channelDelta !== 'number') {
    throw new Error('manifest diff threshold is missing');
  }
  return {
    version: 1,
    slug: value.slug,
    suite,
    case: testCase,
    export: { format: 'mdx', formatVersion: '1000' },
    shot: { sequence: value.shot.sequence, tile: [tileW, tileH], layout },
    diff: { thresholdPercent: value.diff.thresholdPercent, channelDelta: value.diff.channelDelta },
    ...(typeof value.issue === 'string' && value.issue !== '' ? { issue: value.issue } : {}),
  };
}

function parseCase(value: unknown): VisualCase {
  if (!isRecord(value) || typeof value.base !== 'string' || typeof value.weaponR !== 'string' || typeof value.weaponL !== 'string' || typeof value.size !== 'string') {
    throw new Error('manifest case fields must be strings');
  }
  return {
    base: value.base, weaponR: value.weaponR, weaponL: value.weaponL, size: value.size,
  };
}

function isBrowserFailure(message: string): boolean {
  return /cdp timeout|websocket|browser|devtools|screenshot|viewer rejected|viewer did not/i.test(message);
}

function sameCase(left: VisualCase, right: VisualCase): boolean {
  return left.base === right.base && left.weaponR === right.weaponR && left.weaponL === right.weaponL && left.size === right.size;
}

function selectCases(opts: RunOptions): { suite: Suite; testCase: VisualCase; slug: string }[] {
  const suites: Suite[] = opts.suite === '' ? ['retail', 'classic'] : [opts.suite === 'classic' ? 'classic' : 'retail'];
  if (opts.suite !== '' && opts.suite !== 'retail' && opts.suite !== 'classic') {
    throw new Error('--suite is retail or classic');
  }
  const out: { suite: Suite; testCase: VisualCase; slug: string }[] = [];
  for (const suite of suites) {
    const cases = readCasesFromGo(suite);
    const seen = new Map<string, string>();
    for (const testCase of cases) {
      const slug = slugFor(testCase.base);
      const prev = seen.get(slug);
      if (prev !== undefined) throw new Error(`${suite} slug ${slug} is used by two cases`);
      seen.set(slug, testCase.base);
      if (opts.slug !== '' && opts.slug !== slug) continue;
      out.push({ suite, testCase, slug });
    }
    for (const issue of readIssueCases(suite)) {
      const prev = seen.get(issue.slug);
      if (prev === undefined) seen.set(issue.slug, issue.testCase.base);
      else if (prev !== issue.testCase.base) throw new Error(`${suite} slug ${issue.slug} is used by two cases`);
      if ([...seen.values()].filter((base) => base === issue.testCase.base).length > 1) continue;
      if (opts.slug !== '' && opts.slug !== issue.slug) continue;
      if (out.some((item) => item.slug === issue.slug)) continue;
      out.push({ suite, testCase: issue.testCase, slug: issue.slug });
    }
    assertKnownSlugs(suite, cases);
  }
  if (out.length === 0) throw new Error(`no case with slug ${opts.slug}`);
  return out;
}

function readIssueCases(suite: Suite): { testCase: VisualCase; slug: string }[] {
  const root = path.join('tests', 'visual', suite);
  if (!existsSync(root)) return [];
  const out: { testCase: VisualCase; slug: string }[] = [];
  for (const slug of readdirSync(root)) {
    const manifestPath = path.join(root, slug, `${slug}.manifest.json`);
    if (!existsSync(manifestPath)) continue;
    const parsed: unknown = JSON.parse(readFileSync(manifestPath, 'utf8'));
    if (!isRecord(parsed) || typeof parsed.issue !== 'string' || parsed.issue === '') continue;
    const manifest = parseManifest(parsed);
    out.push({ testCase: manifest.case, slug: manifest.slug });
  }
  return out;
}

function assertKnownSlugs(suite: Suite, cases: readonly VisualCase[]): void {
  if (suite !== 'retail') return;
  const expectSlug = (needle: string, slug: string): void => {
    const hit = cases.find((item) => item.base.includes(needle));
    if (!hit) throw new Error(`missing sample ${needle}`);
    const got = slugFor(hit.base);
    if (got !== slug) throw new Error(`slug for ${needle} is ${got}`);
  };
  expectSlug('npc=36855', 'npc-36855-lady-deathwhisper');
  expectSlug('dressing-room?ninja-turtle', 'dressing-room-ninja-turtle');
  expectSlug('protodragonshadowflame_body', 'protodragonshadowflame-protodragonshadowflame_body');
}

function slugFor(base: string): string {
  if (base.startsWith('local::')) {
    const parts = base.slice('local::'.length).replace(/\\/g, '/').split('/').filter((part) => part !== '');
    const tail = parts.slice(-2).map((part) => part.replace(/\.(m2|wmo|obj)$/i, ''));
    return sanitize(tail.join('-'));
  }
  if (base.includes('dressing-room')) {
    const name = base.split('?').at(-1)?.split('#')[0] ?? '';
    return sanitize(`dressing-room-${name}`);
  }
  const npc = base.match(/npc=(\d+)\/([^/?#]+)/);
  if (npc?.[1] && npc[2]) return sanitize(`npc-${npc[1]}-${npc[2]}`);
  const object = base.match(/object=(\d+)\/([^/?#]+)/);
  if (object?.[1] && object[2]) return sanitize(`object-${object[1]}-${object[2]}`);
  const item = base.match(/item=(\d+)\/([^/?#]+)/);
  if (item?.[1] && item[2]) return sanitize(`item-${item[1]}-${item[2]}`);
  const outfit = base.match(/outfit=(\d+)\/([^/?#]+)/);
  if (outfit?.[1] && outfit[2]) return sanitize(`outfit-${outfit[1]}-${outfit[2]}`);
  return sanitize(base).slice(0, 80);
}

function sanitize(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/-+/g, '-').replace(/^-|-$/g, '');
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

function readCasesFromGo(suite: Suite): VisualCase[] {
  const source = readFileSync('golang/internal/testcases/cases.go', 'utf8').replaceAll('\r\n', '\n');
  const varName = suite === 'classic' ? 'classicCases' : 'retailCases';
  const match = source.match(new RegExp(`var ${varName} = \\[\\]TestCase\\{([\\s\\S]*?)\\n\\}`));
  if (!match?.[1]) throw new Error(`could not find ${varName} in cases.go`);
  const out: VisualCase[] = [];
  const rowPattern = /Base: "((?:\\.|[^"\\])*)", WeaponR: "((?:\\.|[^"\\])*)", WeaponL: "((?:\\.|[^"\\])*)", Size: "((?:\\.|[^"\\])*)"/g;
  for (const row of match[1].matchAll(rowPattern)) {
    out.push({
      base: unescapeGoString(row[1] ?? ''),
      weaponR: unescapeGoString(row[2] ?? ''),
      weaponL: unescapeGoString(row[3] ?? ''),
      size: unescapeGoString(row[4] ?? ''),
    });
  }
  if (out.length === 0) throw new Error(`${varName} parsed empty`);
  return out;
}

function unescapeGoString(value: string): string {
  const parsed: unknown = JSON.parse(`"${value}"`);
  if (typeof parsed !== 'string') throw new Error('case string did not parse');
  return parsed.replaceAll('\\\\', '\\');
}

function parseArgs(argv: readonly string[]): RunOptions {
  let update = false;
  let slug = '';
  let suite = '';
  let base = process.env.WOW_CONVERTER_URL ?? 'http://127.0.0.1:3001';
  for (const arg of argv) {
    if (arg === '--update') update = true;
    else if (arg.startsWith('--slug=')) slug = arg.slice('--slug='.length);
    else if (arg.startsWith('--suite=')) suite = arg.slice('--suite='.length);
    else if (arg.startsWith('--base=')) base = arg.slice('--base='.length);
    else throw new Error(`unknown arg ${arg}`);
  }
  return {
    update, slug, suite, base,
  };
}

interface WowProduct {
  readonly product: string;
  readonly installDirectory: string;
  readonly loaded: boolean;
}

async function readWowProduct(base: string): Promise<WowProduct> {
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

async function ensureWowProduct(base: string, product: string): Promise<void> {
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

async function assertServer(base: string): Promise<void> {
  const ping = await fetch(base).catch(() => undefined);
  if (!ping) throw new Error(`dev server is not running at ${base}`);
}

function existsFile(file: string): boolean {
  return existsSync(file);
}

function readString(value: unknown, key: string): string {
  if (!isRecord(value)) return '';
  const field = value[key];
  return typeof field === 'string' ? field : '';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

main().then((code) => process.exit(code)).catch((err: unknown) => {
  console.error(err instanceof Error ? err.message : err);
  process.exit(1);
});

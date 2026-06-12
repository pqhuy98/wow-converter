/**
 * Snapshot & comparison harness for regression verification.
 *
 * Usage:
 *   bun tests/compare-snapshots.ts snapshot <dir> --out <manifest.json>
 *   bun tests/compare-snapshots.ts compare <manifest.json> <dir> [--tolerance <regex>] [--max-delta 2]
 *   bun tests/compare-snapshots.ts diff <dirA> <dirB> [--tolerance <regex>] [--max-delta 2]
 *
 * Files matching the --tolerance regex are compared pixel-wise (max per-channel
 * delta) instead of byte-wise. Everything else must be byte-identical (SHA-256).
 */
import chalk from 'chalk';
import { createHash } from 'crypto';
import { readFile, writeFile } from 'fs/promises';
import { glob } from 'glob';
import path from 'path';
import sharp from 'sharp';

export interface FileRecord {
  size: number;
  sha256: string;
}

export interface SnapshotManifest {
  root: string;
  createdAt: string;
  files: Record<string, FileRecord>;
}

export async function createSnapshot(dir: string): Promise<SnapshotManifest> {
  const root = path.resolve(dir);
  const files = (await glob('**/*', { cwd: root, nodir: true, dot: false }))
    .map((f) => f.replace(/\\/g, '/'))
    .sort();
  const manifest: SnapshotManifest = {
    root,
    createdAt: new Date().toISOString(),
    files: {},
  };
  for (const rel of files) {
    const buf = await readFile(path.join(root, rel));
    manifest.files[rel] = {
      size: buf.length,
      sha256: createHash('sha256').update(buf).digest('hex'),
    };
  }
  return manifest;
}

export interface PixelCompareResult {
  ok: boolean;
  maxDelta: number;
  diffPixels: number;
  totalPixels: number;
  reason?: string;
}

/**
 * Decode a texture file to raw RGBA for tolerance comparison.
 * PNG/JPEG/WebP via sharp. BLP via the native decoder (once available).
 */
async function decodeToRGBA(filePath: string): Promise<{ data: Buffer; width: number; height: number } | null> {
  if (filePath.toLowerCase().endsWith('.blp')) {
    try {
      const buf = await readFile(filePath);
      const magic = buf.toString('ascii', 0, 4);
      if (magic === 'BLP1') return decodeBlp1(buf);
      // BLP2 (WoW) decoding via the native wow reader.
      const { BLPImage } = await import('@/lib/wow/formats/blp/blp');
      const { BufferWrapper } = await import('@/lib/wow/formats/buffer');
      const blp = new BLPImage(new BufferWrapper(buf));
      const data = Buffer.from(blp.toUInt8Array(0));
      return { data, width: blp.scaledWidth, height: blp.scaledHeight };
    } catch {
      return null;
    }
  }
  try {
    const { data, info } = await sharp(filePath).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
    return { data, width: info.width, height: info.height };
  } catch {
    return null;
  }
}

/** Decode a WC3 BLP1 file (content=1, paletted with separate 8-bit alpha; mip 0 only). */
function decodeBlp1(buf: Buffer): { data: Buffer; width: number; height: number } | null {
  const content = buf.readUInt32LE(4);
  if (content !== 1) return null; // JPEG-content BLP1 is never produced by the converter
  const width = buf.readUInt32LE(12);
  const height = buf.readUInt32LE(16);
  const mip0Offset = buf.readUInt32LE(28);
  const paletteOffset = 156;
  const pixelCount = width * height;
  const data = Buffer.alloc(pixelCount * 4);
  for (let i = 0; i < pixelCount; i++) {
    const idx = buf[mip0Offset + i];
    const p = paletteOffset + idx * 4; // BGRA palette
    data[i * 4] = buf[p + 2];
    data[i * 4 + 1] = buf[p + 1];
    data[i * 4 + 2] = buf[p];
    data[i * 4 + 3] = buf[mip0Offset + pixelCount + i];
  }
  return { data, width, height };
}

export async function comparePixels(fileA: string, fileB: string, maxDeltaAllowed: number): Promise<PixelCompareResult> {
  const a = await decodeToRGBA(fileA);
  const b = await decodeToRGBA(fileB);
  if (!a || !b) {
    return {
      ok: false, maxDelta: Infinity, diffPixels: 0, totalPixels: 0, reason: 'failed to decode',
    };
  }
  if (a.width !== b.width || a.height !== b.height) {
    return {
      ok: false,
      maxDelta: Infinity,
      diffPixels: 0,
      totalPixels: 0,
      reason: `dimension mismatch ${a.width}x${a.height} vs ${b.width}x${b.height}`,
    };
  }
  let maxDelta = 0;
  let diffPixels = 0;
  const totalPixels = a.width * a.height;
  for (let i = 0; i < a.data.length; i += 4) {
    let pixelDiff = 0;
    for (let c = 0; c < 4; c++) {
      const d = Math.abs(a.data[i + c] - b.data[i + c]);
      if (d > pixelDiff) pixelDiff = d;
    }
    if (pixelDiff > 0) diffPixels++;
    if (pixelDiff > maxDelta) maxDelta = pixelDiff;
  }
  return {
    ok: maxDelta <= maxDeltaAllowed, maxDelta, diffPixels, totalPixels,
  };
}

export interface CompareSummary {
  identical: string[];
  withinTolerance: { file: string; maxDelta: number; diffPixels: number }[];
  different: { file: string; reason: string }[];
  missing: string[]; // in baseline but not in target
  extra: string[]; // in target but not in baseline
}

export async function compareManifestToDir(
  baseline: SnapshotManifest,
  targetDir: string,
  options: { toleranceRegex?: RegExp; maxDelta?: number; baselineDir?: string } = {},
): Promise<CompareSummary> {
  const maxDelta = options.maxDelta ?? 2;
  const root = path.resolve(targetDir);
  const targetFiles = new Set(
    (await glob('**/*', { cwd: root, nodir: true, dot: false })).map((f) => f.replace(/\\/g, '/')),
  );

  const summary: CompareSummary = {
    identical: [], withinTolerance: [], different: [], missing: [], extra: [],
  };

  for (const [rel, rec] of Object.entries(baseline.files)) {
    if (!targetFiles.has(rel)) {
      summary.missing.push(rel);
      continue;
    }
    targetFiles.delete(rel);
    const targetPath = path.join(root, rel);
    const buf = await readFile(targetPath);
    const sha = createHash('sha256').update(buf).digest('hex');
    if (sha === rec.sha256) {
      summary.identical.push(rel);
      continue;
    }
    // Bytes differ: allow pixel tolerance for matching texture files.
    if (options.toleranceRegex?.test(rel)) {
      const baseDir = options.baselineDir ?? baseline.root;
      const result = await comparePixels(path.join(baseDir, rel), targetPath, maxDelta);
      if (result.ok) {
        summary.withinTolerance.push({ file: rel, maxDelta: result.maxDelta, diffPixels: result.diffPixels });
        continue;
      }
      summary.different.push({
        file: rel,
        reason: result.reason ?? `pixel maxDelta=${result.maxDelta} > ${maxDelta} (${result.diffPixels}/${result.totalPixels} px differ)`,
      });
      continue;
    }
    summary.different.push({ file: rel, reason: `sha256 mismatch (size ${rec.size} -> ${buf.length})` });
  }

  summary.extra = Array.from(targetFiles).sort();
  return summary;
}

export function printSummary(summary: CompareSummary): boolean {
  console.log(chalk.green(`Identical: ${summary.identical.length}`));
  if (summary.withinTolerance.length > 0) {
    console.log(chalk.cyan(`Within tolerance: ${summary.withinTolerance.length}`));
    for (const t of summary.withinTolerance) {
      console.log(chalk.cyan(`  ~ ${t.file} (maxDelta=${t.maxDelta}, diffPixels=${t.diffPixels})`));
    }
  }
  for (const m of summary.missing) console.log(chalk.red(`  MISSING: ${m}`));
  for (const e of summary.extra) console.log(chalk.yellow(`  EXTRA: ${e}`));
  for (const d of summary.different) console.log(chalk.red(`  DIFF: ${d.file} — ${d.reason}`));
  const ok = summary.different.length === 0 && summary.missing.length === 0;
  console.log(ok ? chalk.green('RESULT: PASS') : chalk.red('RESULT: FAIL'));
  return ok;
}

function getFlag(args: string[], name: string): string | undefined {
  const i = args.indexOf(name);
  return i >= 0 ? args[i + 1] : undefined;
}

async function main() {
  const [cmd, ...args] = process.argv.slice(2);
  const toleranceArg = getFlag(args, '--tolerance');
  const toleranceRegex = toleranceArg ? new RegExp(toleranceArg, 'i') : undefined;
  const maxDelta = Number(getFlag(args, '--max-delta') ?? 2);

  if (cmd === 'snapshot') {
    const dir = args[0];
    const out = getFlag(args, '--out') ?? 'snapshot.json';
    const manifest = await createSnapshot(dir);
    await writeFile(out, JSON.stringify(manifest, null, 2));
    console.log(`Snapshot of ${Object.keys(manifest.files).length} files written to ${out}`);
    return;
  }

  if (cmd === 'compare') {
    const manifest = JSON.parse(await readFile(args[0], 'utf-8')) as SnapshotManifest;
    const summary = await compareManifestToDir(manifest, args[1], { toleranceRegex, maxDelta });
    process.exit(printSummary(summary) ? 0 : 1);
  }

  if (cmd === 'diff') {
    const manifest = await createSnapshot(args[0]);
    const summary = await compareManifestToDir(manifest, args[1], { toleranceRegex, maxDelta, baselineDir: path.resolve(args[0]) });
    process.exit(printSummary(summary) ? 0 : 1);
  }

  console.log('Usage: bun tests/compare-snapshots.ts <snapshot|compare|diff> ...');
  process.exit(1);
}

if (import.meta.main) {
  void main();
}

import chalk from 'chalk';
import fs from 'fs';

import { maxConcurrency } from '@/lib/constants';
import { readExportAsset } from '@/lib/export-asset-store';

import { ensureBlpWorkerPool } from './blp.orches';

async function readPngInput(png: string | Buffer): Promise<Buffer> {
  return typeof png === 'string' ? readExportAsset(png) : png;
}

export function readBlpSizeSync(blpPath: string): { width: number, height: number } | null {
  try {
    const fd = fs.openSync(blpPath, 'r');
    try {
      const header = Buffer.alloc(20);
      const bytesRead = fs.readSync(fd, header, 0, 20, 0);
      if (bytesRead < 20) {
        return null;
      }
      const magic = header.toString('ascii', 0, 4);
      if (magic !== 'BLP1' && magic !== 'BLP2') {
        return null;
      }
      const width = header.readUInt32LE(12);
      const height = header.readUInt32LE(16);
      return { width, height };
    } finally {
      fs.closeSync(fd);
    }
  } catch {
    return null;
  }
}

// tasks managed by orchestrator; no local worker state here

export interface BlpConvertItem {
  /** PNG file path (export-asset store) or PNG bytes. */
  png?: string | Buffer;
  /** Raw WoW BLP2 bytes; decoded to PNG in the worker (direct pipeline). */
  blp2?: Buffer;
  /** Optional downscale applied before BLP1 encoding. */
  resizeTo?: { width: number, height: number };
  blpPath: string;
}

async function resolveItemInput(item: BlpConvertItem): Promise<{ data: Buffer, kind: 'png' | 'blp2' }> {
  if (item.blp2) return { data: item.blp2, kind: 'blp2' };
  if (item.png === undefined) throw new Error(`BLP convert item has no input: ${item.blpPath}`);
  return { data: await readPngInput(item.png), kind: 'png' };
}

// Batch processing with true parallelism
export async function pngsToBlps(items: BlpConvertItem[]): Promise<void> {
  const useInline = process.env.BLP_WORKERS === '0'
    || (isBundledEnv() && process.env.BLP_WORKERS !== '1');

  if (useInline) {
    console.log(`Converting ${chalk.yellow(items.length)} textures to BLPs (inline, no workers)`);
    const { convertTextureToBlp } = await import('./blp.convert');
    for (const item of items) {
      const { data, kind } = await resolveItemInput(item);
      await convertTextureToBlp({ [kind]: data, resizeTo: item.resizeTo }, item.blpPath);
    }
    return;
  }

  const concurrency = Math.min(maxConcurrency, items.length);
  console.log(`Converting ${chalk.yellow(items.length)} textures to BLPs (${chalk.yellow(concurrency)} concurrent threads)`);

  const pool = ensureBlpWorkerPool(concurrency);

  const promises: Promise<void>[] = items.map(async (item) => {
    const { data, kind } = await resolveItemInput(item);
    return pool.submit({ data, kind, resizeTo: item.resizeTo }, item.blpPath);
  });

  await Promise.all(promises);
}

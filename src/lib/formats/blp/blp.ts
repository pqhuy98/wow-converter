import chalk from 'chalk';
import fs from 'fs';
import { readFile } from 'fs/promises';

import { maxConcurrency } from '@/lib/constants';

import { ensureBlpWorkerPool } from './blp.orches';

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

// Batch processing with true parallelism
export async function pngsToBlps(
  items: { png: string | Buffer, blpPath: string }[],
): Promise<void> {
  const concurrency = Math.min(maxConcurrency, items.length);
  console.log(`Converting ${chalk.yellow(items.length)} PNG textures to BLPs (${chalk.yellow(concurrency)} concurrent threads)`);

  const pool = ensureBlpWorkerPool(concurrency);

  const promises: Promise<void>[] = items.map(async (item) => {
    const pngBuffer = typeof item.png === 'string' ? await readFile(item.png) : item.png;
    return pool.submit(pngBuffer, item.blpPath);
  });

  await Promise.all(promises);
}

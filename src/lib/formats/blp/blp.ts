import chalk from 'chalk';
import fs from 'fs';
import { cpus } from 'os';
import { Worker } from 'worker_threads';

// Get CPU core count cross-platform
const maxConcurrency = (() => {
  try {
    const cpuCount = cpus().length;
    return Math.max(1, cpuCount - 1);
  } catch {
    return 4;
  }
})();

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

// Batch processing with true parallelism
export async function pngsToBlps(
  items: { png: string | Buffer, blpPath: string }[],
): Promise<void> {
  const concurrency = Math.min(maxConcurrency, items.length);
  console.log(`Converting ${chalk.yellow(items.length)} PNG textures to BLPs (${chalk.yellow(concurrency)} concurrent threads)`);
  const semaphore = new Array(concurrency).fill(null);
  const queue = [...items];
  const results: Promise<void>[] = [];

  while (queue.length > 0 || semaphore.some((s) => s !== null)) {
    // Wait for a slot to become available
    const availableSlot = semaphore.findIndex((s) => s === null);
    if (availableSlot === -1) {
      await Promise.race(semaphore.filter((s) => s !== null));
      continue;
    }

    if (queue.length === 0) break;

    const item = queue.shift()!;
    const promise = pngToBlpAsync(item.png, item.blpPath)
      .finally(() => {
        semaphore[availableSlot] = null;
      });

    semaphore[availableSlot] = promise;
    results.push(promise);
  }

  await Promise.all(results);
}

async function pngToBlpAsync(png: string | Buffer, blpPath: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const pngBuffer = typeof png === 'string' ? fs.readFileSync(png) : png;

    let workerPath = './blp.worker.ts';
    workerPath = fs.existsSync(workerPath) ? workerPath : new URL(workerPath, import.meta.url).href;
    const worker = new Worker(workerPath, {
      workerData: { pngBuffer, blpPath },
    });

    worker.on('message', (result) => {
      if (result.success) {
        resolve();
      } else {
        reject(new Error(`${result.error}\nblpPath:${blpPath}`));
      }
      void worker.terminate();
    });

    worker.on('error', (error) => {
      reject(error);
      void worker.terminate();
    });

    worker.on('exit', (code) => {
      if (code !== 0) {
        reject(new Error(`Worker stopped with exit code ${code}`));
      }
    });
  });
}

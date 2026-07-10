import fs from 'fs-extra';
import path from 'path';

import { wowDataClient } from '@/lib/wow-data-client/wow-data-client';

import { resetWowConfig } from './wow-config-service';

/** Total byte size of files under the repo `.cache` directory. */
export async function getProjectCacheDirSize(): Promise<number> {
  const cacheDir = path.resolve('.cache');
  if (!(await fs.pathExists(cacheDir))) {
    return 0;
  }

  let total = 0;
  const walk = async (dir: string): Promise<void> => {
    const entries = await fs.readdir(dir, { withFileTypes: true });
    await Promise.all(entries.map(async (entry) => {
      const fullPath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        await walk(fullPath);
        return;
      }
      total += (await fs.stat(fullPath)).size;
    }));
  };
  await walk(cacheDir);
  return total;
}

/** Unload WoW data and delete all files under the repo `.cache` directory. */
export async function clearProjectCacheDir(): Promise<void> {
  await resetWowConfig();

  const cacheDir = path.resolve('.cache');
  if (await fs.pathExists(cacheDir)) {
    await fs.rm(cacheDir, {
      recursive: true,
      force: true,
      maxRetries: 5,
      retryDelay: 200,
    });
  }
  await fs.mkdir(cacheDir, { recursive: true });

  wowDataClient.clearRuntimeCaches();
}

import fs from 'fs-extra';
import path from 'path';

import { wowDataClient } from '@/lib/wow-data-client/wow-data-client';

import { resetWowConfig } from './wow-config-service';

type CacheKind = 'casc' | 'browse';
const cacheDirectories = { casc: '.cache', browse: 'exported-assets-browse' };

/** Total byte size of the selected cache. */
export async function getProjectCacheDirSize(kind: CacheKind): Promise<number> {
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
  const cacheDir = path.resolve(cacheDirectories[kind]);
  if (await fs.pathExists(cacheDir)) {
    await walk(cacheDir);
  }
  return total;
}

/** Clear one cache; unload WoW data only for CASC cleanup. */
export async function clearProjectCacheDir(kind: CacheKind): Promise<void> {
  if (kind === 'casc') await resetWowConfig();

  const cacheDir = path.resolve(cacheDirectories[kind]);
  await fs.rm(cacheDir, {
    recursive: true,
    force: true,
    maxRetries: 5,
    retryDelay: 200,
  });
  await fs.mkdir(cacheDir, { recursive: true });

  if (kind === 'casc') wowDataClient.clearRuntimeCaches();
}

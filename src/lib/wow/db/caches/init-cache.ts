/**
 * Model cache initialization, ported from wow.export (src/js/db/caches/init-cache.js).
 */
import { doOnce } from '../../formats/generics';
import { initializeCreatureData, resetCreatureCache } from './db-creatures';
import { initializeItemDisplays, resetItemDisplayCache } from './db-item-displays';
import { initializeModelFileData, resetModelFileDataCache } from './db-model-file-data';
import { initializeTextureFileData, resetTextureFileDataCache } from './db-texture-file-data';

/** Load DB2 skin caches in dependency order (lower peak memory than parallel init). */
export const ensureModelCachesInitialized = doOnce('ensureModelCachesInitialized', async () => {
  await initializeModelFileData();
  await initializeTextureFileData();
  await Promise.all([
    initializeItemDisplays(),
    initializeCreatureData(),
  ]);
});

export function initModelCaches(): Promise<unknown>[] {
  return [ensureModelCachesInitialized()];
}

export function resetDbCaches(): void {
  resetCreatureCache();
  resetItemDisplayCache();
  resetModelFileDataCache();
  resetTextureFileDataCache();
}

export default { initModelCaches, ensureModelCachesInitialized, resetDbCaches };

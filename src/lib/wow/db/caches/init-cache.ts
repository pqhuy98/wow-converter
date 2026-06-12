/**
 * Model cache initialization, ported from wow.export (src/js/db/caches/init-cache.js).
 */
import { initializeCreatureData, resetCreatureCache } from './db-creatures';
import { initializeItemDisplays, resetItemDisplayCache } from './db-item-displays';
import { initializeModelFileData, resetModelFileDataCache } from './db-model-file-data';
import { initializeTextureFileData, resetTextureFileDataCache } from './db-texture-file-data';

export function initModelCaches(): Promise<unknown>[] {
  return [
    initializeModelFileData(),
    initializeItemDisplays(),
    initializeCreatureData(),
    initializeTextureFileData(),
  ];
}

export function resetDbCaches(): void {
  resetCreatureCache();
  resetItemDisplayCache();
  resetModelFileDataCache();
  resetTextureFileDataCache();
}

export default { initModelCaches, resetDbCaches };

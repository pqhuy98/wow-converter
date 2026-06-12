/**
 * Model cache initialization, ported from wow.export (src/js/db/caches/init-cache.js).
 */
import { initializeCreatureData } from './db-creatures';
import { initializeItemDisplays } from './db-item-displays';
import { initializeModelFileData } from './db-model-file-data';
import { initializeTextureFileData } from './db-texture-file-data';

export function initModelCaches(): Promise<unknown>[] {
  return [
    initializeModelFileData(),
    initializeItemDisplays(),
    initializeCreatureData(),
    initializeTextureFileData(),
  ];
}

export default { initModelCaches };

/**
 * Snapshot of major in-process caches for wow-data-server memory debugging.
 */
import * as listfile from '@/lib/wow/archive/casc/listfile';
import { getCharacterCacheStats } from '@/lib/wow/character/headless-character';
import { getCreatureCacheStats } from '@/lib/wow/db/caches/db-creatures';
import { getItemDisplayCacheStats } from '@/lib/wow/db/caches/db-item-displays';
import { getModelFileDataCacheStats } from '@/lib/wow/db/caches/db-model-file-data';
import { getTextureFileDataCacheStats } from '@/lib/wow/db/caches/db-texture-file-data';

import { runtimeState } from './runtime';

export interface MemoryDiagnostics {
  process: NodeJS.MemoryUsage;
  casc: {
    loaded: boolean;
    rootEntries: number;
    encodingKeys: number;
    encodingSizes: number;
  };
  listfile: ReturnType<typeof listfile.getMemoryStats>;
  dbCaches: {
    creatures: ReturnType<typeof getCreatureCacheStats>;
    items: ReturnType<typeof getItemDisplayCacheStats>;
    modelFileData: ReturnType<typeof getModelFileDataCacheStats>;
    textureFileData: ReturnType<typeof getTextureFileDataCacheStats>;
    character: ReturnType<typeof getCharacterCacheStats>;
  };
}

export function collectMemoryDiagnostics(): MemoryDiagnostics {
  const casc = runtimeState.casc;

  return {
    process: process.memoryUsage(),
    casc: {
      loaded: casc?.isLoaded ?? false,
      rootEntries: casc?.rootEntries.size ?? 0,
      encodingKeys: casc?.encodingKeys.size ?? 0,
      encodingSizes: casc?.encodingSizes.size ?? 0,
    },
    listfile: listfile.getMemoryStats(),
    dbCaches: {
      creatures: getCreatureCacheStats(),
      items: getItemDisplayCacheStats(),
      modelFileData: getModelFileDataCacheStats(),
      textureFileData: getTextureFileDataCacheStats(),
      character: getCharacterCacheStats(),
    },
  };
}

export function formatMemoryDiagnostics(d: MemoryDiagnostics): string {
  const mb = (n: number) => `${(n / 1024 / 1024).toFixed(1)} MB`;
  const p = d.process;
  const lines = [
    `RSS ${mb(p.rss)} | heap ${mb(p.heapUsed)}/${mb(p.heapTotal)} | external ${mb(p.external)} | arrayBuffers ${mb(p.arrayBuffers ?? 0)}`,
    `CASC rootEntries=${d.casc.rootEntries} encodingKeys=${d.casc.encodingKeys}`,
    `Listfile id=${d.listfile.idLookup} name=${d.listfile.nameLookup} preloadedId=${d.listfile.preloadedIdLookup} preloadedName=${d.listfile.preloadedNameLookup}`,
    `DB creatureDisplays=${d.dbCaches.creatures.creatureDisplays} itemDisplays=${d.dbCaches.items.itemDisplays} modelResIDs=${d.dbCaches.modelFileData.modelResIDs} textureMatIDs=${d.dbCaches.textureFileData.matResIDs} characterLookups=${d.dbCaches.character.initialized}`,
  ];
  return lines.join('\n');
}

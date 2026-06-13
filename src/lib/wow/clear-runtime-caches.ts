/**
 * Central registry for WoW-related in-memory caches that must be dropped when
 * CASC is unloaded (soft restart) or when the converter learns the active build changed.
 */
import { clearCharacterBakeCache } from '@/lib/converter/character/wowhead-exporter/character-direct';
import { clearSkeletonGraphCache } from '@/lib/converter/wow-model/direct/m2/bones';
import { cdnResolver } from '@/lib/wow/archive/casc/cdn-resolver';
import { clearNameClientCache } from '@/lib/wow/archive/client/name-client';
import { clearRawClientInFlight } from '@/lib/wow/archive/client/raw-client';
import { resetConverterCasc } from '@/lib/wow/archive/client/remote-casc';
import { resetDbCaches } from '@/lib/wow/db/caches/init-cache';
import { releaseAdtExportBatchMemory } from '@/lib/wow/export/adt/adt-export-memory';
import { resetDoOnceCache } from '@/lib/wow/formats/generics';
import { runWowDataServerClearHooks } from '@/lib/wow/wow-data-server-hooks';

/** In-memory caches owned by the converter (Express) process. */
export function clearConverterRuntimeCaches(): void {
  clearSkeletonGraphCache();
  clearRawClientInFlight();
  clearNameClientCache();
  clearCharacterBakeCache();
  resetConverterCasc();
}

/** In-memory caches owned by wow-data-server after CASC unload / soft restart. */
export function clearWowDataServerRuntimeCaches(): void {
  resetDoOnceCache();
  resetDbCaches();
  releaseAdtExportBatchMemory();
  cdnResolver.clearCache();
  runWowDataServerClearHooks();
}

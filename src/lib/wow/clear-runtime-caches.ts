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
import { ADTExporter } from '@/lib/wow/export/adt/adt-exporter';
import { WMOExporter } from '@/lib/wow/export/wmo/wmo-exporter';
import { resetDoOnceCache } from '@/lib/wow/formats/generics';

const wowDataServerClearHooks = new Set<() => void>();

/** Register an extra clear hook (e.g. REST response memoization on wow-data-server). */
export function registerWowDataServerClearHook(fn: () => void): () => void {
  wowDataServerClearHooks.add(fn);
  return () => wowDataServerClearHooks.delete(fn);
}

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
  ADTExporter.clearCache();
  WMOExporter.clearCache();
  cdnResolver.clearCache();
  for (const fn of wowDataServerClearHooks) fn();
}

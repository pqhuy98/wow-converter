import { write } from '@/lib/wow/log';

import { isCascLoading, unloadCasc as unloadCascState } from './casc-load';

/** Drop in-memory WoW data so a different installation/build can be loaded without exiting the process. */
export async function softRestartRuntime(): Promise<void> {
  if (isCascLoading()) {
    throw new Error('WoW data is still loading');
  }
  write('Soft restart: unloading CASC and clearing caches');
  unloadCascState();
  const { clearWowDataServerRuntimeCaches } = await import('@/lib/wow/clear-runtime-caches');
  clearWowDataServerRuntimeCaches();
}

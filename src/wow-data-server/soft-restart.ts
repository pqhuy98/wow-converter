import { resetDbCaches } from '@/lib/wow/db/caches/init-cache';
import { ADTExporter } from '@/lib/wow/export/adt/adt-exporter';
import { WMOExporter } from '@/lib/wow/export/wmo/wmo-exporter';
import { resetDoOnceCache } from '@/lib/wow/formats/generics';
import { write } from '@/lib/wow/log';

import { isCascLoading, unloadCasc as unloadCascState } from './casc-load';

/** Drop in-memory WoW data so a different installation/build can be loaded without exiting the process. */
export function softRestartRuntime(): void {
  if (isCascLoading()) {
    throw new Error('WoW data is still loading');
  }
  write('Soft restart: unloading CASC and clearing caches');
  unloadCascState();
  resetDoOnceCache();
  resetDbCaches();
  ADTExporter.clearCache();
  WMOExporter.clearCache();
}

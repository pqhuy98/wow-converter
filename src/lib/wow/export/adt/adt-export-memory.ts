/**
 * Release in-process memory retained by ADT batch exports on wow-data-server.
 */
import { WMOExporter } from '../wmo/wmo-exporter';
import { ADTExporter } from './adt-exporter';
import { clearGameObjectsCache } from './map-export-utils';
import { clearBakeTextureCache } from './terrain-baker';

/** Drop per-tile bake caches (safe to call after every tile). */
export function releaseAdtExportTileMemory(): void {
  clearBakeTextureCache();
}

/** Drop batch-scoped caches after a multi-tile export finishes. */
export function releaseAdtExportBatchMemory(): void {
  releaseAdtExportTileMemory();
  WMOExporter.clearCache();
  ADTExporter.clearCache();
  clearGameObjectsCache();
}

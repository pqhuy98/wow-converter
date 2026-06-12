/**
 * Registry mapping texture relative paths (as referenced by MDL materials,
 * relative to wowExportAssetDir, .png extension) to their pixel source in the
 * direct M2->MDX pipeline:
 *  - blp: raw WoW BLP fetched via the raw-file layer (fileDataID)
 *  - png: composited PNG bytes (character bakes, body compositing)
 *
 * The legacy OBJ pipeline does not use this registry (PNG files exist on
 * disk / in the export-asset store instead).
 */
import path from 'path';

export type TextureSource =
  | { kind: 'blp'; fileDataID: number }
  | { kind: 'png'; png: Buffer };

const sources = new Map<string, TextureSource>();

function normalizeKey(relPngPath: string): string {
  return path.normalize(relPngPath).toLowerCase();
}

export function registerTextureSource(relPngPath: string, source: TextureSource): void {
  sources.set(normalizeKey(relPngPath), source);
}

export function getTextureSource(relPngPath: string): TextureSource | undefined {
  return sources.get(normalizeKey(relPngPath));
}

export function hasTextureSource(relPngPath: string): boolean {
  return sources.has(normalizeKey(relPngPath));
}

export function clearTextureSources(): void {
  sources.clear();
}

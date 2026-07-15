/**
 * Registry mapping texture relative paths (as referenced by MDL materials,
 * relative to exportAssetDir, .png extension) to their pixel source in the
 * direct M2->MDX pipeline:
 *  - blp: raw WoW BLP fetched via the raw-file layer (fileDataID)
 *  - png: composited PNG bytes (character bakes, body compositing)
 *
 * ADT terrain uses PNG files on disk; direct M2/WMO use the texture-source registry.
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

export function unregisterTextureSource(relPngPath: string): void {
  sources.delete(normalizeKey(relPngPath));
}

/** Drop registered texture sources for paths from a completed export. */
export function releaseTextureSourcePaths(relativePaths: readonly string[]): number {
  let released = 0;
  for (const rel of relativePaths) {
    if (sources.delete(normalizeKey(rel))) released++;
  }
  return released;
}

/** Drop in-memory PNG sources for paths from a completed export. BLP entries are kept. */
export function releaseGeneratedPngSources(relativePaths: readonly string[]): number {
  let released = 0;
  for (const rel of relativePaths) {
    const key = normalizeKey(rel);
    const source = sources.get(key);
    if (source?.kind !== 'png') continue;
    sources.delete(key);
    released++;
  }
  return released;
}

export function clearTextureSources(): void {
  sources.clear();
}

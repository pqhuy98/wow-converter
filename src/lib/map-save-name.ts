/** Matches maps-generate API validation (alphanumeric, _, ., -, optional .w3x). */
export const MAP_SAVE_NAME_REGEX = /^[a-zA-Z0-9_.-]+(\.w3x)?$/i;

export function stripMapSaveNameExtension(name: string): string {
  let base = name.trim();
  while (/\.w3x$/i.test(base)) {
    base = base.replace(/\.w3x$/i, '');
  }
  return base;
}

export function sanitizeMapSaveNameBase(name: string): string {
  return stripMapSaveNameExtension(name)
    .replace(/[^a-zA-Z0-9_.-]+/g, '_')
    .replace(/_+/g, '_')
    .replace(/^[_.-]+|[_.-]+$/g, '');
}

export function normalizeMapSaveName(name: string): string {
  const base = sanitizeMapSaveNameBase(name);
  if (!base) throw new Error('Map save name is required');
  return `${base}.w3x`;
}

export function buildDefaultMapSaveName(
  mapDir: string,
  tiles: { x: number; y: number }[],
): string {
  if (tiles.length === 0) {
    return normalizeMapSaveName(mapDir);
  }
  const xs = tiles.map((t) => t.x);
  const ys = tiles.map((t) => t.y);
  const minX = Math.min(...xs);
  const maxX = Math.max(...xs);
  const minY = Math.min(...ys);
  const maxY = Math.max(...ys);
  const suffix = minX === maxX && minY === maxY
    ? `${minX}_${minY}`
    : `${minX}_${minY}-${maxX}_${maxY}`;
  const base = sanitizeMapSaveNameBase(mapDir) || 'map';
  return `${base}-${suffix}.w3x`;
}

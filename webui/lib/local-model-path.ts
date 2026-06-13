/** Listfile-style path without model file extension (mirrors server helper). */
export function normalizeLocalModelRef(ref: string): string {
  return ref
    .replace(/\\/g, '/')
    .replace(/\.phys\.(obj|m2)$/i, '')
    .replace(/\.(m2|wmo|obj)$/i, '');
}

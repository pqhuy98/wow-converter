import path from 'path';

import { stripModelReferenceExt } from '@/lib/wow/export/model-reference-path';

/** Listfile-style path without .m2/.wmo (or legacy .phys.*). */
export function normalizeLocalModelRef(ref: string): string {
  return stripModelReferenceExt(ref.replace(/\\/g, '/'));
}

export function localModelBasename(ref: string): string {
  return path.basename(normalizeLocalModelRef(ref));
}

/** Virtual cache file under exportAssetDir for direct M2/WMO conversion. */
export function cachePathForLocalRef(exportAssetDir: string, ref: string, ext: '.m2' | '.wmo'): string {
  return path.join(exportAssetDir, `${normalizeLocalModelRef(ref)}${ext}`);
}

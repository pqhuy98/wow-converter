import { normalizeInstallDirectory } from './normalize-install-directory';

/** Local WoW install directory for CASC auto-load. */
export function getCascLocalWow(): string | undefined {
  const v = process.env.CASC_LOCAL_WOW?.trim();
  return v ? normalizeInstallDirectory(v) : undefined;
}

/** WoW product/build flavor for CASC auto-load (default 'wow' = retail). */
export function getCascLocalProduct(): string {
  return process.env.CASC_LOCAL_PRODUCT?.trim() || 'wow';
}

/** CDN region tag for remote CASC auto-load (e.g. 'eu', 'us'). */
export function getCascRemoteRegion(): string | undefined {
  const v = process.env.CASC_REMOTE_REGION?.trim();
  return v || undefined;
}

/** WoW product for remote CASC auto-load (default 'wow' = retail). */
export function getCascRemoteProduct(): string {
  return process.env.CASC_REMOTE_PRODUCT?.trim() || 'wow';
}

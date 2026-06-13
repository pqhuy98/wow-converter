import dotenv from 'dotenv';
import fs from 'fs';
import path from 'path';

import { bundledAppRoot, isBundledApp } from '@/lib/wow-data-server/transport';

import { normalizeInstallDirectory } from './normalize-install-directory';

let bundledEnvCache: Record<string, string> | null | undefined;

/** CASC-related settings from optional `.env` beside the bundled exe (not process.env). */
function readBundledCascEnv(): Record<string, string> {
  if (bundledEnvCache !== undefined) return bundledEnvCache ?? {};

  const envPath = path.join(bundledAppRoot(), '.env');
  if (!fs.existsSync(envPath)) {
    bundledEnvCache = null;
    return {};
  }

  bundledEnvCache = dotenv.parse(fs.readFileSync(envPath, 'utf8'));
  return bundledEnvCache;
}

function cascEnvValue(key: string): string | undefined {
  if (isBundledApp()) {
    return readBundledCascEnv()[key];
  }
  return process.env[key];
}

/** Local WoW install directory for CASC auto-load. */
export function getCascLocalWow(): string | undefined {
  const v = cascEnvValue('CASC_LOCAL_WOW')?.trim();
  return v ? normalizeInstallDirectory(v) : undefined;
}

/** WoW product/build flavor for CASC auto-load (default 'wow' = retail). */
export function getCascLocalProduct(): string {
  return cascEnvValue('CASC_LOCAL_PRODUCT')?.trim() || 'wow';
}

/** CDN region tag for remote CASC auto-load (e.g. 'eu', 'us'). */
export function getCascRemoteRegion(): string | undefined {
  const v = cascEnvValue('CASC_REMOTE_REGION')?.trim();
  return v || undefined;
}

/** WoW product for remote CASC auto-load (default 'wow' = retail). */
export function getCascRemoteProduct(): string {
  return cascEnvValue('CASC_REMOTE_PRODUCT')?.trim() || 'wow';
}

import {
  getCascLocalProduct, getCascLocalWow, getCascRemoteProduct, getCascRemoteRegion,
} from '@/lib/wow/env';
import { write } from '@/lib/wow/log';

import { loadLocalCascFromInstall, loadRemoteCascFromRegion } from './casc-load';

export interface AutoLoadEnvResult {
  loaded: boolean;
  buildName?: string;
  error?: string;
}

/** Load CASC from .env when configured (local takes priority over remote). */
export async function autoLoadCascFromEnv(): Promise<AutoLoadEnvResult> {
  const localDir = getCascLocalWow();
  const remoteRegion = getCascRemoteRegion();

  if (localDir) {
    const product = getCascLocalProduct();
    console.log(`Auto-loading local CASC from ${localDir} (product: ${product})...`);
    try {
      const casc = await loadLocalCascFromInstall(localDir, product);
      return { loaded: true, buildName: casc.getBuildName() };
    } catch (e) {
      const message = (e as Error).message;
      write('Auto-load of local CASC failed: %s', message);
      console.error('Auto-load of local CASC failed:', message);
      return { loaded: false, error: message };
    }
  }

  if (remoteRegion) {
    const product = getCascRemoteProduct();
    console.log(`Auto-loading remote CASC (region: ${remoteRegion}, product: ${product})...`);
    try {
      const casc = await loadRemoteCascFromRegion(remoteRegion, product);
      return { loaded: true, buildName: casc.getBuildName() };
    } catch (e) {
      const message = (e as Error).message;
      write('Auto-load of remote CASC failed: %s', message);
      console.error('Auto-load of remote CASC failed:', message);
      return { loaded: false, error: message };
    }
  }

  console.log('No CASC_LOCAL_WOW or CASC_REMOTE_REGION set; load CASC via REST (/rest/loadCascLocal or /rest/loadCascRemote).');
  return { loaded: false };
}

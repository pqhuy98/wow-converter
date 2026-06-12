/**
 * wow-data-server entry point.
 *
 * Runs the native WoW data reader (src/lib/wow) in its own process so the
 * converter can hot-reload without re-reading CASC/listfile/DB2 data.
 *
 * Usage:
 *   bun src/wow-data-server/index.ts
 *
 * Env:
 *   WOW_DATA_SERVER_PORT  REST port (default 17753)
 *   CASC_LOCAL_WOW        optional: auto-load local CASC from this WoW install
 *   CASC_LOCAL_PRODUCT    product for local auto-load (default 'wow')
 *   CASC_REMOTE_REGION    optional: auto-load remote CASC (e.g. 'eu', 'us')
 *   CASC_REMOTE_PRODUCT   product for remote auto-load (default 'wow')
 *   WOW_EXPORT_DIR        optional: override exportDirectory (default .cache/wow-export)
 */
import {
  getCascLocalProduct, getCascLocalWow, getCascRemoteProduct, getCascRemoteRegion,
} from '../lib/wow/env';
import { write } from '../lib/wow/log';
import { loadLocalCascFromInstall, loadRemoteCascFromRegion } from './casc-load';
import { WowDataServer } from './rest-server';

async function main(): Promise<void> {
  const server = new WowDataServer();

  const shutdown = (signal: string) => {
    console.log(`\n${signal} received, shutting down wow-data-server...`);
    server.stop();
    process.exit(0);
  };
  process.once('SIGINT', () => shutdown('SIGINT'));
  process.once('SIGTERM', () => shutdown('SIGTERM'));
  // Bun on Windows may emit SIGHUP when the parent terminal closes.
  process.once('SIGHUP', () => shutdown('SIGHUP'));

  const localDir = getCascLocalWow();
  const remoteRegion = getCascRemoteRegion();

  if (localDir) {
    const product = getCascLocalProduct();
    const t0 = Date.now();
    console.log(`Auto-loading local CASC from ${localDir} (product: ${product})...`);
    try {
      const casc = await loadLocalCascFromInstall(localDir, product);
      console.log(`CASC loaded (${casc.getBuildName()}) in ${((Date.now() - t0) / 1000).toFixed(1)}s`);
    } catch (e) {
      write('Auto-load of local CASC failed: %s', (e as Error).message);
      console.error('Auto-load of local CASC failed:', (e as Error).message);
      console.error('Server stays up; load CASC via POST /rest/loadCascLocal + /rest/loadCascBuild.');
    }
  } else if (remoteRegion) {
    const product = getCascRemoteProduct();
    const t0 = Date.now();
    console.log(`Auto-loading remote CASC (region: ${remoteRegion}, product: ${product})...`);
    try {
      const casc = await loadRemoteCascFromRegion(remoteRegion, product);
      console.log(`CASC loaded (${casc.getBuildName()}) in ${((Date.now() - t0) / 1000).toFixed(1)}s`);
    } catch (e) {
      write('Auto-load of remote CASC failed: %s', (e as Error).message);
      console.error('Auto-load of remote CASC failed:', (e as Error).message);
      console.error('Server stays up; load CASC via POST /rest/loadCascRemote + /rest/loadCascBuild.');
    }
  } else {
    console.log('No CASC_LOCAL_WOW or CASC_REMOTE_REGION set; load CASC via REST (/rest/loadCascLocal or /rest/loadCascRemote).');
  }

  // Listen only after auto-load completes so REST clients cannot race startup.
  server.load();
}

main().catch((e) => {
  console.error('wow-data-server failed to start:', e);
  process.exit(1);
});

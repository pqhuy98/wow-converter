/**
 * wow-data-server entry point (dev / VPS — separate process from the converter).
 *
 * Usage:
 *   bun src/wow-data-server/index.ts
 *
 * Env:
 *   WOW_DATA_SERVER_PORT    TCP port when socket transport is off (default 17753)
 *   WOW_DATA_SERVER_SOCKET  optional unix socket path (WOW_DATA_TRANSPORT=socket)
 *   CASC_LOCAL_WOW          optional: auto-load local CASC from this WoW install
 *   CASC_LOCAL_PRODUCT      product for local auto-load (default 'wow')
 *   CASC_REMOTE_REGION      optional: auto-load remote CASC (e.g. 'eu', 'us')
 *   CASC_REMOTE_PRODUCT     product for remote auto-load (default 'wow')
 *   WOW_EXPORT_DIR          optional: override exportDirectory (default .cache/wow-export)
 */
import esMain from 'es-main';

import { startWowDataServer } from '@/lib/wow-data-server/bootstrap';

async function main(): Promise<void> {
  const server = await startWowDataServer();

  const shutdown = (signal: string) => {
    console.log(`\n${signal} received, shutting down wow-data-server...`);
    server.stop();
    process.exit(0);
  };
  process.once('SIGINT', () => shutdown('SIGINT'));
  process.once('SIGTERM', () => shutdown('SIGTERM'));
  process.once('SIGHUP', () => shutdown('SIGHUP'));
}

if (esMain(import.meta)) {
  main().catch((e) => {
    console.error('wow-data-server failed to start:', e);
    process.exit(1);
  });
}

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
import { autoLoadCascFromEnv } from './auto-load-env';
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

  await autoLoadCascFromEnv();

  // Listen only after auto-load completes so REST clients cannot race startup.
  server.load();
}

main().catch((e) => {
  console.error('wow-data-server failed to start:', e);
  process.exit(1);
});

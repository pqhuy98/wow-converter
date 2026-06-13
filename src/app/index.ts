/**
 * Single-binary entry point: starts wow-data-server (unix socket) then the converter UI/API.
 *
 * Dev uses separate processes (`bun run dev`); production build compiles this file only.
 */
import { configureBundledTransport } from '@/lib/wow-data-server/transport';

async function runBundledApp(): Promise<void> {
  configureBundledTransport();

  const { startWowDataServer } = await import('@/lib/wow-data-server/bootstrap');
  const { startConverterServer } = await import('@/server/start');

  const dataServer = await startWowDataServer();

  const shutdown = (signal: string) => {
    console.log(`\n${signal} received, shutting down...`);
    dataServer.stop();
    process.exit(0);
  };
  process.once('SIGINT', () => shutdown('SIGINT'));
  process.once('SIGTERM', () => shutdown('SIGTERM'));
  process.once('SIGHUP', () => shutdown('SIGHUP'));

  await startConverterServer();
}

runBundledApp().catch((e) => {
  console.error('wow-converter failed to start:', e);
  process.exit(1);
});

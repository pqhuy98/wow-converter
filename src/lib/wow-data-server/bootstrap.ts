import { autoLoadCascFromEnv } from '@/wow-data-server/auto-load-env';
import { WowDataServer } from '@/wow-data-server/rest-server';

import { getDataServerSocketPath } from './transport';

export interface StartWowDataServerOptions {
  /** Unix socket path; when omitted uses env / default transport. */
  socketPath?: string;
  /** TCP port when socket transport is disabled. */
  port?: number;
}

/** Start the native WoW data REST server (CASC auto-load from env when configured). */
export async function startWowDataServer(options: StartWowDataServerOptions = {}): Promise<WowDataServer> {
  const server = new WowDataServer();
  if (options.port !== undefined) {
    server.port = options.port;
  }
  await autoLoadCascFromEnv();
  server.load({
    socketPath: options.socketPath ?? getDataServerSocketPath(),
    port: options.port,
  });
  return server;
}

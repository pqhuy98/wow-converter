import fs from 'fs';
import path from 'path';

/** True when running the compiled single-binary distribution. */
export function isBundledApp(): boolean {
  if (process.env.WOW_CONVERTER_BUNDLE === '1') return true;
  const argv0 = process.argv[0] ?? '';
  return argv0.endsWith('.exe') && !argv0.toLowerCase().includes('bun');
}

/** Directory containing the running executable (dist layout when bundled). */
export function bundledAppRoot(): string {
  if (isBundledApp()) {
    return path.dirname(process.execPath);
  }
  return process.cwd();
}

/** Unix socket path for in-process wow-data-server (not exposed on TCP). */
export function defaultSocketPath(): string {
  if (process.env.WOW_DATA_SERVER_SOCKET) {
    return path.resolve(process.env.WOW_DATA_SERVER_SOCKET);
  }
  return path.resolve(bundledAppRoot(), '.cache', 'wow-data-server.sock');
}

export function usesSocketTransport(): boolean {
  return process.env.WOW_DATA_TRANSPORT === 'socket' || Boolean(process.env.WOW_DATA_SERVER_SOCKET);
}

export function getDataServerSocketPath(): string | undefined {
  if (!usesSocketTransport()) return undefined;
  return defaultSocketPath();
}

export function getDataServerHttpOrigin(): string {
  if (usesSocketTransport()) return 'http://localhost';
  const port = process.env.WOW_DATA_SERVER_PORT || 17753;
  return process.env.WOW_DATA_SERVER_URL || `http://127.0.0.1:${port}`;
}

/** Configure bundled app: wow-data-server listens on a unix socket only. */
export function configureBundledTransport(): void {
  process.env.WOW_CONVERTER_BUNDLE = '1';
  process.env.WOW_DATA_TRANSPORT = 'socket';
  process.env.WOW_DATA_SERVER_SOCKET = defaultSocketPath();
  delete process.env.WOW_DATA_SERVER_URL;
  delete process.env.WOW_DATA_SERVER_PORT;
}

export function prepareSocketPath(socketPath: string): void {
  fs.mkdirSync(path.dirname(socketPath), { recursive: true });
  try {
    fs.unlinkSync(socketPath);
  } catch {
    // ignore missing socket file
  }
}

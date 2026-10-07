import path from 'node:path';

const repoRoot = path.resolve(import.meta.dir, '../..');
const baseURL = (process.env.WOW_CONVERTER_URL ?? 'http://127.0.0.1:3001').replace(/\/$/, '');

export interface DataServerEndpoint {
  readonly label: string;
  readonly base: string;
  readonly unix?: string;
}

export async function detectDataServer(): Promise<DataServerEndpoint> {
  const explicitSocket = process.env.WOW_DATA_SERVER_SOCKET?.trim();
  if (explicitSocket) return { label: explicitSocket, base: 'http://localhost', unix: explicitSocket };
  const explicitURL = process.env.WOW_DATA_SERVER_URL?.trim();
  if (explicitURL) {
    const base = explicitURL.replace(/\/$/, '');
    return { label: base, base };
  }
  for (const socket of [
    path.join(repoRoot, '.cache', 'wow-data-server.sock'),
    path.join(repoRoot, 'dist-go', '.cache', 'wow-data-server.sock'),
  ]) {
    try {
      const response = await fetch('http://localhost/rest/getCascInfo', {
        unix: socket,
        signal: AbortSignal.timeout(2_000),
      });
      await response.body?.cancel();
      return { label: socket, base: 'http://localhost', unix: socket };
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      const code = error instanceof Error && 'code' in error ? String(error.code) : '';
      if (!/FailedToOpenSocket|ENOENT|ECONNREFUSED|EACCES/i.test(`${code} ${message}`)) throw error;
    }
  }
  return { label: 'http://127.0.0.1:17753', base: 'http://127.0.0.1:17753' };
}

export async function waitForCascInfo(endpoint: DataServerEndpoint, timeoutMs = 180_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  let last = '';
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${endpoint.base}/rest/getCascInfo`, {
        ...(endpoint.unix ? { unix: endpoint.unix } : {}),
        signal: AbortSignal.timeout(5_000),
      });
      const body: unknown = await response.json();
      if (isRecord(body) && body.id === 'CASC_INFO') return;
      last = JSON.stringify(body).slice(0, 300);
    } catch (error: unknown) {
      last = error instanceof Error ? error.message : String(error);
    }
    await Bun.sleep(1_000);
  }
  throw new Error(`wow-data-server at ${endpoint.label} did not load CASC: ${last}`);
}

export function converterURL(): string {
  return baseURL;
}

export async function api(path: string, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(`${baseURL}${path}`, init);
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`converter is not running at ${baseURL}: ${message}`);
  }
}

export async function ensureCascLoaded(): Promise<void> {
  const deadline = Date.now() + 3 * 60_000;
  let last = '';
  while (Date.now() < deadline) {
    const response = await api('/api/wow-config/status');
    const body: unknown = await response.json();
    if (isRecord(body) && body.cascLoaded === true && body.cascLoading === false && body.wowDataServerReachable === true) {
      return;
    }
    last = JSON.stringify(body).slice(0, 300);
    await Bun.sleep(1_000);
  }
  throw new Error(`converter at ${baseURL} did not finish loading CASC: ${last}`);
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export async function readBody(response: Response): Promise<unknown> {
  const text = await response.text();
  if (response.status >= 400) {
    throw new Error(`${response.status} ${response.url}: ${text.slice(0, 400)}`);
  }
  if (text.trim() === '') return null;
  return JSON.parse(text) as unknown;
}

export function jsonInit(body: unknown): RequestInit {
  return {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  };
}

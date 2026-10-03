import path from 'node:path';

import { beforeAll, expect, setDefaultTimeout, test } from 'bun:test';

import { isRecord } from './client';

setDefaultTimeout(120_000);

const repoRoot = path.resolve(import.meta.dir, '../..');

type Endpoint = { readonly label: string; readonly base: string; readonly unix?: string };

let endpoint: Endpoint;

function dataFetch(requestPath: string, timeoutMs = 60_000): Promise<Response> {
  const signal = AbortSignal.timeout(timeoutMs);
  if (endpoint.unix) return fetch(`${endpoint.base}${requestPath}`, { unix: endpoint.unix, signal });
  return fetch(`${endpoint.base}${requestPath}`, { signal });
}

// stat() on a Windows AF_UNIX socket is EACCES, so the choice is one connect, not a file check.
async function detectEndpoint(): Promise<Endpoint> {
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

const samples = [
  {
    name: 'm2',
    fileName: 'creature/murloc/murloc.m2',
    magic: (bytes: Uint8Array) => text(bytes, 4) === 'MD20' || text(bytes, 4) === 'MD21',
  },
  {
    name: 'blp',
    fileName: 'interface/icons/inv_misc_questionmark.blp',
    magic: (bytes: Uint8Array) => text(bytes, 4) === 'BLP1' || text(bytes, 4) === 'BLP2',
  },
  {
    name: 'db2',
    fileName: 'dbfilesclient/map.db2',
    magic: (bytes: Uint8Array) => text(bytes, 3) === 'WDC',
  },
] as const;

beforeAll(async () => {
  endpoint = await detectEndpoint();
  const deadline = Date.now() + 180_000;
  let last = '';
  while (Date.now() < deadline) {
    try {
      const response = await dataFetch('/rest/getCascInfo', 5_000);
      const body: unknown = await response.json();
      if (isRecord(body) && body.id === 'CASC_INFO') return;
      last = JSON.stringify(body).slice(0, 300);
    } catch (error: unknown) {
      last = error instanceof Error ? error.message : String(error);
    }
    await Bun.sleep(1_000);
  }
  throw new Error(`wow-data-server at ${endpoint.label} did not load CASC: ${last}`);
}, 180_000);

for (const sample of samples) {
  test(`${sample.name} file resolves and downloads`, async () => {
    const listed = await getJSON(`/rest/getFileByName?fileName=${encodeURIComponent(sample.fileName)}`);
    expect(listed.id).toBe('LISTFILE_RESULT');
    expect(listed.fileName).toBe(sample.fileName);
    const fileDataID = listed.fileDataID;
    if (typeof fileDataID !== 'number' || fileDataID <= 0) {
      throw new Error(`${sample.fileName} has no fileDataID`);
    }

    const file = await dataFetch(`/rest/cascFile?fileDataID=${fileDataID}`);
    if (!file.ok) throw new Error(`cascFile ${fileDataID}: HTTP ${file.status}`);
    const bytes = new Uint8Array(await file.arrayBuffer());
    expect(bytes.length).toBeGreaterThan(4);
    expect(sample.magic(bytes)).toBe(true);
  });
}

test('missing file name is rejected', async () => {
  const response = await dataFetch('/rest/getFileByName');
  expect(response.status).toBe(400);
  const body: unknown = await response.json();
  if (!isRecord(body)) throw new Error('error body is not an object');
  expect(body.id).toBe('ERR_INVALID_PARAMETERS');
});

async function getJSON(requestPath: string): Promise<Record<string, unknown>> {
  const response = await dataFetch(requestPath);
  const body: unknown = await response.json();
  if (!response.ok) {
    throw new Error(`${response.status} ${requestPath}: ${JSON.stringify(body).slice(0, 300)}`);
  }
  if (!isRecord(body)) throw new Error(`${requestPath} did not return an object`);
  return body;
}

function text(bytes: Uint8Array, length: number): string {
  return String.fromCharCode(...bytes.subarray(0, length));
}

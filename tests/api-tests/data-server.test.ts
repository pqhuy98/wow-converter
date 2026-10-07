import { beforeAll, expect, setDefaultTimeout, test } from 'bun:test';

import { type DataServerEndpoint, detectDataServer, isRecord, waitForCascInfo } from './client';

setDefaultTimeout(120_000);

let endpoint: DataServerEndpoint;

function dataFetch(requestPath: string, timeoutMs = 60_000): Promise<Response> {
  const signal = AbortSignal.timeout(timeoutMs);
  if (endpoint.unix) return fetch(`${endpoint.base}${requestPath}`, { unix: endpoint.unix, signal });
  return fetch(`${endpoint.base}${requestPath}`, { signal });
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
  endpoint = await detectDataServer();
  await waitForCascInfo(endpoint);
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

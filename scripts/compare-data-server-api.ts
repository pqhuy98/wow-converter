// API parity checker for TS vs Go wow-data-server endpoints (responses normalized where needed).
// Helps ensure server behavior stays compatible while exporter parity work is in progress.
import { createHash } from 'crypto';

type Method = 'GET' | 'POST';

interface CaseDef {
  name: string;
  method: Method;
  path: string;
  query?: Record<string, string | number | boolean>;
  body?: unknown;
  binary?: boolean;
  normalize?: (value: unknown, server: ServerName) => unknown;
  destructive?: boolean;
}

type ServerName = 'ts' | 'go';

interface Result {
  status: number;
  contentType: string;
  body: unknown;
}

const tsBase = process.env.TS_WOW_DATA_SERVER_URL ?? 'http://127.0.0.1:17753';
const goBase = process.env.GO_WOW_DATA_SERVER_URL ?? 'http://127.0.0.1:17754';
const includeDestructive = process.env.INCLUDE_DESTRUCTIVE === '1';

const orcWarriorCharMeta = {
  race: 2,
  gender: 0,
  customizations: {
    20: 384,
    19: 9351,
    21: 402,
    22: 417,
    23: 427,
    876: 9816,
    824: 9269,
    6340: 45094,
    826: 9359,
    877: 9825,
    836: 9428,
    875: 9808,
    874: 9804,
    827: 9365,
    828: 9369,
    829: 9373,
    24: 439,
  },
};

const cases: CaseDef[] = [
  {
    name: 'getCascInfo', method: 'GET', path: '/rest/getCascInfo', normalize: normalizeCascInfo,
  },
  {
    name: 'getConfig.full', method: 'GET', path: '/rest/getConfig', normalize: normalizeConfig,
  },
  {
    name: 'getConfig.single', method: 'GET', path: '/rest/getConfig', query: { key: 'exportDirectory' }, normalize: normalizeConfig,
  },
  {
    name: 'searchFiles.literal', method: 'GET', path: '/rest/searchFiles', query: { search: 'character/orc/male/orcmale_hd.m2' }, normalize: normalizeSearch,
  },
  {
    name: 'searchFiles.regex', method: 'GET', path: '/rest/searchFiles', query: { search: '^character/orc/male/orcmale_hd\\.m2$', useRegularExpression: 1 }, normalize: normalizeSearch,
  },
  {
    name: 'getFileById.model', method: 'GET', path: '/rest/getFileById', query: { fileDataID: 917116 },
  },
  {
    name: 'getFileByName.model', method: 'GET', path: '/rest/getFileByName', query: { fileName: 'character/orc/male/orcmale_hd.m2' },
  },
  {
    name: 'getModelSkins.orcMale', method: 'GET', path: '/rest/getModelSkins', query: { fileDataID: 917116 }, normalize: sortObjectDeep,
  },
  { name: 'initModelCaches', method: 'GET', path: '/rest/initModelCaches' },
  {
    name: 'cascFile.model', method: 'GET', path: '/rest/cascFile', query: { fileDataID: 917116 }, binary: true,
  },
  {
    name: 'cascFile.skinTexture', method: 'GET', path: '/rest/cascFile', query: { fileDataID: 3516381 }, binary: true,
  },
  { name: 'download.invalidMissing', method: 'GET', path: '/rest/download' },
  {
    name: 'debugMemory', method: 'GET', path: '/rest/debugMemory', normalize: normalizeDebugMemory,
  },
  {
    name: 'getMapList', method: 'GET', path: '/rest/getMapList', normalize: normalizeMapList,
  },
  { name: 'exportProgress.missingKey', method: 'GET', path: '/rest/exportProgress' },
  {
    name: 'exportProgress.notFound', method: 'GET', path: '/rest/exportProgress', query: { key: 'parity-missing-key' },
  },
  {
    name: 'loadCascLocal.loadedState', method: 'POST', path: '/rest/loadCascLocal', body: { installDirectory: 'D:/does/not/matter/when/loaded' },
  },
  {
    name: 'loadCascRemote.loadedState', method: 'POST', path: '/rest/loadCascRemote', body: { regionTag: 'us' },
  },
  {
    name: 'loadCascBuild.loadedState', method: 'POST', path: '/rest/loadCascBuild', body: { buildIndex: 0 }, normalize: normalizeCascInfo,
  },
  {
    name: 'setConfig.roundTrip', method: 'POST', path: '/rest/setConfig', body: { key: 'maxTextureSize', value: 512 }, normalize: normalizeConfig,
  },
  {
    name: 'charMeta.orcWarrior', method: 'POST', path: '/rest/charMeta', body: orcWarriorCharMeta, normalize: sortObjectDeep,
  },
  {
    name: 'exportADT.northrendMini',
    method: 'POST',
    path: '/rest/exportADT',
    body: {
      mapID: 571, mapDir: 'Northrend', tileX: 21, tileY: 27, quality: -1, includeM2: false, includeWMO: false, includeWMOSets: false, includeGameObjects: false, includeLiquid: true, includeFoliage: false,
    },
    normalize: normalizeExportADT,
  },
  {
    name: 'finalizeExportProgress.missingKey', method: 'POST', path: '/rest/finalizeExportProgress', body: {},
  },
  {
    name: 'finalizeExportProgress.notFound', method: 'POST', path: '/rest/finalizeExportProgress', body: { key: 'parity-missing-key' },
  },
  {
    name: 'unloadCasc', method: 'POST', path: '/rest/unloadCasc', body: {}, destructive: true,
  },
  {
    name: 'softRestart.reloadEnv', method: 'POST', path: '/rest/softRestart', body: { reloadEnv: true }, normalize: sortObjectDeep, destructive: true,
  },
];

async function main() {
  await assertReady(tsBase, 'ts');
  await assertReady(goBase, 'go');

  const failures: string[] = [];
  for (const testCase of cases) {
    if (testCase.destructive && !includeDestructive) {
      console.log(`SKIP ${testCase.name} (set INCLUDE_DESTRUCTIVE=1)`);
      continue;
    }

    const ts = await runCase(tsBase, testCase);
    const go = await runCase(goBase, testCase);
    const tsNorm = normalizeResult(testCase, ts, 'ts');
    const goNorm = normalizeResult(testCase, go, 'go');
    const equal = stableStringify(tsNorm) === stableStringify(goNorm);
    console.log(`${equal ? 'PASS' : 'FAIL'} ${testCase.name}`);
    if (!equal) {
      failures.push(testCase.name);
      console.log('  TS:', compactStringify(tsNorm));
      console.log('  GO:', compactStringify(goNorm));
    }
  }

  if (failures.length > 0) {
    console.error(`API parity failed (${failures.length}): ${failures.join(', ')}`);
    process.exit(1);
  }
  console.log('API parity passed');
  process.exit(0);
}

async function assertReady(baseURL: string, name: ServerName) {
  const response = await fetch(`${baseURL}/rest/getCascInfo`);
  if (!response.ok) {
    throw new Error(`${name} server is not ready: ${response.status} ${await response.text()}`);
  }
}

async function runCase(baseURL: string, testCase: CaseDef): Promise<Result> {
  const url = new URL(testCase.path, baseURL);
  for (const [key, value] of Object.entries(testCase.query ?? {})) {
    url.searchParams.set(key, String(value));
  }
  const response = await fetch(url, {
    method: testCase.method,
    headers: testCase.method === 'POST' ? { 'content-type': 'application/json' } : undefined,
    body: testCase.method === 'POST' ? JSON.stringify(testCase.body ?? {}) : undefined,
  });
  const contentType = response.headers.get('content-type') ?? '';
  if (testCase.binary || contentType.includes('application/octet-stream')) {
    const data = Buffer.from(await response.arrayBuffer());
    return {
      status: response.status,
      contentType: normalizeContentType(contentType),
      body: {
        length: data.length,
        sha256: createHash('sha256').update(data).digest('hex'),
      },
    };
  }
  return {
    status: response.status,
    contentType: normalizeContentType(contentType),
    body: await safeJSON(response),
  };
}

async function safeJSON(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

function normalizeResult(testCase: CaseDef, result: Result, server: ServerName) {
  return {
    status: result.status,
    contentType: result.contentType,
    body: testCase.normalize ? testCase.normalize(result.body, server) : sortObjectDeep(result.body),
  };
}

function normalizeContentType(contentType: string) {
  return contentType.split(';')[0]?.trim().toLowerCase() ?? '';
}

function normalizeCascInfo(value: unknown) {
  const obj = clonePlainObject(value);
  delete obj.buildConfig;
  obj.build = sortObjectDeep(obj.build);
  return sortObjectDeep(obj);
}

function normalizeConfig(value: unknown) {
  const obj = clonePlainObject(value);
  if (obj.config && typeof obj.config === 'object') {
    const config = clonePlainObject(obj.config);
    delete config.runtimePID;
    obj.config = config;
  }
  return sortObjectDeep(obj);
}

function normalizeSearch(value: unknown) {
  const obj = clonePlainObject(value);
  if (Array.isArray(obj.entries)) {
    obj.entries = [...obj.entries].sort((a, b) => Number(a.fileDataID) - Number(b.fileDataID));
  }
  return sortObjectDeep(obj);
}

function normalizeDebugMemory(value: unknown) {
  const obj = clonePlainObject(value);
  return {
    id: obj.id,
    hasSummary: typeof obj.summary === 'object',
    hasProcess: typeof obj.process === 'object',
    hasCasc: typeof obj.casc === 'object',
    hasListfile: typeof obj.listfile === 'object',
    hasDbCaches: typeof obj.dbCaches === 'object',
  };
}

function normalizeMapList(value: unknown) {
  const obj = clonePlainObject(value);
  if (Array.isArray(obj.maps)) {
    obj.maps = [...obj.maps]
      .sort((a, b) => Number(a.id) - Number(b.id))
      .map((entry) => sortObjectDeep(entry));
  }
  return sortObjectDeep(obj);
}

function normalizeExportADT(value: unknown) {
  const obj = clonePlainObject(value);
  delete obj.exportID;
  delete obj.exportPath;
  return sortObjectDeep(obj);
}

function sortObjectDeep(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(sortObjectDeep);
  if (!value || typeof value !== 'object') return value;
  const record = value as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const key of Object.keys(record).sort()) {
    out[key] = sortObjectDeep(record[key]);
  }
  return out;
}

function clonePlainObject(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
  return { ...(value as Record<string, unknown>) };
}

function stableStringify(value: unknown) {
  return JSON.stringify(sortObjectDeep(value));
}

function compactStringify(value: unknown) {
  const text = stableStringify(value);
  if (text.length <= 4000) return text;
  return `${text.slice(0, 4000)}... <truncated ${text.length - 4000} chars>`;
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});

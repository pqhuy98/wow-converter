// Generates reproducible random local M2/WMO parity cases from the wow-data-server listfile.
import { mkdir, readFile, writeFile } from 'fs/promises';
import path from 'path';

export type RandomModelType = 'm2' | 'wmo';

export interface RandomMdlParityCase {
  outputName: string;
  localRef: string;
  skinId: string;
  fileDataID: number;
  fileName: string;
  modelType: RandomModelType;
}

export interface RandomMdlParityCasesFile {
  seed: number;
  count: number;
  cases: RandomMdlParityCase[];
}

/** @deprecated Use RandomMdlParityCase */
export type RandomM2ParityCase = RandomMdlParityCase;

/** @deprecated Use RandomMdlParityCasesFile */
export type RandomM2ParityCasesFile = RandomMdlParityCasesFile;

interface ListfileEntry {
  fileDataID: number;
  fileName: string;
}

interface ModelSkin {
  id: string;
}

/** Mulberry32 PRNG for reproducible case selection. */
export function createSeededRng(seed: number): () => number {
  let state = seed >>> 0;
  return () => {
    state += 0x6D2B79F5;
    let t = state;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function normalizeLocalRef(fileName: string): string {
  return fileName.replace(/\.(m2|wmo|obj)$/i, '').replace(/\\/g, '/');
}

function sanitizeOutputToken(value: string): string {
  return value
    .replace(/[^a-zA-Z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 80) || 'model';
}

export function deriveRandomMdlOutputName(
  index: number,
  localRef: string,
  skinId: string,
  modelType: RandomModelType,
): string {
  const base = sanitizeOutputToken(path.basename(localRef.replace(/\\/g, '/')));
  const skinSuffix = skinId ? `-${sanitizeOutputToken(skinId)}` : '';
  return `random-mdl-${String(index + 1).padStart(4, '0')}-${base}-${modelType}${skinSuffix}`;
}

/** @deprecated Use deriveRandomMdlOutputName */
export function deriveRandomM2OutputName(index: number, localRef: string, skinId: string): string {
  return deriveRandomMdlOutputName(index, localRef, skinId, 'm2');
}

function isRandomM2Candidate(fileName: string): boolean {
  const lower = fileName.toLowerCase().replace(/\\/g, '/');
  if (!lower.endsWith('.m2')) return false;
  if (lower.includes('.phys.')) return false;
  if (/_lod\d/.test(lower)) return false;
  return true;
}

function isRandomWmoCandidate(fileName: string): boolean {
  const lower = fileName.toLowerCase().replace(/\\/g, '/');
  if (!lower.endsWith('.wmo')) return false;
  if (/_([0-9]{3}|lod\d)\.wmo$/i.test(lower)) return false;
  return true;
}

function modelTypeFromFileName(fileName: string): RandomModelType | null {
  if (isRandomM2Candidate(fileName)) return 'm2';
  if (isRandomWmoCandidate(fileName)) return 'wmo';
  return null;
}

async function waitUntilReady(baseUrl: string): Promise<void> {
  const deadline = Date.now() + 120_000;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`${baseUrl}/rest/getCascInfo`);
      if (res.ok) {
        const json = await res.json() as { id?: string };
        if (json.id === 'CASC_INFO') return;
      }
    } catch {
      // retry
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`wow-data-server not ready: ${baseUrl}`);
}

async function getJSON(baseUrl: string, route: string, params?: Record<string, string>): Promise<Record<string, unknown>> {
  const url = new URL(route, baseUrl);
  if (params) {
    for (const [key, value] of Object.entries(params)) url.searchParams.set(key, value);
  }
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${route} failed with HTTP ${res.status}`);
  return res.json() as Promise<Record<string, unknown>>;
}

async function collectRandomMdlCandidates(baseUrl: string): Promise<ListfileEntry[]> {
  const json = await getJSON(baseUrl, '/rest/collectBrowseFileIndex');
  if (json.id !== 'BROWSE_FILE_INDEX') {
    throw new Error(`collectBrowseFileIndex failed: ${String(json.id)}`);
  }
  const models = json.models as ListfileEntry[];
  return models.filter((entry) => modelTypeFromFileName(entry.fileName) != null);
}

async function getModelSkins(baseUrl: string, fileDataID: number): Promise<ModelSkin[]> {
  const json = await getJSON(baseUrl, '/rest/getModelSkins', { fileDataID: String(fileDataID) });
  if (json.id !== 'MODEL_SKINS') throw new Error(`getModelSkins failed for ${fileDataID}`);
  return (json.skins as ModelSkin[] | undefined) ?? [];
}

function pickUniqueIndices(rng: () => number, total: number, count: number): number[] {
  const picked = new Set<number>();
  const maxAttempts = count * 50;
  for (let attempt = 0; picked.size < count && attempt < maxAttempts; attempt++) {
    picked.add(Math.floor(rng() * total));
  }
  if (picked.size < count) {
    throw new Error(`Could only pick ${picked.size}/${count} unique models from ${total} candidates`);
  }
  return [...picked];
}

export async function generateRandomMdlCases(baseUrl: string, count: number, seed: number): Promise<RandomMdlParityCasesFile> {
  await waitUntilReady(baseUrl);
  await getJSON(baseUrl, '/rest/initModelCaches');

  const models = await collectRandomMdlCandidates(baseUrl);
  if (models.length === 0) throw new Error('No .m2/.wmo models found in listfile browse index');
  if (count > models.length) {
    throw new Error(`Requested ${count} random models but only ${models.length} candidates exist`);
  }

  const rng = createSeededRng(seed);
  const indices = pickUniqueIndices(rng, models.length, count);
  const cases: RandomMdlParityCase[] = [];

  for (let i = 0; i < indices.length; i++) {
    const model = models[indices[i]!]!;
    const modelType = modelTypeFromFileName(model.fileName)!;
    const skins = modelType === 'm2' ? await getModelSkins(baseUrl, model.fileDataID) : [];
    const skin = skins.length > 0 ? skins[Math.floor(rng() * skins.length)]! : undefined;
    const localRef = normalizeLocalRef(model.fileName);
    const skinId = skin?.id ?? '';
    cases.push({
      outputName: deriveRandomMdlOutputName(i, localRef, skinId, modelType),
      localRef,
      skinId,
      fileDataID: model.fileDataID,
      fileName: model.fileName,
      modelType,
    });
  }

  return { seed, count, cases };
}

/** @deprecated Use generateRandomMdlCases */
export async function generateRandomM2Cases(baseUrl: string, count: number, seed: number): Promise<RandomMdlParityCasesFile> {
  return generateRandomMdlCases(baseUrl, count, seed);
}

export async function loadRandomMdlCasesFile(casesFile: string): Promise<RandomMdlParityCasesFile> {
  const raw = await readFile(casesFile, 'utf8');
  const payload = JSON.parse(raw) as RandomMdlParityCasesFile;
  for (const testCase of payload.cases) {
    if (!testCase.modelType) {
      testCase.modelType = testCase.fileName.toLowerCase().endsWith('.wmo') ? 'wmo' : 'm2';
    }
  }
  return payload;
}

/** @deprecated Use loadRandomMdlCasesFile */
export async function loadRandomM2CasesFile(casesFile: string): Promise<RandomMdlParityCasesFile> {
  return loadRandomMdlCasesFile(casesFile);
}

export async function saveRandomMdlCasesFile(casesFile: string, payload: RandomMdlParityCasesFile): Promise<void> {
  await mkdir(path.dirname(casesFile), { recursive: true });
  await writeFile(casesFile, `${JSON.stringify(payload, null, 2)}\n`, 'utf8');
}

/** @deprecated Use saveRandomMdlCasesFile */
export async function saveRandomM2CasesFile(casesFile: string, payload: RandomMdlParityCasesFile): Promise<void> {
  return saveRandomMdlCasesFile(casesFile, payload);
}

export async function ensureRandomMdlCasesFile(
  baseUrl: string,
  casesFile: string,
  count: number,
  seed: number,
  regenerate = false,
): Promise<RandomMdlParityCasesFile> {
  if (!regenerate) {
    try {
      const existing = await loadRandomMdlCasesFile(casesFile);
      if (existing.cases.length > 0) return existing;
    } catch {
      // generate below
    }
  }
  const payload = await generateRandomMdlCases(baseUrl, count, seed);
  await saveRandomMdlCasesFile(casesFile, payload);
  return payload;
}

/** @deprecated Use ensureRandomMdlCasesFile */
export async function ensureRandomM2CasesFile(
  baseUrl: string,
  casesFile: string,
  count: number,
  seed: number,
  regenerate = false,
): Promise<RandomMdlParityCasesFile> {
  return ensureRandomMdlCasesFile(baseUrl, casesFile, count, seed, regenerate);
}

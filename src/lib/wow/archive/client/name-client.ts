/**
 * Cached fileDataID -> listfile name lookups via the data server.
 * Mirrors server-side listfile.getByID semantics (undefined when unknown).
 */
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';

const cache = new Map<number, string | undefined>();
const inFlight = new Map<number, Promise<string | undefined>>();

const idCache = new Map<string, number | undefined>();
const idInFlight = new Map<string, Promise<number | undefined>>();

export function clearNameClientCache(): void {
  cache.clear();
  inFlight.clear();
  idCache.clear();
  idInFlight.clear();
}

export async function getFileNameByID(fileDataID: number): Promise<string | undefined> {
  if (cache.has(fileDataID)) return cache.get(fileDataID);

  const pending = inFlight.get(fileDataID);
  if (pending) return pending;

  const promise = (async () => {
    const entry = await wowExportClient.getFileByID(fileDataID);
    const name = entry.fileName || undefined;
    cache.set(fileDataID, name);
    return name;
  })().finally(() => inFlight.delete(fileDataID));

  inFlight.set(fileDataID, promise);
  return promise;
}

/** Mirrors server-side listfile.getByFilename semantics (undefined when unknown). */
export async function getFileIDByName(fileName: string): Promise<number | undefined> {
  if (idCache.has(fileName)) return idCache.get(fileName);

  const pending = idInFlight.get(fileName);
  if (pending) return pending;

  const promise = (async () => {
    let id: number | undefined;
    try {
      const entry = await wowExportClient.getFileByName(fileName);
      id = entry.fileDataID || undefined;
    } catch {
      id = undefined;
    }
    idCache.set(fileName, id);
    return id;
  })().finally(() => idInFlight.delete(fileName));

  idInFlight.set(fileName, promise);
  return promise;
}

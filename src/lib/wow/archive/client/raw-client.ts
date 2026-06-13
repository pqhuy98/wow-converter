/**
 * Converter-side access to raw WoW files.
 *
 * Resolution order:
 *  1. Local raw cache (.cache/wow/data/<buildKey>/<fileDataID>) — shared with
 *     wow-data-server when both run on the same host.
 *  2. GET /rest/cascFile from the data server, then persist into the cache.
 *
 * Concurrent requests for the same fileDataID are coalesced.
 */
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import { readRawCachedFile, writeRawCachedFile } from './raw-cache';

const inFlight = new Map<string, Promise<Buffer>>();

export function clearRawClientInFlight(): void {
  inFlight.clear();
}

function inFlightKey(buildKey: string, fileDataID: number): string {
  return `${buildKey}:${fileDataID}`;
}

async function currentBuildKey(): Promise<string> {
  await wowExportClient.waitUntilReady();
  const buildKey = wowExportClient.cascInfo?.buildKey;
  if (!buildKey) throw new Error('No CASC build key available from data server');
  return buildKey;
}

/** Fetch the raw (BLTE-decoded) bytes of a WoW file by fileDataID. */
export async function getRawWowFile(fileDataID: number): Promise<Buffer> {
  if (!Number.isFinite(fileDataID) || fileDataID <= 0) {
    throw new Error(`Invalid fileDataID: ${fileDataID}`);
  }

  const buildKey = await currentBuildKey();
  const cached = await readRawCachedFile(buildKey, fileDataID);
  if (cached) return cached;

  const flightKey = inFlightKey(buildKey, fileDataID);
  const pending = inFlight.get(flightKey);
  if (pending) return pending;

  const promise = (async () => {
    const buf = await wowExportClient.downloadCascFile(fileDataID);
    await writeRawCachedFile(buildKey, fileDataID, buf);
    return buf;
  })().finally(() => inFlight.delete(flightKey));

  inFlight.set(flightKey, promise);
  return promise;
}

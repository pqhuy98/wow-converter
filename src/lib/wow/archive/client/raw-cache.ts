/**
 * Shared on-disk cache for raw (BLTE-decoded) WoW files, keyed by build + fileDataID.
 *
 * Layout: .cache/wow/data/<buildKey>/<fileDataID>
 *
 * Both wow-data-server and the converter resolve the same directory (relative to
 * the repo cwd), so when they run on the same host the converter reads files the
 * server already cached without any network hop.
 */
import { randomBytes } from 'crypto';
import fs from 'fs';
import {
  mkdir, readFile, rename, writeFile,
} from 'fs/promises';
import path from 'path';

import { constants } from '@/lib/wow/formats/constants';

const RAW_DATA_DIR = path.join(constants.DATA_PATH, 'data');

export function rawFileCachePath(buildKey: string, fileDataID: number): string {
  return path.join(RAW_DATA_DIR, buildKey, String(fileDataID));
}

/** Read a cached raw file, or null when absent. */
export async function readRawCachedFile(buildKey: string, fileDataID: number): Promise<Buffer | null> {
  try {
    const buf = await readFile(rawFileCachePath(buildKey, fileDataID));
    return buf.length > 0 ? buf : null;
  } catch {
    return null;
  }
}

export function rawCachedFileExistsSync(buildKey: string, fileDataID: number): boolean {
  return fs.existsSync(rawFileCachePath(buildKey, fileDataID));
}

/** Atomically persist a raw file (tmp + rename, safe under concurrent writers). */
export async function writeRawCachedFile(buildKey: string, fileDataID: number, data: Buffer): Promise<void> {
  const dest = rawFileCachePath(buildKey, fileDataID);
  await mkdir(path.dirname(dest), { recursive: true });
  const tmp = `${dest}.${randomBytes(6).toString('hex')}.tmp`;
  try {
    await writeFile(tmp, data);
    await rename(tmp, dest);
  } catch (e) {
    try { fs.unlinkSync(tmp); } catch { /* ignore */ }
    // A concurrent writer may have won the rename race; treat as success if so.
    if (!fs.existsSync(dest)) throw e;
  }
}

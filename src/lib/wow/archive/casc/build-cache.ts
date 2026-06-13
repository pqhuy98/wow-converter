/**
 * Build cache for CDN-downloaded CASC data, ported from wow.export
 * (src/js/casc/build-cache.js). UI cache-size tracking removed.
 */
import { promises as fsp } from 'fs';
import path from 'path';

import { write } from '@/lib/wow/log';

import { BufferWrapper } from '../../formats/buffer';
import { constants } from '../../formats/constants';
import { createDirectory, deleteDirectory, readJSON } from '../../formats/generics';
import { wowConfig } from '../../server/config';

let cacheIntegrity: Record<string, string> | null = null;
let integrityInitPromise: Promise<void> | null = null;
let integritySaveChain: Promise<void> = Promise.resolve();

/** Initialize the cache integrity system (lazy, once). */
async function ensureCacheIntegrity(): Promise<Record<string, string>> {
  if (cacheIntegrity) return cacheIntegrity;
  if (!integrityInitPromise) {
    integrityInitPromise = (async () => {
      try {
        const integrity = await readJSON(constants.CACHE.INTEGRITY_FILE, false);
        if (integrity === null) {
          throw new Error(`File cannot be accessed or contains malformed JSON: ${constants.CACHE.INTEGRITY_FILE}`);
        }
        cacheIntegrity = integrity as Record<string, string>;
      } catch (e) {
        write('Unable to load cache integrity file; entire cache will be invalidated.');
        write((e as Error).message);
        cacheIntegrity = {};
      }
    })();
  }
  await integrityInitPromise;
  return cacheIntegrity!;
}

export class BuildCache {
  key: string;

  meta: { lastAccess?: number } = {};

  cacheDir: string;

  manifestPath: string;

  constructor(key: string) {
    this.key = key;
    this.cacheDir = path.join(constants.CACHE.DIR_BUILDS, key);
    this.manifestPath = path.join(this.cacheDir, constants.CACHE.BUILD_MANIFEST);
  }

  /** Initialize the build cache instance. */
  async init(): Promise<void> {
    // Create cache directory if needed.
    await fsp.mkdir(this.cacheDir, { recursive: true });

    // Load manifest values.
    try {
      const manifest = JSON.parse(await fsp.readFile(this.manifestPath, 'utf8'));
      Object.assign(this.meta, manifest);
    } catch (e) {
      write('No cache manifest found for %s', this.key);
    }

    // Save access update without blocking.
    this.meta.lastAccess = Date.now();
    void this.saveManifest();
  }

  /**
   * Attempt to get a file from this build cache.
   * Returns null if the file is not cached.
   * @param file File path relative to build cache.
   * @param dir Optional override directory.
   */
  async getFile(file: string, dir?: string): Promise<BufferWrapper | null> {
    try {
      const filePath = this.getFilePath(file, dir);

      const integrity = await ensureCacheIntegrity();
      const integrityHash = integrity[filePath];

      // File integrity cannot be verified, reject.
      if (typeof integrityHash !== 'string') {
        write('Cannot verify integrity of file, rejecting cache (%s)', filePath);
        return null;
      }

      const data = await BufferWrapper.readFile(filePath);
      const dataHash = data.calculateHash('sha1', 'hex');

      // Reject cache if hash does not match.
      if (dataHash !== integrityHash) {
        write('Bad integrity for file %s, rejecting cache (%s != %s)', filePath, dataHash, integrityHash);
        return null;
      }

      return data;
    } catch (e) {
      return null;
    }
  }

  /** Get a direct path to a cached file. */
  getFilePath(file: string, dir?: string): string {
    return path.join(dir || this.cacheDir, file);
  }

  /**
   * Store a file in this build cache.
   * @param file File path relative to build cache.
   * @param data Data to store in the file.
   * @param dir Optional override directory.
   */
  async storeFile(file: string, data: BufferWrapper, dir?: string): Promise<void> {
    if (!(data instanceof BufferWrapper)) throw new Error('Data provided to cache.storeFile() must be of BufferWrapper type.');

    const filePath = this.getFilePath(file, dir);
    if (dir) await createDirectory(path.dirname(filePath));

    const integrity = await ensureCacheIntegrity();

    // Integrity checking.
    const hash = data.calculateHash('sha1', 'hex');
    integrity[filePath] = hash;

    await fsp.writeFile(filePath, data.raw);

    await this.saveCacheIntegrity();
  }

  /** Save the cache integrity to disk (serialized to avoid concurrent write races). */
  async saveCacheIntegrity(): Promise<void> {
    await createDirectory(path.dirname(constants.CACHE.INTEGRITY_FILE));
    const snapshot = { ...cacheIntegrity };
    integritySaveChain = integritySaveChain.then(() => fsp.writeFile(
      constants.CACHE.INTEGRITY_FILE,
      JSON.stringify(snapshot),
      'utf8',
    ));
    await integritySaveChain;
  }

  /** Save the manifest for this build cache. */
  async saveManifest(): Promise<void> {
    await fsp.writeFile(this.manifestPath, JSON.stringify(this.meta), 'utf8');
  }
}

/**
 * Run clean-up for stale build caches (mirrors wow.export's
 * 'casc-source-changed' clean-up hook; call after a CASC source is selected).
 */
export async function runCacheCleanup(): Promise<void> {
  let cacheExpire = Number(wowConfig.cacheExpiry) || 0;
  cacheExpire *= 24 * 60 * 60 * 1000;

  // If user sets cacheExpiry to 0 in the configuration, we completely
  // skip the clean-up process.
  if (cacheExpire === 0) {
    write('WARNING: Cache clean-up has been skipped due to cacheExpiry being %d', cacheExpire);
    return;
  }

  write('Running clean-up for stale build caches...');
  let entries;
  try {
    entries = await fsp.readdir(constants.CACHE.DIR_BUILDS, { withFileTypes: true });
  } catch (e) {
    return; // No cache directory yet.
  }
  const ts = Date.now();

  for (const entry of entries) {
    // We only care about directories with MD5 names.
    if (!entry.isDirectory() || entry.name.length !== 32) continue;

    let deleteEntry = false;
    const entryDir = path.join(constants.CACHE.DIR_BUILDS, entry.name);
    const entryManifest = path.join(entryDir, constants.CACHE.BUILD_MANIFEST);

    try {
      const manifestRaw = await fsp.readFile(entryManifest, 'utf8');
      const manifest = JSON.parse(manifestRaw) as { lastAccess?: number };

      if (manifest.lastAccess !== undefined && !Number.isNaN(manifest.lastAccess)) {
        const delta = ts - manifest.lastAccess;
        if (delta > cacheExpire) {
          deleteEntry = true;
          write('Build cache %s has expired (%d), marking for deletion.', entry.name, delta);
        }
      } else {
        deleteEntry = true;
        write('Unable to read lastAccess from %s, marking for deletion.', entry.name);
      }
    } catch (e) {
      // Manifest is missing or malformed.
      deleteEntry = true;
      write('Unable to read manifest for %s, marking for deletion.', entry.name);
    }

    if (deleteEntry) await deleteDirectory(entryDir);
  }
}

export default BuildCache;

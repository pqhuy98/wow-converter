/**
 * TACT decryption key management, ported from wow.export (src/js/casc/tact-keys.js).
 */
import { promises as fsp } from 'fs';
import path from 'path';

import { write } from '@/lib/wow/log';

import { constants } from '../../formats/constants';
import { get } from '../../formats/generics';
import { wowConfig } from '../../server/config';

const KEY_RING: Record<string, string> = {};
let isSaving = false;

/** Retrieve a registered decryption key. */
export function getKey(keyName: string): string | undefined {
  return KEY_RING[keyName.toLowerCase()];
}

/** Validate a keyName/key pair. */
function validateKeyPair(keyName: string, key: string): boolean {
  if (keyName.length !== 16) return false;
  if (key.length !== 32) return false;
  return true;
}

/**
 * Add a decryption key. Subject to validation.
 * Decryption keys will be saved to disk on next tick.
 * Returns true if added, else false if the pair failed validation.
 */
export function addKey(keyName: string, key: string): boolean {
  if (!validateKeyPair(keyName, key)) return false;

  const normalizedName = keyName.toLowerCase();
  const normalizedKey = key.toLowerCase();

  if (KEY_RING[normalizedName] !== normalizedKey) {
    KEY_RING[normalizedName] = normalizedKey;
    write('Registered new decryption key %s -> %s', normalizedName, normalizedKey);
    void save();
  }

  return true;
}

/**
 * Load tact keys from disk cache and request updated keys from remote server.
 */
export async function load(): Promise<void> {
  // Load from local cache.
  try {
    const tactKeys = JSON.parse(await fsp.readFile(constants.CACHE.TACT_KEYS, 'utf8')) as Record<string, string>;

    // Validate/add our cached keys manually rather than passing to addKey()
    // to skip over redundant logging/saving calls.
    let added = 0;
    for (const [keyName, key] of Object.entries(tactKeys)) {
      if (validateKeyPair(keyName, key)) {
        KEY_RING[keyName.toLowerCase()] = key.toLowerCase();
        added++;
      } else {
        write('Skipping invalid tact key from cache: %s -> %s', keyName, key);
      }
    }

    write('Loaded %d tact keys from local cache.', added);
  } catch (e) {
    // No tactKeys cached locally, doesn't matter.
  }

  const tactUrl = wowConfig.tactKeysURL;
  const tactUrlFallback = wowConfig.tactKeysFallbackURL;
  const res = await get([tactUrl, tactUrlFallback]);

  if (!res.ok) throw new Error(`Unable to update tactKeys, HTTP ${res.status}`);

  const data = await res.text();
  const lines = data.split(/\r\n|\n|\r/);
  let remoteAdded = 0;

  for (const line of lines) {
    const parts = line.split(' ');
    if (parts.length !== 2) continue;

    const keyName = parts[0].trim();
    const key = parts[1].trim();

    if (validateKeyPair(keyName, key)) {
      KEY_RING[keyName.toLowerCase()] = key.toLowerCase();
      remoteAdded++;
    } else {
      write('Skipping invalid remote tact key: %s -> %s', keyName, key);
    }
  }

  if (remoteAdded > 0) write('Added %d tact keys from %s', remoteAdded, tactUrl);
}

/**
 * Request for tact keys to be saved on the next tick.
 * Multiple calls can be chained in the same tick.
 */
function save(): void {
  if (!isSaving) {
    isSaving = true;
    setImmediate(() => void doSave());
  }
}

/** Saves the tact keys to disk. */
async function doSave(): Promise<void> {
  await fsp.mkdir(path.dirname(constants.CACHE.TACT_KEYS), { recursive: true });
  await fsp.writeFile(constants.CACHE.TACT_KEYS, JSON.stringify(KEY_RING, null, '\t'), 'utf8');
  isSaving = false;
}

export default { load, getKey, addKey };

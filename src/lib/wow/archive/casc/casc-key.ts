/**
 * Compact in-memory representation of CASC content/encoding keys (16-byte MD5).
 *
 * Stored as a latin1 string (one byte per character) instead of a 32-char hex
 * string, halving map key/value string heap for encoding tables (~2.8M entries).
 */
export type CascKey = string;

/** Convert a CDN/config hex key to compact storage form. */
export function cascKeyFromHex(hex: string): CascKey {
  return Buffer.from(hex, 'hex').toString('latin1');
}

/** Convert a compact key back to lowercase hex for paths, caches, and BLTE hashes. */
export function cascKeyToHex(key: CascKey): string {
  return Buffer.from(key, 'latin1').toString('hex');
}

/** Accept hex (from config) or an already-compact key. */
export function asCascKey(key: string): CascKey {
  if (key.length === 32 && /^[0-9a-f]+$/.test(key)) return cascKeyFromHex(key);
  return key;
}

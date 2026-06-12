/**
 * Fast replacement for the legacy JSON file round-trip
 * (JSONWriter.getContent + JSON.parse). Applies the same value normalization
 * in-place without serializing to a string:
 *   -0        -> 0       (JSON.stringify(-0) === '0')
 *   NaN/±Inf  -> null
 *   BigInt    -> string  (JSONWriter's replacer fallback)
 *   undefined -> null    (in arrays; object values read as undefined either way)
 * Byte parity with the legacy pipeline depends on these rules: coordinate
 * negations in the M2/SKEL/WMO loaders routinely produce -0, which the legacy
 * path silently normalized through the JSON file.
 */

function normalizeNumber(v: number): number | null {
  if (!Number.isFinite(v)) return null;
  return v === 0 ? 0 : v; // collapses -0 to +0
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function normalizeValue(v: unknown): any {
  switch (typeof v) {
    case 'number': return normalizeNumber(v);
    case 'bigint': return v.toString();
    case 'undefined': return null;
    case 'object': return v === null ? null : normalizeJsonValues(v);
    default: return v;
  }
}

/**
 * Normalize all values of an object graph in-place (returns the same root).
 * Safe to call repeatedly; idempotent.
 */
export function normalizeJsonValues<T>(obj: T): T {
  if (Array.isArray(obj)) {
    for (let i = 0; i < obj.length; i++) {
      const v = obj[i];
      // Numbers are assigned unconditionally: -0 === 0, so a changed-value
      // check would miss the -0 -> 0 normalization.
      if (typeof v === 'number') obj[i] = normalizeNumber(v) as never;
      else if (v !== null && typeof v === 'object') normalizeJsonValues(v);
      else if (typeof v === 'bigint' || typeof v === 'undefined') obj[i] = normalizeValue(v) as never;
    }
    return obj;
  }
  if (obj !== null && typeof obj === 'object') {
    for (const k of Object.keys(obj)) {
      const v = (obj as Record<string, unknown>)[k];
      if (typeof v === 'number') (obj as Record<string, unknown>)[k] = normalizeNumber(v);
      else if (v !== null && typeof v === 'object') normalizeJsonValues(v);
      else if (typeof v === 'bigint') (obj as Record<string, unknown>)[k] = v.toString();
      // undefined object values stay: JSON omits the key, and reading an
      // omitted key yields undefined as well.
    }
  }
  return obj;
}

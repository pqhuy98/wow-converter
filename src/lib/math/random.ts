/**
 * Deterministic pseudo-random number generation.
 *
 * Used instead of Math.random() anywhere that influences exported artifacts
 * (models, cameras, terrain) so exports are byte-reproducible across runs.
 */

export type Rand = () => number;

/** Mulberry32: fast, high-quality 32-bit seeded PRNG. Returns floats in [0, 1). */
export function mulberry32(seed: number): Rand {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6D2B79F5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** FNV-1a 32-bit string hash, for deriving stable seeds from names. */
export function hashStringToSeed(str: string): number {
  let h = 0x811C9DC5;
  for (let i = 0; i < str.length; i++) {
    h ^= str.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}

/** Seeded PRNG derived from a string (e.g. model name) so each model keeps its own stable variation. */
export function seededRandom(key: string): Rand {
  return mulberry32(hashStringToSeed(key));
}

/**
 * Minimal runtime state replacing wow.export's core.js couplings
 * (load function registry, selected CDN region, progress shim).
 */
import type { CASC } from '@/lib/wow/archive/casc/casc-source';
import { write } from '@/lib/wow/log';

export type LoadFunc = () => Promise<void> | void;

const loadFuncs: LoadFunc[] = [];

/**
 * Register a function to be executed when a CASC source finishes loading
 * (mirrors core.registerLoadFunc; used by DB caches).
 */
export function registerLoadFunc(fn: LoadFunc): void {
  loadFuncs.push(fn);
}

/** Run all registered load functions (mirrors core.runLoadFuncs). */
export async function runLoadFuncs(): Promise<void> {
  for (const fn of loadFuncs) await fn();
}

/** Selected CDN region tag, used for CDN fallback of local installations. */
export const runtimeState: {
  selectedCDNRegionTag: string;
  /** Active CASC source (replaces wow.export's core.view.casc); set once loaded. */
  casc: CASC | null;
} = {
  selectedCDNRegionTag: 'us',
  casc: null,
};

/** Get the active CASC source, throwing if none is loaded. */
export function getCasc(): CASC {
  if (!runtimeState.casc) throw new Error('No CASC source has been loaded.');
  return runtimeState.casc;
}

/** Progress shim replacing core.createProgress(); just logs steps. */
export interface Progress {
  step(name?: string): Promise<void>;
}

export function createProgress(_segments?: number): Progress {
  return {
    step(name?: string): Promise<void> {
      if (name) write('Progress: %s', name);
      return Promise.resolve();
    },
  };
}

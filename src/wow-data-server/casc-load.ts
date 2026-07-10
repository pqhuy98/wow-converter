/**
 * Single-flight CASC loading for wow-data-server.
 * Concurrent REST clients and startup auto-load share one in-flight promise
 * so we never parse indexes/encoding/root/listfile in parallel.
 */
import type { CASC } from '@/lib/wow/archive/casc/casc-source';
import { CASCLocal } from '@/lib/wow/archive/casc/casc-source-local';
import { CASCRemote } from '@/lib/wow/archive/casc/casc-source-remote';
import * as listfile from '@/lib/wow/archive/casc/listfile';
import { load as loadTactKeys } from '@/lib/wow/archive/casc/tact-keys';
import { write } from '@/lib/wow/log';
import { runtimeState } from '@/lib/wow/server/runtime';

let cascLoadPromise: Promise<CASC> | null = null;

export function isCascLoaded(): boolean {
  return runtimeState.casc?.isLoaded ?? false;
}

export function isCascLoading(): boolean {
  return cascLoadPromise !== null;
}

/** Resolve when CASC is loaded, or null if nothing is in flight and CASC is idle. */
export async function awaitCascLoad(): Promise<CASC | null> {
  if (runtimeState.casc?.isLoaded) return runtimeState.casc;
  if (cascLoadPromise) return cascLoadPromise;
  return null;
}

/** Drop the active CASC source so a different installation can be loaded. */
export function unloadCasc(): void {
  if (cascLoadPromise) {
    throw new Error('WoW data is still loading');
  }
  runtimeState.casc = null;
  listfile.resetForCascUnload();
}

async function finalizeCascLoad(casc: CASC, buildIndex: number): Promise<CASC> {
  const t0 = Date.now();
  await loadTactKeys();
  const preload = listfile.preload();
  await casc.load(buildIndex);
  await preload;
  runtimeState.casc = casc;
  const buildName = casc.getBuildName();
  const seconds = ((Date.now() - t0) / 1000).toFixed(1);
  write('CASC loaded (%s) in %ss', buildName, seconds);
  return casc;
}

/** Join an in-flight load or start a new one for the given pending instance. */
export function loadCascBuildSingleFlight(casc: CASC, buildIndex: number): Promise<CASC> {
  if (runtimeState.casc?.isLoaded) return Promise.resolve(runtimeState.casc);
  if (!cascLoadPromise) {
    cascLoadPromise = finalizeCascLoad(casc, buildIndex).finally(() => {
      cascLoadPromise = null;
    });
  }
  return cascLoadPromise;
}

/** Auto-load from a local WoW install (startup path). */
export async function loadLocalCascFromInstall(installDir: string, product: string): Promise<CASC> {
  if (runtimeState.casc?.isLoaded) return runtimeState.casc;

  if (!cascLoadPromise) {
    cascLoadPromise = (async () => {
      const casc = new CASCLocal(installDir);
      await casc.init();
      const buildIndex = casc.builds.findIndex((b) => b.Product === product);
      if (buildIndex === -1) {
        throw new Error(`Product '${product}' not found in install. Available: ${casc.builds.map((b) => b.Product).join(', ')}`);
      }
      return finalizeCascLoad(casc, buildIndex);
    })().finally(() => {
      cascLoadPromise = null;
    });
  }

  return cascLoadPromise;
}

/** Auto-load from a remote CDN region (startup path). */
export async function loadRemoteCascFromRegion(region: string, product: string): Promise<CASC> {
  if (runtimeState.casc?.isLoaded) return runtimeState.casc;

  if (!cascLoadPromise) {
    cascLoadPromise = (async () => {
      const casc = new CASCRemote(region);
      await casc.init();
      const buildIndex = casc.builds.findIndex((b) => b.Product === product);
      if (buildIndex === -1) {
        throw new Error(`Product '${product}' not found for region '${region}'. Available: ${casc.builds.map((b) => b.Product).join(', ')}`);
      }
      return finalizeCascLoad(casc, buildIndex);
    })().finally(() => {
      cascLoadPromise = null;
    });
  }

  return cascLoadPromise;
}

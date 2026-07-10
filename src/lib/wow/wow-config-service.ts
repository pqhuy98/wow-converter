import { constants } from '@/lib/wow/formats/constants';
import { wowDataClient } from '@/lib/wow-data-client/wow-data-client';
import { dataServerGetJson, dataServerPostJson } from '@/lib/wow-data-server/http-request';

import type {
  CascBuildSummary, CascInfoSummary, WowConfig, WowConfigStatus,
} from './wow-config-state';
import {
  getEffectiveWowConfig,
  getMemoryWowConfig,
  getWowConfigError,
  isEnvWowConfigured,
  isWowConfigApplyInFlight,
  setMemoryWowConfig,
  setWowConfigApplyInFlight,
  setWowConfigError,
} from './wow-config-state';

async function getJson(path: string): Promise<{ ok: boolean; status: number; json: Record<string, unknown> }> {
  const res = await dataServerGetJson(path);
  return { ok: res.ok, status: res.status, json: res.json };
}

async function postJson(path: string, body: unknown): Promise<{ ok: boolean; status: number; json: Record<string, unknown> }> {
  const res = await dataServerPostJson(path, body);
  return { ok: res.ok, status: res.status, json: res.json };
}

function summarizeCascInfo(json: Record<string, unknown>): CascInfoSummary {
  const build = json.build as Record<string, string> | undefined;
  return {
    type: String(json.type ?? ''),
    buildName: String(json.buildName ?? ''),
    build: {
      Product: build?.Product ?? '',
      Version: build?.Version,
      VersionsName: build?.VersionsName,
    },
  };
}

export async function fetchCascInfo(): Promise<CascInfoSummary | null> {
  const { ok, json } = await getJson('/rest/getCascInfo');
  if (ok && json.id === 'CASC_INFO') return summarizeCascInfo(json);
  return null;
}

export async function discoverLocalBuilds(installDirectory: string): Promise<CascBuildSummary[]> {
  setWowConfigError(null);
  const { status, json } = await postJson('/rest/loadCascLocal', { installDirectory });
  if (json.id === 'CASC_INSTALL_BUILDS' && Array.isArray(json.builds)) {
    return json.builds as CascBuildSummary[];
  }
  throw discoverFailureError(true, json, status);
}

export async function discoverRemoteBuilds(regionTag: string): Promise<CascBuildSummary[]> {
  setWowConfigError(null);
  const { status, json } = await postJson('/rest/loadCascRemote', { regionTag });
  if (json.id === 'CASC_INSTALL_BUILDS' && Array.isArray(json.builds)) {
    return json.builds as CascBuildSummary[];
  }
  throw discoverFailureError(false, json, status);
}

function discoverFailureError(local: boolean, json: Record<string, unknown>, status: number): Error {
  if (status === 409 && json.id === 'ERR_CASC_ACTIVE') {
    return new Error('WoW data is already loaded. Change the installation source from setup first.');
  }
  if (json.id === 'ERR_UNREACHABLE') {
    return new Error('Cannot reach wow-data-server. Is it running?');
  }
  if (json.id === 'ERR_INVALID_INSTALL') {
    const detail = typeof json.message === 'string' ? json.message : '';
    return new Error(detail || (local ? 'Invalid WoW installation directory' : 'Invalid CDN region'));
  }
  const detail = typeof json.message === 'string' ? json.message : '';
  return new Error(detail || (local ? 'Could not read WoW installation' : 'Could not read CDN region'));
}

function findBuildIndex(builds: CascBuildSummary[], product: string): number {
  const idx = builds.findIndex((b) => b.Product === product);
  if (idx === -1) {
    const available = [...new Set(builds.map((b) => b.Product))].join(', ');
    throw new Error(`Product '${product}' not found. Available: ${available || '(none)'}`);
  }
  return idx;
}

/** If UI saved a config but wow-data-server restarted, attempt apply once. */
let memoryApplyAttempted = false;
/** User reset or applied a config via UI — do not re-apply .env at runtime. */
let runtimeConfigOverride = false;
let prevCascLoaded = false;

export function resetWowConfigSession(): void {
  runtimeConfigOverride = true;
  memoryApplyAttempted = false;
  setMemoryWowConfig(null);
  setWowConfigError(null);
}

export interface ApplyWowConfigOptions {
  /** When false, load CASC without persisting as the active UI config (env bootstrap only). */
  persist?: boolean;
}

export async function applyWowConfig(
  config: WowConfig,
  options: ApplyWowConfigOptions = {},
): Promise<CascInfoSummary> {
  const persist = options.persist !== false;
  setWowConfigApplyInFlight(true);
  setWowConfigError(null);
  try {
    let builds: CascBuildSummary[];
    if (config.mode === 'local') {
      builds = await discoverLocalBuilds(config.installDirectory);
    } else {
      builds = await discoverRemoteBuilds(config.regionTag);
    }

    const buildIndex = findBuildIndex(builds, config.product);
    const { status, json } = await postJson('/rest/loadCascBuild', { buildIndex });
    if (json.id === 'CASC_INFO') {
      if (persist) {
        setMemoryWowConfig(config);
        runtimeConfigOverride = true;
      }
      return summarizeCascInfo(json);
    }
    if (status === 409 && json.id === 'ERR_CASC_ACTIVE') {
      const info = await fetchCascInfo();
      if (info) return info;
    }
    throw new Error(`Failed to load WoW data (${String(json.id ?? status)})`);
  } catch (e) {
    const message = (e as Error).message;
    setWowConfigError(message);
    throw e;
  } finally {
    setWowConfigApplyInFlight(false);
  }
}

export async function ensureEnvWowConfigLoaded(): Promise<void> {
  if (runtimeConfigOverride || !isEnvWowConfigured()) return;

  if (await fetchCascInfo()) {
    setWowConfigError(null);
  }
  // wow-data-server auto-loads CASC from .env on its own; Express only polls status.
}

/** Re-load CASC from the last UI config after wow-data-server restart. */
export async function ensureMemoryWowConfigLoaded(): Promise<void> {
  if (memoryApplyAttempted) return;
  const config = getMemoryWowConfig();
  if (!config) return;
  if (await fetchCascInfo()) {
    setWowConfigError(null);
    return;
  }
  memoryApplyAttempted = true;
  try {
    await applyWowConfig(config);
  } catch (e) {
    setWowConfigError((e as Error).message);
  }
}

export async function resetWowConfig(): Promise<void> {
  const { json, status } = await postJson('/rest/softRestart', {});
  if (status === 404 || json.id === 'ERR_NOT_FOUND') {
    // Older wow-data-server without softRestart — fall back to unload only.
    const fallback = await postJson('/rest/unloadCasc', {});
    if (fallback.status === 404 || fallback.json.id === 'ERR_NOT_FOUND') {
      throw new Error('Restart wow-data-server, then try again.');
    }
    if (fallback.status === 409) {
      throw new Error(String(fallback.json.message ?? 'WoW data is still loading'));
    }
    if (fallback.json.id !== 'CASC_UNLOADED') {
      throw new Error(`Failed to unload WoW data (${String(fallback.json.id ?? fallback.status)})`);
    }
  } else if (status === 409) {
    throw new Error(String(json.message ?? 'WoW data is still loading'));
  } else if (json.id !== 'SOFT_RESTART_DONE') {
    throw new Error(`Failed to reset WoW data (${String(json.id ?? status)})`);
  }
  resetWowConfigSession();
  wowDataClient.clearRuntimeCaches();
}

/** Block until CASC is ready, or throw with a actionable message (CLI/scripts). */
export async function assertWowCascReady(timeoutMs = 5000): Promise<void> {
  await ensureEnvWowConfigLoaded();
  await ensureMemoryWowConfigLoaded();
  const { wowDataClient } = await import('@/lib/wow-data-client/wow-data-client');
  if (wowDataClient.isReady) return;

  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (wowDataClient.isReady) return;
    await new Promise((r) => { setTimeout(r, 200); });
  }

  if (isEnvWowConfigured() && !runtimeConfigOverride) {
    throw new Error('CASC is set in .env but failed to load. Check wow-data-server logs.');
  }
  throw new Error(
    'WoW data not loaded. Set CASC_LOCAL_WOW or CASC_REMOTE_* in .env, '
    + 'or configure via the web UI (/setup) while wow-data-server is running.',
  );
}

export async function getWowConfigStatus(): Promise<WowConfigStatus> {
  const reachable = (await getJson('/rest/getConfig?key=exportDirectory')).status !== 0;
  let cascInfo = reachable ? await fetchCascInfo() : null;
  let cascLoaded = cascInfo !== null;

  if (prevCascLoaded && !cascLoaded && !isWowConfigApplyInFlight()) {
    memoryApplyAttempted = false;
    setWowConfigError(null);
  }

  await ensureEnvWowConfigLoaded();
  await ensureMemoryWowConfigLoaded();

  cascInfo = reachable ? await fetchCascInfo() : null;
  cascLoaded = cascInfo !== null;
  prevCascLoaded = cascLoaded;

  if (cascLoaded) {
    setWowConfigError(null);
  }

  const configuredFromEnv = isEnvWowConfigured();
  const config = getEffectiveWowConfig();
  let cascLoading = isWowConfigApplyInFlight();
  let cascLoadingMessage = '';
  if (reachable) {
    const { ok, json } = await getJson('/rest/getCascLoadProgress');
    if (ok && json.id === 'CASC_LOAD_PROGRESS') {
      if (json.loading === true) cascLoading = true;
      if (typeof json.message === 'string') cascLoadingMessage = json.message;
    }
  }
  const needsSetup = !cascLoaded && !getMemoryWowConfig()
    && (runtimeConfigOverride || !configuredFromEnv);

  return {
    needsSetup,
    configuredFromEnv,
    cascLoaded,
    cascLoading,
    cascLoadingMessage: cascLoadingMessage || undefined,
    wowDataServerReachable: reachable,
    config,
    cascInfo,
    error: cascLoaded ? null : getWowConfigError(),
    products: constants.PRODUCTS,
    regions: [...constants.PATCH.REGIONS],
  };
}

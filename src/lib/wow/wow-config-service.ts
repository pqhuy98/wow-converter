import { constants } from '@/lib/wow/formats/constants';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import type {
  CascBuildSummary, CascInfoSummary, WowConfig, WowConfigStatus,
} from './wow-config-state';
import {
  getEffectiveWowConfig,
  getEnvWowConfig,
  getMemoryWowConfig,
  getWowConfigError,
  isEnvWowConfigured,
  isWowConfigApplyInFlight,
  setMemoryWowConfig,
  setWowConfigApplyInFlight,
  setWowConfigError,
} from './wow-config-state';

function wowDataServerBase(): string {
  return `http://127.0.0.1:${process.env.WOW_DATA_SERVER_PORT || 17753}`;
}

async function getJson(path: string): Promise<{ ok: boolean; status: number; json: Record<string, unknown> }> {
  try {
    const res = await fetch(`${wowDataServerBase()}${path}`);
    const json = await res.json() as Record<string, unknown>;
    return { ok: res.ok, status: res.status, json };
  } catch (e) {
    return { ok: false, status: 0, json: { id: 'ERR_UNREACHABLE', message: (e as Error).message } };
  }
}

async function postJson(path: string, body: unknown): Promise<{ ok: boolean; status: number; json: Record<string, unknown> }> {
  try {
    const res = await fetch(`${wowDataServerBase()}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body ?? {}),
    });
    const json = await res.json() as Record<string, unknown>;
    return { ok: res.ok, status: res.status, json };
  } catch (e) {
    return { ok: false, status: 0, json: { id: 'ERR_UNREACHABLE', message: (e as Error).message } };
  }
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

export async function applyWowConfig(config: WowConfig): Promise<CascInfoSummary> {
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
      if (!isEnvWowConfigured()) setMemoryWowConfig(config);
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

/** If env is configured but CASC is not loaded yet, attempt apply once. */
let envApplyAttempted = false;
/** If UI saved a config but wow-data-server restarted, attempt apply once. */
let memoryApplyAttempted = false;
let prevCascLoaded = false;

export function resetWowConfigSession(): void {
  envApplyAttempted = false;
  memoryApplyAttempted = false;
  setMemoryWowConfig(null);
  setWowConfigError(null);
}

export async function resetWowConfig(): Promise<void> {
  if (isEnvWowConfigured()) {
    throw new Error('WoW is configured automatically and cannot be changed here.');
  }
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
  wowExportClient.clearRuntimeCaches();
}

export async function ensureEnvWowConfigLoaded(): Promise<void> {
  if (envApplyAttempted || !isEnvWowConfigured()) return;
  const cascInfo = await fetchCascInfo();
  if (cascInfo) return;
  envApplyAttempted = true;
  const envConfig = getEnvWowConfig();
  if (!envConfig) return;
  try {
    await applyWowConfig(envConfig);
  } catch (e) {
    setWowConfigError((e as Error).message);
  }
}

/** Re-load CASC from the last UI config after wow-data-server restart. */
export async function ensureMemoryWowConfigLoaded(): Promise<void> {
  if (memoryApplyAttempted || isEnvWowConfigured()) return;
  const config = getMemoryWowConfig();
  if (!config) return;
  if (await fetchCascInfo()) return;
  memoryApplyAttempted = true;
  try {
    await applyWowConfig(config);
  } catch (e) {
    setWowConfigError((e as Error).message);
  }
}

/** Block until CASC is ready, or throw with a actionable message (CLI/scripts). */
export async function assertWowCascReady(timeoutMs = 5000): Promise<void> {
  await ensureEnvWowConfigLoaded();
  await ensureMemoryWowConfigLoaded();
  const { wowExportClient } = await import('@/lib/wowexport-client/wowexport-client');
  if (wowExportClient.isReady) return;

  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (wowExportClient.isReady) return;
    await new Promise((r) => { setTimeout(r, 200); });
  }

  if (isEnvWowConfigured()) {
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
  if (!isEnvWowConfigured()) {
    await ensureMemoryWowConfigLoaded();
  }

  cascInfo = reachable ? await fetchCascInfo() : null;
  cascLoaded = cascInfo !== null;
  prevCascLoaded = cascLoaded;

  const configuredFromEnv = isEnvWowConfigured();
  const config = getEffectiveWowConfig();
  const cascLoading = isWowConfigApplyInFlight();
  const needsSetup = !configuredFromEnv && !cascLoaded && !getMemoryWowConfig();

  return {
    needsSetup,
    configuredFromEnv,
    cascLoaded,
    cascLoading,
    wowDataServerReachable: reachable,
    config,
    cascInfo,
    error: getWowConfigError(),
    products: constants.PRODUCTS,
    regions: [...constants.PATCH.REGIONS],
  };
}

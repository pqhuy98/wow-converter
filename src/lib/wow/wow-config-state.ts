import { constants } from '@/lib/wow/formats/constants';

import {
  getCascLocalProduct, getCascLocalWow, getCascRemoteProduct, getCascRemoteRegion,
} from './env';

export type WowConfigMode = 'local' | 'remote';

export interface WowConfigLocal {
  mode: 'local';
  installDirectory: string;
  product: string;
}

export interface WowConfigRemote {
  mode: 'remote';
  regionTag: string;
  product: string;
}

export type WowConfig = WowConfigLocal | WowConfigRemote;

export interface CascBuildSummary {
  Product: string;
  Region: string;
  VersionsName: string;
  Version?: string;
  BuildKey?: string;
  BuildConfig?: string;
}

export interface CascInfoSummary {
  type: string;
  buildName: string;
  build: {
    Product: string;
    Version?: string;
    VersionsName?: string;
  };
}

export interface WowConfigStatus {
  /** True when the user must pick a WoW data source in the UI. */
  needsSetup: boolean;
  configuredFromEnv: boolean;
  cascLoaded: boolean;
  cascLoading: boolean;
  wowDataServerReachable: boolean;
  config: WowConfig | null;
  cascInfo: CascInfoSummary | null;
  error: string | null;
  products: typeof constants.PRODUCTS;
  regions: string[];
}

let memoryConfig: WowConfig | null = null;
let lastError: string | null = null;
let applyInFlight = false;

/** Config from .env; local takes priority over remote when both are set. */
export function getEnvWowConfig(): WowConfig | null {
  const localDir = getCascLocalWow();
  if (localDir) {
    return { mode: 'local', installDirectory: localDir, product: getCascLocalProduct() };
  }
  const region = getCascRemoteRegion();
  if (region) {
    return { mode: 'remote', regionTag: region, product: getCascRemoteProduct() };
  }
  return null;
}

export function isEnvWowConfigured(): boolean {
  return getEnvWowConfig() !== null;
}

export function getMemoryWowConfig(): WowConfig | null {
  return memoryConfig;
}

export function setMemoryWowConfig(config: WowConfig | null): void {
  memoryConfig = config;
}

export function getEffectiveWowConfig(): WowConfig | null {
  return memoryConfig ?? getEnvWowConfig();
}

export function setWowConfigError(message: string | null): void {
  lastError = message;
}

export function getWowConfigError(): string | null {
  return lastError;
}

export function setWowConfigApplyInFlight(value: boolean): void {
  applyInFlight = value;
}

export function isWowConfigApplyInFlight(): boolean {
  return applyInFlight;
}

export function productLabel(productCode: string): string {
  const entry = constants.PRODUCTS.find((p) => p.product === productCode);
  if (entry) return `${entry.title} (${entry.tag})`;
  return productCode;
}

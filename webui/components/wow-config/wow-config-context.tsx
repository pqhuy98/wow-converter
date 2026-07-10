'use client';

import {
  createContext, useCallback, useContext, useEffect, useMemo, useState,
} from 'react';

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

export interface ProductInfo {
  product: string;
  title: string;
  tag: string;
}

export interface CascBuildSummary {
  Product: string;
  Region: string;
  VersionsName: string;
  Version?: string;
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
  needsSetup: boolean;
  configuredFromEnv: boolean;
  cascLoaded: boolean;
  cascLoading: boolean;
  cascLoadingMessage?: string;
  wowDataServerReachable: boolean;
  config: WowConfig | null;
  cascInfo: CascInfoSummary | null;
  error: string | null;
  products: ProductInfo[];
  regions: string[];
}

const defaultStatus: WowConfigStatus = {
  needsSetup: true,
  configuredFromEnv: false,
  cascLoaded: false,
  cascLoading: false,
  wowDataServerReachable: false,
  config: null,
  cascInfo: null,
  error: null,
  products: [],
  regions: [],
};

async function fetchStatus(): Promise<WowConfigStatus> {
  const res = await fetch('/api/wow-config/status');
  if (!res.ok) throw new Error('Failed to fetch WoW config status');
  return res.json();
}

interface WowConfigContextValue {
  status: WowConfigStatus;
  refresh: () => Promise<void>;
  isReady: boolean;
}

const WowConfigContext = createContext<WowConfigContextValue>({
  status: defaultStatus,
  refresh: async () => {},
  isReady: false,
});

export function WowConfigProvider({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = useState<WowConfigStatus>(defaultStatus);
  const [isReady, setIsReady] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const next = await fetchStatus();
      setStatus(next);
    } catch {
      setStatus((prev) => ({ ...prev, wowDataServerReachable: false }));
    } finally {
      setIsReady(true);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const shouldPoll = !status.cascLoaded || status.cascLoading;
  useEffect(() => {
    if (!shouldPoll) {
      return () => {};
    }
    const id = setInterval(() => { void refresh(); }, 2000);
    return () => clearInterval(id);
  }, [shouldPoll, refresh]);

  const value = useMemo(() => ({ status, refresh, isReady }), [status, refresh, isReady]);

  return (
    <WowConfigContext.Provider value={value}>
      {children}
    </WowConfigContext.Provider>
  );
}

export function useWowConfig() {
  return useContext(WowConfigContext);
}

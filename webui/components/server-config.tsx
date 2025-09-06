'use client';

import {
  createContext, useContext, useEffect, useState,
} from 'react';

type ServerConfig = {
  wowExportAssetDir: string
  isSharedHosting: boolean
  isDev: boolean
  isClassic: boolean
};

const defaultConfig: ServerConfig = {
  wowExportAssetDir: '',
  isSharedHosting: false,
  isDev: false,
  isClassic: false,
};

async function fetchConfig(): Promise<ServerConfig> {
  return fetch('/get-config').then((res) => res.json());
}

const ServerConfigContext = createContext<ServerConfig>(defaultConfig);

export function ServerConfigProvider({ children }: { children: React.ReactNode }) {
  const [config, setConfig] = useState<ServerConfig>(defaultConfig);
  const [isLoaded, setIsLoaded] = useState(false);

  useEffect(() => {
    void fetchConfig().then((config) => {
      setConfig(config);
      setIsLoaded(true);
    });
    const itv = setInterval(() => {
      void fetchConfig().then((config) => {
        setConfig(config);
      });
    }, 5000);
    return () => clearInterval(itv);
  }, []);

  if (!isLoaded) {
    return <div></div>;
  }

  return <ServerConfigContext.Provider value={config}>
    {children}
  </ServerConfigContext.Provider>;
}

export function useServerConfig() {
  return useContext(ServerConfigContext);
}

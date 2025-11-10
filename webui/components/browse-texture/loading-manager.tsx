'use client';

import {
  createContext, useCallback, useContext, useRef, useState,
} from 'react';

interface LoadingManagerContextValue {
  isLoading: (key: string) => boolean;
  setLoading: (key: string, loading: boolean) => void;
  clearKey: (key: string) => void;
  clearAll: () => void;
}

const LoadingManagerContext = createContext<LoadingManagerContextValue | null>(null);

export function LoadingManagerProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  // Track loading state by key
  const [loadingKeys, setLoadingKeys] = useState<Set<string>>(new Set());
  // Track expected keys (keys that should be loading)
  const expectedKeysRef = useRef<Set<string>>(new Set());

  const isLoading = useCallback((key: string): boolean => loadingKeys.has(key), [loadingKeys]);

  const setLoading = useCallback((key: string, loading: boolean) => {
    console.log(`[LoadingManager] setLoading called: key="${key}", loading=${loading}`);
    setLoadingKeys((prev) => {
      const wasLoading = prev.has(key);
      const next = new Set(prev);
      if (loading) {
        if (wasLoading) {
          console.log(`[LoadingManager] WARNING: Setting loading=true for already-loading key: ${key}`);
        }
        next.add(key);
        expectedKeysRef.current.add(key);
      } else {
        if (!wasLoading) {
          console.log(`[LoadingManager] WARNING: Setting loading=false for non-loading key: ${key}`);
        }
        next.delete(key);
        expectedKeysRef.current.delete(key);
      }
      return next;
    });
  }, []);

  const clearKey = useCallback((key: string) => {
    setLoadingKeys((prev) => {
      const next = new Set(prev);
      next.delete(key);
      return next;
    });
    expectedKeysRef.current.delete(key);
  }, []);

  const clearAll = useCallback(() => {
    setLoadingKeys(new Set());
    expectedKeysRef.current.clear();
  }, []);

  return (
    <LoadingManagerContext.Provider value={{
      isLoading, setLoading, clearKey, clearAll,
    }}>
      {children}
    </LoadingManagerContext.Provider>
  );
}

export function useLoadingManager() {
  const context = useContext(LoadingManagerContext);
  if (!context) {
    throw new Error('useLoadingManager must be used within LoadingManagerProvider');
  }
  return context;
}

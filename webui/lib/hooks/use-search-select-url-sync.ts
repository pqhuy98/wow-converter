'use client';

import { useCallback, useEffect, useRef } from 'react';

interface UseSearchSelectUrlSyncOptions {
  readonly basePath: string;
  readonly search: string;
  readonly setSearch: (s: string) => void;
  readonly setDebouncedSearch: (s: string) => void;
  readonly selectedPath?: string | undefined;
  readonly pendingScrollPath: string | null;
  readonly setPendingScrollPath: (s: string | null) => void;
  readonly resetLocalState: () => void;
}
export function useSearchSelectUrlSync(opts: UseSearchSelectUrlSyncOptions) {
  const {
    basePath,
    search,
    setSearch,
    setDebouncedSearch,
    selectedPath,
    pendingScrollPath,
    setPendingScrollPath,
    resetLocalState,
  } = opts;

  const hasInitFromUrlRef = useRef(false);
  const readyToSyncRef = useRef(false);

  // Update the URL query string without triggering a rerender (history.replaceState)
  const updateUrlQuery = useCallback((next: { s?: string | null; c?: string | null }) => {
    if (typeof window === 'undefined') return;
    const current = new URL(window.location.href);
    const params = new URLSearchParams(current.search);
    if (next.s !== undefined) {
      if (next.s && next.s.length > 0) params.set('s', next.s);
      else params.delete('s');
    }
    if (next.c !== undefined) {
      if (next.c && next.c.length > 0) params.set('c', next.c);
      else params.delete('c');
    }
    const newSearch = params.toString();
    const currentSearch = current.search.startsWith('?') ? current.search.slice(1) : current.search;
    if (newSearch !== currentSearch) {
      const newUrl = `${current.pathname}${newSearch ? `?${newSearch}` : ''}${current.hash ?? ''}`;
      window.history.replaceState(window.history.state, '', newUrl);
    }
  }, []);

  // Initial parse once: adopt s/c from URL
  useEffect(() => {
    if (hasInitFromUrlRef.current) return;
    if (typeof window === 'undefined') return;
    if (window.location.pathname !== basePath) {
      hasInitFromUrlRef.current = true;
      readyToSyncRef.current = true;
      return;
    }
    const params = new URLSearchParams(window.location.search);
    const sRaw = params.get('s') ?? '';
    const s = sRaw.replace(/\+/g, ' ');
    const c = params.get('c');
    if (s) {
      setSearch(s);
      setDebouncedSearch(s);
    }
    if (c) {
      setPendingScrollPath(c);
    }
    hasInitFromUrlRef.current = true;
    // Defer enabling URL sync until after state updates have flushed
    if (s || c) {
      if (typeof window !== 'undefined') {
        requestAnimationFrame(() => {
          readyToSyncRef.current = true;
        });
      }
    } else {
      readyToSyncRef.current = true;
    }
  }, [basePath, setSearch, setDebouncedSearch, setPendingScrollPath]);

  // State -> URL sync
  useEffect(() => {
    if (!hasInitFromUrlRef.current) return;
    if (!readyToSyncRef.current) return;
    if (pendingScrollPath) {
      updateUrlQuery({ s: search });
      return;
    }
    updateUrlQuery({ s: search, c: selectedPath ?? null });
  }, [search, selectedPath, pendingScrollPath, updateUrlQuery]);

  // Robust URL listener: listen to history changes (pushState/replaceState/popstate)
  // - If navigating to basePath with no params, clear local state
  // - If navigating to basePath with s/c, adopt them into local state
  useEffect(() => {
    if (typeof window === 'undefined') return undefined;
    const onLocationChange = () => {
      if (window.location.pathname !== basePath) return;
      const params = new URLSearchParams(window.location.search);
      const hasS = params.has('s');
      const hasC = params.has('c');
      if (!hasS && !hasC) {
        resetLocalState();
        return;
      }
      const sRaw = params.get('s') ?? '';
      const sNext = sRaw.replace(/\+/g, ' ');
      const cNext = params.get('c');
      if (sNext !== search) {
        setSearch(sNext);
        setDebouncedSearch(sNext);
      }
      if (cNext && cNext !== selectedPath) {
        setPendingScrollPath(cNext);
      }
    };
    // Patch history methods to emit a custom event
    const originalPushState: History['pushState'] = window.history.pushState.bind(window.history);
    const originalReplaceState: History['replaceState'] = window.history.replaceState.bind(window.history);
    const emit = () => window.dispatchEvent(new Event('_locationchange'));
    window.history.pushState = ((...args: Parameters<History['pushState']>) => {
      originalPushState(...args);
      emit();
    }) as History['pushState'];
    window.history.replaceState = ((...args: Parameters<History['replaceState']>) => {
      originalReplaceState(...args);
      emit();
    }) as History['replaceState'];

    window.addEventListener('popstate', onLocationChange);
    window.addEventListener('_locationchange', onLocationChange);
    // Run once at mount to catch initial state
    onLocationChange();

    const cleanup = () => {
      window.history.pushState = originalPushState;
      window.history.replaceState = originalReplaceState;
      window.removeEventListener('popstate', onLocationChange);
      window.removeEventListener('_locationchange', onLocationChange);
    };
    return cleanup;
  }, [basePath, search, selectedPath, setSearch, setDebouncedSearch, setPendingScrollPath, resetLocalState]);
}

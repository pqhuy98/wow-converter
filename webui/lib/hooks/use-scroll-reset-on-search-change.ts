'use client';

import { useEffect, useRef } from 'react';

interface UseScrollResetOnSearchChangeOptions {
  readonly containerRef: React.RefObject<HTMLDivElement>;
  readonly search: string;
  readonly isPending: boolean;
}

export function useScrollResetOnSearchChange({
  containerRef,
  search,
  isPending,
}: UseScrollResetOnSearchChangeOptions) {
  const prevRef = useRef(search);
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const hasChanged = prevRef.current !== search;
    if (!hasChanged) return;
    prevRef.current = search;
    if (isPending) return;
    requestAnimationFrame(() => {
      el.scrollTop = 0;
    });
  }, [containerRef, search, isPending]);
}

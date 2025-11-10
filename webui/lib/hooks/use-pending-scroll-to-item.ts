'use client';

import { useEffect } from 'react';

interface UsePendingScrollToItemOptions<T> {
  readonly items: readonly T[];
  readonly containerRef: React.RefObject<HTMLDivElement>;
  readonly getRowHeight: (item: T, index: number) => number;
  readonly contentPadding: number;
  readonly matchKey: (item: T) => string;
  readonly pendingKey: string | null;
  readonly setPendingKey: (key: string | null) => void;
  readonly onSelect: (item: T) => void;
}

export function usePendingScrollToItem<T>({
  items,
  containerRef,
  getRowHeight,
  contentPadding,
  matchKey,
  pendingKey,
  setPendingKey,
  onSelect,
}: UsePendingScrollToItemOptions<T>) {
  useEffect(() => {
    if (!pendingKey) return;
    const container = containerRef.current;
    if (!container) return;
    const index = items.findIndex((it) => matchKey(it) === pendingKey);
    if (index < 0) return;

    // Compute scrollTop with variable heights
    let scrollTop = contentPadding;
    for (let i = 0; i < index; i += 1) {
      scrollTop += getRowHeight(items[i], i);
    }

    // Scroll on next frame, select on the frame after
    requestAnimationFrame(() => {
      container.scrollTo({
        top: Math.max(0, scrollTop - 50),
        behavior: 'smooth',
      });
      requestAnimationFrame(() => {
        const item = items[index];
        if (item) {
          onSelect(item);
        }
        setPendingKey(null);
      });
    });
  }, [items, containerRef, getRowHeight, contentPadding, matchKey, pendingKey, setPendingKey, onSelect]);
}

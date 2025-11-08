'use client';

import React, {
  CSSProperties, useEffect, useMemo, useRef, useState,
} from 'react';

type ReactCSS = CSSProperties;

export interface VirtualListBoxProps<T> {
  readonly items: readonly T[];
  readonly getRowKey: (item: T, index: number) => string | number;
  readonly renderRow: (item: T, index: number, style: ReactCSS) => React.ReactNode;
  readonly fixedRowHeight?: number;
  readonly getRowHeight?: (item: T, index: number) => number;
  readonly overscan?: number;
  readonly containerRef?: React.RefObject<HTMLDivElement>;
  readonly containerClassName?: string;
  readonly containerStyle?: ReactCSS;
  readonly contentPadding?: number; // top and bottom padding inside the relative container
}

/**
 * A flexible, virtualized listbox that mirrors the layout behavior of the
 * working list on the model browse page.
 *
 * - Supports fixed row height or variable row height via getRowHeight
 * - Preserves horizontal scroll for long content via min-width:100% + width:max-content
 * - Renders only the visible window with overscan for performance
 * - Extensible through a renderRow callback
 */
export function VirtualListBox<T>({
  items,
  getRowKey,
  renderRow,
  fixedRowHeight,
  getRowHeight,
  overscan = 8,
  containerRef,
  containerClassName,
  containerStyle,
  contentPadding = 0,
}: VirtualListBoxProps<T>) {
  // internal ref if external not provided
  const internalRef = useRef<HTMLDivElement | null>(null);
  const listRef = containerRef ?? internalRef;

  const [viewportHeight, setViewportHeight] = useState(400);
  const [scrollTop, setScrollTop] = useState(0);

  // Track viewport height
  useEffect(() => {
    const el = listRef.current;
    if (!el) return undefined;
    const update = () => setViewportHeight(el.clientHeight || 400);
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, [listRef.current]);

  const isVariable = typeof getRowHeight === 'function';

  // Precompute cumulative heights for variable-height mode
  const rowHeights = useMemo(() => {
    if (!isVariable) return null as unknown as { heights: number[]; totalHeight: number };
    const heights: number[] = [];
    let cumulative = 0;
    for (let i = 0; i < items.length; i += 1) {
      heights.push(cumulative);
      const h = getRowHeight!(items[i], i);
      cumulative += h;
    }
    return { heights, totalHeight: cumulative + contentPadding * 2 };
  }, [items, getRowHeight, isVariable, contentPadding]);

  const fixedTotalHeight = useMemo(() => {
    if (isVariable) return 0;
    const h = fixedRowHeight ?? 28;
    return (items.length * h) + (contentPadding * 2);
  }, [items.length, fixedRowHeight, isVariable, contentPadding]);

  const computeVisibleFixed = (): { startIndex: number; endIndex: number; offsetTop: number } => {
    const h = fixedRowHeight ?? 28;
    const total = items.length;
    const startIndex = Math.max(0, Math.floor(scrollTop / h) - overscan);
    const endIndex = Math.min(total - 1, Math.ceil((scrollTop + viewportHeight) / h) + overscan);
    const offsetTop = startIndex * h;
    return { startIndex, endIndex, offsetTop };
  };

  const computeVisibleVariable = (): { startIndex: number; endIndex: number; offsetTop: number } => {
    const { heights } = rowHeights!;
    if (items.length === 0) return { startIndex: 0, endIndex: -1, offsetTop: 0 };
    const viewportTop = scrollTop;
    const viewportBottom = scrollTop + viewportHeight;
    // Binary search for start index
    let startIndex = 0;
    let left = 0;
    let right = items.length - 1;
    while (left <= right) {
      const mid = Math.floor((left + right) / 2);
      const rowH = getRowHeight!(items[mid], mid);
      if (heights[mid] + rowH < viewportTop) {
        left = mid + 1;
      } else {
        startIndex = mid;
        right = mid - 1;
      }
    }
    startIndex = Math.max(0, startIndex - overscan);

    // Binary search for end index
    let endIndex = items.length - 1;
    left = startIndex;
    right = items.length - 1;
    while (left <= right) {
      const mid = Math.floor((left + right) / 2);
      if (heights[mid] <= viewportBottom) {
        endIndex = mid;
        left = mid + 1;
      } else {
        right = mid - 1;
      }
    }
    endIndex = Math.min(items.length - 1, endIndex + overscan);

    const offsetTop = heights[startIndex];
    return { startIndex, endIndex, offsetTop };
  };

  const {
    startIndex, endIndex, offsetTop,
  } = useMemo(() => (isVariable ? computeVisibleVariable() : computeVisibleFixed()), [
    scrollTop, viewportHeight, items, isVariable, rowHeights, fixedRowHeight, overscan, getRowHeight,
  ]);

  const visibleItems = useMemo(() => {
    if (endIndex < startIndex) return [] as Array<{ item: T; index: number }>;
    const out: Array<{ item: T; index: number }> = [];
    for (let i = startIndex; i <= endIndex; i += 1) {
      out.push({ item: items[i], index: i });
    }
    return out;
  }, [items, startIndex, endIndex]);

  const containerHeight = isVariable ? rowHeights!.totalHeight : fixedTotalHeight;

  return (
    <div
      ref={listRef}
      className={containerClassName}
      style={containerStyle}
      onScroll={(e) => setScrollTop((e.target as HTMLDivElement).scrollTop)}
    >
      <div style={{
        height: containerHeight,
        position: 'relative',
        width: '100%',
      }}>
        <div style={{
          position: 'absolute',
          top: contentPadding + offsetTop,
          left: 0,
          minWidth: '100%',
          width: 'max-content',
        }}>
          {visibleItems.map(({ item, index }) => {
            const height = isVariable ? getRowHeight!(item, index) : (fixedRowHeight ?? 28);
            const style: ReactCSS = { height };
            return (
              <React.Fragment key={getRowKey(item, index)}>
                {renderRow(item, index, style)}
              </React.Fragment>
            );
          })}
        </div>
      </div>
    </div>
  );
}

export default VirtualListBox;


'use client';

import { useCallback, useEffect, useRef } from 'react';

type Point = { x: number; y: number };

export interface MinimapCanvasProps {
  mapId: string;
  mask: boolean[][]; // [y][x] 64x64
  textureMask?: boolean[][]; // [y][x] 64x64 (optional): whether minimap texture exists
  tileSize?: number; // base pixels per tile (before zoom)
  className?: string;
  onHoverChange?: (tile: Point | null, world: { x: number; y: number } | null) => void;
  onSelectionChange?: (tiles: Point[]) => void;
}

const MAP_SIZE = 64;
const MAP_COORD_BASE = 0; // not used directly for world display in UI
const TILE_WORLD_SIZE = 533.333; // world units per tile (display only)

type CacheTile = {
  base: ImageBitmap;
  mips?: { sizes: number[]; canvases: HTMLCanvasElement[] };
} | true | undefined;

function createDefaultState() {
  return {
    offsetX: 0,
    offsetY: 0,
    zoomFactor: 1,
    tileQueue: [] as [number, number, number][],
    pending: new Set<string>(),
    cache: new Array(MAP_SIZE * MAP_SIZE) as CacheTile[],
    missing: new Set<number>(),
    isPanning: false,
    // selection state
    selectionMode: 'none' as 'none' | 'rect' | 'paint',
    rectAction: 'toggle' as 'toggle' | 'remove',
    rectStartX: null as number | null,
    rectStartY: null as number | null,
    rectCurX: null as number | null,
    rectCurY: null as number | null,
    paintAddMode: null as boolean | null,
    hoverTile: null as number | null,
    isHovering: false,
    panBaseX: 0,
    panBaseY: 0,
    mouseBaseX: 0,
    mouseBaseY: 0,
    controllers: new Map<string, AbortController>(),
    activeLoads: 0,
    // click/pan discrimination
    clickPending: false,
    clickStartClientX: 0,
    clickStartClientY: 0,
    clickTileX: 0,
    clickTileY: 0,
    mouseDownButton: null as number | null,
    mouseDownShift: false,
  };
}
type State = ReturnType<typeof createDefaultState>;

export default function MinimapCanvas({
  mapId, mask, textureMask, tileSize = 64, className, onHoverChange, onSelectionChange,
}: MinimapCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const ctxRef = useRef<CanvasRenderingContext2D | null>(null);

  // persistent interactive state
  const stateRef = useRef<State>(createDefaultState());

  const versionRef = useRef<number>(0); // bump to invalidate async loads on map change
  const rafPendingRef = useRef<boolean>(false);

  const scheduleRender = () => {
    if (rafPendingRef.current) return;
    rafPendingRef.current = true;
    requestAnimationFrame(() => {
      rafPendingRef.current = false;
      render();
    });
  };

  const getContext = () => {
    const canvas = canvasRef.current;
    if (!canvas) return null;
    if (!ctxRef.current) ctxRef.current = canvas.getContext('2d');
    return ctxRef.current;
  };

  const computeTileSize = (): number => {
    const s = stateRef.current!;
    return Math.max(1, Math.floor(tileSize / s.zoomFactor));
  };

  const getDrawXY = (x: number, y: number, size: number) => {
    const s = stateRef.current!;
    return {
      x: Math.round((x * size) + s.offsetX),
      y: Math.round((y * size) + s.offsetY),
    };
  };

  const isVisible = (drawX: number, drawY: number, size: number, viewportW: number, viewportH: number) => {
    const margin = 1; // be tolerant to rounding
    if (drawX > (viewportW - margin)) return false;
    if (drawY > (viewportH - margin)) return false;
    if (drawX + size < -margin) return false;
    if (drawY + size < -margin) return false;
    return true;
  };

  const mapPositionFromClientPoint = (clientX: number, clientY: number) => {
    const canvas = canvasRef.current!;
    const rect = canvas.getBoundingClientRect();
    const s = stateRef.current!;
    const viewX = (clientX - rect.x) - s.offsetX;
    const viewY = (clientY - rect.y) - s.offsetY;
    const tSize = computeTileSize();
    const tileX = Math.floor(viewX / tSize);
    const tileY = Math.floor(viewY / tSize);
    return {
      tileX, tileY, posX: MAP_COORD_BASE - (tileX * TILE_WORLD_SIZE), posY: MAP_COORD_BASE - (tileY * TILE_WORLD_SIZE),
    };
  };

  const queueTile = (x: number, y: number, index: number) => {
    const s = stateRef.current!;
    const canvas = canvasRef.current!;
    const size = computeTileSize();
    const { x: drawX, y: drawY } = getDrawXY(x, y, size);
    // Skip if offscreen (tolerant)
    if (!isVisible(drawX, drawY, size, canvas.clientWidth, canvas.clientHeight)) return;
    // Don't retry tiles that returned 404 previously
    if (s.missing.has(index)) return;
    const key = `${index}`;
    if (s.pending.has(key)) return;
    s.pending.add(key);
    const node: [number, number, number] = [x, y, index];
    s.tileQueue.push(node);
    processQueue();
  };

  const processQueue = () => {
    const s = stateRef.current!;
    const MAX_CONCURRENT_LOADS = 8;
    if (s.activeLoads >= MAX_CONCURRENT_LOADS || s.tileQueue.length === 0) return;

    const canvas = canvasRef.current!;
    const rect = canvas.getBoundingClientRect();
    const center = mapPositionFromClientPoint(rect.x + rect.width / 2, rect.y + rect.height / 2);

    const computePriority = (x: number, y: number): number => {
      if (s.hoverTile !== null) {
        const hx = Math.floor(s.hoverTile / MAP_SIZE);
        const hy = s.hoverTile % MAP_SIZE;
        return Math.hypot(x - hx, y - hy);
      }
      return Math.hypot(x - center.tileX, y - center.tileY);
    };

    s.tileQueue.sort((a, b) => computePriority(a[0], a[1]) - computePriority(b[0], b[1]));
    while (s.activeLoads < MAX_CONCURRENT_LOADS && s.tileQueue.length > 0) {
      const node = s.tileQueue.shift()!;
      void loadTile(node);
    }
  };

  const loadTile = useCallback(async (node: [number, number, number]) => {
    const s = stateRef.current!;
    s.activeLoads += 1;
    const [x, y, index] = node;
    const localVersion = versionRef.current;
    try {
      // Re-validate masks before fetching (map may have changed rapidly)
      if (!(textureMask?.[y]?.[x])) return; // only fetch if texture known to exist
      // Ensure base bitmap
      let entry = s.cache[index];
      if (!entry || entry === true) {
        const key = `${index}`;
        const prev = s.controllers.get(key);
        prev?.abort();
        const controller = new AbortController();
        s.controllers.set(key, controller);
        const res = await fetch(`/api/maps/${encodeURIComponent(mapId)}/minimap/${x}/${y}`, { signal: controller.signal, cache: 'force-cache' });
        if (res.status === 404) { s.missing.add(index); return; }
        if (!res.ok) throw new Error(String(res.status));
        const blob = await res.blob();
        const bitmap = await createImageBitmap(blob, { imageOrientation: 'none', colorSpaceConversion: 'none', premultiplyAlpha: 'none' });
        if (localVersion !== versionRef.current) return; // stale
        entry = { base: bitmap } as CacheTile;
        s.cache[index] = entry;
      }
      // Build mipmaps once per tile
      if (entry && entry !== true && !entry.mips) {
        const baseW = entry.base.width;
        const baseH = entry.base.height;
        let sizePow2 = 1;
        while (sizePow2 * 2 <= Math.min(baseW, baseH)) sizePow2 *= 2; // largest power-of-two <= min dimension
        const sizes: number[] = [];
        const canvases: HTMLCanvasElement[] = [];
        for (let sSize = sizePow2; sSize >= 32; sSize = Math.floor(sSize / 2)) {
          const c = document.createElement('canvas');
          c.width = sSize;
          c.height = sSize;
          const cctx = c.getContext('2d');
          if (cctx) {
            cctx.imageSmoothingEnabled = false;
            cctx.drawImage(entry.base, 0, 0, sSize, sSize);
          }
          sizes.push(sSize);
          canvases.push(c);
        }
        entry.mips = { sizes, canvases };
      }
    } catch {
      // mark as missing with false-like entry
      s.cache[index] = undefined;
    } finally {
      s.pending.delete(`${index}`);
      s.controllers.delete(`${index}`);
      s.activeLoads = Math.max(0, s.activeLoads - 1);
      scheduleRender();
      processQueue();
    }
  }, [mapId, mask, textureMask]);

  const pickMipCanvas = (entry: Exclude<CacheTile, true | undefined>, drawSize: number): HTMLCanvasElement | ImageBitmap => {
    if (!entry.mips) return entry.base;
    const { sizes, canvases } = entry.mips;
    // sizes are descending; pick the smallest mip that is still >= drawSize
    for (let i = sizes.length - 1; i >= 0; i--) { if (sizes[i] >= drawSize) return canvases[i]; }
    // drawSize larger than the largest mip; use the largest available
    return canvases[0] ?? entry.base;
  };

  const render = () => {
    const canvas = canvasRef.current;
    const ctx = getContext();
    if (!canvas || !ctx) return;

    // Resize canvas to element size; this clears content
    const dpr = Math.max(1, Math.min(3, Math.floor(window.devicePixelRatio || 1)));
    const cssW = canvas.offsetWidth;
    const cssH = canvas.offsetHeight;
    const needResize = canvas.width !== Math.floor(cssW * dpr) || canvas.height !== Math.floor(cssH * dpr);
    if (needResize) {
      canvas.width = Math.floor(cssW * dpr);
      canvas.height = Math.floor(cssH * dpr);
      const ctx2 = getContext();
      if (ctx2) ctx2.setTransform(dpr, 0, 0, dpr, 0, 0);
    }

    ctx.imageSmoothingEnabled = false;
    // Always clear the frame in CSS pixel space to avoid lingering artifacts
    ctx.clearRect(0, 0, cssW, cssH);

    const s = stateRef.current!;
    const viewportW = cssW;
    const viewportH = cssH;
    const size = computeTileSize();

    const cache = s.cache;

    for (let x = 0; x < MAP_SIZE; x++) {
      for (let y = 0; y < MAP_SIZE; y++) {
        const { x: drawX, y: drawY } = getDrawXY(x, y, size);
        const index = (x * MAP_SIZE) + y;

        // Cull offscreen but keep tile in cache
        if (!isVisible(drawX, drawY, size, viewportW, viewportH)) continue;

        const hasTexture = textureMask?.[y]?.[x] ?? false;
        const cached = cache[index];

        if (!mask[y]?.[x] && !hasTexture) {
          // No ADT and no texture: draw grid
          strokeRect(ctx, drawX, drawY, size, 'rgba(55,65,81,0.1)');
          continue;
        }

        if (!hasTexture) {
          // Tile exists but lacks minimap texture; draw a distinct placeholder and skip fetching
          fillRect(ctx, drawX, drawY, size, '#0f172a');
        } else if (cached === undefined) {
          // mark as loading to skip duplicate work
          cache[index] = true;
          // Only queue load if we know a texture exists
          if (hasTexture) queueTile(x, y, index);
          // placeholder
          fillRect(ctx, drawX, drawY, size, '#1f2937');
        } else if (cached === true) {
          // loading placeholder
          fillRect(ctx, drawX, drawY, size, '#111827');
        } else {
          const src = pickMipCanvas(cached, size);
          ctx.drawImage(src, drawX, drawY, size, size);
        }

        // Selection overlay
        if (selectionRef.current.has(index)) {
          fillRect(ctx, drawX, drawY, size, 'rgba(16,185,129,0.35)');
        }

        // Hover overlay
        if (stateRef.current!.hoverTile === index) {
          // fillRect(ctx, drawX, drawY, size, 'rgba(59,130,246,0.1)');
        }
      }
    }

    // Live rectangle selection preview overlay (empty interior, thick green border)
    if (
      s.selectionMode === 'rect'
      && s.rectStartX !== null && s.rectStartY !== null
      && s.rectCurX !== null && s.rectCurY !== null
    ) {
      const startX = Math.max(0, Math.min(MAP_SIZE - 1, Math.min(s.rectStartX, s.rectCurX)));
      const endX = Math.max(0, Math.min(MAP_SIZE - 1, Math.max(s.rectStartX, s.rectCurX)));
      const startY = Math.max(0, Math.min(MAP_SIZE - 1, Math.min(s.rectStartY, s.rectCurY)));
      const endY = Math.max(0, Math.min(MAP_SIZE - 1, Math.max(s.rectStartY, s.rectCurY)));
      const { x: drawX, y: drawY } = getDrawXY(startX, startY, size);
      const width = (endX - startX + 1) * size;
      const height = (endY - startY + 1) * size;
      ctx.save();
      if (s.rectAction === 'remove') {
        ctx.lineWidth = 1;
        ctx.strokeStyle = '#ef4444';
        ctx.strokeRect(drawX, drawY, width, height);
      } else {
        ctx.fillStyle = 'rgba(16,185,129,0.35)';
        // Only fill not-yet-selected tiles within the rectangle
        for (let tx = startX; tx <= endX; tx++) {
          for (let ty = startY; ty <= endY; ty++) {
            const idx = (tx * MAP_SIZE) + ty;
            if (selectionRef.current.has(idx)) continue;
            const p = getDrawXY(tx, ty, size);
            ctx.fillRect(p.x, p.y, size, size);
          }
        }
      }
      ctx.restore();
    }
  };

  // Selection state (internal) with callback to parent
  const selectionRef = useRef<Set<number>>(new Set());
  const isSelectableTile = (x: number, y: number): boolean => Boolean(mask[y]?.[x] || textureMask?.[y]?.[x]);
  const emitSelection = () => {
    if (!onSelectionChange) return;
    const tiles: Point[] = [];
    selectionRef.current.forEach((index) => {
      const x = Math.floor(index / MAP_SIZE);
      const y = index % MAP_SIZE;
      tiles.push({ x, y });
    });
    onSelectionChange(tiles);
  };

  // Event handlers
  useEffect(() => {
    const canvas = canvasRef.current!;
    const ctx = getContext();
    if (!canvas || !ctx) return undefined;

    // Initial center on tile (32,32)
    const s = stateRef.current!;
    const tSize = computeTileSize();
    s.offsetX = (canvas.clientWidth / 2) - (32 * tSize);
    s.offsetY = (canvas.clientHeight / 2) - (32 * tSize);

    const onMouseMoveDoc = (e: MouseEvent) => {
      const st = stateRef.current!;
      // update hover tile continuously
      const point = mapPositionFromClientPoint(e.clientX, e.clientY);
      const clampedX = Math.max(0, Math.min(MAP_SIZE - 1, point.tileX));
      const clampedY = Math.max(0, Math.min(MAP_SIZE - 1, point.tileY));
      st.hoverTile = (clampedX * MAP_SIZE) + clampedY;

      if (st.selectionMode === 'rect') {
        st.rectCurX = clampedX;
        st.rectCurY = clampedY;
        scheduleRender();
      } else if (st.selectionMode === 'paint') {
        if (!isSelectableTile(clampedX, clampedY)) { scheduleRender(); return; }
        const idx = (clampedX * MAP_SIZE) + clampedY;
        if (st.paintAddMode) {
          selectionRef.current.add(idx);
        } else {
          selectionRef.current.delete(idx);
        }
        scheduleRender();
      } else if (st.clickPending && st.mouseDownButton === 0) {
        // convert to rectangle selection if moved beyond threshold
        const dxAbs = Math.abs(e.clientX - st.clickStartClientX);
        const dyAbs = Math.abs(e.clientY - st.clickStartClientY);
        if (dxAbs > 3 || dyAbs > 3) {
          st.selectionMode = 'rect';
          st.rectAction = st.mouseDownShift ? 'remove' : 'toggle';
          st.rectStartX = st.clickTileX;
          st.rectStartY = st.clickTileY;
          st.rectCurX = clampedX;
          st.rectCurY = clampedY;
          st.clickPending = false;
          scheduleRender();
        }
      } else if (st.isPanning) {
        const dx = st.mouseBaseX - e.clientX;
        const dy = st.mouseBaseY - e.clientY;
        st.offsetX = st.panBaseX - dx;
        st.offsetY = st.panBaseY - dy;
        scheduleRender();
      }
    };

    const onMouseUpDoc = () => {
      const st = stateRef.current!;
      if (st.isPanning) st.isPanning = false;

      if (st.selectionMode === 'rect') {
        // finalize rectangle selection (replace selection with the rect if non-empty)
        if (
          st.rectStartX !== null && st.rectStartY !== null
          && st.rectCurX !== null && st.rectCurY !== null
        ) {
          const startX = Math.max(0, Math.min(MAP_SIZE - 1, Math.min(st.rectStartX, st.rectCurX)));
          const endX = Math.max(0, Math.min(MAP_SIZE - 1, Math.max(st.rectStartX, st.rectCurX)));
          const startY = Math.max(0, Math.min(MAP_SIZE - 1, Math.min(st.rectStartY, st.rectCurY)));
          const endY = Math.max(0, Math.min(MAP_SIZE - 1, Math.max(st.rectStartY, st.rectCurY)));
          const rectSet = new Set<number>();
          console.log('mask', mask, 'textureMask', textureMask);
          for (let x = startX; x <= endX; x++) {
            for (let y = startY; y <= endY; y++) {
              console.log('x', x, 'y', y, 'isSelectableTile', isSelectableTile(x, y), 'mask', mask[y]?.[x], 'textureMask', textureMask?.[y]?.[x]);
              if (!isSelectableTile(x, y)) continue;
              rectSet.add((x * MAP_SIZE) + y);
            }
          }
          console.log('rectSet', rectSet);
          if (rectSet.size > 0) {
            if (st.rectAction === 'remove') {
              rectSet.forEach((idx) => selectionRef.current.delete(idx));
            } else {
              // toggle: add if any missing, otherwise remove all
              const shouldAdd = true;
              // for (const idx of rectSet) {
              //   if (!selectionRef.current.has(idx)) { shouldAdd = true; break; }
              // }
              if (shouldAdd) rectSet.forEach((idx) => selectionRef.current.add(idx));
              else rectSet.forEach((idx) => selectionRef.current.delete(idx));
            }
          }
        }
        st.selectionMode = 'none';
        st.rectAction = 'toggle';
        st.rectStartX = st.rectStartY = st.rectCurX = st.rectCurY = null;
        scheduleRender();
        emitSelection();
      } else if (st.selectionMode === 'paint') {
        st.selectionMode = 'none';
        st.paintAddMode = null;
        emitSelection();
      } else if (st.clickPending && st.mouseDownButton === 0) {
        // single click selection behavior
        st.clickPending = false;
        const tileX = Math.max(0, Math.min(MAP_SIZE - 1, st.clickTileX));
        const tileY = Math.max(0, Math.min(MAP_SIZE - 1, st.clickTileY));
        if (isSelectableTile(tileX, tileY)) {
          const idx = (tileX * MAP_SIZE) + tileY;
          selectionRef.current.add(idx);
          scheduleRender();
          emitSelection();
        }
      }
      st.mouseDownButton = null;
    };

    const onKeyDownDoc = (e: KeyboardEvent) => {
      const st = stateRef.current!;
      if (!st.isHovering) return;
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'a') {
        e.preventDefault();
        selectionRef.current.clear();
        for (let y = 0; y < MAP_SIZE; y++) {
          for (let x = 0; x < MAP_SIZE; x++) {
            if (isSelectableTile(x, y)) selectionRef.current.add((x * MAP_SIZE) + y);
          }
        }
        scheduleRender();
        emitSelection();
      } else if (e.key === 'Escape') {
        // clear selection and cancel modes
        selectionRef.current.clear();
        st.selectionMode = 'none';
        st.paintAddMode = null;
        st.rectStartX = st.rectStartY = st.rectCurX = st.rectCurY = null;
        st.clickPending = false;
        scheduleRender();
        emitSelection();
      }
    };

    const onResize = () => scheduleRender();

    document.addEventListener('mousemove', onMouseMoveDoc);
    document.addEventListener('mouseup', onMouseUpDoc);
    document.addEventListener('keydown', onKeyDownDoc);
    window.addEventListener('resize', onResize);
    const ro = new ResizeObserver(() => scheduleRender());
    ro.observe(canvas);

    scheduleRender();

    return () => {
      document.removeEventListener('mousemove', onMouseMoveDoc);
      document.removeEventListener('mouseup', onMouseUpDoc);
      document.removeEventListener('keydown', onKeyDownDoc);
      window.removeEventListener('resize', onResize);
      ro.disconnect();
    };
  }, [mapId, mask, textureMask]);

  // Map change: reset caches and cancel in-flight fetches
  useEffect(() => {
    const s = stateRef.current!;
    // abort all in-flight fetches
    s.controllers.forEach((c) => c.abort());
    // re-init state to defaults
    stateRef.current = createDefaultState();
    // center camera at world (tile 32,32)
    const canvas = canvasRef.current;
    if (canvas) {
      const tSize = computeTileSize();
      stateRef.current.offsetX = (canvas.clientWidth / 2) - (32 * tSize);
      stateRef.current.offsetY = (canvas.clientHeight / 2) - (32 * tSize);
    } else {
      stateRef.current.offsetX = 0;
      stateRef.current.offsetY = 0;
    }
    versionRef.current += 1;
    scheduleRender();
  }, [mapId, mask, textureMask]);

  // Clear selection overlay on map change
  useEffect(() => {
    selectionRef.current.clear();
  }, [mapId]);

  // Canvas-bound handlers
  const handleMouseOver = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const st = stateRef.current!;
    st.isHovering = true;
    const point = mapPositionFromClientPoint(e.clientX, e.clientY);
    const clampedX = Math.max(0, Math.min(MAP_SIZE - 1, point.tileX));
    const clampedY = Math.max(0, Math.min(MAP_SIZE - 1, point.tileY));
    st.hoverTile = (clampedX * MAP_SIZE) + clampedY;
    onHoverChange?.({ x: point.tileX, y: point.tileY }, { x: point.tileX * TILE_WORLD_SIZE, y: point.tileY * TILE_WORLD_SIZE });
    scheduleRender();
  };

  const handleMouseOut = () => {
    const st = stateRef.current!;
    st.isHovering = false;
    st.hoverTile = null;
    onHoverChange?.(null, null);
    scheduleRender();
  };

  const handleMouseDown = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const st = stateRef.current!;
    const point = mapPositionFromClientPoint(e.clientX, e.clientY);
    const clampedX = Math.max(0, Math.min(MAP_SIZE - 1, point.tileX));
    const clampedY = Math.max(0, Math.min(MAP_SIZE - 1, point.tileY));
    const startIdx = (clampedX * MAP_SIZE) + clampedY;

    if (e.shiftKey && e.button === 0) {
      // start rectangle selection immediately with Shift + left
      st.selectionMode = 'rect';
      st.rectAction = 'remove';
      st.rectStartX = clampedX;
      st.rectStartY = clampedY;
      st.rectCurX = clampedX;
      st.rectCurY = clampedY;
      st.clickPending = false;
      st.mouseDownButton = 0;
      st.mouseDownShift = true;
      scheduleRender();
    } else if (e.ctrlKey && e.button === 0) {
      // start paint selection
      st.selectionMode = 'paint';
      const isSelected = selectionRef.current.has(startIdx);
      // inverted state of first clicked tile determines mode
      st.paintAddMode = !isSelected;
      if (mask[clampedY]?.[clampedX]) {
        if (st.paintAddMode) selectionRef.current.add(startIdx);
        else selectionRef.current.delete(startIdx);
      }
      scheduleRender();
    } else if (e.button === 0) {
      // prepare for potential click; will convert to rectangle selection on move
      st.clickPending = true;
      st.clickStartClientX = e.clientX;
      st.clickStartClientY = e.clientY;
      st.clickTileX = clampedX;
      st.clickTileY = clampedY;
      st.mouseDownButton = 0;
      st.mouseDownShift = e.shiftKey;
    } else if (!st.isPanning && (e.button === 1 || e.button === 2)) {
      // panning only with middle or right button
      st.isPanning = true;
      st.mouseBaseX = e.clientX;
      st.mouseBaseY = e.clientY;
      st.panBaseX = st.offsetX;
      st.panBaseY = st.offsetY;
    }
  };

  const handleContextMenu = (e: React.MouseEvent<HTMLCanvasElement>) => {
    e.preventDefault();
  };

  const handleMouseWheel = (e: React.WheelEvent<HTMLCanvasElement>) => {
    const factor = e.deltaY > 0 ? 1.1 : 1 / 1.1;
    const st = stateRef.current!;
    const newZoom = Math.max(0.01, Math.min(10, st.zoomFactor * factor));
    if (newZoom !== st.zoomFactor) {
      const canvas = canvasRef.current!;
      const rect = canvas.getBoundingClientRect();
      const localX = e.clientX - rect.x;
      const localY = e.clientY - rect.y;
      const oldSize = computeTileSize();
      const fracX = (localX - st.offsetX) / oldSize;
      const fracY = (localY - st.offsetY) / oldSize;
      st.zoomFactor = newZoom;
      const newSize = computeTileSize();
      st.offsetX = localX - (fracX * newSize);
      st.offsetY = localY - (fracY * newSize);
      scheduleRender();
    }
  };

  return (
    <canvas
      ref={canvasRef}
      className={className}
      onMouseDown={handleMouseDown}
      onMouseMove={handleMouseOver}
      onMouseOut={handleMouseOut}
      onWheel={handleMouseWheel}
      onContextMenu={handleContextMenu}
      tabIndex={0}
    />
  );
}

// simple draw helpers
const fillRect = (ctx: CanvasRenderingContext2D, x: number, y: number, size: number, color: string) => {
  ctx.fillStyle = color;
  ctx.fillRect(x, y, size, size);
};
const strokeRect = (ctx: CanvasRenderingContext2D, x: number, y: number, size: number, color: string) => {
  ctx.strokeStyle = color;
  ctx.lineWidth = 1;
  ctx.strokeRect(x, y, size, size);
};

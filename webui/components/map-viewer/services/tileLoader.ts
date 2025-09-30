'use client';

import type { MapInfo } from '../minimap-viewer';
import type { MapStore } from '../store';

export type TileLoaderService = ReturnType<typeof createTileLoader>;

export function createTileLoader({
  canvas, store: s, mapInfo, scheduleRender,
}: {
  canvas: HTMLCanvasElement | null;
  store: MapStore;
  mapInfo: MapInfo;
  scheduleRender: () => void;
}) {
  const isVisible = (drawX: number, drawY: number, size: number, viewportW: number, viewportH: number) => {
    const margin = 1; // be tolerant to rounding
    if (drawX > (viewportW - margin)) return false;
    if (drawY > (viewportH - margin)) return false;
    if (drawX + size < -margin) return false;
    if (drawY + size < -margin) return false;
    return true;
  };

  const computeTileSize = (): number => Math.max(1, Math.floor(s.settings.maxTiles / s.camera.zoom));

  const getDrawXY = (x: number, y: number, size: number) => ({
    x: Math.round((x * size) + s.camera.offsetX),
    y: Math.round((y * size) + s.camera.offsetY),
  });

  const ensureTile = (x: number, y: number, index: number) => {
    if (!canvas) return;
    const size = computeTileSize();
    const p = getDrawXY(x, y, size);
    if (!isVisible(p.x, p.y, size, canvas.clientWidth, canvas.clientHeight)) return;
    if (s.tilesData.missing.has(index)) return;
    const key = `${index}`;
    if (s.tilesData.pending.has(key)) return;
    s.tilesData.pending.add(key);
    s.tilesData.queue.push([x, y, index]);
    processQueue();
  };

  const pickMipCanvas = (
    entry: Exclude<MapStore['tilesData']['cache'][number], undefined>,
    drawSize: number,
  ): HTMLCanvasElement | ImageBitmap => {
    if (!entry.mips) return entry.base;
    const { sizes, canvases } = entry.mips;
    for (let i = sizes.length - 1; i >= 0; i--) {
      if (sizes[i] >= drawSize) return canvases[i];
    }
    return canvases[0] ?? entry.base;
  };

  const processQueue = () => {
    const MAX_CONCURRENT_LOADS = 8;
    if (s.tilesData.activeLoads >= MAX_CONCURRENT_LOADS || s.tilesData.queue.length === 0) return;

    if (!canvas) return;

    const tSize = computeTileSize();
    const centerLocalX = canvas.clientWidth / 2;
    const centerLocalY = canvas.clientHeight / 2;
    const centerTileX = Math.floor((centerLocalX - s.camera.offsetX) / tSize);
    const centerTileY = Math.floor((centerLocalY - s.camera.offsetY) / tSize);

    const computePriority = (x: number, y: number): number => {
      if (s.controllers.hover.hoverTile !== null) {
        const hx = Math.floor(s.controllers.hover.hoverTile / 64);
        const hy = s.controllers.hover.hoverTile % 64;
        return Math.hypot(x - hx, y - hy);
      }
      return Math.hypot(x - centerTileX, y - centerTileY);
    };

    s.tilesData.queue.sort((a, b) => computePriority(a[0], a[1]) - computePriority(b[0], b[1]));
    while (s.tilesData.activeLoads < MAX_CONCURRENT_LOADS && s.tilesData.queue.length > 0) {
      const node = s.tilesData.queue.shift()!;
      void loadTile(node);
    }
  };

  const loadTile = async (node: [number, number, number]) => {
    s.tilesData.activeLoads += 1;
    const [x, y, index] = node;
    const localVersion = s.tilesData.version;
    try {
      if (!(mapInfo.textureMask?.[y]?.[x])) return;
      let entry = s.tilesData.cache[index];
      if (!entry) {
        const key = `${index}`;
        const prev = s.tilesData.controllers.get(key);
        prev?.abort();
        const controller = new AbortController();
        s.tilesData.controllers.set(key, controller);
        const res = await fetch(`/api/maps/${encodeURIComponent(mapInfo.mapId)}/minimap/${x}/${y}`, {
          signal: controller.signal,
          cache: 'force-cache',
        });
        if (res.status === 404) { s.tilesData.missing.add(index); return; }
        if (!res.ok) throw new Error(String(res.status));
        const blob = await res.blob();
        const bitmap = await createImageBitmap(blob, { imageOrientation: 'none', colorSpaceConversion: 'none', premultiplyAlpha: 'none' });
        if (localVersion !== s!.tilesData.version) return;
        entry = { base: bitmap };
        s.tilesData.cache[index] = entry;
      }
      if (entry && !entry.mips) {
        const baseW = entry.base.width;
        const baseH = entry.base.height;
        let sizePow2 = 1;
        while (sizePow2 * 2 <= Math.min(baseW, baseH)) sizePow2 *= 2;
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
      s.tilesData.cache[index] = undefined;
    } finally {
      s.tilesData.pending.delete(`${index}`);
      s.tilesData.controllers.delete(`${index}`);
      s.tilesData.activeLoads = Math.max(0, s.tilesData.activeLoads - 1);
      scheduleRender();
      processQueue();
    }
  };

  return { ensureTile, pickMipCanvas };
}

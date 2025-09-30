'use client';

import { useEffect, useRef, useState } from 'react';

import { useInputControllers } from './controllers';
import { renderAllLayers } from './renderer/renderer';
import { createTileLoader } from './services/tileLoader';
import { createInitialStore, MapStore } from './store';

type Point = { x: number; y: number };

export type MapInfo = {
  mapId: string;
  mask: boolean[][]; // [y][x] 64x64
  textureMask?: boolean[][]; // [y][x] 64x64 (optional): whether minimap texture exists
}

export default function MinimapViewer({
  mapInfo, className, onHoverChange, onSelectionChange,
}: {
  mapInfo: MapInfo;
  className?: string;
  onHoverChange?: (tile: Point | null) => void;
  onSelectionChange?: (tiles: Point[]) => void;
}) {
  const storeRef = useRef<MapStore>(createInitialStore());
  const store = storeRef.current;

  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [ctx, setCtx] = useState<CanvasRenderingContext2D | null>(null);
  const canvas = canvasRef.current;
  useEffect(() => {
    if (canvas) {
      setCtx(canvas.getContext('2d'));
    }
  }, [canvas]);

  const renderPendingRef = useRef<boolean>(false);
  const scheduleRender = () => {
    if (renderPendingRef.current) return;
    renderPendingRef.current = true;
    requestAnimationFrame(() => {
      renderPendingRef.current = false;
      render();
    });
  };

  const render = () => {
    if (!canvas || !ctx) return;

    // Resize canvas to element size; this clears content
    const dpr = Math.max(1, Math.min(3, Math.floor(window.devicePixelRatio || 1)));
    const cssW = canvas.offsetWidth;
    const cssH = canvas.offsetHeight;
    const needResize = canvas.width !== Math.floor(cssW * dpr) || canvas.height !== Math.floor(cssH * dpr);
    if (needResize) {
      canvas.width = Math.floor(cssW * dpr);
      canvas.height = Math.floor(cssH * dpr);
      if (ctx) ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    }

    ctx.imageSmoothingEnabled = false;
    // Always clear the frame in CSS pixel space to avoid lingering artifacts
    ctx.clearRect(0, 0, cssW, cssH);

    const viewportW = cssW;
    const viewportH = cssH;

    renderAllLayers(ctx, store, {
      mapInfo,
      viewportWidth: viewportW,
      viewportHeight: viewportH,
      services: { tileLoader },
    });
  };

  const tileLoader = createTileLoader({
    canvas,
    store,
    mapInfo,
    scheduleRender,
  });

  const emitTilesSelection = () => {
    if (!onSelectionChange) return;
    const tiles: Point[] = [];
    const maxTiles = store.settings.maxTiles;
    store.controllers.selection.selectedTiles.forEach((index) => {
      const x = Math.floor(index / maxTiles);
      const y = index % maxTiles;
      tiles.push({ x, y });
    });
    onSelectionChange(tiles);
  };

  useInputControllers({
    canvas,
    store,
    mapInfo,
    scheduleRender,
    onHoverChange,
    emitTilesSelection,
  });

  // Map change: reset caches and cancel in-flight fetches
  useEffect(() => {
    const s = store;
    // abort current loads, then reset to defaults while preserving root identity
    s.tilesData.controllers.forEach((c) => c.abort());
    Object.assign(s, createInitialStore());
    s.mapInfo = mapInfo;
    if (canvas) {
      const tileSize = s.settings.maxTiles / s.camera.zoom;
      s.camera.offsetX = (canvas.clientWidth / 2) - (32 * tileSize);
      s.camera.offsetY = (canvas.clientHeight / 2) - (32 * tileSize);
    } else {
      s.camera.offsetX = 0;
      s.camera.offsetY = 0;
    }
    // bump version to invalidate any late responses and clear selection
    s.tilesData.version += 1;
    scheduleRender();
  }, [mapInfo, canvas]);

  return (
    <canvas
      ref={canvasRef}
      className={className}
    />
  );
}

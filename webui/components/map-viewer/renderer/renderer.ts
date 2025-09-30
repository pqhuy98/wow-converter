'use client';

import { MapInfo } from '../minimap-viewer';
import type { TileLoaderService } from '../services/tileLoader';
import type { MapStore } from '../store';
import { GridLayer } from './layers/GridLayer';
import { HoverLayer } from './layers/HoverLayer';
import { RasterTileLayer } from './layers/RasterTileLayer';
import { RectPreviewLayer } from './layers/RectPreviewLayer';
import { SelectionLayer } from './layers/SelectionLayer';

export type RenderContext = {
  mapInfo: MapInfo;
  viewportWidth: number;
  viewportHeight: number;
  services: {
    tileLoader: TileLoaderService
  }
};

type Utils = {
  computeTileSize: () => number;
  getDrawXY: (x: number, y: number, size: number) => { x: number; y: number };
  isVisible: (x: number, y: number, size: number, viewportWidth: number, viewportHeight: number) => boolean;
};

export type Layer = {
  id: string;
  render: (ctx2d: CanvasRenderingContext2D, store: MapStore, rc: RenderContext, utils: Utils) => void;
};

export function renderAllLayers(
  ctx: CanvasRenderingContext2D,
  store: MapStore,
  rc: RenderContext,
) {
  const layers = [
    GridLayer,
    RasterTileLayer,
    SelectionLayer,
    HoverLayer,
    RectPreviewLayer,
  ];

  for (const layer of layers) {
    layer.render(ctx, store, {
      mapInfo: rc.mapInfo,
      viewportWidth: rc.viewportWidth,
      viewportHeight: rc.viewportHeight,
      services: { tileLoader: rc.services.tileLoader },
    }, {
      computeTileSize: (): number => Math.max(1, Math.floor(store.settings.maxTiles / store.camera.zoom)),

      getDrawXY: (x: number, y: number, size: number) => ({
        x: Math.round((x * size) + store.camera.offsetX),
        y: Math.round((y * size) + store.camera.offsetY),
      }),

      isVisible: (drawX: number, drawY: number, size: number) => {
        const margin = 1; // be tolerant to rounding
        if (drawX > (rc.viewportWidth - margin)) return false;
        if (drawY > (rc.viewportHeight - margin)) return false;
        if (drawX + size < -margin) return false;
        if (drawY + size < -margin) return false;
        return true;
      },
    });
  }
}

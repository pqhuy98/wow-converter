'use client';

import { useEffect } from 'react';

import type { MapInfo } from '../minimap-viewer';
import type { MapStore } from '../store';
import { useHoverController } from './hover';
import { usePanController } from './pan';
import { useSelectionController } from './selection';
import { useWheelZoomController } from './wheelZoom';

export type Helpers = {
  scheduleRender: () => void;
  clampTile: (x: number, y: number) => { clampedX: number; clampedY: number };
  mapPositionFromClientPoint: (clientX: number, clientY: number) => { tileX: number; tileY: number };
};

export function useInputControllers({
  canvas,
  store: s,
  mapInfo,
  scheduleRender,
  onHoverChange,
  emitTilesSelection,
}: {
  canvas: HTMLCanvasElement | null;
  store: MapStore;
  mapInfo: MapInfo;
  scheduleRender: () => void;
  onHoverChange?: (tile: { x: number; y: number } | null) => void;
  emitTilesSelection: () => void;
}) {
  // Initial center on tile (32,32)
  useEffect(() => {
    if (!canvas) return;
    const tSize = Math.max(1, Math.floor(s.settings.maxTiles / s.camera.zoom));
    s.camera.offsetX = (canvas.clientWidth / 2) - (32 * tSize);
    s.camera.offsetY = (canvas.clientHeight / 2) - (32 * tSize);
  }, [s, canvas, mapInfo]);

  // Apply all input controllers
  useEffect(() => {
    if (!canvas) return undefined;

    const maxTiles = s.settings.maxTiles;

    const helpers: Helpers = {
      scheduleRender,
      mapPositionFromClientPoint: (clientX: number, clientY: number) => {
        if (!canvas) {
          throw new Error('Canvas not found, this should not happen');
        }

        const rect = canvas.getBoundingClientRect();
        const viewX = (clientX - rect.x) - s.camera.offsetX;
        const viewY = (clientY - rect.y) - s.camera.offsetY;
        const tSize = Math.max(1, Math.floor(s.settings.maxTiles / s.camera.zoom));
        const tileX = Math.floor(viewX / tSize);
        const tileY = Math.floor(viewY / tSize);
        return { tileX, tileY };
      },
      clampTile: (tileX: number, tileY: number) => ({
        clampedX: Math.max(0, Math.min(maxTiles - 1, tileX)),
        clampedY: Math.max(0, Math.min(maxTiles - 1, tileY)),
      }),
    };

    const disposePan = usePanController({ canvas, store: s, helpers });
    const disposeHover = useHoverController({
      canvas, store: s, helpers, onHoverChange,
    });
    const disposeSelection = useSelectionController({
      canvas, store: s, helpers, emitTilesSelection,
    });
    const disposeWheelZoom = useWheelZoomController({ canvas, store: s, helpers });

    // Rerender when window is resized
    const onResize = () => scheduleRender();
    const ro = new ResizeObserver(() => scheduleRender());
    ro.observe(canvas);
    window.addEventListener('resize', onResize);

    // Prevent context menu
    const onContextMenu = (e: MouseEvent) => e.preventDefault();
    canvas.addEventListener('contextmenu', onContextMenu);

    scheduleRender();

    return () => {
      disposePan?.();
      disposeHover?.();
      disposeSelection?.();
      disposeWheelZoom?.();
      window.removeEventListener('resize', onResize);
      ro.disconnect();

      canvas.removeEventListener('contextmenu', onContextMenu);
    };
  }, [canvas, s, mapInfo, scheduleRender, onHoverChange, emitTilesSelection]);
}

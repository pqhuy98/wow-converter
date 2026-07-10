import type { MapInfo } from '../minimap-viewer';
import type { MapStore } from '../store';
import { Helpers } from '.';

export type MapHoverChange = {
  tile: { x: number; y: number };
  x: number;
  y: number;
} | null;

export function useHoverController({
  canvas, store: s, mapInfo, helpers, onHoverChange,
}: {
  canvas: HTMLCanvasElement;
  store: MapStore;
  mapInfo: MapInfo;
  helpers: Helpers;
  onHoverChange?: (hover: MapHoverChange) => void;
}) {
  const maxTiles = s.settings.maxTiles;

  const isSelectableTile = (x: number, y: number): boolean => (
    Boolean(mapInfo.mask[y]?.[x] || mapInfo.textureMask?.[y]?.[x])
  );

  const onMouseMove = (e: MouseEvent) => {
    const point = helpers.mapPositionFromClientPoint(e.clientX, e.clientY);
    const { clampedX, clampedY } = helpers.clampTile(point.tileX, point.tileY);
    const hoverIdx = (clampedX * maxTiles) + clampedY;
    s.controllers.hover.isHovering = true;
    s.controllers.hover.hoverTile = isSelectableTile(clampedX, clampedY) ? hoverIdx : null;
    if (isSelectableTile(clampedX, clampedY)) {
      const rect = canvas.getBoundingClientRect();
      onHoverChange?.({
        tile: { x: clampedX, y: clampedY },
        x: e.clientX - rect.left,
        y: e.clientY - rect.top,
      });
    } else {
      onHoverChange?.(null);
    }
    helpers.scheduleRender();
  };

  const onMouseOut = () => {
    s.controllers.hover.isHovering = false;
    s.controllers.hover.hoverTile = null;
    onHoverChange?.(null);
    helpers.scheduleRender();
  };

  canvas.addEventListener('mousemove', onMouseMove);
  canvas.addEventListener('mouseout', onMouseOut);
  return () => {
    canvas.removeEventListener('mousemove', onMouseMove);
    canvas.removeEventListener('mouseout', onMouseOut);
  };
}

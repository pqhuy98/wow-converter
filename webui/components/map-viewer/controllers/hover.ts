import type { MapStore } from '../store';
import { Helpers } from '.';

export function useHoverController({
  canvas, store: s, helpers, onHoverChange,
}: {
  canvas: HTMLCanvasElement;
  store: MapStore;
  helpers: Helpers;
  onHoverChange?: (tile: { x: number; y: number } | null) => void;
}) {
  const maxTiles = s.settings.maxTiles;

  const onMouseMove = (e: MouseEvent) => {
    const point = helpers.mapPositionFromClientPoint(e.clientX, e.clientY);
    const { clampedX, clampedY } = helpers.clampTile(point.tileX, point.tileY);
    s.controllers.hover.hoverTile = (clampedX * maxTiles) + clampedY;
    s.controllers.hover.isHovering = true;
    s.controllers.hover.hoverTile = (clampedX * maxTiles) + clampedY;
    onHoverChange?.({ x: point.tileX, y: point.tileY });
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

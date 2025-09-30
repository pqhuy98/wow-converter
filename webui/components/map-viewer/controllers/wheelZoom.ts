import type { MapStore } from '../store';
import { Helpers } from '.';

export function useWheelZoomController({ canvas, store: s, helpers }: {
  canvas: HTMLCanvasElement;
  store: MapStore;
  helpers: Helpers;
}) {
  const handleWheel = (e: WheelEvent) => {
    const factor = e.deltaY > 0 ? 1.1 : 1 / 1.1;
    const newZoom = Math.max(0.01, Math.min(10, s.camera.zoom * factor));
    if (newZoom === s.camera.zoom) return;

    const rect = canvas.getBoundingClientRect();
    const localX = e.clientX - rect.x;
    const localY = e.clientY - rect.y;
    const oldSize = s.settings.maxTiles / s.camera.zoom;
    const fracX = (localX - s.camera.offsetX) / oldSize;
    const fracY = (localY - s.camera.offsetY) / oldSize;
    s.camera.zoom = newZoom;
    const newSize = s.settings.maxTiles / s.camera.zoom;
    s.camera.offsetX = localX - (fracX * newSize);
    s.camera.offsetY = localY - (fracY * newSize);
    helpers.scheduleRender();
  };

  canvas.addEventListener('wheel', handleWheel);
  return () => canvas.removeEventListener('wheel', handleWheel);
}

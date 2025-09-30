import type { MapStore } from '../store';
import { Helpers } from '.';

export function usePanController({ canvas, store: s, helpers }: {
  canvas: HTMLCanvasElement;
  store: MapStore;
  helpers: Helpers;
}) {
  const onMouseDown = (e: MouseEvent) => {
    if (e.button === 1 || e.button === 2) {
      s.controllers.pan.isPanning = true;
      s.controllers.pan.mouseStart = [e.clientX, e.clientY];
      s.controllers.pan.cameraOffsetStart = [s.camera.offsetX, s.camera.offsetY];
    }
  };

  const onMouseMove = (e: MouseEvent) => {
    if (!s.controllers.pan.isPanning) return;
    const dx = s.controllers.pan.mouseStart[0] - e.clientX;
    const dy = s.controllers.pan.mouseStart[1] - e.clientY;
    s.camera.offsetX = s.controllers.pan.cameraOffsetStart[0] - dx;
    s.camera.offsetY = s.controllers.pan.cameraOffsetStart[1] - dy;
    helpers.scheduleRender();
  };

  const onMouseUp = () => {
    if (s.controllers.pan.isPanning) s.controllers.pan.isPanning = false;
  };

  canvas.addEventListener('mousedown', onMouseDown);
  canvas.addEventListener('mousemove', onMouseMove);
  canvas.addEventListener('mouseup', onMouseUp);

  return () => {
    canvas.removeEventListener('mousedown', onMouseDown);
    canvas.removeEventListener('mousemove', onMouseMove);
    canvas.removeEventListener('mouseup', onMouseUp);
  };
}

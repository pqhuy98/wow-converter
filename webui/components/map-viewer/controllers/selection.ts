import type { MapStore } from '../store';
import { Helpers } from '.';

export function useSelectionController({
  canvas, store: s, helpers, emitTilesSelection,
}: {
  canvas: HTMLCanvasElement;
  store: MapStore;
  helpers: Helpers;
  emitTilesSelection: () => void;
}) {
  const maxTiles = s.settings.maxTiles;

  const state = s.controllers.selection;

  const isSelectableTile = (x: number, y: number): boolean => Boolean(s.mapInfo?.mask[y]?.[x] || s.mapInfo?.textureMask?.[y]?.[x]);

  const onMouseDown = (e: MouseEvent) => {
    const point = helpers.mapPositionFromClientPoint(e.clientX, e.clientY);
    const { clampedX, clampedY } = helpers.clampTile(point.tileX, point.tileY);
    const startIdx = (clampedX * maxTiles) + clampedY;

    if (e.button === 0) {
      state.isDragging = true;
      state.action = e.shiftKey ? 'remove' : 'add';
      state.mode = e.ctrlKey ? 'paint' : 'rect';
      state.rectStart = [clampedX, clampedY];
      state.rectCur = [clampedX, clampedY];
      helpers.scheduleRender();
    }
    if (e.ctrlKey && e.button === 0) {
      state.mode = 'paint';
      const isSelected = state.selectedTiles.has(startIdx);
      state.action = isSelected ? 'remove' : 'add';
      helpers.scheduleRender();
    }
  };

  const onMouseMove = (e: MouseEvent) => {
    if (!state.isDragging) return;
    if (state.mode === 'rect') {
      const point = helpers.mapPositionFromClientPoint(e.clientX, e.clientY);
      const { clampedX, clampedY } = helpers.clampTile(point.tileX, point.tileY);
      state.rectCur = [clampedX, clampedY];
      helpers.scheduleRender();
    } else if (state.mode === 'paint') {
      const point = helpers.mapPositionFromClientPoint(e.clientX, e.clientY);
      const { clampedX, clampedY } = helpers.clampTile(point.tileX, point.tileY);
      if (isSelectableTile(clampedX, clampedY)) {
        const idx = (clampedX * maxTiles) + clampedY;
        if (state.action === 'add') state.selectedTiles.add(idx);
        else state.selectedTiles.delete(idx);
      }
      helpers.scheduleRender();
    }
  };

  const onMouseUp = () => {
    if (!state.isDragging) return;
    if (state.mode === 'rect') {
      if (state.rectStart !== null && state.rectCur) {
        const startX = Math.max(0, Math.min(maxTiles - 1, Math.min(state.rectStart[0], state.rectCur[0])));
        const endX = Math.max(0, Math.min(maxTiles - 1, Math.max(state.rectStart[0], state.rectCur[0])));
        const startY = Math.max(0, Math.min(maxTiles - 1, Math.min(state.rectStart[1], state.rectCur[1])));
        const endY = Math.max(0, Math.min(maxTiles - 1, Math.max(state.rectStart[1], state.rectCur[1])));
        const rectSet = new Set<number>();
        for (let x = startX; x <= endX; x++) {
          for (let y = startY; y <= endY; y++) {
            const idx = (x * maxTiles) + y;
            if (!isSelectableTile(x, y)) continue;
            rectSet.add(idx);
          }
        }
        if (rectSet.size > 0) {
          if (state.action === 'remove') {
            rectSet.forEach((idx) => s.controllers.selection.selectedTiles.delete(idx));
          } else {
            rectSet.forEach((idx) => s.controllers.selection.selectedTiles.add(idx));
          }
        }
      }
      state.action = 'add';
      helpers.scheduleRender();
    }
    state.rectStart = state.rectCur = null;
    state.isDragging = false;
    emitTilesSelection();
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (!s.controllers.hover.isHovering) return;
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'a') {
      e.preventDefault();
      s.controllers.selection.selectedTiles.clear();
      for (let y = 0; y < maxTiles; y++) {
        for (let x = 0; x < maxTiles; x++) {
          if (isSelectableTile(x, y)) s.controllers.selection.selectedTiles.add((x * maxTiles) + y);
        }
      }
      helpers.scheduleRender();
      emitTilesSelection();
    } else if (e.key === 'Escape') {
      if (!state.isDragging) {
        s.controllers.selection.selectedTiles.clear();
      }
      s.controllers.selection.rectStart = s.controllers.selection.rectCur = null;
      helpers.scheduleRender();
      emitTilesSelection();
    }
  };

  canvas.addEventListener('mousedown', onMouseDown);
  canvas.addEventListener('mousemove', onMouseMove);
  canvas.addEventListener('mouseup', onMouseUp);
  document.addEventListener('keydown', onKeyDown);

  return () => {
    canvas.removeEventListener('mousedown', onMouseDown);
    canvas.removeEventListener('mousemove', onMouseMove);
    canvas.removeEventListener('mouseup', onMouseUp);
    document.removeEventListener('keydown', onKeyDown);
  };
}

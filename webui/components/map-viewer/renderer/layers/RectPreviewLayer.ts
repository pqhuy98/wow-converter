'use client';

import type { Layer } from '../renderer';

export const RectPreviewLayer: Layer = {
  id: 'rect-preview',
  render(ctx, store, _rc, utils) {
    const maxTiles = store.settings.maxTiles;

    const sel = store.controllers.selection;
    if (
      sel.mode !== 'rect'
      || sel.rectStart == null || sel.rectCur == null
      || !sel.isDragging
    ) {
      return;
    }

    const size = utils.computeTileSize();
    const startX = Math.max(0, Math.min(maxTiles - 1, Math.min(sel.rectStart[0], sel.rectCur[0])));
    const endX = Math.max(0, Math.min(maxTiles - 1, Math.max(sel.rectStart[0], sel.rectCur[0])));
    const startY = Math.max(0, Math.min(maxTiles - 1, Math.min(sel.rectStart[1], sel.rectCur[1])));
    const endY = Math.max(0, Math.min(maxTiles - 1, Math.max(sel.rectStart[1], sel.rectCur[1])));

    const p = utils.getDrawXY(startX, startY, size);
    const width = (endX - startX + 1) * size;
    const height = (endY - startY + 1) * size;
    ctx.save();
    if (sel.action === 'remove') {
      ctx.lineWidth = 1;
      ctx.strokeStyle = '#ef4444';
      ctx.strokeRect(p.x, p.y, width, height);
    } else {
      ctx.fillStyle = 'rgba(16,185,129,0.35)';
      for (let tx = startX; tx <= endX; tx++) {
        for (let ty = startY; ty <= endY; ty++) {
          const idx = (tx * maxTiles) + ty;
          if (store.controllers.selection.selectedTiles.has(idx)) continue;
          const q = utils.getDrawXY(tx, ty, size);
          ctx.fillRect(q.x, q.y, size, size);
        }
      }
    }
    ctx.restore();
  },
};

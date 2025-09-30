'use client';

import type { Layer } from '../renderer';

export const SelectionLayer: Layer = {
  id: 'selection',
  render(ctx, store, rc, utils) {
    const maxTiles = store.settings.maxTiles;
    const size = utils.computeTileSize();
    store.controllers.selection.selectedTiles.forEach((index) => {
      const x = Math.floor(index / maxTiles);
      const y = index % maxTiles;
      const { x: drawX, y: drawY } = utils.getDrawXY(x, y, size);
      if (!utils.isVisible(drawX, drawY, size, rc.viewportWidth, rc.viewportHeight)) return;
      ctx.fillStyle = 'rgba(16,185,129,0.35)';
      ctx.fillRect(drawX, drawY, size, size);
    });
  },
};

'use client';

import type { Layer } from '../renderer';

export const HoverLayer: Layer = {
  id: 'hover',
  render(ctx, store, rc, utils) {
    const maxTiles = store.settings.maxTiles;
    const size = utils.computeTileSize();
    const idx = store.controllers.hover.hoverTile;
    if (idx == null) return;
    const x = Math.floor(idx / maxTiles);
    const y = idx % maxTiles;
    const { x: drawX, y: drawY } = utils.getDrawXY(x, y, size);
    if (!utils.isVisible(drawX, drawY, size, rc.viewportWidth, rc.viewportHeight)) return;
    // Subtle overlay if needed
    ctx.fillStyle = 'rgba(59,130,246,0.2)';
    ctx.fillRect(drawX, drawY, size, size);
  },
};

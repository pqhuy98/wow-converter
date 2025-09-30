'use client';

import type { Layer } from '../renderer';

export const GridLayer: Layer = {
  id: 'grid',
  render(ctx, store, rc, utils) {
    const maxTiles = store.settings.maxTiles;
    const { mask, textureMask } = rc.mapInfo;
    const size = utils.computeTileSize();
    for (let x = 0; x < maxTiles; x++) {
      for (let y = 0; y < maxTiles; y++) {
        const { x: drawX, y: drawY } = utils.getDrawXY(x, y, size);
        if (!utils.isVisible(drawX, drawY, size, rc.viewportWidth, rc.viewportHeight)) continue;
        const hasTexture = textureMask?.[y]?.[x] ?? false;
        if (!mask[y]?.[x] && !hasTexture) {
          ctx.strokeStyle = 'rgba(55,65,81,0.1)';
          ctx.lineWidth = 1;
          ctx.strokeRect(drawX, drawY, size, size);
        }
      }
    }
  },
};

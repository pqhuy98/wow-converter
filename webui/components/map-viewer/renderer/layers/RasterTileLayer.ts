'use client';

import type { Layer } from '../renderer';

export const RasterTileLayer: Layer = {
  id: 'raster',
  render(ctx, store, rc, utils) {
    const maxTiles = store.settings.maxTiles;
    const { mask, textureMask } = rc.mapInfo;
    const { tileLoader } = rc.services;
    const size = utils.computeTileSize();
    const cache = store.tilesData.cache;
    for (let x = 0; x < maxTiles; x++) {
      for (let y = 0; y < maxTiles; y++) {
        const hasTile = mask[y]?.[x] ?? false;
        if (!hasTile) continue;
        const { x: drawX, y: drawY } = utils.getDrawXY(x, y, size);
        if (!utils.isVisible(drawX, drawY, size, rc.viewportWidth, rc.viewportHeight)) continue;
        const index = (x * maxTiles) + y;
        const hasTexture = textureMask?.[y]?.[x] ?? false;
        const cached = cache[index];

        if (!hasTexture) {
          ctx.fillStyle = '#0f172a';
          ctx.fillRect(drawX, drawY, size, size);
        } else if (cached === undefined) {
          tileLoader.ensureTile(x, y, index);
          ctx.fillStyle = '#111827';
          ctx.fillRect(drawX, drawY, size, size);
        } else {
          const src = tileLoader.pickMipCanvas(cached, size);
          ctx.drawImage(src, drawX, drawY, size, size);
        }
      }
    }
  },
};

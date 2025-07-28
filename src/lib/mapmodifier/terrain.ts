import { Terrain } from '@/vendors/wc3maptranslator/data';

import {
  dataHeightMax, dataHeightMin, defaultLayer, distancePerTile,
} from '../constants';
import { nArray } from '../utils';

export function getInitialTerrain(
  height: number,
  width: number,
  defaultHeight: number = (dataHeightMin + dataHeightMax) >> 1,
): Terrain {
  const fill = <T>(v: T): T[][] => nArray(height, width, v);

  return {
    tileset: 'L',
    customTileset: true,
    tilePalette: ['Ldrt', 'Ldro', 'Ldrg', 'Lrok', 'Lgrs', 'Lgrd'],
    cliffTilePalette: ['CLdi', 'CLgr'],
    map: {
      height,
      width,
      // 32x32 map has offset -2048,-2048.
      offset: { x: -distancePerTile / 2 * width, y: -distancePerTile / 2 * height }, // Scale it according to above.
    },
    // "Masks"

    groundHeight: fill(defaultHeight),
    waterHeight: fill(dataHeightMin + 728 / 4),

    // boundaryFlag: 0: not boundary, 1: boundary. Can be all 0
    boundaryFlag: fill(false),

    // flags: 32: blight, 64: water, 128: boundary
    flags: fill(0),

    // groundTexture: texture ID Can be all 0
    groundTexture: fill(0),

    // groundVariation: looks random 0, 8, 16
    groundVariation: fill(0),

    // cliffVariation: looks random 0-7
    cliffVariation: fill(0),

    // cliffTexture: all 240
    cliffTexture: fill(240),

    layerHeight: fill(defaultLayer),
  };
}

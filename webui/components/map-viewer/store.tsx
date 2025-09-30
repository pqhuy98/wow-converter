'use client';

import { MapInfo } from './minimap-viewer';

type CacheTile = {
  base: ImageBitmap;
  mips?: { sizes: number[]; canvases: HTMLCanvasElement[] };
};

type CameraState = {
  offsetX: number;
  offsetY: number;
  zoom: number;
};

type ControllersState = {
  pan: {
    isPanning: boolean;
    cameraOffsetStart: [number, number];
    mouseStart: [number, number];
  };
  hover: {
    hoverTile: number | null;
    isHovering: boolean;
  };
  selection: {
    isDragging: boolean;
    selectedTiles: Set<number>;
    mode: 'rect' | 'paint';
    action: 'add' | 'remove';
    rectStart: [number, number] | null;
    rectCur: [number, number] | null;
  };
};

type TilesData = {
  queue: [number, number, number][];
  pending: Set<string>;
  cache: (CacheTile | undefined)[];
  missing: Set<number>;
  controllers: Map<string, AbortController>;
  activeLoads: number;
  version: number;
};

export type MapStore = {
  mapInfo?: MapInfo;
  settings: {
    maxTiles: number;
    tileRealSize: number;
  }
  camera: CameraState;
  controllers: ControllersState;
  tilesData: TilesData;
};

export function createInitialStore(): MapStore {
  const MAP_SIZE = 64;
  return {
    mapInfo: undefined,
    settings: { maxTiles: MAP_SIZE, tileRealSize: 533.33 },
    camera: { offsetX: 0, offsetY: 0, zoom: 1 },
    controllers: {
      pan: {
        isPanning: false,
        cameraOffsetStart: [0, 0],
        mouseStart: [0, 0],
      },
      hover: { hoverTile: null, isHovering: false },
      selection: {
        isDragging: false,
        selectedTiles: new Set<number>(),
        mode: 'rect',
        action: 'add',
        rectStart: null,
        rectCur: null,
      },
    },
    tilesData: {
      queue: [],
      pending: new Set<string>(),
      cache: new Array(MAP_SIZE * MAP_SIZE) as CacheTile[],
      missing: new Set<number>(),
      controllers: new Map<string, AbortController>(),
      activeLoads: 0,
      version: 0,
    },
  };
}

import {
  describe, expect, test,
} from 'bun:test';

import {
  failMapGenerateOnTileErrors,
  type MapGenerateJobRequest,
  type MapGenerateJobResult,
} from '@/server/controllers/maps-generate';

describe('map generate tile failures', () => {
  test('throws and preserves the partial result before conversion', () => {
    const request: MapGenerateJobRequest = {
      mapDir: 'northrend',
      mapID: 571,
      body: {
        tiles: [{ x: 21, y: 27 }, { x: 22, y: 27 }],
        quality: 4096,
        mapSaveName: 'parity.w3x',
        clampLower: 0,
        clampUpper: 1,
        autoClampPercent: true,
        mapAngleDeg: 0,
        unitScale: 2,
        includeBuildingInteriors: true,
        freshExport: true,
        creatures: { enable: true, allAreDoodads: true },
      },
      orderedTiles: [{ x: 21, y: 27 }, { x: 22, y: 27 }],
      tileBounds: { min: [21, 27], max: [22, 27] },
    };
    const job: { result?: MapGenerateJobResult } = {};
    const succeeded = [{
      tileX: 21,
      tileY: 27,
      result: { exportType: 'ADT_EXPORT', mainFile: 'tile.obj' },
    }];
    const failed = [{ tileX: 22, tileY: 27, error: 'missing ADT' }];

    expect(() => failMapGenerateOnTileErrors(job, request, succeeded, failed, 1))
      .toThrow('1 tile export failed (22,27: missing ADT)');
    expect(job.result).toEqual({
      id: 'WC3_MAP_GENERATE_SUMMARY',
      map: 'northrend',
      mapID: 571,
      mapSaveName: 'parity.w3x',
      outputDir: '',
      quality: 4096,
      total: 2,
      stepsPerTile: 1,
      totalSteps: 2,
      succeeded,
      failed,
    });
  });
});

import assert from 'assert';

import { MapTranslator } from '@/vendors/wc3maptranslator/translators';

/** Keep the original world coordinates and append a 180-degree copy to the south. */
export function duplicateMapSouthRotated(map: MapTranslator): void {
  const terrain = map.terrain;
  const { width, height, offset } = terrain.map;
  assert(width > 0 && height > 0 && height * 2 <= 480, 'Expanded map exceeds editor dimensions');
  const reflectX = (x: number) => 2 * offset.x + width * 128 - x;
  const reflectY = (y: number) => 2 * offset.y - y;
  const rotateDegrees = (angle: number) => ((angle + 180) % 360 + 360) % 360;

  // W3E rows start at the southwest. The shared seam belongs to the original.
  const extendVertices = <T>(rows: T[][]): T[][] => {
    assert(rows.length === height + 1 && rows.every(row => row.length === width + 1), 'Invalid terrain grid');
    return [...rows.slice(1).reverse().map(row => [...row].reverse()), ...rows.map(row => [...row])];
  };
  terrain.groundHeight = extendVertices(terrain.groundHeight);
  terrain.waterHeight = extendVertices(terrain.waterHeight);
  terrain.boundaryFlag = extendVertices(terrain.boundaryFlag);
  terrain.flags = extendVertices(terrain.flags);
  terrain.groundTexture = extendVertices(terrain.groundTexture);
  terrain.groundVariation = extendVertices(terrain.groundVariation);
  terrain.cliffVariation = extendVertices(terrain.cliffVariation);
  terrain.cliffTexture = extendVertices(terrain.cliffTexture);
  terrain.layerHeight = extendVertices(terrain.layerHeight);

  const regionIds = new Map<number, number>();
  let nextRegionId = Math.max(-1, ...map.regions.map(region => region.id)) + 1;
  const regionNames = new Set(map.regions.map(region => region.name));
  const regions = map.regions.map(region => {
    const copy = structuredClone(region);
    copy.id = nextRegionId++;
    regionIds.set(region.id, copy.id);
    copy.name = `${region.name} South`;
    while (regionNames.has(copy.name)) copy.name += ' Copy';
    regionNames.add(copy.name);
    copy.position = {
      left: reflectX(region.position.right), right: reflectX(region.position.left),
      bottom: reflectY(region.position.top), top: reflectY(region.position.bottom),
    };
    return copy;
  });
  map.regions.push(...regions);

  let nextDoodadId = Math.max(-1, ...map.doodads.map(doodad => doodad.id)) + 1;
  map.doodads.push(...map.doodads.map(doodad => {
    const copy = structuredClone(doodad);
    copy.id = nextDoodadId++;
    copy.position = [reflectX(doodad.position[0]), reflectY(doodad.position[1]), doodad.position[2]];
    copy.angle = rotateDegrees(doodad.angle);
    return copy;
  }));
  // Special terrain doodads store [variation, tileX, tileY], not world coordinates.
  const originalSpecial = map.specialDoodads.map(doodad => structuredClone(doodad));
  map.specialDoodads.forEach(doodad => { doodad.position[2] += height; });
  map.specialDoodads.push(...originalSpecial.map(doodad => ({
    ...doodad,
    position: [doodad.position[0], width - 1 - doodad.position[1], height - 1 - doodad.position[2]] as [number, number, number],
  })));

  let nextUnitId = Math.max(-1, ...map.units.map(unit => unit.id)) + 1;
  map.units.push(...map.units.map(unit => {
    const copy = structuredClone(unit);
    copy.id = nextUnitId++;
    copy.position = [reflectX(unit.position[0]), reflectY(unit.position[1]), unit.position[2]];
    // DOO unit facing is stored in radians (doodad JSON angles are degrees).
    copy.rotation = ((unit.rotation + Math.PI) % (2 * Math.PI) + 2 * Math.PI) % (2 * Math.PI);
    if (copy.waygate >= 0) copy.waygate = regionIds.get(copy.waygate) ?? copy.waygate;
    return copy;
  }));
  const cameraNames = new Set(map.cameras.map(camera => camera.name));
  map.cameras.push(...map.cameras.map(camera => {
    const copy = structuredClone(camera);
    copy.target = { x: reflectX(camera.target.x), y: reflectY(camera.target.y) };
    copy.rotation = rotateDegrees(camera.rotation);
    copy.name = `${camera.name} South`;
    while (cameraNames.has(copy.name)) copy.name += ' Copy';
    cameraNames.add(copy.name);
    return copy;
  }));

  terrain.map = { width, height: height * 2, offset: { x: offset.x, y: offset.y - height * 128 } };
  const info = map.info;
  const [left, right, , top] = info.camera.complements;
  info.camera.complements = [left, right, top, top];
  info.map.playableArea = { width: width - left - right, height: 2 * height - 2 * top };
  const oldBounds = info.camera.bounds;
  const minX = Math.min(oldBounds[0], oldBounds[2], oldBounds[4], oldBounds[6]);
  const maxX = Math.max(oldBounds[0], oldBounds[2], oldBounds[4], oldBounds[6]);
  const maxY = Math.max(oldBounds[1], oldBounds[3], oldBounds[5], oldBounds[7]);
  const minY = reflectY(maxY);
  info.camera.bounds = [minX, minY, maxX, maxY, minX, maxY, maxX, minY];
}

/** WPM and SHD contain cells, so their rotated halves do not share a row. */
export function duplicateRasterSouthRotated(buffer: Buffer, width: number, height: number, pathing: boolean): Buffer {
  const headerSize = pathing ? 16 : 0;
  const columns = width * 4;
  const rows = height * 4;
  assert(buffer.length === headerSize + columns * rows, 'Raster dimensions do not match terrain');
  if (pathing) {
    assert(buffer.toString('ascii', 0, 4) === 'MP3W' && buffer.readInt32LE(4) === 0, 'Unsupported WPM format');
    assert(buffer.readInt32LE(8) === columns && buffer.readInt32LE(12) === rows, 'WPM dimensions do not match terrain');
  }
  const cells = buffer.subarray(headerSize);
  const rotated = Buffer.from(cells).reverse();
  const header = Buffer.from(buffer.subarray(0, headerSize));
  if (pathing) header.writeInt32LE(rows * 2, 12);
  return Buffer.concat([header, rotated, cells]);
}

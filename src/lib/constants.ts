import { cpus } from 'os';

// Map generation configs
export const defaultLayer = 7;
// max height after override UI/MiscData.txt
// export const dataHeightMin = 0;
// export const dataHeightMax = 8192 * 2 - 1;
// max height for default UI/MiscData.txt
// eslint-disable-next-line import/no-mutable-exports
export let dataHeightMin = 512;
// eslint-disable-next-line import/no-mutable-exports
export let dataHeightMax = 8192 * 2 - 512;

export function setDataHeightLimit(min: number, max: number) {
  (dataHeightMin as number) = min;
  (dataHeightMax as number) = max;
}
// console.log(dataHeightMin, dataHeightMax);

export const maxGameHeightDiff = (dataHeightMax - dataHeightMin) / 4;

export const distancePerTile = 128;
export const BlizzardNull = 65535;

// Source: https://github.com/stijnherfst/HiveWE/wiki/war3map.w3e-Terrain

export function dataHeightToGameZ(dataHeight: number): number {
  return (dataHeight - 8192 + (defaultLayer - 2) * 512) / 4;
}

export function gameZToDataHeight(gameZ: number): number {
  return Math.round(gameZ * 4 + 8192 - (defaultLayer - 2) * 512);
}

export function gameZToWaterHeight(waterZ: number): number {
  return Math.round(waterZ + 89.6 * 4 + 8192);
}

export function waterHeightToGameZ(waterHeight: number): number {
  return (waterHeight - 8192) / 4 - 89.6;
}

// Get CPU core count cross-platform
export const maxConcurrency = (() => {
  try {
    const cpuCount = cpus().length;
    return Math.max(1, cpuCount - 1);
  } catch {
    return 4;
  }
})();

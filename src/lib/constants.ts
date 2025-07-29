import minimist from 'minimist';

// Map generation configs
export const defaultLayer = 15;
// max height after override UI/MiscData.txt
export const dataHeightMin = 0;
export const dataHeightMax = 8192 * 2 - 1;
// max height for default UI/MiscData.txt
// export const dataHeightMin = gameZToDataHeight(-128);
// export const dataHeightMax = gameZToDataHeight(1536);
// console.log(dataHeightMin, dataHeightMax);

export const maxGameHeightDiff = (dataHeightMax - dataHeightMin) / 4;

export const distancePerTile = 4096 / 32;
export const BlizzardNull = 65535;
export const args = minimist(process.argv.slice(2));

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

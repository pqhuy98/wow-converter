import minimist from 'minimist';

export const distancePerTile = 4096 / 32;
export const BlizzardNull = 65535;
export const args = minimist(process.argv.slice(2));
console.log(args);

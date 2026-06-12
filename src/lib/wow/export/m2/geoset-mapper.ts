/**
 * Geoset naming, ported from wow.export (src/js/3D/GeosetMapper.js).
 */

const GEOSET_GROUPS: Record<number, string> = {
  0: 'Hair',
  100: 'FacialA',
  200: 'FacialB',
  300: 'FacialC',
  400: 'Gloves',
  500: 'Boots',
  600: 'Tail',
  700: 'Ears',
  800: 'Wrists',
  900: 'Kneepads',
  1000: 'Chest',
  1100: 'Pants',
  1200: 'Tabard',
  1300: 'Trousers',
  1500: 'Cloak',
  1600: 'Chins',
  1700: 'Eyeglow',
  1800: 'Belt',
  1900: 'Bone/Tail',
  2000: 'Feet',
  2200: 'Torso',
  2300: 'HandAttach',
  2400: 'HeadAttach',
  2500: 'DHBlindfolds',
  2700: 'Head',
  2800: 'Chest',
  2900: 'MechagnomeArms',
  3000: 'MechagnomeLegs',
  3100: 'MechagnomeFeet',
  3200: 'Face',
  3300: 'Eyes',
  3400: 'Eyebrows',
  3500: 'Earrings',
  3600: 'Necklace',
  3700: 'Headdress',
  3800: 'Tails',
  3900: 'Vines',
  4000: 'Chins/Tusks',
  4100: 'Noses',
  4200: 'HairDecoA',
  4300: 'HairDecoB',
  4400: 'BodySize',
  5100: 'EyeGlowB',
};

/** Get the label for a geoset based on the group. */
export function getGeosetName(index: number, id: number): string {
  if (id === 0) return `Geoset${index}`;

  const base = Math.floor(id / 100) * 100;
  const group = GEOSET_GROUPS[base];

  if (group) return group + (id - base);

  return `Geoset${index}_${base}`;
}

export default { getGeosetName };

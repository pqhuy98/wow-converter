/* eslint-disable max-len */

import { TestConfig } from './classic';

const testCases: string[][] = [ // [base, weaponR, weaponL, size][]
  ['https://www.wowhead.com/wotlk/npc=36855/lady-deathwhisper'],
  ['https://www.wowhead.com/wotlk/npc=36612/lord-marrowgar'],
  ['https://www.wowhead.com/mop-classic/npc=71953/xuen'],
  ['https://www.wowhead.com/npc=154515/yulon', '', '', 'hero'],
  ['https://www.wowhead.com/npc=56439/sha-of-doubt', '', '', 'giant'],
  [
    'https://www.wowhead.com/npc=37187/high-overlord-saurfang',
    'https://www.wowhead.com/wotlk/item=49623/shadowmourne',
    'https://www.wowhead.com/wotlk/item=49623/shadowmourne',
  ],
  [
    'https://www.wowhead.com/npc=37119/highlord-tirion-fordring',
    'https://www.wowhead.com/item=120978/ashbringer',
  ],
  [
    'https://www.wowhead.com/wotlk/npc=36597/the-lich-king',
    'https://www.wowhead.com/classic/item=231885/frostmourne',
  ],
  ['https://www.wowhead.com/npc=102672/nythendra', '', '', 'hero'],
  ['https://www.wowhead.com/npc=211664/elisande'],
  ['https://www.wowhead.com/npc=113201/thicket-manahunter'],
  ['https://www.wowhead.com/npc=68397/lei-shen'],
  [
    'https://www.wowhead.com/npc=22917/illidan-stormrage',
    'https://www.wowhead.com/item=32837/warglaive-of-azzinoth',
    'https://www.wowhead.com/item=32838/warglaive-of-azzinoth',
    'hero',
  ],
  ['https://www.wowhead.com/npc=114895/nightbane#modelviewer', '', '', 'hero'],
  ['https://www.wowhead.com/mop-classic/npc=64986/heavenly-onyx-cloud-serpent', '', '', 'hero'],
  ['local::creature\\protodragonshadowflame\\protodragonshadowflame_body.obj', '', '', 'hero'],
  [
    'https://www.wowhead.com/npc=87607/sever-frostsprocket',
    'https://www.wowhead.com/item=141376/icy-ebon-warsword?bonus=4790',
    'https://www.wowhead.com/item=51010/the-facelifter',
  ],
  [
    'https://www.wowhead.com/npc=36857/blood-elf-warrior',
    'https://www.wowhead.com/item=31331/the-night-blade',
    'https://www.wowhead.com/item=31331/the-night-blade',
  ],
  ['https://www.wowhead.com/npc=172613/rokhan'],
  ['https://www.wowhead.com/npc=187590/merithra'],
  ['https://www.wowhead.com/npc=176789/lady-liadrin'],
  ['https://www.wowhead.com/npc=208418/flamecrested-portalweaver'],
  ['https://www.wowhead.com/npc=187609/earthcaller-yevaa'],
  ['https://www.wowhead.com/npc=209065/austin-huxworth'],
  ['https://www.wowhead.com/npc=229161/darkfuse-brute'],
  ['https://www.wowhead.com/npc=181398/malganis'],
  ['https://www.wowhead.com/npc=82057/shattered-hand'],
  ['https://www.wowhead.com/npc=245601/enforcer-jaktull'],
  ['https://www.wowhead.com/npc=214899/ebyssian'],
  ['https://www.wowhead.com/npc=3095/fela'],
  [
    'https://www.wowhead.com/npc=71865/garrosh-hellscream',
    'https://www.wowhead.com/item=28773/gorehowl',
  ],
  ['https://www.wowhead.com/npc=16867/shattered-hand-grunt'],
  ['https://www.wowhead.com/npc=37007/deathbound-ward'],
  ['https://www.wowhead.com/mop-classic/item=87777/reins-of-the-astral-cloud-serpent#modelviewer'],
  ['local::creature\\ragnaros2\\ragnaros2.obj', '', '', 'giant'],
  ['https://www.wowhead.com/object=531961/untethered-xybucha'],
  ['https://www.wowhead.com/dressing-room?ninja-turtle#fM80z0zN89c8G8ol8u8MPd8I8o28N8kq8A8kI8fo8M1r8rb8MKS8zYh8dAn8Mx4808rr8MPI8fk8M1A8rL8MQg8fV8M1o8fm8M1M8rf8MPj8rw8MPt8ri8MPE8L8ke877ozqXd87czbRH808MTC808zqgi87cwLU87czm5q808zqyd808zqhM808zbgk87MzbSq87o'],
  ['https://www.wowhead.com/dressing-room?marauder#fl80z0zN89c8ko86y8V586h8oM86C8oc86Q8om8638fb8M2k8fa8M1O8zY48dTo8MtC808fr8M2w8ff8M2N8fd8M2b8oV8zzV877hSXK8082My8082N38MzG87mSXX808SXQ808SXS8082AM8MzG8SXW8082bB87M2bB87o'],
  ['https://www.wowhead.com/dressing-room?red-dh#fm8zb0zN89c858fD8bb8Muh8bl8MNW8qP8MfC8Mtc808bF8MAa8zYA8dAU8X8rG8Z8rX8Y8MIF8bh8MuC848fx818fV8bW8Mpz8br8MuA8bw8Mut8bU8MLa8W8dY8oy8fb838fI8bx8MIS8bn8MIi876H1R8081PB808DHH87sYdM808mjK808muM8083Aj8MzI8083Aj8MzI87V'],
  ['https://www.wowhead.com/dressing-room?black-dh#fm8zb0zN89c8X8rI8W8dZ8Y8rt8Z8rX8bn8MIi8bx8MIS8bl8MNU8bF8MAs858fD8bU8MLa8qP8MfC818fa8bW8MT28zYA8dAU8bh8MuC8bb8Muf8bw8Mut8br8MuA8Mtc80848fL8oy8fq838MLZ874FKk87VzaUr87kzoyR87c30N808vxh808HPT87MHPT87o'],
  ['https://www.wowhead.com/dressing-room?frost-prince#fa80o0zN89c8zZ8jY8z18nw8z28nj8z38n18z48yo8M08yL8Mz8yt8MM8yC8og8yh8sW8z3g8aM8z5P8zYv8dLv8Mtr877inNk808zMaD808zMal808zMaO808zVpW87Vzzei8MIv83MZ8Maz8zVDJ808zMnO8MIv8zoFW87MzoFW87o'],
  ['https://www.wowhead.com/dressing-room?samurai-sniper#fz80z0zN89c8a8G8s8z8q8O8b8zc8fh8M2Q8d8zh8fu8M218sc8zya8MEt8wUK8fG8M238k58zdw8zYw8dAG8Mx2877iG4k808G4s87cG4V87VI0p808G4o808G5s808G4R808G51808r5J87s'],
  ['https://www.wowhead.com/dressing-room?plague-mage#fm80R0zN89c8H8bA8F8q58J8bH8K8MLf8bK8MAt8bO8MAB8O8ds8S8dn8U8dE8bJ8MAy8bC8MA08qO8MfV8zYN8dAO8bQ8MLs8bg8MNr8bv8MNB8MgK8wTB8MtM808on8df8Q8MLQ877MyX4808yYm87cyYM87VyP1808yX2808yPK808yY7MyPW808tAf87MtG7k'],
  ['https://www.wowhead.com/dressing-room?orc-warrior#fM80m0zN89c8G8ol8u8oV8I8o28N8kq8A8kI8fo8M1r8rb8MKS8zYh8dAn8Mx4808rr8MPI8fk8M1A8rL8MQg8fV8M1o8fm8M1M8rf8MPj8rw8MPt8ri8MPE8L8kg877oyxZ808yx287cytM87VtVQ808ytz808tmw808yx1808tVb808tMM87MtLs87o'],
  ['https://www.wowhead.com/dressing-room?tauren-druid#fn80k0zN89c8oO8SD8oK8VHu8oP8SZ8TS8VHE8rX8MWJ8TU8VHD8T18VJb8zYO8dpw8MtL808mf8VJQ8mw8SW8TZ8VJk8T28VJf8T38VJG8j08VJC8jM8VJF8jc8VJU8r18MXm8oQ8S58T48VJt8T58VJE876zRZ7MzRZM808zRZm808zRY387VzRZV808zRY5808zRZc808zRZz808zRY4808zbCP8cBO808zbC58cBO87V'],
];

export const testConfigRetail: TestConfig = {
  name: 'WoW Retail',
  map: 'maps/test-regression-retail.w3x',
  testCases,
};

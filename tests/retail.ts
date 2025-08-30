/* eslint-disable max-len */
import { Size } from '@/lib/converter/character';

import { TestConfig } from './classic';

const testCases: [string, string, string, Size | ''][] = [
  ['https://www.wowhead.com/wotlk/npc=36855/lady-deathwhisper', '', '', ''],
  ['https://www.wowhead.com/wotlk/npc=36612/lord-marrowgar', '', '', ''],
  ['https://www.wowhead.com/mop-classic/npc=71953/xuen', '', '', ''],
  ['https://www.wowhead.com/npc=154515/yulon', '', '', 'hero'],
  ['https://www.wowhead.com/npc=56439/sha-of-doubt', '', '', 'giant'],
  [
    'https://www.wowhead.com/npc=37187/high-overlord-saurfang',
    'https://www.wowhead.com/wotlk/item=49623/shadowmourne',
    'https://www.wowhead.com/wotlk/item=49623/shadowmourne',
    '',
  ],
  [
    'https://www.wowhead.com/npc=37119/highlord-tirion-fordring',
    'https://www.wowhead.com/item=120978/ashbringer',
    '',
    '',
  ],
  [
    'https://www.wowhead.com/wotlk/npc=36597/the-lich-king',
    'https://www.wowhead.com/classic/item=231885/frostmourne',
    '',
    '',
  ],
  ['https://www.wowhead.com/npc=102672/nythendra', '', '', 'hero'],
  ['https://www.wowhead.com/npc=211664/elisande', '', '', ''],
  ['https://www.wowhead.com/npc=113201/thicket-manahunter', '', '', ''],
  ['https://www.wowhead.com/npc=68397/lei-shen', '', '', ''],
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
    '',
  ],
  [
    'https://www.wowhead.com/npc=36857/blood-elf-warrior',
    'https://www.wowhead.com/item=31331/the-night-blade',
    'https://www.wowhead.com/item=31331/the-night-blade',
    '',
  ],
  ['https://www.wowhead.com/npc=172613/rokhan', '', '', ''],
  ['https://www.wowhead.com/npc=187590/merithra', '', '', ''],
  ['https://www.wowhead.com/npc=176789/lady-liadrin', '', '', ''],
  ['https://www.wowhead.com/npc=208418/flamecrested-portalweaver', '', '', ''],
  ['https://www.wowhead.com/npc=187609/earthcaller-yevaa', '', '', ''],
  ['https://www.wowhead.com/npc=209065/austin-huxworth', '', '', ''],
  ['https://www.wowhead.com/npc=229161/darkfuse-brute', '', '', ''],
  ['https://www.wowhead.com/npc=181398/malganis', '', '', ''],
  ['https://www.wowhead.com/npc=82057/shattered-hand', '', '', ''],
  ['https://www.wowhead.com/npc=245601/enforcer-jaktull', '', '', ''],
  ['https://www.wowhead.com/npc=214899/ebyssian', '', '', ''],
  ['https://www.wowhead.com/npc=3095/fela', '', '', ''],
  [
    'https://www.wowhead.com/npc=71865/garrosh-hellscream',
    'https://www.wowhead.com/item=28773/gorehowl',
    '',
    '',
  ],
  ['https://www.wowhead.com/npc=16867/shattered-hand-grunt', '', '', ''],
  ['https://www.wowhead.com/dressing-room#fM80z0zN89c8G8ol8u8MPd8I8o28N8kq8A8kI8fo8M1r8rb8MKS8zYh8dAn8Mx4808rr8MPI8fk8M1A8rL8MQg8fV8M1o8fm8M1M8rf8MPj8rw8MPt8ri8MPE8L8ke877ozqXd87czbRH808MTC808zqgi87cwLU87czm5q808zqyd808zqhM808zbgk87MzbSq87o?ninja-turtle', '', '', ''],
  ['https://www.wowhead.com/dressing-room#fl80z0zN89c8ko86y8V586h8oM86C8oc86Q8om8638fb8M2k8fa8M1O8zY48dTo8MtC808fr8M2w8ff8M2N8fd8M2b8oV8zzV877hSXK8082My8082N38MzG87mSXX808SXQ808SXS8082AM8MzG8SXW8082bB87M2bB87o?marauder', '', '', ''],
  ['https://www.wowhead.com/dressing-room#fm8zb0zN89c858fD8bb8Muh8bl8MNW8qP8MfC8Mtc808bF8MAa8zYA8dAU8X8rG8Z8rX8Y8MIF8bh8MuC848fx818fV8bW8Mpz8br8MuA8bw8Mut8bU8MLa8W8dY8oy8fb838fI8bx8MIS8bn8MIi876H1R8081PB808DHH87sYdM808mjK808muM8083Aj8MzI8083Aj8MzI87V?red-dh', '', '', ''],
  ['https://www.wowhead.com/dressing-room#fm8zb0zN89c8X8rI8W8dZ8Y8rt8Z8rX8bn8MIi8bx8MIS8bl8MNU8bF8MAs858fD8bU8MLa8qP8MfC818fa8bW8MT28zYA8dAU8bh8MuC8bb8Muf8bw8Mut8br8MuA8Mtc80848fL8oy8fq838MLZ874FKk87VzaUr87kzoyR87c30N808vxh808HPT87MHPT87o?black-dh', '', '', ''],
  ['https://www.wowhead.com/dressing-room#fa80o0zN89c8zZ8jY8z18nw8z28nj8z38n18z48yo8M08yL8Mz8yt8MM8yC8og8yh8sW8z3g8aM8z5P8zYv8dLv8Mtr877inNk808zMaD808zMal808zMaO808zVpW87Vzzei8MIv83MZ8Maz8zVDJ808zMnO8MIv8zoFW87MzoFW87o?frost-prince', '', '', ''],
];

export const testConfigRetail: TestConfig = {
  name: 'WoW Retail',
  map: 'maps/test-regression-retail.w3x',
  testCases,
};

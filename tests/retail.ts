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
  ['https://www.wowhead.com/npc=187609/earthcaller-yevaa', '', '', ''],
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
];

export const testConfigRetail: TestConfig = {
  name: 'WoW Retail',
  map: 'maps/test-regression-retail.w3x',
  testCases,
};

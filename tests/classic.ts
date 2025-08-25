import { Size } from '@/lib/converter/character';

export interface TestConfig {
  name: string;
  map: string;
  testCases: [string, string, string, Size | ''][];
}

const classicTestCases: [string, string, string, Size | ''][] = [
  ['https://www.wowhead.com/mop-classic/npc=28714/ildine-sorrowspear', '', '', ''],
  ['https://www.wowhead.com/mop-classic/npc=28674/aludane-whitecloud', '', '', ''],
  ['https://www.wowhead.com/mop-classic/npc=30115/vereesa-windrunner', '', '', ''],
  ['https://www.wowhead.com/mop-classic/npc=32678/emeline-fizzlefry', '', '', ''],
  ['https://www.wowhead.com/mop-classic/npc=32677/whirt-the-all-knowing', '', '', ''],
];

export const testConfigClassic: TestConfig = {
  name: 'WoW Classic',
  map: 'maps/test-regression-classic.w3x',
  testCases: classicTestCases,
};

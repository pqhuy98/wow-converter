export interface TestConfig {
  name: string;
  map: string;
  testCases: string[][];
}

const classicTestCases: string[][] = [
  ['https://www.wowhead.com/mop-classic/npc=28714/ildine-sorrowspear'],
  ['https://www.wowhead.com/mop-classic/npc=28674/aludane-whitecloud'],
  ['https://www.wowhead.com/mop-classic/npc=30115/vereesa-windrunner'],
  ['https://www.wowhead.com/mop-classic/npc=32678/emeline-fizzlefry'],
  ['https://www.wowhead.com/mop-classic/npc=32677/whirt-the-all-knowing'],
  ['https://www.wowhead.com/wotlk/npc=36612/lord-marrowgar'],
  ['https://www.wowhead.com/wotlk/npc=36853/sindragosa', '', '', 'hero'],
  ['https://www.wowhead.com/mop-classic/dressing-room?orc-warrior#fM80m0zN89c8u8VkZ8G8VRs8I8VRh8N8VRp8A8VRe877gyxZ808yx2808zRZm808ytM87VtVQ808ytz808tmw808yx1808tVb808tMM87MtLs87o'],
  ['https://www.wowhead.com/mop-classic/dressing-room?plague-mage#fm80R0zN89c8F8Vqp8H8Vqv8J8VqK8K8VqX8O8Vb77eyX4808yYm87cyYM87VyP1808yX2808yPK808yY7MyPW808tAf87MtG7k'],
  ['https://www.wowhead.com/mop-classic/dressing-room?samurai-sniper#fz80z0zN89c8s8VVX8a8Vom8q8Vow8b8Von8d8VoC877gG4k808G4s87cG4V87VI0p808G4o808G5s808G4R808G51808r5J87s'],
  ['https://www.wowhead.com/mop-classic/dressing-room?dwarf-archer#fc80z0zN89c8x8Vsy8t8VsU8g8Va08e8Vaq8v8VaI877gfq3808fq187cfqZ87Vfpm808fq4808fsq808fq2808fP5808rWK87s'],
  ['https://www.wowhead.com/mop-classic/dressing-room?goblin-rogue#fs80m0zN89c8zJ8Vhv8MvI8wl38zO8VhB8MvN8wBL8MvA8wBl877vpOB87cpKv87VpS7MpKl808pUw808pKC808pSY808tAf87MtLi87o'],
  ['https://www.wowhead.com/mop-classic/dressing-room?troll-warrior#fR80m0zN89c8zg8Vik8ze8ViI8zv8wlE8zE8Vie8zl8ViD877vpOi87cpOG87VpSN808pOu808pUu808pOd808pST808pqS87s'],
  ['https://www.wowhead.com/mop-classic/dressing-room?undead-rogue#fV80m0zN89c868VbW8zz8Vb38zM8Rja8zc8Vdi8zm8Vdj877vdTV87cdxz87VdAJ808dlh808dls808dlT808dLC808fck87MdCt87o'],
];

export const testConfigClassic: TestConfig = {
  name: 'WoW Classic',
  map: 'maps/test-regression-classic.w3x',
  testCases: classicTestCases,
};

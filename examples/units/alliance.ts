import esMain from 'es-main';

import { local, wowhead } from '@/lib/converter/character';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { outputDir } from '@/server/config';

import { ce } from './common';

async function muradin() {
  const mdl = await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/npc=37200/muradin-bronzebeard'),
    attackTag: '1H',
    size: 'hero',
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: local('item/objectcomponents/weapon/mace_1h_alliancecovenant_d_01_blue.obj'), scale: 1 },
      [WoWAttachmentID.HandLeft]: { path: local('item/objectcomponents/weapon/axe_1h_pvealliance_d_01.obj'), scale: 1 },
    },
    portraitCameraSequenceName: 'Stand Ready',
    inGameMovespeed: 270,
  }, 'muradin');

  mdl.sequences = mdl.sequences.filter((s) => [
    'Swim', 'Fast', 'Hit', 'Spin', 'Sleep',
  ].every((str) => !s.name.includes(str)));

  const spell = mdl.sequences.find((s) => s.name.includes('Spell'));
  mdl.sequences = mdl.sequences.filter((s) => s === spell || !s.name.includes('Spell'));

  const atkSlam = mdl.sequences.find((s) => s.name.includes('Attack Slam'));
  mdl.sequences = mdl.sequences.filter((s) => s === atkSlam || !s.name.includes('Attack Slam'));
}

async function muradin2() {
  const mdl = await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/npc=37200/muradin-bronzebeard'),
    attackTag: '2H',
    size: 'hero',
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: local('item/objectcomponents/weapon/mace_1h_alliancecovenant_d_01_blue.obj'), scale: 1 },
    },
    portraitCameraSequenceName: 'Stand Ready',
    inGameMovespeed: 270,
  }, 'muradin2');

  mdl.sequences = mdl.sequences.filter((s) => [
    'Swim', 'Fast', 'Hit', 'Spin', 'Sleep',
  ].every((str) => !s.name.includes(str)));

  const spell = mdl.sequences.find((s) => s.name.includes('Spell'));
  mdl.sequences = mdl.sequences.filter((s) => s === spell || !s.name.includes('Spell'));

  const atkSlam = mdl.sequences.find((s) => s.name.includes('Attack Slam'));
  mdl.sequences = mdl.sequences.filter((s) => s === atkSlam || !s.name.includes('Attack Slam'));
}

export async function main() {
  await muradin();
  await muradin2();

  ce.optimizeModelsTextures();
  await ce.writeAllTextures(outputDir);
  ce.writeAllModels(outputDir, 'mdx');
  console.log('Alliance done');
}

if (esMain(import.meta)) {
  void main().then(() => process.exit(0));
}

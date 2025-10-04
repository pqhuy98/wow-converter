import esMain from 'es-main';

import { local, wowhead } from '@/lib/converter/character';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { outputDir } from '@/server/config';

import { ce } from './common';

async function garrosh() {
  const mdl = await ce.exportCharacter({
    base: local('creature/garrosh2/garrosh2'),
    inGameMovespeed: 270,
    size: 'hero',
    attackTag: '2H',
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: local('item/objectcomponents/weapon/axe_2h_gorehowl_d_01_axe_2h_gorehowl_c_01'), scale: 1 },
    },
  }, 'garrosh');
  mdl.sequences = mdl.sequences.filter((s) => [
    'Swim', 'Fast', 'Hit', 'Spin', 'Sleep',
  ].every((str) => !s.name.includes(str)));
}

async function varokSaurfang() {
  const mdl = await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/wotlk/npc=37187/high-overlord-saurfang'),
    attackTag: '2H',
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: wowhead('https://www.wowhead.com/mop-classic/item=51905/ramaladnis-blade-of-culling'), scale: 1 },
    },
    inGameMovespeed: 270,
  }, 'varok-saurfang');
  mdl.sequences = mdl.sequences.filter((s) => [
    'Swim', 'Fast', 'Hit', 'Spin', 'Sleep',
  ].every((str) => !s.name.includes(str)));

  const spell = mdl.sequences.find((s) => s.name.includes('Spell'));
  mdl.sequences = mdl.sequences.filter((s) => s === spell || !s.name.includes('Spell'));

  const atkSlam = mdl.sequences.find((s) => s.name.includes('Attack Slam'));
  mdl.sequences = mdl.sequences.filter((s) => s === atkSlam || !s.name.includes('Attack Slam'));
}

async function korkronGeneral() {
  const urls = [
    // 'https://wow.zamimg.com/modelviewer/wrath/meta/npc/30750.json',
    'https://wow.zamimg.com/modelviewer/wrath/meta/npc/30751.json',
    'https://wow.zamimg.com/modelviewer/wrath/meta/npc/30752.json',
  ];
  for (const url of urls) {
    const mdl = await ce.exportCharacter({
      base: wowhead(url),
      attackTag: '1H',
      attachItems: {
        [WoWAttachmentID.HandRight]: { path: wowhead('https://www.wowhead.com/mop-classic/item=36580/dire-axe'), scale: 1 },
        [WoWAttachmentID.HandLeft]: { path: wowhead('https://www.wowhead.com/mop-classic/item=36580/dire-axe'), scale: 1 },
      },
      inGameMovespeed: 270,
    }, `korkron-general-${url.split('/').pop()!.split('.')[0]}`);

    mdl.sequences = mdl.sequences.filter((s) => [
      'Swim', 'Fast', 'Hit', 'Spin', 'Sleep',
    ].every((str) => !s.name.includes(str)));

    const spell = mdl.sequences.find((s) => s.name.includes('Spell'));
    mdl.sequences = mdl.sequences.filter((s) => s === spell || !s.name.includes('Spell'));

    const atkSlam = mdl.sequences.find((s) => s.name.includes('Attack Slam'));
    mdl.sequences = mdl.sequences.filter((s) => s === atkSlam || !s.name.includes('Attack Slam'));
  }
}

export async function main() {
  // await garrosh();
  await varokSaurfang();
  // await korkronGeneral();

  ce.optimizeModelsTextures();
  await ce.writeAllTextures(outputDir);
  ce.writeAllModels(outputDir, 'mdx');
}

if (esMain(import.meta)) {
  void main().then(() => process.exit(0));
}

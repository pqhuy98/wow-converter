import { writeFileSync } from 'fs';

import { local, wowhead } from '@/lib/converter/character';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';

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
  mdl.modify
    .setWowAttachmentScale(WoWAttachmentID.ShoulderLeft, 1.75)
    .setWowAttachmentScale(WoWAttachmentID.ShoulderRight, 1.75);
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
    if (!url.includes('30752')) {
      mdl.modify
        .setWowAttachmentScale(WoWAttachmentID.ShoulderLeft, 1.75)
        .setWowAttachmentScale(WoWAttachmentID.ShoulderRight, 1.75);
    }

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
  await garrosh();
  await varokSaurfang();
  await korkronGeneral();

  ce.models.forEach(([model, path]) => {
    model.modify
      .sortSequences()
      .removeUnusedVertices()
      .removeUnusedNodes()
      .removeUnusedMaterialsTextures()
      .removeCinematicSequences()
      .optimizeKeyFrames();
    model.sync();
    // model.sequences.sort((s1, s2) => s1.interval[0] - s2.interval[0])
    // model.sequences.forEach(s => s.name += " " + s.data.wowName)
    // writeFileSync(path + ".mdl", model.toString())
    writeFileSync(`${path}.mdx`, model.toMdx());
    console.log('Wrote character model to', path);
  });
  ce.assetManager.purgeTextures(ce.models.flatMap(([m]) => m.textures.map((t) => t.image)));
  ce.assetManager.exportTextures(ce.outputPath);
}

if (require.main === module) {
  void main();
}

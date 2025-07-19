import fs from 'fs-extra';

import { wowhead } from '@/lib/converter/character';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';

import { ce } from './common';

async function ebonBladeCommanders() {
  const urls = [
    'https://wow.zamimg.com/modelviewer/wrath/meta/npc/30859.json', // orc
    'https://wow.zamimg.com/modelviewer/wrath/meta/npc/30860.json', // human
    'https://wow.zamimg.com/modelviewer/wrath/meta/npc/30861.json', // night elf
    'https://wow.zamimg.com/modelviewer/wrath/meta/npc/30862.json', // undead
  ];

  for (let i = 0; i < urls.length; i++) {
    const mdl = await ce.exportCharacter({
      base: wowhead(urls[i]),
      attachItems: {
        [WoWAttachmentID.HandRight]: { path: wowhead('https://www.wowhead.com/wotlk/item=38632/greatsword-of-the-ebon-blade'), scale: 1 },
      },
      size: 'hero',
      attackTag: '2H',
      inGameMovespeed: 270,
      portraitCameraSequenceName: 'Stand Ready',
    }, `ebon-blade-commander-${i}`);
    if (urls[i].includes('30859')) {
      mdl.modify.setWowAttachmentScale(WoWAttachmentID.ShoulderLeft, 2);
      mdl.modify.setWowAttachmentScale(WoWAttachmentID.ShoulderRight, 2);
    }
    if (urls[i].includes('30862')) {
      mdl.geosets.find((g) => g.name.includes('Facial'))!.material = mdl.materials[0];
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

async function argentCommanders() {
  const urls = [
    'https://wow.zamimg.com/modelviewer/wrath/meta/npc/26224.json', // human
    'https://wow.zamimg.com/modelviewer/wrath/meta/npc/26225.json', // dwarf
    'https://wow.zamimg.com/modelviewer/wrath/meta/npc/26226.json', // blood elf male
    'https://wow.zamimg.com/modelviewer/wrath/meta/npc/26227.json', // blood elf female
  ];

  for (let i = 0; i < urls.length; i++) {
    const mdl = await ce.exportCharacter({
      base: wowhead(urls[i]),
      attachItems: {
        [WoWAttachmentID.HandRight]: { path: wowhead('https://www.wowhead.com/wotlk/item=40395/torch-of-holy-fire'), scale: 1 },
        [WoWAttachmentID.Shield]: { path: wowhead('https://www.wowhead.com/wotlk/item=49933/argent-crusaders-shield'), scale: 1 },
      },
      size: 'hero',
      attackTag: '1H',
      inGameMovespeed: 270,
      portraitCameraSequenceName: urls[i].includes('26224') ? 'Stand Ready' : undefined,
    }, `argent-commander-${i}`);
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
  await ebonBladeCommanders();
  await argentCommanders();

  ce.models.forEach(([model, filePath]) => {
    model.modify
      .sortSequences()
      .removeUnusedVertices()
      .removeUnusedNodes()
      .removeUnusedMaterialsTextures()
      .removeCinematicSequences()
      .optimizeKeyFrames();
    model.sync();
    // model.sequences.sort((s1, s2) => s1.interval[0] - s2.interval[0])
    // model.sequences.forEach((s) => s.name = `${s.name} | ${s.data.wowName} | ${s.data.attackTag}`);
    // writeFileSync(`${filePath}.mdl`, model.toString());
    fs.writeFileSync(`${filePath}.mdx`, model.toMdx());
    console.log('Wrote character model to', filePath);
  });

  ce.assetManager.purgeTextures(ce.models.flatMap(([m]) => m.textures.map((t) => t.image)));
  ce.assetManager.exportTextures(ce.outputPath);
}

if (require.main === module) {
  void main();
}

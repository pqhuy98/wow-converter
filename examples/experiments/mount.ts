import 'dotenv/config';

import esMain from 'es-main';

import { Character, wowhead } from '@/lib/converter/character';
import { getWoWAttachmentName, WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';

import { ce } from '../units/common';

// const outputDir = 'maps/test.w3x';
const outputDir2 = 'exported-assets';

async function mountTest() {
  const params: Character = {
    base: wowhead('https://www.wowhead.com/wotlk/npc=36597/the-lich-king'),
    attackTag: '2H',
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: wowhead('https://www.wowhead.com/classic/item=231885/frostmourne'), scale: 1 },
    },
    inGameMovespeed: 270,
  };

  const char = await ce.exportCharacter(params, 'main');
  const rider = await ce.exportCharacter(params, 'rider');
  rider.sequences = rider.sequences.filter((s) => ['Mount', 'Death'].some((name) => s.name === name));
  rider.sequences.filter((s) => s.name.includes('Mount')).forEach((s) => {
    s.name = 'Stand';
  });
  const newOverhead = rider.attachments.find((a) => a.data?.wowAttachment.wowAttachmentId === WoWAttachmentID.PlayerNameMounted);
  const oldOverhead = rider.attachments.find((a) => a.data?.wowAttachment.wowAttachmentId === WoWAttachmentID.PlayerName);
  if (newOverhead) {
    newOverhead.name = 'Overhead';
    if (oldOverhead) {
      oldOverhead.name = `Wow:${WoWAttachmentID.PlayerName}:${getWoWAttachmentName(WoWAttachmentID.PlayerName)}`;
    }
  }

  const mount1 = await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/wotlk/item=50818/invincibles-reins'),
    inGameMovespeed: 270,
  }, 'invincible-reins');

  const mount2 = await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/npc=28531/frost-wyrm-mount'),
    inGameMovespeed: 270,
  }, 'frost-wyrm');
  mount2.modify.scale(0.75);

  [mount1, mount2].forEach((mount) => {
    const mountBone = mount.wowAttachments.find((a) => a.wowAttachmentId === WoWAttachmentID.Shield)!.bone;
    const atm = mount.modify.addItemPathToBone('rider.mdx', mountBone, true);
    if (mount === mount1) {
      atm.translation = {
        type: 'translation',
        globalSeq: mount.globalSequences.at(-1)!,
        interpolation: 'DontInterp',
        keyFrames: new Map([[0, [-10, 0, 10]]]),
      };
    }
  });
}

export async function main() {
  await mountTest();

  ce.optimizeModelsTextures();
  // ce.writeAllModels(outputDir, 'mdx');
  ce.writeAllModels(outputDir2, 'mdx');
  // await ce.writeAllTextures(outputDir);
  await ce.writeAllTextures(outputDir2);
}

if (esMain(import.meta)) {
  void main().then(() => process.exit(0));
}

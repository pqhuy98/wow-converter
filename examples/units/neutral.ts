import esMain from 'es-main';

import { displayID, local, wowhead } from '@/lib/converter/character';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { outputDir } from '@/server/config';

import { ce } from './common';

async function bolvarIcc() {
  await ce.exportCharacter({
    base: local('creature\\lavaman\\lavaman'),
    inGameMovespeed: 270,
    attackTag: '2H',
    size: 'hero',
  }, 'bolvar');
}

async function wolf() {
  await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/wotlk/npc=299/diseased-young-wolf#modelviewer'),
    inGameMovespeed: 270,
    // attackTag: '2H',
    size: 'hero',
  }, 'wolf');

  // Second model of https://www.wowhead.com/wotlk/npc=37012/ancient-skeletal-soldier
  const mdl = await ce.exportCharacter({
    base: wowhead('https://wow.zamimg.com/modelviewer/wrath/meta/npc/30617.json'),
    size: 'medium',
    attackTag: '1H',
    inGameMovespeed: 270,
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: displayID(65580), scale: 1 },
      [WoWAttachmentID.Shield]: { path: displayID(65580), scale: 1 },
    },
    portraitCameraSequenceName: 'Stand Ready',
  }, 'ancient-skeletal-soldier');
  mdl.sequences = mdl.sequences.filter((s) => !s.name.includes('Attack') || s.data.wowName === 'Attack1H');
}

export async function main() {
  await bolvarIcc();
  await wolf();
  ce.optimizeModelsTextures();
  await ce.writeAllTextures(outputDir);
  ce.writeAllModels(outputDir, 'mdx');
}

if (esMain(import.meta)) {
  void main().then(() => process.exit(0));
}

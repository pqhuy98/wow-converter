import esMain from 'es-main';

import { wowhead } from '@/lib/converter/character';
import { MDL } from '@/lib/formats/mdl/mdl';
import { Vector3 } from '@/lib/math/common';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { outputDir } from '@/server/config';

import { ce } from './common';

const scourgeAxe = wowhead('https://www.wowhead.com/wotlk/item=40384/betrayer-of-humanity');

async function cultFanatic() {
  const mdl = await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/wotlk/npc=37890/cult-fanatic'),
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: scourgeAxe, scale: 1 },
    },
    size: 'hero',
    attackTag: '2H',
    inGameMovespeed: 270,
    portraitCameraSequenceName: 'Stand Ready',
  }, 'cult-fanatic');
  mdl.geosets = mdl.geosets.filter((g) => !g.name.includes('Hair'));
  mdl.sequences.filter((s) => s.data.wowName.startsWith('Kneel')).forEach((s) => s.keep = true);
  hideWeapon(mdl, WoWAttachmentID.HandRight, ['Cinematic Kneel']);
}

async function reanimatedFanatic() {
  const mdl = await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/wotlk/npc=38009/reanimated-fanatic'),
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: scourgeAxe, scale: 1 },
    },
    size: 'hero',
    attackTag: '2H',
    inGameMovespeed: 270,
  }, 'reanimated-fanatic');
  boneMelee2H(mdl);
}

const scourgeStaff = wowhead('https://www.wowhead.com/wotlk/item=39394/charmed-cierge#modelviewer');

async function cultAdherent() {
  const mdl = await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/wotlk/npc=37949/cult-adherent'),
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: scourgeStaff, scale: 1 },
    },
    size: 'hero',
    attackTag: '1H',
    inGameMovespeed: 270,
    portraitCameraSequenceName: 'Stand Ready',
    keepCinematic: true,
  }, 'cult-adherent');
  mdl.geosets = mdl.geosets.filter((g) => !g.name.includes('Hair'));
  mdl.sequences = mdl.sequences.filter((s) => !s.name.includes('Attack') || s.data.wowName === 'Attack1H');
  mdl.sequences.filter((s) => s.data.wowName.startsWith('Kneel')).forEach((s) => s.keep = true);
  hideWeapon(mdl, WoWAttachmentID.HandRight, ['Cinematic Kneel']);
  mdl.modify.removeCinematicSequences();
}

async function reanimatedAdherent() {
  const mdl = await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/wotlk/npc=38010/reanimated-adherent'),
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: scourgeStaff, scale: 1.25 },
    },
    size: 'hero',
    attackTag: '1H',
    inGameMovespeed: 270,
  }, 'reanimated-adherent');
  mdl.modify.useWalkSequenceByWowName('Walk');
  mdl.sequences = mdl.sequences.filter((s) => !s.name.includes('Attack') || s.data.wowName === 'Attack1H');
}

async function deathboundWard() {
  const mdl = await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/wotlk/npc=37007/deathbound-ward'),
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: wowhead('https://www.wowhead.com/wotlk/item=50267/tyrannical-beheader'), scale: 1.25 },
    },
    size: 'giant',
    attackTag: '2H',
    inGameMovespeed: 270,
  }, 'deathbound-ward');
  boneMelee2H(mdl);
  hideWeapon(mdl, WoWAttachmentID.HandRight, ['Sleep']);
}

async function ancientSkeletalSoldier() {
  // Second model of https://www.wowhead.com/wotlk/npc=37012/ancient-skeletal-soldier
  const mdl = await ce.exportCharacter({
    base: wowhead('https://wow.zamimg.com/modelviewer/wrath/meta/npc/30617.json'),
    size: 'medium',
    attackTag: '1H',
    inGameMovespeed: 270,
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: wowhead('https://www.wowhead.com/wotlk/item=39730/widows-fury'), scale: 1 },
      [WoWAttachmentID.Shield]: { path: wowhead('https://www.wowhead.com/wotlk/item=51452/wrathful-gladiators-barrier'), scale: 1 },
    },
    portraitCameraSequenceName: 'Stand Ready',
  }, 'ancient-skeletal-soldier');
  boneMelee2H(mdl);
  mdl.sequences = mdl.sequences.filter((s) => !s.name.includes('Attack') || s.data.wowName === 'Attack1H');
}

export async function main() {
  // await cultFanatic();
  await cultAdherent();
  // await reanimatedFanatic();
  // await reanimatedAdherent();
  // await deathboundWard();
  // await ancientSkeletalSoldier();

  ce.optimizeModelsTextures();
  await ce.writeAllTextures(outputDir);
  await ce.writeAllModels(outputDir, 'mdx');
  process.exit(0);
}

function boneMelee2H(mdl: MDL) {
  mdl.modify.useWalkSequenceByWowName('Walk');
  // delete the 3rd sequence that the name "Attack"
  const atk3 = mdl.sequences.filter((s) => s.name === 'Attack')[2];
  mdl.sequences = mdl.sequences.filter((s) => s !== atk3);
}

function hideWeapon(mdl: MDL, attachmentId: WoWAttachmentID, sequenceNames: string[]) {
  const bone = mdl.wowAttachments.find((a) => a.wowAttachmentId === attachmentId)!.bone;
  const oldScaling: Vector3 = bone.scaling?.keyFrames.values().next().value ?? [1, 1, 1];

  bone.scaling = {
    interpolation: 'DontInterp',
    keyFrames: new Map(mdl.sequences.flatMap((seq) => {
      const scale: Vector3 = sequenceNames.some((name) => seq.name.includes(name)) ? [0, 0, 0] : [...oldScaling];
      return [[seq.interval[0], [...scale]], [seq.interval[1], [...scale]]];
    })),
    type: 'scaling',
  };
}

if (esMain(import.meta)) {
  void main().then(() => process.exit(0));
}

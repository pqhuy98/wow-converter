import esMain from 'es-main';

import { CharacterExporter, local, wowhead } from '@/lib/converter/character';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';

import { ceConfig } from './common';

const outputDir = './maps/demo.w3x/';
const ce = new CharacterExporter(ceConfig);

export async function bosses() {
  await ce.exportCharacter({
    base: local('creature\\ministerofdeath\\ministerofdeath'),
    inGameMovespeed: 270,
    size: 'hero',
  }, 'lady-deathwhisper');
}

export async function items() {
  await Promise.all([
    ['item\\objectcomponents\\weapon\\sword_1h_naxxramas_d_01', 'sword'],
    ['item\\objectcomponents\\head\\helm_plate_raiddeathknight_g_01_hum_helm_plate_raiddeathknight_g_01', 'helm'],
    ['item\\objectcomponents\\shoulder\\lshoulder_plate_raiddeathknight_g_01_shoulder_plate_raiddeathknight_g_01', 'shoulderL'],
    ['item\\objectcomponents\\shoulder\\rshoulder_plate_raiddeathknight_g_01', 'shoulderR'],
  ].map(([base, outputFile]) => ce.exportCharacter({ base: local(base), inGameMovespeed: 0 }, outputFile)));
}

export async function ghouls() {
  const models = await Promise.all([
    ce.exportCharacter({
      base: local('creature\\northrendgeist\\northrendgeist_green'),
      size: 'medium',
      inGameMovespeed: 270,
      keepCinematic: true,
    }, 'geist'),
    ce.exportCharacter({
      base: local('creature\\ghoul2\\ghoul2grey'),
      size: 'medium',
      inGameMovespeed: 270,
      keepCinematic: true,
    }, 'ghoul-1'),
    ce.exportCharacter({
      base: local('creature\\northrendghoul2\\northrendghoul2_grey'),
      size: 'medium',
      inGameMovespeed: 270,
      keepCinematic: true,
    }, 'ghoul-2'),
    ce.exportCharacter({
      base: local('creature\\northrendghoul2spiked\\northrendghoul2spiked_northrendghoul2_blue'),
      size: 'large',
      inGameMovespeed: 270,
      keepCinematic: true,
    }, 'ghoul-3'),
  ]);
  models.forEach((model) => {
    model.modify.useWalkSequenceByWowName('Walk');
    // model.modify.removeCinematicSequences();
    model.modify.addEventObjectBySequenceName('SNDXDGHO', 'Death', 0);
  });
}

export async function abominations() {
  const models = await Promise.all([
    ce.exportCharacter({ base: local('creature\\fleshgolem\\fleshgolemskin1'), size: 'large', inGameMovespeed: 270 }, 'abom-1'),
    ce.exportCharacter({ base: local('creature\\fleshgolem2\\fleshgolem2_original'), size: 'large', inGameMovespeed: 270 }, 'abom-2'),
  ]);
  models.forEach((model) => {
    model.modify.addEventObjectBySequenceName('SNDXDABO', 'Death', 0);
  });
}

export async function fleshGiants() {
  const models = await Promise.all([
    ce.exportCharacter({ base: local('creature\\northrendfleshgiant\\northrendfleshgiant01'), size: 'giant', inGameMovespeed: 270 }, 'flesh-giant'),
    ce.exportCharacter({ base: local('creature\\northrendfleshgiant\\northrendfleshgiant01frost'), size: 'giant', inGameMovespeed: 270 }, 'flesh-giant-frost'),
    ce.exportCharacter({
      base: local('creature\\icecrownfleshbeast\\icecrownfleshbeast_01'),
      size: 'giant',
      inGameMovespeed: 0,
    }, 'flesh-beast'),
  ]);
  models.forEach((model) => {
    model.modify.addEventObjectBySequenceName('SNDXDABO', 'Death', 0);
  });
}

export async function fleshGiantCorpse() {
  const model = await ce.exportCharacter({ base: local('creature\\northrendfleshgiant\\northrendfleshgiant01'), size: 'giant', inGameMovespeed: 0 }, 'flesh-giant-corpse');
  model.sequences = model.sequences.filter((s) => s.name === 'Decay Flesh');
  model.sequences[0].name = 'Stand';
}

export async function skeletons() {
  const mdl = await ce.exportCharacter({
    base: local('creature\\bonegolem\\bonegolemskin'), size: 'medium', inGameMovespeed: 220, scale: 0.6,
  }, 'bone-soldier');
  mdl.modify.addEventObjectBySequenceName('SNDXDSKE', 'Death', 0);
  // something wrong with this model, manually set camera position
  const camera = mdl.cameras[0];
  camera.position = [120, 0, 90];
  camera.target.position = [34, 0, 90];

  const skeleton = await ce.exportCharacter({
    base: local('creature\\skeletonnaked\\skeletonnakedskin_blue'),
    attackTag: 'Unarmed',
    size: 'medium',
    keepCinematic: true,
    inGameMovespeed: 0,
  }, 'skeleton');
  skeleton.modify.addEventObjectBySequenceName('SNDXDSKE', 'Death', 0);

  const skeletonMage = await ce.exportCharacter({ base: local('creature\\skeletonmage\\skeletonmage'), size: 'medium', inGameMovespeed: 270 }, 'skeleton-mage');
  skeletonMage.modify.addEventObjectBySequenceName('SNDXDSKE', 'Death', 0);

  const warrior = await ce.exportCharacter({
    base: local('creature\\scourgewarrior\\scourgewarrior_classic101,202,401'),
    size: 'large',
    inGameMovespeed: 270,
    attackTag: '2H',
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: local('item\\objectcomponents\\weapon\\sword_2h_northrend_b_01copper'), scale: 1 },
    },
  }, 'skeleton-warrior');
  warrior.modify.addEventObjectBySequenceName('SNDXDSKE', 'Death', 0);
}

export async function cryptFiends() {
  const cryptfiend = await ce.exportCharacter({ base: local('creature\\cryptfiend\\cryptfiendskin'), size: 'large', inGameMovespeed: 270 }, 'cryptfiend');
  cryptfiend.modify.addEventObjectBySequenceName('SNDxDPIT', 'Death', 0);
  const cryptfiend2 = await ce.exportCharacter({ base: local('creature\\nerubianwarrior\\nerubianwarrior_undead'), size: 'large', inGameMovespeed: 270 }, 'cryptfiend-2');
  cryptfiend2.modify.addEventObjectBySequenceName('SNDxDPIT', 'Death', 0);
}

export async function zombies() {
  const zombieDog = await ce.exportCharacter({ base: local('creature\\undeadbeast\\undeadbeast'), size: 'large', inGameMovespeed: 270 }, 'zombie-dog');
  zombieDog.modify.addEventObjectBySequenceName('SNDXDABO', 'Death', 0);
  const superZombie = await ce.exportCharacter({ base: local('creature\\superzombie\\superzombiegreen'), size: 'large', inGameMovespeed: 270 }, 'super-zombie');
  superZombie.modify.addEventObjectBySequenceName('SNDXDABO', 'Death', 0);
}

export async function doodads() {
  await ce.exportCharacter({
    base: local('world\\expansion02\\doodads\\icecrown\\elevator\\icecrown_elevator02'), scale: 1, inGameMovespeed: 0,
  }, 'icecrown-elevator');
  await ce.exportCharacter({
    base: local('world\\expansion02\\doodads\\icecrown\\doors\\icecrown_door_04'), scale: 1, inGameMovespeed: 0,
  }, 'icecrown_door_04');
  await ce.exportCharacter({
    base: local('world\\expansion02\\doodads\\icecrown\\icewall\\icecrown_icewall'),
    scale: 1,
    noDecay: true,
    inGameMovespeed: 0,
  }, 'icecrown_icewall');
  await ce.exportCharacter({
    base: local('world\\expansion02\\doodads\\generic\\scourge\\sc_teleportpad2'),
    scale: 1,
    noDecay: true,
    inGameMovespeed: 0,
  }, 'scourge_teleporter');
  await ce.exportCharacter({
    base: local('world\\expansion02\\doodads\\generic\\scourge\\sc_teleportpad3'),
    scale: 1,
    noDecay: true,
    inGameMovespeed: 0,
  }, 'scourge_teleporter2');
}

export async function spells() {
  await ce.exportCharacter({
    base: local('spells\\boneguardspike'), scale: 0.25, inGameMovespeed: 0,
  }, 'bonespike');
}

export async function vrykul() {
  const zombieVrykul = await ce.exportCharacter({
    base: local('creature\\zombiefiedvrykul\\zombiefiedvrykul1pale'),
    size: 'large',
    inGameMovespeed: 270,
    keepCinematic: true,
    attackTag: 'Unarmed',
  }, 'vrykul-zombie');
  zombieVrykul.modify.useWalkSequenceByWowName('Walk');
  zombieVrykul.modify.removeCinematicSequences();

  const vrykulWarrior = await ce.exportCharacter({
    base: local('creature\\frostvrykulmale\\frostvrykulmaleskin'),
    size: 'hero',
    inGameMovespeed: 270,
    attackTag: '1H',
    attachItems: {
      [WoWAttachmentID.HandRight]: { path: local('item\\objectcomponents\\weapon\\axe_2h_northrend_c_03red'), scale: 1 },
      [WoWAttachmentID.HandLeft]: { path: local('item\\objectcomponents\\weapon\\axe_2h_northrend_c_03red'), scale: 1 },
    },
  }, 'vrykul-warrior');
  vrykulWarrior.modify
    .removeWowSequence('Attack1HPierce')
    .removeWowSequence('AttackOffPierce')
    .removeWowSequence('AttackOff', 0);
}

export async function frostWyrm() {
  const mdl = await ce.exportCharacter({
    base: wowhead('https://www.wowhead.com/npc=111640/frost-wyrm'),
    inGameMovespeed: 270,
    size: 'giant',
  }, 'frost-wyrm');
  mdl.modify.removeUnusedNodes();
}

export async function main() {
  await ghouls();
  await abominations();
  await fleshGiants();
  await skeletons();
  await cryptFiends();
  await vrykul();
  await items();
  await bosses();
  await doodads();
  await spells();
  await zombies();
  await fleshGiantCorpse();
  await frostWyrm();

  ce.models.forEach(([model]) => model.modify.optimizeAll());
  ce.writeAllModels(outputDir, 'mdx');
  await ce.writeAllTextures(outputDir);
}

if (esMain(import.meta)) {
  void main().then(() => process.exit(0));
}

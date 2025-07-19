import { writeFileSync } from 'fs';

import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';

import { ce } from './common';
import { local } from '@/lib/converter/character';

export function bosses() {
  ce.exportCharacter({
    base: local('creature\\ministerofdeath\\ministerofdeath'),
    inGameMovespeed: 270,
    size: 'hero',
  }, 'lady-deathwhisper');
}

export function items() {
  [
    ['item\\objectcomponents\\weapon\\sword_1h_naxxramas_d_01', 'sword'],
    ['item\\objectcomponents\\head\\helm_plate_raiddeathknight_g_01_hum_helm_plate_raiddeathknight_g_01', 'helm'],
    ['item\\objectcomponents\\shoulder\\lshoulder_plate_raiddeathknight_g_01_shoulder_plate_raiddeathknight_g_01', 'shoulderL'],
    ['item\\objectcomponents\\shoulder\\rshoulder_plate_raiddeathknight_g_01', 'shoulderR'],
  ].forEach(([base, outputFile]) => ce.exportCharacter({ base: local(base), inGameMovespeed: 0 }, outputFile));
}

export async function ghouls() {
  const models = await Promise.all([
    ce.exportCharacter({ base: local('creature\\northrendgeist\\northrendgeist_green'), size: 'medium', inGameMovespeed: 270 }, 'geist'),
    ce.exportCharacter({ base: local('creature\\ghoul2\\ghoul2grey'), size: 'medium', inGameMovespeed: 270 }, 'ghoul-1'),
    ce.exportCharacter({ base: local('creature\\northrendghoul2\\northrendghoul2_grey'), size: 'medium', inGameMovespeed: 270 }, 'ghoul-2'),
    ce.exportCharacter({ base: local('creature\\northrendghoul2spiked\\northrendghoul2spiked_northrendghoul2_blue'), size: 'large', inGameMovespeed: 270 }, 'ghoul-3'),
  ]);
  models.forEach((model) => {
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
    ce.exportCharacter({ base: local('creature\\icecrownfleshbeast\\icecrownfleshbeast_01'), size: 'giant', inGameMovespeed: 270 }, 'flesh-beast'),
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
    base: local('creature\\skeletonnaked\\skeletonnakedskin_blue'), attackTag: 'Unarmed', size: 'medium', inGameMovespeed: 270,
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
    attackTag: 'Unarmed',
  }, 'vrykul-zombie');
  zombieVrykul.modify.useWalkSequenceByWowName('Walk');

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

export function main() {
  ghouls()
  abominations()
  fleshGiants();
  skeletons();
  cryptFiends();
  vrykul()
  items()
  bosses();
  doodads()
  spells()
  zombies();
  fleshGiantCorpse();

  ce.assetManager.exportTextures(ce.outputPath);
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
}

if (require.main === module) {
  main();
}

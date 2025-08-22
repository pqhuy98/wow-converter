import path from 'path';

import { ExportCharacterParams, wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { fetchNpcMeta } from '@/lib/wowhead-client/npc';
import { NpcZamUrl, ZamUrl } from '@/lib/wowhead-client/zam-url';
import { MDL } from '@/lib/objmdl/mdl/mdl';
import { ANIM_NAMES, AttackTag, getWc3AnimName, getWowAnimName } from '@/lib/objmdl/animation/animation_mapper';
import { EquipmentSlot, ItemMetata, processItemData } from './item-model';
import { exportModelFileIdAsMdl, ExportContext, exportTexture } from './utils';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import chalk from 'chalk';

export async function exportCharacterNpcAsMdl({
  ctx,
  zam,
  keepCinematic,
  attackTag,
}: {
  ctx: ExportContext;
  zam: NpcZamUrl;
  keepCinematic: boolean
  attackTag: AttackTag | undefined
}): Promise<MDL> {
  const prep = await prepareCharacterNpcExport(zam);
  const result = await wowExportClient.exportCharacter({
    ...prep.rpcParams,
    excludeAnimationIds: getExcludedAnimIds(keepCinematic, attackTag),
  });
  const baseDir = await wowExportClient.getAssetDir();
  const relative = path.relative(baseDir, result.exportPath);
  const charMdl = ctx.assetManager.parse(relative, true).mdl;

  // NPC main texture file
  if (prep.npcTextureFile) {
    const npcTexturePath = await exportTexture(prep.npcTextureFile);
    ctx.assetManager.addPngTexture(npcTexturePath);
    const baseTexturePath = charMdl.geosets[0].material.layers[0].texture.image;
    const newTexturePath = path.join(ctx.config.assetPrefix, npcTexturePath).replace('.png', '.blp');

    charMdl.geosets.forEach((geoset, i) => {
      // For skeleton, we don't want to override anything but
      // For some reason, the skull is not included in the npc baked texture
      let skip = false;
      const raceSkeleton = 20;
      if (prep.rpcParams.race === raceSkeleton) {
        skip = true;
        let geosetIds: number[] | undefined;
        if (geoset.name.includes('Glove') && charMdl.geosets[i - 1].name.includes('Glove')) {
          geosetIds = prep.equipmentSlots.find((s) => s.slotId === EquipmentSlot.Gloves.toString())?.data.geosetIds;
        } else if (geoset.name.includes('Boot') && charMdl.geosets[i + 1].name.includes('Boot')) {
          geosetIds = prep.equipmentSlots.find((s) => s.slotId === EquipmentSlot.Boots.toString())?.data.geosetIds;
        } else if (geoset.name.includes('Trousers')) {
          geosetIds = prep.equipmentSlots.find((s) => s.slotId === EquipmentSlot.Legs.toString())?.data.geosetIds;
        }
        if (geosetIds && geosetIds.length > 0) {
          skip = false;
        }
      }
      if (skip) return;

      // Replace the base texture with the npc baked texture
      geoset.material.layers.forEach((layer) => {
        if (layer.texture.image === baseTexturePath || layer.texture.image === '') {
          layer.texture.image = newTexturePath;
        }
      });
    });
  }

  // Attach items
  const attachItem = async (fileDataId: number, textures: number[], attachmentId: WoWAttachmentID) => {
    const attachmentBone = charMdl.wowAttachments.find((a) => a.wowAttachmentId === attachmentId);
    if (!attachmentBone) {
      console.error(chalk.red(`Cannot find bone for wow attachment ${attachmentId}`));
      if (charMdl.wowAttachments.length === 0) {
        console.error(chalk.red(`No WoW attachments data found in this model "${zam.displayId}"`));
      }
      return;
    }
    const itemMdl = await exportModelFileIdAsMdl(ctx, fileDataId, textures);
    charMdl.modify.addMdlItemToBone(itemMdl, attachmentBone.bone.name);
  }

  const helmetSlot = prep.equipmentSlots.find((s) => s.slotId === EquipmentSlot.Head.toString());
  if (helmetSlot) {
    await attachItem(helmetSlot.data.modelFiles[0], helmetSlot.data.textureFiles, WoWAttachmentID.Helm);
  }

  // Attach shoulders
  const shoulderSlot = prep.equipmentSlots.find((s) => s.slotId === EquipmentSlot.Shoulder.toString());
  if (shoulderSlot) {
    await attachItem(shoulderSlot.data.modelFiles[0], shoulderSlot.data.textureFiles, WoWAttachmentID.ShoulderLeft);
    await attachItem(shoulderSlot.data.modelFiles[1], shoulderSlot.data.textureFiles, WoWAttachmentID.ShoulderRight);
  }

  // Cloak etc additional textures
  const textureSlotConfigs: Partial<Record<EquipmentSlot, { geosetNames: string[]; twoSided?: boolean }>> = {
    [EquipmentSlot.Back]: { geosetNames: ['Cloak'], twoSided: true },
  };

  for (const slot of prep.equipmentSlots) {
    const slotEnum = parseInt(slot.slotId, 10) as EquipmentSlot;
    const cfg = textureSlotConfigs[slotEnum];
    if (!cfg) continue;
    const matching = charMdl.geosets.filter((g) => cfg.geosetNames.some((name) => g.name.includes(name)));
    if (matching.length === 0) continue;

    const texPath = await exportTexture(slot.data.textureFiles[0]);
    ctx.assetManager.addPngTexture(texPath);

    charMdl.textures.push({
      id: charMdl.textures.length,
      image: path.join(ctx.config.assetPrefix, texPath).replace('.png', '.blp'),
      wrapWidth: false,
      wrapHeight: false,
    });
    charMdl.materials.push({
      id: charMdl.materials.length,
      constantColor: false,
      twoSided: cfg.twoSided ?? false,
      layers: [
        {
          filterMode: 'Transparent',
          texture: charMdl.textures.at(-1)!,
          twoSided: cfg.twoSided ?? false,
          unshaded: false,
          sphereEnvMap: false,
          unfogged: false,
          unlit: false,
          noDepthTest: false,
          noDepthSet: false,
          alpha: {
            static: true,
            value: 1,
          },
        },
      ],
    });
    matching.forEach((g) => { g.material = charMdl.materials.at(-1)!; });
  }

  return charMdl;
}

async function prepareCharacterNpcExport(zam: NpcZamUrl): Promise<{
  rpcParams: ExportCharacterParams;
  npcTextureFile: number | null;
  equipmentSlots: {slotId: string, data: ItemMetata}[];
}> {
  const metadata = await fetchNpcMeta(zam);
  if (!metadata.Character) {
    throw new Error('prepareCharacterNpcExport: NPC has no character metadata');
  }
  const character = metadata.Character;
  const race = character.Race;
  const gender = character.Gender;
  const chrModelId = character.ChrModelId;
  
  // Decide which geosets to include/hide based on equipment
  const geosetIds = new Set<number>();
  const hideGeosetIds = new Set<number>();
  const equipmentSlots: {slotId: string, data: ItemMetata}[] = [];

  const slotIds = Object.values(EquipmentSlot).filter((v) => typeof v === 'number') as number[];
  for (const slotId of slotIds) {
    const itemId = metadata.Equipment?.[slotId.toString()];
    if (!itemId) continue;
    const itemData = await processItemData(slotId, itemId, race, gender, zam);
    if ([
      EquipmentSlot.Head,
      EquipmentSlot.Shoulder,
      EquipmentSlot.Legs,
      EquipmentSlot.Boots,
      EquipmentSlot.Chest,
      EquipmentSlot.Gloves,
      EquipmentSlot.Belt,
      EquipmentSlot.Back,
      EquipmentSlot.Tabard,
    ].includes(slotId)) {
      itemData.geosetIds?.forEach((id: number) => geosetIds.add(id));
      itemData.hideGeosetIds?.forEach((id: number) => hideGeosetIds.add(id));
      equipmentSlots.push({ slotId: slotId.toString(), data: itemData });
    }
  }

  // Prepare RPC params for wowexport
  const rpcParams: ExportCharacterParams = {
    race,
    gender,
    chrModelId,
    customizations: Object.fromEntries((metadata.Creature?.CreatureCustomizations || []).map((c) => [c.optionId, c.choiceId])),
    format: 'obj',
    include_animations: true,
    include_base_clothing: false,
    geosetIds: Array.from(geosetIds),
    hideGeosetIds: Array.from(hideGeosetIds),
  };

  const npcTextureFile = metadata.TextureFiles ? Object.values(metadata.TextureFiles)[0]?.[0]?.FileDataId : null;

  return { rpcParams, npcTextureFile, equipmentSlots };
}

function getExcludedAnimIds(keepCinematic: boolean, attackTag: AttackTag | undefined) : number[] {
  const excludedAnimIds: number[] = [];
  // Iterate through a reasonable range of animation IDs to detect cinematic ones
  for (let animId = 0; animId < ANIM_NAMES.length; animId++) {
    const wc3Anim = getWc3AnimName(getWowAnimName(animId));

    if (!keepCinematic && wc3Anim.wc3Name.includes('Cinematic')) {
      excludedAnimIds.push(animId);
      continue;
    }
    if (attackTag && wc3Anim.attackTag !== '' && wc3Anim.attackTag !== attackTag) {
      excludedAnimIds.push(animId);
      continue;
    }
  }
  return excludedAnimIds;
}
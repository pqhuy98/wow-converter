import chalk from 'chalk';
import _ from 'lodash';
import path from 'path';

import { MDL } from '@/lib/formats/mdl/mdl';
import { canAddMdlCollectionItemToModel } from '@/lib/formats/mdl/modify/add-item-to-model';
import {
  ANIM_NAMES, AttackTag, getWc3AnimName, getWowAnimName,
} from '@/lib/objmdl/animation/animation_mapper';
import { getWoWAttachmentName, WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { ExportCharacterParams, wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { fetchNpcMeta } from '@/lib/wowhead-client/npc';
import { NpcZamUrl } from '@/lib/wowhead-client/zam-url';

import {
  applyReplaceableTextrures, ExportContext, exportModelFileIdAsMdl, exportTexture,
} from '../utils';
import {
  EquipmentSlot, filterCollectionGeosets,
  getEquipmentSlotName, getSubmeshName, ItemMetata, processItemData,
} from './item-model';

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
  // Export the base model
  const prep = await prepareCharacterNpcExport(zam);
  const start = performance.now();
  const result = await wowExportClient.exportCharacter({
    ...prep.rpcParams,
    excludeAnimationIds: getExcludedAnimIds(keepCinematic, attackTag),
  });
  console.log('wow.export exportCharacter took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  const baseDir = await wowExportClient.getAssetDir();
  const relative = path.relative(baseDir, result.exportPath);
  const charMdl = ctx.assetManager.parse(relative, true).mdl;

  // Replace the base texture with the npc baked texture
  await applyNpcBaseTextrure(ctx, charMdl, prep);

  // Attach items with models
  await attachEquipmentsWithModel(ctx, charMdl, prep.equipmentSlots);

  // Cloak etc additional textures
  await attachEquipmentsWithTexturesOnly(ctx, charMdl, prep.equipmentSlots);

  // Hacky fixes to be removed later
  // EyeGlow make them a bit transparent with geoset anims
  charMdl.geosets.forEach((geoset) => {
    if (geoset.name.startsWith('Eyeglow')) {
      charMdl.geosetAnims.push({
        id: -1,
        geoset,
        alpha: {
          static: true,
          value: 0.3,
        },
      });
    }
  });

  // If the item has trousers, remove the tabard geoset
  if (charMdl.geosets.some((g) => g.name.startsWith('Trousers') && g.name !== 'Trousers1')) {
    charMdl.geosets = charMdl.geosets.filter((g) => !g.name.startsWith('Tabard'));
  }

  // Apply replaceable textures
  await applyReplaceableTextrures(ctx, charMdl, prep.replaceableTextures);

  return charMdl;
}

type EquipmentSlotData = {
  slotId: EquipmentSlot;
  data: ItemMetata;
}

type Prep ={
  rpcParams: ExportCharacterParams;
  npcTextureFile: number | null;
  equipmentSlots: EquipmentSlotData[];
  replaceableTextures: Record<string, number>;
}

async function prepareCharacterNpcExport(zam: NpcZamUrl): Promise<Prep> {
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
  const equipmentSlots: {slotId: EquipmentSlot, data: ItemMetata}[] = [];

  const slotIds = Object.values(EquipmentSlot).filter((v) => typeof v === 'number') as number[];
  for (const slotId of slotIds) {
    const itemId = metadata.Equipment?.[slotId.toString()];
    if (!itemId) continue;
    const itemData = await processItemData({
      expansion: zam.expansion, type: 'item', displayId: itemId, slotId,
    }, race, gender);
    itemData.geosetIds?.forEach((id: number) => geosetIds.add(id));
    itemData.hideGeosetIds?.forEach((id: number) => hideGeosetIds.add(id));
    equipmentSlots.push({ slotId, data: itemData });
  }

  console.log('Equipments:', equipmentSlots.map((s) => getEquipmentSlotName(s.slotId)));

  if (!slotIds.includes(EquipmentSlot.Head)) {
    // otherwise the model will be missing ears
    geosetIds.add(701);
  } else {
    geosetIds.add(702);
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

  return {
    rpcParams,
    npcTextureFile,
    equipmentSlots,
    replaceableTextures: metadata.Textures || {},
  };
}

async function attachEquipmentsWithModel(ctx: ExportContext, charMdl: MDL, equipmentSlots: EquipmentSlotData[]) {
  const collections = new Map<number, MDL>();
  const attachmentResults: {
    attachmentId: WoWAttachmentID | undefined,
    itemMdl: MDL,
    ok: boolean,
    fileDataId: number,
  }[] = [];

  // Attach individual item model to the character model
  const attachItemModel = async (fileDataId: number, itemData: ItemMetata, attachmentId: WoWAttachmentID | undefined) => {
    const itemMdl = !collections.has(fileDataId)
      ? await exportModelFileIdAsMdl(ctx, fileDataId, { textureIds: itemData.textureFiles })
      : _.cloneDeep(collections.get(fileDataId)!);

    const isCollection = collections.has(fileDataId) || canAddMdlCollectionItemToModel(charMdl, itemMdl);

    if (isCollection) {
      if (!collections.has(fileDataId)) {
        collections.set(fileDataId, _.cloneDeep(itemMdl));
      }

      const debug = true;
      debug && console.log('itemData.slotId', itemData.slotId, itemData.slotId ? getEquipmentSlotName(itemData.slotId) : 'null');
      const enabledGeosets = filterCollectionGeosets(itemData.slotId!, itemData.originalData, itemMdl);

      debug && console.log('all available geosets', fileDataId, itemMdl.geosets.map((g) => `${getSubmeshName(g.wowData.submeshId)} (${g.wowData.submeshId})`));

      itemMdl.geosets = enabledGeosets;
      debug && console.log('chosen geosets', itemMdl.geosets.map((g) => g.name));

      charMdl.modify.addMdlCollectionItemToModel(itemMdl);
      attachmentResults.push({
        attachmentId, itemMdl, ok: true, fileDataId,
      });
      return;
    }

    // not a collection, add to bone

    if (!attachmentId) {
      console.error(chalk.red(`Cannot add item ${fileDataId} to model as bone because no attachment id is provided.`));
      return;
    }

    const attachment = charMdl.wowAttachments.find((a) => a.wowAttachmentId === attachmentId);
    if (!attachment) {
      console.error(chalk.red(`Cannot find bone for wow attachment ${attachmentId} (${getWoWAttachmentName(attachmentId)})`));
      if (charMdl.wowAttachments.length === 0) {
        console.error(chalk.red(`No WoW attachments data found in this model ${charMdl.model.name}`));
      }
      attachmentResults.push({
        attachmentId, itemMdl, ok: false, fileDataId,
      });
      return;
    }
    charMdl.modify.addMdlItemToBone(itemMdl, attachment.bone);
    attachmentResults.push({
      attachmentId, itemMdl, ok: true, fileDataId,
    });
  };

  const attachmentList: [EquipmentSlot, number, WoWAttachmentID | undefined][] = [
    [EquipmentSlot.Head, 0, WoWAttachmentID.Helm],
    [EquipmentSlot.Shoulder, 0, WoWAttachmentID.ShoulderLeft],
    [EquipmentSlot.Shoulder, 1, WoWAttachmentID.ShoulderRight],
    [EquipmentSlot.Belt, 0, WoWAttachmentID.BeltBuckle],
    [EquipmentSlot.Back, 0, WoWAttachmentID.Backpack],
    [EquipmentSlot.Chest, 0, undefined],
    [EquipmentSlot.Legs, 0, undefined],
    [EquipmentSlot.Feet, 0, undefined],
    [EquipmentSlot.Gloves, 0, undefined],
  ];
  for (const [slotId, index, attachmentId] of attachmentList) {
    const slot = equipmentSlots.find((s) => s.slotId === slotId);
    if (slot && slot.data.modelFiles[index]) {
      await attachItemModel(slot.data.modelFiles[index], slot.data, attachmentId);
    }
  }

  // print results
  attachmentResults.forEach((r) => {
    const attachmentName = r.attachmentId ? getWoWAttachmentName(r.attachmentId) : 'collection';
    const itemName = path.basename(r.itemMdl.model.name);
    if (r.ok) {
      console.log(`Attach ${attachmentName} -> ${itemName} (${r.fileDataId})`);
    } else {
      console.error(chalk.red(`Failed to attach ${attachmentName} -> ${itemName}`));
    }
  });
}

async function attachEquipmentsWithTexturesOnly(ctx: ExportContext, charMdl: MDL, slots: EquipmentSlotData[]) {
  const textureSlotConfigs: Partial<Record<EquipmentSlot, { geosetNames: string[]; twoSided?: boolean }>> = {
    [EquipmentSlot.Back]: { geosetNames: ['Cloak'], twoSided: true },
  };
  for (const slot of slots) {
    const cfg = textureSlotConfigs[slot.slotId];
    if (!cfg) continue;
    const matching = charMdl.geosets.filter((g) => cfg.geosetNames.some((name) => g.name.includes(name)));
    if (matching.length === 0) continue;

    const textureFile = slot.data.textureFiles[0];

    const texPath = await exportTexture(textureFile);
    ctx.assetManager.addPngTexture(texPath);
    charMdl.textures.push({
      id: charMdl.textures.length,
      image: path.join(ctx.config.assetPrefix, texPath).replace('.png', '.blp'),
      wrapWidth: false,
      wrapHeight: false,
      wowData: {
        type: 0,
      },
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
    console.log('Set texture', getEquipmentSlotName(slot.slotId), '->', path.basename(texPath).replace('.png', ''));
  }
}

const debug = false;

async function applyNpcBaseTextrure(ctx: ExportContext, charMdl: MDL, prep: Prep) {
  debug && console.log('applyNpcBaseTextrure', prep.npcTextureFile);
  if (prep.npcTextureFile) {
    const npcTexturePath = await exportTexture(prep.npcTextureFile);
    ctx.assetManager.addPngTexture(npcTexturePath);

    // base texture is the one of the geoset with highest face count. Hacky heuristic
    // TODO: find replaceable textures
    const baseTexturePath = charMdl.textures.find((t) => t.wowData.type === 1)?.image;
    debug && console.log('baseTexturePath', baseTexturePath);

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
          geosetIds = prep.equipmentSlots.find((s) => s.slotId === EquipmentSlot.Gloves)?.data.geosetIds;
        } else if (geoset.name.includes('Boot') && charMdl.geosets[i + 1].name.includes('Boot')) {
          geosetIds = prep.equipmentSlots.find((s) => s.slotId === EquipmentSlot.Feet)?.data.geosetIds;
        } else if (geoset.name.includes('Trousers')) {
          geosetIds = prep.equipmentSlots.find((s) => s.slotId === EquipmentSlot.Legs)?.data.geosetIds;
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
}

function getExcludedAnimIds(keepCinematic: boolean, attackTag: AttackTag | undefined): number[] {
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

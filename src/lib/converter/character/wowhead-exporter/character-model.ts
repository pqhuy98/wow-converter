import chalk from 'chalk';
import { createHash } from 'crypto';
import { writeFileSync } from 'fs';
import _ from 'lodash';
import path from 'path';

import { MDL } from '@/lib/formats/mdl/mdl';
import { canAddMdlCollectionItemToModel } from '@/lib/formats/mdl/modify/add-item-to-model';
import { drawPngsOnBasePng, PngDraw } from '@/lib/formats/png';
import {
  ANIM_NAMES, AttackTag, getWc3AnimName, getWowAnimName,
} from '@/lib/objmdl/animation/animation_mapper';
import { getWoWAttachmentName, WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { ExportCharacterParams, wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { fetchCharacterCustomization } from '@/lib/wowhead-client/character-customization';
import { EquipmentSlot } from '@/lib/wowhead-client/item-armor';
import { CharacterData } from '@/lib/wowhead-client/npc-object';
import { ZamExpansion } from '@/lib/wowhead-client/zam-url';

import { Model } from '../../common/models';
import { InventoryType } from '../item-mapper';
import {
  applyReplaceableTextures, ExportContext, exportModelFileIdAsMdl, exportTexture,
} from '../utils';
import {
  EquipmentSlotData,
  filterCollectionGeosets,
  getEquipmentSlotName, getGeosetIdsFromEquipments,
  getSubmeshName, processItemData,
} from './item-model';

export async function exportCharacterAsMdl({
  ctx,
  metaData,
  expansion,
  keepCinematic,
  attackTag,
}: {
  ctx: ExportContext;
  metaData: CharacterData;
  expansion: ZamExpansion
  keepCinematic: boolean
  attackTag: AttackTag | undefined
}): Promise<MDL> {
  // Export the base model
  const prep = await prepareCharacterExport(metaData, expansion);
  const start = performance.now();
  !ctx.config.isBulkExport && console.log('wow.export character - race:', prep.rpcParams.race, 'gender:', prep.rpcParams.gender);
  const result = await wowExportClient.exportCharacter({
    ...prep.rpcParams,
    excludeAnimationIds: getExcludedAnimIds(keepCinematic, attackTag),
  });
  console.log('wow.export character took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');

  const baseDir = await wowExportClient.getAssetDir();
  const relative = path.relative(baseDir, result.exportPath);
  const charMdl = ctx.assetManager.parse(relative, true).mdl;

  // Replace the base texture with the prebaked texture
  await applyPrebakedTextrure(ctx, charMdl, prep);
  await applyEquipmentsBodyTextures(ctx, charMdl, prep, expansion);
  await applyCloakTexture(ctx, charMdl, prep.equipmentSlots);

  // Attach items with models
  await attachEquipmentsWithModel(ctx, charMdl, prep.equipmentSlots);

  // If the item has trousers, remove the tabard geoset
  if (charMdl.geosets.some((g) => g.name.startsWith('Trousers') && g.name !== 'Trousers1')) {
    charMdl.geosets = charMdl.geosets.filter((g) => !g.name.startsWith('Tabard'));
  }

  // Apply replaceable textures
  await applyReplaceableTextures(ctx, charMdl, prep.replaceableTextures);

  return charMdl;
}

type Prep = {
  rpcParams: ExportCharacterParams;
  prebakedTexture: number | null;
  equipmentSlots: EquipmentSlotData[];
  replaceableTextures: Record<string, number>;
  chrModelId: number;
}

async function prepareCharacterExport(metadata: CharacterData, expansion: ZamExpansion): Promise<Prep> {
  if (!metadata.Character) {
    throw new Error('prepareCharacterExport: URL has no character metadata');
  }
  const character = metadata.Character;
  const race = character.Race;
  const gender = character.Gender;

  // Decide which geosets to include/hide based on equipment
  const equipmentSlots: EquipmentSlotData[] = [];

  const slotIds = Object.values(EquipmentSlot).filter((v) => typeof v === 'number') as number[];
  for (const slotId of slotIds) {
    const itemId = metadata.Equipment?.[slotId.toString()];
    if (!itemId) continue;
    try {
      const itemData = await processItemData({
        expansion, type: 'item', displayId: itemId, slotId,
      }, race, gender);
      equipmentSlots.push({ slotId, data: itemData });
    } catch (e) {
      console.error(chalk.red(`Failed to process item ${itemId} for slot ${slotId}: ${e}`));
      continue;
    }
  }

  console.log('Equipments:', equipmentSlots.map((s) => getEquipmentSlotName(s.slotId)));

  const { geosetIds, hideGeosetIds } = getGeosetIdsFromEquipments(equipmentSlots);

  // Prepare RPC params for wowexport
  const rpcParams: ExportCharacterParams = {
    race,
    gender,
    customizations: Object.fromEntries((metadata.Creature?.CreatureCustomizations || []).map((c) => [c.optionId, c.choiceId])),
    format: 'obj',
    include_animations: true,
    include_base_clothing: true,
    geosetIds,
    hideGeosetIds,
  };

  const prebakedTexture = metadata.TextureFiles ? Object.values(metadata.TextureFiles)[0]?.[0]?.FileDataId : null;

  return {
    rpcParams,
    prebakedTexture,
    equipmentSlots,
    replaceableTextures: metadata.Textures || {},
    chrModelId: character.ChrModelId,
  };
}

async function attachEquipmentsWithModel(ctx: ExportContext, charMdl: MDL, equipmentSlots: EquipmentSlotData[]) {
  const collections = new Map<number, Model>();
  const attachmentResults: {
    attachmentId: WoWAttachmentID | undefined,
    itemMdl: MDL,
    ok: boolean,
    fileDataId: number,
  }[] = [];

  const debug = false;

  // Attach individual item model to the character model
  const attachItemModel = async (slotData: EquipmentSlotData, idx: number, attachmentId: WoWAttachmentID | undefined) => {
    debug && console.log('attachItemModel', attachmentId != null ? getWoWAttachmentName(attachmentId) : 'undefined', idx);

    const itemData = slotData.data;

    const fileDataId = itemData.modelFiles[idx].fileDataId;
    const itemReplaceableTextures = Object.fromEntries(itemData.modelTextureFiles[idx].map((f) => [f.componentId, f.fileDataId]));
    debug && console.log(fileDataId, 'itemReplaceableTextures', itemReplaceableTextures);

    const itemModel = !collections.has(fileDataId)
      ? await exportModelFileIdAsMdl(ctx, fileDataId, {})
      : _.cloneDeep(collections.get(fileDataId)!);
    const itemMdl = itemModel.mdl;

    await applyReplaceableTextures(ctx, itemMdl, itemReplaceableTextures);

    const isCollection = collections.has(fileDataId) || canAddMdlCollectionItemToModel(charMdl, itemMdl);

    if (isCollection) {
      if (!collections.has(fileDataId)) {
        collections.set(fileDataId, _.cloneDeep(itemModel));
      }

      const debug = true;
      debug && console.log('itemData.slotId', itemData.slotId, itemData.slotId ? getEquipmentSlotName(itemData.slotId) : 'null');
      const enabledGeosets = filterCollectionGeosets(equipmentSlots, slotData, itemMdl);

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

    if (attachmentId == null) {
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

    if (attachmentId === WoWAttachmentID.HandLeft) {
      const shouldFlipY = itemData.flags & 256;
      if (shouldFlipY) {
        itemMdl.modify.flipY();
      }
    }

    charMdl.modify.addMdlItemToBone(itemMdl, attachment.bone);
    attachmentResults.push({
      attachmentId, itemMdl, ok: true, fileDataId,
    });
  };

  const attachmentList: Record<EquipmentSlot, WoWAttachmentID[]> = {
    [EquipmentSlot.Head]: [WoWAttachmentID.Helm],
    [EquipmentSlot.Shoulder]: [WoWAttachmentID.ShoulderLeft, WoWAttachmentID.ShoulderRight],
    [EquipmentSlot.Waist]: [WoWAttachmentID.BeltBuckle],
    [EquipmentSlot.Cloak]: [WoWAttachmentID.Backpack],
    [EquipmentSlot.Chest]: [],
    [EquipmentSlot.Legs]: [],
    [EquipmentSlot.Feet]: [],
    [EquipmentSlot.Hands]: [],
    [EquipmentSlot.MainHand]: [WoWAttachmentID.HandRight],
    [EquipmentSlot.OffHand]: [WoWAttachmentID.HandLeft],
    [EquipmentSlot.Shirt]: [],
    [EquipmentSlot.Tabard]: [],
    [EquipmentSlot.Wrist]: [],
    [EquipmentSlot.Robe]: [],
  };

  const isWeapon = (slot: EquipmentSlotData) => [
    InventoryType.WEAPON,
    InventoryType.SHIELD,
    InventoryType.RANGED,
    InventoryType.RANGEDRIGHT,
    InventoryType.TWO_HANDED_WEAPON,
    InventoryType.WEAPONMAINHAND,
    InventoryType.WEAPONOFFHAND,
    InventoryType.HOLDABLE,
    InventoryType.THROWN,
    InventoryType.RELIC,
  ].includes(slot.data.inventoryType);

  for (const [slotId, attachmentIds] of Object.entries(attachmentList)) {
    const slot = equipmentSlots.find((s) => s.slotId === Number(slotId));
    if (slot) {
      if (isWeapon(slot)) {
        if (Number(slotId) === EquipmentSlot.MainHand) ctx.weaponInventoryTypes[0] ??= slot.data.inventoryType;
        if (Number(slotId) === EquipmentSlot.OffHand) ctx.weaponInventoryTypes[1] ??= slot.data.inventoryType;
      }
      for (let i = 0; i < slot.data.modelFiles.length; i++) {
        let attachmentId = attachmentIds[i] ?? attachmentIds[0] ?? undefined;
        if (Number(slotId) === EquipmentSlot.OffHand && slot.data.inventoryType === InventoryType.SHIELD) {
          attachmentId = WoWAttachmentID.Shield;
        }
        if (Number(slotId) === EquipmentSlot.MainHand && slot.data.inventoryType === InventoryType.RANGED) {
          attachmentId = WoWAttachmentID.HandLeft;
        }
        await attachItemModel(slot, i, attachmentId);
      }
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

async function applyCloakTexture(ctx: ExportContext, charMdl: MDL, slots: EquipmentSlotData[]) {
  const cloakSlot = slots.find((s) => s.slotId === EquipmentSlot.Cloak);
  if (!cloakSlot || cloakSlot.data.bodyTextureFiles.length === 0) return;

  const textureFile = cloakSlot.data.bodyTextureFiles[0];
  const texPath = await exportTexture(textureFile.fileDataId);
  ctx.assetManager.addPngTexture(texPath);
  charMdl.textures.push({
    id: charMdl.textures.length,
    image: path.join(ctx.config.assetPrefix, texPath).replace('.png', '.blp'),
    wrapWidth: false,
    wrapHeight: false,
    wowData: {
      type: 0,
      pngPath: texPath,
    },
  });
  charMdl.materials.push({
    id: charMdl.materials.length,
    constantColor: false,
    twoSided: true,
    layers: [
      {
        filterMode: 'Transparent',
        texture: charMdl.textures.at(-1)!,
        twoSided: true,
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
  charMdl.geosets.forEach((g) => {
    if (g.name.includes('Cloak')) {
      g.material = charMdl.materials.at(-1)!;
    }
  });
}

async function applyPrebakedTextrure(ctx: ExportContext, charMdl: MDL, prep: Prep) {
  if (!prep.prebakedTexture) return;

  const prebakedTexturePath = await exportTexture(prep.prebakedTexture);
  console.log('Character has prebaked texture', prep.prebakedTexture, prebakedTexturePath);
  ctx.assetManager.addPngTexture(prebakedTexturePath);

  const newTexturePath = path.join(ctx.config.assetPrefix, prebakedTexturePath).replace('.png', '.blp');
  charMdl.geosets.forEach((geoset) => {
    // Replace the base texture with the npc baked texture
    geoset.material.layers.forEach((layer) => {
      if (layer.texture.wowData.type === 1) {
        layer.texture.image = newTexturePath;
      }
    });
  });
}

async function applyEquipmentsBodyTextures(ctx: ExportContext, charMdl: MDL, prep: Prep, expansion: ZamExpansion) {
  if (prep.prebakedTexture) return;

  console.log('applyEquipmentsBodyTextures', charMdl.textures.map((t) => `${t.image} ${t.wowData.type}`));
  const baseTexture = charMdl.textures.find((t) => t.wowData.type === 1) ?? charMdl.textures[0];

  console.log('Character has no prebaked texture. Using default texture:', baseTexture.wowData.pngPath);
  if (baseTexture.image === '') {
    throw new Error(`Cannot find the model's base texture.\nIf you are using wowhead dressing room URL,
      it means the expansions of the wowhead URL (${expansion}) doesn't work in WoW ${wowExportClient.cascInfo?.build.Version}
    `);
  }

  const charCus = await fetchCharacterCustomization({
    expansion,
    type: 'character-customization',
    chrModelId: prep.chrModelId,
  });

  // Wowhead overlays texture-only items onto the body compositor in a specific slot priority order (hr)
  // Reference (minified): viewer.min.js hr = [0,16,0,15,1,7,10,5,6,6,8,0,0,17,18,19,14,20,0,9,7,21,22,23,0,24,25,0]
  const wowheadSlotPriority: Partial<Record<EquipmentSlot, number>> = {
    [EquipmentSlot.Head]: 16,
    [EquipmentSlot.Shoulder]: 15,
    [EquipmentSlot.Shirt]: 1,
    [EquipmentSlot.Chest]: 7,
    [EquipmentSlot.Waist]: 10,
    [EquipmentSlot.Legs]: 5,
    [EquipmentSlot.Feet]: 6,
    [EquipmentSlot.Wrist]: 6,
    [EquipmentSlot.Hands]: 8,
    [EquipmentSlot.MainHand]: 17,
    [EquipmentSlot.OffHand]: 18,
    [EquipmentSlot.Tabard]: 9,
  };

  // Build a list of overlays with slot priority and region id; skip cloaks (Back)
  const overlays = prep.equipmentSlots
    .filter((s) => s.slotId !== EquipmentSlot.Cloak)
    .flatMap((s) => {
      // Base priority from Wowhead mapping
      let basePriority = wowheadSlotPriority[s.slotId] ?? 0;
      // Wowhead bumps legs priority by +2 when the legs item has GeosetGroup[2] > 0 (robe-like)
      if (s.slotId === EquipmentSlot.Legs && s.data.originalData?.Item?.GeosetGroup?.[2] > 0) {
        basePriority += 2;
      }
      return s.data.bodyTextureFiles.map((f) => ({
        slotId: s.slotId,
        priority: basePriority,
        componentId: f.componentId,
        fileDataId: f.fileDataId,
      }));
    });

  // Wowhead sorts by slot priority, then applies item regions in ascending SectionType; skip region 12 (cloak)
  overlays.sort((a, b) => (a.priority - b.priority) || (a.componentId - b.componentId));

  const debug = false;

  const textureDraws: PngDraw[] = [];
  for (const t of overlays) {
    if (t.slotId === EquipmentSlot.Cloak) continue; // cloak/back is not baked into base
    const section = charCus.TextureSections.find((s) => s.SectionType === t.componentId);
    if (!section) {
      console.error(chalk.red(`Texture section not found for file ${t.fileDataId} component ${t.componentId}`));
      continue;
    }
    debug && console.log('Draw', getEquipmentSlotName(t.slotId), t.fileDataId, t.priority, t.componentId, section.X, section.Y, section.Width, section.Height);
    const pngPath = path.join(ctx.config.wowExportAssetDir, await exportTexture(t.fileDataId));
    textureDraws.push({
      pngPath,
      x: section.X,
      y: section.Y,
      width: section.Width,
      height: section.Height,
    });
  }

  const basePng = path.join(ctx.config.wowExportAssetDir, baseTexture.wowData.pngPath);
  debug && console.log('Base PNG:', basePng);
  debug && console.log('Texture draws:', textureDraws);

  const newPng = await drawPngsOnBasePng(basePng, textureDraws);

  let newPngName = createHash('md5').update(JSON.stringify({ basePng, textureDraws })).digest('hex');
  newPngName = `${ctx.outputFile}-${newPngName}`;

  const newPngPath = path.join(ctx.config.wowExportAssetDir, `${newPngName}.png`);
  writeFileSync(newPngPath, newPng);
  const newBlpPath = path.join(ctx.config.assetPrefix, path.relative(ctx.config.wowExportAssetDir, newPngPath))
    .replace('.png', '.blp');

  charMdl.textures.forEach((t) => {
    if (t.wowData.type === 1) {
      t.image = newBlpPath;
      t.wowData.pngPath = newPngPath;
    }
  });
  ctx.assetManager.addPngTexture(path.relative(ctx.config.wowExportAssetDir, newPngPath), true);
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
    if (attackTag && attackTag !== 'Auto' && wc3Anim.attackTag !== '' && wc3Anim.attackTag !== attackTag) {
      excludedAnimIds.push(animId);
      continue;
    }
  }
  return excludedAnimIds;
}

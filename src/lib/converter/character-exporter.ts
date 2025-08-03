import chalk from 'chalk';
import path, { join } from 'path';
import { z } from 'zod';

import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import {
  EquipmentSlot,
  getDisplayIdFromUrl,
  getPreferredBaseFromWowheadUrl,
  prepareNpcExport,
  processItemData,
} from '@/lib/wowhead-client/wowhead-client';

import { Config } from '../global-config';
import {
  ANIM_NAMES, AttackTagSchema, getWc3AnimName, getWowAnimName,
} from '../objmdl/animation/animation_mapper';
import { MDL } from '../objmdl/mdl/mdl';
import { AssetManager } from './common/model-manager';

// Local file path must be a relative path and must not contain ".." or start with a slash.
// This is to prevent path traversal attacks and other security issues.
export const LocalRefValueSchema = z.string().refine(
  (val) => {
    // Must not be absolute path
    if (path.isAbsolute(val)) return false;
    // Must not contain ".." as a path segment
    if (val.split(/[\\/]/).some((seg) => seg === '..')) return false;
    // Must not start with "/" or "\"
    if (/^[\\/]/.test(val)) return false;
    // Must not contain null bytes or suspicious chars
    if (/[\0]/.test(val)) return false;
    return true;
  },
  {
    message: 'Local file path must be a relative path and must not contain ".." or start with a slash.',
  },
);

export const LocalRefSchema = z.object({ type: z.literal('local'), value: LocalRefValueSchema });
export const WowheadRefSchema = z.object({ type: z.literal('wowhead'), value: z.string() });
export const DisplayRefSchema = z.object({ type: z.literal('displayID'), value: z.string() });
export const RefSchema = z.discriminatedUnion('type', [LocalRefSchema, WowheadRefSchema, DisplayRefSchema]);
export const AttachItemSchema = z.object({ path: RefSchema, scale: z.number().optional() });
export const CharacterSchema = z.object({
  base: RefSchema,
  attackTag: AttackTagSchema.optional(),
  keepCinematic: z.boolean().optional(),
  inGameMovespeed: z.number(),
  size: z.enum(['small', 'medium', 'large', 'hero', 'semi-giant', 'giant']).optional(),
  scale: z.number().optional(),
  attachItems: z.record(z.union([z.number(), z.string()]), AttachItemSchema).optional(),
  noDecay: z.boolean().optional(),
  portraitCameraSequenceName: z.string().optional(),
});

export type Character = z.infer<typeof CharacterSchema>;
export type Size = z.infer<typeof CharacterSchema>['size'];
export type Ref = z.infer<typeof RefSchema>;
export type LocalRef = z.infer<typeof LocalRefSchema>;
export type WowheadRef = z.infer<typeof WowheadRefSchema>;
export type DisplayRef = z.infer<typeof DisplayRefSchema>;
export type AttachItem = z.infer<typeof AttachItemSchema>;

export const local = (pathStr: string): LocalRef => ({ type: 'local', value: pathStr });
export const wowhead = (url: string): WowheadRef => ({ type: 'wowhead', value: url });
export const displayID = (id: number | string): DisplayRef => ({ type: 'displayID', value: String(id) });

const wantedZPerSize = {
  small: 60,
  medium: 104,
  large: 150,
  hero: 175,
  giant: 350,
};

const debug = true;

export class CharacterExporter {
  public models: [MDL, string][] = [];

  public assetManager: AssetManager;

  constructor(public outputPath: string, public config: Config) {
    this.assetManager = new AssetManager(config);
  }

  public async exportCharacter(char: Character, outputFile: string) {
    await wowExportClient.waitUntilReady();

    const baseRef = char.base;

    if (baseRef.type === 'local') {
      const resolvedAttach = await this.resolveAttachItems(char.attachItems);
      return this.exportModel({ ...char, base: baseRef, attachItems: resolvedAttach }, outputFile);
    }

    return this.exportWowheadModel(baseRef, char, outputFile);
  }

  public includeMdlToOutput(mdl: MDL, outputFile: string) {
    const fullOutputFile = join(this.outputPath, outputFile);
    this.models.push([mdl, fullOutputFile]);
  }

  private exportModel(char: Character, outputFile: string) {
    const start = performance.now();
    console.log('Base model:', path.join(this.config.wowExportAssetDir, char.base.value));
    const model = this.assetManager.parse(char.base.value, true).mdl;
    debug && console.log('Parsed wow.export model took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');
    this.includeMdlToOutput(model, outputFile);

    if (char.attackTag != null) {
      model.sequences = model.sequences.filter((seq) => !char.attackTag || seq.data.attackTag === '' || seq.data.attackTag === char.attackTag);
    }
    if (!char.keepCinematic) {
      model.sequences = model.sequences.filter((seq) => !seq.name.includes('Cinematic'));
    }
    model.modify.optimizeKeyFrames();

    model.modify.addPortraitCamera(char.portraitCameraSequenceName);

    if (char.attachItems) {
      Object.entries(char.attachItems).forEach(([wowAttachmentId, itemPath]) => {
        const wowAttachment = model.wowAttachments.find((a) => a.wowAttachmentId === Number(wowAttachmentId));
        if (wowAttachment) {
          const itemMdl = this.assetManager.parse(itemPath.path.value, true).mdl;
          itemMdl.bones[0].name += `_atm_${wowAttachmentId}`;
          if (itemPath.scale) {
            itemMdl.modify.scale(itemPath.scale);
          }
          const useAttachmentPath = false;
          if (!useAttachmentPath) {
            model.modify.addMdlItemToBone(itemMdl, wowAttachment.bone.name);
          } else {
            this.models.push([itemMdl, join(this.outputPath, itemPath.path.value)]);
            model.modify.addItemPathToBone(`${itemPath.path.value}.mdx`, wowAttachment.bone.name);
          }
        } else {
          console.error(chalk.red(`Cannot find bone for wow attachment ${wowAttachmentId}`));
          if (model.wowAttachments.length === 0) {
            console.error(chalk.red(`No WoW attachments data found in this model "${char.base.value}"`));
          }
        }
      });
    }

    if (char.size) {
      const wantedZ = wantedZPerSize[char.size];
      const standSeq = model.sequences.find((seq) => seq.name === 'Stand') ?? model.sequences[0];
      if (standSeq) {
        const maxStandZ = model.modify.getMaxZAtTimestamp(standSeq, 0);
        debug && console.log(model.model.name, { wantedZ, maxStandZ });
        model.modify.scale((char.scale ?? 1) * wantedZ / maxStandZ);
      } else {
        model.modify.scale((char.scale ?? 1) * wantedZ / model.model.maximumExtent[2]);
      }
    } else if (char.scale) {
      model.modify.scale(char.scale);
    }

    {
      const walkSeq = model.sequences.find((seq) => seq.name === 'Walk');
      const walkFastSeq = model.sequences.find((seq) => seq.name === 'Walk Fast');
      if (!walkSeq && walkFastSeq) {
        walkFastSeq.name = 'Walk';
      }
    }

    if (char.inGameMovespeed) {
      model.sequences.filter((seq) => seq.moveSpeed > 0 && seq.name.includes('Walk')
      && !seq.name.includes('Spin')
      && !seq.name.includes('Swim')
      && !seq.name.includes('Alternate')).forEach((seq) => {
        debug && console.log(model.model.name, `${seq.name} (${seq.data.wowName})`, 'old moveSpeed', seq.moveSpeed, 'new moveSpeed', char.inGameMovespeed);
        const scale = (seq.moveSpeed || 450) / char.inGameMovespeed; // duration is inverse of speed
        model.modify.scaleSequenceDuration(seq, scale);
        seq.moveSpeed = char.inGameMovespeed;
      });
    }

    if (!char.noDecay) {
      model.modify.addDecayAnimation();
    }
    return model;
  }

  // ---------------------------------------------------------------------------
  // Wowhead export pipeline (handles both model-only and character exports)
  // ---------------------------------------------------------------------------

  private async exportWowheadModel(
    baseRef: Ref,
    originalChar: Character,
    outputFile: string,
  ) {
    const start = performance.now();
    const npcDisplayId = baseRef.type === 'wowhead' ? await getDisplayIdFromUrl(baseRef.value) : Number(baseRef.value);

    const baseWowheadUrl = baseRef.type === 'wowhead' ? baseRef.value : undefined;
    const prep = await prepareNpcExport(npcDisplayId, baseWowheadUrl);

    const slotModelPaths = new Map<string, [string, number]>();
    const slotModelPathsR = new Map<string, [string, number]>();
    const slotTexturePaths = new Map<string, string>();
    let npcTexturePath: string | undefined;
    let exportPath = '';

    // ---------------- CHARACTER-BASED NPC ----------------
    if (prep.type === 'character') {
      const modelExports: { fileDataID: number; skinName?: string }[] = [];
      const modelSlots = prep.equipmentSlots.filter((s) => s.hasModel && s.data);

      for (const slot of modelSlots) {
        const data = slot.data;
        if (data.modelFiles?.[0]?.fileDataId) {
          const modelId = data.modelFiles[0].fileDataId;
          modelExports.push({ fileDataID: modelId, skinName: await this.getSkinName(modelId, data.textureFiles) });
          if (slot.slotId === EquipmentSlot.Shoulder.toString() && data.modelFiles?.[1]?.fileDataId) {
            const modelIdR = data.modelFiles[1].fileDataId;
            modelExports.push({ fileDataID: modelIdR, skinName: await this.getSkinName(modelIdR, data.textureFiles) });
          }
        }
      }

      const exportedModels = modelExports.length > 0 ? await wowExportClient.exportModels(modelExports) : [];

      // Map to slots
      for (const slot of modelSlots) {
        const data = slot.data;
        if (!data.modelFiles?.[0]?.fileDataId) continue;
        const modelId = data.modelFiles[0].fileDataId;
        const exp = exportedModels.find((m) => m.fileDataID === modelId);

        slotModelPaths.set(slot.slotId, [
          this.relativeToExport(exp?.files.find((f) => f.type === 'OBJ')?.file)!,
          1,
        ]);

        if (slot.slotId === EquipmentSlot.Shoulder.toString() && data.modelFiles?.[1]?.fileDataId) {
          const modelIdR = data.modelFiles[1].fileDataId;
          const expR = exportedModels.find((m) => m.fileDataID === modelIdR);
          slotModelPathsR.set(slot.slotId, [
            this.relativeToExport(expR?.files.find((f) => f.type === 'OBJ')?.file)!,
            1,
          ]);
        }
      }

      // Texture-only slots
      const textureSlots = prep.equipmentSlots.filter((s) => !s.hasModel && s.data);
      for (const slot of textureSlots) {
        if (slot.data?.textureFiles?.[0]?.fileDataId) {
          const textureId = slot.data.textureFiles[0].fileDataId;
          const texExport = await wowExportClient.exportTextures([textureId]);
          slotTexturePaths.set(slot.slotId, this.relativeToExport(texExport[0].file)!);
        }
      }

      // NPC base texture
      if (prep.npcTextureFile) {
        const tex = await wowExportClient.exportTextures([prep.npcTextureFile]);
        npcTexturePath = this.relativeToExport(tex[0].file);
      }

      // If cinematic animations should be removed, compute their IDs and pass to RPC to reduce export size
      const excludedAnimIds: number[] = [];
      // Iterate through a reasonable range of animation IDs to detect cinematic ones
      for (let animId = 0; animId < ANIM_NAMES.length; animId++) {
        const wc3Anim = getWc3AnimName(getWowAnimName(animId));

        if (!originalChar.keepCinematic && wc3Anim.wc3Name.includes('Cinematic')) {
          excludedAnimIds.push(animId);
        }
        if (originalChar.attackTag && wc3Anim.attackTag !== '' && wc3Anim.attackTag !== originalChar.attackTag) {
          excludedAnimIds.push(animId);
        }
      }
      prep.exportCharRpcParams.excludeAnimationIds = [...new Set(excludedAnimIds)];

      // Export character via RPC to OBJ directory
      const result = await wowExportClient.exportCharacter({ ...prep.exportCharRpcParams });
      exportPath = this.relativeToExport(result.exportPath)!;
      if (!exportPath) {
        console.error('Failed to export character NPC');
        console.error({ exportedModels, result });
      } else {
        debug && console.log('exported character path:', exportPath);
      }
    } else {
      // ---------------- MODEL-ONLY NPC ----------------
      const modelExports: { fileDataID: number; skinName?: string }[] = [prep.baseModel];
      const modelSlots = prep.equipmentSlots.filter((s) => s.hasModel && s.data);

      for (const slot of modelSlots) {
        const data = slot.data;
        if (!data.modelFiles?.[0]?.fileDataId) continue;
        const modelId = data.modelFiles[0].fileDataId;
        modelExports.push({ fileDataID: modelId, skinName: await this.getSkinName(modelId, data.textureFiles) });
        if (slot.slotId === EquipmentSlot.Shoulder.toString() && data.modelFiles?.[1]?.fileDataId) {
          const modelIdR = data.modelFiles[1].fileDataId;
          modelExports.push({ fileDataID: modelIdR, skinName: await this.getSkinName(modelIdR, data.textureFiles) });
        }
      }

      // Deduplicate & export
      const exportedModels = await wowExportClient.exportModels(modelExports);

      // Base model path
      const baseExp = exportedModels.find((m) => m.fileDataID === prep.baseModel.fileDataID);
      exportPath = this.relativeToExport(baseExp?.files.find((f) => f.type === 'OBJ')?.file)!;
      if (!exportPath) {
        console.error('Failed to export model-only NPC', prep.baseModel);
        console.error(JSON.stringify({ exportedModels, baseExp, modelExports }, null, 2));
      }

      // Map equipment
      for (const slot of modelSlots) {
        const data = slot.data;
        if (!data.modelFiles?.[0]?.fileDataId) continue;
        const modelId = data.modelFiles[0].fileDataId;
        const exp = exportedModels.find((m) => m.fileDataID === modelId);
        slotModelPaths.set(slot.slotId, [
          this.relativeToExport(exp?.files.find((f) => f.type === 'OBJ')?.file)!,
          1,
        ]);

        if (slot.slotId === EquipmentSlot.Shoulder.toString() && data.modelFiles?.[1]?.fileDataId) {
          const modelIdR = data.modelFiles[1].fileDataId;
          const expR = exportedModels.find((m) => m.fileDataID === modelIdR);
          slotModelPathsR.set(slot.slotId, [
            this.relativeToExport(expR?.files.find((f) => f.type === 'OBJ')?.file)!,
            1,
          ]);
        }
      }

      // Texture-only slots
      const textureSlots = prep.equipmentSlots.filter((s) => !s.hasModel && s.data);
      for (const slot of textureSlots) {
        if (!slot.data?.textureFiles?.[0]?.fileDataId) continue;
        const textureId = slot.data.textureFiles[0].fileDataId;
        const texExport = await wowExportClient.exportTextures([textureId]);
        slotTexturePaths.set(slot.slotId, this.relativeToExport(texExport[0].file)!);
      }
    }

    // -----------------------------------------------------------------------
    // Build attachItems map (existing custom + equipment models)
    // -----------------------------------------------------------------------

    debug && console.log('start resolveAttachItems');
    const attachItems = await this.resolveAttachItems(originalChar.attachItems);
    debug && console.log('end resolveAttachItems');

    // Equipment attachments
    const equipmentToAttachmentMap: Partial<Record<EquipmentSlot, WoWAttachmentID[]>> = {
      [EquipmentSlot.Head]: [WoWAttachmentID.Helm],
      [EquipmentSlot.Shoulder]: [WoWAttachmentID.ShoulderLeft, WoWAttachmentID.ShoulderRight],
    };
    const attachItemModel = (
      equipSlot: EquipmentSlot,
      left: [string, number] | undefined,
      right: [string, number] | undefined,
    ) => {
      const atts = equipmentToAttachmentMap[equipSlot];
      if (!atts) return;
      if (left) attachItems[atts[0]] = { path: local(left[0]), scale: left[1] };
      if (right && atts.length > 1) attachItems[atts[1]] = { path: local(right[0]), scale: right[1] };
    };
    attachItemModel(EquipmentSlot.Head, slotModelPaths.get(EquipmentSlot.Head.toString()), undefined);
    attachItemModel(EquipmentSlot.Shoulder, slotModelPaths.get(EquipmentSlot.Shoulder.toString()), slotModelPathsR.get(EquipmentSlot.Shoulder.toString()));

    // -----------------------------------------------------------------------
    // Final MDL export
    // -----------------------------------------------------------------------

    console.log('wow.export took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');
    const start2 = performance.now();
    const mdl = this.exportModel({ ...originalChar, base: local(exportPath), attachItems }, outputFile);
    console.log('exportModel took', chalk.yellow(((performance.now() - start2) / 1000).toFixed(2)), 's');

    // NPC face/skin override
    if (npcTexturePath) {
      this.assetManager.addPngTexture(npcTexturePath);
      const baseTexturePath = mdl.geosets[0].material.layers[0].texture.image;
      const newTexturePath = path.join(this.config.assetPrefix, npcTexturePath).replace('.png', '.blp');

      mdl.geosets.forEach((geoset, i) => {
        // For skeleton, we don't want to override anything but
        // For some reason, the skull is not included in the npc baked texture
        let skip = false;
        const raceSkeleton = 20;
        if (prep.type === 'character' && prep.exportCharRpcParams.race === raceSkeleton) {
          skip = true;
          let geosetIds: number[] | undefined;
          if (geoset.name.includes('Glove') && mdl.geosets[i - 1].name.includes('Glove')) {
            geosetIds = prep.equipmentSlots.find((s) => s.slotId === EquipmentSlot.Gloves.toString())?.data.geosetIds;
          } else if (geoset.name.includes('Boot') && mdl.geosets[i + 1].name.includes('Boot')) {
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

    // Cloak etc additional textures
    const textureSlotConfigs: Partial<Record<EquipmentSlot, { geosetNames: string[]; twoSided?: boolean }>> = {
      [EquipmentSlot.Back]: { geosetNames: ['Cloak'], twoSided: true },
    };

    for (const [slotId, texPath] of slotTexturePaths) {
      const slotEnum = parseInt(slotId, 10) as EquipmentSlot;
      const cfg = textureSlotConfigs[slotEnum];
      if (!cfg) continue;
      const matching = mdl.geosets.filter((g) => cfg.geosetNames.some((name) => g.name.includes(name)));
      if (matching.length === 0) continue;
      this.assetManager.addPngTexture(texPath);
      mdl.textures.push({
        id: mdl.textures.length,
        image: path.join(this.config.assetPrefix, texPath).replace('.png', '.blp'),
        wrapWidth: false,
        wrapHeight: false,
      });
      mdl.materials.push({
        id: mdl.materials.length,
        constantColor: false,
        twoSided: cfg.twoSided ?? false,
        layers: [
          {
            filterMode: 'Transparent',
            texture: mdl.textures.at(-1)!,
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
      matching.forEach((g) => { g.material = mdl.materials.at(-1)!; });
    }

    return mdl;
  }

  private async getSkinName(
    modelId: number,
    textureFiles: { fileDataId: number }[],
  ): Promise<string | undefined> {
    if (textureFiles.length === 0) return undefined;
    const skins = await wowExportClient.getModelSkins(modelId);
    return skins.find((skin) => skin.textureIDs
      .every((id: number) => textureFiles.map((t) => t.fileDataId).includes(id)))?.id
    ?? skins[0]?.id;
  }

  private async resolveItemRef(ref: Ref): Promise<string> {
    if (ref.type === 'local') return ref.value;

    debug && console.log('resolveItemRef getDisplayIdFromUrl start', ref);
    const preferredBase = ref.type === 'wowhead' ? getPreferredBaseFromWowheadUrl(ref.value) : undefined;

    const displayId = ref.type === 'wowhead' ? await getDisplayIdFromUrl(ref.value) : Number(ref.value);
    debug && console.log('resolveItemRef getDisplayIdFromUrl end', displayId);
    const itemData = await processItemData(-1, displayId, 0, 0, preferredBase);
    const skinName = await this.getSkinName(itemData.modelFiles[0].fileDataId, itemData.textureFiles);
    const exported = (await wowExportClient.exportModels([
      { fileDataID: itemData.modelFiles[0].fileDataId, skinName },
    ]))[0];
    const obj = exported.files.find((f) => f.type === 'OBJ')?.file;
    if (!obj) throw new Error('Failed to export attachment item OBJ');
    return this.relativeToExport(obj)!;
  }

  private async resolveAttachItems(map?: Record<number, AttachItem>): Promise<Record<number, AttachItem>> {
    const resolved: Record<number, AttachItem> = {};
    if (!map) return resolved;
    for (const [id, data] of Object.entries(map)) {
      const p = await this.resolveItemRef(data.path);
      resolved[Number(id)] = { path: local(p), scale: data.scale };
    }
    return resolved;
  }

  private relativeToExport(p: string | undefined): string | undefined {
    return p ? path.relative(this.config.wowExportAssetDir, p) : undefined;
  }
}

import chalk from 'chalk';
import { writeFileSync } from 'fs';
import { ensureDirSync } from 'fs-extra';
import path, { dirname, join } from 'path';
import { z } from 'zod';

import { AssetManager } from '@/lib/converter/common/asset-manager';
import { Sequence } from '@/lib/formats/mdl/components/sequence';
import { MDL } from '@/lib/formats/mdl/mdl';
import { canAddMdlCollectionItemToModel } from '@/lib/formats/mdl/modify/add-item-to-model';
import { Config } from '@/lib/global-config';
import { Vector3 } from '@/lib/math/common';
import { V3 } from '@/lib/math/vector';
import { getWoWAttachmentName, WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { decodeDressingRoom } from '@/lib/wowhead-client/dressing-room';
import { CharacterData, fetchNpcMeta, fetchObjectMeta } from '@/lib/wowhead-client/objects';
import { getZamUrlFromWowheadUrl, ZamUrl } from '@/lib/wowhead-client/zam-url';

import { AttackTagSchema } from '../../objmdl/animation/animation_mapper';
import { Model } from '../common/models';
import { guessAttackTag, InventoryType } from './item-mapper';
import { ensureLocalModelFileExists, ExportContext } from './utils';
import { exportCharacterAsMdl } from './wowhead-exporter/character-model';
import { exportCreatureNpcAsMdl } from './wowhead-exporter/creature-model';
import { exportZamItemAsMdl } from './wowhead-exporter/item-model';

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
  particlesDensity: z.number().optional(),
  portraitCameraSequenceName: z.string().optional(),
  mount: z.object({
    path: RefSchema,
    scale: z.number().optional(),
    seatOffset: z.number().array().length(3).optional(),
    animation: z.string().optional(),
  }).optional(),
  forceSheathed: z.boolean().optional(),
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

const debug = false;

export class CharacterExporter {
  public assetManager: AssetManager;

  public models: [MDL, string][] = [];

  constructor(public config: Config) {
    this.assetManager = new AssetManager(config);
  }

  public includeMdlToOutput(mdl: MDL, outputFile: string) {
    this.models.push([mdl, outputFile]);
    return this.models.at(-1)!;
  }

  async exportCharacter(char: Character, outputFile: string): Promise<MDL> {
    await wowExportClient.waitUntilReady();

    console.log('Exporting character', char.base.value);

    const start = performance.now();

    if (char.mount && char.mount.path.value !== '') {
      return (await this.exportCharacterWithMount(char, outputFile)).mountMdl;
    }

    const ctx: ExportContext = {
      assetManager: this.assetManager,
      config: this.config,
      outputFile,
      weaponInventoryTypes: [undefined, undefined],
      forceSheathed: char.forceSheathed,
    };

    const model = await this.exportBaseMdl(ctx, char);
    model.model.name = outputFile;
    this.includeMdlToOutput(model, outputFile);

    if (!char.keepCinematic) {
      model.sequences = model.sequences.filter((seq) => !seq.name.includes('Cinematic') || seq.keep);
    }
    char.portraitCameraSequenceName = 'Stand';
    model.modify.addPortraitCamera(char.portraitCameraSequenceName);

    if (char.attachItems) {
      for (const [wowAttachmentId, itemPath] of Object.entries(char.attachItems)) {
        const wowAttachment = model.wowAttachments.find((a) => a.wowAttachmentId === Number(wowAttachmentId));
        if (wowAttachment) {
          const { model: itemModel, inventoryType } = await this.exportItem(ctx, itemPath.path);
          const itemMdl = itemModel.mdl;
          if (canAddMdlCollectionItemToModel(model, itemMdl)) {
            model.modify.addMdlCollectionItemToModel(itemMdl);
            continue;
          }

          if (inventoryType) {
            if (Number(wowAttachmentId) === WoWAttachmentID.HandRight) ctx.weaponInventoryTypes[0] ??= inventoryType;
            if (Number(wowAttachmentId) === WoWAttachmentID.HandLeft) ctx.weaponInventoryTypes[1] ??= inventoryType;
            if (Number(wowAttachmentId) === WoWAttachmentID.Shield) ctx.weaponInventoryTypes[1] ??= inventoryType;
          }

          itemMdl.bones[0].name += `_atm_${wowAttachmentId}`;
          if (itemPath.scale) {
            itemMdl.modify.scale(itemPath.scale);
          }
          const useAttachmentPath = false;
          if (!useAttachmentPath) {
            model.modify.addMdlItemToBone(itemMdl, wowAttachment.bone);
          } else {
            this.includeMdlToOutput(itemMdl, itemModel.relativePath);
            model.modify.addItemPathToBone(`${itemModel.relativePath}.mdx`, wowAttachment.bone);
          }
        } else {
          console.error(chalk.red(`Cannot find bone for wow attachment ${wowAttachmentId} (${getWoWAttachmentName(Number(wowAttachmentId))})`));
          if (model.wowAttachments.length === 0) {
            console.error(chalk.red(`No WoW attachments data found in this model "${char.base.value}"`));
          }
        }
      }
    }

    if (char.attackTag != null) {
      if (char.attackTag === 'Auto') {
        char.attackTag = guessAttackTag(ctx.weaponInventoryTypes[0] ?? 0, ctx.weaponInventoryTypes[1] ?? 0);
      }
      console.log('Chosen attack tag:', char.attackTag);
      model.sequences = model.sequences.filter((seq) => !char.attackTag || seq.data.attackTag === '' || seq.data.attackTag === char.attackTag);
    }

    if (char.size) {
      const wantedZ = wantedZPerSize[char.size];
      const standSeq = model.sequences.find((seq) => seq.name === 'Stand') ?? model.sequences[0];
      if (standSeq) {
        const maxStandZ = model.modify.getMaxZAtTimestamp(standSeq, 0);
        if (maxStandZ > 0) {
          debug && console.log(model.model.name, { wantedZ, maxStandZ });
          model.modify.scale((char.scale ?? 1) * wantedZ / maxStandZ);
        } else {
          console.log(chalk.red(`Cannot scale model ${model.model.name} because the model is not above the ground:`), { maxStandZ });
        }
      } else {
        if (model.model.maximumExtent[2] > 0) {
          model.modify.scale((char.scale ?? 1) * wantedZ / model.model.maximumExtent[2]);
        } else {
          console.log(chalk.red(`Cannot scale model ${model.model.name} because it has non-positive maximum extent Z:`, model.model.maximumExtent));
        }
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

    if (model.sequences.find((seq) => seq.name.startsWith('Portrait Talk'))) {
      model.sequences.forEach((seq) => {
        if (seq.name === 'Stand') {
          model.modify.cloneSequence(seq, 'Portrait');
        }
      });
    }

    // Concatenate Bow, Rifle, Thrown sequences from wow Load and Attack
    ['Bow', 'Rifle', 'Thrown'].forEach((attackTag) => {
      const attacks = model.sequences.filter((seq) => seq.data.attackTag === attackTag && seq.name === 'Attack');
      if (attacks.length === 0) return;
      const score = (seq: Sequence) => {
        if (seq.data.wowName.startsWith('Load')) return 1;
        if (seq.data.wowName.startsWith('Attack')) return 2;
        return 3;
      };
      attacks.sort((a, b) => score(a) - score(b));
      console.log('concatenate animations', 'Attack', attackTag, 'from', attacks.map((s) => s.data.wowName));
      model.modify.concatenateSequences(attacks, 'Attack');
      model.sequences = model.sequences.filter((seq) => !attacks.includes(seq));
    });

    model.modify.optimizeKeyFrames();
    console.log('CharacterExporter.exportCharacter took', chalk.yellow(((performance.now() - start) / 1000).toFixed(2)), 's');
    return model;
  }

  private async exportBaseMdl(ctx: ExportContext, char: Character): Promise<MDL> {
    if (char.base.type === 'local') {
      await ensureLocalModelFileExists(char.base.value);
      return this.assetManager.parse(char.base.value, true).mdl;
    }
    if (char.base.type === 'wowhead' || char.base.type === 'displayID') {
      const baseZam: ZamUrl = char.base.type === 'wowhead'
        ? await getZamUrlFromWowheadUrl(char.base.value)
        : { expansion: 'live', type: 'npc', displayId: Number(char.base.value) };

      let npcMeta: CharacterData;
      switch (baseZam.type) {
        case 'item':
          try {
            npcMeta = await fetchNpcMeta({
              ...baseZam,
              type: 'npc',
              expansion: wowExportClient.isClassic()
                ? baseZam.expansion
                : 'latest-available', // in latest wow installation, classic models are not available
            });
            break;
          } catch (e) {
            console.log(chalk.red('Failed to fetch npc meta, trying item meta'));
            return (await this.exportItem(ctx, char.base)).model.mdl;
          }
        case 'npc':
          npcMeta = await fetchNpcMeta({
            ...baseZam,
            type: 'npc',
            expansion: wowExportClient.isClassic()
              ? baseZam.expansion
              : 'latest-available', // in latest wow installation, classic models are not available
          });
          break;
        case 'object':
          npcMeta = await fetchObjectMeta({
            ...baseZam,
            type: 'object',
            expansion: wowExportClient.isClassic()
              ? baseZam.expansion
              : 'latest-available', // in latest wow installation, classic models are not available
          });
          break;
        case 'dressing-room':
          npcMeta = await decodeDressingRoom(baseZam.expansion, baseZam.hash);
          break;
        default:
          throw new Error(`Unallowed base zam type: ${baseZam.type}`);
      }

      if (npcMeta.Model) {
        return exportCreatureNpcAsMdl(ctx, npcMeta);
      }

      return exportCharacterAsMdl({
        ctx,
        metaData: npcMeta,
        expansion: baseZam.expansion,
        keepCinematic: Boolean(char.keepCinematic),
        attackTag: char.attackTag,
      });
    }
    throw new Error('Unknown base type');
  }

  private async exportItem(ctx: ExportContext, ref: Ref): Promise<{model: Model, inventoryType?: InventoryType}> {
    if (ref.type === 'local') {
      await ensureLocalModelFileExists(ref.value);
      return { model: this.assetManager.parse(ref.value, true) };
    }
    if (ref.type === 'wowhead' || ref.type === 'displayID') {
      const zam: ZamUrl = ref.type === 'wowhead'
        ? await getZamUrlFromWowheadUrl(ref.value)
        : {
          expansion: 'live', type: 'item', displayId: Number(ref.value), slotId: null,
        };
      if (zam.type !== 'item') throw new Error('Expected item zam url');
      const { model, itemData } = await exportZamItemAsMdl({
        ctx,
        zam,
        targetRace: 0, // universal
        targetGender: 2, // universal
      });
      return { model, inventoryType: itemData.inventoryType };
    }
    throw new Error('Unknown item type');
  }

  private async exportCharacterWithMount(char: Character, outputFile: string): Promise<{charMdl: MDL, mountMdl: MDL}> {
    if (!char.mount) throw new Error('Mount is required');

    const debug = false;

    // Export mount model
    const mount = char.mount;
    const mountName = `${outputFile}_mount`;
    const mountMdl = await this.exportCharacter({
      base: mount.path,
      inGameMovespeed: char.inGameMovespeed,
      attackTag: char.attackTag,
      keepCinematic: char.keepCinematic,
      noDecay: char.noDecay,
      particlesDensity: char.particlesDensity,
      portraitCameraSequenceName: char.portraitCameraSequenceName,
      size: undefined,
      scale: undefined,
      attachItems: undefined,
      mount: undefined,
    }, mountName);
    const mountBone = mountMdl.wowAttachments.find((a) => a.wowAttachmentId === WoWAttachmentID.Shield)?.bone;
    if (!mountBone) throw new Error('Mount model doesn\'t have any attachment for rider');

    const charMdl = await this.exportCharacter({ ...char, mount: undefined, forceSheathed: true }, outputFile);
    charMdl.sequences = charMdl.sequences.filter((s) => ['Mount', 'Death'].some((name) => s.name.includes(name)));
    const mountAnimName = mount.animation ?? 'Mount';
    debug && console.log('mountAnimName', mountAnimName);
    debug && console.log('charMdl.sequences', charMdl.sequences.map((s) => s.data.wowName));
    const mountAnims = charMdl.sequences.filter((s) => s.data.wowName === mountAnimName);
    if (mountAnims.length === 0) {
      throw new Error(`Character model doesn't have any ${mountAnimName} animation`);
    }
    charMdl.sequences.forEach((s) => {
      if (s.data.wowName === mountAnimName) {
        s.name = 'Stand';
      } else if (s.name.includes('Mount')) {
        s.name = `Mount ${s.data.wowName}`;
      }
    });

    const newOverhead = charMdl.attachments.find((a) => a.data?.wowAttachment.wowAttachmentId === WoWAttachmentID.PlayerNameMounted);
    const oldOverhead = charMdl.attachments.find((a) => a.data?.wowAttachment.wowAttachmentId === WoWAttachmentID.PlayerName);
    if (newOverhead) {
      newOverhead.name = 'Overhead';
      if (oldOverhead) {
        oldOverhead.name = `Wow:${WoWAttachmentID.PlayerName}:${getWoWAttachmentName(WoWAttachmentID.PlayerName)}`;
      }
    }

    // Scale mount model to match character model scale and mount scale
    mountMdl.modify.scale(charMdl.accumScale * (mount.scale ?? 1));

    const atm = mountMdl.modify.addItemPathToBone(`${outputFile}.mdx`, mountBone, false);
    if (mount.seatOffset) {
      atm.translation = {
        type: 'translation',
        globalSeq: mountMdl.globalSequences.at(-1)!,
        interpolation: 'DontInterp',
        keyFrames: new Map([[0, V3.scale(mount.seatOffset as Vector3, charMdl.accumScale)]]),
      };
    }

    // Hide rider when mount dies
    const deathDecaySeqs = mountMdl.sequences.filter((s) => s.name.includes('Death') || s.name.includes('Decay'));
    deathDecaySeqs.forEach((s) => {
      if (!atm.scaling) {
        atm.scaling = {
          type: 'scaling',
          globalSeq: mountMdl.globalSequences.at(-1)!,
          interpolation: 'DontInterp',
          keyFrames: new Map(),
        };
      }
      atm.scaling?.keyFrames.set(s.interval[0], [0, 0, 0]);
      atm.scaling?.keyFrames.set(s.interval[1], [0, 0, 0]);
    });
    return { charMdl, mountMdl };
  }

  optimizeModelsTextures() {
    this.models.forEach(([model]) => model.modify.optimizeAll());
    this.assetManager.purgeTextures(this.models.flatMap(([m]) => m.textures.map((t) => t.image)));
  }

  writeAllModels(outputDir: string, format: 'mdx' | 'mdl') {
    const fullPaths: string[] = [];
    for (const [model, path] of this.models) {
      const fullPath = join(outputDir, path);
      fullPaths.push(fullPath);
      ensureDirSync(dirname(fullPath));
      if (format === 'mdx') {
        writeFileSync(`${fullPath}.mdx`, model.toMdx());
      } else {
        writeFileSync(`${fullPath}.mdl`, model.toMdl());
      }
      console.log('Wrote model', chalk.green(`${fullPath}.${format}`));
    }
    return fullPaths;
  }

  async writeAllTextures(outputDir: string) {
    return this.assetManager.exportTextures(outputDir);
  }
}

import chalk from 'chalk';
import { writeFileSync } from 'fs';
import { ensureDirSync } from 'fs-extra';
import path, { dirname, join } from 'path';
import { z } from 'zod';

import { AssetManager } from '@/lib/converter/common/asset-manager';
import { Config } from '@/lib/global-config';
import { getWoWAttachmentName } from '@/lib/objmdl/animation/bones_mapper';
import { MDL } from '@/lib/objmdl/mdl/mdl';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { fetchNpcMeta } from '@/lib/wowhead-client/npc';
import { getZamUrlFromWowheadUrl, ZamUrl } from '@/lib/wowhead-client/zam-url';

import { AttackTagSchema } from '../../objmdl/animation/animation_mapper';
import { ensureLocalModelFileExists } from './utils';
import { exportCharacterNpcAsMdl } from './wowhead-exporter/character-model';
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

const debug = false;

export class CharacterExporter {
  public assetManager: AssetManager;

  public models: [MDL, string][] = [];

  constructor(public config: Config) {
    this.assetManager = new AssetManager(config);
  }

  public includeMdlToOutput(mdl: MDL, outputFile: string) {
    this.models.push([mdl, outputFile]);
  }

  async exportCharacter(char: Character, outputFile: string) {
    await wowExportClient.waitUntilReady();

    const model = await this.exportBaseMdl(char);
    this.includeMdlToOutput(model, outputFile);

    if (char.attackTag != null) {
      model.sequences = model.sequences.filter((seq) => !char.attackTag || seq.data.attackTag === '' || seq.data.attackTag === char.attackTag);
    }
    if (!char.keepCinematic) {
      model.sequences = model.sequences.filter((seq) => !seq.name.includes('Cinematic') || seq.keep);
    }
    char.portraitCameraSequenceName = 'Stand';
    model.modify.addPortraitCamera(char.portraitCameraSequenceName);

    if (char.attachItems) {
      for (const [wowAttachmentId, itemPath] of Object.entries(char.attachItems)) {
        const wowAttachment = model.wowAttachments.find((a) => a.wowAttachmentId === Number(wowAttachmentId));
        if (wowAttachment) {
          const itemMdl = await this.exportItem(itemPath.path);
          itemMdl.bones[0].name += `_atm_${wowAttachmentId}`;
          if (itemPath.scale) {
            itemMdl.modify.scale(itemPath.scale);
          }
          const useAttachmentPath = false;
          if (!useAttachmentPath) {
            model.modify.addMdlItemToBone(itemMdl, wowAttachment.bone.name);
          } else {
            this.includeMdlToOutput(itemMdl, itemPath.path.value);
            model.modify.addItemPathToBone(`${itemPath.path.value}.mdx`, wowAttachment.bone.name);
          }
        } else {
          console.error(chalk.red(`Cannot find bone for wow attachment ${wowAttachmentId} (${getWoWAttachmentName(Number(wowAttachmentId))})`));
          if (model.wowAttachments.length === 0) {
            console.error(chalk.red(`No WoW attachments data found in this model "${char.base.value}"`));
          }
        }
      }
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

    model.modify.optimizeKeyFrames();
    return model;
  }

  private async exportBaseMdl(char: Character): Promise<MDL> {
    if (char.base.type === 'local') {
      await ensureLocalModelFileExists(char.base.value);
      return this.assetManager.parse(char.base.value, true).mdl;
    }
    if (char.base.type === 'wowhead' || char.base.type === 'displayID') {
      const baseZam: ZamUrl = char.base.type === 'wowhead'
        ? await getZamUrlFromWowheadUrl(char.base.value)
        : { expansion: 'live', type: 'npc', displayId: Number(char.base.value) };

      if (baseZam.type !== 'npc') throw new Error(`Expected npc zam url, got ${baseZam.type}`);

      const npcMeta = await fetchNpcMeta(baseZam);
      if (npcMeta.Model) {
        return exportCreatureNpcAsMdl({ assetManager: this.assetManager, config: this.config }, baseZam);
      }

      return exportCharacterNpcAsMdl({
        ctx: { assetManager: this.assetManager, config: this.config },
        zam: {
          ...baseZam,
          expansion: wowExportClient.isClassic()
            ? baseZam.expansion
            : 'latest-available', // // in latest wow installation, classic models are not available
        },
        keepCinematic: Boolean(char.keepCinematic),
        attackTag: char.attackTag,
      });
    }
    throw new Error('Unknown base type');
  }

  private async exportItem(ref: Ref): Promise<MDL> {
    if (ref.type === 'local') {
      await ensureLocalModelFileExists(ref.value);
      return this.assetManager.parse(ref.value, true).mdl;
    }
    if (ref.type === 'wowhead' || ref.type === 'displayID') {
      const zam: ZamUrl = ref.type === 'wowhead'
        ? await getZamUrlFromWowheadUrl(ref.value)
        : {
          expansion: 'live', type: 'item', displayId: Number(ref.value), slotId: null,
        };
      if (zam.type !== 'item') throw new Error('Expected item zam url');
      return exportZamItemAsMdl({
        ctx: { assetManager: this.assetManager, config: this.config },
        zam,
        targetRace: 0, // universal
        targetGender: 2, // universal
      });
    }
    throw new Error('Unknown item type');
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

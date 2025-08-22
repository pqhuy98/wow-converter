import { join } from 'path';

import { Config } from '@/lib/global-config';
import { MDL } from '@/lib/objmdl/mdl/mdl';
import { AssetManager } from '../common/model-manager';
import { exportCharacterNpcAsMdl } from './character-model';
import { exportCreatureNpcAsMdl } from './creature-model';
import { ItemZamUrl, ZamUrl, parseWowheadUrlToZam } from '@/lib/wowhead-client/zam-url';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';
import { Character, Ref } from '../character-exporter';
import { fetchNpcMeta } from '@/lib/wowhead-client/npc';
import { exportZamItemAsMdl } from './item-model';
import chalk from 'chalk';

const wantedZPerSize = {
  small: 60,
  medium: 104,
  large: 150,
  hero: 175,
  giant: 350,
};

const debug = false;

export class CharacterExporterTest {
  public assetManager: AssetManager;
  public models: [MDL, string][] = [];

  constructor(public outputPath: string, public config: Config) {
    this.assetManager = new AssetManager(config);
  }

  public includeMdlToOutput(mdl: MDL, outputFile: string) {
    const fullOutputFile = join(this.outputPath, outputFile);
    this.models.push([mdl, fullOutputFile]);
  }

  async exportCharacter(char: Character, outputFile: string) {
    await wowExportClient.waitUntilReady();

    const model = await this.exportBaseMdl(char);
    this.includeMdlToOutput(model, outputFile);

    if (char.attackTag != null) {
      model.sequences = model.sequences.filter((seq) => !char.attackTag || seq.data.attackTag === '' || seq.data.attackTag === char.attackTag);
    }
    if (!char.keepCinematic) {
      model.sequences = model.sequences.filter((seq) => !seq.name.includes('Cinematic'));
    }
    model.modify.addPortraitCamera(char.portraitCameraSequenceName);

    if (char.attachItems) {
      for (const [wowAttachmentId, itemPath] of Object.entries(char.attachItems)) {
        const wowAttachment = model.wowAttachments.find((a) => a.wowAttachmentId === Number(wowAttachmentId));
        if (wowAttachment) {
          const itemMdl = await this.exportItem(itemPath.path)
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
          console.error(chalk.red(`Cannot find bone for wow attachment ${wowAttachmentId}`));
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
    if (char.base.type === "local") {
      return this.assetManager.parse(char.base.value, true).mdl;
    } else if (char.base.type === "wowhead" || char.base.type === "displayID") {
      const baseZam: ZamUrl = char.base.type === "wowhead"
        ? await parseWowheadUrlToZam(char.base.value)
        : { expansion: 'live', type: 'npc', displayId: Number(char.base.value) };
      if (baseZam.type !== 'npc') throw new Error("Expected npc zam url, got " + baseZam.type);
      const npcMeta = await fetchNpcMeta(baseZam);
      if (npcMeta.Model) {
        return await exportCreatureNpcAsMdl({ assetManager: this.assetManager, config: this.config }, baseZam);
      } else {
        return await exportCharacterNpcAsMdl({
          ctx: { assetManager: this.assetManager, config: this.config },
          zam: baseZam,
          keepCinematic: Boolean(char.keepCinematic),
          attackTag: char.attackTag,
        });
      }
    }
    throw new Error("Unknown base type");
  }

  private async exportItem(ref: Ref): Promise<MDL> {
    if (ref.type === "local") {
      return this.assetManager.parse(ref.value, true).mdl;
    } else if (ref.type === "wowhead" || ref.type === "displayID") {
      const zam: ZamUrl = ref.type === "wowhead"
        ? await parseWowheadUrlToZam(ref.value)
        : { expansion: 'live', type: 'item', displayId: Number(ref.value), slotId: null };
      if (zam.type !== 'item') throw new Error("Expected item zam url");
      return await exportZamItemAsMdl({
        ctx: { assetManager: this.assetManager, config: this.config },
        zam,
        targetRace: 0, // universal
        targetGender: 2, // universal
      });
    }
    throw new Error("Unknown item type");
  }
}

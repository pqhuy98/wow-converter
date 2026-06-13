/**
 * .skel file loader, ported from wow.export (src/js/3D/loaders/SKELLoader.js).
 * See: https://wowdev.wiki/M2/.skel
 */
import { getCasc } from '@/lib/wow/server/runtime';

import { BufferWrapper } from '../buffer';
import { ANIMLoader } from './anim-loader';
import * as M2Generics from './m2-generics';
import type {
  AnimFileIDEntry, M2Animation, M2Attachment, M2Bone,
} from './m2-types';

const CHUNK_SKB1 = 0x31424B53;
const CHUNK_SKPD = 0x44504B53;
const CHUNK_SKS1 = 0x31534B53;
const CHUNK_SKA1 = 0x31414B53;
const CHUNK_AFID = 0x44494641;
const CHUNK_BFID = 0x44494642;

export class SKELLoader {
  data: BufferWrapper;

  isLoaded = false;

  animFiles = new Map<number, BufferWrapper>();

  animFileIDs: AnimFileIDEntry[] = [];

  parent_skel_file_id?: number;

  boneOffset = 0;

  attachmentsOffset = 0;

  bones?: M2Bone[];

  globalLoops?: number[];

  animations: M2Animation[] = [];

  animationLookup?: number[];

  attachments?: M2Attachment[];

  attachmentLookup?: number[];

  boneFileIDs?: number[];

  constructor(data: BufferWrapper) {
    this.data = data;
  }

  /** Load the skeleton file. */
  load(): void {
    // Prevent multiple loading of the same file.
    if (this.isLoaded === true) return;

    while (this.data.remainingBytes > 0) {
      const chunkID = this.data.readUInt32LE();
      const chunkSize = this.data.readUInt32LE();
      const nextChunkPos = this.data.offset + chunkSize;

      switch (chunkID) {
        case CHUNK_SKA1: this.parse_chunk_ska1(); break;
        case CHUNK_SKB1: this.parse_chunk_skb1(); break;
        case CHUNK_SKPD: this.parse_chunk_skpd(); break;
        case CHUNK_SKS1: this.parse_chunk_sks1(); break;
        case CHUNK_AFID: this.parse_chunk_afid(chunkSize); break;
        case CHUNK_BFID: this.parse_chunk_bfid(chunkSize); break;
        default: break;
      }

      // Ensure that we start at the next chunk exactly.
      this.data.seek(nextChunkPos);
    }

    this.isLoaded = true;
  }

  private parse_chunk_skpd(): void {
    this.data.move(8); // _0x00[8]
    this.parent_skel_file_id = this.data.readUInt32LE();
    this.data.move(4); // _0x0c[4]
  }

  parse_chunk_skb1(useAnims = false): void {
    const data = this.data;
    const chunk_ofs = data.offset;
    this.boneOffset = data.offset;

    const bone_count = data.readUInt32LE();
    const bone_ofs = data.readUInt32LE();

    const base_ofs = data.offset;
    data.seek(chunk_ofs + bone_ofs);

    const bones: M2Bone[] = this.bones = Array(bone_count);
    for (let i = 0; i < bone_count; i++) {
      const bone: M2Bone = {
        boneID: data.readInt32LE(),
        flags: data.readUInt32LE(),
        parentBone: data.readInt16LE(),
        subMeshID: data.readUInt16LE(),
        boneNameCRC: data.readUInt32LE(),
        translation: M2Generics.read_m2_track<number[]>(data, chunk_ofs, 'float3', useAnims, this.animFiles),
        rotation: M2Generics.read_m2_track<number[]>(data, chunk_ofs, 'compquat', useAnims, this.animFiles),
        scale: M2Generics.read_m2_track<number[]>(data, chunk_ofs, 'float3', useAnims, this.animFiles),
        pivot: data.readFloatLE(3),
      };

      // Convert bone transformations coordinate system.
      const translations = bone.translation.values;
      const rotations = bone.rotation.values;
      const scale = bone.scale.values;
      const pivot = bone.pivot;

      for (let a = 0; a < translations.length; a++) {
        for (let j = 0; j < translations[a].length; j++) {
          const dx = translations[a][j][0];
          const dy = translations[a][j][1];
          const dz = translations[a][j][2];

          translations[a][j][0] = dx;
          translations[a][j][2] = dy * -1;
          translations[a][j][1] = dz;
        }
      }

      for (let a = 0; a < rotations.length; a++) {
        for (let j = 0; j < rotations[a].length; j++) {
          const dx = rotations[a][j][0];
          const dy = rotations[a][j][1];
          const dz = rotations[a][j][2];
          const dw = rotations[a][j][3];

          rotations[a][j][0] = dx;
          rotations[a][j][2] = dy * -1;
          rotations[a][j][1] = dz;
          rotations[a][j][3] = dw;
        }
      }

      for (let a = 0; a < scale.length; a++) {
        for (let j = 0; j < scale[a].length; j++) {
          const dx = scale[a][j][0];
          const dy = scale[a][j][1];
          const dz = scale[a][j][2];

          scale[a][j][0] = dx;
          scale[a][j][2] = dy;
          scale[a][j][1] = dz;
        }
      }

      {
        const pivotX = pivot[0];
        const pivotY = pivot[1];
        const pivotZ = pivot[2];
        pivot[0] = pivotX;
        pivot[2] = pivotY * -1;
        pivot[1] = pivotZ;
      }

      bones[i] = bone;
    }

    data.seek(base_ofs);
  }

  private parse_chunk_sks1(): void {
    // Global loops
    const chunk_ofs = this.data.offset;

    const globalLoopCount = this.data.readUInt32LE();
    const globalLoopOfs = this.data.readUInt32LE();

    let prevPos = this.data.offset;
    this.data.seek(globalLoopOfs + chunk_ofs);

    this.globalLoops = this.data.readInt16LE(globalLoopCount);

    this.data.seek(prevPos);

    // Sequences
    const animationCount = this.data.readUInt32LE();
    const animationOfs = this.data.readUInt32LE();

    prevPos = this.data.offset;
    this.data.seek(animationOfs + chunk_ofs);

    const animations: M2Animation[] = this.animations = new Array(animationCount);
    for (let i = 0; i < animationCount; i++) {
      animations[i] = {
        id: this.data.readUInt16LE(),
        variationIndex: this.data.readUInt16LE(),
        duration: this.data.readUInt32LE(),
        movespeed: this.data.readFloatLE(),
        flags: this.data.readUInt32LE(),
        frequency: this.data.readInt16LE(),
        padding: this.data.readUInt16LE(),
        replayMin: this.data.readUInt32LE(),
        replayMax: this.data.readUInt32LE(),
        blendTimeIn: this.data.readUInt16LE(),
        blendTimeOut: this.data.readUInt16LE(),
        boxPosMin: this.data.readFloatLE(3),
        boxPosMax: this.data.readFloatLE(3),
        boxRadius: this.data.readFloatLE(),
        variationNext: this.data.readInt16LE(),
        aliasNext: this.data.readUInt16LE(),
      };
    }

    this.data.seek(prevPos);

    // Sequence lookups
    const animationLookupCount = this.data.readUInt32LE();
    const animationLookupOfs = this.data.readUInt32LE();

    prevPos = this.data.offset;
    this.data.seek(animationLookupOfs + chunk_ofs);

    this.animationLookup = this.data.readInt16LE(animationLookupCount);

    this.data.seek(prevPos);

    // Unused spot (for now)
    this.data.move(8);
  }

  parse_chunk_ska1(useAnims = false): void {
    const data = this.data;
    const chunk_ofs = data.offset;
    this.attachmentsOffset = chunk_ofs;

    const attachmentCount = data.readUInt32LE();
    const attachmentOfs = data.readUInt32LE();
    const lookupCount = data.readUInt32LE();
    const lookupOfs = data.readUInt32LE();

    const base = data.offset;

    if (attachmentCount > 0 && attachmentOfs > 0) {
      data.seek(chunk_ofs + attachmentOfs);
      const entries: M2Attachment[] = this.attachments = new Array(attachmentCount);
      for (let i = 0; i < attachmentCount; i++) {
        const attachment: M2Attachment = {
          id: data.readUInt32LE(),
          bone: data.readUInt16LE(),
          unknown: data.readUInt16LE(),
          position: data.readFloatLE(3),
          animateAttached: M2Generics.read_m2_track(data, chunk_ofs, 'uint8', useAnims, this.animFiles),
        };

        // Match attachment position conversion used in M2 loader: x=x, y=z, z=-y
        const pos = attachment.position;
        const posX = pos[0];
        const posY = pos[1];
        const posZ = pos[2];
        pos[0] = posX;
        pos[2] = posY * -1;
        pos[1] = posZ;

        entries[i] = attachment;
      }
      // write('[SKEL] Parsed %d attachments from SKA1', attachmentCount);
    }

    if (lookupCount > 0 && lookupOfs > 0) {
      data.seek(chunk_ofs + lookupOfs);
      this.attachmentLookup = data.readUInt16LE(lookupCount);
    }

    data.seek(base);
  }

  /** Parse AFID chunk for .anim file data IDs. */
  private parse_chunk_afid(chunkSize: number): void {
    const entryCount = chunkSize / 8;
    const entries: AnimFileIDEntry[] = this.animFileIDs = new Array(entryCount);

    for (let i = 0; i < entryCount; i++) {
      entries[i] = {
        animID: this.data.readUInt16LE(),
        subAnimID: this.data.readUInt16LE(),
        fileDataID: this.data.readUInt32LE(),
      };
    }
  }

  /** Parse BFID chunk for .bone file data IDs. */
  private parse_chunk_bfid(chunkSize: number): void {
    this.boneFileIDs = this.data.readUInt32LE(chunkSize / 4);
  }

  async loadAnims(): Promise<void> {
    for (let i = 0; i < this.animations.length; i++) await this.loadAnimsForIndex(i, false);

    this.data.seek(this.boneOffset);
    this.parse_chunk_skb1(true);
  }

  async loadAnimsForIndex(animation_index: number, reparseBones = true): Promise<boolean> {
    if (this.animFiles.has(animation_index)) return true;

    let animation = this.animations[animation_index];

    if ((animation.flags & 0x40) === 0x40) {
      while ((animation.flags & 0x40) === 0x40) animation = this.animations[animation.aliasNext];
    }

    if ((animation.flags & 0x20) === 0x20) return false;

    for (const entry of this.animFileIDs) {
      if (entry.animID !== animation.id || entry.subAnimID !== animation.variationIndex) continue;

      const fileDataID = entry.fileDataID;
      if (fileDataID === 0) return false;

      // write('lazy load .anim for %d (%s) - %d', entry.animID, get_anim_name(entry.animID), entry.subAnimID);

      const loader = new ANIMLoader(await getCasc().getFile(fileDataID));
      loader.load(true);

      if (loader.skeletonBoneData !== undefined) this.animFiles.set(animation_index, BufferWrapper.from(loader.skeletonBoneData));
      else this.animFiles.set(animation_index, BufferWrapper.from(loader.animData!));

      if (reparseBones) {
        this.data.seek(this.boneOffset);
        this.parse_chunk_skb1(true);
      }

      return true;
    }

    return false;
  }
}

export default SKELLoader;

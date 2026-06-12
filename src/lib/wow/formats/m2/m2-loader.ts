/**
 * M2 model loader, ported from wow.export (src/js/3D/loaders/M2Loader.js).
 */
import { write } from '@/lib/wow/log';
import { getCasc } from '@/lib/wow/server/runtime';

import { BufferWrapper } from '../buffer';
import { constants } from '../constants';
import { ANIMLoader } from './anim-loader';
import { get_anim_name } from './anim-mapper';
import type { CAaBox, M2SplineKey, M2Track } from './m2-generics';
import * as M2Generics from './m2-generics';
import type {
  AnimFileIDEntry,
  M2Animation,
  M2Attachment,
  M2Bone,
  M2Camera,
  M2Color,
  M2Light,
  M2Material,
  M2ParticleEmitter,
  M2RibbonEmitter,
  M2TextureTransform,
} from './m2-types';
import { SKELLoader } from './skel-loader';
import { Skin } from './skin';
import { Texture } from './texture';

const CHUNK_SFID = 0x44494653;
const CHUNK_TXID = 0x44495854;
const CHUNK_SKID = 0x44494B53;
const CHUNK_BFID = 0x44494642;
const CHUNK_AFID = 0x44494641;

export class M2Loader {
  data: BufferWrapper;

  isLoaded = false;

  isAnimLoaded = false;

  animFiles = new Map<number, BufferWrapper>();

  animFileIDs: AnimFileIDEntry[] = [];

  version!: number;

  name!: string;

  flags!: number;

  viewCount!: number;

  md21Ofs!: number;

  skeletonFileID = 0;

  boneFileIDs?: number[];

  skins: Skin[] = [];

  lodSkins: Skin[] = [];

  textures: Texture[] = [];

  textureTypes: number[] = [];

  textureCombos: number[] = [];

  textureWeights: M2Track[] = [];

  textureTransforms: M2TextureTransform[] = [];

  textureTransformsLookup: number[] = [];

  transparencyLookup: number[] = [];

  replaceableTextureLookup: number[] = [];

  materials: M2Material[] = [];

  colors: M2Color[] = [];

  vertices: number[] = [];

  normals: number[] = [];

  uv: number[] = [];

  uv2: number[] = [];

  boneWeights: number[] = [];

  boneIndices: number[] = [];

  boundingBox!: CAaBox;

  boundingSphereRadius!: number;

  collisionBox!: CAaBox;

  collisionSphereRadius!: number;

  collisionIndices: number[] = [];

  collisionPositions: number[] = [];

  collisionNormals: number[] = [];

  animations: M2Animation[] = [];

  animationLookup: number[] = [];

  globalLoops: number[] = [];

  bones: M2Bone[] = [];

  attachments: M2Attachment[] = [];

  cameras?: M2Camera[];

  cameraLookup?: number[];

  lights?: M2Light[];

  ribbonEmitters?: M2RibbonEmitter[];

  particleEmitters?: M2ParticleEmitter[];

  constructor(data: BufferWrapper) {
    this.data = data;
  }

  /** Load the M2 model. */
  async load(): Promise<void> {
    // Prevent multiple loading of the same M2.
    if (this.isLoaded === true) return;

    while (this.data.remainingBytes > 0) {
      const chunkID = this.data.readUInt32LE();
      const chunkSize = this.data.readUInt32LE();
      const nextChunkPos = this.data.offset + chunkSize;

      switch (chunkID) {
        case constants.MAGIC.MD21: await this.parseChunk_MD21(); break;
        case CHUNK_SFID: this.parseChunk_SFID(chunkSize); break;
        case CHUNK_TXID: this.parseChunk_TXID(); break;
        case CHUNK_SKID: this.parseChunk_SKID(); break;
        case CHUNK_BFID: this.parseChunk_BFID(chunkSize); break;
        case CHUNK_AFID: this.parseChunk_AFID(chunkSize); break;
        default: break;
      }

      // Ensure that we start at the next chunk exactly.
      this.data.seek(nextChunkPos);
    }

    this.isLoaded = true;
  }

  /** Get a skin at a given index from this.skins. */
  async getSkin(index: number): Promise<Skin> {
    const skin = this.skins[index];
    if (!skin.isLoaded) await skin.load();

    return skin;
  }

  /**
   * Returns the internal array of Skin objects.
   * Note: Unlike getSkin(), this does not load any of the skins.
   */
  getSkinList(): Skin[] {
    return this.skins;
  }

  /** Load and apply .anim files to loaded M2 model. */
  async loadAnims(): Promise<void> {
    if (this.isAnimLoaded) return;
    for (let i = 0; i < this.animations.length; i++) await this.loadAnimsForIndex(i, false);

    this.data.seek(this.md21Ofs + 44);

    this.parseChunk_MD21_bones(this.md21Ofs, true);
    this.isAnimLoaded = true;
  }

  /**
   * Load .anim file for a specific animation index (lazy loading).
   * @returns true if loaded successfully, false otherwise
   */
  async loadAnimsForIndex(animationIndex: number, reparseBones = true): Promise<boolean> {
    // Already loaded for this animation index.
    if (this.animFiles.has(animationIndex)) return true;

    if (!this.animFileIDs || animationIndex >= this.animations.length) return false;

    let animation = this.animations[animationIndex];

    // Resolve animation alias chain.
    if ((animation.flags & 0x40) === 0x40) {
      while ((animation.flags & 0x40) === 0x40) animation = this.animations[animation.aliasNext];
    }

    // Animation data is embedded in the M2 file.
    if ((animation.flags & 0x20) === 0x20) {
      // write('Animation %s should be in M2, not loading .anim', get_anim_name(animation.id));
      return false;
    }

    // Find matching AFID entry.
    for (const entry of this.animFileIDs) {
      if (entry.animID !== animation.id || entry.subAnimID !== animation.variationIndex) continue;

      const fileDataID = entry.fileDataID;
      if (fileDataID === 0) {
        write('Skipping .anim loading for %s because it has no fileDataID', get_anim_name(entry.animID));
        return false;
      }

      // write('Loading .anim file for animation: %d (%s) - %d', entry.animID, get_anim_name(entry.animID), entry.subAnimID);

      let animIsChunked = false;
      if ((this.flags & 0x200000) === 0x200000 || this.skeletonFileID > 0) animIsChunked = true;

      try {
        const loader = new ANIMLoader(await getCasc().getFile(fileDataID));
        loader.load(animIsChunked);

        // Store .anim data.
        if (loader.skeletonBoneData !== undefined) this.animFiles.set(animationIndex, BufferWrapper.from(loader.skeletonBoneData));
        else this.animFiles.set(animationIndex, BufferWrapper.from(loader.animData!));

        // Re-parse bones to apply the newly loaded animation data.
        if (reparseBones) {
          this.data.seek(this.md21Ofs + 44);
          this.parseChunk_MD21_bones(this.md21Ofs, true);
        }
        return true;
      } catch (e) {
        write('Failed to load .anim file for animation %d: %s', animation.id, (e as Error).message);
        return false;
      }
    }

    write('No .anim file found for animation: %d (%s) - %d', animation.id, get_anim_name(animation.id), animation.variationIndex);
    return false;
  }

  /** Parse SFID chunk for skin file data IDs. */
  private parseChunk_SFID(chunkSize: number): void {
    if (this.viewCount === undefined) throw new Error('Cannot parse SFID chunk in M2 before MD21 chunk!');

    const lodSkinCount = (chunkSize / 4) - this.viewCount;
    this.skins = new Array(this.viewCount);
    this.lodSkins = new Array(lodSkinCount);

    for (let i = 0; i < this.viewCount; i++) this.skins[i] = new Skin(this.data.readUInt32LE());

    for (let i = 0; i < lodSkinCount; i++) this.lodSkins[i] = new Skin(this.data.readUInt32LE());
  }

  /** Parse TXID chunk for texture file data IDs. */
  private parseChunk_TXID(): void {
    if (this.textures === undefined) throw new Error('Cannot parse TXID chunk in M2 before MD21 chunk!');

    for (let i = 0, n = this.textures.length; i < n; i++) this.textures[i].fileDataID = this.data.readUInt32LE();
  }

  /** Parse SKID chunk for .skel file data ID. */
  private parseChunk_SKID(): void {
    this.skeletonFileID = this.data.readUInt32LE();
  }

  /** Parse BFID chunk for .bone file data IDs. */
  private parseChunk_BFID(chunkSize: number): void {
    this.boneFileIDs = this.data.readUInt32LE(chunkSize / 4);
  }

  /** Parse AFID chunk for animation file data IDs. */
  private parseChunk_AFID(chunkSize: number): void {
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

  /** Parse MD21 chunk. */
  private async parseChunk_MD21(): Promise<void> {
    const ofs = this.data.offset;

    const magic = this.data.readUInt32LE();
    if (magic !== constants.MAGIC.MD20) throw new Error(`Invalid M2 magic: ${magic}`);

    this.version = this.data.readUInt32LE();
    this.parseChunk_MD21_modelName(ofs);
    this.flags = this.data.readUInt32LE();
    this.parseChunk_MD21_globalLoops(ofs);
    this.parseChunk_MD21_animations(ofs);
    this.parseChunk_MD21_animationLookup(ofs);
    this.parseChunk_MD21_bones(ofs);
    this.data.move(8);
    this.parseChunk_MD21_vertices(ofs);
    this.viewCount = this.data.readUInt32LE();
    this.parseChunk_MD21_colors(ofs);
    this.parseChunk_MD21_textures(ofs);
    this.parseChunk_MD21_textureWeights(ofs);
    this.parseChunk_MD21_textureTransforms(ofs);
    this.parseChunk_MD21_replaceableTextureLookup(ofs);
    this.parseChunk_MD21_materials(ofs);
    this.data.move(2 * 4); // boneCombos
    this.parseChunk_MD21_textureCombos(ofs);
    this.data.move(8); // textureTransformBoneMap
    this.parseChunk_MD21_transparencyLookup(ofs);
    this.parseChunk_MD21_textureTransformLookup(ofs);
    this.parseChunk_MD21_collision(ofs);
    await this.parseChunk_MD21_attachments(ofs);
    this.data.move(8); // attachmentIndicesByID / attachment_lookup_table
    this.data.move(8); // events
    this.parseChunk_MD21_lights(ofs);
    this.parseChunk_MD21_cameras(ofs);
    this.parseChunk_MD21_camera_lookup(ofs);
    this.parseChunk_MD21_ribbon_emitters(ofs);
    this.parseChunk_MD21_particle_emitters(ofs);
  }

  /** Parse cameras from MD21 chunk. */
  private parseChunk_MD21_cameras(ofs: number): void {
    const count = this.data.readUInt32LE();
    const tableOfs = this.data.readUInt32LE();
    if (count === 0 || tableOfs === 0) return;

    const base = this.data.offset;
    this.data.seek(tableOfs + ofs);

    this.cameras = new Array(count);
    for (let i = 0; i < count; i++) {
      const type = this.data.readInt32LE();
      // Pre-Cata had a single float fov after type. Most modern M2s use tracked FoV at end.
      // We can't reliably branch by version here without additional flags; skip legacy fov.
      // Read clipping planes
      const far_clip = this.data.readFloatLE();
      const near_clip = this.data.readFloatLE();
      // Tracks (spline-key based)
      const positions = M2Generics.read_m2_spline_track(this.data, ofs, 'float3');
      const position_base = this.data.readFloatLE(3);
      const target_position = M2Generics.read_m2_spline_track(this.data, ofs, 'float3');
      const target_position_base = this.data.readFloatLE(3);
      const roll = M2Generics.read_m2_spline_track(this.data, ofs, 'float');
      // Cata+: FoV track
      let FoV: M2Track<M2SplineKey> | null = null;
      try {
        FoV = M2Generics.read_m2_spline_track(this.data, ofs, 'float');
      } catch (e) {
        FoV = null;
      }

      // Coordinate conversion for bases, like bones/attachments: x=x, y=z, z=-y
      const convPosBase = [position_base[0], position_base[2], position_base[1] * -1];
      const convTargetBase = [target_position_base[0], target_position_base[2], target_position_base[1] * -1];

      this.cameras[i] = {
        type,
        far_clip,
        near_clip,
        positions,
        position_base: convPosBase,
        target_position,
        target_position_base: convTargetBase,
        roll,
        FoV,
      };
    }

    this.data.seek(base);
  }

  /** Parse camera lookup table from MD21 chunk. */
  private parseChunk_MD21_camera_lookup(ofs: number): void {
    const count = this.data.readUInt32LE();
    const tableOfs = this.data.readUInt32LE();
    if (count === 0 || tableOfs === 0) return;

    const base = this.data.offset;
    this.data.seek(tableOfs + ofs);
    this.cameraLookup = this.data.readUInt16LE(count);
    this.data.seek(base);
  }

  parseChunk_MD21_bones(ofs: number, useAnims = false): void {
    const data = this.data;
    const boneCount = data.readUInt32LE();
    const boneOfs = data.readUInt32LE();

    const base = data.offset;
    data.seek(boneOfs + ofs);

    this.md21Ofs = ofs;

    const bones: M2Bone[] = this.bones = Array(boneCount);
    for (let i = 0; i < boneCount; i++) {
      const bone: M2Bone = {
        boneID: data.readInt32LE(),
        flags: data.readUInt32LE(),
        parentBone: data.readInt16LE(),
        subMeshID: data.readUInt16LE(),
        boneNameCRC: data.readUInt32LE(),
        translation: M2Generics.read_m2_track<number[]>(data, ofs, 'float3', useAnims, this.animFiles),
        rotation: M2Generics.read_m2_track<number[]>(data, ofs, 'compquat', useAnims, this.animFiles),
        scale: M2Generics.read_m2_track<number[]>(data, ofs, 'float3', useAnims, this.animFiles),
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

    data.seek(base);
  }

  /** Parse collision data from an MD21 chunk. */
  private parseChunk_MD21_collision(ofs: number): void {
    // Parse collision boxes before the full collision chunk.
    this.boundingBox = M2Generics.read_caa_bb(this.data);
    this.boundingSphereRadius = this.data.readFloatLE();
    this.collisionBox = M2Generics.read_caa_bb(this.data);
    this.collisionSphereRadius = this.data.readFloatLE();

    const indicesCount = this.data.readUInt32LE();
    const indicesOfs = this.data.readUInt32LE();

    const positionsCount = this.data.readUInt32LE();
    const positionsOfs = this.data.readUInt32LE();

    const normalsCount = this.data.readUInt32LE();
    const normalsOfs = this.data.readUInt32LE();

    const base = this.data.offset;

    // indices
    this.data.seek(indicesOfs + ofs);
    this.collisionIndices = this.data.readUInt16LE(indicesCount);

    // Positions
    this.data.seek(positionsOfs + ofs);
    const positions: number[] = this.collisionPositions = new Array(positionsCount * 3);
    for (let i = 0; i < positionsCount; i++) {
      const index = i * 3;

      positions[index] = this.data.readFloatLE();
      positions[index + 2] = this.data.readFloatLE() * -1;
      positions[index + 1] = this.data.readFloatLE();
    }

    // Normals
    this.data.seek(normalsOfs + ofs);
    const normals: number[] = this.collisionNormals = new Array(normalsCount * 3);
    for (let i = 0; i < normalsCount; i++) {
      const index = i * 3;

      normals[index] = this.data.readFloatLE();
      normals[index + 2] = this.data.readFloatLE() * -1;
      normals[index + 1] = this.data.readFloatLE();
    }

    this.data.seek(base);
  }

  /** Parse attachments data from an MD21 chunk. */
  private async parseChunk_MD21_attachments(ofs: number): Promise<void> {
    const attachmentCount = this.data.readUInt32LE();
    const attachmentOffset = this.data.readUInt32LE();

    // Check if attachments are valid
    if (attachmentCount > 0 && attachmentOffset > 0) {
      const base = this.data.offset;
      this.data.seek(attachmentOffset + ofs);

      const entries: M2Attachment[] = this.attachments = new Array(attachmentCount);
      for (let i = 0; i < attachmentCount; i++) {
        entries[i] = {
          id: this.data.readUInt32LE(),
          bone: this.data.readUInt16LE(),
          unknown: this.data.readUInt16LE(),
          position: this.data.readFloatLE(3),
          animateAttached: M2Generics.read_m2_track(this.data, this.md21Ofs, 'uint8'),
        };

        // Convert attachment position coordinate system
        // According to wowdev, attachments use the same coordinate system as bones
        const pos = entries[i].position;
        const posX = pos[0];
        const posY = pos[1];
        const posZ = pos[2];
        pos[0] = posX;
        pos[2] = posY * -1;
        pos[1] = posZ;
      }

      // write('[M2] Parsed %d MD21 attachments', attachmentCount);
      this.data.seek(base);
    }

    // Fallback: if no attachments parsed from MD21 and a skel is referenced, try SKEL's SKA1
    if ((!this.attachments || this.attachments.length === 0) && this.skeletonFileID > 0) {
      try {
        write('[M2] No MD21 attachments found. Attempting SKEL fallback. skeletonFileID=%d', this.skeletonFileID);
        const skelData = await getCasc().getFile(this.skeletonFileID);
        const skel = new SKELLoader(skelData);
        skel.load();
        // If SKEL has attachments, adopt them directly
        if (skel.attachments && skel.attachments.length > 0) {
          this.attachments = skel.attachments;
          write('[M2] Adopted %d attachments from SKEL', this.attachments.length);
        }
      } catch (e) {
        write('Failed to read attachments from SKEL: %s', e);
      }
    }
  }

  private parseChunk_MD21_ribbon_emitters(ofs: number): void {
    const ribbonEmitterCount = this.data.readUInt32LE();
    const ribbonEmitterOfs = this.data.readUInt32LE();
    // https://wowdev.wiki/M2#Ribbon_emitters
    if (ribbonEmitterCount > 0 && ribbonEmitterOfs > 0) {
      const base = this.data.offset;
      this.data.seek(ribbonEmitterOfs + ofs);

      this.ribbonEmitters = new Array(ribbonEmitterCount);
      for (let i = 0; i < ribbonEmitterCount; i++) {
        const ribbonId = this.data.readUInt32LE();
        const boneIndex = this.data.readUInt32LE();
        const position = this.data.readFloatLE(3);
        const textureIndices = M2Generics.read_m2_array(this.data, this.md21Ofs, 'uint16');
        const materialIndices = M2Generics.read_m2_array(this.data, this.md21Ofs, 'uint16');
        const colorTrack = M2Generics.read_m2_track(this.data, this.md21Ofs, 'float3');
        const alphaTrack = M2Generics.read_m2_track(this.data, this.md21Ofs, 'int16');
        const heightAboveTrack = M2Generics.read_m2_track(this.data, this.md21Ofs, 'float');
        const heightBelowTrack = M2Generics.read_m2_track(this.data, this.md21Ofs, 'float');
        const edgesPerSecond = this.data.readFloatLE();
        const edgeLifetime = this.data.readFloatLE();
        const gravity = this.data.readFloatLE();
        const textureRows = this.data.readUInt16LE();
        const textureCols = this.data.readUInt16LE();
        const texSlotTrack = M2Generics.read_m2_track(this.data, this.md21Ofs, 'uint16');
        const visibilityTrack = M2Generics.read_m2_track(this.data, this.md21Ofs, 'uint8');
        const priorityPlane = this.data.readInt16LE();
        const ribbonColorIndex = this.data.readInt8();
        const textureTransformLookupIndex = this.data.readInt8();

        this.ribbonEmitters[i] = {
          ribbonId,
          boneIndex,
          position,
          textureIndices,
          materialIndices,
          colorTrack,
          alphaTrack,
          heightAboveTrack,
          heightBelowTrack,
          edgesPerSecond,
          edgeLifetime,
          gravity,
          textureRows,
          textureCols,
          texSlotTrack,
          visibilityTrack,
          priorityPlane,
          ribbonColorIndex,
          textureTransformLookupIndex,
        };
      }
      this.data.seek(base);
    }
  }

  private parseChunk_MD21_particle_emitters(ofs: number): void {
    const count = this.data.readUInt32LE();
    const tableOfs = this.data.readUInt32LE();
    if (count === 0 || tableOfs === 0) return;

    const base = this.data.offset;
    this.data.seek(tableOfs + ofs);

    this.particleEmitters = new Array(count);
    for (let i = 0; i < count; i++) {
      // Read fixed part of M2ParticleOld
      const particleId = this.data.readUInt32LE();
      const flags = this.data.readUInt32LE();
      const position = this.data.readFloatLE(3);
      const bone = this.data.readUInt16LE();
      // Packed texture field (uint16) in early versions; treat as a single texture index for metadata
      const texturePacked = this.data.readUInt16LE();
      const geometryModelNameLen = this.data.readUInt32LE();
      const geometryModelNameOfs = this.data.readUInt32LE();
      const recursionModelNameLen = this.data.readUInt32LE();
      const recursionModelNameOfs = this.data.readUInt32LE();
      const blendingType = this.data.readUInt8();
      const emitterType = this.data.readUInt8();
      const particleColorIndex = this.data.readUInt16LE();
      // multiTextureParamX[2]
      const multiTextureParamX0 = this.data.readUInt8();
      const multiTextureParamX1 = this.data.readUInt8();
      const textureTileRotation = this.data.readUInt16LE();
      const rows = this.data.readUInt16LE();
      const cols = this.data.readUInt16LE();

      // Tracks
      const emissionSpeed = M2Generics.read_m2_track(this.data, ofs, 'float');
      const speedVariation = M2Generics.read_m2_track(this.data, ofs, 'float');
      const verticalRange = M2Generics.read_m2_track(this.data, ofs, 'float');
      const horizontalRange = M2Generics.read_m2_track(this.data, ofs, 'float');
      // gravity track may be either float or compressed; read as uint32 because it's not float in some cases, but a compressed vector3
      const gravity = M2Generics.read_m2_track(this.data, ofs, 'uint32');
      const lifespan = M2Generics.read_m2_track(this.data, ofs, 'float');
      const lifespanVary = this.data.readFloatLE();
      const emissionRate = M2Generics.read_m2_track(this.data, ofs, 'float');
      const emissionRateVary = this.data.readFloatLE();
      const emissionAreaLength = M2Generics.read_m2_track(this.data, ofs, 'float');
      const emissionAreaWidth = M2Generics.read_m2_track(this.data, ofs, 'float');
      const zSource = M2Generics.read_m2_track(this.data, ofs, 'float');

      // Part tracks (age-based): color (float3), alpha (int16 fixed), scale (float2), head/tail (uint16)
      const colorTrack = M2Generics.read_m2_part_track(this.data, ofs, 'float3');
      const alphaTrack = M2Generics.read_m2_part_track(this.data, ofs, 'int16');
      const scaleTrack = M2Generics.read_m2_part_track(this.data, ofs, 'float2');
      const scaleVaryX = this.data.readFloatLE();
      const scaleVaryY = this.data.readFloatLE();
      const headCellTrack = M2Generics.read_m2_part_track(this.data, ofs, 'uint16');
      const tailCellTrack = M2Generics.read_m2_part_track(this.data, ofs, 'uint16');
      const tailLength = this.data.readFloatLE();
      const twinkleSpeed = this.data.readFloatLE();
      const twinklePercent = this.data.readFloatLE();
      // twinkleScale (range)
      const twinkleScaleMin = this.data.readFloatLE();
      const twinkleScaleMax = this.data.readFloatLE();
      const burstMultiplier = this.data.readFloatLE();
      const drag = this.data.readFloatLE();
      const baseSpin = this.data.readFloatLE();
      const baseSpinVary = this.data.readFloatLE();
      const spin = this.data.readFloatLE();
      const spinVary = this.data.readFloatLE();
      // tumble M2Box: two C3Vectors (min/max) -> 6 floats
      const tumble = this.data.readFloatLE(6);
      const windVector = this.data.readFloatLE(3);
      const windTime = this.data.readFloatLE();
      const followSpeed1 = this.data.readFloatLE();
      const followScale1 = this.data.readFloatLE();
      const followSpeed2 = this.data.readFloatLE();
      const followScale2 = this.data.readFloatLE();

      // spline points (array of C3Vector) - header read, contents skipped (matches wow.export)
      const splinePointsCount = this.data.readUInt32LE();
      const splinePointsOfs = this.data.readUInt32LE();
      const splinePoints: number[][] = [];
      void splinePointsCount; void splinePointsOfs;

      // enabledIn M2Track<uint8> (bool), use as visibility hint
      const enabledIn = M2Generics.read_m2_track(this.data, ofs, 'uint8');

      // Cata+: two arrays appended after old: multiTextureParam0[2], multiTextureParam1[2]
      const read_fp69_vec2 = (): number[] => {
        const x = this.data.readUInt16LE();
        const y = this.data.readUInt16LE();
        // fp_6_9 approx: divide by 2^9
        return [x / 512.0, y / 512.0];
      };
      let multiTextureParam0: number[][] | null = null;
      let multiTextureParam1: number[][] | null = null;
      try {
        multiTextureParam0 = [read_fp69_vec2(), read_fp69_vec2()];
        multiTextureParam1 = [read_fp69_vec2(), read_fp69_vec2()];
      } catch (e) {
        // Older clients may not have these fields; ignore if reading fails
      }

      // Names not resolved (matches wow.export behaviour)
      const geometryModel = '';
      const recursionModel = '';
      void geometryModelNameLen; void geometryModelNameOfs;
      void recursionModelNameLen; void recursionModelNameOfs;

      // Coordinate conversion for position (match attachments/bones conversion): x=x, y=z, z=-y
      const convPos = [position[0], position[2], position[1] * -1];

      this.particleEmitters[i] = {
        particleId,
        flags,
        position: convPos,
        bone,
        texturePacked,
        geometryModel,
        recursionModel,
        blendingType,
        emitterType,
        particleColorIndex,
        multiTextureParamX: [multiTextureParamX0, multiTextureParamX1],
        textureTileRotation,
        textureRows: rows,
        textureCols: cols,
        emissionSpeed,
        speedVariation,
        verticalRange,
        horizontalRange,
        gravity,
        lifespan,
        lifespanVary,
        emissionRate,
        emissionRateVary,
        emissionAreaLength,
        emissionAreaWidth,
        zSource,
        colorTrack,
        alphaTrack,
        scaleTrack,
        scaleVary: [scaleVaryX, scaleVaryY],
        headCellTrack,
        tailCellTrack,
        tailLength,
        twinkleSpeed,
        twinklePercent,
        twinkleScale: { min: twinkleScaleMin, max: twinkleScaleMax },
        burstMultiplier,
        drag,
        baseSpin,
        baseSpinVary,
        spin,
        spinVary,
        tumble,
        windVector,
        windTime,
        followSpeed1,
        followScale1,
        followSpeed2,
        followScale2,
        multiTextureParam0,
        multiTextureParam1,
        splinePoints,
        enabledIn,
      };
    }

    this.data.seek(base);
  }

  /**
   * Parse lights from MD21 chunk.
   * See wowdev wiki: M2Light
   */
  private parseChunk_MD21_lights(ofs: number): void {
    const count = this.data.readUInt32LE();
    const tableOfs = this.data.readUInt32LE();
    if (count === 0 || tableOfs === 0) return;

    const base = this.data.offset;
    this.data.seek(tableOfs + ofs);

    this.lights = new Array(count);
    for (let i = 0; i < count; i++) {
      const type = this.data.readUInt16LE();
      const bone = this.data.readInt16LE();
      const position = this.data.readFloatLE(3);
      const ambient_color = M2Generics.read_m2_track(this.data, ofs, 'float3');
      const ambient_intensity = M2Generics.read_m2_track(this.data, ofs, 'float');
      const diffuse_color = M2Generics.read_m2_track(this.data, ofs, 'float3');
      const diffuse_intensity = M2Generics.read_m2_track(this.data, ofs, 'float');
      const attenuation_start = M2Generics.read_m2_track(this.data, ofs, 'float');
      const attenuation_end = M2Generics.read_m2_track(this.data, ofs, 'float');
      const visibility = M2Generics.read_m2_track(this.data, ofs, 'uint8');

      // Coordinate conversion like bones/attachments: x=x, y=z, z=-y
      const convPos = [position[0], position[2], position[1] * -1];

      this.lights[i] = {
        type,
        bone,
        position: convPos,
        ambient_color,
        ambient_intensity,
        diffuse_color,
        diffuse_intensity,
        attenuation_start,
        attenuation_end,
        visibility,
      };
    }

    this.data.seek(base);
  }

  /** Parse replaceable texture lookups from an MD21 chunk. */
  private parseChunk_MD21_replaceableTextureLookup(ofs: number): void {
    const lookupCount = this.data.readUInt32LE();
    const lookupOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(lookupOfs + ofs);

    this.replaceableTextureLookup = this.data.readInt16LE(lookupCount);

    this.data.seek(base);
  }

  /** Parse material meta-data from an MD21 chunk. */
  private parseChunk_MD21_materials(ofs: number): void {
    const materialCount = this.data.readUInt32LE();
    const materialOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(materialOfs + ofs);

    this.materials = new Array(materialCount);
    for (let i = 0; i < materialCount; i++) this.materials[i] = { flags: this.data.readUInt16LE(), blendingMode: this.data.readUInt16LE() };

    this.data.seek(base);
  }

  /** Parse the model name from an MD21 chunk. */
  private parseChunk_MD21_modelName(ofs: number): void {
    const modelNameLength = this.data.readUInt32LE();
    const modelNameOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(modelNameOfs + ofs);

    // Always followed by single 0x0 character, -1 to trim).
    this.data.seek(modelNameOfs + ofs);
    this.name = this.data.readString(modelNameLength - 1);

    this.data.seek(base);
  }

  /** Parse vertices from an MD21 chunk. */
  private parseChunk_MD21_vertices(ofs: number): void {
    const verticesCount = this.data.readUInt32LE();
    const verticesOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(verticesOfs + ofs);

    // Read vertices.
    const vertices: number[] = this.vertices = new Array(verticesCount * 3);
    const normals: number[] = this.normals = new Array(verticesCount * 3);
    const uv: number[] = this.uv = new Array(verticesCount * 2);
    const uv2: number[] = this.uv2 = new Array(verticesCount * 2);
    const boneWeights: number[] = this.boneWeights = Array(verticesCount * 4);
    const boneIndices: number[] = this.boneIndices = Array(verticesCount * 4);

    for (let i = 0; i < verticesCount; i++) {
      vertices[i * 3] = this.data.readFloatLE();
      vertices[i * 3 + 2] = this.data.readFloatLE() * -1;
      vertices[i * 3 + 1] = this.data.readFloatLE();

      for (let x = 0; x < 4; x++) boneWeights[i * 4 + x] = this.data.readUInt8();

      for (let x = 0; x < 4; x++) boneIndices[i * 4 + x] = this.data.readUInt8();

      normals[i * 3] = this.data.readFloatLE();
      normals[i * 3 + 2] = this.data.readFloatLE() * -1;
      normals[i * 3 + 1] = this.data.readFloatLE();

      uv[i * 2] = this.data.readFloatLE();
      uv[i * 2 + 1] = (this.data.readFloatLE() - 1) * -1;

      uv2[i * 2] = this.data.readFloatLE();
      uv2[i * 2 + 1] = (this.data.readFloatLE() - 1) * -1;
    }

    this.data.seek(base);
  }

  /** Parse texture transformation definitions from an MD21 chunk. */
  private parseChunk_MD21_textureTransforms(ofs: number): void {
    const transformCount = this.data.readUInt32LE();
    const transformOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(transformOfs + ofs);

    const transforms: M2TextureTransform[] = this.textureTransforms = new Array(transformCount);
    for (let i = 0; i < transformCount; i++) {
      transforms[i] = {
        translation: M2Generics.read_m2_track(this.data, this.md21Ofs, 'float3'),
        rotation: M2Generics.read_m2_track(this.data, this.md21Ofs, 'float4'),
        scaling: M2Generics.read_m2_track(this.data, this.md21Ofs, 'float3'),
      };
    }

    this.data.seek(base);
  }

  /** Parse texture transform lookup table from an MD21 chunk. */
  private parseChunk_MD21_textureTransformLookup(ofs: number): void {
    const entryCount = this.data.readUInt32LE();
    const entryOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(entryOfs + ofs);

    const entries: number[] = this.textureTransformsLookup = new Array(entryCount);
    for (let i = 0; i < entryCount; i++) entries[i] = this.data.readUInt16LE();

    this.data.seek(base);
  }

  /** Parse transparency lookup table from an MD21 chunk. */
  private parseChunk_MD21_transparencyLookup(ofs: number): void {
    const entryCount = this.data.readUInt32LE();
    const entryOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(entryOfs + ofs);

    const entries: number[] = this.transparencyLookup = new Array(entryCount);
    for (let i = 0; i < entryCount; i++) entries[i] = this.data.readUInt16LE();

    this.data.seek(base);
  }

  /** Parse global transparency weights from an MD21 chunk. */
  private parseChunk_MD21_textureWeights(ofs: number): void {
    const weightCount = this.data.readUInt32LE();
    const weightOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(weightOfs + ofs);

    const weights: M2Track[] = this.textureWeights = new Array(weightCount);
    for (let i = 0; i < weightCount; i++) weights[i] = M2Generics.read_m2_track(this.data, this.md21Ofs, 'int16');

    this.data.seek(base);
  }

  /** Parse color/transparency data from an MD21 chunk. */
  private parseChunk_MD21_colors(ofs: number): void {
    const colorsCount = this.data.readUInt32LE();
    const colorsOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(colorsOfs + ofs);

    const colors: M2Color[] = this.colors = new Array(colorsCount);
    for (let i = 0; i < colorsCount; i++) {
      colors[i] = {
        color: M2Generics.read_m2_track(this.data, this.md21Ofs, 'float3'),
        alpha: M2Generics.read_m2_track(this.data, this.md21Ofs, 'int16'),
      };
    }

    this.data.seek(base);
  }

  /** Parse textures from an MD21 chunk. */
  private parseChunk_MD21_textures(ofs: number): void {
    const texturesCount = this.data.readUInt32LE();
    const texturesOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(texturesOfs + ofs);

    // Read textures.
    const textures: Texture[] = this.textures = new Array(texturesCount);
    const textureTypes: number[] = this.textureTypes = new Array(texturesCount);

    for (let i = 0; i < texturesCount; i++) {
      const textureType = textureTypes[i] = this.data.readUInt32LE();
      const texture = new Texture(this.data.readUInt32LE());

      const nameLength = this.data.readUInt32LE();
      const nameOfs = this.data.readUInt32LE();

      // Check if texture has a filename (legacy).
      if (textureType === 0 && nameOfs > 0) {
        const pos = this.data.offset;

        this.data.seek(nameOfs);
        const fileName = this.data.readString(nameLength);
        fileName.replace('\0', ''); // Remove NULL characters.

        if (fileName.length > 0) texture.setFileName(fileName);

        this.data.seek(pos);
      }

      textures[i] = texture;
    }

    this.data.seek(base);
  }

  /** Parse texture combos from an MD21 chunk. */
  private parseChunk_MD21_textureCombos(ofs: number): void {
    const textureComboCount = this.data.readUInt32LE();
    const textureComboOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(textureComboOfs + ofs);
    this.textureCombos = this.data.readUInt16LE(textureComboCount);
    this.data.seek(base);
  }

  /** Parse animations. */
  private parseChunk_MD21_animations(ofs: number): void {
    const animationCount = this.data.readUInt32LE();
    const animationOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(animationOfs + ofs);

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

    this.data.seek(base);
  }

  /** Parse animation lookup. */
  private parseChunk_MD21_animationLookup(ofs: number): void {
    const animationLookupCount = this.data.readUInt32LE();
    const animationLookupOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(animationLookupOfs + ofs);

    this.animationLookup = this.data.readInt16LE(animationLookupCount);

    this.data.seek(base);
  }

  /** Parse global loops. */
  private parseChunk_MD21_globalLoops(ofs: number): void {
    const globalLoopCount = this.data.readUInt32LE();
    const globalLoopOfs = this.data.readUInt32LE();

    const base = this.data.offset;
    this.data.seek(globalLoopOfs + ofs);

    this.globalLoops = this.data.readInt16LE(globalLoopCount);

    this.data.seek(base);
  }
}

export default M2Loader;

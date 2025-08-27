import chalk from 'chalk';
import { readFileSync } from 'fs';
import _ from 'lodash';
import { dirname, join, relative } from 'path';

import { Animation, AnimationOrStatic, AnimationType } from '@/lib/formats/mdl/components/animation';
import { Geoset } from '@/lib/formats/mdl/components/geoset';
import { GlobalSequence } from '@/lib/formats/mdl/components/global-sequence';
import { Material } from '@/lib/formats/mdl/components/material';
import { Light, LightType } from '@/lib/formats/mdl/components/node/light';
import {
  FilterMode as WC3FilterMode, HeadOrTail as WC3HeadOrTail, ParticleEmitter2, ParticleEmitter2Flag,
} from '@/lib/formats/mdl/components/node/particle-emitter-2';
import { Texture } from '@/lib/formats/mdl/components/texture';
import { TextureAnim } from '@/lib/formats/mdl/components/texture-anim';
import { MDL } from '@/lib/formats/mdl/mdl';
import { degrees } from '@/lib/math/rotation';
import { V3 } from '@/lib/math/vector';

import { BlizzardNull } from '../../constants';
import { Config } from '../../global-config';
import { QuaternionRotation, Vector3 } from '../../math/common';
import { AnimationFile } from '../animation/animation';
import { getLayerFilterMode, wowToWc3Interpolation } from '../utils';

namespace Data {
  export interface Texture {
    fileNameInternal?: string
    fileNameExternal?: string
    mtlName: string
    flags: number
    fileDataID: number
  }

  export interface Material {
    flags: number
    blendingMode: number
  }

  export interface AnimFileId {
    animID: number
    subAnimID: number
    fileDataID: number
  }

  // Generic animation track
  // - timestamps/values are 2D array, first dimension is per animation, second dimension is per keyframe
  export interface M2Track<T> {
    globalSeq: number
    interpolation: number
    timestamps: number[][]
    values: T[][]
  }

  // Age-based track over a particle's lifetime
  // - timestamps are fixed16 fractions: 0..32767 maps to 0..100% of lifespan linearly
  // - values are looked up by agePercent = clamp(ageSeconds / lifespanSeconds, 0..1)
  // - interpolation is linear between the given keys
  export interface PartTrack<T> {
    timestamps: number[]
    values: T[]
  }

  export interface Color {
    color: M2Track<Vector3>
    alpha: M2Track<number>
  }

  type Translation = M2Track<Vector3>
  type Rotation = M2Track<QuaternionRotation>
  type Scaling = M2Track<Vector3>

  export interface TextureTransform {
    translation: Translation
    rotation: Rotation
    scaling: Scaling
  }

  export interface LightMeta {
    type: number
    bone: number
    position: Vector3
    ambient_color: M2Track<Vector3>
    ambient_intensity: M2Track<number>
    diffuse_color: M2Track<Vector3>
    diffuse_intensity: M2Track<number>
    attenuation_start: M2Track<number>
    attenuation_end: M2Track<number>
    visibility: M2Track<number>
  }

  // Coordinate system and units
  // - Coordinates: Z is up; XY is the ground plane. Positions are already converted to [x, z, -y].
  // - Angles: radians.
  // - Time: seconds unless noted.
  export interface ParticleEmitter {
    particleId: number // identifier from the model; kept for reference

    flags: number // behavior switches used by the renderer (e.g. head/tail on/off, model-space, random start cell)

    position: Vector3 // emitter pivot relative to its bone, in [x, z, -y] space
    bone: number // bone index to attach the emitter to

    // Blending mode (0..7):
    //   0 Opaque (no blending, depth write on, alpha test off)
    //   1 AlphaKey (alpha test at ~0.5 threshold, no blending)
    //   2 Alpha (standard srcAlpha, oneMinusSrcAlpha)
    //   3 Add(NoAlpha)
    //   4 Additive (srcAlpha, one)
    //   5 Modulate
    //   6 Modulate2x
    //   7 BlendAdd
    blendingType: number
    emitterType: number // 1 = rectangle area, 2 = spherical shell, 3 = spline path, 4 = bone
    particleColorIndex: number // 0 or 11..13 to use model-provided Start/Mid/End color sets

    // Texture grid on a single image (sprite sheet)

    // Texture selection:
    // - Single-texture: texturePacked is an index into the model's textures array.
    // - Multi-texture (Cata+): the same 16-bit field packs three indices:
    //     t0 = texturePacked & 0x1F; t1 = (texturePacked >> 5) & 0x1F; t2 = (texturePacked >> 10) & 0x1F
    //   These select the primary and the two extra layer textures from the model's textures array.
    //   This metadata leaves them packed; your renderer can unpack as above when multi-texture is active.
    // Detecting multi-texture in use:
    // - Prefer flags bit: (flags & 0x10000000) !== 0 indicates multi-texturing.
    // - Alternatively, presence of multiTextureParam0/multiTextureParam1 implies multi-texture support on this emitter.
    texturePacked: number // older files: single texture index on the model; multi-texture: 3 indices packed into 16 bits

    textureRows: number // number of rows in the texture grid
    textureCols: number // number of columns in the texture grid

    // Motion and emission
    emissionSpeed: M2Track<number> // initial speed of spawned particles (units per second)
    speedVariation: M2Track<number> // random variation applied to the speed (per particle and/or burst)
    // Direction ranges (radians):
    // - Rectangle (1): verticalRange = max polar tilt from +Z; horizontalRange = azimuth around +Z.
    // - Sphere (2): verticalRange = max elevation of the spawn point; horizontalRange = azimuth around +Z.
    verticalRange: M2Track<number>
    horizontalRange: M2Track<number>

    // Gravity: downward acceleration magnitude along Z (units per second^2). Positive values accelerate toward -Z.
    // if flags & 0x8000000 then gravity is a vector3 compressed to float, otherwise it's a scalar
    gravity: M2Track<number>
    lifespan: M2Track<number> // seconds each particle lives
    emissionRate: M2Track<number> // particles per second

    // Emitter size:
    // - Rectangle: left/right (local X) = emissionAreaLength; forward/backward (local Y) = emissionAreaWidth.
    //   Spawn position is uniform over the rectangle area.
    // - Sphere: inner radius = emissionAreaLength; outer radius = emissionAreaWidth (uniform in shell).
    emissionAreaWidth: M2Track<number>
    emissionAreaLength: M2Track<number>

    // Initial direction override:
    // If zSource > 0, set initial direction dir = normalize(spawnPos - [0,0,zSource]),
    // then velocity = dir * emissionSpeed. Otherwise:
    //   Rectangle: sample direction by polar in [0, verticalRange], azimuth in [0, horizontalRange].
    //   Sphere:    if not forced-up, initial direction points outward from the sphere.
    zSource: M2Track<number>

    tailLength: number // seconds of recent motion to show when drawing a trail
    enabledIn: M2Track<number> // optional on/off track used as a visibility hint per animation

    // Age-based appearance (ageSeconds starts at 0; agePercent = ageSeconds / lifespanSeconds)
    colorTrack: PartTrack<Vector3> // RGB 0..255 per key; renderer divides by 255
    alphaTrack: PartTrack<number> // fixed16 alpha 0..32767 per key; renderer divides by 32767
    scaleTrack: PartTrack<[number, number]> // quad width/height multipliers over life

    // Selecting a cell on the texture grid (one frame on the sprite sheet):
    // Given rows, cols, and a head/tail index:
    //   total = rows * cols; require 0 <= index < total
    //   col = index % cols; row = Math.floor(index / cols)
    //   uvMin = [col/cols, row/rows], uvMax = [(col+1)/cols, (row+1)/rows]
    // The renderer picks that UV rect for the corresponding quad.
    // Additionally, per-emitter a 16-bit random offset may be added (and masked by total-1) to vary start cells,
    // and if no headCellTrack is present but a model flag requests it, the starting cell is chosen randomly.
    headCellTrack: PartTrack<number>
    tailCellTrack: PartTrack<number>

    // Multi-texture (Cata+): two extra overlaid textures controlled by scroll/tiling parameters.
    // These do not select images; they control scrolling/tiling of the extra layers.
    // - multiTextureParam0: base scroll rates (dU/dt, dV/dt) for extra textures 1 and 2.
    // - multiTextureParam1: extra randomizable scroll rates (dU/dt, dV/dt) for extra textures 1 and 2.
    //   Per particle at spawn: scrollRate = param0 + rand[0..1] * param1; UV offset accumulates each frame: uv += scrollRate * dt.
    // - multiTextureParamX: per-texture tiling factor (packed byte). Convert to float scale by:
    //     scale = (byte & 31) / 32 + (byte >> 5)
    // To use: unpack t0/t1/t2 from texturePacked (see above) to pick three textures; then apply these scroll/tiling params
    // to the two extra layers (t1, t2) when sampling.
    multiTextureParam0: [[number, number], [number, number]] // [[du/dt,dv/dt] for tex1, [du/dt,dv/dt] for tex2]
    multiTextureParam1: [[number, number], [number, number]] // [[du/dt,dv/dt] for tex1, [du/dt,dv/dt] for tex2]
    multiTextureParamX: [number, number] // packed bytes for tex1/tex2; use formula above for tiling scale

    // Optional model emitters
    geometryModel: string // if not empty string, this spawns a model instead of a flat quad
    recursionModel: string // if not empty string, this is an alias for another emitter set

    lifespanVary: number
    emissionRateVary: number
    scaleVary: [number, number]

    baseSpin: number
    baseSpinVary: number
    spin: number
    spinVary: number

    drag: number // drag coefficient. new velocity = old velocity * (1 - drag * dt)
  }

  export interface BoundingBox {
    min: Vector3
    max: Vector3
  }

  export interface CollisionBox {
    min: Vector3
    max: Vector3
  }

  export interface Skin {
    subMeshes: SubMesh[]
    textureUnits: TextureUnit[]
    fileName: string
    fileDataID: number
  }

  export interface SubMesh {
    enabled: boolean
    submeshID: number
    level: number
    vertexStart: number
    vertexCount: number
    triangleStart: number
    triangleCount: number
    boneCount: number
    boneStart: number
    boneInfluences: number
    centerBoneIndex: number
    centerPosition: number[]
    sortCenterPosition: number[]
    sortRadius: number
  }

  export interface TextureUnit {
    flags: number
    priority: number
    shaderID: number
    skinSectionIndex: number
    flags2: number
    colorIndex: number
    materialIndex: number
    materialLayer: number
    textureCount: number
    textureComboIndex: number
    textureCoordComboIndex: number
    textureWeightComboIndex: number
    textureTransformComboIndex: number
  }
}

export class M2MetadataFile {
  fileType: string;

  fileDataID: number;

  fileName: string;

  internalName: string;

  textures: Data.Texture[] = [];

  textureTypes: number[];

  materials: Data.Material[];

  textureCombos: number[];

  animFileIDs: Data.AnimFileId[];

  colors: Data.Color[];

  textureWeights: Data.M2Track<number>;

  transparencyLookup: number[];

  textureTransforms: Data.TextureTransform[] = [];

  textureTransformsLookup: number[];

  boundingBox: Data.BoundingBox;

  boundingSphereRadius: number;

  collisionBox: Data.CollisionBox;

  collisionSphereRadius: number;

  // New: particles exported by wow.export
  particleEmitters?: Data.ParticleEmitter[];

  // New: lights exported by wow.export
  lights?: Data.LightMeta[];

  skin: Data.Skin = {
    subMeshes: [],
    textureUnits: [],
    fileName: '',
    fileDataID: 0,
  };

  isLoaded = false;

  globalSequenceMap: Map<number, GlobalSequence>;

  constructor(private filePath: string, private config: Config, private animFile: AnimationFile, private mdl: MDL) {
    try {
      !config.isBulkExport && console.log('Loading metadata file', this.filePath);
      Object.assign(this, JSON.parse(readFileSync(this.filePath, 'utf-8')));
      if (this.fileType === 'm2') {
      // ADT files (terrain) won't have metadata JSON.
      // WMO files (world object)'s metadata file is not yet supported because it has different format.
      // Therefore fallback to heuristic OBJ textures/materials decoding.
      // Heuristic OBJ textures/materials uses `guessFilterMode` which is not always correct.
        this.isLoaded = true;
      } else {
      // Metadata of other files (WMO) are not supported.
        this.isLoaded = false;
      }
    } catch (e) {
      if (e.code === 'ENOENT') {
        // file not exist, do not throw.
        return;
      }
      throw e;
    }
    this.globalSequenceMap = new Map<number, GlobalSequence>(this.mdl.globalSequences.map((gs) => [gs.id, gs]));
  }

  subMeshGeosetMap: Map<Data.Skin['subMeshes'][number], Geoset> = new Map();

  geosetSubMeshMap: Map<Geoset, Data.Skin['subMeshes'][number]> = new Map();

  mapSubMeshesToMdlGeosets(mdl: MDL) {
    let geosetIdx = 0;
    this.skin.subMeshes.forEach((subMesh) => {
      if (!subMesh.enabled) return;
      const geoset = mdl.geosets[geosetIdx];
      this.subMeshGeosetMap.set(subMesh, geoset);
      this.geosetSubMeshMap.set(geoset, subMesh);
      geoset.wowData.submeshId = subMesh.submeshID;
      geosetIdx++;
    });
  }

  private getGlobalSeq(id: number) {
    if (!this.globalSequenceMap.has(id)) {
      const newGs: GlobalSequence = {
        id, duration: 1,
      };
      this.globalSequenceMap.set(id, newGs);
      this.mdl.globalSequences.push(newGs);
    }
    return this.globalSequenceMap.get(id);
  }

  private m2trackToAnimation<T>(
    m2track: Data.M2Track<T>,
    type: AnimationType,
    transformation: (v: T) => T = (v) => v,
  ): Animation<T> | undefined {
    const result: Animation<T> = {
      interpolation: wowToWc3Interpolation(m2track.interpolation),
      globalSeq: m2track.globalSeq !== BlizzardNull ? this.getGlobalSeq(m2track.globalSeq) : undefined,
      keyFrames: new Map(),
      type,
    };

    let accumTime = 0;
    m2track.timestamps.forEach((timestamps, animId) => {
      let maxTimestamp = -Infinity;
      timestamps.forEach((timestamp, timestampI) => {
        const value = transformation(m2track.values[animId][timestampI]);
        result.keyFrames.set(timestamp + accumTime, value);
        maxTimestamp = Math.max(maxTimestamp, timestamp + accumTime);
      });
      if (maxTimestamp >= -1 && !result.globalSeq) {
        // Add the last key frame to the end of the animation to not lose the last frame of the animation
        result.keyFrames.set(
          accumTime + this.animFile.animations![animId].duration,
          _.cloneDeep(result.keyFrames.get(maxTimestamp)!),
        );
      }
      accumTime += this.animFile.animations![animId].duration + 1;
    });
    if (!result.keyFrames.size) return undefined;
    return result;
  }

  private m2trackToAnimationOrStatic<T>(
    m2track: Data.M2Track<T>,
    type: AnimationType,
    transformation: (v: T) => T = (v) => v,
  ): AnimationOrStatic<T> | undefined {
    if (m2track.values.length === 1 && m2track.values[0].length === 1) {
      return { static: true, value: transformation(m2track.values[0][0]) };
    }
    return this.m2trackToAnimation(m2track, type, transformation);
  }

  extractMDLGeosetAnim() {
    if (!this.isLoaded) {
      throw new Error(`Metadata file is not loaded: ${this.filePath}`);
    }

    const textureUnits = this.skin.textureUnits.filter((t) => t.colorIndex !== 2 ** 16 - 1);
    const result: MDL['geosetAnims'] = [];
    textureUnits.forEach((tu) => {
      const wowColor = this.colors[tu.colorIndex];
      const geoset = this.subMeshGeosetMap.get(this.skin.subMeshes[tu.skinSectionIndex]);
      if (!geoset) {
        if (this.skin.subMeshes[tu.skinSectionIndex].enabled) {
          console.log(chalk.red('geoset not found'), tu.skinSectionIndex, this.skin.subMeshes[tu.skinSectionIndex]);
        }
        return;
      }

      const geosetAnim: MDL['geosetAnims'][number] = {
        id: 0,
        geoset,

        // MDL color order is blue, green, red, but WoW uses red, green, blue
        color: this.m2trackToAnimationOrStatic(wowColor.color, 'color', (v) => [v[2], v[1], v[0]]),

        // MDL alpha is 0..1, but WoW is 0..32767
        alpha: this.m2trackToAnimationOrStatic(wowColor.alpha, 'alpha', (v) => v / 32767),
      };
      result.push(geosetAnim);
      if (geosetAnim.alpha && 'keyFrames' in geosetAnim.alpha) {
        // geosetAnim.alpha.interpolation = 'DontInterp';
      }
    });
    this.mdl.geosetAnims = result;
  }

  extractMDLTexturesMaterials(): {textures: Texture[], submeshIdToMat: Map<number, Material>} {
    if (!this.isLoaded) {
      return {
        textures: [],
        submeshIdToMat: new Map(),
      };
    }

    const debug = false;

    // Textures
    const textures: Texture[] = this.textures.map((tex, i) => ({
      id: 0,
      image: tex.fileNameExternal
        ? join(this.config.assetPrefix, relative(this.config.wowExportAssetDir, join(dirname(this.filePath), tex.fileNameExternal.replace('.png', '.blp'))))
        : '',
      wrapWidth: (tex.flags & 1) > 0,
      wrapHeight: (tex.flags & 2) > 0,
      wowData: {
        type: this.textureTypes[i],
      },
    }));

    // Texture anims
    const textureAnims: TextureAnim[] = this.textureTransforms.map((transform) => ({
      id: 0,
      translation: this.m2trackToAnimation(transform.translation, 'others'),
      rotation: this.m2trackToAnimation(transform.rotation, 'rotation'),
      scaling: this.m2trackToAnimation(transform.scaling, 'scaling'),
    }));
    this.mdl.textureAnims = textureAnims;

    // Materials
    const submeshMaterials = new Map<number, Material>();

    this.skin.textureUnits.forEach((tu) => {
      const submeshId = tu.skinSectionIndex;
      const material = this.materials[tu.materialIndex];
      const textAnimId = this.textureTransformsLookup[tu.textureTransformComboIndex];
      const twoSided = (material.flags & 0x04) > 0;
      const shaderId = tu.shaderID;

      if (!submeshMaterials.has(submeshId)) {
        submeshMaterials.set(submeshId, {
          id: 0,
          constantColor: false,
          twoSided,
          layers: [],
        });
      }

      const layers = submeshMaterials.get(submeshId)!.layers;

      const textureCount = tu.textureCount;

      for (let i = 0; i < Math.min(textureCount, 4); i++) {
        const textureId = this.textureCombos[tu.textureComboIndex + i];
        const texture = textures[textureId];
        const filterMode = getLayerFilterMode(material.blendingMode, shaderId, i, texture);

        if (!filterMode) {
          continue;
        }
        if (layers.length > 0) {
          debug && console.log('layer', {
            shaderId, filterMode, textureId, image: textures[textureId].image,
          });
        }

        layers.push({
          texture,
          filterMode,
          tvertexAnim: textAnimId !== BlizzardNull ? textureAnims[textAnimId] : undefined,

          // https://wowdev.wiki/M2#Render_flags_and_blending_modes
          unshaded: false,
          sphereEnvMap: false,
          unlit: (material.flags & 0x01) > 0,
          unfogged: (material.flags & 0x02) > 0,
          twoSided: (material.flags & 0x04) > 0,
          noDepthTest: (material.flags & 0x08) > 0,
          noDepthSet: (material.flags & 0x10) > 0,
          alpha: {
            static: true,
            value: 1,
          },
        });
      }
    });

    return {
      textures,
      submeshIdToMat: submeshMaterials,
    };
  }

  extractMDLParticlesEmitters(textures: Texture[]) {
    if (!this.isLoaded || !Array.isArray(this.particleEmitters)) {
      return;
    }

    const mapBlend = (b?: number): WC3FilterMode => {
      switch (b) {
        case 0: return WC3FilterMode.Blend;
        case 1: return WC3FilterMode.AlphaKey;
        case 2: return WC3FilterMode.Blend;
        case 3: return WC3FilterMode.Additive;
        case 4: return WC3FilterMode.Additive;
        case 5: return WC3FilterMode.Modulate;
        case 6: return WC3FilterMode.Modulate2x;
        case 7: return WC3FilterMode.Additive;
        default: return WC3FilterMode.Blend;
      }
    };

    const particleEmitter2s: ParticleEmitter2[] = [];

    this.particleEmitters.forEach((p, i) => {
      const hasHead = (p.flags & 0x20000) > 0;
      const hasTail = (p.flags & 0x40000) > 0;

      let textureId = p.texturePacked;
      if (p.flags & 0x10000000) { // multi-texture then use only the first texture
        textureId &= 0x1F;
      }

      const parent = this.mdl.bones[(p.bone ?? 0)] ?? this.mdl.bones[0];

      const partTimestamps = Array.from(new Set(<Data.M2Track<number>['timestamps'][0]>[
        // ...p.colorTrack.timestamps,
        ...p.alphaTrack.timestamps,
        // ...p.scaleTrack.timestamps,
        // ...p.headCellTrack.timestamps,
        // ...p.tailCellTrack.timestamps,
      ])).sort((a, b) => a - b);

      const timeMid = partTimestamps[Math.floor(partTimestamps.length / 2)];
      const timeEnd = partTimestamps[partTimestamps.length - 1];

      const getValueAt = <T>(partTrack: Data.PartTrack<T>, time: number): T | undefined => {
        const index = partTrack.timestamps.findIndex((t) => t >= time);
        if (index === -1) return undefined;
        return partTrack.values[index];
      };

      const decompressGravity = (vUint32: number) => {
        // Try float first
        const buf = new ArrayBuffer(4);
        const view = new DataView(buf);
        view.setUint32(0, vUint32, true);
        const f = view.getFloat32(0, true);
        // Heuristic: valid scalar gravity tends to be finite and |f| < ~1000 in model units.
        // If f is NaN/Inf/abs too big, or the lower 16 bits are unlikely for IEEE-754,
        // fallback to compressed decode.
        const saneFloat = Number.isFinite(f) && Math.abs(f) < 1e4;

        if (saneFloat) {
          // WoW scalar uses -float on Z (viewer applies downward accel). Keep as-is for WC3.
          return f;
        }

        // Decompression to vector3: https://wowdev.wiki/M2#Compressed_Particle_Gravity
        const bx = vUint32 & 0xFF;
        const by = (vUint32 >>> 8) & 0xFF;
        let bz = (vUint32 >>> 16) & 0xFFFF;

        // unsign int8 to signed int8
        const x = (bx & 0x80) ? bx - 0x100 : bx;
        const y = (by & 0x80) ? by - 0x100 : by;

        // unsign int16 to signed int16
        if (bz & 0x8000) bz -= 0x10000;

        const dirX = x * (1 / 128);
        const dirY = y * (1 / 128);
        const dot = dirX * dirX + dirY * dirY;
        let z = Math.sqrt(Math.max(0, 1 - dot));
        let mag = bz * 0.04238648;
        if (mag < 0) {
          z = -z;
          mag = -mag;
        }
        z *= mag;
        return -z;
      };

      const speed = this.m2trackToAnimationOrStatic(p.emissionSpeed, 'others')!;

      const node: ParticleEmitter2 = {
        objectId: -1,
        type: 'ParticleEmitter2',
        name: `ParticleEmitter_${i}`,
        pivotPoint: [p.position[0], -p.position[2], p.position[1]], // x -z y
        parent,
        flags: [],
        flags2: [],
        filterMode: mapBlend(p.blendingType),
        width: this.m2trackToAnimationOrStatic(p.emissionAreaWidth, 'others')!,
        length: this.m2trackToAnimationOrStatic(p.emissionAreaLength, 'others')!,
        speed,
        variation: this.m2trackToAnimationOrStatic(p.speedVariation, 'others')!,
        emissionRate: this.m2trackToAnimationOrStatic(p.emissionRate, 'others')!,
        latitude: this.m2trackToAnimationOrStatic(p.verticalRange, 'others', degrees)!,
        visibility: this.m2trackToAnimation(p.enabledIn, 'others')!,
        texture: textures[textureId],
        tailLength: p.tailLength,
        columns: Math.max(1, p.textureCols),
        rows: Math.max(1, p.textureRows),
        headOrTail: hasHead && hasTail ? WC3HeadOrTail.Both : hasTail ? WC3HeadOrTail.Tail : WC3HeadOrTail.Head,
        priorityPlane: 0,
        replaceableId: 0,
        gravity: this.m2trackToAnimationOrStatic(p.gravity, 'others', decompressGravity)!,
        timeMiddle: timeMid / timeEnd,
        squirt: (p.flags & 0x40) > 0,
        lifeSpan: Math.max(...p.lifespan.values.flat()),

        segmentColors: (() => {
          const track = p.colorTrack;
          const v0 = getValueAt(track, 0) ?? [255, 255, 255];
          const v1 = getValueAt(track, timeMid) ?? v0;
          const v2 = getValueAt(track, timeEnd) ?? v1;
          const f = (v: Vector3): Vector3 => [v[2] / 255, v[1] / 255, v[0] / 255];
          return [f(v0), f(v1), f(v2)];
        })(),
        segmentAlphas: (() => {
          const track = p.alphaTrack;
          const v0 = getValueAt(track, 0) ?? 32767;
          const v1 = getValueAt(track, timeMid) ?? v0;
          const v2 = getValueAt(track, timeEnd) ?? v1;
          const f = (v: number) => Math.round(255 * v / 32767);
          return [f(v0), f(v1), f(v2)];
        })(),
        segmentScaling: (() => {
          const scalingTrack = p.scaleTrack;
          const scaling0 = getValueAt(scalingTrack, 0) ?? [1, 1];
          const scaling1 = getValueAt(scalingTrack, timeMid) ?? scaling0;
          const scaling2 = getValueAt(scalingTrack, timeEnd) ?? scaling1;

          // in wow, non-moving particles are scale down by 0.05
          // as seem in source code of WebWowViewerCpp
          const factor = 'static' in speed && speed.value < 0.0001 ? 0.05 : 1;

          const f = (scaling: [number, number]) => Math.min(scaling[0], scaling[1]) * factor;
          return [f(scaling0), f(scaling1), f(scaling2)];
        })(),
        headIntervals: (() => {
          const track = p.headCellTrack;
          const v0 = getValueAt(track, 0) ?? 0;
          const v1 = getValueAt(track, timeMid) ?? v0;
          return [v0, v1, 1];
        })(),
        decayIntervals: (() => {
          const track = p.headCellTrack;
          const v1 = getValueAt(track, timeMid) ?? 0;
          const v2 = getValueAt(track, timeEnd) ?? v1;
          return [v1, v2, 1];
        })(),
        tailIntervals: (() => {
          const track = p.tailCellTrack;
          const v0 = getValueAt(track, 0) ?? 0;
          const v1 = getValueAt(track, timeMid) ?? v0;
          return [v0, v1, 1];
        })(),
        tailDecayIntervals: (() => {
          const track = p.tailCellTrack;
          const v1 = getValueAt(track, timeMid) ?? 0;
          const v2 = getValueAt(track, timeEnd) ?? v1;
          return [v1, v2, 1];
        })(),
      };

      // Some flags are different than documented in https://wowdev.wiki/M2#Particle_Flags
      // Because it follows the rendering code in the https://github.com/Deamon87/WebWowViewerCpp
      if (p.flags & 0x1) node.flags2.push(ParticleEmitter2Flag.Unshaded);
      if (p.flags & 0x10) node.flags2.push(ParticleEmitter2Flag.ModelSpace);
      // if (p.flags & 0x1000) node.flags2.push(ParticleEmitter2Flag.XYQuad);

      // Apply drag
      if (p.drag > 0) {
        if ('static' in speed) {
          const equivalentVelocity = calculateEquivalentVelocityNoDrag(speed.value, node.lifeSpan, p.drag);
          speed.value = equivalentVelocity;
        } else {
          const keyFrames = speed.keyFrames;
          keyFrames.forEach((v, k) => {
            const equivalentVelocity = calculateEquivalentVelocityNoDrag(v, node.lifeSpan, p.drag);
            keyFrames.set(k, equivalentVelocity);
          });
        }
      }

      const varyThreshold = 0.1;
      const chooseRandomTexture = p.flags & 0x10000
        && p.headCellTrack.timestamps.length === 0
        && p.textureCols * p.textureRows > 1;
      const scaleVary = (p.scaleVary[0] + p.scaleVary[1]) / 2;
      const hasVary = chooseRandomTexture || p.lifespanVary >= varyThreshold || scaleVary >= varyThreshold;

      const diableVariants = false;
      if (!hasVary || diableVariants) {
        particleEmitter2s.push(node);
        return;
      }

      // sample N different variants of the particle emitter
      let maxVariants = 1;
      if (chooseRandomTexture) maxVariants *= p.textureRows * p.textureCols;
      if (p.lifespanVary >= varyThreshold) maxVariants *= 3;
      if (scaleVary >= varyThreshold) maxVariants *= 3;

      // TODO: replace / this.particleEmitters!.length with only PEs that need variants
      const variantCount = Math.round(Math.min(maxVariants, 200 / this.particleEmitters!.length));
      if (variantCount < 1) {
        particleEmitter2s.push(node);
        return;
      }

      const rand = () => Math.random() * 2 - 1;
      // Sample N variants
      const variants: ParticleEmitter2[] = [];
      const token = new Map<ParticleEmitter2, number>();

      // // Create a dedicated bone to group the variants of the same particle emitter
      // TODO: this doesn't work yet, for some reason the Sha of Doubt model breaks. Other test cases worked fine.
      // const newParent: Bone = {
      //   objectId: -1,
      //   type: 'Bone',
      //   name: `${node.name}_group`,
      //   parent,
      //   flags: [],
      //   pivotPoint: [...parent.pivotPoint],
      //   geoset: 'Multiple', // without it the model will be broken
      //   translation: { // wc3 requires a translation for the bone
      //     interpolation: 'DontInterp',
      //     type: 'translation',
      //     keyFrames: new Map([[0, [0, 0, 0]]]),
      //   },
      // };
      // this.mdl.bones.push(newParent);

      node.parent = undefined; // remove to avoid cloning the parent
      for (let i = 0; i < variantCount; i++) {
        const variant = _.cloneDeep(node);
        variant.parent = parent;
        // variant.parent = newParent;

        if (chooseRandomTexture) {
          const cell = Math.floor(Math.random() * p.textureRows * p.textureCols);
          variant.headIntervals = [cell, cell, 1];
          variant.decayIntervals = [cell, cell, 1];
          variant.tailIntervals = [cell, cell, 1];
          variant.tailDecayIntervals = [cell, cell, 1];
        }

        let scaleMult = 1;
        if (scaleVary >= varyThreshold) {
          scaleMult = Math.max(1 + rand() * scaleVary);
          variant.segmentScaling = V3.scale(variant.segmentScaling, scaleMult);
        }

        let lifeSpanMult = 1;
        if (p.lifespanVary >= varyThreshold) {
          const newLifeSpan = variant.lifeSpan + Math.random() * p.lifespanVary;
          lifeSpanMult = newLifeSpan / variant.lifeSpan;
          variant.lifeSpan = newLifeSpan;
        }

        token.set(variant, lifeSpanMult * scaleMult);
        variants.push(variant);
      }

      // const totalTokens = Array.from(token.values()).reduce((a, b) => a + b, 0);
      for (const variant of variants) {
        // we need to randomize the emission rate to avoid the same value for all variants
        // causing all particles to spawn at the same time
        const emissionRateFactor = 1 / variantCount * (1 + rand() * 0.5);
        // const emissionRateFactor = token.get(variant)! / totalTokens;
        if ('static' in variant.emissionRate) {
          variant.emissionRate.value *= emissionRateFactor;
        } else if ('keyFrames' in variant.emissionRate) {
          const keyFrames = variant.emissionRate.keyFrames;
          keyFrames.forEach((v, k) => {
            keyFrames.set(k, v * emissionRateFactor);
          });
        }
      }

      particleEmitter2s.push(...variants);
    });

    this.mdl.particleEmitter2s = particleEmitter2s;
    this.mdl.textures.push(...particleEmitter2s.map((e) => e.texture));
    !this.config.isBulkExport && console.log('Particle emitters 2:', this.mdl.particleEmitter2s.length);
  }

  extractMDLLights(): void {
    if (!this.isLoaded || !Array.isArray(this.lights)) {
      return;
    }

    const lights: Light[] = [];

    const toAnimOrStatic = <T>(track: Data.M2Track<T>, type: AnimationType, transform?: (v: T) => T) => this.m2trackToAnimationOrStatic(track, type, transform ?? ((v) => v));

    this.lights.forEach((l, i) => {
      const parent = this.mdl.bones[(l.bone ?? 0)] ?? this.mdl.bones[0];

      // Warcraft 3 light types mapping
      const wc3Type: LightType = l.type === 1 ? LightType.Omnidirectional : LightType.Directional;

      // WoW colors are RGB 0..1, WC3 MDL expects BGR in writer utilities convert via Vector3 ordering in geosetAnims.
      // For lights we will keep RGB order; node writer expects `Color`/`AmbColor` in RGB and handles printing.
      const colorXform = (v: Vector3): Vector3 => [v[0], v[1], v[2]];

      const node: Light = {
        objectId: -1,
        type: 'Light',
        name: `Light_${i}`,
        pivotPoint: [l.position[0], -l.position[2], l.position[1]],
        parent,
        flags: [],
        lightType: wc3Type,
        attenuationStart: toAnimOrStatic(l.attenuation_start, 'others')!,
        attenuationEnd: toAnimOrStatic(l.attenuation_end, 'others')!,
        intensity: toAnimOrStatic(l.diffuse_intensity, 'others')!,
        color: toAnimOrStatic(l.diffuse_color, 'others', colorXform)!,
        ambientIntensity: toAnimOrStatic(l.ambient_intensity, 'others')!,
        ambientColor: toAnimOrStatic(l.ambient_color, 'others', colorXform)!,
        visibility: this.m2trackToAnimation(l.visibility, 'others'),
      };

      lights.push(node);
    });

    this.mdl.lights = lights;
    // !this.config.isBulkExport &&
    lights.length > 0 && console.log(
      chalk.yellow('Lights:'),
      this.mdl.model.name,
      this.mdl.lights.length,
    );
  }

  objToSubmesh = new Map<number, number>();

  getSkinWeightIndex(geosetVertexIndex: number) {
    if (this.objToSubmesh.size === 0) {
      let idx = 0;
      this.skin.subMeshes.forEach((submesh) => {
        if (!submesh.enabled) return;
        for (let v = submesh.vertexStart; v < submesh.vertexStart + submesh.vertexCount; v++) {
          this.objToSubmesh.set(idx, v);
          idx++;
        }
      });
    }
    return this.objToSubmesh.get(geosetVertexIndex);
  }
}

/**
 * Calculate the equivalent initial velocity for a particle without drag
 * that travels the same distance as a particle with drag over the same lifetime
 *
 * @param initialVelocity - Initial velocity of particle with drag
 * @param lifetime - Particle lifetime in seconds
 * @param drag - Drag coefficient
 * @param deltaTime - Time step (typically 0.016-0.033 seconds)
 * @returns Equivalent initial velocity for particle without drag
 */
function calculateEquivalentVelocityNoDrag(
  initialVelocity: number,
  lifetime: number,
  drag: number,
  deltaTime: number = 1 / 30,
): number {
  if (drag === 0) {
    return initialVelocity;
  }

  const decayFactor = Math.max(0, 1 - drag * deltaTime);
  const steps = lifetime / deltaTime;

  // Calculate the integral of velocity over time
  // ∫₀^T v₀ * (1 - drag * dt)^(t/dt) dt
  const integral = initialVelocity * (decayFactor ** steps - 1) / Math.log(decayFactor) * deltaTime;

  // Equivalent velocity without drag = distance / lifetime
  const result = integral / lifetime;
  if (isNaN(result)) {
    console.log(chalk.red('NaN'), {
      initialVelocity,
      lifetime,
      drag,
      deltaTime,
      decayFactor,
      steps,
      integral,
    });
    return initialVelocity;
  }
  return result;
}

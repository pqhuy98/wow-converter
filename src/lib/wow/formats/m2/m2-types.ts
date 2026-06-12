/**
 * Shared M2/SKEL structure interfaces for the native WoW reader,
 * mirroring the object shapes produced by wow.export's loaders.
 */
import type {
  M2PartTrack, M2SplineKey, M2Track, M2TrackValue,
} from './m2-generics';

export interface M2Animation {
  id: number;
  variationIndex: number;
  duration: number;
  movespeed: number;
  flags: number;
  frequency: number;
  padding: number;
  replayMin: number;
  replayMax: number;
  blendTimeIn: number;
  blendTimeOut: number;
  boxPosMin: number[];
  boxPosMax: number[];
  boxRadius: number;
  variationNext: number;
  aliasNext: number;
}

export interface M2Bone {
  boneID: number;
  flags: number;
  parentBone: number;
  subMeshID: number;
  boneNameCRC: number;
  translation: M2Track<number[]>;
  rotation: M2Track<number[]>;
  scale: M2Track<number[]>;
  pivot: number[];
}

export interface M2Attachment {
  id: number;
  bone: number;
  unknown: number;
  position: number[];
  animateAttached: M2Track;
}

export interface AnimFileIDEntry {
  animID: number;
  subAnimID: number;
  fileDataID: number;
}

export interface M2Material {
  flags: number;
  blendingMode: number;
}

export interface M2Color {
  color: M2Track;
  alpha: M2Track;
}

export interface M2TextureTransform {
  translation: M2Track;
  rotation: M2Track;
  scaling: M2Track;
}

export interface M2Camera {
  type: number;
  far_clip: number;
  near_clip: number;
  positions: M2Track<M2SplineKey>;
  position_base: number[];
  target_position: M2Track<M2SplineKey>;
  target_position_base: number[];
  roll: M2Track<M2SplineKey>;
  FoV: M2Track<M2SplineKey> | null;
}

export interface M2Light {
  type: number;
  bone: number;
  position: number[];
  ambient_color: M2Track;
  ambient_intensity: M2Track;
  diffuse_color: M2Track;
  diffuse_intensity: M2Track;
  attenuation_start: M2Track;
  attenuation_end: M2Track;
  visibility: M2Track;
}

export interface M2RibbonEmitter {
  ribbonId: number;
  boneIndex: number;
  position: number[];
  textureIndices: M2TrackValue[];
  materialIndices: M2TrackValue[];
  colorTrack: M2Track;
  alphaTrack: M2Track;
  heightAboveTrack: M2Track;
  heightBelowTrack: M2Track;
  edgesPerSecond: number;
  edgeLifetime: number;
  gravity: number;
  textureRows: number;
  textureCols: number;
  texSlotTrack: M2Track;
  visibilityTrack: M2Track;
  priorityPlane: number;
  ribbonColorIndex: number;
  textureTransformLookupIndex: number;
}

export interface M2ParticleEmitter {
  particleId: number;
  flags: number;
  position: number[];
  bone: number;
  texturePacked: number;
  geometryModel: string;
  recursionModel: string;
  blendingType: number;
  emitterType: number;
  particleColorIndex: number;
  multiTextureParamX: number[];
  textureTileRotation: number;
  textureRows: number;
  textureCols: number;
  emissionSpeed: M2Track;
  speedVariation: M2Track;
  verticalRange: M2Track;
  horizontalRange: M2Track;
  gravity: M2Track;
  lifespan: M2Track;
  lifespanVary: number;
  emissionRate: M2Track;
  emissionRateVary: number;
  emissionAreaLength: M2Track;
  emissionAreaWidth: M2Track;
  zSource: M2Track;
  colorTrack: M2PartTrack;
  alphaTrack: M2PartTrack;
  scaleTrack: M2PartTrack;
  scaleVary: number[];
  headCellTrack: M2PartTrack;
  tailCellTrack: M2PartTrack;
  tailLength: number;
  twinkleSpeed: number;
  twinklePercent: number;
  twinkleScale: { min: number; max: number };
  burstMultiplier: number;
  drag: number;
  baseSpin: number;
  baseSpinVary: number;
  spin: number;
  spinVary: number;
  tumble: number[];
  windVector: number[];
  windTime: number;
  followSpeed1: number;
  followScale1: number;
  followSpeed2: number;
  followScale2: number;
  multiTextureParam0: number[][] | null;
  multiTextureParam1: number[][] | null;
  splinePoints: number[][];
  enabledIn: M2Track;
}

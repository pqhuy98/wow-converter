import { EulerRotation, Vector3 } from '../math/common';
import { MDL } from '../objmdl/mdl/mdl';

export type WowObjectType = 'adt' | 'wmo' | 'm2' | 'gobj';

export function isWowObjectType(type: string): type is WowObjectType {
  return ['adt', 'wmo', 'm2', 'gobj', 'null'].includes(type);
}

export interface WowObject {
  id: string
  model?: Model
  position: Vector3
  rotation: EulerRotation
  scaleFactor: number
  children: WowObject[]
  type: string
}

export interface Config {
  assetPrefix: string
  terrainHeightClampPercent: {
    upper: number,
    lower: number;
  }
  waterZThreshold: number;
  pitchRollThresholdRadians: number
  verticalHorizontalRatio: number
  rawModelScaleUp: number
  infiniteExtentBoundRadiusThreshold: number
  overrideModels: boolean
  placeCreatures: boolean
  exportCreatureModels: boolean
  release?: boolean
}

export interface Model {
  relativePath: string
  mdl: MDL
}

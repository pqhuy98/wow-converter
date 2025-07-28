import { EulerRotation, Vector3 } from '../../math/common';
import { MDL } from '../../objmdl/mdl/mdl';

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
  type: WowObjectType
}

export interface Model {
  relativePath: string
  mdl: MDL
}

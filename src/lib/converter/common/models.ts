import { Creature } from '@/lib/azerothcore-client/creatures';

import { MDL } from '../../formats/mdl/mdl';
import { EulerRotation, Vector3 } from '../../math/common';

export type WowObjectType = 'adt' | 'wmo' | 'm2' | 'gobj' | 'unit';

export function isWowObjectType(type: string): type is WowObjectType {
  return ['adt', 'wmo', 'm2', 'gobj', 'unit'].includes(type);
}

export interface Model {
  relativePath: string;
  mdl: MDL;
}

export interface WowObject {
  id: string;
  type: WowObjectType;
  model?: Model; // undefined for ADT and UNIT
  position: Vector3; // local to parent
  rotation: EulerRotation; // local to parent
  scaleFactor: number; // local to parent
  children: WowObject[];
}

export interface WowAdt extends WowObject {
  type: 'adt';
  tileX: number;
  tileY: number;
}

export interface WowWmo extends WowObject { type: 'wmo' }
export interface WowM2 extends WowObject { type: 'm2' }
export interface WowGobj extends WowObject { type: 'gobj' }

export interface WowUnit extends WowObject {
  type: 'unit';
  creature: Creature;
}

// export type WowObject = WowAdt | WowWmo | WowM2 | WowGobj | WowUnit;

export function isWowUnit(o: WowObject): o is WowUnit { return o.type === 'unit'; }
export function isWowAdt(o: WowObject): o is WowAdt { return o.type === 'adt'; }

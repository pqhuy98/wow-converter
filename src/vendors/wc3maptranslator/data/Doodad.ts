import { type angle } from '../CommonInterfaces';
import { ItemSet } from './ItemSet';

interface Doodad {
  groupId?: number
  unknown1?: number
  roll?: number
  pitch?: number
  lights?: DoodadLight[]
  state?: number
  type: string
  variation: number
  position: [number, number, number]
  angle: angle
  scale: [number, number, number]
  skinId: string
  flags: DoodadFlag
  life: number
  randomItemSetPtr: number
  droppedItemSets: ItemSet[]
  id: number
}

interface DoodadFlag {
  visible: boolean
  solid: boolean
  customHeight: boolean
}

interface SpecialDoodad {
  type: string,
  position: [number, number, number]
}

export type { Doodad, DoodadFlag, SpecialDoodad };

export type DoodadList = [Doodad[], SpecialDoodad[]]

interface DoodadLight {
  index: number
  shadowCasting: number
  color: number // packed BGRA
  intensity: number
  shadowCastingStart: number
  shadowCastingEnd: number
  quadraticFalloff: number
  linearFalloff: number
  damping: number
}

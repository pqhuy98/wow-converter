

export type RefType = "local" | "wowhead" | "displayID"
export type AttackTag = "" | "1H" | "2H" | "2HL" | "Unarmed" | "Bow" | "Rifle" | "Thrown"
export type ModelSize = "small" | "medium" | "large" | "hero" | "giant"
export type ModelFormat = "mdx" | "mdl"

export interface RefSchema {
  type: RefType
  value: string
}

export interface AttachItem {
  path: RefSchema
  scale?: number
}

export interface Character {
  base: RefSchema
  attackTag?: AttackTag
  keepCinematic?: boolean
  inGameMovespeed: number
  size?: ModelSize
  scale?: number
  attachItems?: Record<number, AttachItem>
  noDecay?: boolean
  portraitCameraSequenceName?: string
}

export interface ExportRequest {
  character: Character
  outputFileName: string
  optimization: {
    sortSequences?: boolean
    removeUnusedVertices?: boolean
    removeUnusedNodes?: boolean
    removeUnusedMaterialsTextures?: boolean
  }
  format?: ModelFormat
}

export interface ExportCharacterResponse {
  exportedModels: string[]
  exportedTextures: string[]
  outputDirectory?: string
  versionId: string
}

export interface JobStatus {
  id: string
  status: 'pending' | 'processing' | 'done' | 'failed'
  position: number | null
  result: ExportCharacterResponse | null
  error: string | null
  request?: ExportRequest
  submittedAt: number
  startedAt?: number
  finishedAt?: number
}

export interface FullJobStatus extends JobStatus {
  request: ExportRequest
}

export enum WoWAttachmentID {
  Shield = 0,
  HandRight = 1,
  HandLeft = 2,
  ElbowRight = 3,
  ElbowLeft = 4,
  ShoulderRight = 5,
  ShoulderLeft = 6,
  KneeRight = 7,
  KneeLeft = 8,
  HipRight = 9,
  HipLeft = 10,
  Helm = 11,
  Back = 12,
  ShoulderFlapRight = 13,
  ShoulderFlapLeft = 14,
  ChestBloodFront = 15,
  ChestBloodBack = 16,
  Breath = 17,
  PlayerName = 18,
  Base = 19,
  Head = 20,
  SpellLeftHand = 21,
  SpellRightHand = 22,
  Special1 = 23,
  Special2 = 24,
  Special3 = 25,
  SheathMainHand = 26,
  SheathOffHand = 27,
  SheathShield = 28,
  PlayerNameMounted = 29,
  LargeWeaponLeft = 30,
  LargeWeaponRight = 31,
  HipWeaponLeft = 32,
  HipWeaponRight = 33,
  Chest = 34,
  HandArrow = 35,
  Bullet = 36,
  SpellHandOmni = 37,
  SpellHandDirected = 38,
  VehicleSeat1 = 39,
  VehicleSeat2 = 40,
  VehicleSeat3 = 41,
  VehicleSeat4 = 42,
  VehicleSeat5 = 43,
  VehicleSeat6 = 44,
  VehicleSeat7 = 45,
  VehicleSeat8 = 46,
  LeftFoot = 47,
  RightFoot = 48,
  ShieldNoGlove = 49,
  SpineLow = 50,
  AlteredShoulderR = 51,
  AlteredShoulderL = 52,
  BeltBuckle = 53,
  SheathCrossbow = 54,
  HeadTop = 55,
  VirtualSpellDirected = 56,
  Backpack = 57,
  Unknown = 60,
}

export const commonAttachments = [
  { id: WoWAttachmentID.HandRight, name: "Hand Right" },
  { id: WoWAttachmentID.HandLeft, name: "Hand Left" },
  { id: WoWAttachmentID.Shield, name: "Shield" },
]

export const otherAttachments = [
  { id: WoWAttachmentID.ElbowRight, name: "Elbow Right" },
  { id: WoWAttachmentID.ElbowLeft, name: "Elbow Left" },
  { id: WoWAttachmentID.ShoulderRight, name: "Shoulder Right" },
  { id: WoWAttachmentID.ShoulderLeft, name: "Shoulder Left" },
  { id: WoWAttachmentID.KneeRight, name: "Knee Right" },
  { id: WoWAttachmentID.KneeLeft, name: "Knee Left" },
  { id: WoWAttachmentID.HipRight, name: "Hip Right" },
  { id: WoWAttachmentID.HipLeft, name: "Hip Left" },
  { id: WoWAttachmentID.Helm, name: "Helm" },
  { id: WoWAttachmentID.Back, name: "Back" },
  { id: WoWAttachmentID.ShoulderFlapRight, name: "Shoulder Flap Right" },
  { id: WoWAttachmentID.ShoulderFlapLeft, name: "Shoulder Flap Left" },
  { id: WoWAttachmentID.ChestBloodFront, name: "Chest Blood Front" },
  { id: WoWAttachmentID.ChestBloodBack, name: "Chest Blood Back" },
  { id: WoWAttachmentID.Breath, name: "Breath" },
  { id: WoWAttachmentID.PlayerName, name: "Player Name" },
  { id: WoWAttachmentID.Base, name: "Base" },
  { id: WoWAttachmentID.Head, name: "Head" },
  { id: WoWAttachmentID.SpellLeftHand, name: "Spell Left Hand" },
  { id: WoWAttachmentID.SpellRightHand, name: "Spell Right Hand" },
  { id: WoWAttachmentID.Special1, name: "Special 1" },
  { id: WoWAttachmentID.Special2, name: "Special 2" },
  { id: WoWAttachmentID.Special3, name: "Special 3" },
  { id: WoWAttachmentID.SheathMainHand, name: "Sheath Main Hand" },
  { id: WoWAttachmentID.SheathOffHand, name: "Sheath Off Hand" },
  { id: WoWAttachmentID.SheathShield, name: "Sheath Shield" },
  { id: WoWAttachmentID.PlayerNameMounted, name: "Player Name Mounted" },
  { id: WoWAttachmentID.LargeWeaponLeft, name: "Large Weapon Left" },
  { id: WoWAttachmentID.LargeWeaponRight, name: "Large Weapon Right" },
  { id: WoWAttachmentID.HipWeaponLeft, name: "Hip Weapon Left" },
  { id: WoWAttachmentID.HipWeaponRight, name: "Hip Weapon Right" },
  { id: WoWAttachmentID.Chest, name: "Chest" },
  { id: WoWAttachmentID.HandArrow, name: "Hand Arrow" },
  { id: WoWAttachmentID.Bullet, name: "Bullet" },
  { id: WoWAttachmentID.SpellHandOmni, name: "Spell Hand Omni" },
  { id: WoWAttachmentID.SpellHandDirected, name: "Spell Hand Directed" },
  { id: WoWAttachmentID.VehicleSeat1, name: "Vehicle Seat 1" },
  { id: WoWAttachmentID.VehicleSeat2, name: "Vehicle Seat 2" },
  { id: WoWAttachmentID.VehicleSeat3, name: "Vehicle Seat 3" },
  { id: WoWAttachmentID.VehicleSeat4, name: "Vehicle Seat 4" },
  { id: WoWAttachmentID.VehicleSeat5, name: "Vehicle Seat 5" },
  { id: WoWAttachmentID.VehicleSeat6, name: "Vehicle Seat 6" },
  { id: WoWAttachmentID.VehicleSeat7, name: "Vehicle Seat 7" },
  { id: WoWAttachmentID.VehicleSeat8, name: "Vehicle Seat 8" },
  { id: WoWAttachmentID.LeftFoot, name: "Left Foot" },
  { id: WoWAttachmentID.RightFoot, name: "Right Foot" },
  { id: WoWAttachmentID.ShieldNoGlove, name: "Shield No Glove" },
  { id: WoWAttachmentID.SpineLow, name: "Spine Low" },
  { id: WoWAttachmentID.AlteredShoulderR, name: "Altered Shoulder R" },
  { id: WoWAttachmentID.AlteredShoulderL, name: "Altered Shoulder L" },
  { id: WoWAttachmentID.BeltBuckle, name: "Belt Buckle" },
  { id: WoWAttachmentID.SheathCrossbow, name: "Sheath Crossbow" },
  { id: WoWAttachmentID.HeadTop, name: "Head Top" },
  { id: WoWAttachmentID.VirtualSpellDirected, name: "Virtual Spell Directed" },
  { id: WoWAttachmentID.Backpack, name: "Backpack" },
  { id: WoWAttachmentID.Unknown, name: "Unknown" },
]
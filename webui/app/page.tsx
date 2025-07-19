"use client"

import { useState, useMemo } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Checkbox } from "@/components/ui/checkbox"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { Plus, Trash2, Download, User, Sword, HelpCircle, AlertCircle } from "lucide-react"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { host } from "@/app/config"
import Link from "next/link"

type RefType = "local" | "wowhead" | "displayID"
type AttackTag = "" | "1H" | "2H" | "2HL" | "Unarmed" | "Bow" | "Rifle" | "Thrown"
type Size = "small" | "medium" | "large" | "hero" | "giant"
type Format = "mdx" | "mdl"

interface RefSchema {
  type: RefType
  value: string
}

interface AttachItem {
  path: RefSchema
  scale?: number
}

interface Character {
  base: RefSchema
  attackTag?: AttackTag
  keepCinematic?: boolean
  inGameMovespeed: number
  size?: Size
  scale?: number
  attachItems?: Record<number, AttachItem>
  noDecay?: boolean
  portraitCameraSequenceName?: string
}

interface ExportRequest {
  character: Character
  outputFileName: string
  optimization?: {
    sortSequences?: boolean
    removeUnusedVertices?: boolean
    removeUnusedNodes?: boolean
    removeUnusedMaterialsTextures?: boolean
  }
  format?: Format
}

// Tooltips organized in a record
const tooltips = {
  baseModel: "The base character model to use. Can be a Wowhead URL, local file relative to wow.export folder, or Display ID number.",
  attackAnimation: "Determines which attack animations the character will use.",
  characterSize: "How tall the character is in the game.",
  movementSpeed: "Base movement speed of the unit type",
  scaleMultiplier: "Additional scale multiplier (1.0 = normal size, optional)",
  keepCinematic: "Preserve cinematic animation sequences in the exported model. Warning: WoW models have many cinematic sequences, this significantly increases file size.",
  noDecay: "Do not automatically add Decay animations",
  portraitCamera: "Name of the sequence to use for positioning the character portrait camera. E.g. if later you use Stand Ready as default stand animation, the portrait camera needs to be placed lower since the model will usually hunch a bit.",
  itemReference: "The item to attach - can be a Wowhead URL, local file relative to wow.export folder, or Display ID.",
  attachmentPoint: "Where on the character model this item will be attached",
  itemScale: "Scale multiplier for this specific item (1.0 = normal size)",
}

enum WoWAttachmentID {
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

const commonAttachments = [
  { id: WoWAttachmentID.HandRight, name: "Hand Right" },
  { id: WoWAttachmentID.HandLeft, name: "Hand Left" },
  { id: WoWAttachmentID.Shield, name: "Shield" },
]

const otherAttachments = [
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

function AttachmentSelector({
  value,
  onChange,
  usedIds,
}: {
  value: number
  onChange: (id: number) => void
  usedIds: Set<number>
}) {
  const availableCommon = commonAttachments.filter((att) => att.id === value || !usedIds.has(att.id))
  const availableOther = otherAttachments.filter((att) => att.id === value || !usedIds.has(att.id))

  return (
    <Select value={value.toString()} onValueChange={(val) => onChange(Number(val))}>
      <SelectTrigger>
        <SelectValue />
      </SelectTrigger>
      <SelectContent align="start">
        {availableCommon.map((attachment) => (
          <SelectItem key={attachment.id} value={attachment.id.toString()}>
            {attachment.name} ({attachment.id})
          </SelectItem>
        ))}
        {availableOther.length > 0 && (
          <>
            <div className="px-2 py-1.5 text-xs text-muted-foreground border-t">Other attachments (untested):</div>
            {availableOther.map((attachment) => (
              <SelectItem key={attachment.id} value={attachment.id.toString()}>
                {attachment.name} ({attachment.id})
              </SelectItem>
            ))}
          </>
        )}
      </SelectContent>
    </Select>
  )
}

type RefCategory = "npc" | "item"

function RefInput({
  value,
  onChange,
  label,
  tooltipKey,
  category,
}: {
  value: RefSchema
  onChange: (ref: RefSchema) => void
  label: string
  tooltipKey: keyof typeof tooltips
  category: RefCategory
}) {
  const error = validateRef(value, category)

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <Label className="text-sm font-medium">{label}</Label>
        <TooltipProvider delayDuration={200}>
          <Tooltip>
            <TooltipTrigger>
              <HelpCircle className="h-4 w-4 text-muted-foreground" />
            </TooltipTrigger>
            <TooltipContent>
              <p className="max-w-xs">{tooltips[tooltipKey]}</p>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
        <Select value={value.type} onValueChange={(type: RefType) => onChange({ ...value, type })}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent align="start">
            <SelectItem value="wowhead">Wowhead URL</SelectItem>
            <SelectItem value="local">Local File</SelectItem>
            <SelectItem value="displayID">Display ID</SelectItem>
          </SelectContent>
        </Select>

        <div className="md:col-span-2">
          <Input
            placeholder={
              value.type === "local"
                ? "Enter file name..."
                : value.type === "wowhead"
                  ? "https://www.wowhead.com/npc=12345/..."
                  : "Enter Display ID number..."
            }
            value={value.value}
            onChange={(e) => onChange({ ...value, value: e.target.value })}
            className={`border-2 bg-white ${error ? "border-red-500" : "border-gray-300 focus:border-blue-500"}`}
          />
          {error && (
            <div className="flex items-center gap-1 mt-1 text-sm text-red-600">
              <AlertCircle className="h-3 w-3" />
              {error}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

const validateRef = (ref: RefSchema, category: RefCategory): string | null => {
  if (ref.type === "wowhead" && !ref.value.startsWith(`https://www.wowhead.com/${category}=`)) {
    return "Invalid Wowhead URL"
  }
  if (ref.type === "displayID" && isNaN(Number(ref.value))) {
    return "Invalid Display ID"
  }
  return null
}

const attackTagOptions = [
  { value: "none", label: "None", description: "No attack animation" },
  { value: "1H", label: "1H Weapon", description: "One-handed weapon animation" },
  { value: "2H", label: "2H Weapon", description: "Two-handed weapon animation" },
  { value: "2HL", label: "2H Large Weapon", description: "Two-handed large weapon animation" },
  { value: "Unarmed", label: "Unarmed", description: "Unarmed attack animation" },
  { value: "Bow", label: "Bow", description: "Bow attack animation" },
  { value: "Rifle", label: "Rifle", description: "Rifle attack animation" },
  { value: "Thrown", label: "Thrown weapon animation" },
]

const sizeOptions = [
  { value: "none", label: "Default", description: "Use default character size" },
  { value: "small", label: "Small", description: "Small size category" },
  { value: "medium", label: "Medium", description: "Medium size category" },
  { value: "large", label: "Large", description: "Large size category" },
  { value: "hero", label: "Hero", description: "Hero size category" },
  { value: "semi-giant", label: "Semi-Giant", description: "Semi-giant size category" },
  { value: "giant", label: "Giant", description: "Giant size category" },
]

const portraitSuggestions = ["Stand", "Stand Ready"]

export default function WoWNPCExporter() {
  const [character, setCharacter] = useState<Character>({
    base: { type: "wowhead", value: "https://www.wowhead.com/npc=88002/highlord-darion-mograine" },
    inGameMovespeed: 270,
    attachItems: {},
    portraitCameraSequenceName: "Stand",
  })

  const [outputFileName, setOutputFileName] = useState("")
  const [format, setFormat] = useState<Format>("mdx")
  const [optimization, setOptimization] = useState({
    sortSequences: true,
    removeUnusedVertices: true,
    removeUnusedNodes: true,
    removeUnusedMaterialsTextures: true
  })

  const [isExporting, setIsExporting] = useState(false)
  const [exportResult, setExportResult] = useState<any>(null)

  const addAttachItem = () => {
    // Find the first unused attachment ID, starting with common ones
    const usedIds = new Set(Object.keys(character.attachItems || {}).map(Number))

    let newId = commonAttachments[0].id
    for (const attachment of [...commonAttachments, ...otherAttachments]) {
      if (!usedIds.has(attachment.id)) {
        newId = attachment.id
        break
      }
    }

    setCharacter({
      ...character,
      attachItems: {
        ...character.attachItems,
        [newId]: {
          path: { type: "wowhead", value: Object.keys(character.attachItems || {}).length === 0 ? "https://www.wowhead.com/item=40276/monster-sword-1h-highlord-darion-mograine-non-instanced" : "" },
        },
      },
    })
  }

  const removeAttachItem = (id: number) => {
    const newAttachItems = { ...character.attachItems }
    delete newAttachItems[id]
    setCharacter({ ...character, attachItems: newAttachItems })
  }

  const updateAttachItem = (id: number, item: AttachItem) => {
    setCharacter({
      ...character,
      attachItems: {
        ...character.attachItems,
        [id]: item,
      },
    })
  }

  const isValidForExport = useMemo(() => {
    // Check base model
    if (validateRef(character.base, "npc")) return false

    // Check output filename
    if (!outputFileName.trim()) return false

    // Check all attach items have valid references
    const attachItems = character.attachItems || {}
    for (const item of Object.values(attachItems)) {
      if (validateRef(item.path, "item")) return false
    }

    return true
  }, [character, outputFileName])

  const handleExport = async () => {
    setIsExporting(true)
    setExportResult(null)

    try {
      // Create a clean character object for export
      const exportCharacter = {
        ...character,
        attackTag: character.attackTag === undefined ? "" : character.attackTag,
      }

      const request: ExportRequest = {
        character: exportCharacter,
        outputFileName,
        optimization,
        format,
      }

      const response = await fetch(`${host}/export/character`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(request),
      })

      if (!response.ok) {
        throw new Error(await response.text())
      }

      const result = await response.json()
      setExportResult(result)
    } catch (error: any) {
      console.error("Export error:", error)
      setExportResult({ error: "Export failed. " + (error.error ?? error) })
    } finally {
      setIsExporting(false)
    }
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 p-4">
      <div className="max-w-6xl mx-auto space-y-4">
        <div className="text-center space-y-2">
          <h1 className="text-4xl font-bold text-gray-900">Huy's wow-converter</h1>
          <p className="text-lg text-gray-600">Easily export World of Warcraft NPC models</p>
          <p className="text-lg text-gray-600">Written by <b>pqhuy98</b> - <a href="https://github.com/pqhuy98" target="_blank" rel="noopener noreferrer">"wc3-sandbox"</a></p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* Character Configuration */}
          <Card>
            <CardHeader className="pb-4">
              <CardTitle className="flex items-center gap-2 text-lg">
                <User className="h-5 w-5" />
                Character Configuration
              </CardTitle>
              <CardDescription>Configure the base character model and its properties</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <RefInput
                value={character.base}
                onChange={(base) => setCharacter({ ...character, base })}
                label="Base Model"
                tooltipKey="baseModel"
                category="npc"
              />

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Label className="text-sm">Attack Animation</Label>
                    <TooltipProvider delayDuration={200}>
                      <Tooltip>
                        <TooltipTrigger>
                          <HelpCircle className="h-4 w-4 text-muted-foreground" />
                        </TooltipTrigger>
                        <TooltipContent>
                          <p className="max-w-xs">{tooltips.attackAnimation}</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </div>
                  <Select
                    value={character.attackTag || "none"}
                    onValueChange={(value: string) =>
                      setCharacter({
                        ...character,
                        attackTag: value === "none" ? undefined : (value as AttackTag),
                      })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Select attack type" />
                    </SelectTrigger>
                    <SelectContent align="start">
                      {attackTagOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          <div className="flex flex-col">
                            <span className="text-left">{option.label}</span>
                            <span className="text-xs text-muted-foreground">{option.description}</span>
                          </div>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Label className="text-sm">Character Size</Label>
                    <TooltipProvider delayDuration={200}>
                      <Tooltip>
                        <TooltipTrigger>
                          <HelpCircle className="h-4 w-4 text-muted-foreground" />
                        </TooltipTrigger>
                        <TooltipContent>
                          <p className="max-w-xs">{tooltips.characterSize}</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </div>
                  <Select
                    value={character.size || "none"}
                    onValueChange={(value: string) =>
                      setCharacter({
                        ...character,
                        size: value === "none" ? undefined : (value as Size),
                      })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="Select size" />
                    </SelectTrigger>
                    <SelectContent align="start">
                      {sizeOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          <div className="flex flex-col">
                            <span className="text-left">{option.label}</span>
                            <span className="text-xs text-muted-foreground">{option.description}</span>
                          </div>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="flex items-center gap-3">
                  <Label htmlFor="movespeed" className="text-sm min-w-fit">
                    Movement Speed
                  </Label>
                  <TooltipProvider delayDuration={200}>
                    <Tooltip>
                      <TooltipTrigger>
                        <HelpCircle className="h-4 w-4 text-muted-foreground" />
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="max-w-xs">{tooltips.movementSpeed}</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                  <Input
                    id="movespeed"
                    type="number"
                    step="1"
                    value={character.inGameMovespeed}
                    onChange={(e) =>
                      setCharacter({ ...character, inGameMovespeed: Number.parseInt(e.target.value) || 270 })
                    }
                    className="flex-1 border-2 border-gray-300 bg-white focus:border-blue-500"
                  />
                </div>

                <div className="flex items-center gap-3">
                  <Label htmlFor="scale" className="text-sm min-w-fit">
                    Scale Multiplier
                  </Label>
                  <TooltipProvider delayDuration={200}>
                    <Tooltip>
                      <TooltipTrigger>
                        <HelpCircle className="h-4 w-4 text-muted-foreground" />
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="max-w-xs">{tooltips.scaleMultiplier}</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                  <Input
                    id="scale"
                    type="number"
                    step="0.1"
                    placeholder="1.0"
                    value={character.scale || ""}
                    onChange={(e) =>
                      setCharacter({ ...character, scale: Number.parseFloat(e.target.value) || undefined })
                    }
                    className="flex-1 border-2 border-gray-300 bg-white focus:border-blue-500"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">

              <div className="space-y-3">
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="keepCinematic"
                    checked={character.keepCinematic || false}
                    onCheckedChange={(checked) => setCharacter({ ...character, keepCinematic: checked as boolean })}
                  />
                  <Label htmlFor="keepCinematic" className="flex items-center gap-2 text-sm">
                    Keep Cinematic Sequences
                    <TooltipProvider delayDuration={200}>
                      <Tooltip>
                        <TooltipTrigger>
                          <HelpCircle className="h-4 w-4 text-muted-foreground" />
                        </TooltipTrigger>
                        <TooltipContent>
                          <p className="max-w-xs">{tooltips.keepCinematic}</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </Label>
                </div>

                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="noDecay"
                    checked={character.noDecay || false}
                    onCheckedChange={(checked) => setCharacter({ ...character, noDecay: checked as boolean })}
                  />
                  <Label htmlFor="noDecay" className="flex items-center gap-2 text-sm">
                    No Decay
                    <TooltipProvider delayDuration={200}>
                      <Tooltip>
                        <TooltipTrigger>
                          <HelpCircle className="h-4 w-4 text-muted-foreground" />
                        </TooltipTrigger>
                        <TooltipContent>
                          <p className="max-w-xs">{tooltips.noDecay}</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </Label>
                </div>
              </div>

              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <Label htmlFor="portraitCamera" className="text-sm">
                    Portrait Camera Sequence
                  </Label>
                  <TooltipProvider delayDuration={200}>
                    <Tooltip>
                      <TooltipTrigger>
                        <HelpCircle className="h-4 w-4 text-muted-foreground" />
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="max-w-xs">{tooltips.portraitCamera}</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </div>
                <div className="relative">
                  <Input
                    id="portraitCamera"
                    placeholder="Enter sequence name..."
                    value={character.portraitCameraSequenceName || ""}
                    onChange={(e) =>
                      setCharacter({ ...character, portraitCameraSequenceName: e.target.value || undefined })
                    }
                    list="portrait-suggestions"
                    className="border-2 border-gray-300 bg-white focus:border-blue-500"
                  />
                  <datalist id="portrait-suggestions">
                    {portraitSuggestions.map((suggestion) => (
                      <option key={suggestion} value={suggestion} />
                    ))}
                  </datalist>
                </div>
              </div>
              </div>
            </CardContent>
          </Card>

          {/* Attached Items */}
          <Card>
            <CardHeader className="pb-4">
              <CardTitle className="flex items-center gap-2 text-lg">
                <Sword className="h-5 w-5" />
                Attached Items
              </CardTitle>
              <CardDescription>Add weapons, armor, and other items to attach to the character</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="space-y-3">
                {Object.entries(character.attachItems || {}).map(([id, item]) => {
                  const attachmentId = Number(id)
                  const usedIds = new Set(Object.keys(character.attachItems || {}).map(Number))
                  const attachmentName =
                    [...commonAttachments, ...otherAttachments].find((att) => att.id === attachmentId)?.name ||
                    "Unknown"

                  return (
                    <Card key={id} className="p-3 bg-blue-50 border-blue-200">
                      <div className="flex items-start justify-between mb-3">
                        <Badge variant="secondary" className="text-xs">
                          {attachmentName} ({attachmentId})
                        </Badge>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => removeAttachItem(attachmentId)}
                          className="text-red-500 hover:text-red-700 h-6 w-6 p-0"
                        >
                          <Trash2 className="h-3 w-3" />
                        </Button>
                      </div>

                      <div className="space-y-3">
                        <RefInput
                          value={item.path}
                          onChange={(path) => updateAttachItem(attachmentId, { ...item, path })}
                          label="Item Reference"
                          tooltipKey="itemReference"
                          category="item"
                        />

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                          <div className="space-y-2">
                            <div className="flex items-center gap-2">
                              <Label className="text-sm">Attachment Point</Label>
                              <TooltipProvider delayDuration={200}>
                                <Tooltip>
                                  <TooltipTrigger>
                                    <HelpCircle className="h-4 w-4 text-muted-foreground" />
                                  </TooltipTrigger>
                                  <TooltipContent>
                                    <p className="max-w-xs">{tooltips.attachmentPoint}</p>
                                  </TooltipContent>
                                </Tooltip>
                              </TooltipProvider>
                            </div>
                            <AttachmentSelector
                              value={attachmentId}
                              onChange={(newId) => {
                                // Move the item to the new attachment ID
                                const newAttachItems = { ...character.attachItems }
                                delete newAttachItems[attachmentId]
                                newAttachItems[newId] = item
                                setCharacter({ ...character, attachItems: newAttachItems })
                              }}
                              usedIds={usedIds}
                            />
                          </div>

                          <div className="flex items-end gap-3">
                            <div className="flex-1 space-y-2">
                              <div className="flex items-center gap-2">
                                <Label htmlFor={`scale-${id}`} className="text-sm">
                                  Scale
                                </Label>
                                <TooltipProvider delayDuration={200}>
                                  <Tooltip>
                                    <TooltipTrigger>
                                      <HelpCircle className="h-4 w-4 text-muted-foreground" />
                                    </TooltipTrigger>
                                    <TooltipContent>
                                      <p className="max-w-xs">{tooltips.itemScale}</p>
                                    </TooltipContent>
                                  </Tooltip>
                                </TooltipProvider>
                              </div>
                              <Input
                                id={`scale-${id}`}
                                type="number"
                                step="0.1"
                                placeholder="1.0"
                                value={item.scale || ""}
                                onChange={(e) =>
                                  updateAttachItem(attachmentId, {
                                    ...item,
                                    scale: Number.parseFloat(e.target.value) || undefined,
                                  })
                                }
                                className="border-2 border-gray-300 bg-white focus:border-blue-500"
                              />
                            </div>
                          </div>
                        </div>
                      </div>
                    </Card>
                  )
                })}
              </div>

              <Button onClick={addAttachItem} variant="outline" className="w-full bg-transparent">
                <Plus className="h-4 w-4 mr-2" />
                Add Attached Item
              </Button>
            </CardContent>
          </Card>
        </div>

        {/* Export Settings */}
        <Card>
          <CardHeader className="pb-4">
            <CardTitle className="text-lg">Export Configuration</CardTitle>
            <CardDescription>Configure output settings and optimizations</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4 items-end">
              <div className="space-y-2">
                <Label htmlFor="filename" className="text-sm">
                  Output File Name
                </Label>
                <Input
                  id="filename"
                  placeholder="my-character"
                  value={outputFileName}
                  onChange={(e) => setOutputFileName(e.target.value)}
                  className={`border-2 bg-white ${!outputFileName.trim() ? "border-red-500" : "border-gray-300 focus:border-blue-500"}`}
                />
              </div>

              <div className="space-y-2">
                <Label className="text-sm">Export Format</Label>
                <Select value={format} onValueChange={(value: Format) => setFormat(value)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent align="start">
                    <SelectItem value="mdx">.mdx (Binary)</SelectItem>
                    <SelectItem value="mdl">.mdl (Text)</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="md:col-span-2">
                <Button onClick={handleExport} disabled={isExporting || !isValidForExport} className="w-full" size="lg">
                  {isExporting ? (
                    <>
                      <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
                      Exporting...
                    </>
                  ) : (
                    <>
                      <Download className="h-4 w-4 mr-2" />
                      Export Character
                    </>
                  )}
                </Button>
              </div>
            </div>

            <Separator />

            <div className="space-y-3">
              <h3 className="text-base font-semibold">Optimization Options</h3>
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3">
                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="sortSequences"
                    checked={optimization.sortSequences}
                    onCheckedChange={(checked) =>
                      setOptimization({ ...optimization, sortSequences: checked as boolean })
                    }
                  />
                  <Label htmlFor="sortSequences" className="text-sm">
                    Sort Sequences
                  </Label>
                </div>

                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="removeUnusedVertices"
                    checked={optimization.removeUnusedVertices}
                    onCheckedChange={(checked) =>
                      setOptimization({ ...optimization, removeUnusedVertices: checked as boolean })
                    }
                  />
                  <Label htmlFor="removeUnusedVertices" className="text-sm">
                    Remove Unused Vertices
                  </Label>
                </div>

                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="removeUnusedNodes"
                    checked={optimization.removeUnusedNodes}
                    onCheckedChange={(checked) =>
                      setOptimization({ ...optimization, removeUnusedNodes: checked as boolean })
                    }
                  />
                  <Label htmlFor="removeUnusedNodes" className="text-sm">
                    Remove Unused Nodes
                  </Label>
                </div>

                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="removeUnusedMaterials"
                    checked={optimization.removeUnusedMaterialsTextures}
                    onCheckedChange={(checked) =>
                      setOptimization({ ...optimization, removeUnusedMaterialsTextures: checked as boolean })
                    }
                  />
                  <Label htmlFor="removeUnusedMaterials" className="text-sm">
                    Optimize Materials
                  </Label>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Export Results */}
        {exportResult && (
          <Card>
            <CardHeader className="pb-4">
              <CardTitle className="flex items-center gap-2 text-lg">
                {exportResult.error ? (
                  <>
                    <div className="h-5 w-5 rounded-full bg-red-500" />
                    Export Failed
                  </>
                ) : (
                  <>
                    <div className="h-5 w-5 rounded-full bg-green-500" />
                    Export Successful
                  </>
                )}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {exportResult.error ? (
                <p className="text-red-600">{exportResult.error}</p>
              ) : (
                <div className="space-y-4">
                  <div className="flex items-center gap-2">
                      <div className="flex items-center gap-2 w-full">
                        <Button
                          variant="outline"
                          size="icon"
                          onClick={() => {
                            navigator.clipboard.writeText(exportResult.outputDirectory)
                          }}
                          title="Copy output directory"
                        >
                          <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                            <rect x="9" y="9" width="13" height="13" rx="2" ry="2" stroke="currentColor" />
                            <path d="M5 15V5a2 2 0 0 1 2-2h10" stroke="currentColor" />
                          </svg>
                        </Button>
                        <span className="text-lg font-mono select-all">{exportResult.outputDirectory}</span> 
                      </div>
                    {/* <Button variant="outline" size="icon">
                      <Download className="h-4 w-4" />
                    </Button> */}
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div>
                      <h4 className="font-semibold mb-2">Exported Models:</h4>
                      <ul className="list-disc list-inside space-y-1">
                        {exportResult.exportedModels?.map((model: string, index: number) => (
                          <li key={index} className="text-sm">
                            {model}
                          </li>
                        ))}
                      </ul>
                    </div>
                    <div>
                      <h4 className="font-semibold mb-2">Exported Textures:</h4>
                      <ul className="list-disc list-inside space-y-1">
                        {exportResult.exportedTextures?.map((texture: string, index: number) => (
                          <li key={index} className="text-sm">
                            {texture}
                          </li>
                        ))}
                      </ul>
                    </div>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}

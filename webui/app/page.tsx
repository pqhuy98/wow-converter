"use client"

import { useState, useMemo, useEffect } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Checkbox } from "@/components/ui/checkbox"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { Plus, Trash2, Download, User, Sword, HelpCircle, AlertCircle, History } from "lucide-react"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { host } from "@/app/config"
import { isLocalRef } from "@/lib/utils"
import ModelViewerUi from "./model-viewer"
import { commonAttachments, otherAttachments, RefSchema, RefType, Character, AttachItem, ExportRequest, AttackTag, ModelFormat, ModelSize, JobStatus, ModelFormatVersion } from "@/lib/models/export-character.model"


// Tooltips organized in a record
const tooltips = {
  baseModel: "The base character model to use. Can be a Wowhead URL, local file inside wow.export folder, or Display ID number.",
  attackAnimation: "Determines which attack animations the character will use.",
  characterSize: "How tall the character is in the game.",
  movementSpeed: "Animation - walk speed (\"uwal\") of the unit in World Editor. The tool will try to slow down/speed up the Walk animations to match the Warcraft movement speed. If you experience a bug with too fast or too slow walk animation, set to 0 to keep the original WoW animation speed.",
  scaleMultiplier: "Additional scale multiplier (1.0 = no change, optional). Firstly the model will be scaled to match the character size, then this multiplier will be applied.",
  keepCinematic: "Preserve cinematic animation sequences in the exported model. Warning: WoW models have many cinematic sequences, this significantly increases file size.",
  noDecay: "Do not automatically add Decay animations.",
  portraitCamera: "Name of the sequence to use for positioning the character portrait camera. E.g. if later you use Stand Ready as default stand animation, the portrait camera needs to be placed lower since the model will usually hunch a bit.",
  itemReference: "The item to attach - can be a Wowhead URL, local file inside wow.export folder, or Display ID.",
  attachmentPoint: "Where on the character model this item will be attached",
  itemScale: "Additional scale multiplier for this specific item (1.0 = no change). Firstly the item will be scaled to match the character, then this multiplier will be applied.",
  sortSequences: "Sort animations by name in the order of: Stand, Walk, Attack, Spell, Death, Decay, Cinematic XXX.",
  removeUnusedVertices: "Remove geoset vertices that are not used by any geoset faces.",
  removeUnusedNodes: "Remove nodes that are not used in any geosets or do not contain used children nodes.",
  removeUnusedMaterials: "Remove materials and textures that are not used in any geosets.",
  optimizeKeyFrames: "Remove key frames that are not used in any animation, or are insignificant.",
  format: "Model format (MDX vs MDL). MDX is the binary format, the file is most compact and lowest file size. MDL is the text format for debugging purposes, the file is human readable when opened in text editors, at the cost of larger file size.",
  formatVersion: "Model format version (HD vs SD). HD models work in all Warcraft 3 Retail's Reforged and Classic graphics modes, it has the highest fidelity with precise WoW model data. However HD models cannot be opened in legacy modeling tools like Magos Model Editor. If you want to use those legacy tools for post-processing, choose SD 800 instead. WARNING: wow-converter might export very broken SD models on complex WoW models. SD conversion does not guaranteed to work, use at your own risk.",
}


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
        <TooltipProvider>
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
                  ? `https://www.wowhead.com/${category}=12345/...`
                  : "Enter Display ID number..."
            }
            value={value.value}
            onChange={(e) => onChange({ ...value, value: e.target.value })}
            className={`border-2 bg-white text-left ${error ? "border-red-500" : "border-gray-300 focus:border-blue-500"}`}
            style={{ direction: "rtl" }}
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
  if (ref.type === "wowhead") {
    // Allow URLs with expansion prefixes like /wotlk/, /classic/, etc. (only a-z characters)
    const wowheadPattern = new RegExp(`^https://www\\.wowhead\\.com/(?:[a-z\-]+/)?${category}=`)
    if (!wowheadPattern.test(ref.value)) {
      return "Invalid Wowhead URL, must contain /" + category + "=..."
    }
  }
  if (ref.type === "local" && !isLocalRef(ref.value)) {
    return "Invalid local file path"
  }
  if (ref.type === "displayID" && isNaN(Number(ref.value))) {
    return "Invalid Display ID"
  }
  return null
}

const attackTagOptions: { value: AttackTag | "all", label: string, description: string }[] = [
  { value: "all", label: "All", description: "Include all attack animations" },
  { value: "1H", label: "1H Weapon", description: "The model uses 1H weapon(s)" },
  { value: "2H", label: "2H Weapon", description: "The model uses a 2H weapon" },
  { value: "2HL", label: "2HL Weapon", description: "The model uses a 2H polearm" },
  { value: "Unarmed", label: "Unarmed", description: "The model uses fists and kicks" },
  // { value: "Bow", label: "Bow", description: "The model uses a bow." },
  // { value: "Rifle", label: "Rifle", description: "The model uses a rifle." },
  // { value: "Thrown", label: "Thrown", description: "The model uses a thrown weapon." },
]

const sizeOptions: { value: ModelSize | "none", label: string, description: string }[] = [
  { value: "none", label: "Default", description: "Original WoW size times 56" },
  { value: "small", label: "Small", description: "As tall as Undead Ghoul" },
  { value: "medium", label: "Medium", description: "As tall as Orc Grunt" },
  { value: "large", label: "Large", description: "As tall as Undead Abomination" },
  { value: "hero", label: "Hero", description: "As tall as Tauren Chieftain" },
  { value: "giant", label: "Giant", description: "As tall as Flesh Golem" },
]

const portraitSuggestions = ["Stand", "Stand Ready"]

export default function WoWNPCExporter() {
  const [character, setCharacter] = useState<Character>({
    base: { type: 'wowhead', value: 'https://www.wowhead.com/npc=71865/garrosh-hellscream' },
    size: 'hero',
    attackTag: '2H',
    inGameMovespeed: 270,
    attachItems: {
      1: {
        path: { type: 'wowhead', value: 'https://www.wowhead.com/item=28773/gorehowl' },
      },
    },
    portraitCameraSequenceName: 'Stand',
  })

  const [outputFileName, setOutputFileName] = useState(getNpcNameFromWowheadUrl(character.base.value) ?? "")
  useEffect(() => {
    if (character.base.type !== "wowhead") return
    const npcName = getNpcNameFromWowheadUrl(character.base.value)
    if (npcName) {
      setOutputFileName(npcName)
    }
  }, [character.base.value])

  const [format, setFormat] = useState<ModelFormat>("mdx")
  const [formatVersion, setFormatVersion] = useState<ModelFormatVersion>("1000")
  const [optimization, setOptimization] = useState({
    sortSequences: true,
    removeUnusedVertices: true,
    removeUnusedNodes: true,
    removeUnusedMaterialsTextures: true
  })

  const [isExporting, setIsExporting] = useState(false)

  // Job/queue tracking
  const [jobStatus, setJobStatus] = useState<JobStatus | null>(null)
  const [viewerModelPath, setViewerModelPath] = useState<string | undefined>(undefined)

  useEffect(() => {
    const checkExportResult = async () => {
      const res = await fetch(`${host}/export/character/demos`)
      const jobs = await res.json()
      if (jobs.length > 0) {
        setViewerModelPath(jobs[Math.floor(Math.random() * jobs.length)].result.exportedModels[0])
      }
    }
    checkExportResult()
  }, [])

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
    if (!isLocalRef(outputFileName)) return false

    // Check all attach items have valid references
    const attachItems = character.attachItems || {}
    for (const item of Object.values(attachItems)) {
      if (validateRef(item.path, "item")) return false
    }

    return true
  }, [character, outputFileName])

  const handleExport = async () => {
    setIsExporting(true)
    setJobStatus(null)

    try {
      // Prepare request
      const exportCharacter = {
        ...character,
        attackTag: character.attackTag === undefined ? "" : character.attackTag,
      }

      const request: ExportRequest = {
        character: exportCharacter,
        outputFileName,
        optimization,
        format,
        formatVersion,
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
      setJobStatus(result)
    } catch (error: any) {
      console.error("Export error:", error)
      setJobStatus({
        id: '',
        status: 'failed',
        position: null,
        result: null,
        error: error?.message || String(error),
        submittedAt: Date.now(),
      })
    } finally {
      setIsExporting(false)
    }
  }

  // is window focused?
  const [isWindowFocused, setIsWindowFocused] = useState(true)
  useEffect(() => {
    const handleFocus = () => setIsWindowFocused(true)
    const handleBlur = () => setIsWindowFocused(false)
    window.addEventListener('focus', handleFocus)
    window.addEventListener('blur', handleBlur)
  }, [])

  const [doneCount, setDoneCount] = useState(0)

  // Poll job status every 1s when a job is active
  useEffect(() => {
    if (!jobStatus || jobStatus.status === 'done' || jobStatus.status === 'failed') return

    let pendingFetches = 0
    const fetchJobStatus = async () => {
      console.log('fetching job status', jobStatus)
      try {
        pendingFetches++
        if (pendingFetches > 1) return
        const res = await fetch(`${host}/export/character/status/${jobStatus.id}`)
        if (!res.ok) {
          throw new Error(await res.text())
        }
        const data = await res.json()
        setJobStatus(data)

        if (data.status === 'pending') {
        } else if (data.status === 'processing') {
        } else if (data.status === 'done') {
          setDoneCount(doneCount + 1)
          setViewerModelPath(data.result.exportedModels[0])
          clearInterval(interval)
        } else if (data.status === 'failed') {
          clearInterval(interval)
        }
      } catch (e: any) {
        console.error('Polling error:', e)
        setJobStatus({
          id: '',
          status: 'failed',
          position: null,
          result: null,
          error: e?.message || String(e),
          submittedAt: Date.now(),
        })
        clearInterval(interval)
      } finally {
        pendingFetches--
      }
    }

    const interval = setInterval(fetchJobStatus, 1000)
    fetchJobStatus()

    return () => clearInterval(interval)
  }, [jobStatus?.id, isWindowFocused])

  /**
   * Download the exported assets as a ZIP by calling the new POST /download API.
   */
  const handleDownloadZip = async () => {
    if (!jobStatus?.result) return

    const files = [
      ...(jobStatus.result.exportedModels || []),
      ...(jobStatus.result.exportedTextures || []),
    ]

    if (files.length === 0) {
      alert("Nothing to download – exported files list is empty")
      return
    }

    try {
      const res = await fetch(`${host}/download`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ files }),
      })

      if (!res.ok) {
        throw new Error(await res.text())
      }

      const blob = await res.blob()
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement("a")
      link.href = url
      link.download = `${outputFileName || "export"}.zip`
      document.body.appendChild(link)
      link.click()
      link.remove()
      window.URL.revokeObjectURL(url)
    } catch (e: any) {
      console.error("Download ZIP error:", e)
      alert(e?.message || String(e))
    }
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 p-4">
      <div className="max-w-6xl mx-auto space-y-4">
        {/* Navigation Header */}
        <div className="flex justify-between items-center mb-6">
          <div className="text-center flex-1">
            <h1 className="text-4xl font-bold text-gray-900">Huy's WOW-CONVERTER</h1>
            <p className="text-lg text-gray-600">Easily export WoW NPC models into Warcraft 3 MDL/MDX</p>
          </div>
          <Button
            variant="outline"
            onClick={() => window.location.href = '/recents'}
            className="flex items-center gap-2"
          >
            <History className="h-4 w-4" />
            Recent Exports
          </Button>
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
                onChange={(base) => {
                  setCharacter({ ...character, base })
                }}
                label="Base Model"
                tooltipKey="baseModel"
                category="npc"
              />

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Label className="text-sm">Attack Animation</Label>
                    <TooltipProvider>
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
                    value={character.attackTag || "all"}
                    onValueChange={(value: AttackTag | "all") =>
                      setCharacter({
                        ...character,
                        attackTag: value === "all" ? undefined : (value as AttackTag),
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
                    <TooltipProvider>
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
                        size: value === "none" ? undefined : (value as ModelSize),
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
                    Animation Walk Speed
                  </Label>
                  <TooltipProvider>
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
                    value={character.inGameMovespeed || ""}
                    onChange={(e) =>
                      setCharacter({ ...character, inGameMovespeed: Number.parseInt(e.target.value) || 0 })
                    }
                    className="flex-1 border-2 border-gray-300 bg-white focus:border-blue-500"
                  />
                </div>

                <div className="flex items-center gap-3">
                  <Label htmlFor="scale" className="text-sm min-w-fit">
                    Scale Multiplier
                  </Label>
                  <TooltipProvider>
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
                    <TooltipProvider>
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
                    <TooltipProvider>
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
                  <TooltipProvider>
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
              <CardDescription>Add weapons and other items to attach to the character</CardDescription>
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
                              <TooltipProvider>
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
                                <TooltipProvider>
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
            <div className="grid grid-cols-1 md:grid-cols-8 gap-4 items-end">
              <div className="space-y-2 md:col-span-3">
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

              <div className="space-y-2 md:col-span-1">
                <Label className="text-sm flex items-center gap-2">
                  Export Format
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger>
                        <HelpCircle className="h-4 w-4 text-muted-foreground" />
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="max-w-xs">{tooltips.format}</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                  </Label>
                <Select value={format} onValueChange={(value: ModelFormat) => setFormat(value)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent align="start">
                    <SelectItem value="mdx">.mdx</SelectItem>
                    <SelectItem value="mdl">.mdl</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2 md:col-span-1">
                <Label className="text-sm flex items-center gap-2">
                  Model Version
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger>
                        <HelpCircle className="h-4 w-4 text-muted-foreground" />
                      </TooltipTrigger>
                      <TooltipContent>
                        <p className="max-w-xs">{tooltips.formatVersion}</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </Label>
                <Select value={formatVersion} onValueChange={(value: ModelFormatVersion) => setFormatVersion(value)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent align="start">
                    <SelectItem value="1000">1000 (HD)</SelectItem>
                    <SelectItem value="800">800 (SD, experimental)</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="md:col-span-3">
                <Button onClick={handleExport} disabled={isExporting || !isValidForExport || jobStatus?.status === 'pending' || jobStatus?.status === 'processing'} className="w-full" size="lg">
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
                  <Label htmlFor="sortSequences" className="text-sm flex items-center gap-2">
                    Sort Sequences
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger>
                          <HelpCircle className="h-4 w-4 text-muted-foreground" />
                        </TooltipTrigger>
                        <TooltipContent>
                          <p className="max-w-xs">{tooltips.sortSequences}</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
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
                  <Label htmlFor="removeUnusedVertices" className="text-sm flex items-center gap-2">
                    Remove Unused Vertices
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger>
                          <HelpCircle className="h-4 w-4 text-muted-foreground" />
                        </TooltipTrigger>
                        <TooltipContent>
                          <p className="max-w-xs">{tooltips.removeUnusedVertices}</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
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
                  <Label htmlFor="removeUnusedNodes" className="text-sm flex items-center gap-2">
                    Remove Unused Nodes
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger>
                          <HelpCircle className="h-4 w-4 text-muted-foreground" />
                        </TooltipTrigger>
                        <TooltipContent>
                          <p className="max-w-xs">{tooltips.removeUnusedNodes}</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
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
                  <Label htmlFor="removeUnusedMaterials" className="text-sm flex items-center gap-2">
                    Optimize Materials
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger>
                          <HelpCircle className="h-4 w-4 text-muted-foreground" />
                        </TooltipTrigger>
                        <TooltipContent>
                          <p className="max-w-xs">{tooltips.removeUnusedMaterials}</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </Label>
                </div>

                <div className="flex items-center space-x-2">
                  <Checkbox
                    id="optimizeKeyFrames"
                    disabled
                    checked={true}
                  />
                  <Label htmlFor="optimizeKeyFrames" className="text-sm flex items-center gap-2">
                    Optimize Key Frames
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger>
                          <HelpCircle className="h-4 w-4 text-muted-foreground" />
                        </TooltipTrigger>
                        <TooltipContent>
                          <p className="max-w-xs">{tooltips.optimizeKeyFrames}</p>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </Label>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="pt-6">
          {jobStatus && jobStatus.status !== 'done' && (
            <CardContent className="py-6">
              {jobStatus.status === 'pending' && (
                <p className="text-center">Your request is queued. {jobStatus.position ? `Position: ${jobStatus.position}` : ''}</p>
              )}
              {jobStatus.status === 'processing' && (
                <p className="text-center">Your request is being processed...</p>
              )}
              {jobStatus.status === 'failed' && (
                <p className="text-center text-red-600">{jobStatus.error || 'Job failed'}</p>
              )}
            </CardContent>
          )}
          {jobStatus?.result && (
            <CardHeader className="pb-4 pt-0">
              <CardTitle className="flex items-center gap-2 text-lg">
                {jobStatus.error ? (
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
          )}
          <CardContent>
            {jobStatus?.error && <p className="text-red-600">{jobStatus.error}</p>}
            <div className="space-y-4">
              {jobStatus?.result && <div className="flex-col items-center gap-10">
                {jobStatus.result.outputDirectory && <div className="flex items-center gap-2 w-full">
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => {
                      navigator.clipboard.writeText(jobStatus.result!.outputDirectory!)
                    }}
                    title="Copy output directory"
                  >
                    <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2" stroke="currentColor" />
                      <path d="M5 15V5a2 2 0 0 1 2-2h10" stroke="currentColor" />
                    </svg>
                  </Button>
                  <span className="text-lg font-mono select-all">{jobStatus.result!.outputDirectory}</span> 
                </div>}
                <div className="flex items-center gap-2 w-full pt-2">
                  <Button variant="default" size="icon" onClick={handleDownloadZip}>
                    <Download className="h-4 w-4" />
                  </Button>
                  <span className="text-lg">Download: {outputFileName}.zip</span>
                </div>
              </div>}
              {viewerModelPath && (
                <div className="h-[600px]">
                  <ModelViewerUi key={viewerModelPath + ":" + doneCount} modelPath={viewerModelPath} />
                </div>
              )}
              {jobStatus?.result && <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                  <h4 className="font-semibold mb-2">Exported Models:</h4>
                  <ul className="list-disc list-inside space-y-1">
                    {jobStatus.result.exportedModels?.map((model: string, index: number) => (
                      <li key={index} className="text-sm">
                        {jobStatus.result!.versionId ? model.replace("__" + jobStatus.result!.versionId, '') : model}
                      </li>
                    ))}
                  </ul>
                </div>
                <div>
                  <h4 className="font-semibold mb-2">Exported Textures:</h4>
                  <ul className="list-disc list-inside space-y-1">
                    {jobStatus.result.exportedTextures?.map((texture: string, index: number) => (
                      <li key={index} className="text-sm">
                        {texture}
                      </li>
                    ))}
                  </ul>
                </div>
              </div>}
            </div>
          </CardContent>
        </Card>
      </div>
      <div className="text-lg text-center text-gray-600 mt-4">
        Created by <a href="https://github.com/pqhuy98" target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:underline">wc3-sandbox</a>
        {" | "}
        <a href="https://github.com/pqhuy98/wow-converter" target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:underline">Source code</a>
        {" | "}
        <a href="https://www.youtube.com/@wc3-sandbox" target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:underline">YouTube</a>
      </div>
    </div>
  )
}

function getNpcNameFromWowheadUrl(url: string) {
  // extract npc name from ...npc=1234/name, handling expansion prefixes
  url = url.split("#")[0].split("?")[0]
  const parts = url.split("/")
  // Find the part that contains the category=id/name pattern
  for (let i = parts.length - 1; i >= 0; i--) {
    const part = parts[i]
    if (part.includes("=")) {
      const npcName = (parts[i+1] || parts[i]).split("=").pop()
      return npcName
    }
  }
  return undefined
}

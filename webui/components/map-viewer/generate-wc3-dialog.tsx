'use client';

import { LoaderCircle, TriangleAlert } from 'lucide-react';
import {
  type ReactNode, useCallback, useEffect, useRef, useState,
} from 'react';

import { TooltipHelp } from '@/components/common/tooltip-help';
import { useServerConfig } from '@/components/server-config';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { GenerateWc3FormValues, MapTextureSettings } from '@/lib/models/map-generate.model';
import {
  defaultGenerateWc3FormValues, MAP_CLAMP_STORAGE_KEY, MAP_SAVE_NAME_BASE_REGEX, mapClampCacheKey,
  readMapClampCache, readMapExportPreferences, touchMapClampCache,
} from '@/lib/models/map-generate.model';
import type { MapPreviewData } from '@/lib/models/map-preview.model';
import { fitTerrainClampRange, minimumTerrainClampWidth, readMapPreviewStream } from '@/lib/models/map-preview.model';
import {
  normalizeMapSaveName,
  stripMapSaveNameExtension,
} from '@/lib/utils/map-save-name';
import {
  isMapExpansionAfterWotlk,
  isMapExpansionWithinWotlk,
} from '@/lib/utils/wow-expansions';

import { MapExportPreview } from './map-export-preview';
import { TerrainClampSlider } from './terrain-clamp-slider';

const MAP_ANGLE_OPTIONS = [0, 90, 180, 270] as const;
const MODEL_TEXTURE_OPTIONS = [256, 512, 1024, 0];
const TEXTURE_FIELDS: { key: keyof MapTextureSettings; label: string; options: number[] }[] = [
  { key: 'terrain', label: 'Terrain', options: [1024, 2048, 4096, 8192] },
  { key: 'unit', label: 'Unit', options: MODEL_TEXTURE_OPTIONS },
  { key: 'doodad', label: 'Doodad', options: MODEL_TEXTURE_OPTIONS },
];

const tooltips = {
  terrainClamp:
    'WoW ground can be much taller than a WC3 map allows. This slider picks the lowest and highest ground that become hills—anything outside that range is left flat or cut off. On very steep maps, use a smaller range so mountains do not look squashed.',
  suggestClamp:
    'Suggest a height range from NPC elevations in the selected cells. Applies the suggestion to the slider so you can review and change it before exporting.',
  unitScale: 'Size multiplier for NPC units on the map. Higher values make NPCs larger relative to terrain and doodads. Also affects the suggested terrain height clamp.',
  mapAngle: 'Rotates the exported map. Use this option to change terrain facing.',
  exportNpcs: 'Export WoW NPC models from the selected area. In unit mode, NPCs outside playable heights or too far from the ground are not placed; their models are still included.',
  allNpcsAsDoodads:
    'Place all NPCs as destructibles instead of units. Destructibles keep their exact height; WC3 units are snapped to the ground, which can look wrong on slopes or floating placements.',
  placePathBlockers:
    'Use NPC-reachable collision surfaces to place ground and air path blockers as doodads. Works even when Export NPCs is off.',
  terrainAsWc3Tilesets:
    'Paint playable ground with generated Warcraft III tileset textures instead of covering it with ADT models. This significantly increases in-game performance, but lowers terrain visual quality.',
  freshExport: 'Delete the existing map folder before converting. Unchecked reuses cached models and textures.',
  includeBuildingInteriors:
    'Include interior-only building props such as chairs and tables. Exterior decorations are always included. Props with missing or ambiguous group data are kept.',
};

function FieldLabelRow({
  htmlFor,
  children,
  tooltip,
}: {
  htmlFor?: string;
  children: ReactNode;
  tooltip?: string;
}) {
  return (
    <div className="flex min-h-5 items-center gap-2">
      <Label htmlFor={htmlFor} className="text-sm">{children}</Label>
      {tooltip ? <TooltipHelp tooltips={tooltip} /> : <span className="size-4 shrink-0" aria-hidden />}
    </div>
  );
}

interface GenerateWc3DialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  mapDir: string | null;
  mapId: number | string | null;
  expansionID?: number;
  defaultMapSaveName: string;
  selectedTiles: { x: number; y: number }[];
  onConfirm: (values: GenerateWc3FormValues) => void;
}

export default function GenerateWc3Dialog({
  open,
  onOpenChange,
  mapDir,
  mapId,
  expansionID,
  defaultMapSaveName,
  selectedTiles,
  onConfirm,
}: GenerateWc3DialogProps) {
  const [values, setValues] = useState<GenerateWc3FormValues>(defaultGenerateWc3FormValues);
  const { buildKey, isClassic, product } = useServerConfig();
  const preferenceStorageKey = `wow-converter.map-textures.${product || (isClassic ? 'wow_classic' : 'wow')}`;
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [preview, setPreview] = useState<MapPreviewData | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewProgress, setPreviewProgress] = useState('Preparing map preview (0/1)');
  const [previewError, setPreviewError] = useState('');
  const [suggesting, setSuggesting] = useState(false);
  const [suggestError, setSuggestError] = useState('');
  const suggestionRef = useRef<AbortController | null>(null);
  const settingsSelectionRef = useRef('');
  const settingsBySelectionRef = useRef(new Map<string, GenerateWc3FormValues>());
  const autoSuggestionRef = useRef(new Set<string>());
  const [creatureCheck, setCreatureCheck] = useState({
    loading: false,
    hasCreatures: false,
    count: 0,
  });

  useEffect(() => {
    if (!open) setIsFullscreen(false);
  }, [open]);

  const selectedTilesKey = [...new Set(selectedTiles.map((t) => `${t.x},${t.y}`))]
    .sort()
    .join('|');

  const suggestionSelectionKey = `${mapDir?.toLowerCase()}:${selectedTilesKey}`;
  const clampCacheKey = mapId == null ? '' : mapClampCacheKey(mapId, selectedTiles);
  const rememberClamp = useCallback((lower: number, upper: number) => {
    if (!clampCacheKey) return;
    try {
      const entries = readMapClampCache(localStorage.getItem(MAP_CLAMP_STORAGE_KEY));
      localStorage.setItem(MAP_CLAMP_STORAGE_KEY, JSON.stringify(touchMapClampCache(entries, { key: clampCacheKey, lower, upper })));
    } catch { /* Clamp edits still work when browser storage is unavailable. */ }
  }, [clampCacheKey]);

  const savePreferences = (textures: MapTextureSettings, includeBuildingInteriors: boolean) => {
    try { localStorage.setItem(preferenceStorageKey, JSON.stringify({ ...textures, includeBuildingInteriors })); } catch { /* Keep using these settings when storage is unavailable. */ }
  };
  const npcDataSupported = isMapExpansionWithinWotlk(expansionID);
  const expansionAfterWotlk = isMapExpansionAfterWotlk(expansionID);
  const creaturesSupported = npcDataSupported && creatureCheck.hasCreatures && !creatureCheck.loading;
  const noNpcDataInArea = npcDataSupported && !creatureCheck.loading && !creatureCheck.hasCreatures;
  const minimumClampWidth = preview ? minimumTerrainClampWidth(preview) : 1;
  const clampUnavailable = !preview || previewLoading || minimumClampWidth > 1;
  const clampInvalid = clampUnavailable || values.clampUpper - values.clampLower < minimumClampWidth - 1e-12;

  useEffect(() => {
    if (!preview || minimumClampWidth > 1) return;
    setValues((v) => {
      const [clampLower, clampUpper] = fitTerrainClampRange(v.clampLower, v.clampUpper, minimumClampWidth);
      return clampLower === v.clampLower && clampUpper === v.clampUpper ? v : { ...v, clampLower, clampUpper };
    });
  }, [preview, minimumClampWidth, values.clampLower, values.clampUpper]);

  useEffect(() => {
    if (!open || !mapDir || !selectedTilesKey || !npcDataSupported) {
      setCreatureCheck({ loading: false, hasCreatures: false, count: 0 });
      return undefined;
    }
    const controller = new AbortController();
    setCreatureCheck({ loading: true, hasCreatures: false, count: 0 });
    void (async () => {
      try {
        const res = await fetch(`/api/maps/${encodeURIComponent(mapDir)}/creatures-check`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ tiles: selectedTilesKey.split('|').map((part) => { const [x, y] = part.split(',').map(Number); return { x, y }; }) }),
          signal: controller.signal,
        });
        if (!res.ok) throw new Error('Could not check NPC data.');
        const data: { hasCreatures: boolean; creatureCount: number } = await res.json();
        if (!controller.signal.aborted) setCreatureCheck({ loading: false, hasCreatures: data.hasCreatures, count: data.creatureCount });
      } catch {
        if (!controller.signal.aborted) setCreatureCheck({ loading: false, hasCreatures: false, count: 0 });
      }
    })();
    return () => controller.abort();
  }, [open, mapDir, selectedTilesKey, buildKey, npcDataSupported]);

  useEffect(() => {
    if (!open) return;
    let saved: string | null = null;
    try { saved = localStorage.getItem(preferenceStorageKey); } catch { /* Storage may be unavailable. */ }
    const { includeBuildingInteriors, ...textures } = readMapExportPreferences(saved, isClassic);
    setValues((v) => ({ ...v, textures, includeBuildingInteriors }));
  }, [open, preferenceStorageKey, isClassic]);

  useEffect(() => {
    if (!open || !mapDir || !selectedTilesKey) return;
    const selection = `${buildKey}:${mapDir.toLowerCase()}:${selectedTilesKey}`;
    if (settingsSelectionRef.current === selection) return;
    if (settingsSelectionRef.current) settingsBySelectionRef.current.set(settingsSelectionRef.current, values);
    settingsSelectionRef.current = selection;
    setValues((v) => ({
      ...(settingsBySelectionRef.current.get(selection) ?? {
        ...defaultGenerateWc3FormValues,
        mapSaveName: defaultMapSaveName,
      }),
      textures: v.textures,
      includeBuildingInteriors: v.includeBuildingInteriors,
    }));
  }, [open, mapDir, selectedTilesKey, defaultMapSaveName, values, buildKey]);

  useEffect(() => {
    if (!open || !clampCacheKey || !selectedTilesKey) return;
    try {
      const entries = readMapClampCache(localStorage.getItem(MAP_CLAMP_STORAGE_KEY));
      const saved = entries.find((entry) => entry.key === clampCacheKey);
      if (!saved) return;
      autoSuggestionRef.current.add(suggestionSelectionKey);
      setValues((v) => ({ ...v, clampLower: saved.lower, clampUpper: saved.upper }));
      localStorage.setItem(MAP_CLAMP_STORAGE_KEY, JSON.stringify(touchMapClampCache(entries, saved)));
    } catch { /* Use the in-memory selection settings when storage is unavailable. */ }
  }, [open, clampCacheKey, selectedTilesKey, suggestionSelectionKey, buildKey]);

  useEffect(() => {
    suggestionRef.current?.abort();
    setSuggesting(false); setSuggestError('');
    if (!open || !mapDir || selectedTilesKey === '') {
      setPreview(null); setPreviewLoading(false);
      return undefined;
    }
    const controller = new AbortController();
    setPreview(null); setPreviewLoading(true); setPreviewError('');
    setPreviewProgress('Checking preview cache (0/1)');
    void (async () => {
      try {
        const res = await fetch(`/api/maps/${encodeURIComponent(mapDir)}/preview?progress=1`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ tiles: selectedTilesKey.split('|').map((part) => { const [x, y] = part.split(',').map(Number); return { x, y }; }) }),
          signal: controller.signal,
        });
        const data = await readMapPreviewStream(res, (progress) => {
          if (!controller.signal.aborted) setPreviewProgress(`${progress.message} (${progress.completed}/${progress.total})`);
        });
        if (!controller.signal.aborted) {
          setPreview(data);
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setPreviewError(err instanceof Error ? err.message : 'Could not prepare the map preview.');
        }
      } finally {
        if (!controller.signal.aborted) setPreviewLoading(false);
      }
    })();
    return () => { controller.abort(); suggestionRef.current?.abort(); };
  }, [open, mapDir, selectedTilesKey, buildKey]);

  const suggestClamp = useCallback(async () => {
    if (!mapDir || !creaturesSupported) return;
    suggestionRef.current?.abort();
    const controller = new AbortController(); suggestionRef.current = controller;
    setSuggesting(true); setSuggestError('');
    try {
      const res = await fetch(`/api/maps/${encodeURIComponent(mapDir)}/suggest-clamp`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tiles: selectedTilesKey.split('|').map((part) => { const [x, y] = part.split(',').map(Number); return { x, y }; }), unitScale: values.unitScale }),
        signal: controller.signal,
      });
      if (!res.ok) { const failure: { error?: string } = await res.json(); throw new Error(failure.error ?? 'Could not suggest a height range.'); }
      const data: { lower: number; upper: number } = await res.json();
      if (!controller.signal.aborted) {
        setValues((v) => ({ ...v, clampLower: data.lower, clampUpper: data.upper }));
        rememberClamp(data.lower, data.upper);
      }
    } catch (err) {
      if (!controller.signal.aborted) setSuggestError(err instanceof Error ? err.message : 'Could not suggest a height range.');
    } finally { if (!controller.signal.aborted) setSuggesting(false); }
  }, [mapDir, creaturesSupported, selectedTilesKey, values.unitScale, rememberClamp]);

  useEffect(() => {
    if (!open || !creaturesSupported || autoSuggestionRef.current.has(suggestionSelectionKey)) return;
    autoSuggestionRef.current.add(suggestionSelectionKey);
    void suggestClamp();
  }, [open, creaturesSupported, suggestionSelectionKey, suggestClamp]);

  const trimmedName = values.mapSaveName.trim();
  const nameInvalid = trimmedName.length === 0 || !MAP_SAVE_NAME_BASE_REGEX.test(trimmedName);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={`gap-0 overflow-y-auto p-0 lg:overflow-hidden ${isFullscreen
          ? 'h-dvh w-dvw max-w-none rounded-none border-0 sm:rounded-none'
          : 'h-[85dvh] w-[calc(100vw-2rem)] max-w-[94rem]'}`}
        onEscapeKeyDown={(event) => {
          if (isFullscreen) {
            event.preventDefault();
            setIsFullscreen(false);
          }
        }}
      >
        <div className="grid min-h-0 lg:h-full lg:grid-cols-[28rem_minmax(0,1fr)]">
        <form
          className="space-y-5 p-6 lg:min-h-0 lg:overflow-y-auto"
          onSubmit={(e) => {
            e.preventDefault();
            if (nameInvalid || creatureCheck.loading || suggesting || clampInvalid) return;
            rememberClamp(values.clampLower, values.clampUpper);
            onConfirm({
              ...values,
              mapSaveName: normalizeMapSaveName(values.mapSaveName),
              placePathBlockers: creaturesSupported && values.placePathBlockers,
              creatures: creaturesSupported
                ? values.creatures
                : { enable: false, allAreDoodads: false },
            });
            onOpenChange(false);
          }}
        >
        <DialogHeader>
          <DialogTitle>Generate WC3 map</DialogTitle>
          <DialogDescription>
            Export {selectedTiles.length} selected tile{selectedTiles.length === 1 ? '' : 's'}, then convert to a Warcraft III map.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-6 py-2">
          <div className="space-y-2">
            <Label htmlFor="mapSaveName">Map save name</Label>
            <div className="flex items-center gap-2">
              <Input
                id="mapSaveName"
                className="flex-1"
                value={values.mapSaveName}
                onChange={(e) => setValues((v) => ({
                  ...v,
                  mapSaveName: stripMapSaveNameExtension(e.target.value),
                }))}
                placeholder="my-map"
              />
              <span className="shrink-0 text-sm text-muted-foreground">.w3x</span>
            </div>
            {nameInvalid && trimmedName.length > 0 ? (
              <p className="text-xs text-red-700 dark:text-red-400">
                Use letters, numbers, underscores, dots, and hyphens only.
              </p>
            ) : null}
          </div>

          {expansionAfterWotlk && (
            <Alert className="border-yellow-500/50 bg-yellow-500/10 py-3 text-yellow-800 dark:text-yellow-300 [&>svg]:text-yellow-600 dark:[&>svg]:text-yellow-400">
              <TriangleAlert className="h-4 w-4" />
              <AlertDescription>
                This map is from an expansion after Wrath of the Lich King, so clamp suggestions, unit scale, NPC export, and path blockers are unavailable.
              </AlertDescription>
            </Alert>
          )}

          {noNpcDataInArea && (
            <Alert className="border-yellow-500/50 bg-yellow-500/10 py-3 text-yellow-800 dark:text-yellow-300 [&>svg]:text-yellow-600 dark:[&>svg]:text-yellow-400">
              <TriangleAlert className="h-4 w-4" />
              <AlertDescription>
                This selected area has no NPC data, so clamp suggestions, unit scale, NPC export, and path blockers are unavailable.
              </AlertDescription>
            </Alert>
          )}

          <div className={creaturesSupported ? 'grid gap-x-4 gap-y-2 sm:grid-cols-2' : ''}>
            <div className="flex items-center gap-3">
              <FieldLabelRow htmlFor="mapAngleDeg" tooltip={tooltips.mapAngle}>Map angle</FieldLabelRow>
              <Select
                value={String(values.mapAngleDeg)}
                onValueChange={(v) => setValues((prev) => ({
                  ...prev,
                  mapAngleDeg: Number(v),
                }))}
              >
                <SelectTrigger id="mapAngleDeg" className="h-9 w-[5.5rem] shrink-0">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {MAP_ANGLE_OPTIONS.map((angle) => (
                    <SelectItem key={angle} value={String(angle)}>
                      {angle}°
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {creaturesSupported && (
              <div className="flex items-center gap-3">
                <FieldLabelRow htmlFor="unitScale" tooltip={tooltips.unitScale}>Unit scale</FieldLabelRow>
                <Input
                  id="unitScale"
                  type="number"
                  min={0.1}
                  step={0.1}
                  className="h-9 w-20 shrink-0"
                  value={values.unitScale}
                  onChange={(e) => {
                    suggestionRef.current?.abort(); setSuggesting(false);
                    setValues((v) => ({
                      ...v,
                      unitScale: Math.max(0.1, parseFloat(e.target.value) || 1),
                    }));
                  }}
                />
              </div>
            )}
          </div>

          <div className="space-y-2 rounded-md border border-primary/60 bg-primary/10 px-3 py-2.5 ring-1 ring-primary/25">
            <div className="flex min-h-5 flex-wrap items-center gap-x-3 gap-y-1">
              <FieldLabelRow tooltip={tooltips.terrainClamp}>Terrain height clamp</FieldLabelRow>
              {creaturesSupported && (
                <div className="flex items-center gap-1.5">
                  <Button type="button" variant="outline" size="sm" disabled={suggesting} onClick={() => void suggestClamp()}>
                    {suggesting && <LoaderCircle className="mr-1.5 h-3 w-3 animate-spin" />}Suggest
                  </Button>
                  <TooltipHelp tooltips={tooltips.suggestClamp} />
                </div>
              )}
            </div>
              <TerrainClampSlider
                lower={values.clampLower}
                upper={values.clampUpper}
                minimumWidth={minimumClampWidth}
                disabled={clampUnavailable}
                onChange={(clampLower, clampUpper) => {
                  autoSuggestionRef.current.add(suggestionSelectionKey);
                  suggestionRef.current?.abort(); setSuggesting(false);
                  const [safeLower, safeUpper] = fitTerrainClampRange(clampLower, clampUpper, minimumClampWidth);
                  setValues((v) => ({ ...v, clampLower: safeLower, clampUpper: safeUpper }));
                  rememberClamp(safeLower, safeUpper);
                }}
              />
            {preview && minimumClampWidth > 1 && (
              <p className="text-xs text-red-700 dark:text-red-400" role="alert">This selection exceeds WC3’s 480-tile map limit even at the full height range. Select fewer cells.</p>
            )}
            {suggestError && <p className="text-xs text-red-700 dark:text-red-400" role="alert">{suggestError}</p>}
          </div>

          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={values.terrainAsWc3Tilesets}
              onCheckedChange={(checked) => setValues((v) => ({
                ...v,
                terrainAsWc3Tilesets: checked === true,
              }))}
            />
            Terrain as WC3 tilesets
            <TooltipHelp tooltips={tooltips.terrainAsWc3Tilesets} />
          </label>

          {npcDataSupported && creatureCheck.loading && (
            <p className="text-xs text-muted-foreground">
              Checking creatures in selected tiles…
            </p>
          )}

          {creaturesSupported && (
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={values.placePathBlockers}
                onCheckedChange={(checked) => setValues((v) => ({
                  ...v,
                  placePathBlockers: checked === true,
                }))}
              />
              Place path blockers
              <TooltipHelp tooltips={tooltips.placePathBlockers} />
            </label>
          )}

          {creaturesSupported && (
            <fieldset className="rounded-md border px-3 py-2">
              <legend className="px-1 text-sm font-medium">NPC</legend>
              <label className="flex items-center gap-2 text-sm">
                <Checkbox
                  checked={values.creatures.enable}
                  onCheckedChange={(checked) => setValues((v) => ({
                    ...v,
                    creatures: { ...v.creatures, enable: checked === true },
                  }))}
                />
                Export NPCs ({creatureCheck.count.toLocaleString()} units)
                <TooltipHelp tooltips={tooltips.exportNpcs} />
              </label>
              {values.creatures.enable && (
                <label className="mt-2 flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={values.creatures.allAreDoodads}
                    onCheckedChange={(checked) => setValues((v) => ({
                      ...v,
                      creatures: { ...v.creatures, allAreDoodads: checked === true },
                    }))}
                  />
                  All NPCs as destructibles
                  <TooltipHelp tooltips={tooltips.allNpcsAsDoodads} />
                </label>
              )}
              {values.creatures.enable && !values.creatures.allAreDoodads && (
                <div className="mt-2 flex items-center gap-3">
                  <FieldLabelRow htmlFor="playerFaction" tooltip="Your WoW affiliation determines NPC reactions and your WC3 slot: Horde is red Orc, Alliance is blue Human.">Player faction</FieldLabelRow>
                  <Select value={values.playerFaction} onValueChange={(faction) => {
                    if (faction === 'Horde' || faction === 'Alliance') setValues((v) => ({ ...v, playerFaction: faction }));
                  }}>
                    <SelectTrigger id="playerFaction" className="h-8 w-28 text-sm"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="Alliance">Alliance</SelectItem>
                      <SelectItem value="Horde">Horde</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              )}
            </fieldset>
          )}

          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={values.includeBuildingInteriors}
              onCheckedChange={(checked) => {
                const includeBuildingInteriors = checked === true;
                setValues((v) => ({ ...v, includeBuildingInteriors }));
                savePreferences(values.textures, includeBuildingInteriors);
              }}
            />
            Include building interiors
            <TooltipHelp tooltips={tooltips.includeBuildingInteriors} />
          </label>

          <fieldset className="space-y-2 rounded-md border px-3 py-2">
            <legend className="px-1 text-sm font-medium">Textures</legend>
            {TEXTURE_FIELDS.map(({ key, label, options }) => (
              <div key={key} className="flex items-center justify-between gap-3">
                <Label htmlFor={`${key}TextureSize`}>{label}</Label>
                <Select
                  value={String(values.textures[key])}
                  onValueChange={(value) => {
                    const textures = { ...values.textures, [key]: Number(value) };
                    setValues((v) => ({ ...v, textures }));
                    savePreferences(textures, values.includeBuildingInteriors);
                  }}
                >
                  <SelectTrigger id={`${key}TextureSize`} className="h-8 w-28 text-sm"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {options.map((size) => (
                      <SelectItem key={size} value={String(size)}>{key === 'terrain' ? `${size / 1024}k` : size === 0 ? 'Unlimited' : `${size}px`}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            ))}
          </fieldset>

          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={values.freshExport}
              onCheckedChange={(checked) => setValues((v) => ({ ...v, freshExport: checked === true }))}
            />
            Fresh export
            <TooltipHelp tooltips={tooltips.freshExport} />
          </label>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button
            type="submit"
            disabled={nameInvalid || creatureCheck.loading || suggesting || clampInvalid}
          >
            Generate
          </Button>
        </DialogFooter>
        </form>
        <MapExportPreview
          data={preview}
          values={values}
          loading={previewLoading}
          progress={previewProgress}
          error={previewError}
          isFullscreen={isFullscreen}
          onToggleFullscreen={() => setIsFullscreen((fullscreen) => !fullscreen)}
        />
        </div>
      </DialogContent>
    </Dialog>
  );
}

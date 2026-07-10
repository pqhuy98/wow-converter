'use client';

import { type ReactNode, useEffect, useState } from 'react';
import { TriangleAlert } from 'lucide-react';

import { TooltipHelp } from '@/components/common/tooltip-help';
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
import type { GenerateWc3FormValues } from '@/lib/models/map-generate.model';
import { defaultGenerateWc3FormValues, MAP_SAVE_NAME_BASE_REGEX } from '@/lib/models/map-generate.model';
import {
  isMapExpansionAfterWotlk,
  isMapExpansionWithinWotlk,
} from '@/lib/utils/wow-expansions';
import {
  normalizeMapSaveName,
  stripMapSaveNameExtension,
} from '@/lib/utils/map-save-name';

import { TerrainClampSlider } from './terrain-clamp-slider';

const MAP_ANGLE_OPTIONS = [0, 90, 180, 270] as const;

const tooltips = {
  terrainClamp:
    'WoW ground can be much taller than a WC3 map allows. This slider picks the lowest and highest ground that become hills—anything outside that range is left flat or cut off. On very steep maps, use a smaller range so mountains do not look squashed.',
  autoClamp:
    'Pick the height range automatically from where NPCs stand in the selected area, so exported units stay on usable ground.',
  unitScale: 'Size multiplier for NPC units on the map. Higher values make NPCs larger relative to terrain and doodads. Also affects automatic terrain height clamp.',
  mapAngle: 'Rotates the exported map. Use this option to change terrain facing.',
  exportNpcs: 'Export WoW NPC models from the selected area and place them on the WC3 map.',
  allNpcsAsDoodads:
    'Place all NPCs as destructible doodads instead of units. Doodads keep their exact height; WC3 units are snapped to the ground, which can look wrong on slopes or floating placements.',
  freshExport: 'Delete the existing map folder before converting. Unchecked reuses cached models and textures.',
  includeBuildingInteriors:
    'Export interior doodads inside buildings (chairs, tables, etc.). Exports more models and textures, so generation takes longer and uses more disk space.',
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
  expansionID?: number;
  defaultMapSaveName: string;
  selectedTiles: { x: number; y: number }[];
  onConfirm: (values: GenerateWc3FormValues) => void;
}

export default function GenerateWc3Dialog({
  open,
  onOpenChange,
  mapDir,
  expansionID,
  defaultMapSaveName,
  selectedTiles,
  onConfirm,
}: GenerateWc3DialogProps) {
  const [values, setValues] = useState<GenerateWc3FormValues>(defaultGenerateWc3FormValues);
  const [creatureCheck, setCreatureCheck] = useState({
    loading: false,
    hasCreatures: false,
    count: 0,
  });

  const selectedTilesKey = selectedTiles
    .map((t) => `${t.x},${t.y}`)
    .sort()
    .join('|');

  const npcDataSupported = isMapExpansionWithinWotlk(expansionID);
  const expansionAfterWotlk = isMapExpansionAfterWotlk(expansionID);
  const creaturesSupported = npcDataSupported && creatureCheck.hasCreatures && !creatureCheck.loading;
  const noNpcDataInArea = npcDataSupported && !creatureCheck.loading && !creatureCheck.hasCreatures;
  const showManualClamp = !creaturesSupported || !values.autoClampPercent;

  useEffect(() => {
    if (!open) return;
    setValues({
      ...defaultGenerateWc3FormValues,
      mapSaveName: defaultMapSaveName,
      creatures: {
        enable: false,
        allAreDoodads: false,
      },
    });
  }, [open, defaultMapSaveName]);

  useEffect(() => {
    if (!open || !mapDir || selectedTiles.length === 0 || expansionAfterWotlk) {
      setCreatureCheck({ loading: false, hasCreatures: false, count: 0 });
      return undefined;
    }

    let cancelled = false;
    setCreatureCheck((prev) => ({ ...prev, loading: true }));

    void (async () => {
      try {
        const res = await fetch(
          `/api/maps/${encodeURIComponent(mapDir)}/creatures-check`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ tiles: selectedTiles }),
            cache: 'no-store',
          },
        );
        if (cancelled) return;
        if (!res.ok) {
          setCreatureCheck({ loading: false, hasCreatures: false, count: 0 });
          return;
        }
        const data = (await res.json()) as { hasCreatures?: boolean; creatureCount?: number };
        setCreatureCheck({
          loading: false,
          hasCreatures: data.hasCreatures === true,
          count: data.creatureCount ?? 0,
        });
      } catch {
        if (!cancelled) {
          setCreatureCheck({ loading: false, hasCreatures: false, count: 0 });
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [open, mapDir, selectedTilesKey, selectedTiles, expansionAfterWotlk]);

  const trimmedName = values.mapSaveName.trim();
  const nameInvalid = trimmedName.length === 0 || !MAP_SAVE_NAME_BASE_REGEX.test(trimmedName);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (nameInvalid) return;
            onConfirm({
              ...values,
              mapSaveName: normalizeMapSaveName(values.mapSaveName),
              autoClampPercent: creaturesSupported ? values.autoClampPercent : false,
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
              <p className="text-xs text-destructive">
                Use letters, numbers, underscores, dots, and hyphens only.
              </p>
            ) : null}
          </div>

          {expansionAfterWotlk && (
            <Alert className="border-yellow-500/50 bg-yellow-500/10 py-3 text-yellow-800 dark:text-yellow-300 [&>svg]:text-yellow-600 dark:[&>svg]:text-yellow-400">
              <TriangleAlert className="h-4 w-4" />
              <AlertDescription>
                This map is from an expansion after Wrath of the Lich King, so auto terrain clamp, unit scale, and NPC export are unavailable.
              </AlertDescription>
            </Alert>
          )}

          {noNpcDataInArea && (
            <Alert className="border-yellow-500/50 bg-yellow-500/10 py-3 text-yellow-800 dark:text-yellow-300 [&>svg]:text-yellow-600 dark:[&>svg]:text-yellow-400">
              <TriangleAlert className="h-4 w-4" />
              <AlertDescription>
                This selected area has no NPC data, so auto terrain clamp, unit scale, and NPC export are unavailable.
              </AlertDescription>
            </Alert>
          )}

          <div className={creaturesSupported ? 'grid grid-cols-2 gap-x-4 gap-y-2' : ''}>
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
                  onChange={(e) => setValues((v) => ({
                    ...v,
                    unitScale: parseFloat(e.target.value) || 1,
                  }))}
                />
              </div>
            )}
          </div>

          <div className="space-y-2">
            <div className="flex min-h-5 flex-wrap items-center gap-x-3 gap-y-1">
              <FieldLabelRow tooltip={tooltips.terrainClamp}>Terrain height clamp</FieldLabelRow>
              {creaturesSupported && (
                <label className="flex items-center gap-1.5 text-sm">
                  <Checkbox
                    checked={values.autoClampPercent}
                    onCheckedChange={(checked) => setValues((v) => ({
                      ...v,
                      autoClampPercent: checked === true,
                    }))}
                  />
                  Auto
                  <TooltipHelp tooltips={tooltips.autoClamp} />
                </label>
              )}
            </div>
            {showManualClamp && (
              <TerrainClampSlider
                lower={values.clampLower}
                upper={values.clampUpper}
                onChange={(clampLower, clampUpper) => setValues((v) => ({
                  ...v,
                  clampLower,
                  clampUpper,
                }))}
              />
            )}
          </div>

          {npcDataSupported && creatureCheck.loading && (
            <p className="text-xs text-muted-foreground">
              Checking creatures in selected tiles…
            </p>
          )}

          {/* Use a fieldset only when grouping 2+ related inputs (see NPC below). */}
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
                  All NPCs as doodads
                  <TooltipHelp tooltips={tooltips.allNpcsAsDoodads} />
                </label>
              )}
            </fieldset>
          )}

          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={values.includeBuildingInteriors}
              onCheckedChange={(checked) => setValues((v) => ({
                ...v,
                includeBuildingInteriors: checked === true,
              }))}
            />
            Include building interiors
            <TooltipHelp tooltips={tooltips.includeBuildingInteriors} />
          </label>

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
            disabled={nameInvalid}
          >
            Generate
          </Button>
        </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

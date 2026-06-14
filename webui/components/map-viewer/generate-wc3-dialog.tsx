'use client';

import { type ReactNode, useEffect, useState } from 'react';

import { TooltipHelp } from '@/components/common/tooltip-help';
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
import type { GenerateWc3FormValues } from '@/lib/models/map-generate.model';
import { defaultGenerateWc3FormValues, MAP_SAVE_NAME_REGEX } from '@/lib/models/map-generate.model';

import { TerrainClampSlider } from './terrain-clamp-slider';

const tooltips = {
  terrainClamp: 'Maps a slice of WoW elevation onto WC3 terrain height. In steep areas with tall mountains, narrow the range to scale the map up—high ground outside the slice is left out of the converted terrain.',
  unitScale: 'Size multiplier for NPC units placed on the map. Also affects automatic terrain height clamp.',
  exportNpcs: 'Export WoW NPC models from the selected area and place them on the WC3 map.',
  allNpcsAsDoodads: 'Place all NPCs as destructible doodads instead of units.',
  freshExport: 'Delete the existing map folder before converting. Unchecked reuses cached models and textures.',
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
  defaultMapSaveName: string;
  tileCount: number;
  onConfirm: (values: GenerateWc3FormValues) => void;
}

export default function GenerateWc3Dialog({
  open,
  onOpenChange,
  defaultMapSaveName,
  tileCount,
  onConfirm,
}: GenerateWc3DialogProps) {
  const [values, setValues] = useState<GenerateWc3FormValues>(defaultGenerateWc3FormValues);

  useEffect(() => {
    if (!open) return;
    setValues({
      ...defaultGenerateWc3FormValues,
      mapSaveName: defaultMapSaveName,
    });
  }, [open, defaultMapSaveName]);

  const trimmedName = values.mapSaveName.trim();
  const nameInvalid = trimmedName.length === 0 || !MAP_SAVE_NAME_REGEX.test(trimmedName);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (nameInvalid) return;
            onConfirm(values);
            onOpenChange(false);
          }}
        >
        <DialogHeader>
          <DialogTitle>Generate WC3 map</DialogTitle>
          <DialogDescription>
            Export {tileCount} selected tile{tileCount === 1 ? '' : 's'}, then convert to a Warcraft III map.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="mapSaveName">Map save name</Label>
            <Input
              id="mapSaveName"
              value={values.mapSaveName}
              onChange={(e) => setValues((v) => ({ ...v, mapSaveName: e.target.value }))}
              placeholder="my-map.w3x"
            />
            {nameInvalid && trimmedName.length > 0 ? (
              <p className="text-xs text-destructive">
                Use letters, numbers, underscores, dots, and hyphens only (optional .w3x suffix).
              </p>
            ) : null}
          </div>

          <div className="space-y-2">
            <FieldLabelRow tooltip={tooltips.terrainClamp}>Terrain height clamp</FieldLabelRow>
            <TerrainClampSlider
              lower={values.clampLower}
              upper={values.clampUpper}
              onChange={(clampLower, clampUpper) => setValues((v) => ({
                ...v,
                clampLower,
                clampUpper,
              }))}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <FieldLabelRow htmlFor="mapAngleDeg">Map angle (°)</FieldLabelRow>
              <Input
                id="mapAngleDeg"
                type="number"
                step={1}
                value={values.mapAngleDeg}
                onChange={(e) => setValues((v) => ({ ...v, mapAngleDeg: parseFloat(e.target.value) || 0 }))}
              />
            </div>
            <div className="space-y-2">
              <FieldLabelRow htmlFor="unitScale" tooltip={tooltips.unitScale}>Unit scale</FieldLabelRow>
              <Input
                id="unitScale"
                type="number"
                min={0.1}
                step={0.1}
                value={values.unitScale}
                onChange={(e) => setValues((v) => ({
                  ...v,
                  unitScale: parseFloat(e.target.value) || 1,
                }))}
              />
            </div>
          </div>

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
              Export NPCs
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

'use client';

import { useEffect, useState } from 'react';

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
import { defaultGenerateWc3FormValues } from '@/lib/models/map-generate.model';
import { cn } from '@/lib/utils/css';

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

  const clampInvalid = values.clampUpper < values.clampLower;
  const nameInvalid = values.mapSaveName.trim().length === 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (clampInvalid || nameInvalid) return;
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
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label htmlFor="clampLower">Lower clamp %</Label>
              <Input
                id="clampLower"
                type="number"
                min={0}
                max={1}
                step={0.01}
                value={values.clampLower}
                onChange={(e) => setValues((v) => ({ ...v, clampLower: parseFloat(e.target.value) || 0 }))}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="clampUpper">Upper clamp %</Label>
              <Input
                id="clampUpper"
                type="number"
                min={0}
                max={1}
                step={0.01}
                value={values.clampUpper}
                onChange={(e) => setValues((v) => ({ ...v, clampUpper: parseFloat(e.target.value) || 0 }))}
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="mapAngleDeg">Map angle (degrees)</Label>
            <Input
              id="mapAngleDeg"
              type="number"
              step={1}
              value={values.mapAngleDeg}
              onChange={(e) => setValues((v) => ({ ...v, mapAngleDeg: parseFloat(e.target.value) || 0 }))}
            />
          </div>

          <fieldset className="space-y-3 rounded-md border p-3">
            <legend className="px-1 text-sm font-medium">Creatures</legend>
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={values.creatures.enable}
                onCheckedChange={(checked) => setValues((v) => ({
                  ...v,
                  creatures: { ...v.creatures, enable: checked === true },
                }))}
              />
              Export creatures
            </label>
            <div
              className={cn(
                'grid transition-[grid-template-rows] duration-300 ease-in-out',
                values.creatures.enable ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]',
              )}
            >
              <div className="min-h-0 overflow-hidden">
                <div
                  className={cn(
                    'space-y-3 pt-3 transition-opacity duration-300',
                    values.creatures.enable ? 'opacity-100' : 'pointer-events-none opacity-0',
                  )}
                  aria-hidden={!values.creatures.enable}
                >
                  <label className="flex items-center gap-2 text-sm">
                    <Checkbox
                      checked={values.creatures.allAreDoodads}
                      onCheckedChange={(checked) => setValues((v) => ({
                        ...v,
                        creatures: { ...v.creatures, allAreDoodads: checked === true },
                      }))}
                    />
                    All creatures as doodads
                  </label>
                  <div className="space-y-2">
                    <Label htmlFor="creatureScaleUp">Creature scale up</Label>
                    <Input
                      id="creatureScaleUp"
                      type="number"
                      min={0.1}
                      step={0.1}
                      value={values.creatures.scaleUp}
                      onChange={(e) => setValues((v) => ({
                        ...v,
                        creatures: { ...v.creatures, scaleUp: parseFloat(e.target.value) || 1 },
                      }))}
                    />
                  </div>
                </div>
              </div>
            </div>
          </fieldset>

          <label className="flex items-start gap-2 text-sm">
            <Checkbox
              checked={values.freshExport}
              onCheckedChange={(checked) => setValues((v) => ({ ...v, freshExport: checked === true }))}
            />
            <span>
              Fresh export
              <span className="block text-muted-foreground text-xs mt-0.5">
                Delete the existing map folder before converting. Unchecked reuses cached models and textures.
              </span>
            </span>
          </label>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button
            type="submit"
            disabled={clampInvalid || nameInvalid}
          >
            Generate
          </Button>
        </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

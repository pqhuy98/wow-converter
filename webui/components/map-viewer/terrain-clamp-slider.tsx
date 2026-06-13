'use client';

import { Slider } from '@/components/ui/slider';

function clampPercent(value: number): number {
  return Math.round(Math.min(100, Math.max(0, value * 100)));
}

function percentToUnit(value: number): number {
  return value / 100;
}

interface TerrainClampSliderProps {
  lower: number;
  upper: number;
  onChange: (lower: number, upper: number) => void;
}

export function TerrainClampSlider({ lower, upper, onChange }: TerrainClampSliderProps) {
  const lowerPct = clampPercent(lower);
  const upperPct = clampPercent(upper);

  return (
    <div className="space-y-2">
      <Slider
        min={0}
        max={100}
        step={1}
        value={[lowerPct, upperPct]}
        onValueChange={([nextLower, nextUpper]) => {
          onChange(percentToUnit(nextLower), percentToUnit(nextUpper));
        }}
        aria-label="Terrain height clamp range"
      />
      <div className="flex justify-between text-xs text-muted-foreground tabular-nums">
        <span>{lowerPct}%</span>
        <span>{upperPct}%</span>
      </div>
    </div>
  );
}

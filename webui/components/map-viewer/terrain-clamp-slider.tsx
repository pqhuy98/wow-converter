'use client';

import { Slider } from '@/components/ui/slider';

function clampPercent(value: number): number {
  return Math.round(Math.min(100, Math.max(0, value * 100)) * 100) / 100;
}

function percentToUnit(value: number): number {
  return value / 100;
}

interface TerrainClampSliderProps {
  lower: number;
  upper: number;
  minimumWidth: number;
  disabled?: boolean;
  onChange: (lower: number, upper: number) => void;
}

export function TerrainClampSlider({
  lower, upper, minimumWidth, disabled, onChange,
}: TerrainClampSliderProps) {
  const lowerPct = clampPercent(lower);
  const upperPct = clampPercent(upper);

  return (
    <div className="space-y-2">
      <Slider
        className="[&>span:first-child]:bg-slate-400/50 [&>span:first-child]:ring-1 [&>span:first-child]:ring-inset [&>span:first-child]:ring-slate-400/40"
        min={0}
        max={100}
        step={0.1}
        minStepsBetweenThumbs={Math.min(1000, Math.ceil(minimumWidth * 1000 - 1e-9))}
        disabled={disabled}
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

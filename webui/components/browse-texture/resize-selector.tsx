'use client';

import { useCallback, useMemo } from 'react';

import { TooltipHelp } from '@/components/common/tooltip-help';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import type { IconResizeMode, IconSize } from '@/lib/models/icon-export.model';

interface ResizeSelectorProps {
  size: IconSize;
  resizeMode?: IconResizeMode;
  textureDimensions: { width: number; height: number } | null;
  onSizeChange: (size: IconSize) => void;
  onResizeModeChange?: (mode: IconResizeMode) => void;
  disabled?: boolean;
}

interface ResizeOption {
  value: string; // Format: "64-normal", "128-normal", "128-ai", etc.
  label: string;
  size: IconSize;
  mode: IconResizeMode;
  width: number;
}

function getCurrentValue(size: IconSize, resizeMode?: IconResizeMode, textureDimensions?: { width: number; height: number } | null): string {
  const mode = resizeMode ?? 'normal';
  if (size === 'original') {
    return `original-${mode}`;
  }
  const sizeNum = size === '64x64' ? 64 : size === '128x128' ? 128 : 256;
  // If texture matches size exactly or is larger, always use 'normal' mode (can't AI upscale)
  if (textureDimensions) {
    const { width, height } = textureDimensions;
    if (width >= sizeNum && height >= sizeNum) {
      return `${sizeNum}-normal`;
    }
  }
  return `${sizeNum}-${mode}`;
}

function parseValue(value: string): { size: IconSize; mode: IconResizeMode } {
  const [sizeNum, mode] = value.split('-');
  if (sizeNum === 'original') {
    return { size: 'original', mode: (mode as IconResizeMode) ?? 'normal' };
  }
  const size = sizeNum === '64' ? '64x64' : sizeNum === '128' ? '128x128' : '256x256';
  return { size, mode: (mode as IconResizeMode) ?? 'normal' };
}

function getLabel(
  size: IconSize,
  resizeMode: IconResizeMode | undefined,
  textureDimensions: { width: number; height: number } | null,
): string {
  if (size === 'original') {
    if (!textureDimensions) {
      return 'Original';
    }
    const { width } = textureDimensions;
    return `${width} (original)`;
  }
  if (!textureDimensions) {
    const sizeNum = size === '64x64' ? 64 : size === '128x128' ? 128 : 256;
    const mode = resizeMode ?? 'normal';
    return `${sizeNum} ${mode === 'ai' ? 'with AI' : 'Normal'}`;
  }
  const { width, height } = textureDimensions;
  const sizeNum = size === '64x64' ? 64 : size === '128x128' ? 128 : 256;
  const mode = resizeMode ?? 'normal';

  if (width === sizeNum && height === sizeNum) {
    return `${sizeNum} (original)`;
  }
  // If texture is larger than or equal to target size, always show Normal (can't AI upscale)
  if (width >= sizeNum && height >= sizeNum) {
    return `${sizeNum} Normal`;
  }
  return `${sizeNum} ${mode === 'ai' ? 'with AI' : 'Normal'}`;
}

function renderResizeOptions(
  textureDimensions: { width: number; height: number },
) {
  const { width, height } = textureDimensions;
  const is64 = width === 64 && height === 64;
  const is128 = width === 128 && height === 128;
  const is256 = width === 256 && height === 256;
  const isStandardSize = is64 || is128 || is256;

  const options: ResizeOption[] = [];

  // Original option - only show for non-standard sizes
  if (!isStandardSize) {
    options.push({
      value: 'original-normal',
      label: `${width} (original)`,
      size: 'original',
      mode: 'normal',
      width,
    });
  }

  // 64 Normal - always available
  options.push({
    value: '64-normal',
    label: is64 ? '64 (original)' : '64 Normal',
    size: '64x64',
    mode: 'normal',
    width: 64,
  });

  // 128 options
  if (width < 128 && height < 128) {
    // Can upscale to 128
    options.push({
      value: '128-normal',
      label: '128 Normal',
      size: '128x128',
      mode: 'normal',
      width: 128,
    });
    options.push({
      value: '128-ai',
      label: '128 with AI',
      size: '128x128',
      mode: 'ai',
      width: 128,
    });
  } else if (is128) {
    // Already 128x128
    options.push({
      value: '128-normal',
      label: '128 (original)',
      size: '128x128',
      mode: 'normal',
      width: 128,
    });
  } else if (width >= 128 && height >= 128) {
    // Can downscale to 128
    options.push({
      value: '128-normal',
      label: '128 Normal',
      size: '128x128',
      mode: 'normal',
      width: 128,
    });
  }

  // 256 options
  if (width < 256 && height < 256) {
    // Can upscale to 256
    options.push({
      value: '256-normal',
      label: '256 Normal',
      size: '256x256',
      mode: 'normal',
      width: 256,
    });
    options.push({
      value: '256-ai',
      label: '256 with AI',
      size: '256x256',
      mode: 'ai',
      width: 256,
    });
  } else if (is256) {
    // Already 256x256
    options.push({
      value: '256-normal',
      label: '256 (original)',
      size: '256x256',
      mode: 'normal',
      width: 256,
    });
  } else if (width >= 256 && height >= 256) {
    // Can downscale to 256
    options.push({
      value: '256-normal',
      label: '256 Normal',
      size: '256x256',
      mode: 'normal',
      width: 256,
    });
  }

  // Sort by width (ascending), then by mode (normal before AI)
  options.sort((a, b) => {
    if (a.width !== b.width) {
      return a.width - b.width;
    }
    // Same width: normal before AI
    if (a.mode === 'normal' && b.mode === 'ai') return -1;
    if (a.mode === 'ai' && b.mode === 'normal') return 1;
    return 0;
  });

  return (
    <>
      {options.map((option) => (
        <SelectItem key={option.value} value={option.value}>
          {option.label}
        </SelectItem>
      ))}
    </>
  );
}

export default function ResizeSelector({
  size,
  resizeMode,
  textureDimensions,
  onSizeChange,
  onResizeModeChange,
  disabled = false,
}: ResizeSelectorProps) {
  const currentValue = useMemo(() => getCurrentValue(size, resizeMode, textureDimensions), [size, resizeMode, textureDimensions]);
  const currentLabel = useMemo(() => getLabel(size, resizeMode, textureDimensions), [size, resizeMode, textureDimensions]);

  const handleValueChange = useCallback((value: string) => {
    const { size: newSize, mode: newMode } = parseValue(value);
    onSizeChange(newSize);
    if (onResizeModeChange) {
      onResizeModeChange(newMode);
    }
  }, [onSizeChange, onResizeModeChange]);

  // Check if both normal and AI options are available for current size
  const canToggleAi = useMemo(() => {
    if (!textureDimensions || size === 'original') {
      return false;
    }
    const { width, height } = textureDimensions;
    const targetSize = size === '64x64' ? 64 : size === '128x128' ? 128 : 256;
    // AI is available if texture is smaller than target size
    return width < targetSize && height < targetSize;
  }, [size, textureDimensions]);

  const handleToggleMode = useCallback(() => {
    if (!onResizeModeChange || !canToggleAi) {
      return;
    }
    const newMode: IconResizeMode = resizeMode === 'ai' ? 'normal' : 'ai';
    onResizeModeChange(newMode);
  }, [resizeMode, onResizeModeChange, canToggleAi]);

  if (!textureDimensions) {
    return null;
  }

  return (
    <div className="flex items-center gap-2">
      <span className="text-sm text-muted-foreground flex items-center gap-2">
        Resize:
        <TooltipHelp
          tooltips={
            <>
              Resize icons to different resolutions.
              AI upscaling uses{' '}
              <a
                href="https://upscayl.org/"
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-500 hover:underline"
              >
                Upscayl
              </a>{' '}
              to produce hopefully better results than "Normal" which uses standard image scaling.
              <br />
              <br />
              <strong>Recommendations:</strong>
              <br />
              • Classic HD 2.0: <strong>128px</strong>
              <br />
              • Reforged HD: <strong>256px</strong>
              <br />
              • Classic SD: <strong>64px</strong>
            </>
          }
        />
      </span>
      <Select
        value={currentValue}
        onValueChange={handleValueChange}
        disabled={disabled}
      >
        <SelectTrigger className="w-[150px]">
          <SelectValue>
            {currentLabel}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          {renderResizeOptions(textureDimensions)}
        </SelectContent>
      </Select>
      {canToggleAi && onResizeModeChange && (
        <button
          type="button"
          onClick={handleToggleMode}
          disabled={disabled}
          className="text-base font-medium text-blue-500 hover:text-blue-600 hover:underline disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:no-underline"
        >
          {resizeMode === 'ai' ? 'Use normal' : 'Use AI upscale'}
        </button>
      )}
    </div>
  );
}

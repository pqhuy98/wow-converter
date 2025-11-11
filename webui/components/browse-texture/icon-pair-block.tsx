'use client';

import { HelpCircle } from 'lucide-react';
import {
  useCallback, useEffect, useRef, useState,
} from 'react';

import { IconImage } from '@/components/common/icon-image';
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';
import type {
  IconFrame, IconResizeMode, IconSize, IconStyle,
} from '@/lib/models/icon-export.model';

import { getWc3PathForTooltip } from './utils';

export interface IconVariant {
  size: IconSize;
  style: IconStyle;
  frame: IconFrame;
  label: string;
  groupIndex: number;
}

interface IconPairBlockProps {
  groupIndex: number;
  groupVariants: IconVariant[];
  texturePath: string;
  isSelected: boolean;
  onPairClick: () => void;
  buildIconUrl: (texturePath: string, variant: IconVariant, size: IconSize, resizeMode?: IconResizeMode, useFallback?: boolean) => string;
  onImageLoad: (imageSrc?: string) => void;
  selectedSize: IconSize;
  selectedResizeMode: IconResizeMode | undefined;
  textureDimensions?: { width: number; height: number } | null;
}

const GROUP_LABELS: Readonly<Record<number, string>> = {
  0: 'Active',
  1: 'Passive',
  2: 'Autocast',
  3: 'Attack type',
  4: 'Upgrade',
  5: 'Hero score',
  6: 'Player score',
  7: 'Original',
} as const;

function splitWc3Path(path: string): { prefix: string; filename: string } {
  const lastSlash = path.lastIndexOf('/');
  if (lastSlash === -1) {
    return { prefix: '', filename: path };
  }
  return {
    prefix: path.substring(0, lastSlash + 1),
    filename: path.substring(lastSlash + 1),
  };
}

function IconImageWrapper({
  imageUrl,
  expectedSize,
  variantLabel,
  variant,
  texturePath,
  resizeMode,
  buildIconUrl,
  onImageLoad,
}: {
  imageUrl: string;
  expectedSize: number;
  variantLabel: string;
  variant: IconVariant;
  texturePath: string;
  resizeMode?: IconResizeMode;
  buildIconUrl: (texturePath: string, variant: IconVariant, size: IconSize, resizeMode?: IconResizeMode, useFallback?: boolean) => string;
  onImageLoad?: (imageSrc?: string) => void;
}) {
  const [isLoading, setIsLoading] = useState(true);
  const prevImageUrlRef = useRef<string | undefined>(undefined);

  // Build fallback URL: 64x64, same style/frame, no resizeMode - always generate when AI is active
  const fallbackUrl = resizeMode === 'ai' ? buildIconUrl(texturePath, variant, '64x64', undefined, false) : null;

  const handleLoad = useCallback(() => {
    setIsLoading(false);
    onImageLoad?.(imageUrl);
  }, [imageUrl, onImageLoad]);

  const handleError = useCallback(() => {
    setIsLoading(false);
    onImageLoad?.(imageUrl);
  }, [imageUrl, onImageLoad]);

  // Reset loading state only when imageUrl actually changes
  useEffect(() => {
    if (prevImageUrlRef.current !== imageUrl) {
      prevImageUrlRef.current = imageUrl;
      setIsLoading(true);
    }
  }, [imageUrl]);

  return (
    <IconImage
      src={imageUrl}
      alt={variantLabel}
      width={expectedSize}
      height={expectedSize}
      fallbackUrl={fallbackUrl ?? undefined}
      isLoading={isLoading}
      showLoadingSpinner={true}
      showCheckerboard={variant.frame !== 'none'}
      onLoad={handleLoad}
      onError={handleError}
      containerClassName="relative flex flex-col items-center justify-center rounded"
    />
  );
}

export default function IconPairBlock({
  groupIndex,
  groupVariants,
  texturePath,
  isSelected,
  onPairClick,
  buildIconUrl,
  onImageLoad,
  selectedSize,
  selectedResizeMode,
  textureDimensions,
}: IconPairBlockProps) {
  const groupLabel = GROUP_LABELS[groupIndex] ?? 'Unknown';

  return (
    <div
      className={`relative flex flex-col items-center justify-center rounded cursor-pointer transition-colors duration-200 ease-in-out self-start p-4 mr-4 last:mr-0 ${
        isSelected ? 'bg-primary/20' : 'bg-transparent'
      }`}
      onClick={onPairClick}
    >
      <div className="mb-2 flex items-center gap-1">
        <div className="text-sm font-semibold text-foreground">
          {groupLabel}
        </div>
        <TooltipProvider>
          <Tooltip delayDuration={0}>
            <TooltipTrigger asChild>
              <HelpCircle className="h-4 w-4 text-muted-foreground cursor-help" />
            </TooltipTrigger>
            <TooltipContent className="max-w-none">
              <div className="font-mono text-xs space-y-1">
                {groupVariants.map((variant) => {
                  const wc3Path = getWc3PathForTooltip(texturePath, variant.frame);
                  const { prefix, filename } = splitWc3Path(wc3Path);
                  return (
                    <p key={variant.label} className="break-all">
                      {prefix && <span className="text-muted-foreground">{prefix}</span>}
                      <span>{filename}</span>
                    </p>
                  );
                })}
              </div>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
      <div className="flex gap-x-1">
        {groupVariants.map((variant) => {
          // Use selectedSize and selectedResizeMode from props (for shop items) or fallback to defaults
          const size = selectedSize ?? '128x128';
          const resizeMode = selectedResizeMode;
          const imageUrl = buildIconUrl(texturePath, variant, size, resizeMode, false);
          // Determine expected size from URL
          const getExpectedSize = (url: string): number => {
            if (url.includes('size=256x256')) return 256;
            if (url.includes('size=128x128')) return 128;
            if (url.includes('size=64x64')) return 64;
            if (textureDimensions) {
              return textureDimensions.width;
            }
            return 64;
          };
          const expectedSize = getExpectedSize(imageUrl);
          return (
            <IconImageWrapper
              key={`${texturePath}-${variant.label}-${size}-${resizeMode ?? 'normal'}`}
              imageUrl={imageUrl}
              expectedSize={expectedSize}
              variantLabel={variant.label}
              variant={variant}
              texturePath={texturePath}
              resizeMode={resizeMode}
              buildIconUrl={buildIconUrl}
              onImageLoad={onImageLoad}
            />
          );
        })}
      </div>
    </div>
  );
}

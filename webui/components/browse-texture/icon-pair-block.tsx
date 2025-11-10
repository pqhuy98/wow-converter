'use client';

import {
  Info, Loader2,
} from 'lucide-react';
import {
  useCallback, useEffect, useRef, useState,
} from 'react';

import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';
import type {
  IconFrame, IconResizeMode, IconSize, IconStyle,
} from '@/lib/models/icon-export.model';
import { getWc3Path } from '@/lib/utils/wc3.utils';

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
  const lastBackslash = path.lastIndexOf('\\');
  if (lastBackslash === -1) {
    return { prefix: '', filename: path };
  }
  return {
    prefix: path.substring(0, lastBackslash + 1),
    filename: path.substring(lastBackslash + 1),
  };
}

function IconImage({
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
    <div
      className="relative flex flex-col items-center justify-center bg-black rounded"
      style={{ width: `${expectedSize}px`, height: `${expectedSize}px` }}
    >
      {fallbackUrl && (
        <img
          src={fallbackUrl}
          alt=""
          className="absolute inset-0 w-full h-full object-contain rounded blur-sm z-0 opacity-50"
          style={{ imageRendering: 'pixelated' }}
          loading="eager"
        />
      )}
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center z-20">
          <Loader2 className="h-12 w-12 animate-spin text-white" />
        </div>
      )}
      <img
        src={imageUrl}
        alt={variantLabel}
        className={`w-auto h-auto max-w-full max-h-full object-contain relative z-10 transition-opacity ${isLoading ? 'opacity-0' : 'opacity-100'}`}
        loading="lazy"
        onLoad={handleLoad}
        onError={handleError}
      />
    </div>
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
        isSelected ? 'bg-background hover:bg-background/90' : 'bg-transparent hover:bg-muted/30'
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
              <Info className="h-4 w-4 text-muted-foreground cursor-help" />
            </TooltipTrigger>
            <TooltipContent className="max-w-none">
              <div className="font-mono text-xs space-y-1">
                {groupVariants.map((variant) => {
                  const wc3Path = getWc3Path(texturePath, variant.frame);
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
            <IconImage
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

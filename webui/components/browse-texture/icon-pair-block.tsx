'use client';

import {
  Info, Loader2, X,
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
  isInShop: boolean;
  isSelected: boolean;
  onPairClick: () => void;
  buildIconUrl: (texturePath: string, variant: IconVariant, size: IconSize, resizeMode?: IconResizeMode, useFallback?: boolean) => string;
  onImageLoad: (imageSrc?: string) => void;
  selectedSize: IconSize;
  selectedResizeMode: IconResizeMode | undefined;
  selectedStyle: string | undefined;
  showPath?: boolean;
  showRemoveButton?: boolean;
  onRemove?: () => void;
  onSearchClick?: (texturePath: string, style?: string, size?: IconSize, resizeMode?: IconResizeMode) => void;
  textureDimensions?: { width: number; height: number } | null;
}

const SELECTION_ICON_SIZE = 64;

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
          <Loader2 className="h-6 w-6 animate-spin text-white" />
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
  isInShop,
  isSelected,
  onPairClick,
  buildIconUrl,
  showPath = false,
  showRemoveButton = false,
  onRemove,
  onImageLoad,
  selectedSize,
  selectedResizeMode,
  selectedStyle,
  onSearchClick,
  textureDimensions,
}: IconPairBlockProps) {
  const groupLabel = GROUP_LABELS[groupIndex] ?? 'Unknown';

  return (
    <div
      className={`relative flex flex-col items-center justify-center rounded cursor-pointer transition-colors self-start group ${
        isInShop
          ? `p-4 mr-4 last:mr-0 ${isSelected ? 'bg-background hover:bg-background/90' : ''}`
          : `w-[64px] h-[64px] ${showRemoveButton ? 'p-0' : ''} overflow-visible bg-transparent`
      }`}
      onClick={onPairClick}
    >
      {showRemoveButton && onRemove && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
          className="absolute -top-2 -right-2 p-1 rounded-full bg-destructive hover:bg-destructive/90 text-destructive-foreground shadow-md z-20 w-5 h-5 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity"
          aria-label="Remove from selection"
        >
          <X className="h-2.5 w-2.5" />
        </button>
      )}
      {isInShop && (
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
      )}
      {!isInShop && showPath && groupVariants.length >= 1 ? (() => {
        const filename = texturePath.split('/').pop() ?? texturePath;
        const mainVariant = groupVariants[0];

        const handleCartIconClick = (e: React.MouseEvent) => {
          e.stopPropagation();
          if (onSearchClick) {
            onSearchClick(texturePath, selectedStyle, selectedSize, selectedResizeMode);
          }
        };

        return (
          <Tooltip delayDuration={0} disableHoverableContent>
            <TooltipTrigger asChild>
              <div onClick={handleCartIconClick} className="relative bg-black rounded cursor-pointer">
                {(() => {
                  // For cart items, always use 64px size (no resize option)
                  // Build URL directly without resize to ensure it's always 64px
                  const selectionImageUrl = (() => {
                    const encodedPath = encodeURIComponent(texturePath);
                    const params = new URLSearchParams({
                      mode: 'icon',
                      style: mainVariant.style,
                      frame: mainVariant.frame,
                      size: '64x64',
                    });
                    if (mainVariant.frame !== 'none') {
                      params.set('extras', JSON.stringify({ crop: true }));
                    }
                    // Explicitly don't include resize parameter for cart items
                    return `/api/texture/png/${encodedPath}?${params.toString()}`;
                  })();
                  return (
                    <img
                      key={`${texturePath}-selection-${selectedSize ?? '64x64'}-${selectedResizeMode ?? 'normal'}`}
                      src={selectionImageUrl}
                      alt={mainVariant.label}
                      width={SELECTION_ICON_SIZE}
                      height={SELECTION_ICON_SIZE}
                      className="object-contain relative z-10"
                      loading="lazy"
                      onLoad={() => { onImageLoad?.(selectionImageUrl); }}
                      onError={() => { onImageLoad?.(selectionImageUrl); }}
                    />
                  );
                })()}
              </div>
            </TooltipTrigger>
            <TooltipContent
              className="min-w-[280px] max-w-none"
              onClick={(e) => e.stopPropagation()}
              side="top"
              sideOffset={8}
            >
                <div className="font-mono text-xs space-y-1 whitespace-pre-wrap break-words">
                  <p>Path: <span className="select-none"></span><span className="text-yellow-400">{filename}</span></p>
                  {(() => {
                    // Format resize info: "64px", "128px AI", "256px AI", etc.
                    const formatResize = (size?: IconSize, resizeMode?: IconResizeMode): string => {
                      if (!size) return '64px';
                      const sizeNum = size === '64x64' ? 64 : size === '128x128' ? 128 : 256;
                      const modeText = resizeMode === 'ai' ? ' AI' : '';
                      return `${sizeNum}px${modeText}`;
                    };
                    // Format style: "Classic HD 2.0", "Reforged HD", "Classic SD"
                    const formatStyle = (style?: string): string => {
                      if (!style) return 'Classic SD';
                      if (style === 'classic-hd-2.0') return 'Classic HD 2.0';
                      if (style === 'reforged-hd') return 'Reforged HD';
                      return 'Classic SD';
                    };
                    const resizeText = formatResize(selectedSize, selectedResizeMode);
                    const styleText = formatStyle(selectedStyle || mainVariant.style);
                    return (
                      <p>
                        {resizeText}, {styleText}
                      </p>
                    );
                  })()}
                  <p className="text-muted-foreground mt-1">Click to find in search</p>
                </div>
            </TooltipContent>
          </Tooltip>
        );
      })() : (
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
      )}
    </div>
  );
}

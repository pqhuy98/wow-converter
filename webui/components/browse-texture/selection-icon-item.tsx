'use client';

import { AlertTriangle } from 'lucide-react';
import { useCallback, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';
import type {
  IconFrame, IconResizeMode, IconSize, IconStyle,
} from '@/lib/models/icon-export.model';
import { FRAME_LABEL_MAP, STYLE_LABEL_MAP } from '@/lib/models/icon-export.model';

import type { IconVariant } from './icon-pair-block';

export interface SelectionIconItemProps {
  texturePath: string;
  variants: IconVariant[];
  size: IconSize;
  resizeMode?: IconResizeMode;
  style: IconStyle;
  outputName: string;
  hasDuplicateOutputName?: boolean;
  onImageLoad?: (imageSrc?: string) => void;
  onRemove: () => void;
  onRename?: (outputName: string) => void;
  onSearchClick?: (texturePath: string, style?: string, size?: IconSize, resizeMode?: IconResizeMode) => void;
}

const SELECTION_ICON_SIZE = 64;

function extractBaseName(texturePath: string): string {
  const filename = texturePath.split('/').pop() ?? texturePath;
  return filename.replace(/\.(blp|png|jpg|jpeg)$/i, '');
}

function formatIconName(texturePath: string, frame: IconFrame, outputName: string): string {
  const framePrefix = FRAME_LABEL_MAP[frame] ?? '';
  return framePrefix ? `${framePrefix}${outputName}` : outputName;
}

function formatSizeAndStyle(size: IconSize, resizeMode?: IconResizeMode, style?: IconStyle): string {
  const sizeNum = size === '64x64' ? 64 : size === '128x128' ? 128 : size === '256x256' ? 256 : 'original';
  const modeText = resizeMode === 'ai' ? ' AI' : '';
  const styleText = style ? STYLE_LABEL_MAP[style] : 'Classic SD';
  return `${sizeNum}px${modeText}, ${styleText}`;
}

export default function SelectionIconItem({
  texturePath,
  variants,
  size,
  resizeMode,
  style,
  outputName,
  hasDuplicateOutputName,
  onImageLoad,
  onRemove,
  onRename,
  onSearchClick,
}: SelectionIconItemProps) {
  const mainVariant = variants[0];
  const [isRenameDialogOpen, setIsRenameDialogOpen] = useState(false);
  const [renameValue, setRenameValue] = useState(outputName);

  const iconName = formatIconName(texturePath, mainVariant.frame, outputName);
  const sizeAndStyle = formatSizeAndStyle(size, resizeMode, style);

  const handleClick = useCallback(() => {
    if (onSearchClick) {
      // Pass the full texture path - scrollToAndSelectTexture will extract the base directory
      onSearchClick(texturePath, style, size, resizeMode);
    }
  }, [onSearchClick, texturePath, style, size, resizeMode]);

  const handleRename = useCallback(() => {
    const trimmedValue = renameValue.trim();
    // Ensure outputName always has a value (fallback to original base name with underscore)
    const finalValue = trimmedValue || `_${extractBaseName(texturePath)}`;
    onRename?.(finalValue);
    setIsRenameDialogOpen(false);
  }, [renameValue, texturePath, onRename]);

  const handleRenameDialogOpen = useCallback(() => {
    setRenameValue(outputName);
    setIsRenameDialogOpen(true);
  }, [outputName]);

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
    return `/api/texture/png/${encodedPath}?${params.toString()}`;
  })();

  return (
    <div
      className="flex items-start gap-3 p-2 rounded hover:bg-muted/50 group relative cursor-pointer transition-colors duration-300"
      onClick={handleClick}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          handleClick();
        }
      }}
    >
      <div className="flex-shrink-0">
        <img
          src={selectionImageUrl}
          alt={iconName}
          width={SELECTION_ICON_SIZE}
          height={SELECTION_ICON_SIZE}
          className="object-contain bg-black rounded"
          loading="lazy"
          onLoad={() => { onImageLoad?.(selectionImageUrl); }}
          onError={() => { onImageLoad?.(selectionImageUrl); }}
        />
      </div>
      <div className="flex flex-col justify-between whitespace-nowrap pr-8" style={{ height: `${SELECTION_ICON_SIZE}px` }}>
        {hasDuplicateOutputName ? (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <div className="flex items-center gap-1.5 text-base font-semibold text-yellow-600 dark:text-yellow-500 cursor-help">
                  <AlertTriangle className="h-4 w-4 shrink-0" />
                  <span>{iconName}</span>
                </div>
              </TooltipTrigger>
              <TooltipContent>
                <p>This icon has the same output name as another selected icon. One will overwrite the other during export.</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        ) : (
          <div className="flex items-center gap-1.5 text-base font-semibold text-foreground">
            <span>{iconName}</span>
          </div>
        )}
        <div className="text-sm text-foreground/60">
          {sizeAndStyle}
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              handleRenameDialogOpen();
            }}
            className="text-sm text-muted-foreground hover:text-foreground hover:underline text-left self-start"
            aria-label="Rename icon"
          >
            Rename
          </button>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onRemove();
            }}
            className="text-sm text-red-500 dark:text-red-400 hover:text-red-600 dark:hover:text-red-300 hover:underline text-left self-start"
            aria-label="Remove from selection"
          >
            Remove
          </button>
        </div>
      </div>
      <Dialog open={isRenameDialogOpen} onOpenChange={setIsRenameDialogOpen}>
        <DialogContent onClick={(e) => e.stopPropagation()}>
          <DialogHeader>
            <DialogTitle>Rename Icon</DialogTitle>
            <DialogDescription>
              Enter a custom name for this icon. The prefix (BTN, PAS, etc.) will be added automatically.
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <Input
              value={renameValue}
              onChange={(e) => setRenameValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  handleRename();
                }
              }}
              placeholder={extractBaseName(texturePath)}
              autoFocus
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsRenameDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleRename}>
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

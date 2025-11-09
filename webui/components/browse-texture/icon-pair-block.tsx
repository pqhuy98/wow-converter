'use client';

import { X } from 'lucide-react';

import { TooltipHelp } from '@/components/common/tooltip-help';
import {
  Tooltip, TooltipContent, TooltipTrigger,
} from '@/components/ui/tooltip';
import type { IconFrame, IconSize, IconStyle } from '@/lib/models/icon-export.model';

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
  isSelected?: boolean;
  onPairClick: () => void;
  buildIconUrl: (texturePath: string, variant: IconVariant) => string;
  showPath?: boolean;
  showRemoveButton?: boolean;
  onRemove?: () => void;
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

function getWc3Path(texturePath: string, frame: IconFrame): string {
  const filename = texturePath.split('/').pop() ?? texturePath;
  const baseName = filename.replace(/\.(blp|png|jpg|jpeg)$/i, '');

  switch (frame) {
    case 'btn':
      return `ReplaceableTextures\\CommandButtons\\BTN_${baseName}.blp`;
    case 'disbtn':
      return `ReplaceableTextures\\CommandButtonsDisabled\\DISBTN_${baseName}.blp`;
    case 'pas':
      return `ReplaceableTextures\\PassiveButtons\\PAS_${baseName}.blp`;
    case 'dispas':
      return `ReplaceableTextures\\CommandButtonsDisabled\\DISPAS_${baseName}.blp`;
    case 'atc':
      return `ReplaceableTextures\\CommandButtons\\ATC_${baseName}.blp`;
    case 'disatc':
      return `ReplaceableTextures\\CommandButtonsDisabled\\DISATC_${baseName}.blp`;
    case 'upg':
      return `ReplaceableTextures\\CommandButtons\\UPG_${baseName}.blp`;
    case 'att':
      return `ReplaceableTextures\\CommandButtons\\ATT_${baseName}.blp`;
    case 'ssh':
      return `scorescreen-hero-${baseName}.blp`;
    case 'ssp':
      return `scorescreen-player-${baseName}.blp`;
    case 'none':
      return filename;
    default:
      return filename;
  }
}

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

export default function IconPairBlock({
  groupIndex,
  groupVariants,
  texturePath,
  isInShop,
  isSelected = false,
  onPairClick,
  buildIconUrl,
  showPath = false,
  showRemoveButton = false,
  onRemove,
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
          className="absolute -top-2 -right-2 p-1 rounded-full bg-destructive hover:bg-destructive/90 text-destructive-foreground shadow-md z-10 w-5 h-5 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity"
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
          <TooltipHelp
            tooltips={
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
            }
          />
        </div>
      )}
      {!isInShop && showPath && groupVariants.length >= 1 ? (() => {
        const filename = texturePath.split('/').pop() ?? texturePath;
        const mainVariant = groupVariants.find((v) => !v.frame.startsWith('dis')) ?? groupVariants[0];
        const disabledVariant = groupVariants.find((v) => v.frame.startsWith('dis'));
        const mainPath = getWc3Path(texturePath, mainVariant.frame);
        const disabledPath = disabledVariant ? getWc3Path(texturePath, disabledVariant.frame) : null;

        return (
          <Tooltip>
            <TooltipTrigger asChild>
              <div onClick={(e) => e.stopPropagation()} className="bg-black rounded">
                <img
                  src={buildIconUrl(texturePath, { ...mainVariant, size: '64x64' as const })}
                  alt={mainVariant.label}
                  width={SELECTION_ICON_SIZE}
                  height={SELECTION_ICON_SIZE}
                  className="object-contain"
                  loading="lazy"
                />
              </div>
            </TooltipTrigger>
            <TooltipContent
              className="min-w-[280px] max-w-xl"
              onClick={(e) => e.stopPropagation()}
              side="top"
              sideOffset={8}
            >
                <div className="font-mono text-xs space-y-1 whitespace-pre-wrap break-words">
                  <p>Path: <span className="select-none"></span><span className="text-yellow-400">{filename}</span></p>
                  {(() => {
                    const { prefix, filename: mainFilename } = splitWc3Path(mainPath);
                    return (
                      <p>
                        {prefix && <span className="text-muted-foreground">{prefix}</span>}
                        <span>{mainFilename}</span>
                      </p>
                    );
                  })()}
                  {disabledPath && (() => {
                    const { prefix, filename: disabledFilename } = splitWc3Path(disabledPath);
                    return (
                      <p>
                        {prefix && <span className="text-muted-foreground">{prefix}</span>}
                        <span>{disabledFilename}</span>
                      </p>
                    );
                  })()}
                </div>
            </TooltipContent>
          </Tooltip>
        );
      })() : (
        <div className="flex gap-x-1">
          {groupVariants.map((variant) => {
            const imageUrl = buildIconUrl(texturePath, variant);
            return (
              <div key={variant.label} className="flex flex-col items-center bg-black rounded">
                <img
                  src={imageUrl}
                  alt={variant.label}
                  className="w-auto h-auto max-w-full max-h-full object-contain"
                  loading="lazy"
                />
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

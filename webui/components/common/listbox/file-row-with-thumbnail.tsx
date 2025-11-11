'use client';

import { CheckIcon, CopyIcon } from 'lucide-react';
import {
  memo, useEffect, useRef, useState,
} from 'react';

import { IconImage } from '@/components/common/icon-image';
import { TooltipHelp } from '@/components/common/tooltip-help';
import { useServerConfig } from '@/components/server-config';
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';

export const ICON_SIZE = 48; // Icon thumbnail size in pixels
const ICON_PADDING = 8;
export const ICON_ROW_HEIGHT = ICON_SIZE + ICON_PADDING; // Icon size + padding for text
const IMAGE_LOAD_DEBOUNCE_MS = 500; // Debounce delay before loading image

type FileEntry = { fileDataID: number; fileName: string };

interface FileRowWithThumbnailProps {
  file: FileEntry;
  index: number;
  isSelected: boolean;
  isBusy: boolean;
  highlightRegex: RegExp | null;
  lowerWordsSet: Set<string>;
  copyBtnRef?: React.RefObject<HTMLDivElement>;
  onClick?: (file: FileEntry) => void;
  onCopy?: () => void;
  style?: React.CSSProperties;
  thumbnailUrl?: string;
}

const FileRowWithThumbnailComponent = memo(({
  file,
  index,
  isSelected,
  isBusy,
  highlightRegex,
  lowerWordsSet,
  copyBtnRef,
  onClick,
  onCopy,
  style,
  thumbnailUrl,
}: FileRowWithThumbnailProps) => {
  const [hasCopied, setHasCopied] = useState(false);
  const [imageError, setImageError] = useState(false);
  const [imageDimensions, setImageDimensions] = useState<{ width: number; height: number } | null>(null);
  const { isSharedHosting } = useServerConfig();
  const [shouldLoadImage, setShouldLoadImage] = useState(!isSharedHosting);
  const loadTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const renderHighlightedText = (text: string) => {
    if (!highlightRegex) return text;
    const parts = text.split(highlightRegex);
    return parts.map((part, idx) => (
      lowerWordsSet.has(part.toLowerCase())
        ? <mark key={idx} className="bg-yellow-200 rounded px-0.5">{part}</mark>
        : <span key={idx}>{part}</span>
    ));
  };

  const imageUrl = thumbnailUrl ?? `/api/texture/png/${encodeURIComponent(file.fileName)}`;

  // Debounce image loading to avoid loading too many images during fast scrolling (only on shared hosting)
  useEffect(() => {
    if (!isSharedHosting) {
      if (!shouldLoadImage) setShouldLoadImage(true);
      return undefined;
    }

    // Clear any pending timeout
    if (loadTimeoutRef.current) {
      clearTimeout(loadTimeoutRef.current);
      loadTimeoutRef.current = null;
    }

    // Reset loading state when imageUrl changes
    setShouldLoadImage(false);
    setImageError(false);

    // Schedule image load after debounce delay
    loadTimeoutRef.current = setTimeout(() => {
      setShouldLoadImage(true);
    }, IMAGE_LOAD_DEBOUNCE_MS);

    return () => {
      if (loadTimeoutRef.current) {
        clearTimeout(loadTimeoutRef.current);
        loadTimeoutRef.current = null;
      }
    };
  }, [imageUrl, isSharedHosting]);

  // Load image dimensions when shouldLoadImage becomes true
  useEffect(() => {
    if (!shouldLoadImage || imageError) {
      setImageDimensions(null);
      return;
    }
    const img = new Image();
    img.onload = () => {
      setImageDimensions({ width: img.naturalWidth, height: img.naturalHeight });
    };
    img.onerror = () => {
      setImageDimensions(null);
    };
    img.src = imageUrl;
  }, [shouldLoadImage, imageUrl, imageError]);

  return (
    <div
      style={{ height: ICON_ROW_HEIGHT, ...style }}
      className={`px-2 flex items-center gap-2 text-sm min-w-full w-max ${isBusy ? 'cursor-not-allowed opacity-60' : ''} ${isSelected ? 'bg-primary/20' : 'hover:bg-accent cursor-pointer'}`}
      onClick={() => { if (!isBusy && !isSelected && onClick) onClick(file); }}
    >
      <span className="text-muted-foreground w-16 shrink-0">{index + 1}.</span>
      {/* Thumbnail */}
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger asChild>
            <div className="shrink-0">
              {!shouldLoadImage && isSharedHosting ? (
                <div className="text-xs text-muted-foreground text-center px-1" style={{ width: ICON_SIZE, height: ICON_SIZE }} />
              ) : !imageError ? (
                <IconImage
                  src={imageUrl}
                  alt=""
                  width={ICON_SIZE}
                  height={ICON_SIZE}
                  containerClassName="border border-border rounded cursor-pointer"
                  className="rounded"
                  loading="lazy"
                  onLoad={() => {
                    const img = new Image();
                    img.onload = () => {
                      setImageDimensions({ width: img.naturalWidth, height: img.naturalHeight });
                    };
                    img.onerror = () => {
                      setImageDimensions(null);
                    };
                    img.src = imageUrl;
                  }}
                  onError={() => setImageError(true)}
                />
              ) : (
                <div className="text-xs text-muted-foreground text-center px-1 border border-border rounded" style={{ width: ICON_SIZE, height: ICON_SIZE }}>Error</div>
              )}
            </div>
          </TooltipTrigger>
          {!imageError && imageDimensions && (
            <TooltipContent side="right" sideOffset={8}>
              {`${imageDimensions.width}x${imageDimensions.height}`}
            </TooltipContent>
          )}
        </Tooltip>
      </TooltipProvider>
      {/* Text content - 2 lines */}
      <div className="flex-1 flex flex-col justify-center min-w-0 py-1">
        <div className="font-mono text-foreground/70 whitespace-nowrap" title={file.fileName}>
          {renderHighlightedText(file.fileName)}
        </div>
        <div className="font-mono text-xs text-yellow-600 flex items-center gap-2">
          <span>[{renderHighlightedText(String(file.fileDataID))}]</span>
          <div
            ref={isSelected ? copyBtnRef : undefined}
            className={`text-muted-foreground shrink-0 rounded-md p-1 ${isSelected ? 'cursor-pointer' : 'invisible'}`}
            onMouseLeave={() => {
              if (isSelected) {
                setTimeout(() => setHasCopied(false), 1000);
              }
            }}
            onClick={(e) => {
              if (!isSelected) return;
              e.stopPropagation();
              void navigator.clipboard.writeText(file.fileName);
              setHasCopied(true);
              onCopy?.();
            }}
          >
            <TooltipHelp
              trigger={hasCopied
                ? <CheckIcon className="w-4 h-4" />
                : <CopyIcon className="w-4 h-4" />}
              tooltips={hasCopied ? 'Copied!' : 'Copy path'} />
          </div>
        </div>
      </div>
    </div>
  );
});

FileRowWithThumbnailComponent.displayName = 'FileRowWithThumbnail';
export const FileRowWithThumbnail: typeof FileRowWithThumbnailComponent & { ROW_HEIGHT: number } = Object.assign(FileRowWithThumbnailComponent, { ROW_HEIGHT: ICON_ROW_HEIGHT });

'use client';

import { CheckIcon, CopyIcon } from 'lucide-react';
import { memo, useState } from 'react';

import { TooltipHelp } from '@/components/common/tooltip-help';

type FileEntry = { fileDataID: number; fileName: string };

interface FileRowProps {
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
  disabledHover?: boolean;
  copyTooltip?: {
    copied: string;
    default: string;
  };
}

const ROW_HEIGHT = 28;

const FileRowComponent = memo(({
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
  disabledHover = false,
  copyTooltip,
}: FileRowProps) => {
  const [hasCopied, setHasCopied] = useState(false);

  const renderHighlightedText = (text: string) => {
    if (!highlightRegex) return text;
    const parts = text.split(highlightRegex);
    return parts.map((part, idx) => (
      lowerWordsSet.has(part.toLowerCase())
        ? <mark key={idx} className="bg-yellow-200 rounded px-0.5">{part}</mark>
        : <span key={idx}>{part}</span>
    ));
  };

  const hoverClass = disabledHover ? '' : 'hover:bg-accent cursor-pointer';
  const selectedClass = isSelected ? 'bg-primary/20' : '';
  const busyClass = isBusy ? 'cursor-not-allowed opacity-60' : '';

  return (
    <div
      style={{ height: ROW_HEIGHT, ...style }}
      className={`px-2 flex items-center text-sm min-w-full w-max whitespace-nowrap ${busyClass} ${selectedClass} ${hoverClass}`}
      onClick={() => { if (!isBusy && !isSelected && onClick) onClick(file); }}
    >
      <span className="text-muted-foreground w-16 shrink-0">{index + 1}.</span>
      <span className="font-mono text-foreground/70" title={file.fileName}>
        {renderHighlightedText(file.fileName)}{' '}
        [<span className="text-yellow-600">{renderHighlightedText(String(file.fileDataID))}</span>]
      </span>
      {isSelected && (
        <div
          ref={copyBtnRef}
          className="text-muted-foreground shrink-0 cursor-pointer ml-6 rounded-md p-1"
          onMouseLeave={() => {
            setTimeout(() => setHasCopied(false), 1000);
          }}
          onClick={(e) => {
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
            tooltips={hasCopied
              ? (copyTooltip?.copied ?? 'Copied!')
              : (copyTooltip?.default ?? 'Copy path')} />
        </div>
      )}
    </div>
  );
});

FileRowComponent.displayName = 'FileRow';
export const FileRow: typeof FileRowComponent & { ROW_HEIGHT: number } = Object.assign(FileRowComponent, { ROW_HEIGHT });

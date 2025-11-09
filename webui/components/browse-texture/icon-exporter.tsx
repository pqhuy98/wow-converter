'use client';

import {
  CheckCircle2,
  Copy,
  Download,
  Info,
  Trash2,
} from 'lucide-react';
import {
  useCallback, useEffect, useMemo, useState,
} from 'react';

import { Button } from '@/components/ui/button';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';
import {
  IconFrame, IconSize, IconStyle,
} from '@/lib/models/icon-export.model';

import { useServerConfig } from '../server-config';
import IconPairBlock, { type IconVariant } from './icon-pair-block';

const SELECTION_ITEM_GAP = 12;

interface SelectionItem {
  texturePath: string;
  style: IconStyle;
  groupIndex: number;
  variants: IconVariant[];
  id: string;
}

const frameGroups: IconFrame[][] = [
  ['btn', 'disbtn'],
  ['pas', 'dispas'],
  ['atc', 'disatc'],
  ['att'],
  ['upg'],
  ['ssh'],
  ['ssp'],
  ['none'],
];

function generateIconVariants(styles: IconStyle[]): IconVariant[] {
  const size: IconSize = 'original';
  const variants: IconVariant[] = [];
  for (const style of styles) {
    for (let groupIndex = 0; groupIndex < frameGroups.length; groupIndex++) {
      const group = frameGroups[groupIndex];
      for (const frame of group) {
        variants.push({
          size,
          style,
          frame,
          label: `${style} ${frame}`,
          groupIndex,
        });
      }
    }
  }
  return variants;
}

function buildIconUrl(texturePath: string, variant: IconVariant): string {
  const encodedPath = encodeURIComponent(texturePath);
  const params = new URLSearchParams({
    mode: 'icon',
    size: variant.size,
    style: variant.style,
    frame: variant.frame,
  });
  // Auto-enable crop for reforged HD style, but not for raw frame
  if (variant.frame !== 'none') {
    params.set('extras', JSON.stringify({ crop: true }));
  }
  return `/api/texture/png/${encodedPath}?${params.toString()}`;
}

interface IconExporterProps {
  texturePath: string;
}

const STORAGE_KEY_STYLE = 'icon-exporter-selected-style';
const STORAGE_KEY_SELECTION = 'icon-exporter-selection';
const DEFAULT_STYLE: IconStyle = 'classic-sd';

const tooltips = {
  exportAll: 'Export all icons in the selection as BLP files',
  clearSelection: 'Clear all items from the selection',
};

export default function IconExporter({ texturePath }: IconExporterProps) {
  const [selectedStyle, setSelectedStyle] = useState<IconStyle>(loadStyleFromStorage);
  const [selection, setSelection] = useState<SelectionItem[]>(loadSelectionFromStorage);
  const [cleaningAssets, setCleaningAssets] = useState<'ready' | 'pending' | 'cooldown'>('ready');
  const [exportingSelection, setExportingSelection] = useState(false);
  const [exportedOutputDirectory, setExportedOutputDirectory] = useState<string | null>(null);
  const [exportSuccessMessage, setExportSuccessMessage] = useState<string | null>(null);
  const [copiedPath, setCopiedPath] = useState(false);
  const serverConfig = useServerConfig();

  useEffect(() => {
    saveStyleToStorage(selectedStyle);
  }, [selectedStyle]);

  useEffect(() => {
    saveSelectionToStorage(selection);
  }, [selection]);

  const iconVariants = useMemo(() => generateIconVariants([selectedStyle]), [selectedStyle]);

  const handleStyleToggle = useCallback((value: string) => {
    if (value) {
      setSelectedStyle(value as IconStyle);
    }
  }, []);

  const handleAddToSelection = useCallback((groupIndex: number) => {
    const groupVariants = iconVariants.filter((v) => v.groupIndex === groupIndex);
    if (groupVariants.length === 0) return;

    const newItem: SelectionItem = {
      texturePath,
      style: selectedStyle,
      groupIndex,
      variants: groupVariants,
      id: `${texturePath}-${selectedStyle}-${groupIndex}-${Date.now()}`,
    };

    setSelection((prev) => [...prev, newItem]);
  }, [texturePath, selectedStyle, iconVariants]);

  const handleRemoveFromSelection = useCallback((id: string) => {
    setSelection((prev) => prev.filter((item) => item.id !== id));
  }, []);

  const handleClearSelection = useCallback(() => {
    setSelection([]);
  }, []);

  const handleCleanAssets = useCallback(() => {
    setCleaningAssets('pending');
    void fetch('/api/export/character/clean', { method: 'POST' }).then(() => {
      setCleaningAssets('cooldown');
      setTimeout(() => {
        setCleaningAssets('ready');
      }, 1000);
    });
  }, []);

  const handleExportSelection = useCallback(async () => {
    if (selection.length === 0) return;

    setExportingSelection(true);
    try {
      // Map selection to API format - flatten variants into separate items
      const items: Array<{ texturePath: string; options?: { size: string; style: string; frame: string; extras?: { crop?: boolean } } }> = [];

      for (const item of selection) {
        for (const variant of item.variants) {
          items.push({
            texturePath: item.texturePath,
            options: {
              size: variant.size,
              style: variant.style,
              frame: variant.frame,
              ...(variant.frame !== 'none' ? { extras: { crop: true } } : {}),
            },
          });
        }
      }

      const response = await fetch('/api/texture/blp', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ items }),
      });

      if (!response.ok) {
        const error = await response.json().catch(() => ({ error: 'Failed to export selection' }));
        throw new Error(error.error || 'Failed to export selection');
      }

      const result = await response.json();
      console.log(`Successfully exported ${result.count} icon(s)`);

      // Store output directory if available (non-shared hosting)
      if (result.outputDirectory) {
        setExportedOutputDirectory(result.outputDirectory);
      }

      // Download the exported files as a ZIP (only for shared hosting)
      if (serverConfig.isSharedHosting && result.paths && result.paths.length > 0) {
        try {
          const downloadResponse = await fetch('/api/download', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              files: result.paths,
              source: 'export',
            }),
          });

          if (!downloadResponse.ok) {
            throw new Error('Failed to download exported files');
          }

          // Get the blob and trigger download
          const blob = await downloadResponse.blob();
          const url = window.URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = 'icons.zip';
          document.body.appendChild(a);
          a.click();
          window.URL.revokeObjectURL(url);
          document.body.removeChild(a);

          setExportSuccessMessage(`Successfully exported ${result.count} icon(s) and downloaded as ZIP`);
        } catch (downloadError) {
          console.error('Error downloading files:', downloadError);
          setExportSuccessMessage('Files exported successfully, but download failed.');
        }
      } else {
        // Non-shared hosting: show success message
        setExportSuccessMessage(`Successfully exported ${result.count} icon(s)`);
      }
    } catch (error) {
      console.error('Error exporting selection:', error);
      setExportSuccessMessage(null);
      alert(error instanceof Error ? error.message : 'Failed to export selection');
    } finally {
      setExportingSelection(false);
    }
  }, [selection, serverConfig.isSharedHosting]);

  const renderShopGroup = (groupIndex: number) => {
    const groupVariants = iconVariants.filter((v) => v.groupIndex === groupIndex);
    if (groupVariants.length === 0) return null;

    const isInSelection = selection.some(
      (item) => item.texturePath === texturePath
        && item.style === selectedStyle
        && item.groupIndex === groupIndex,
    );

    const selectionItemId = selection.find(
      (item) => item.texturePath === texturePath
        && item.style === selectedStyle
        && item.groupIndex === groupIndex,
    )?.id;

    const handlePairClick = () => {
      if (isInSelection && selectionItemId) {
        handleRemoveFromSelection(selectionItemId);
      } else {
        handleAddToSelection(groupIndex);
      }
    };

    return (
      <IconPairBlock
        key={groupIndex}
        groupIndex={groupIndex}
        groupVariants={groupVariants}
        texturePath={texturePath}
        isInShop
        isSelected={isInSelection}
        onPairClick={handlePairClick}
        buildIconUrl={buildIconUrl}
      />
    );
  };

  return (
    <div className="h-full overflow-y-auto overflow-x-visible bg-secondary rounded-md">
      <div className="flex flex-col lg:flex-row gap-0 pl-4 pr-0 min-h-full">
        {/* Shop section */}
        <div className="flex-1 min-w-0 py-4">
          <div className="mb-4">
            <h2 className="text-2xl font-semibold mb-2">Icon Exporter</h2>
            <p className="text-sm text-muted-foreground">
              Texture: {' '}
              <span className="font-semibold text-foreground">{texturePath}</span>
            </p>
            <div className="flex flex-col sm:flex-row gap-3 mt-3">
              <div className="flex items-center gap-2">
                <span className="text-sm text-muted-foreground">Style:</span>
                <ToggleGroup
                  type="single"
                  value={selectedStyle}
                  onValueChange={handleStyleToggle}
                  className="flex-wrap"
                >
                  <ToggleGroupItem value="classic-hd-2.0" aria-label="Classic HD 2.0">
                    Classic HD 2.0
                  </ToggleGroupItem>
                  <ToggleGroupItem value="classic-sd" aria-label="Classic SD">
                    Classic SD
                  </ToggleGroupItem>
                  <ToggleGroupItem value="reforged-hd" aria-label="Reforged HD">
                    Reforged HD
                  </ToggleGroupItem>
                </ToggleGroup>
              </div>
            </div>
          </div>
          <div className="flex flex-wrap items-start gap-x-1 gap-y-3">
            {renderShopGroup(0)}
            {renderShopGroup(1)}
            {renderShopGroup(2)}
            {renderShopGroup(3)}
            {renderShopGroup(4)}
            {renderShopGroup(5)}
            {renderShopGroup(6)}
            {renderShopGroup(7)}
          </div>
        </div>
        {/* Selection section */}
        <div className="flex flex-col min-w-0 bg-muted rounded-r-lg p-4 w-[345px] flex-shrink-0 border-l border-t border-b border-r border-border self-stretch" style={{ overflowX: 'hidden' }}>
          <div className="flex items-center gap-2 mb-3 flex-shrink-0">
            <h3 className="text-2xl font-semibold">Selection</h3>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={selection.length === 0}
                    onClick={handleClearSelection}
                    className="text-destructive hover:text-destructive hover:bg-destructive/10"
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{tooltips.clearSelection}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          <div className="overflow-y-auto" style={{ maxHeight: 'calc(100% - 140px)' }}>
            {selection.length > 0 ? (
              <TooltipProvider>
                <div className="flex flex-wrap items-start pt-2 pr-4" style={{ rowGap: `${SELECTION_ITEM_GAP}px`, columnGap: `${SELECTION_ITEM_GAP}px`, overflowX: 'hidden' }}>
                  {selection.map((item) => (
                    <IconPairBlock
                      key={item.id}
                      groupIndex={item.groupIndex}
                      groupVariants={item.variants}
                      texturePath={item.texturePath}
                      isInShop={false}
                      onPairClick={() => {
                        handleRemoveFromSelection(item.id);
                      }}
                      buildIconUrl={buildIconUrl}
                      showPath
                      showRemoveButton
                      onRemove={() => {
                        handleRemoveFromSelection(item.id);
                      }}
                    />
                  ))}
                </div>
              </TooltipProvider>
            ) : (
              <div className="py-4 text-left">
                <p className="text-muted-foreground">
                  Click on icon pairs to add them to your selection
                </p>
              </div>
            )}
          </div>
          <div className="mt-3 pt-3 border-t flex items-center gap-2 flex-shrink-0">
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    size="sm"
                    disabled={selection.length === 0 || exportingSelection}
                    onClick={() => {
                      void handleExportSelection();
                    }}
                    className="flex-1"
                  >
                    {exportingSelection ? (
                      <>
                        <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-current mr-2" />
                        Exporting...
                      </>
                    ) : (
                      <>
                        <Download className="h-4 w-4 mr-2" />
                        Export selection
                      </>
                    )}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>{tooltips.exportAll}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
            {!serverConfig.isSharedHosting && (
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      size="sm"
                      variant="destructive"
                      disabled={cleaningAssets !== 'ready'}
                      onClick={handleCleanAssets}
                    >
                      {cleaningAssets === 'pending' ? (
                        <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-current" />
                      ) : (
                        <Trash2 className="h-4 w-4" />
                      )}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>Clear exported-assets directory</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            )}
          </div>
          {exportSuccessMessage && (
            <div className="mt-3 space-y-2">
              <div className="flex items-start gap-2 text-base">
                <CheckCircle2 className="h-5 w-5 shrink-0 mt-0.5 text-green-600 dark:text-green-400" />
                <div className="flex-1 space-y-1">
                  <span className="text-green-600 dark:text-green-400">{exportSuccessMessage}</span>
                  {exportedOutputDirectory && (
                    <div className="flex items-center gap-2">
                      <span className="text-green-600 dark:text-green-400">Exported to:</span>
                      <span className="font-mono text-muted-foreground">exported-assets</span>
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-5 w-5"
                              onClick={() => {
                                if (exportedOutputDirectory) {
                                  void navigator.clipboard.writeText(exportedOutputDirectory);
                                  setCopiedPath(true);
                                  setTimeout(() => setCopiedPath(false), 2000);
                                }
                              }}
                            >
                              {copiedPath ? (
                                <CheckCircle2 className="h-3 w-3 text-green-600 dark:text-green-400" />
                              ) : (
                                <Copy className="h-3 w-3" />
                              )}
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>
                            <p>Copy full path</p>
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </div>
                  )}
                </div>
              </div>
              {exportedOutputDirectory && (
                <div className="flex items-start gap-2 text-sm text-muted-foreground">
                  <Info className="h-4 w-4 shrink-0 mt-0.5" />
                  <div className="space-y-1">
                    <span>
                      Tip: for faster import to map, copy the contents into your WC3 map folder (save map as folder, not .w3x archive).
                    </span>
                    <span className="block">
                      Alternatively, use the old-school method: World Editor's asset manager to import files one by one, setting the correct paths exactly as they appear in the export directory. This manual method is not recommended due to being slow and tedious.
                    </span>
                  </div>
                </div>
              )}
              {exportSuccessMessage && serverConfig.isSharedHosting && exportSuccessMessage.includes('downloaded as ZIP') && (
                <div className="flex items-start gap-2 text-sm text-muted-foreground">
                  <Info className="h-4 w-4 shrink-0 mt-0.5" />
                  <div className="space-y-1">
                    <span>
                      Tip: extract the ZIP file and copy the contents into your WC3 map folder (save map as folder, not .w3x archive) for faster import to map.
                    </span>
                    <span className="block">
                      Alternatively, use the old-school method: World Editor's asset manager to import files one by one from the extracted ZIP, setting the correct paths exactly as they appear in the ZIP structure. This manual method is not recommended due to being slow and tedious.
                    </span>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function loadStyleFromStorage(): IconStyle {
  if (typeof window === 'undefined') return DEFAULT_STYLE;
  try {
    const stored = localStorage.getItem(STORAGE_KEY_STYLE);
    if (!stored) return DEFAULT_STYLE;
    const parsed = stored as IconStyle;
    if (['classic-sd', 'reforged-hd', 'classic-hd-2.0'].includes(parsed)) {
      return parsed;
    }
    return DEFAULT_STYLE;
  } catch {
    return DEFAULT_STYLE;
  }
}

function saveStyleToStorage(style: IconStyle): void {
  if (typeof window === 'undefined') return;
  try {
    localStorage.setItem(STORAGE_KEY_STYLE, style);
  } catch {
    // Ignore storage errors
  }
}

function loadSelectionFromStorage(): SelectionItem[] {
  if (typeof window === 'undefined') return [];
  try {
    const stored = localStorage.getItem(STORAGE_KEY_SELECTION);
    if (!stored) return [];
    const parsed = JSON.parse(stored) as SelectionItem[];
    if (Array.isArray(parsed)) {
      return parsed;
    }
    return [];
  } catch {
    return [];
  }
}

function saveSelectionToStorage(selection: SelectionItem[]): void {
  if (typeof window === 'undefined') return;
  try {
    localStorage.setItem(STORAGE_KEY_SELECTION, JSON.stringify(selection));
  } catch {
    // Ignore storage errors
  }
}

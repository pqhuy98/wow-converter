'use client';

import {
  CheckCircle2,
  Copy,
  Download,
  HelpCircle,
  Trash2,
} from 'lucide-react';
import {
  useCallback, useEffect, useMemo, useRef, useState,
} from 'react';

import { Button } from '@/components/ui/button';
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip';
import {
  IconFrame, IconResizeMode, IconSize, IconStyle,
} from '@/lib/models/icon-export.model';
import { getWc3Path } from '@/lib/utils/wc3.utils';

import { useServerConfig } from '../server-config';
import BorderStyleSelector from './border-style-selector';
import IconPairBlock, { type IconVariant } from './icon-pair-block';
import ResizeSelector from './resize-selector';
import SelectionIconItem from './selection-icon-item';
import { loadSettings, saveSettings } from './settings';

export interface SelectionItem {
  texturePath: string;
  style: IconStyle;
  groupIndex: number;
  variants: IconVariant[];
  size: IconSize;
  resizeMode?: IconResizeMode;
  id: string;
  outputName: string;
}

function extractBaseName(texturePath: string): string {
  const filename = texturePath.split('/').pop() ?? texturePath;
  return filename.replace(/\.(blp|png|jpg|jpeg)$/i, '');
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
  const size: IconSize = '128x128'; // Default size for variant display
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

function buildIconUrl(texturePath: string, variant: IconVariant, size: IconSize, resizeMode?: IconResizeMode, useFallbackResize?: boolean): string {
  const encodedPath = encodeURIComponent(texturePath);
  const params = new URLSearchParams({
    mode: 'icon',
    size, // Use size directly
    style: variant.style,
    frame: variant.frame,
  });
  // Auto-enable crop for reforged HD style, but not for raw frame
  if (variant.frame !== 'none') {
    params.set('extras', JSON.stringify({ crop: true }));
  }

  // If AI resize is loading, use normal resize as fallback (blurry but visible)
  let actualResizeMode = resizeMode;
  if (useFallbackResize && resizeMode === 'ai') {
    actualResizeMode = 'normal';
  }

  if (actualResizeMode) {
    params.set('resizeMode', actualResizeMode);
  }
  return `/api/texture/png/${encodedPath}?${params.toString()}`;
}

interface IconExporterProps {
  texturePath: string;
  onSearchClick?: (texturePath: string, style?: IconStyle, size?: IconSize, resizeMode?: IconResizeMode) => void;
}

const tooltips = {
  exportAll: 'Export all icons in the selection as BLP files',
  clearSelection: 'Clear all items from the selection',
};

function IconExporterContent({ texturePath, onSearchClick }: IconExporterProps) {
  const settings = loadSettings();
  const [selectedStyle, setSelectedStyle] = useState<IconStyle>(settings.style);
  const [selectedSize, setSelectedSize] = useState<IconSize>(settings.size);
  const [selectedResizeMode, setSelectedResizeMode] = useState<IconResizeMode | undefined>(settings.resizeMode);
  const [textureDimensions, setTextureDimensions] = useState<{ width: number; height: number } | null>(null);
  const [selection, setSelection] = useState<SelectionItem[]>(settings.selection);
  const [cleaningAssets, setCleaningAssets] = useState<'ready' | 'pending' | 'cooldown'>('ready');
  const [exportingSelection, setExportingSelection] = useState(false);
  const [exportedOutputDirectory, setExportedOutputDirectory] = useState<string | null>(null);
  const [exportSuccessMessage, setExportSuccessMessage] = useState<string | null>(null);
  const [copiedPath, setCopiedPath] = useState(false);
  const [removingIds, setRemovingIds] = useState<Set<string>>(new Set());
  const [newItemIds, setNewItemIds] = useState<Set<string>>(new Set());
  // Track loaded AI image URLs
  const [loadedAiImageUrls, setLoadedAiImageUrls] = useState<Set<string>>(new Set());
  const serverConfig = useServerConfig();
  // Track previous texture path and resize mode to detect changes
  const prevTexturePathRef = useRef<string>(texturePath);
  const prevResizeModeRef = useRef<IconResizeMode | undefined>(selectedResizeMode);

  useEffect(() => {
    saveSettings({ style: selectedStyle });
  }, [selectedStyle]);

  useEffect(() => {
    saveSettings({ size: selectedSize });
  }, [selectedSize]);

  useEffect(() => {
    saveSettings({ resizeMode: selectedResizeMode });
  }, [selectedResizeMode]);

  useEffect(() => {
    saveSettings({ selection });
  }, [selection]);

  // Reset loaded images when texture path changes
  useEffect(() => {
    setLoadedAiImageUrls(new Set());
  }, [texturePath]);

  // Reset resize mode to normal when texture path changes on shared hosting if it was previously AI
  useEffect(() => {
    const prevTexturePath = prevTexturePathRef.current;
    const prevResizeMode = prevResizeModeRef.current;

    // Check if texture path changed and previous resize mode was AI
    if (
      texturePath !== prevTexturePath
      && serverConfig.isSharedHosting
      && prevResizeMode === 'ai'
    ) {
      setSelectedResizeMode(undefined);
    }

    // Update refs for next comparison
    prevTexturePathRef.current = texturePath;
    prevResizeModeRef.current = selectedResizeMode;
  }, [texturePath, selectedResizeMode, serverConfig.isSharedHosting]);

  // Load texture image to get dimensions (image will be cached by browser)
  useEffect(() => {
    const img = new Image();
    const handleLoad = () => {
      setTextureDimensions({
        width: img.naturalWidth,
        height: img.naturalHeight,
      });
    };
    const handleError = () => {
      setTextureDimensions(null);
    };

    img.onload = handleLoad;
    img.onerror = handleError;
    // Use the PNG texture URL (will be cached by browser)
    img.src = `/api/texture/png/${encodeURIComponent(texturePath)}`;

    return () => {
      img.onload = null;
      img.onerror = null;
    };
  }, [texturePath]);

  const iconVariants = useMemo(() => generateIconVariants([selectedStyle]), [selectedStyle]);

  // Compute expected AI image URLs - only include URLs that are actually rendered (shop icons)
  const expectedAiImageUrls = useMemo(() => {
    const isAiResize = selectedResizeMode === 'ai';
    if (!isAiResize) {
      return new Set<string>();
    }

    const expected = new Set<string>();
    // Only add shop icon variant URLs (these are the ones actually rendered)
    // Selection URLs will load when rendered, but we don't block dropdown for them
    for (const variant of iconVariants) {
      const url = buildIconUrl(texturePath, variant, selectedSize, selectedResizeMode, false);
      expected.add(url);
    }
    return expected;
  }, [texturePath, selectedSize, selectedResizeMode, iconVariants]);

  // Check if dropdown should be disabled (when AI images are still loading)
  const isBusy = useMemo(() => {
    if (selectedResizeMode !== 'ai' || expectedAiImageUrls.size === 0) {
      return false;
    }

    // Parse URLs and compare by parameter sets (more reliable than string normalization)
    const parseUrlParams = (url: string): { pathname: string; params: Map<string, string> } => {
      try {
        let urlObj: URL;
        if (url.startsWith('http://') || url.startsWith('https://')) {
          urlObj = new URL(url);
        } else {
          urlObj = new URL(url, window.location.origin);
        }
        const params = new URLSearchParams(urlObj.search);
        const paramsMap = new Map<string, string>();
        for (const [key, value] of params.entries()) {
          paramsMap.set(key, value);
        }
        return { pathname: urlObj.pathname, params: paramsMap };
      } catch {
        return { pathname: url, params: new Map() };
      }
    };

    const urlMatches = (url1: string, url2: string): boolean => {
      const parsed1 = parseUrlParams(url1);
      const parsed2 = parseUrlParams(url2);
      if (parsed1.pathname !== parsed2.pathname) return false;
      if (parsed1.params.size !== parsed2.params.size) return false;
      for (const [key, value] of parsed1.params.entries()) {
        if (parsed2.params.get(key) !== value) return false;
      }
      return true;
    };

    // Check if all expected images are loaded by comparing parameter sets
    const allLoaded = Array.from(expectedAiImageUrls).every((expectedUrl) => Array.from(loadedAiImageUrls).some((loadedUrl) => urlMatches(expectedUrl, loadedUrl)));

    return !allLoaded;
  }, [selectedResizeMode, expectedAiImageUrls, loadedAiImageUrls]);

  const handleStyleToggle = useCallback((value: string) => {
    const newStyle = value as IconStyle;
    setSelectedStyle(newStyle);
    // Do not update selection - selection only changes when user explicitly clicks shop items
  }, []);

  const handleSizeChange = useCallback((newSize: IconSize) => {
    setSelectedSize(newSize);
    setLoadedAiImageUrls(new Set());
  }, []);

  const handleResizeModeChange = useCallback((mode: IconResizeMode) => {
    setSelectedResizeMode(mode);
    setLoadedAiImageUrls(new Set());
  }, []);

  const handleImageLoad = useCallback((imageSrc?: string) => {
    // Only track AI-resized images
    if (!imageSrc || selectedResizeMode !== 'ai') {
      return;
    }

    // Check if this is an AI-resized image (not fallback/normal resize)
    // Normalize URL for comparison (decode and re-encode to handle encoding differences)
    const normalizedSrc = decodeURIComponent(imageSrc);
    const resizeParam = 'resizeMode=ai';
    const isAiResizeUrl = normalizedSrc.includes(resizeParam);

    if (isAiResizeUrl) {
      // Use the normalized URL for consistent comparison
      const urlToAdd = imageSrc; // Keep original URL format
      setLoadedAiImageUrls((prev) => {
        // Check if already added
        if (prev.has(urlToAdd)) {
          return prev;
        }
        return new Set(prev).add(urlToAdd);
      });
    }
  }, [selectedResizeMode]);

  const handleRemoveFromSelection = useCallback((id: string) => {
    // Mark as removing to trigger exit animation
    setRemovingIds((prev) => new Set(prev).add(id));

    // Remove from selection after animation completes
    setTimeout(() => {
      setSelection((prev) => prev.filter((item) => item.id !== id));
      setRemovingIds((prev) => {
        const next = new Set(prev);
        next.delete(id);
        return next;
      });
    }, 300); // Match transition duration
  }, []);

  const handleRenameSelection = useCallback((id: string, outputName: string) => {
    setSelection((prev) => prev.map((item) => (item.id === id ? { ...item, outputName } : item)));
  }, []);

  const handleClearSelection = useCallback(() => {
    setSelection([]);
  }, []);

  const handleCleanAssets = useCallback(() => {
    setCleaningAssets('pending');
    setExportSuccessMessage(null);
    setExportedOutputDirectory(null);
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
    setExportSuccessMessage(null);
    setExportedOutputDirectory(null);
    try {
      // Map selection to API format - flatten variants into separate items
      // Only include first occurrence of each Wc3 output path

      const items: Array<{ texturePath: string; options?: { size: string; style: string; frame: string; extras?: { crop?: boolean }; resizeMode?: string }; outputPath?: string }> = [];
      const seenKeys = new Set<string>();

      for (const item of selection) {
        for (const variant of item.variants) {
          // Format texture path with output name
          // getWc3Path only uses the filename, so we use the output name with .blp extension
          const effectiveTexturePath = `${item.outputName}.blp`;

          // Create unique key: Wc3 output path
          const wc3Path = getWc3Path(effectiveTexturePath, variant.frame);
          if (!seenKeys.has(wc3Path)) {
            seenKeys.add(wc3Path);
            items.push({
              texturePath: item.texturePath,
              options: {
                size: item.size,
                style: variant.style,
                frame: variant.frame,
                ...(variant.frame !== 'none' ? { extras: { crop: true } } : {}),
                ...(item.resizeMode ? { resizeMode: item.resizeMode } : {}),
              },
              // Always use custom output path since outputName is required
              outputPath: wc3Path,
            });
          }
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
    // When original size is selected, only show the Original frame group (groupIndex 7)
    // But only if the texture's original size is NOT a standard icon size (64, 128, or 256)
    if (selectedSize === 'original' && textureDimensions) {
      const isStandardSize = (
        (textureDimensions.width === 64 && textureDimensions.height === 64)
        || (textureDimensions.width === 128 && textureDimensions.height === 128)
        || (textureDimensions.width === 256 && textureDimensions.height === 256)
      );

      // Only hide other groups if original size is NOT standard
      if (!isStandardSize && groupIndex !== 7) {
        return null;
      }
    }

    const groupVariants = iconVariants.filter((v) => v.groupIndex === groupIndex);
    if (groupVariants.length === 0) return null;

    // Check if there's an item with exact match on all fields: texturePath, style, groupIndex, size, and resizeMode
    const existingItem = selection.find(
      (item) => item.texturePath === texturePath
        && item.style === selectedStyle
        && item.groupIndex === groupIndex
        && item.size === selectedSize
        && item.resizeMode === selectedResizeMode,
    );

    // Consider item as not selected if it's being removed
    const isSelected = existingItem ? !removingIds.has(existingItem.id) : false;

    const handlePairClick = () => {
      if (existingItem) {
        // Exact match: remove it
        handleRemoveFromSelection(existingItem.id);
      } else {
        // Different resize or not in selection: replace/add
        // Use setSelection directly to ensure atomic update
        const groupVariantsForClick = iconVariants.filter((v) => v.groupIndex === groupIndex);
        if (groupVariantsForClick.length === 0) return;

        setSelection((prev) => {
          // Remove existing item with same texturePath and groupIndex (if exists)
          const filtered = prev.filter((item) => !(
            item.texturePath === texturePath
            && item.groupIndex === groupIndex
          ));

          // Remove conflicting items (same Wc3 path) and add new item
          const newSelection = filtered.filter((item) => !item.variants.some((itemVariant) => {
            const itemWc3Path = getWc3Path(item.texturePath, itemVariant.frame);
            return groupVariantsForClick.some((newVariant) => {
              const newWc3Path = getWc3Path(texturePath, newVariant.frame);
              return itemWc3Path === newWc3Path;
            });
          }));

          const newItem: SelectionItem = {
            texturePath,
            style: selectedStyle,
            groupIndex,
            variants: groupVariantsForClick,
            size: selectedSize,
            resizeMode: selectedResizeMode,
            id: `${texturePath}-${selectedStyle}-${groupIndex}-${Date.now()}`,
            // Set default output name with underscore prefix
            outputName: `_${extractBaseName(texturePath)}`,
          };

          // Mark as new item for entrance animation
          setNewItemIds((current) => new Set(current).add(newItem.id));
          setTimeout(() => {
            setNewItemIds((current) => {
              const next = new Set(current);
              next.delete(newItem.id);
              return next;
            });
          }, 300); // Match transition duration

          return [...newSelection, newItem];
        });
      }
    };

    return (
      <IconPairBlock
        key={`${texturePath}-${groupIndex}-${selectedStyle}-${selectedSize}-${selectedResizeMode ?? 'normal'}`}
        groupIndex={groupIndex}
        groupVariants={groupVariants}
        texturePath={texturePath}
        isSelected={isSelected}
        onPairClick={handlePairClick}
        buildIconUrl={(path, variant, size, resizeMode, useFallback) => buildIconUrl(path, variant, size ?? selectedSize, resizeMode ?? selectedResizeMode, useFallback)}
        onImageLoad={handleImageLoad}
        textureDimensions={textureDimensions}
        selectedSize={selectedSize}
        selectedResizeMode={selectedResizeMode}
      />
    );
  };

  return (
    <>
      <style>{`
        @keyframes slideIn {
          from {
            max-height: 0;
            opacity: 0;
          }
          to {
            max-height: 100px;
            opacity: 1;
          }
        }
      `}</style>
      <div className="h-full overflow-hidden bg-secondary rounded-md flex flex-col lg:flex-row">
        {/* Shop section */}
        <div className="flex-1 min-w-0 flex flex-col overflow-hidden">
          <div className="flex-shrink-0 pl-4 pr-0 pt-4">
            <div className="mb-4">
              <h2 className="text-2xl font-semibold mb-2">Icon Exporter</h2>
              <p className="text-sm text-muted-foreground">
                Texture: {' '}
                <span className="font-semibold text-foreground">{texturePath}</span>
              </p>
              <div className="flex flex-col sm:flex-row gap-3 mt-3">
                <BorderStyleSelector
                  value={selectedStyle}
                  onValueChange={handleStyleToggle}
                />
                <ResizeSelector
                  size={selectedSize}
                  resizeMode={selectedResizeMode}
                  textureDimensions={textureDimensions}
                  onSizeChange={handleSizeChange}
                  onResizeModeChange={handleResizeModeChange}
                  disabled={isBusy}
                />
              </div>
            </div>
          </div>
          <div className="flex-1 overflow-y-auto overflow-x-visible pl-4 pr-0 pb-4">
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
        </div>
        {/* Selection section */}
        <div className="flex flex-col min-w-0 bg-muted rounded-r-lg w-[345px] flex-shrink-0 border-l border-t border-b border-r border-border overflow-hidden">
          <div className="flex items-center gap-2 mb-3 flex-shrink-0 p-4 pb-0">
            <h3 className="text-2xl font-semibold">Selection</h3>
            <div className="ml-auto">
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
          </div>
          <div className="flex-1 overflow-y-auto overflow-x-auto relative">
            <div className="absolute top-0 left-0 right-0 h-4 bg-gradient-to-b from-muted to-transparent pointer-events-none z-10" />
            <div className="pt-2 pl-4 pb-4">
              {selection.length > 0 ? (
                <div className="flex flex-col gap-1">
                  {selection.map((item) => {
                    // Check if this item has duplicate output names with other items
                    // An item has duplicates if any of its variants would produce the same output path
                    // as any variant of any other item
                    const hasDuplicate = selection.some((otherItem) => {
                      if (otherItem.id === item.id) return false;
                      // Check if any variant of this item conflicts with any variant of otherItem
                      return item.variants.some((variant) => {
                        const thisOutputPath = getWc3Path(`${item.outputName}.blp`, variant.frame);
                        return otherItem.variants.some((otherVariant) => {
                          const otherOutputPath = getWc3Path(`${otherItem.outputName}.blp`, otherVariant.frame);
                          return thisOutputPath === otherOutputPath;
                        });
                      });
                    });

                    const isRemoving = removingIds.has(item.id);
                    const isNew = newItemIds.has(item.id);

                    return (
                      <div
                        key={item.id}
                        className={`transition-all duration-300 ease-in-out ${
                          isRemoving
                            ? 'opacity-0 max-h-0 overflow-hidden -mb-1'
                            : isNew
                              ? 'opacity-0 max-h-0 overflow-hidden animate-[slideIn_0.3s_ease-out_forwards]'
                              : 'opacity-100 max-h-[100px]'
                        }`}
                      >
                        <SelectionIconItem
                          texturePath={item.texturePath}
                          variants={item.variants}
                          size={item.size}
                          resizeMode={item.resizeMode}
                          style={item.style}
                          outputName={item.outputName}
                          hasDuplicateOutputName={hasDuplicate}
                          onImageLoad={handleImageLoad}
                          onRemove={() => handleRemoveFromSelection(item.id)}
                          onRename={(outputName) => handleRenameSelection(item.id, outputName)}
                          onSearchClick={(path, style, size, resizeMode) => {
                            if (style) setSelectedStyle(style as IconStyle);
                            if (size) setSelectedSize(size as IconSize);
                            if (resizeMode !== undefined) setSelectedResizeMode(resizeMode as IconResizeMode);
                            onSearchClick?.(path, style as IconStyle | undefined, size as IconSize | undefined, resizeMode as IconResizeMode | undefined);
                          }}
                        />
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="py-4 text-left px-4">
                  <p className="text-muted-foreground">
                    Click on icon pairs to add them to your selection
                  </p>
                </div>
              )}
            </div>
          </div>
          {/* Success message section - expands when content appears */}
          {exportSuccessMessage ? (
            <div className="border-t flex-shrink-0 p-4 transition-all duration-1000">
              <div className="space-y-3 animate-in fade-in slide-in-from-bottom-2">
                <div className="flex items-start gap-2">
                  <CheckCircle2 className="h-5 w-5 shrink-0 mt-0.5 text-green-600 dark:text-green-500" />
                  <div className="flex-1 space-y-2">
                    <div className="text-sm font-medium text-green-600 dark:text-green-500">
                      {exportSuccessMessage}
                    </div>
                    {exportedOutputDirectory && (
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-foreground/70">Exported to:</span>
                        <code className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded text-foreground/80">
                          exported-assets
                        </code>
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
                                  <CheckCircle2 className="h-3 w-3 text-green-600 dark:text-green-500" />
                                ) : (
                                  <Copy className="h-3 w-3 text-foreground/60" />
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
                  <div className="flex items-start gap-2 text-xs mt-2">
                    <HelpCircle className="h-5 w-5 shrink-0 mt-0.5 text-foreground/50" />
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <button
                            type="button"
                            className="flex-1 text-left text-foreground/70 hover:text-foreground/90 underline underline-offset-2 cursor-help"
                          >
                            How to import to WC3 map
                          </button>
                        </TooltipTrigger>
                        <TooltipContent className="max-w-sm">
                          <div className="space-y-2 text-xs">
                            <p>
                              Copy the contents of exported-assets folder into your WC3 map folder (save your map as folder, not .w3x archive).
                            </p>
                            <p className="text-muted-foreground">
                              Alternatively, use World Editor's asset manager to import files one by one, setting the correct paths exactly as they appear in the export directory. This old-school manual method is not recommended because it's slow.
                            </p>
                          </div>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </div>
                )}
                {exportSuccessMessage && serverConfig.isSharedHosting && exportSuccessMessage.includes('downloaded as ZIP') && (
                  <div className="flex items-start gap-2 text-xs mt-2">
                    <HelpCircle className="h-5 w-5 shrink-0 mt-0.5 text-foreground/50" />
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <button
                            type="button"
                            className="flex-1 text-left text-foreground/70 hover:text-foreground/90 underline underline-offset-2 cursor-help"
                          >
                            How to import to WC3 map
                          </button>
                        </TooltipTrigger>
                        <TooltipContent className="max-w-sm">
                          <div className="space-y-2 text-xs">
                            <p>
                              Extract the ZIP file and copy the contents into your WC3 map folder (save your map as folder, not .w3x archive).
                            </p>
                            <p className="text-muted-foreground">
                            Alternatively, use World Editor's asset manager to import files one by one, setting the correct paths exactly as they appear in the export directory. This old-school manual method is not recommended because it's slow.
                            </p>
                          </div>
                        </TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </div>
                )}
              </div>
            </div>
          ) : null}
          {/* Export button section */}
          <div className="border-t flex items-center gap-2 flex-shrink-0 p-4">
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
        </div>
      </div>
    </>
  );
}

export default function IconExporter({ texturePath, onSearchClick }: IconExporterProps) {
  return <IconExporterContent texturePath={texturePath} onSearchClick={onSearchClick} />;
}

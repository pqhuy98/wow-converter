'use client';

import { SearchIcon } from 'lucide-react';
import {
  useCallback, useDeferredValue, useEffect, useMemo, useRef, useState,
} from 'react';

import {
  FileRow, FileRowWithThumbnail, VirtualListBox,
} from '@/components/common/listbox';
import TextureViewer from '@/components/common/texture-viewer';
import {
  Card, CardContent, CardHeader, CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';

import IconExporter from './icon-exporter';

type FileEntry = { fileDataID: number; fileName: string };

const OVERSCAN = 8;

function isIcon(fileName: string): boolean {
  return fileName.toLowerCase().startsWith('interface/icons/');
}

const suggestions = ['interface/icons/', 'loadingscreens/'] as const;

export default function BrowseTexturePage() {
  const [allFiles, setAllFiles] = useState<FileEntry[]>([]);

  async function fetchAllFiles() {
    if (!allFiles.length) {
      const res = await fetch('/api/browse?q=texture');
      if (!res.ok) {
        throw new Error('Failed to fetch texture list files');
      }
      const files = await res.json();
      if (!files.length) {
        throw new Error('No texture files found');
      }
      setAllFiles(files);
    }
  }

  useEffect(() => {
    void fetchAllFiles();
  }, []);

  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [selected, setSelected] = useState<FileEntry | null>(null);
  const [selectedTexturePath, setSelectedTexturePath] = useState<string | undefined>(undefined);
  const [isImageLoading, setIsImageLoading] = useState(false);
  const listRef = useRef<HTMLDivElement | null>(null);
  const copyBtnRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const filtered = useMemo(() => {
    const q = debouncedQuery.trim();
    if (!q) return allFiles;
    const words = q.split(/ +/).filter(Boolean).map((w) => w.toLowerCase());
    return allFiles.filter((f) => {
      const nameLc = f.fileName.toLowerCase();
      const idStr = String(f.fileDataID);
      return words.every((w) => nameLc.includes(w) || idStr.includes(w));
    });
  }, [allFiles, debouncedQuery]);

  // Defer expensive calculations to avoid blocking the UI
  const deferredFiltered = useDeferredValue(filtered);

  // words used for highlighting (non-debounced for immediate feedback)
  const queryWords = useMemo(() => {
    const q = query.trim();
    if (!q) return [] as string[];
    return q.split(/ +/).filter(Boolean);
  }, [query]);

  const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

  const highlightRegex = useMemo(() => {
    if (queryWords.length === 0) return null as RegExp | null;
    const pattern = `(${queryWords.map(escapeRegExp).join('|')})`;
    return new RegExp(pattern, 'gi');
  }, [queryWords]);

  const lowerWordsSet = useMemo(() => new Set(queryWords.map((w) => w.toLowerCase())), [queryWords]);

  const CONTAINER_PADDING = 4; // Top and bottom padding in pixels

  // debounce query updates to reduce search frequency
  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 250);
    return () => clearTimeout(t);
  }, [query]);

  // whenever the debounced query changes (and filtered list will update), scroll back to top
  useEffect(() => {
    const el = listRef.current;
    if (!el) return;
    // Defer scroll reset to avoid blocking the main thread
    requestAnimationFrame(() => {
      el.scrollTop = 0;
    });
  }, [debouncedQuery]);

  // Ensure the copy icon on the selected row is visible if it is clipped
  useEffect(() => {
    const el = listRef.current;
    if (!el) return undefined;
    // Wait a frame so the copy icon (which appears only when selected) is mounted
    const id = requestAnimationFrame(() => {
      const copyEl = copyBtnRef.current;
      if (!copyEl) return;
      const containerRect = el.getBoundingClientRect();
      const copyRect = copyEl.getBoundingClientRect();
      const padding = 32;
      if (copyRect.right > containerRect.right - padding) {
        const delta = copyRect.right - (containerRect.right - padding);
        el.scrollTo({ left: el.scrollLeft + delta, behavior: 'smooth' });
        return;
      }
      if (copyRect.left < containerRect.left + padding) {
        const delta = (containerRect.left + padding) - copyRect.left;
        el.scrollTo({ left: Math.max(0, el.scrollLeft - delta), behavior: 'smooth' });
      }
    });
    return () => cancelAnimationFrame(id);
  }, [selected]);

  const handleSelect = useCallback((file: FileEntry) => {
    setSelected(file);
    setSelectedTexturePath(file.fileName);
    // For icons, we show variant grid (lazy-loaded), so no need to wait for single image
    // For regular textures, TextureViewer will call onLoad/onError
    if (isIcon(file.fileName)) {
      setIsImageLoading(false);
    } else {
      setIsImageLoading(true);
    }
  }, []);

  const selectedIsIcon = selected ? isIcon(selected.fileName) : false;

  const handleImageLoad = useCallback(() => {
    setIsImageLoading(false);
  }, []);

  const handleImageError = useCallback(() => {
    setIsImageLoading(false);
  }, []);

  const isBusy = isImageLoading;

  const applySuggestion = (s: typeof suggestions[number]) => {
    const v = `${s} `;
    setQuery(v);
    // Update debouncedQuery immediately for filter suggestions (user intent is clear)
    setDebouncedQuery(v);
    const el = inputRef.current;
    if (el) {
      el.focus();
      // onFocus selects all; place caret at end on next tick
      setTimeout(() => el.setSelectionRange(v.length, v.length), 0);
    }
  };

  if (!allFiles.length) {
    return <div>Loading...</div>;
  }

  return (
    <div className="h-full p-4 flex flex-col overflow-x-hidden">
      <div className="mx-auto flex-1 flex flex-col w-full max-w-full">
        <div className="mb-4" />
        <div className="flex flex-col lg:flex-row gap-6 h-full min-w-0" style={{ height: 'calc(100vh - 125px)' }}>
          {/* Left: list */}
          <div className="lg:w-1/3 w-full lg:h-full h-[40vh] overflow-hidden min-w-0">
            <Card className="h-full flex flex-col min-w-0">
              <CardHeader className="flex flex-row justify-between items-center py-2 px-3 pb-0 pt-3">
                <CardTitle className="text-lg flex items-center gap-2">
                  <SearchIcon className="w-4 h-4" />
                  Browse Texture Files
                </CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col flex-1 overflow-hidden p-3 min-w-0">
                <div className="flex items-center relative w-full mb-2">
                  <Input
                    placeholder="Search texture, e.g. 'interface/icons'..."
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    disabled={isBusy}
                    onKeyDown={(e) => {
                      if (e.key === 'Escape') {
                        e.preventDefault();
                        setQuery('');
                        setDebouncedQuery('');
                      }
                    }}
                    ref={inputRef}
                    className="w-full sm:pr-[170px]"
                  />
                  <div className="absolute inset-y-0 right-2 hidden sm:flex items-center gap-2 pointer-events-none z-20">
                    {suggestions.map((s) => (
                      <button
                        key={s}
                        type="button"
                        className="text-[10px] sm:text-xs px-1.5 py-0.5 sm:px-2 sm:py-1 rounded bg-secondary hover:bg-accent border border-border pointer-events-auto"
                        onClick={() => applySuggestion(s)}
                        disabled={isBusy}
                        title={`Search for ${s}`}
                      >
                        {s}
                      </button>
                    ))}
                  </div>
                </div>
                <VirtualListBox<FileEntry>
                  items={deferredFiltered}
                  containerRef={listRef}
                  containerClassName="overflow-y-scroll overflow-x-auto border rounded-md bg-background flex-1"
                  contentPadding={CONTAINER_PADDING}
                  overscan={OVERSCAN}
                  getRowKey={(f) => f.fileDataID + f.fileName}
                  getRowHeight={(f) => (isIcon(f.fileName) ? FileRowWithThumbnail.ROW_HEIGHT : FileRow.ROW_HEIGHT)}
                  renderRow={(file, index, style) => {
                    const isSelected = selected === file;
                    const isIconFile = isIcon(file.fileName);
                    return isIconFile ? (
                      <FileRowWithThumbnail
                        key={file.fileDataID + file.fileName}
                        file={file}
                        index={index}
                        isSelected={isSelected}
                        isBusy={isBusy}
                        highlightRegex={highlightRegex}
                        lowerWordsSet={lowerWordsSet}
                        copyBtnRef={isSelected ? copyBtnRef : undefined}
                        onClick={handleSelect}
                        style={style}
                      />
                    ) : (
                      <FileRow
                        key={file.fileDataID + file.fileName}
                        file={file}
                        index={index}
                        isSelected={isSelected}
                        isBusy={isBusy}
                        highlightRegex={highlightRegex}
                        lowerWordsSet={lowerWordsSet}
                        copyBtnRef={isSelected ? copyBtnRef : undefined}
                        onClick={handleSelect}
                        style={style}
                      />
                    );
                  }}
                />
              </CardContent>
            </Card>
          </div>

          {/* Right: viewer */}
          <div className="lg:w-2/3 w-full h-full overflow-y-auto overflow-x-visible p-0 relative min-w-0">
            {selectedTexturePath && selectedIsIcon ? (
              <IconExporter texturePath={selectedTexturePath} />
            ) : selectedTexturePath ? (
              <TextureViewer
                texturePath={selectedTexturePath}
                onLoad={handleImageLoad}
                onError={handleImageError}
              />
            ) : (
              <div className="absolute inset-0 bg-secondary flex items-center justify-center text-center text-muted-foreground w-full px-4">
                <p className="text-lg mb-2">Select a texture to view</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

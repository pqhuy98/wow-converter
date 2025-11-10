'use client';

import { SearchIcon } from 'lucide-react';
import React, {
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import { SettingsDialogButton } from '@/components/browse-model/settings-dialog';
import { FileRow, VirtualListBox } from '@/components/common/listbox';
import ModelViewerUi from '@/components/common/model-viewer';
import { Terminal } from '@/components/common/terminal';
import {
  Card, CardContent, CardHeader, CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { usePendingScrollToItem } from '@/lib/hooks/use-pending-scroll-to-item';
import { useScrollResetOnSearchChange } from '@/lib/hooks/use-scroll-reset-on-search-change';
import { useSearchSelectUrlSync } from '@/lib/hooks/use-search-select-url-sync';
import {
  Character,
  JobStatus,
  ModelFormat,
  ModelFormatVersion,
  Optimization,
} from '@/lib/models/export-character.model';

type FileEntry = { fileDataID: number; fileName: string };

const defaultCharacter: Character = {
  base: { type: 'local', value: '' },
  inGameMovespeed: 270,
  keepCinematic: true,
  noDecay: true,
};

export default function BrowseModelPage() {
  const [allFiles, setAllFiles] = useState<FileEntry[]>([]);

  async function fetchAllFiles() {
    if (!allFiles.length) {
      const res = await fetch('/api/browse?q=model');
      if (!res.ok) {
        throw new Error('Failed to fetch m2 list files');
      }
      const files = await res.json();
      if (!files.length) {
        throw new Error('No m2 list files found');
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
  const [job, setJob] = useState<JobStatus | undefined>(undefined);
  const [modelPath, setModelPath] = useState<string | undefined>(undefined);
  const [pendingScrollToPath, setPendingScrollToPath] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement | null>(null);
  const copyBtnRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const OVERSCAN = 8;

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

  // debounce query updates to reduce search frequency
  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 250);
    return () => clearTimeout(t);
  }, [query]);

  // URL sync and resets (shared with texture page, but generic)
  useSearchSelectUrlSync({
    basePath: '/browse',
    search: query,
    setSearch: setQuery,
    setDebouncedSearch: setDebouncedQuery,
    selectedPath: selected?.fileName,
    pendingScrollPath: pendingScrollToPath,
    setPendingScrollPath: setPendingScrollToPath,
    resetLocalState: () => {
      setQuery('');
      setDebouncedQuery('');
      setSelected(null);
      setJob(undefined);
      setModelPath(undefined);
      setPendingScrollToPath(null);
    },
  });

  // Reset scroll only when debounced search changes and no pending scroll
  useScrollResetOnSearchChange({
    containerRef: listRef,
    search: debouncedQuery,
    isPending: !!pendingScrollToPath,
  });

  // Generic pending scroll + export using shared hook
  usePendingScrollToItem<FileEntry>({
    items: deferredFiltered,
    containerRef: listRef,
    getRowHeight: () => FileRow.ROW_HEIGHT,
    contentPadding: 0,
    matchKey: (f) => f.fileName,
    pendingKey: pendingScrollToPath,
    setPendingKey: setPendingScrollToPath,
    onSelect: (file) => { void triggerExport(file); },
  });

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

  // poll job status
  useEffect(() => {
    if (!job?.id) return undefined;
    if (job.status === 'done') {
      setModelPath(job.result?.exportedModels[0]?.path);
      return undefined;
    }
    const fetchJob = async () => {
      const r = await fetch(`/api/export/character/status/${job.id}`);
      if (!r.ok) return;
      const js = (await r.json()) as JobStatus;
      setJob(js);
      if (js.status === 'done' && js.result) {
        setModelPath(js.result.exportedModels[0]?.path);
        clearInterval(interval);
      }
      if (js.status === 'failed') {
        clearInterval(interval);
      }
    };
    void fetchJob();
    const interval = setInterval(() => void fetchJob(), 500);
    return () => clearInterval(interval);
  }, [job?.id]);

  const [isExporting, setIsExporting] = useState(false);
  // Shared export settings state
  const [character, setCharacter] = useState<Character>(defaultCharacter);
  const [outputFileName, setOutputFileName] = useState<string>('');
  const [format, setFormat] = useState<ModelFormat>('mdx');
  const [formatVersion, setFormatVersion] = useState<ModelFormatVersion>('1000');
  const [optimization, setOptimization] = useState<Optimization>({
    sortSequences: true,
    removeUnusedVertices: true,
    removeUnusedNodes: true,
    removeUnusedMaterialsTextures: true,
  });
  const isBusy = isExporting || job?.status === 'pending' || job?.status === 'processing';

  const triggerExport = async (file: FileEntry) => {
    if (isExporting) {
      console.log('Please wait for the current export to finish.');
      return;
    }
    setSelected(file);
    // Output file name: m2 path with / and \ replaced by _
    const guessedName = file.fileName.replace(/[\\/]/g, '_').replace(/\.m2$/i, '');
    setOutputFileName(guessedName);
    const localBase = { type: 'local', value: file.fileName.replace(/\.m2$/i, '.obj') } as const;

    const exportCharacter: Character = {
      ...character,
      base: localBase,
      attackTag: character.attackTag === undefined ? '' : character.attackTag,
    };
    const request = {
      character: exportCharacter,
      outputFileName: guessedName,
      optimization,
      format,
      formatVersion,
      isBrowse: true,
    };

    setIsExporting(true);
    const res = await fetch('/api/export/character', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    });
    setIsExporting(false);
    const js = (await res.json()) as JobStatus;
    setJob(js);
  };

  const renderRow = useCallback((file: FileEntry, index: number, style: React.CSSProperties) => {
    const isSelected = selected === file;
    return (
      <FileRow
        file={file}
        index={index}
        isSelected={isSelected}
        isBusy={isBusy}
        highlightRegex={highlightRegex}
        lowerWordsSet={lowerWordsSet}
        copyBtnRef={isSelected ? copyBtnRef : undefined}
        onClick={(f) => { void triggerExport(f); }}
        style={style}
        disabledHover={isExporting}
        copyTooltip={{
          copied: 'Copied, you can now paste it in local file input field in Character Export',
          default: 'Copy path for local file export',
        }}
      />
    );
  }, [selected, isBusy, isExporting, highlightRegex, lowerWordsSet, triggerExport]);

  const suggestions = ['creature/', 'spells/', 'doodads/', 'wmo/'] as const;
  const applySuggestion = (s: typeof suggestions[number]) => {
    const v = `${s} `;
    setQuery(v);
    // update results immediately for a snappier UX
    setDebouncedQuery(v);
    const el = inputRef.current;
    if (el) {
      el.focus();
      // onFocus selects all; place caret at end on next tick
      setTimeout(() => el.setSelectionRange(v.length, v.length), 0);
    }
  };

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
                  Browse Model Files
                </CardTitle>
                <SettingsDialogButton
                  className="ml-auto !mt-0"
                  character={character}
                  setCharacter={setCharacter}
                  outputFileName={outputFileName}
                  setOutputFileName={setOutputFileName}
                  format={format}
                  setFormat={setFormat}
                  formatVersion={formatVersion}
                  setFormatVersion={setFormatVersion}
                  optimization={optimization}
                  setOptimization={setOptimization}
                  disabled={isBusy}
                />
              </CardHeader>
              <CardContent className="flex flex-col flex-1 overflow-hidden p-3 min-w-0">
                <div className="flex items-center w-full mb-2">
                  <div className="relative w-full">
                    <Input
                      placeholder="Search model, e.g. 'spell fire'..."
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
                </div>
                <VirtualListBox
                  items={deferredFiltered}
                  getRowKey={(file) => file.fileDataID + file.fileName}
                  renderRow={renderRow}
                  fixedRowHeight={FileRow.ROW_HEIGHT}
                  overscan={OVERSCAN}
                  containerRef={listRef}
                  containerClassName="overflow-y-scroll overflow-x-auto border rounded-md bg-background flex-1"
                />
              </CardContent>
            </Card>
          </div>

          {/* Right: viewer */}
          <div className="lg:w-2/3 w-full h-full overflow-hidden min-w-0">
            <div className="p-0 h-full relative overflow-hidden min-w-0">
              {modelPath && (
                <ModelViewerUi modelPath={modelPath} source="browse" />
              )}
              {job?.status !== 'done' && (
                <div className="absolute inset-0 bg-secondary flex items-center justify-center z-10">
                  <div className="text-center text-muted-foreground w-full px-4">
                    {job?.status === 'processing' || job?.status === 'pending' ? (
                      <>
                      <div className="flex items-center justify-center gap-2">
                        <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-primary" />
                        <p className="text-lg">{job.status === 'processing' ? 'Exporting...' : `Queue position: ${job.position}`}</p>
                      </div>
                        <div className="mt-4 mx-auto w-full sm:w-3/4 lg:w-1/2 sm:min-w-[75%] sm:max-w-[75%] lg:min-w-[75%] lg:max-w-[75%]">
                          <Terminal logs={job.logs || []} className="w-full" />
                        </div>
                      </>
                    ) : job?.status === 'failed' ? (
                      <>
                        <p className="text-lg mb-2 text-destructive">Export failed</p>
                        {job?.error && (
                          <pre className="text-left text-sm text-destructive whitespace-pre-wrap bg-card border border-border rounded p-2 max-w-[90%] mx-auto overflow-x-auto">
                            {job.error}
                          </pre>
                        )}
                      </>
                    ) : (
                      <>
                        <p className="text-lg mb-2">Select a file to export and preview</p>
                      </>
                    )}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

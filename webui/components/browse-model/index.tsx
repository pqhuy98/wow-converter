'use client';

import { CheckIcon, CopyIcon, SearchIcon } from 'lucide-react';
import {
  useEffect, useMemo, useRef, useState,
} from 'react';

import { SettingsDialogButton } from '@/components/browse-model/settings-dialog';
import ModelViewerUi from '@/components/common/model-viewer';
import { Terminal } from '@/components/common/terminal';
import {
  Card, CardContent, CardHeader, CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import {
  Character,
  JobStatus,
  ModelFormat,
  ModelFormatVersion,
  Optimization,
} from '@/lib/models/export-character.model';

import { TooltipHelp } from '../common/tooltip-help';

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
  const listRef = useRef<HTMLDivElement | null>(null);
  const copyBtnRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const [viewportHeight, setViewportHeight] = useState(400);
  const [scrollTop, setScrollTop] = useState(0);

  const ROW_HEIGHT = 28;
  const OVERSCAN = 8;

  const idToFile = useMemo(() => {
    const m = new Map<number, FileEntry>();
    for (const f of allFiles) m.set(f.fileDataID, f);
    return m;
  }, [allFiles]);

  const filtered = useMemo(() => {
    const q = debouncedQuery.trim();
    if (!q) return allFiles;
    const words = q.split(/ +/).filter(Boolean).map((w) => w.toLowerCase());
    return allFiles.filter((f) => words.every((w) => f.fileName.toLowerCase().includes(w)));
  }, [allFiles, debouncedQuery, idToFile]);

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

  const renderHighlightedName = (name: string) => {
    if (!highlightRegex) return name;
    const parts = name.split(highlightRegex);
    return parts.map((part, idx) => (
      lowerWordsSet.has(part.toLowerCase())
        ? <mark key={idx} className="bg-yellow-200 rounded px-0.5">{part}</mark>
        : <span key={idx}>{part}</span>
    ));
  };

  // debounce query updates to reduce search frequency
  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 250);
    return () => clearTimeout(t);
  }, [query]);

  // whenever the debounced query changes (and filtered list will update), scroll back to top
  useEffect(() => {
    const el = listRef.current;
    if (!el) return;
    el.scrollTop = 0;
    setScrollTop(0);
  }, [debouncedQuery]);

  // track viewport height for virtualization
  useEffect(() => {
    const el = listRef.current;
    if (!el) return undefined;
    const update = () => setViewportHeight(el.clientHeight || 400);
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, [listRef.current]);

  const total = filtered.length;
  const startIndex = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN);
  const endIndex = Math.min(total - 1, Math.ceil((scrollTop + viewportHeight) / ROW_HEIGHT) + OVERSCAN);
  const visibleItems = filtered.slice(startIndex, endIndex + 1);

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

  const [hasCopied, setHasCopied] = useState(false);

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

  if (!allFiles) {
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
              <CardContent className="space-y-2 flex-1 overflow-hidden p-3 min-w-0">
                <div className="flex items-center w-full">
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
                <div
                  ref={listRef}
                  className="mt-2 overflow-y-scroll overflow-x-auto border rounded-md bg-background h-[calc(100vh-260px)]"
                  onScroll={(e) => setScrollTop((e.target as HTMLDivElement).scrollTop)}
                >
                  <div style={{
                    height: total * ROW_HEIGHT,
                    position: 'relative',
                  }}>
                    <div style={{
                      position: 'absolute',
                      top: startIndex * ROW_HEIGHT,
                      left: 0,
                      minWidth: '100%',
                      width: 'max-content',
                    }}>
                      {visibleItems.map((f, i) => {
                        const isSelected = selected === f;
                        return <div
                          key={f.fileDataID + f.fileName}
                          style={{ height: ROW_HEIGHT }}
                          className={`px-2 flex items-center text-sm min-w-full w-max whitespace-nowrap ${isBusy ? 'cursor-not-allowed opacity-60' : ''} ${isSelected ? 'bg-primary/20' : isExporting ? '' : 'hover:bg-accent cursor-pointer'}`}
                          onClick={() => { if (!isBusy && !isSelected) void triggerExport(f); }}
                        >
                          <span className="text-muted-foreground w-16 shrink-0">{startIndex + i + 1}.</span>
                          <span className="font-mono text-foreground/70" title={f.fileName}>
                            {renderHighlightedName(f.fileName)}{' '}
                            [<span className="text-yellow-600">{f.fileDataID}</span>]
                          </span>
                          {isSelected && (
                            <div ref={copyBtnRef} className="text-muted-foreground shrink-0 cursor-pointer ml-6 rounded-md p-1"
                              onMouseLeave={() => {
                                setTimeout(() => setHasCopied(false), 1000);
                              }}
                              onClick={() => {
                                void navigator.clipboard.writeText(f.fileName);
                                setHasCopied(true);
                              }}>
                                <TooltipHelp
                                trigger={hasCopied
                                  ? <CheckIcon className="w-4 h-4" />
                                  : <CopyIcon className="w-4 h-4" />}
                                tooltips={hasCopied ? 'Copied, you can now paste it in local file input field in Character Export' : 'Copy path for local file export'} />
                            </div>
                          )}
                        </div>;
                      })}
                    </div>
                  </div>
                </div>
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

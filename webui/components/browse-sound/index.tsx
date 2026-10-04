'use client';

import {
  CaptionsIcon, DownloadIcon, Loader2, Music2Icon,
} from 'lucide-react';
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import { FileRow, VirtualListBox } from '@/components/common/listbox';
import { useServerConfig } from '@/components/server-config';
import { Button } from '@/components/ui/button';
import {
  Card, CardContent, CardHeader, CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { TooltipProvider } from '@/components/ui/tooltip';
import { withCascBuild } from '@/lib/api/casc-cache';
import { usePendingScrollToItem } from '@/lib/hooks/use-pending-scroll-to-item';
import { useScrollResetOnSearchChange } from '@/lib/hooks/use-scroll-reset-on-search-change';
import { useSearchSelectUrlSync } from '@/lib/hooks/use-search-select-url-sync';

type FileEntry = { fileDataID: number; fileName: string };

const OVERSCAN = 8;
const CONTAINER_PADDING = 4;

const suggestions = ['sound/creature/', 'sound/music/'] as const;
const AUTO_PLAY_KEY = 'browse-sound-autoplay';

const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
const markClass = 'bg-yellow-200 rounded px-0.5 text-neutral-950';

function Highlighted({
  text,
  regex,
  words,
}: {
  text: string;
  regex: RegExp | null;
  words: Set<string>;
}) {
  if (!regex) return text;
  const parts = text.split(regex);
  return parts.map((part, idx) => (
    words.has(part.toLowerCase())
      ? <mark key={idx} className={markClass}>{part}</mark>
      : <span key={idx}>{part}</span>
  ));
}

function SoundPlayer({ src, autoPlay }: { src: string; autoPlay: boolean }) {
  const [phase, setPhase] = useState<'loading' | 'ready' | 'failed'>('loading');

  const markReady = (el: HTMLAudioElement) => {
    const duration = el.duration;
    if (Number.isFinite(duration) && duration > 0) setPhase('ready');
  };

  return (
    <>
      {phase === 'loading' && (
        <div className="flex h-10 items-center" role="status" aria-label="Loading sound">
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        </div>
      )}
      {phase === 'failed' && <p className="text-sm text-destructive">Could not load this sound</p>}
      <audio
        src={src}
        controls
        autoPlay={autoPlay}
        className={phase === 'ready' ? 'w-full' : 'hidden'}
        onLoadedMetadata={(e) => markReady(e.currentTarget)}
        onDurationChange={(e) => markReady(e.currentTarget)}
        onError={() => setPhase('failed')}
      />
    </>
  );
}

function transcriptHasEveryWord(sub: string, words: readonly string[]): boolean {
  const lc = sub.toLowerCase();
  return words.every((word) => lc.includes(word.toLowerCase()));
}

function TranscriptBadge({ highlighted }: { highlighted: boolean }) {
  const iconClass = highlighted
    ? 'bg-yellow-200 text-neutral-950'
    : 'text-muted-foreground';
  return (
    <span className={`ml-1.5 inline-flex shrink-0 select-none items-center rounded p-0.5 ${iconClass}`}>
      <CaptionsIcon className="h-3.5 w-3.5" />
    </span>
  );
}

export default function BrowseSoundPage() {
  const { buildKey, isSharedHosting } = useServerConfig();
  const [allFiles, setAllFiles] = useState<FileEntry[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [selected, setSelected] = useState<FileEntry | null>(null);
  const [pendingScrollToPath, setPendingScrollToPath] = useState<string | null>(null);
  const [autoPlay, setAutoPlay] = useState(false);
  const [searchTranscript, setSearchTranscript] = useState(false);
  const [transcripts, setTranscripts] = useState<ReadonlyMap<number, string>>(() => new Map());
  const [playNonce, setPlayNonce] = useState(0);
  const listRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (isSharedHosting) return;
    setAutoPlay(localStorage.getItem(AUTO_PLAY_KEY) === '1');
  }, [isSharedHosting]);

  useEffect(() => {
    setAllFiles([]);
    setLoadError(null);
    fetch(withCascBuild('/api/sound', buildKey))
      .then(async (res) => {
        if (!res.ok) throw new Error(`Failed to fetch sound list (${res.status})`);
        const files: FileEntry[] = await res.json();
        if (files.length === 0) throw new Error('No sound files found');
        setAllFiles(files);
      })
      .catch((e: Error) => setLoadError(e.message));
  }, [buildKey]);

  useEffect(() => {
    fetch('/api/sound/transcripts')
      .then(async (res) => {
        if (!res.ok) return;
        const body: Record<string, string> = await res.json();
        const next = new Map<number, string>();
        for (const [id, sub] of Object.entries(body)) {
          const fileId = Number(id);
          if (fileId > 0 && sub) next.set(fileId, sub);
        }
        setTranscripts(next);
      })
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 250);
    return () => clearTimeout(t);
  }, [query]);

  const queryWords = useMemo(() => debouncedQuery.trim().split(/ +/).filter(Boolean), [debouncedQuery]);

  const filtered = useMemo(() => {
    if (queryWords.length === 0) return allFiles;
    const words = queryWords.map((w) => w.toLowerCase());
    return allFiles.filter((f) => {
      const nameLc = f.fileName.toLowerCase();
      const idStr = String(f.fileDataID);
      const sub = searchTranscript ? (transcripts.get(f.fileDataID) ?? '').toLowerCase() : '';
      return words.every((w) => nameLc.includes(w) || idStr.includes(w) || sub.includes(w));
    });
  }, [allFiles, queryWords, searchTranscript, transcripts]);

  const highlightRegex = useMemo(
    () => (queryWords.length === 0 ? null : new RegExp(`(${queryWords.map(escapeRegExp).join('|')})`, 'gi')),
    [queryWords],
  );
  const lowerWordsSet = useMemo(() => new Set(queryWords.map((w) => w.toLowerCase())), [queryWords]);

  useScrollResetOnSearchChange({
    containerRef: listRef,
    search: `${debouncedQuery}\n${searchTranscript ? '1' : '0'}`,
    isPending: !!pendingScrollToPath,
  });

  const handleSelect = useCallback((file: FileEntry) => {
    setSelected(file);
    if (!isSharedHosting && localStorage.getItem(AUTO_PLAY_KEY) === '1') setPlayNonce((n) => n + 1);
  }, [isSharedHosting]);

  usePendingScrollToItem<FileEntry>({
    items: filtered,
    containerRef: listRef,
    getRowHeight: () => FileRow.ROW_HEIGHT,
    contentPadding: CONTAINER_PADDING,
    matchKey: (f) => f.fileName,
    pendingKey: pendingScrollToPath,
    setPendingKey: setPendingScrollToPath,
    onSelect: handleSelect,
  });

  const resetLocalState = useCallback(() => {
    setQuery('');
    setDebouncedQuery('');
    setSelected(null);
    setPendingScrollToPath(null);
  }, []);

  useSearchSelectUrlSync({
    basePath: '/browse-sound',
    search: query,
    setSearch: setQuery,
    setDebouncedSearch: setDebouncedQuery,
    selectedPath: selected?.fileName ?? null,
    pendingScrollPath: pendingScrollToPath,
    setPendingScrollPath: setPendingScrollToPath,
    resetLocalState,
  });

  const applySuggestion = (s: typeof suggestions[number]) => {
    const v = `${s} `;
    setQuery(v);
    setDebouncedQuery(v);
    const el = inputRef.current;
    if (el) {
      el.focus();
      setTimeout(() => el.setSelectionRange(v.length, v.length), 0);
    }
  };

  const selectedUrl = selected ? withCascBuild(`/api/sound/${selected.fileDataID}`, buildKey) : undefined;
  const selectedTranscript = selected ? transcripts.get(selected.fileDataID) : undefined;

  return (
    <TooltipProvider>
    <div className="h-full p-4 flex flex-col overflow-x-hidden">
      <div className="mx-auto flex-1 flex flex-col w-full max-w-full">
        <div className="flex flex-col lg:flex-row gap-6 h-full min-w-0" style={{ height: 'calc(100vh - 125px)' }}>
          {/* Left: list */}
          <div className="lg:w-1/3 w-full lg:h-full h-[40vh] overflow-hidden min-w-0">
            <Card className="h-full flex flex-col min-w-0">
              <CardHeader className="flex flex-row justify-between items-center py-2 px-3 pb-0 pt-3">
                <CardTitle className="text-lg flex items-center gap-2">
                  <Music2Icon className="w-4 h-4" />
                  Browse Sound Files
                </CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col flex-1 overflow-hidden p-3 min-w-0">
                <div className="relative w-full mb-2">
                  <Input
                    placeholder="Search sound, e.g. 'sound/creature/illidan'..."
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Escape') {
                        e.preventDefault();
                        setQuery('');
                        setDebouncedQuery('');
                      }
                    }}
                    ref={inputRef}
                    className="w-full sm:pr-[240px]"
                  />
                  <div className="absolute inset-y-0 right-1.5 hidden sm:flex items-center gap-1.5 pointer-events-none z-20">
                    {suggestions.map((s) => (
                      <button
                        key={s}
                        type="button"
                        className="text-[10px] sm:text-xs px-1.5 py-0.5 sm:px-2 sm:py-1 rounded bg-secondary hover:bg-accent border border-border pointer-events-auto"
                        onClick={() => applySuggestion(s)}
                        title={`Search for ${s}`}
                      >
                        {s}
                      </button>
                    ))}
                  </div>
                </div>
                <div className="flex items-center justify-between gap-2 mb-2 text-xs text-muted-foreground">
                  <span>{loadError ?? (allFiles.length === 0 ? 'Loading sound list...' : `${filtered.length.toLocaleString()} files`)}</span>
                  <div className="flex items-center gap-2 shrink-0">
                    {!isSharedHosting && (
                      <label className="flex items-center gap-1.5 cursor-pointer">
                        Auto-play
                        <span className="relative inline-block h-3.5 w-6 shrink-0">
                          <Switch
                            className="absolute left-0 top-0 origin-top-left scale-[0.55]"
                            checked={autoPlay}
                            onCheckedChange={(on) => {
                              setAutoPlay(on);
                              localStorage.setItem(AUTO_PLAY_KEY, on ? '1' : '0');
                            }}
                            aria-label="Auto-play"
                          />
                        </span>
                      </label>
                    )}
                    <label className="flex items-center gap-1.5 cursor-pointer">
                      Transcript search
                      <span className="relative inline-block h-3.5 w-6 shrink-0">
                        <Switch
                          className="absolute left-0 top-0 origin-top-left scale-[0.55]"
                          checked={searchTranscript}
                          onCheckedChange={setSearchTranscript}
                          aria-label="Transcript search"
                        />
                      </span>
                    </label>
                  </div>
                </div>
                <VirtualListBox<FileEntry>
                  items={filtered}
                  listKey={`${debouncedQuery}\n${searchTranscript ? '1' : '0'}`}
                  containerRef={listRef}
                  containerClassName="overflow-y-scroll overflow-x-auto border rounded-md bg-background flex-1"
                  contentPadding={CONTAINER_PADDING}
                  overscan={OVERSCAN}
                  getRowKey={(f) => f.fileDataID}
                  fixedRowHeight={FileRow.ROW_HEIGHT}
                  renderRow={(file, index, style) => {
                    const isSelected = selected === file;
                    const sub = transcripts.get(file.fileDataID);
                    const matchedByTranscript = searchTranscript
                      && queryWords.length > 0
                      && sub !== undefined
                      && transcriptHasEveryWord(sub, queryWords);
                    return (
                      <FileRow
                        file={file}
                        index={index}
                        isSelected={isSelected}
                        isBusy={false}
                        highlightRegex={highlightRegex}
                        lowerWordsSet={lowerWordsSet}
                        clickWhenSelected={!isSharedHosting && autoPlay}
                        onClick={handleSelect}
                        style={style}
                        nameExtra={sub ? <TranscriptBadge highlighted={matchedByTranscript} /> : undefined}
                        nameTooltip={sub ? (
                          <Highlighted
                            text={sub}
                            regex={queryWords.length > 0 ? highlightRegex : null}
                            words={lowerWordsSet}
                          />
                        ) : undefined}
                      />
                    );
                  }}
                />
              </CardContent>
            </Card>
          </div>

          {/* Right: player */}
          <div className="lg:w-2/3 w-full h-full overflow-y-auto p-0 relative min-w-0">
            {selected && selectedUrl ? (
              <div className="h-full overflow-auto bg-secondary">
                <div className="mx-auto flex w-full max-w-xl flex-col gap-5 px-8 py-8">
                  <p className="min-w-0 break-all font-mono text-sm leading-5">
                    <span className="inline-block max-w-full break-all align-bottom">{selected.fileName}</span>
                    <span className="inline-block select-none px-0.5" aria-hidden>{'\u00a0'}</span>
                    <span className="inline-block align-bottom text-yellow-600">[{selected.fileDataID}]</span>
                  </p>
                  {selectedTranscript && (
                    <p className="rounded-md bg-background px-4 py-3 text-lg leading-7">
                      <Highlighted
                        text={selectedTranscript}
                        regex={searchTranscript ? highlightRegex : null}
                        words={lowerWordsSet}
                      />
                    </p>
                  )}
                  <SoundPlayer
                    key={`${selected.fileDataID}-${playNonce}`}
                    src={selectedUrl}
                    autoPlay={!isSharedHosting && autoPlay}
                  />
                  <Button asChild variant="outline" size="sm" className="w-fit">
                    <a href={`${selectedUrl}${selectedUrl.includes('?') ? '&' : '?'}download=1`} download>
                      <DownloadIcon />
                      Download {selected.fileName.slice(selected.fileName.lastIndexOf('.'))}
                    </a>
                  </Button>
                </div>
              </div>
            ) : (
              <div className="absolute inset-0 bg-secondary flex items-center justify-center text-muted-foreground">
                <p>Select a sound to play</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
    </TooltipProvider>
  );
}

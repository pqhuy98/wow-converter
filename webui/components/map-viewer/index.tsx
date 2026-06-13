'use client';

import { Copy } from 'lucide-react';
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';

import { Button } from '@/components/ui/button';
import {
  Card, CardContent, CardHeader, CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
import { usePendingScrollToItem } from '@/lib/hooks/use-pending-scroll-to-item';
import { useScrollResetOnSearchChange } from '@/lib/hooks/use-scroll-reset-on-search-change';
import { useSearchSelectUrlSync } from '@/lib/hooks/use-search-select-url-sync';
import type { GenerateWc3FormValues, MapGenerateJobStatus } from '@/lib/models/map-generate.model';
import {
  clearStoredGenerateJob,
  formatElapsedDuration,
  formatProgressLabel,
  persistGenerateJobFromStatus,
  readStoredGenerateJob,
} from '@/lib/models/map-generate.model';
import {
  expansionsPresentInMaps,
  WOW_EXPANSION_ALL,
} from '@/lib/wow-expansions';

import { useServerConfig } from '../server-config';
import { ExpansionFilterBar } from './expansion-filter-bar';
import { ExpansionIcon } from './expansion-icon';
import GenerateWc3Dialog from './generate-wc3-dialog';
import MinimapViewer, { MapInfo } from './minimap-viewer';

interface MapResponse {
  id: number | string;
  name: string;
  dir: string;
  expansionID?: number;
}

type TextureResolution = '0' | '512' | '1024' | '4096' | '8192' | '16384'

const showMapExport = true;
const POLL_INTERVAL_MS = 500;

function defaultMapSaveName(mapDir: string, tiles: { x: number; y: number }[]): string {
  if (tiles.length === 0) return `${mapDir}.w3x`;
  const xs = tiles.map((t) => t.x);
  const ys = tiles.map((t) => t.y);
  const minX = Math.min(...xs);
  const maxX = Math.max(...xs);
  const minY = Math.min(...ys);
  const maxY = Math.max(...ys);
  const suffix = minX === maxX && minY === maxY
    ? `${minX}_${minY}`
    : `${minX}_${minY}-${maxX}_${maxY}`;
  return `${mapDir}-${suffix}.w3x`;
}

function isActiveJob(job: MapGenerateJobStatus | undefined): job is MapGenerateJobStatus {
  return job?.status === 'pending' || job?.status === 'processing';
}

export default function MapViewer() {
  const { isSharedHosting } = useServerConfig();
  const [maps, setMaps] = useState<MapResponse[]>([]);
  const [mapsError, setMapsError] = useState<string | null>(null);
  const [selectedMapDir, setSelectedMapDir] = useState<string | null>(null);
  const [mapInfo, setMapInfo] = useState<MapInfo | null>(null);
  const [hover, setHover] = useState<{ tile: { x: number; y: number } | null }>({ tile: null });
  const [selectedTiles, setSelectedTiles] = useState<{ x: number; y: number }[]>([]);
  const [texSize, setTexSize] = useState<TextureResolution>('8192');
  const [generateJob, setGenerateJob] = useState<MapGenerateJobStatus | undefined>(undefined);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [clockNow, setClockNow] = useState(() => Date.now());

  const pollInFlightRef = useRef(false);
  const jobIdRef = useRef<string | undefined>(undefined);

  const isGenerating = isActiveJob(generateJob);

  const listRef = useRef<HTMLDivElement | null>(null);
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [expansionFilter, setExpansionFilter] = useState<number>(WOW_EXPANSION_ALL);
  const [pendingScrollToMapName, setPendingScrollToMapName] = useState<string | null>(null);
  const [viewportHeight, setViewportHeight] = useState(400);
  const [scrollTop, setScrollTop] = useState(0);
  const ROW_HEIGHT = 32;
  const OVERSCAN = 8;

  const applyJobUpdate = useCallback((data: MapGenerateJobStatus) => {
    setGenerateJob(data);
    persistGenerateJobFromStatus(data);
  }, []);

  useEffect(() => {
    void (async () => {
      const res = await fetch('/api/maps', { cache: 'no-store' });
      if (!res.ok) {
        setMapsError('Failed to fetch maps');
        return;
      }
      const data = await res.json();
      if (!Array.isArray(data) || data.length === 0) {
        setMapsError('No maps available');
        return;
      }
      type MapsApiItem = { id: number | string; name: string; dir?: string; expansionID?: number };
      setMaps((data as MapsApiItem[]).map((m) => ({
        id: m.id,
        name: m.name,
        dir: m.dir ?? String(m.id),
        expansionID: typeof m.expansionID === 'number' ? m.expansionID : undefined,
      })));
    })();
  }, []);

  useEffect(() => {
    void (async () => {
      const stored = readStoredGenerateJob();
      if (stored?.jobId) {
        try {
          const res = await fetch(`/api/maps/generate-wc3/status/${stored.jobId}`, { cache: 'no-store' });
          if (res.ok) {
            const data = (await res.json()) as MapGenerateJobStatus;
            if (isActiveJob(data)) {
              applyJobUpdate(data);
              if (stored.mapDir) setSelectedMapDir(stored.mapDir);
              return;
            }
            clearStoredGenerateJob();
          }
        } catch {
          // fall through to active jobs lookup
        }
      }

      try {
        const res = await fetch('/api/maps/generate-wc3/active', { cache: 'no-store' });
        if (!res.ok) return;
        const jobs = (await res.json()) as MapGenerateJobStatus[];
        const active = jobs.find(isActiveJob);
        if (active) {
          applyJobUpdate(active);
          if (active.mapDir) setSelectedMapDir(active.mapDir);
        }
      } catch {
        // ignore restore errors
      }
    })();
  }, [applyJobUpdate]);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 200);
    return () => clearTimeout(t);
  }, [query]);

  const filteredMaps = useMemo(() => {
    let list = maps;
    if (expansionFilter !== WOW_EXPANSION_ALL) {
      list = list.filter((m) => m.expansionID === expansionFilter);
    }
    const q = debouncedQuery.trim().toLowerCase();
    if (!q) return list;
    const words = q.split(/ +/).filter(Boolean);
    return list.filter((m) => words.every((w) => (
      m.name?.toLowerCase().includes(w) || String(m.dir).toLowerCase().includes(w)
    )));
  }, [maps, debouncedQuery, expansionFilter]);

  const expansionFilterOptions = useMemo(
    () => expansionsPresentInMaps(maps),
    [maps],
  );

  const selectedMapName = useMemo(
    () => maps.find((m) => m.dir === selectedMapDir)?.name ?? null,
    [maps, selectedMapDir],
  );

  const resetMapsUrlState = useCallback(() => {
    setQuery('');
    setDebouncedQuery('');
    setSelectedMapDir(null);
    setMapInfo(null);
    setSelectedTiles([]);
    setPendingScrollToMapName(null);
  }, []);

  const selectedPathForUrl = selectedMapDir == null
    ? null
    : (selectedMapName ?? undefined);

  useSearchSelectUrlSync({
    basePath: '/maps',
    search: query,
    setSearch: setQuery,
    setDebouncedSearch: setDebouncedQuery,
    selectedPath: selectedPathForUrl,
    pendingScrollPath: pendingScrollToMapName,
    setPendingScrollPath: setPendingScrollToMapName,
    resetLocalState: resetMapsUrlState,
  });

  useScrollResetOnSearchChange({
    containerRef: listRef,
    search: debouncedQuery,
    isPending: !!pendingScrollToMapName,
  });

  useEffect(() => {
    if (listRef.current) listRef.current.scrollTop = 0;
    setScrollTop(0);
  }, [expansionFilter]);

  usePendingScrollToItem<MapResponse>({
    items: filteredMaps,
    containerRef: listRef,
    getRowHeight: () => ROW_HEIGHT,
    contentPadding: 0,
    matchKey: (m) => m.name,
    pendingKey: pendingScrollToMapName,
    setPendingKey: setPendingScrollToMapName,
    onSelect: (m) => setSelectedMapDir(m.dir),
  });

  useEffect(() => {
    const el = listRef.current;
    if (!el) return undefined;
    const update = () => setViewportHeight(el.clientHeight || 400);
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, [listRef.current]);

  useEffect(() => {
    if (!isGenerating) return undefined;
    const t = setInterval(() => setClockNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [isGenerating]);

  const total = filteredMaps.length;
  const startIndex = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN);
  const endIndex = Math.min(total - 1, Math.ceil((scrollTop + viewportHeight) / ROW_HEIGHT) + OVERSCAN);
  const visibleItems = filteredMaps.slice(startIndex, endIndex + 1);

  useEffect(() => {
    if (!selectedMapDir) return;
    void (async () => {
      const res = await fetch(`/api/maps/${encodeURIComponent(selectedMapDir)}/wdt-mask`, { cache: 'no-store' });
      if (!res.ok) {
        setMapInfo(null);
        return;
      }
      const data = await res.json();
      const maskMatrix: boolean[][] = Array.from({ length: 64 }, () => Array.from({ length: 64 }, () => false));
      const textureMatrix: boolean[][] = Array.from({ length: 64 }, () => Array.from({ length: 64 }, () => false));
      if (Array.isArray(data.tiles)) {
        for (const t of data.tiles as { x: number; y: number; hasTexture: boolean }[]) {
          if (t.y >= 0 && t.y < 64 && t.x >= 0 && t.x < 64) {
            maskMatrix[t.y][t.x] = true;
            if (t.hasTexture) textureMatrix[t.y][t.x] = true;
          }
        }
      }
      setMapInfo({
        mapId: selectedMapDir,
        mask: maskMatrix,
        textureMask: textureMatrix,
      });
    })();
  }, [selectedMapDir]);

  useEffect(() => {
    setSelectedTiles([]);
  }, [selectedMapDir]);

  useEffect(() => {
    setHover({ tile: null });
  }, [mapInfo]);

  const onGenerateWc3 = useCallback(async (form: GenerateWc3FormValues) => {
    if (!mapInfo || selectedTiles.length === 0 || isGenerating) return;
    setGenerateJob({
      id: '',
      status: 'pending',
      submittedAt: Date.now(),
      mapSaveName: form.mapSaveName,
      mapDir: String(mapInfo.mapId),
    });
    try {
      const body = {
        tiles: selectedTiles,
        quality: parseInt(texSize, 10),
        mapSaveName: form.mapSaveName,
        clampLower: form.clampLower,
        clampUpper: form.clampUpper,
        mapAngleDeg: form.mapAngleDeg,
        unitScale: form.unitScale,
        freshExport: form.freshExport,
        creatures: form.creatures,
      };
      const res = await fetch(`/api/maps/${encodeURIComponent(String(mapInfo.mapId))}/generate-wc3`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const text = await res.text();
        setGenerateJob({
          id: '',
          status: 'failed',
          error: text,
          submittedAt: Date.now(),
        });
        clearStoredGenerateJob();
        return;
      }
      const data = (await res.json()) as MapGenerateJobStatus;
      applyJobUpdate(data);
    } catch (e) {
      setGenerateJob({
        id: '',
        status: 'failed',
        error: e instanceof Error ? e.message : String(e),
        submittedAt: Date.now(),
      });
      clearStoredGenerateJob();
    }
  }, [mapInfo, selectedTiles, texSize, isGenerating, applyJobUpdate]);

  useEffect(() => {
    jobIdRef.current = generateJob?.id || undefined;
  }, [generateJob?.id]);

  useEffect(() => {
    const jobId = generateJob?.id;
    const terminal = generateJob?.status === 'done' || generateJob?.status === 'failed';
    if (!jobId || terminal) return undefined;

    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const scheduleNext = () => {
      if (cancelled) return;
      timer = setTimeout(() => void pollOnce(), POLL_INTERVAL_MS);
    };

    const pollOnce = async () => {
      if (cancelled || !jobIdRef.current) return;
      if (pollInFlightRef.current) {
        scheduleNext();
        return;
      }

      pollInFlightRef.current = true;
      let terminal = false;
      try {
        const res = await fetch(`/api/maps/generate-wc3/status/${jobIdRef.current}`, { cache: 'no-store' });
        if (!res.ok) throw new Error(await res.text());
        const data = (await res.json()) as MapGenerateJobStatus;
        if (cancelled) return;

        if (data.status === 'done' || data.status === 'failed') {
          applyJobUpdate(data);
          terminal = true;
          return;
        }

        setGenerateJob((prev) => {
          if (!prev) return data;
          const prevSteps = prev.progress?.completedSteps ?? 0;
          const nextSteps = data.progress?.completedSteps ?? prevSteps;
          const merged = data.status === 'processing' && nextSteps < prevSteps
            ? { ...data, progress: prev.progress }
            : data;
          persistGenerateJobFromStatus(merged);
          return merged;
        });
      } catch (e) {
        if (!cancelled) {
          setGenerateJob({
            id: jobIdRef.current ?? jobId,
            status: 'failed',
            error: e instanceof Error ? e.message : String(e),
            submittedAt: generateJob?.submittedAt ?? Date.now(),
          });
          clearStoredGenerateJob();
        }
        terminal = true;
      } finally {
        pollInFlightRef.current = false;
        if (!cancelled && !terminal && jobIdRef.current) scheduleNext();
      }
    };

    void pollOnce();

    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [generateJob?.id, generateJob?.status, generateJob?.submittedAt, applyJobUpdate]);

  const suggestedMapName = mapInfo
    ? defaultMapSaveName(String(mapInfo.mapId), selectedTiles)
    : '';

  const elapsedMs = generateJob
    ? clockNow - (generateJob.startedAt ?? generateJob.submittedAt)
    : 0;

  const displayMapSaveName = generateJob?.mapSaveName
    ?? generateJob?.result?.mapSaveName;

  return (
    <div className="h-full p-4 flex flex-col overflow-x-hidden">
      <GenerateWc3Dialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        defaultMapSaveName={suggestedMapName}
        tileCount={selectedTiles.length}
        onConfirm={(values) => void onGenerateWc3(values)}
      />
      <div className="mx-auto flex-1 flex flex-col w-full max-w-full">
        <div className="mb-2" />
        <div className="flex flex-col lg:flex-row gap-6 h-full min-w-0" style={{ height: 'calc(100vh - 125px)' }}>
          <div className="lg:w-1/4 w-full lg:h-full h-[40vh] overflow-hidden min-w-0">
            <Card className="h-full flex flex-col min-w-0">
              <CardHeader className="flex flex-row justify-between items-center py-2 px-3 pb-0 pt-3">
                <CardTitle className="text-lg">Maps</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 flex flex-col min-h-0 flex-1 overflow-hidden p-3 min-w-0">
                <div className="flex gap-2">
                  <Input
                    className="flex-1 min-w-0"
                    placeholder="Search maps..."
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                  />
                  <ExpansionFilterBar
                    expansions={expansionFilterOptions}
                    value={expansionFilter}
                    onChange={setExpansionFilter}
                  />
                </div>
                <div
                  ref={listRef}
                  className="mt-2 flex-1 min-h-0 overflow-y-auto border rounded-md bg-background"
                  onScroll={(e) => setScrollTop((e.target as HTMLDivElement).scrollTop)}
                >
                  {!mapsError ? (
                    <div style={{ height: total * ROW_HEIGHT, position: 'relative' }}>
                      <div style={{
                        position: 'absolute',
                        top: startIndex * ROW_HEIGHT,
                        left: 0,
                        right: 0,
                        minWidth: '100%',
                        width: 'max-content',
                      }}>
                        {visibleItems.map((m) => {
                          const isSelected = mapInfo?.mapId === m.dir;
                          return (
                            <div
                              key={String(m.id)}
                              style={{ height: ROW_HEIGHT }}
                              className={`font-mono px-2 flex items-center text-sm min-w-full w-max cursor-pointer whitespace-nowrap gap-2 ${isSelected ? 'bg-primary/20' : 'hover:bg-accent'}`}
                              onClick={() => setSelectedMapDir(m.dir)}
                              title={m.dir}
                            >
                              {typeof m.expansionID === 'number' && (
                                <ExpansionIcon expansionID={m.expansionID} />
                              )}
                              <span>[<span className="text-yellow-600">{m.id}</span>]</span>
                              <span className="text-foreground/80">{m.name}</span>
                              <span className="text-muted-foreground/60">({m.dir})</span>
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  ) : (
                    <div className="text-destructive text-sm p-2">{mapsError}</div>
                  )}
                </div>
                {!isSharedHosting && showMapExport && <div>
                  <div className="flex items-center gap-2 pt-2 mt-auto">
                    <label className="text-sm text-muted-foreground whitespace-nowrap">Terrain texture size</label>
                    <select className="border rounded px-2 py-1 bg-background" value={texSize} onChange={(e) => setTexSize(e.target.value as TextureResolution)}>
                      <option value="0">None</option>
                      <option value="512">512</option>
                      <option value="1024">1024</option>
                      <option value="4096">4096</option>
                      <option value="8192">8192</option>
                      <option value="16384">16384</option>
                    </select>
                    <Button
                      type="button"
                      className="ml-auto"
                      onClick={() => setDialogOpen(true)}
                      disabled={!mapInfo || selectedTiles.length === 0 || isGenerating}
                    >
                      {isGenerating ? 'Generating…' : `Generate WC3 map (${selectedTiles.length})`}
                    </Button>
                  </div>
                  {generateJob && generateJob.status !== 'done' && (
                    <div className="pt-3 space-y-2">
                      {displayMapSaveName && (
                        <p className="text-xs font-medium truncate" title={displayMapSaveName}>
                          {displayMapSaveName}
                        </p>
                      )}
                      <div className="flex items-center gap-2">
                        <Progress
                          className="flex-1"
                          value={generateJob.progress?.percent ?? (generateJob.status === 'pending' ? 0 : undefined)}
                        />
                        <span className="text-xs text-muted-foreground tabular-nums whitespace-nowrap">
                          {formatElapsedDuration(elapsedMs)}
                        </span>
                      </div>
                      <p className="text-xs text-muted-foreground">
                        {formatProgressLabel(generateJob)}
                      </p>
                      {generateJob.status === 'processing'
                        && generateJob.queuePending != null
                        && generateJob.queuePending > 0 && (
                        <p className="text-xs text-muted-foreground">
                          {generateJob.queuePending} job{generateJob.queuePending === 1 ? '' : 's'} waiting in queue
                        </p>
                      )}
                    </div>
                  )}
                  {generateJob?.status === 'done' && generateJob.result && (
                    <div className="pt-2 space-y-2">
                      <p className="text-xs text-green-600">
                        Generated {generateJob.result.mapSaveName} in {formatElapsedDuration(
                        (generateJob.finishedAt ?? clockNow) - generateJob.submittedAt,
                      )} — exported {generateJob.result.succeeded.length}/{generateJob.result.total} tiles
                        {generateJob.result.failed.length > 0
                          ? ` (${generateJob.result.failed.length} tile export failures)`
                          : ''}
                      </p>
                      {generateJob.result.outputDir && (
                        <div className="flex items-center gap-2">
                          <Button
                            type="button"
                            variant="outline"
                            size="icon"
                            className="shrink-0 h-7 w-7"
                            title="Copy map path"
                            onClick={() => {
                              void navigator.clipboard.writeText(generateJob.result!.outputDir);
                            }}
                          >
                            <Copy className="h-3.5 w-3.5" />
                          </Button>
                          <span
                            className="text-xs font-mono select-all break-all text-muted-foreground"
                            title={generateJob.result.outputDir}
                          >
                            {generateJob.result.outputDir}
                          </span>
                        </div>
                      )}
                    </div>
                  )}
                  {generateJob?.status === 'failed' && (
                    <p className="text-xs text-destructive pt-2">{generateJob.error ?? 'Generation failed'}</p>
                  )}
                </div>}
              </CardContent>
            </Card>
          </div>

          <div className="lg:w-3/4 w-full h-full overflow-hidden min-w-0">
              <div className="p-0 h-full relative overflow-hidden min-w-0 rounded-md border bg-background">
                {mapInfo && (
                  <MinimapViewer
                    mapInfo={mapInfo}
                    className="w-full h-full block"
                    onHoverChange={(tile) => setHover({ tile })}
                    onSelectionChange={(tiles) => setSelectedTiles(tiles)}
                  />
                )}
                {!mapInfo && (
                  <div className="absolute inset-0 flex items-center justify-center text-foreground/60 text-2xl">
                    {!mapsError ? 'Select a map to view minimap' : mapsError}
                  </div>
                )}
                {selectedMapDir && !mapInfo && (
                  <div className="absolute inset-0 flex items-center justify-center text-muted-foreground text-sm pointer-events-none">
                    Loading...
                  </div>
                )}
                <div className="absolute bottom-2 left-2 text-xs text-muted-foreground bg-background/80 rounded px-2 py-1">
                  {hover.tile ? (
                    <span>Tile {hover.tile.x},{hover.tile.y}</span>
                  ) : (
                    <span>Hover tiles to see coordinates</span>
                  )}
                </div>
              </div>
          </div>
        </div>
      </div>
    </div>
  );
}

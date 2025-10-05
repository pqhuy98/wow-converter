'use client';

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

import { useServerConfig } from '../server-config';
import MinimapViewer, { MapInfo } from './minimap-viewer';

interface MapResponse { id: number | string; name: string; dir: string }

type TextureResolution = '512' | '1024' | '4096' | '8192' | '16384'

const showMapExport = false;

export default function MapViewer() {
  const { isDev } = useServerConfig();
  const [maps, setMaps] = useState<MapResponse[]>([]);
  const [mapsError, setMapsError] = useState<string | null>(null);
  const [selectedMapDir, setSelectedMapDir] = useState<string | null>(null);
  const [mapInfo, setMapInfo] = useState<MapInfo | null>(null);
  const [hover, setHover] = useState<{ tile: { x: number; y: number } | null }>({ tile: null });
  const [selectedTiles, setSelectedTiles] = useState<{ x: number; y: number }[]>([]);
  const [texSize, setTexSize] = useState<TextureResolution>('4096');
  const [includeWMO, setIncludeWMO] = useState(true);
  const [includeM2, setIncludeM2] = useState(true);
  const [includeWMOSets, setIncludeWMOSets] = useState(true);
  const [includeGameObjects, setIncludeGameObjects] = useState(true);
  const [includeLiquid, setIncludeLiquid] = useState(true);
  const [includeFoliage, setIncludeFoliage] = useState(true);
  const [includeHoles, setIncludeHoles] = useState(true);
  const [isExporting, setIsExporting] = useState(false);

  // Virtualized list state (borrowed from browse page)
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const listRef = useRef<HTMLDivElement | null>(null);
  const [viewportHeight, setViewportHeight] = useState(400);
  const [scrollTop, setScrollTop] = useState(0);
  const ROW_HEIGHT = 32;
  const OVERSCAN = 8;

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
      type MapsApiItem = { id: number | string; name: string; dir?: string };
      setMaps((data as MapsApiItem[]).map((m) => ({ id: m.id, name: m.name, dir: m.dir ?? String(m.id) })));
    })();
  }, []);

  // debounce search
  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 200);
    return () => clearTimeout(t);
  }, [query]);

  // reset scroll on new query
  useEffect(() => {
    const el = listRef.current;
    if (!el) return;
    el.scrollTop = 0;
    setScrollTop(0);
  }, [debouncedQuery]);

  useEffect(() => {
    const el = listRef.current;
    if (!el) return undefined;
    const update = () => setViewportHeight(el.clientHeight || 400);
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, [listRef.current]);

  const filteredMaps = useMemo(() => {
    const q = debouncedQuery.trim().toLowerCase();
    if (!q) return maps;
    const words = q.split(/ +/).filter(Boolean);
    return maps.filter((m) => words.every((w) => (
      m.name?.toLowerCase().includes(w) || String(m.dir).toLowerCase().includes(w)
    )));
  }, [maps, debouncedQuery]);

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
      setSelectedTiles([]);
    })();
  }, [selectedMapDir]);

  // When selecting a new map, clear hover and force a quick redraw by toggling state minimaly
  useEffect(() => {
    setHover({ tile: null });
  }, [mapInfo]);

  const onExportTerrain = useCallback(async () => {
    if (!mapInfo || selectedTiles.length === 0) return;
    setIsExporting(true);
    try {
      const body = {
        tiles: selectedTiles,
        quality: parseInt(texSize, 10),
        includeM2,
        includeWMO,
        includeWMOSets,
        includeGameObjects,
        includeLiquid,
        includeFoliage,
        includeHoles,
      };
      const res = await fetch(`/api/maps/${encodeURIComponent(String(mapInfo.mapId))}/export-adt`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        console.error('Export failed', await res.text());
      } else {
        const data = await res.json();
        console.log('Export finished', data);
      }
    } catch (e) {
      console.error('Export error', e);
    } finally {
      setIsExporting(false);
    }
  }, [mapInfo, selectedTiles, texSize, includeM2, includeWMO, includeWMOSets, includeGameObjects, includeLiquid, includeFoliage, includeHoles]);

  return (
    <div className="h-full p-4 flex flex-col overflow-x-hidden">
      <div className="mx-auto flex-1 flex flex-col w-full max-w-full">
        <div className="mb-2" />
        <div className="flex flex-col lg:flex-row gap-6 h-full min-w-0" style={{ height: 'calc(100vh - 125px)' }}>
          {/* Left: map list & controls */}
          <div className="lg:w-1/4 w-full lg:h-full h-[40vh] overflow-hidden min-w-0">
            <Card className="h-full flex flex-col min-w-0">
              <CardHeader className="flex flex-row justify-between items-center py-2 px-3 pb-0 pt-3">
                <CardTitle className="text-lg">Maps</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 flex flex-col min-h-0 flex-1 overflow-hidden p-3 min-w-0">
                <Input
                  placeholder="Search maps..."
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
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
                {isDev && showMapExport && <div>
                  <div className="flex flex-wrap gap-3 pt-2 text-sm">
                    <label className="inline-flex items-center gap-2">
                      <input type="checkbox" checked={includeWMO} onChange={(e) => setIncludeWMO(e.target.checked)} />
                      WMO
                    </label>
                    <label className="inline-flex items-center gap-2">
                      <input type="checkbox" checked={includeWMOSets} onChange={(e) => setIncludeWMOSets(e.target.checked)} />
                      WMO sets
                    </label>
                    <label className="inline-flex items-center gap-2">
                      <input type="checkbox" checked={includeM2} onChange={(e) => setIncludeM2(e.target.checked)} />
                      M2
                    </label>
                    <label className="inline-flex items-center gap-2">
                      <input type="checkbox" checked={includeGameObjects} onChange={(e) => setIncludeGameObjects(e.target.checked)} />
                      Gameobjects
                    </label>
                    <label className="inline-flex items-center gap-2">
                      <input type="checkbox" checked={includeLiquid} onChange={(e) => setIncludeLiquid(e.target.checked)} />
                      Liquid
                    </label>
                    <label className="inline-flex items-center gap-2">
                      <input type="checkbox" checked={includeFoliage} onChange={(e) => setIncludeFoliage(e.target.checked)} />
                      Foliage
                    </label>
                    <label className="inline-flex items-center gap-2">
                      <input type="checkbox" checked={includeHoles} onChange={(e) => setIncludeHoles(e.target.checked)} />
                      Holes
                    </label>
                  </div>
                  <div className="flex items-center gap-2 pt-2 mt-auto">
                    <label className="text-sm text-muted-foreground">Texture size</label>
                    <select className="border rounded px-2 py-1 bg-background" value={texSize} onChange={(e) => setTexSize(e.target.value as TextureResolution)}>
                      <option value="512">512</option>
                      <option value="1024">1024</option>
                      <option value="4096">4096</option>
                      <option value="8192">8192</option>
                      <option value="16384">16384</option>
                    </select>
                    <Button className="ml-auto" onClick={() => void onExportTerrain()} disabled={!mapInfo || selectedTiles.length === 0 || isExporting}>
                      {isExporting ? 'Exporting…' : `Export Tiles (${selectedTiles.length})`}
                    </Button>
                  </div>
                </div>}
              </CardContent>
            </Card>
          </div>

          {/* Right: minimap */}
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

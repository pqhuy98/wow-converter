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

import MinimapCanvas, { MinimapCanvasProps } from './minimap-canvas';

interface MapInfo { id: number | string; name: string; dir: string }

type TextureResolution = '512' | '1024' | '2048' | '4096'

export default function MapViewer() {
  const [maps, setMaps] = useState<MapInfo[]>([]);
  const [mapsError, setMapsError] = useState<string | null>(null);
  const [selectedMapDir, setSelectedMapDir] = useState<string | null>(null);
  const [mask, setMask] = useState<boolean[][] | null>(null);
  const [textureMask, setTextureMask] = useState<boolean[][] | null>(null);
  const [hover, setHover] = useState<{ tile: { x: number; y: number } | null; world: { x: number; y: number } | null }>({ tile: null, world: null });
  const [selectedTiles, setSelectedTiles] = useState<{ x: number; y: number }[]>([]);
  const [texSize, setTexSize] = useState<TextureResolution>('512');

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
    setMask(null);
    setTextureMask(null);
    void (async () => {
      const res = await fetch(`/api/maps/${encodeURIComponent(selectedMapDir)}/wdt-mask`, { cache: 'no-store' });
      if (!res.ok) {
        setMask(null);
        setTextureMask(null);
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
      setMask(maskMatrix);
      setTextureMask(textureMatrix);
      setSelectedTiles([]);
    })();
  }, [selectedMapDir]);

  // When selecting a new map, clear hover and force a quick redraw by toggling state minimaly
  useEffect(() => {
    setHover({ tile: null, world: null });
  }, [selectedMapDir]);

  const onHoverChange = useCallback<NonNullable<MinimapCanvasProps['onHoverChange']>>((tile, world) => {
    setHover({ tile, world });
  }, []);

  const onSelectionChange = useCallback<NonNullable<MinimapCanvasProps['onSelectionChange']>>((tiles) => {
    setSelectedTiles(tiles);
  }, []);

  const onViewTerrain = useCallback(() => {
    console.log('View Terrain clicked', { map: selectedMapDir, tiles: selectedTiles, texSize });
  }, [selectedMapDir, selectedTiles, texSize]);

  const emptyMask = useMemo(() => Array.from({ length: 64 }, () => Array.from({ length: 64 }, () => false)), []);

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
              <CardContent className="space-y-2 flex-1 overflow-hidden p-3 min-w-0">
                <Input
                  placeholder="Search maps..."
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
                <div
                  ref={listRef}
                  className="mt-2 overflow-y-scroll border rounded-md bg-background h-[calc(100vh-296px)]"
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
                          const isSelected = selectedMapDir === m.dir;
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
                <div className="flex items-center gap-2 pt-2">
                  <label className="text-sm text-muted-foreground">Texture size</label>
                  <select className="border rounded px-2 py-1 bg-background" value={texSize} onChange={(e) => setTexSize(e.target.value as TextureResolution)}>
                    <option value="512">512</option>
                    <option value="1024">1024</option>
                    <option value="2048">2048</option>
                    <option value="4096">4096</option>
                  </select>
                  <Button className="ml-auto" onClick={onViewTerrain} disabled={!mask || selectedTiles.length === 0}>
                    View Terrain ({selectedTiles.length})
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Right: minimap */}
          <div className="lg:w-3/4 w-full h-full overflow-hidden min-w-0">
            <div className="p-0 h-full relative overflow-hidden min-w-0 rounded-md border bg-background">
              {selectedMapDir && (
                <MinimapCanvas
                  mapId={selectedMapDir}
                  mask={mask ?? emptyMask}
                  textureMask={textureMask ?? emptyMask}
                  tileSize={64}
                  className="w-full h-full block"
                  onHoverChange={onHoverChange}
                  onSelectionChange={onSelectionChange}
                />
              )}
              {!selectedMapDir && (
                <div className="absolute inset-0 flex items-center justify-center text-foreground/60 text-2xl">
                  {!mapsError ? 'Select a map to view minimap' : mapsError}
                </div>
              )}
              {selectedMapDir && !mask && (
                <div className="absolute inset-0 flex items-center justify-center text-muted-foreground text-sm pointer-events-none">
                  Loading mask…
                </div>
              )}
              <div className="absolute bottom-2 left-2 text-xs text-muted-foreground bg-background/80 rounded px-2 py-1">
                {hover.tile ? (
                  <span>Tile {hover.tile.x},{hover.tile.y} • World {hover.world?.x.toFixed(1)},{hover.world?.y.toFixed(1)}</span>
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

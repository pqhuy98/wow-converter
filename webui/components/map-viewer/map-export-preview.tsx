'use client';

import {
  Check, Expand, LoaderCircle, Mouse, Shrink, X,
} from 'lucide-react';
import {
  useEffect, useMemo, useRef, useState,
} from 'react';

import { TooltipHelp } from '@/components/common/tooltip-help';
import { Button } from '@/components/ui/button';
import type { GenerateWc3FormValues } from '@/lib/models/map-generate.model';
import type { MapPreviewData } from '@/lib/models/map-preview.model';
import { previewMapSize, previewNPCPlacement, previewWaterLevels } from '@/lib/models/map-preview.model';

import type { MapPreviewHit } from './map-export-preview-viewer';
import { MapExportPreviewViewer } from './map-export-preview-viewer';

interface Props {
  data: MapPreviewData | null;
  values: GenerateWc3FormValues;
  loading: boolean;
  progress: string;
  error: string;
  isFullscreen: boolean;
  onToggleFullscreen: () => void;
}

export function MapExportPreview({
  data, values, loading, progress, error, isFullscreen, onToggleFullscreen,
}: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const viewerRef = useRef<MapExportPreviewViewer | null>(null);
  const hoverFrame = useRef(0);
  const [hover, setHover] = useState<{ x: number; y: number; hit: MapPreviewHit } | null>(null);
  const [viewerError, setViewerError] = useState('');
  const [loadingAssets, setLoadingAssets] = useState(false);
  const [assetProgress, setAssetProgress] = useState('Loading preview assets — 0/0');
  const counts = useMemo(() => {
    const result = { unit: 0, destructible: 0, excluded: 0 };
    for (const npc of data?.npcs ?? []) {
      if (data) result[previewNPCPlacement(npc, data, values)]++;
    }
    return result;
  }, [data, values]);
  const water = useMemo(() => data && previewWaterLevels(data, values), [data, values]);
  const mapSize = data && previewMapSize(data, values);

  const clearHover = () => {
    if (hoverFrame.current) cancelAnimationFrame(hoverFrame.current);
    hoverFrame.current = 0;
    setHover(null);
  };

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return undefined;
    try {
      const viewer = new MapExportPreviewViewer(canvas);
      viewerRef.current = viewer;
      return () => {
        if (hoverFrame.current) cancelAnimationFrame(hoverFrame.current);
        viewerRef.current = null;
        viewer.dispose();
      };
    } catch (err) {
      setViewerError(err instanceof Error ? err.message : 'Could not initialize the map preview.');
      return undefined;
    }
  }, []);

  useEffect(() => {
    viewerRef.current?.setOptions(values);
    if (hoverFrame.current) cancelAnimationFrame(hoverFrame.current);
    hoverFrame.current = 0;
    setHover(null);
  }, [values]);
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer) return undefined;
    let cancelled = false;
    if (hoverFrame.current) cancelAnimationFrame(hoverFrame.current);
    hoverFrame.current = 0;
    setHover(null); setViewerError(''); setLoadingAssets(Boolean(data));
    void viewer.setScene(data, (step) => {
      if (!cancelled) setAssetProgress(`${step.message} — ${step.completed}/${step.total}`);
    }).catch((err: unknown) => {
      if (!cancelled) setViewerError(err instanceof Error ? err.message : 'Could not load the map preview.');
    }).finally(() => { if (!cancelled) setLoadingAssets(false); });
    return () => { cancelled = true; };
  }, [data]);

  return (
    <section className="relative min-h-[360px] overflow-hidden border-t bg-[#0e1318] lg:min-h-0 lg:border-l lg:border-t-0" aria-label="Map export preview">
      <canvas
        ref={canvasRef}
        className="absolute inset-0 h-full w-full touch-none"
        aria-label="Interactive terrain, water and NPC preview"
        onPointerLeave={clearHover}
        onPointerDown={clearHover}
        onWheel={clearHover}
        onPointerMove={(event) => {
          if (event.buttons || loading || loadingAssets || error || viewerError) { clearHover(); return; }
          const rect = event.currentTarget.getBoundingClientRect();
          const x = event.clientX - rect.left; const y = event.clientY - rect.top;
          if (hoverFrame.current) cancelAnimationFrame(hoverFrame.current);
          hoverFrame.current = requestAnimationFrame(() => {
            hoverFrame.current = 0;
            const hit = viewerRef.current?.pick(x, y);
            setHover(hit ? { x: Math.min(x + 16, Math.max(8, rect.width - 264)), y: Math.min(y + 16, Math.max(8, rect.height - 72)), hit } : null);
          });
        }}
      />
      <div className="pointer-events-none absolute left-4 right-24 top-4 w-fit max-w-[calc(100%-7rem)] rounded-md bg-black/75 px-4 py-3 text-base leading-6 text-slate-100">
        <div className="flex flex-wrap gap-x-4 gap-y-1 font-semibold">
          <span>Adjust terrain clamp to select playable areas.</span>
          {mapSize && <span className="tabular-nums">WC3 map size: {mapSize.width} x {mapSize.height}</span>}
        </div>
        {data && (
          <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 tabular-nums">
            <span className="text-green-400">● {counts.unit.toLocaleString()} NPCs as units</span>
            <span className="text-yellow-300">● {counts.destructible.toLocaleString()} NPCs as destructibles</span>
            <span className="text-red-400">● {counts.excluded.toLocaleString()} NPCs excluded</span>
          </div>
        )}
        {water?.clipped && <div className="mt-2 max-w-xl text-amber-300" role="status">Some water is outside the usable range. Adjust the clamp to include it.</div>}
        {data?.warnings.length ? <div className="mt-2 max-w-xl text-amber-300">{[...new Set(data.warnings)].join('. ')}</div> : null}
      </div>
      <Button
        type="button"
        size="icon"
        variant="secondary"
        className="absolute right-12 top-3"
        onClick={onToggleFullscreen}
        aria-label={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
        aria-pressed={isFullscreen}
        title={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
      >
        {isFullscreen ? <Shrink className="h-4 w-4" /> : <Expand className="h-4 w-4" />}
      </Button>
      {(loading || loadingAssets) && !error && !viewerError && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center gap-2 bg-black/30 text-base text-white" role="status">
          <LoaderCircle className="h-5 w-5 animate-spin" /> {loading ? progress : assetProgress}
        </div>
      )}
      {(error || viewerError) && <div className="absolute left-4 right-4 top-24 rounded-md bg-background/95 p-3 text-base text-destructive" role="alert">{error || viewerError}</div>}
      <div className="absolute bottom-2 left-2 z-10">
        <TooltipHelp
          trigger={<button type="button" aria-label="Camera controls" className="inline-flex"><Mouse className="h-6 w-6 text-slate-100 drop-shadow" /></button>}
          tooltips={(
            <div className="text-base leading-6">
              <div><span className="font-semibold">Left drag</span>: orbit</div>
              <div><span className="font-semibold">Right drag</span>: pan ground</div>
              <div><span className="font-semibold">Middle or left+right drag</span>: pan screen</div>
              <div><span className="font-semibold">Wheel</span>: zoom</div>
            </div>
          )}
        />
      </div>
      {hover && (
        <div role="tooltip" className="pointer-events-none absolute z-10 max-w-64 break-words rounded-md bg-black/90 px-3 py-2 text-base text-white shadow-lg" style={{ left: hover.x, top: hover.y }}>
          {hover.hit.kind === 'npc' ? (
            <>
              <span className="text-white">NPC:</span>{' '}
              <span className={{ unit: 'text-green-400', destructible: 'text-yellow-300', excluded: 'text-red-400' }[hover.hit.placement]}>{hover.hit.name}</span>
            </>
          ) : hover.hit.playable ? (
            <span className="flex items-center gap-2 text-white"><Check className="h-4 w-4 shrink-0 text-green-400" aria-hidden="true" />Playable terrain</span>
          ) : (
            <span className="flex items-center gap-2 text-gray-400"><X className="h-4 w-4 shrink-0 text-gray-600" aria-hidden="true" />Unreachable terrain</span>
          )}
        </div>
      )}
    </section>
  );
}

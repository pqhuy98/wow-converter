import MdxModelInstance from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/modelinstance';
import {
  useCallback, useEffect, useMemo, useRef, useState,
} from 'react';

type OrthoPlane = 'xy' | 'xz';
type CutBoxSide = 'minX' | 'maxX' | 'minY' | 'maxY' | 'minZ' | 'maxZ';

export type CutBox = Readonly<{
  minX: number
  minY: number
  minZ: number
  maxX: number
  maxY: number
  maxZ: number
}>;

export function CutBoxOrthoView(props: Readonly<{
  title: string
  enabled: boolean
  box: CutBox
  modelBounds?: CutBox
  plane: OrthoPlane
  onChangeBox: (next: CutBox) => void
}>) {
  const {
    title, enabled, box, modelBounds, plane, onChangeBox,
  } = props;

  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [canvasSize, setCanvasSize] = useState<Readonly<{ w: number; h: number }>>({ w: 280, h: 170 });
  const [hoveredSide, setHoveredSide] = useState<CutBoxSide | null>(null);
  const draggingRef = useRef<{
    side: CutBoxSide
  } | null>(null);
  const panningRef = useRef<{
    startX: number
    startY: number
    startPanX: number
    startPanY: number
  } | null>(null);
  const [zoom, setZoom] = useState<number>(1);
  const [viewSpan, setViewSpan] = useState<number | null>(null);
  const [viewCenter, setViewCenter] = useState<Readonly<{ x: number; y: number }> | null>(null);
  const movingBoxRef = useRef<{
    startX: number
    startY: number
    startBox: CutBox
  } | null>(null);

  // Keep canvas sized to container for crisp hit-testing.
  useEffect(() => {
    const el = containerRef.current;
    const canvas = canvasRef.current;
    if (!el || !canvas) return undefined;
    const ro = new ResizeObserver(() => {
      const rect = el.getBoundingClientRect();
      const w = Math.max(1, Math.floor(rect.width));
      const h = Math.max(1, Math.floor(rect.height));
      canvas.width = w;
      canvas.height = h;
      setCanvasSize({ w, h });
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Initialize view span once (no auto-fit as the box grows/shrinks).
  useEffect(() => {
    if (!enabled) return;
    if (viewSpan != null) return;
    const dx = Math.max(0.001, box.maxX - box.minX);
    const dy = plane === 'xy'
      ? Math.max(0.001, box.maxY - box.minY)
      : Math.max(0.001, box.maxZ - box.minZ);
    // Pad a bit so the initial view isn't too tight.
    setViewSpan(Math.max(dx, dy) * 1.3);
    setZoom(1);
  }, [enabled, box, plane, viewSpan]);

  // Initialize view center once (do not auto-recenter as the box moves).
  useEffect(() => {
    if (!enabled) return;
    if (viewCenter != null) return;
    const cx = (box.minX + box.maxX) / 2;
    const cy = plane === 'xy'
      ? (box.minY + box.maxY) / 2
      : (box.minZ + box.maxZ) / 2;
    setViewCenter({ x: cx, y: cy });
  }, [enabled, box, plane, viewCenter]);

  // If disabled, allow re-init next time it is enabled.
  useEffect(() => {
    if (enabled) return;
    setViewSpan(null);
    setViewCenter(null);
    panningRef.current = null;
    draggingRef.current = null;
    setHoveredSide(null);
  }, [enabled]);

  const baseScale = useMemo(() => {
    // Stable world-to-screen scale: based on initial view span, not current box size.
    const w = Math.max(1, canvasSize.w);
    const h = Math.max(1, canvasSize.h);
    const span = Math.max(0.001, viewSpan ?? 1);
    const s = Math.min(w, h) / span;
    return s;
  }, [canvasSize, viewSpan]);

  const getCenterWorld = (): { cx: number; cy: number } => {
    const c = viewCenter ?? { x: 0, y: 0 };
    return { cx: c.x, cy: c.y };
  };

  const worldToScreen = (wx: number, wy: number): { sx: number; sy: number } => {
    const canvas = canvasRef.current;
    const w = canvas?.width ?? 1;
    const h = canvas?.height ?? 1;
    const { cx, cy } = getCenterWorld();
    const s = baseScale * zoom;
    if (plane === 'xy') {
      // Rotated top view: Y goes right, X goes down.
      const sx = (w / 2) + (wy - cy) * s;
      const sy = (h / 2) + (wx - cx) * s;
      return { sx, sy };
    }
    const sx = (w / 2) + (wx - cx) * s;
    const sy = (h / 2) - (wy - cy) * s;
    return { sx, sy };
  };

  const screenToWorld = (sx: number, sy: number): { wx: number; wy: number } => {
    const canvas = canvasRef.current;
    const w = canvas?.width ?? 1;
    const h = canvas?.height ?? 1;
    const { cx, cy } = getCenterWorld();
    const s = baseScale * zoom;
    if (plane === 'xy') {
      // Inverse of rotated top view: Y right, X down.
      const wy = cy + (sx - w / 2) / s;
      const wx = cx + (sy - h / 2) / s;
      return { wx, wy };
    }
    const wx = cx + (sx - w / 2) / s;
    const wy = cy - (sy - h / 2) / s;
    return { wx, wy };
  };

  const hitTestSide = (sx: number, sy: number): CutBoxSide | null => {
    const thresholdPx = 8;
    const x0 = box.minX;
    const x1 = box.maxX;
    const y0 = plane === 'xy' ? box.minY : box.minZ;
    const y1 = plane === 'xy' ? box.maxY : box.maxZ;
    const p00 = worldToScreen(x0, y0);
    const p11 = worldToScreen(x1, y1);
    const left = Math.min(p00.sx, p11.sx);
    const right = Math.max(p00.sx, p11.sx);
    const top = Math.min(p00.sy, p11.sy);
    const bottom = Math.max(p00.sy, p11.sy);

    const withinY = sy >= top - thresholdPx && sy <= bottom + thresholdPx;
    const withinX = sx >= left - thresholdPx && sx <= right + thresholdPx;
    const dLeft = Math.abs(sx - left);
    const dRight = Math.abs(sx - right);
    const dTop = Math.abs(sy - top);
    const dBottom = Math.abs(sy - bottom);

    let bestSide: CutBoxSide | null = null;
    let bestD = Number.POSITIVE_INFINITY;
    const consider = (side: CutBoxSide, d: number, ok: boolean) => {
      if (!ok || d > thresholdPx) return;
      if (d < bestD) {
        bestD = d;
        bestSide = side;
      }
    };
    if (plane === 'xy') {
      // Rotated top view: horizontal axis is Y, vertical axis is X.
      // Left/right edges correspond to minY/maxY.
      consider('minY', dLeft, withinY);
      consider('maxY', dRight, withinY);
      // Top/bottom edges correspond to minX/maxX (X increases downward).
      consider('minX', dTop, withinX);
      consider('maxX', dBottom, withinX);
    } else {
      // Side view (X/Z): horizontal axis is X, vertical axis is Z.
      consider('minX', dLeft, withinY);
      consider('maxX', dRight, withinY);
      consider('maxZ', dTop, withinX);
      consider('minZ', dBottom, withinX);
    }
    return bestSide;
  };

  const isInsideRect = (sx: number, sy: number): boolean => {
    const x0 = box.minX;
    const x1 = box.maxX;
    const y0 = plane === 'xy' ? box.minY : box.minZ;
    const y1 = plane === 'xy' ? box.maxY : box.maxZ;
    const p00 = worldToScreen(x0, y0);
    const p11 = worldToScreen(x1, y1);
    const left = Math.min(p00.sx, p11.sx);
    const right = Math.max(p00.sx, p11.sx);
    const top = Math.min(p00.sy, p11.sy);
    const bottom = Math.max(p00.sy, p11.sy);
    return sx >= left && sx <= right && sy >= top && sy <= bottom;
  };

  const draw = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const w = canvas.width;
    const h = canvas.height;
    ctx.clearRect(0, 0, w, h);

    // Background
    ctx.fillStyle = 'rgba(0,0,0,0.22)';
    ctx.fillRect(0, 0, w, h);

    // World axis lines (origin) for orientation.
    // X = 0 (vertical), Y/Z = 0 (horizontal)
    {
      // In rotated XY view, x=0 is horizontal and y=0 is vertical.
      const originX = plane === 'xy'
        ? worldToScreen(getCenterWorld().cx, 0).sx
        : worldToScreen(0, getCenterWorld().cy).sx;
      const originY = plane === 'xy'
        ? worldToScreen(0, getCenterWorld().cy).sy
        : worldToScreen(getCenterWorld().cx, 0).sy;
      ctx.save();
      ctx.setLineDash([6, 6]);
      ctx.lineWidth = 1;
      ctx.strokeStyle = 'rgba(255,255,255,0.22)';
      // vertical line
      ctx.beginPath();
      ctx.moveTo(originX, 0);
      ctx.lineTo(originX, h);
      ctx.stroke();
      // horizontal line
      ctx.beginPath();
      ctx.moveTo(0, originY);
      ctx.lineTo(w, originY);
      ctx.stroke();
      ctx.restore();
    }

    // Projected model bounds (read-only dashed rectangle)
    if (modelBounds) {
      const bx0 = modelBounds.minX;
      const bx1 = modelBounds.maxX;
      const by0 = plane === 'xy' ? modelBounds.minY : modelBounds.minZ;
      const by1 = plane === 'xy' ? modelBounds.maxY : modelBounds.maxZ;
      const b00 = worldToScreen(bx0, by0);
      const b11 = worldToScreen(bx1, by1);
      const bLeft = Math.min(b00.sx, b11.sx);
      const bRight = Math.max(b00.sx, b11.sx);
      const bTop = Math.min(b00.sy, b11.sy);
      const bBottom = Math.max(b00.sy, b11.sy);
      ctx.save();
      ctx.setLineDash([4, 5]);
      ctx.lineWidth = 1.5;
      ctx.strokeStyle = 'rgba(200, 210, 255, 0.35)';
      ctx.strokeRect(bLeft, bTop, bRight - bLeft, bBottom - bTop);
      ctx.restore();
    }

    // Rectangle
    const x0 = box.minX;
    const x1 = box.maxX;
    const y0 = plane === 'xy' ? box.minY : box.minZ;
    const y1 = plane === 'xy' ? box.maxY : box.maxZ;
    const p00 = worldToScreen(x0, y0);
    const p11 = worldToScreen(x1, y1);
    const left = Math.min(p00.sx, p11.sx);
    const right = Math.max(p00.sx, p11.sx);
    const top = Math.min(p00.sy, p11.sy);
    const bottom = Math.max(p00.sy, p11.sy);

    // Sides (with hover highlight)
    const baseColor = 'rgba(210, 220, 255, 0.85)';
    const hoverColor = 'rgba(255, 220, 60, 0.98)'; // yellow highlight
    const sideStroke = (side: CutBoxSide) => (hoveredSide === side ? hoverColor : baseColor);

    ctx.lineWidth = 2;
    ctx.strokeStyle = baseColor;
    ctx.strokeRect(left, top, right - left, bottom - top);

    // Left
    ctx.strokeStyle = sideStroke(plane === 'xy' ? 'minY' : 'minX');
    ctx.beginPath();
    ctx.moveTo(left, top);
    ctx.lineTo(left, bottom);
    ctx.stroke();

    // Right
    ctx.strokeStyle = sideStroke(plane === 'xy' ? 'maxY' : 'maxX');
    ctx.beginPath();
    ctx.moveTo(right, top);
    ctx.lineTo(right, bottom);
    ctx.stroke();

    // Top/Bottom depend on plane
    if (plane === 'xy') {
      // Top is minX (X goes down), bottom is maxX.
      ctx.strokeStyle = sideStroke('minX');
      ctx.beginPath();
      ctx.moveTo(left, top);
      ctx.lineTo(right, top);
      ctx.stroke();

      ctx.strokeStyle = sideStroke('maxX');
      ctx.beginPath();
      ctx.moveTo(left, bottom);
      ctx.lineTo(right, bottom);
      ctx.stroke();
    } else {
      ctx.strokeStyle = sideStroke('maxZ');
      ctx.beginPath();
      ctx.moveTo(left, top);
      ctx.lineTo(right, top);
      ctx.stroke();

      ctx.strokeStyle = sideStroke('minZ');
      ctx.beginPath();
      ctx.moveTo(left, bottom);
      ctx.lineTo(right, bottom);
      ctx.stroke();
    }

    // Small label in-canvas for orientation
    ctx.fillStyle = 'rgba(255,255,255,0.75)';
    ctx.font = '12px ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace';
    ctx.fillText(plane === 'xy' ? 'Y →, X ↓' : 'X →, Z ↑', 10, 18);
  }, [baseScale, box, hoveredSide, modelBounds, plane, viewCenter, zoom]);

  useEffect(() => {
    draw();
  }, [draw]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return undefined;

    const onWheel = (e: WheelEvent) => {
      if (!enabled) return;
      e.preventDefault();
      const delta = e.deltaY;
      const factor = delta > 0 ? 0.9 : 1.1;
      // No max zoom limit; keep only a tiny minimum to avoid division by zero.
      setZoom((z) => Math.max(1e-9, z * factor));
    };
    canvas.addEventListener('wheel', onWheel, { passive: false });
    return () => canvas.removeEventListener('wheel', onWheel);
  }, [enabled]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return undefined;

    const onPointerMove = (e: PointerEvent) => {
      const rect = canvas.getBoundingClientRect();
      const sx = e.clientX - rect.left;
      const sy = e.clientY - rect.top;
      if (!enabled) {
        setHoveredSide(null);
        return;
      }
      if (panningRef.current) {
        e.preventDefault();
        const panState = panningRef.current;
        const dxPx = sx - panState.startX;
        const dyPx = sy - panState.startY;
        const s = baseScale * zoom;
        // Dragging moves the view; keep content moving with the cursor ("grab to pan").
        if (plane === 'xy') {
          // Rotated mapping: screen dx pans world Y, screen dy pans world X.
          setViewCenter({
            x: panState.startPanX + (-dyPx / s),
            y: panState.startPanY + (-dxPx / s),
          });
        } else {
          const dxWorld = -dxPx / s;
          const dyWorld = dyPx / s;
          setViewCenter({
            x: panState.startPanX + dxWorld,
            y: panState.startPanY + dyWorld,
          });
        }
        return;
      }
      if (movingBoxRef.current) {
        e.preventDefault();
        const moveState = movingBoxRef.current;
        const dxPx = sx - moveState.startX;
        const dyPx = sy - moveState.startY;
        const s = baseScale * zoom;
        // Move the rectangle in the same on-screen direction as the cursor drag.
        const dxWorld = plane === 'xy' ? (dyPx / s) : (dxPx / s);
        const dyWorld = plane === 'xy' ? (dxPx / s) : (-dyPx / s);
        const startBox = moveState.startBox;
        const next: CutBox = plane === 'xy'
          ? {
            ...startBox,
            minX: startBox.minX + dxWorld,
            maxX: startBox.maxX + dxWorld,
            minY: startBox.minY + dyWorld,
            maxY: startBox.maxY + dyWorld,
          }
          : {
            ...startBox,
            minX: startBox.minX + dxWorld,
            maxX: startBox.maxX + dxWorld,
            minZ: startBox.minZ + dyWorld,
            maxZ: startBox.maxZ + dyWorld,
          };
        onChangeBox(next);
        return;
      }
      if (draggingRef.current) {
        const { wx, wy } = screenToWorld(sx, sy);
        const eps = 0.001;
        onChangeBox(normalizeCutBox(applyDrag(box, plane, draggingRef.current.side, wx, wy, eps)));
      } else {
        setHoveredSide(hitTestSide(sx, sy));
      }
    };

    const onPointerDown = (e: PointerEvent) => {
      if (!enabled) return;
      e.preventDefault();
      const rect = canvas.getBoundingClientRect();
      const sx = e.clientX - rect.left;
      const sy = e.clientY - rect.top;

      // Middle mouse drag = pan the ortho view.
      if (e.button === 1) {
        const c = viewCenter ?? { x: 0, y: 0 };
        panningRef.current = {
          startX: sx,
          startY: sy,
          startPanX: c.x,
          startPanY: c.y,
        };
        setHoveredSide(null);
        try {
          canvas.setPointerCapture(e.pointerId);
        } catch {
          // ignore
        }
        return;
      }

      const side = hitTestSide(sx, sy);
      if (!side) {
        // If clicking inside the rectangle (but not on edges), drag moves the whole box.
        if (e.button === 0 && isInsideRect(sx, sy)) {
          movingBoxRef.current = { startX: sx, startY: sy, startBox: box };
          setHoveredSide(null);
          try {
            canvas.setPointerCapture(e.pointerId);
          } catch {
            // ignore
          }
        }
        return;
      }
      draggingRef.current = { side };
      setHoveredSide(side);
      try {
        canvas.setPointerCapture(e.pointerId);
      } catch {
        // ignore
      }
    };

    const onPointerUp = (e: PointerEvent) => {
      if (!enabled) return;
      draggingRef.current = null;
      panningRef.current = null;
      movingBoxRef.current = null;
      try {
        canvas.releasePointerCapture(e.pointerId);
      } catch {
        // ignore
      }
    };

    canvas.addEventListener('pointermove', onPointerMove);
    canvas.addEventListener('pointerdown', onPointerDown);
    canvas.addEventListener('pointerup', onPointerUp);
    canvas.addEventListener('pointercancel', onPointerUp);
    return () => {
      canvas.removeEventListener('pointermove', onPointerMove);
      canvas.removeEventListener('pointerdown', onPointerDown);
      canvas.removeEventListener('pointerup', onPointerUp);
      canvas.removeEventListener('pointercancel', onPointerUp);
    };
  }, [enabled, box, plane, onChangeBox, baseScale, viewCenter, zoom]);

  return (
    <div className={`rounded border border-[hsl(var(--viewer-divider))] ${enabled ? 'opacity-100' : 'opacity-60'} flex flex-col h-full min-h-0`}>
      <div className="px-3 py-2 text-sm font-semibold border-b border-[hsl(var(--viewer-divider))] bg-[hsl(var(--viewer-control-bg))]/40 flex-shrink-0">
        {title}
      </div>
      <div ref={containerRef} className="flex-1 min-h-0 w-full relative">
        <canvas
          ref={canvasRef}
          width={280}
          height={170}
          className="absolute inset-0 w-full h-full"
          style={{ touchAction: 'none' }}
        />
      </div>
    </div>
  );
}

function applyDrag(box: CutBox, plane: OrthoPlane, side: CutBoxSide, wx: number, wy: number, eps: number): CutBox {
  if (side === 'minX') return { ...box, minX: Math.min(wx, box.maxX - eps) };
  if (side === 'maxX') return { ...box, maxX: Math.max(wx, box.minX + eps) };
  if (plane === 'xy') {
    if (side === 'minY') return { ...box, minY: Math.min(wy, box.maxY - eps) };
    if (side === 'maxY') return { ...box, maxY: Math.max(wy, box.minY + eps) };
  } else {
    if (side === 'minZ') return { ...box, minZ: Math.min(wy, box.maxZ - eps) };
    if (side === 'maxZ') return { ...box, maxZ: Math.max(wy, box.minZ + eps) };
  }
  return box;
}

export function cutBoxFromInstanceBounds(inst: MdxModelInstance): CutBox {
  const b = inst.getBounds();
  const r = Math.max(0.001, b.r);
  return normalizeCutBox({
    minX: b.x - r,
    minY: b.y - r,
    minZ: b.z - r,
    maxX: b.x + r,
    maxY: b.y + r,
    maxZ: b.z + r,
  });
}

function normalizeCutBox(box: CutBox): CutBox {
  const eps = 0.001;
  const minX = Math.min(box.minX, box.maxX - eps);
  const maxX = Math.max(box.maxX, box.minX + eps);
  const minY = Math.min(box.minY, box.maxY - eps);
  const maxY = Math.max(box.maxY, box.minY + eps);
  const minZ = Math.min(box.minZ, box.maxZ - eps);
  const maxZ = Math.max(box.maxZ, box.minZ + eps);
  return {
    minX,
    minY,
    minZ,
    maxX,
    maxY,
    maxZ,
  };
}

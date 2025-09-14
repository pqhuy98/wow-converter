'use client';

import { downloadAssetsZip } from '@api/download';
import mdlx from '@pqhuy98/mdx-m3-viewer/dist/cjs/utils/mdlx';
import Camera from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/camera';
import blpHandler from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/blp/handler';
import mdxHandler from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/handler';
import MdxModel from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/model';
import MdxModelInstance from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/modelinstance';
import Sequence from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/sequence';
import Scene from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/scene';
import ModelViewer from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/viewer';
import { vec3 } from 'gl-matrix';
import { Download, Mouse } from 'lucide-react';
import {
  useEffect, useRef, useState,
} from 'react';

import { Button } from '@/components/ui/button';

import { TooltipHelp } from './tooltip-help';

interface ModelViewerProps {
  modelPath?: string
  alwaysFullscreen?: boolean
  assetsBase?: string
}

// Normalises backslashes to forward slashes for safe URL usage
const normalizePath = (p: string) => p.replace(/\\+/g, '/').replace(/\/+/, '/');

const MAX_DISTANCE = 2000000;

export default function ModelViewerUi({ modelPath, alwaysFullscreen, assetsBase }: ModelViewerProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [sequences, setSequences] = useState<Sequence[]>([]);
  const [currentSeq, setCurrentSeq] = useState<number>(0);
  const instanceRef = useRef<MdxModelInstance | null>(null);
  const vecHeap = vec3.create();

  const [viewer, setViewer] = useState<ModelViewer | null>(null);
  const [scene, setScene] = useState<Scene | null>(null);
  const [camera, setCamera] = useState<Camera | null>(null);
  // Collision toggling and primitive refs
  const [collisionsVisible, setCollisionsVisible] = useState(false);
  const collisionInstancesRef = useRef<MdxModelInstance[]>([]);
  const boxModelRef = useRef<MdxModel | undefined>(undefined);
  const sphereModelRef = useRef<MdxModel | undefined>(undefined);
  const primsReadyRef = useRef<Promise<void> | undefined>(undefined);
  const loadRequestIdRef = useRef(0);
  const [gridVisible, setGridVisible] = useState(true);
  const gridInstancesRef = useRef<MdxModelInstance[]>([]);
  // Track loaded asset files for download
  const baseUrlRef = useRef<string>('/api/assets');
  const loadedFilesRef = useRef<Set<string>>(new Set());
  const [loadedCount, setLoadedCount] = useState<number>(0);
  useEffect(() => {
    baseUrlRef.current = assetsBase || '/api/assets';
  }, [assetsBase]);
  useEffect(() => {
    if (!canvasRef.current) return undefined;
    const viewer = new ModelViewer(canvasRef.current);
    viewer.addHandler(mdxHandler);
    viewer.addHandler(blpHandler);
    setViewer(viewer);

    const scene = viewer.addScene();
    setScene(scene);
    scene.color.fill(0.15);
    void (async () => {
      const gridInsts = await createGridModel(viewer, scene, 50, 128);
      gridInstancesRef.current = gridInsts;
    })();

    const camera = scene.camera;
    setCamera(camera);
    // Prepare primitive models for collision visualization
    primsReadyRef.current = (async () => {
      try {
        const [boxM, sphereM] = await Promise.all([
          mdlx.createPrimitive(viewer, mdlx.primitives.createUnitCube(), { lines: true }),
          mdlx.createPrimitive(viewer, mdlx.primitives.createUnitSphere(12, 12), { lines: true }),
        ]);
        boxModelRef.current = boxM;
        sphereModelRef.current = sphereM;
      } catch (e) {
        // Fallbacks handled above; ignore
      }
    })();

    viewer.on('loadstart', (e) => {
      const fetchUrl = String(e.fetchUrl ?? '');
      console.log(`[Viewer] Loading ${fetchUrl}`);
      // Record files that are fetched from our assets endpoints
      const bases = ['/api/assets', '/api/browse-assets', baseUrlRef.current].filter(Boolean);
      for (const b of bases) {
        const base = String(b);
        if (fetchUrl.startsWith(`${base}/`)) {
          const rel = fetchUrl.slice((`${base}/`).length).replace(/^\/+/, '');
          if (rel) {
            loadedFilesRef.current.add(rel);
            setLoadedCount(loadedFilesRef.current.size);
          }
          break;
        }
      }
    });

    viewer.on('loadend', (e) => {
      console.log(`[Viewer] Loaded ${e.fetchUrl}`);
    });

    viewer.on('error', (e) => {
      if (e.fetchUrl) {
        console.error(`[Viewer] ${e.error}: ${e.fetchUrl}`);
      } else {
        console.error(`[Viewer] ${e.error}: ${e.reason}`);
      }
    });

    let animationFrameId: number;
    let lastTime = performance.now();
    const step = () => {
      const now = performance.now();
      const dt = now - lastTime;
      lastTime = now;
      viewer.updateAndRender(dt);
      animationFrameId = requestAnimationFrame(step);
    };
    step();
    canvasRef.current.addEventListener('contextmenu', (e) => e.preventDefault());
    return () => {
      if (animationFrameId) cancelAnimationFrame(animationFrameId);
    };
  }, [canvasRef.current]);

  console.log('Model path:', modelPath);

  useEffect(() => {
    if (!modelPath || !canvasRef.current || !viewer || !scene || !camera) return undefined;

    // references for cleanup
    const canvas = canvasRef.current;
    // reset loaded asset tracker on new load
    loadedFilesRef.current = new Set();
    setLoadedCount(0);
    const requestId = ++loadRequestIdRef.current;
    let cancelled = false;
    let onMouseDown: ((e: MouseEvent) => void) | null = null;
    let onMouseMove: ((e: MouseEvent) => void) | null = null;
    let onMouseUp: ((e: MouseEvent) => void) | null = null;
    let onWheel: ((e: WheelEvent) => void) | null = null;
    let resizeCanvas: (() => void) | null = null;
    let onTouchStart: ((e: TouchEvent) => void) | null = null;
    let onTouchMove: ((e: TouchEvent) => void) | null = null;
    let onTouchEnd: ((e: TouchEvent) => void) | null = null;

    let modelInstance: MdxModelInstance | null = null;

    void (async () => {
      // Ensure collision primitives are ready before loading shapes
      if (primsReadyRef.current) {
        try { await primsReadyRef.current; } catch {
          // ignore
        }
      }
      // Path solver so the viewer fetches every dependant file via our assets route
      const base = assetsBase || '/api/assets';
      const pathSolver = (src: unknown) => `${base}/${normalizePath(src as string)}`;

      // Load the model (assumed to be in MDX|MDL format)
      const model = await viewer.load(`${normalizePath(modelPath)}`, pathSolver);
      if (cancelled || loadRequestIdRef.current !== requestId) return;
      if (!(model instanceof MdxModel)) return;
      modelInstance = model.addInstance();

      if (cancelled || loadRequestIdRef.current !== requestId) return;
      instanceRef.current = modelInstance;
      modelInstance.setSequence(0);
      modelInstance.sequenceLoopMode = 2; // always loop
      setSequences(model.sequences);
      setCurrentSeq(0);

      // Add scene and basic camera, grid setup
      scene.addInstance(modelInstance);

      // Build collision shape instances (hidden by default)
      collisionInstancesRef.current = [];
      const boxModel = boxModelRef.current;
      const sphereModel = sphereModelRef.current;
      try {
        const shapes = model.collisionShapes || [];
        for (const shape of shapes) {
          // Shape types: 0=Box, 1=Cylinder, 2=Sphere (per mdx-m3-viewer)
          if (shape.type === 0 && boxModel) {
            const inst = boxModel.addInstance();
            const [min, max] = shape.vertices;
            const x = (max[0] + min[0]) / 2;
            const y = (max[1] + min[1]) / 2;
            const z = (max[2] + min[2]) / 2;
            const w = (max[0] - min[0]) / 2;
            const d = (max[1] - min[1]) / 2;
            const h = (max[2] - min[2]) / 2;
            inst.setLocation([x, y, z]);
            inst.setScale([w, d, h]);
            inst.dontInheritScaling = false;
            inst.setParent(modelInstance.nodes[shape.index]);
            inst.setScene(scene);
            inst[collisionsVisible ? 'show' : 'hide']();
            collisionInstancesRef.current.push(inst);
          } else if (shape.type === 2 && sphereModel) {
            const inst = sphereModel.addInstance();
            inst.setLocation([0, 0, 0]);
            inst.uniformScale(shape.boundsRadius);
            inst.dontInheritScaling = false;
            inst.setParent(modelInstance.nodes[shape.index]);
            inst.setScene(scene);
            inst[collisionsVisible ? 'show' : 'hide']();
            collisionInstancesRef.current.push(inst);
          } else {
            console.log('COLLISION SHAPE NOT SUPPOTED', shape);
          }
        }
      } catch {
        // ignore
      }

      // Utility to update camera position from spherical coords
      let isDragging = false;
      let lastX = 0;
      let lastY = 0;
      let lastDistance = 0;
      let horizontalAngle = 0;
      let verticalAngle = Math.PI / 6;
      let distance = Math.max(200, Math.min(1000, modelInstance.getBounds().r * 5));
      const target = vec3.fromValues(0, 0, 0);
      target[0] = modelInstance.getBounds().x;
      target[1] = modelInstance.getBounds().y;

      const updateCamera = () => {
        const x = distance * Math.cos(verticalAngle) * Math.cos(horizontalAngle);
        const y = distance * Math.cos(verticalAngle) * Math.sin(horizontalAngle);
        const z = distance * Math.sin(verticalAngle);
        const camPos = vec3.fromValues(target[0] + x, target[1] + y, target[2] + z);
        camera.moveToAndFace(camPos, target, [0, 0, 1]);
      };
      updateCamera();

      // ensure canvas always matches element size
      resizeCanvas = () => {
        if (canvas) {
          const width = canvas.clientWidth;
          const height = canvas.clientHeight;
          canvas.width = width;
          canvas.height = height;
          scene.viewport[2] = width;
          scene.viewport[3] = height;
          camera.perspective(
            camera.fov,
            width / height,
            camera.nearClipPlane,
            9999999,
          );
        }
      };
      resizeCanvas();

      // Mouse & wheel controls
      let isTouch = false;
      let leftDown = false;
      let middleDown = false;
      let rightDown = false;

      onMouseDown = (e: MouseEvent) => {
        e.preventDefault();
        if (e.button === 0) leftDown = true;
        if (e.button === 1) middleDown = true;
        if (e.button === 2) rightDown = true;
        isDragging = leftDown || middleDown || rightDown;
        lastX = e.clientX;
        lastY = e.clientY;
      };

      onTouchStart = (e: TouchEvent) => {
        e.preventDefault();
        if (e.touches.length === 1) {
          isTouch = true;
          isDragging = true;
          lastX = e.touches[0].clientX;
          lastY = e.touches[0].clientY;
        } else if (e.touches.length === 2) {
          isTouch = true;
          isDragging = true;
          // Calculate initial distance between two fingers
          const dx = e.touches[0].clientX - e.touches[1].clientX;
          const dy = e.touches[0].clientY - e.touches[1].clientY;
          lastDistance = Math.sqrt(dx * dx + dy * dy);
        }
      };

      onMouseMove = (e: MouseEvent) => {
        if (!isDragging || isTouch) return;
        if (middleDown || (leftDown && rightDown)) { // pan with middle or L+R
          const dx = e.clientX - lastX;
          const dy = e.clientY - lastY;
          lastX = e.clientX;
          lastY = e.clientY;
          if (!canvas) return;
          const w = canvas.width;
          const h2 = canvas.height;
          const sw = -dx / w * distance;
          const sh = dy / h2 * distance;
          // Move target along camera right (directionX) and up (directionY)
          vec3.add(
            target,
            target,
            vec3.scale(
              vecHeap,
              vec3.normalize(vecHeap, vec3.set(vecHeap, camera.directionX[0], camera.directionX[1], camera.directionX[2])),
              sw,
            ),
          );
          vec3.add(
            target,
            target,
            vec3.scale(
              vecHeap,
              vec3.normalize(vecHeap, vec3.set(vecHeap, camera.directionY[0], camera.directionY[1], camera.directionY[2])),
              sh,
            ),
          );
        } else if (leftDown) { // left click to rotate
          const dx = e.clientX - lastX;
          const dy = e.clientY - lastY;
          lastX = e.clientX;
          lastY = e.clientY;
          const ROT_SPEED = Math.PI / 360; // radians per pixel
          horizontalAngle -= dx * ROT_SPEED;
          verticalAngle += dy * ROT_SPEED;
          verticalAngle = Math.max(-Math.PI / 2 + 0.01, Math.min(Math.PI / 2 - 0.01, verticalAngle));
        } else if (rightDown) { // right click to move target along ground plane
          const dx = e.clientX - lastX;
          const dy = e.clientY - lastY;
          lastX = e.clientX;
          lastY = e.clientY;

          const dirX = camera.directionX;
          const dirY = camera.directionY;
          if (!canvas) return;
          const w = canvas.width;
          const h2 = canvas.height;
          const sw = -dx / w * distance;
          const sh = dy / h2 * distance;
          vec3.add(
            target,
            target,
            vec3.scale(vecHeap, vec3.normalize(vecHeap, vec3.set(vecHeap, dirX[0], dirX[1], 0)), sw),
          );
          vec3.add(
            target,
            target,
            vec3.scale(vecHeap, vec3.normalize(vecHeap, vec3.set(vecHeap, dirY[0], dirY[1], 0)), sh),
          );
        }
        updateCamera();
      };

      onMouseUp = (e: MouseEvent) => {
        if (e.button === 0) leftDown = false;
        if (e.button === 1) middleDown = false;
        if (e.button === 2) rightDown = false;
        isDragging = leftDown || middleDown || rightDown;
      };

      onTouchMove = (e: TouchEvent) => {
        if (!isDragging || !isTouch) return;
        e.preventDefault();
        if (e.touches.length === 1) {
          const dx = e.touches[0].clientX - lastX;
          const dy = e.touches[0].clientY - lastY;
          lastX = e.touches[0].clientX;
          lastY = e.touches[0].clientY;
          const ROT_SPEED = Math.PI / 360; // radians per pixel
          horizontalAngle -= dx * ROT_SPEED;
          verticalAngle += dy * ROT_SPEED;
          verticalAngle = Math.max(-Math.PI / 2 + 0.01, Math.min(Math.PI / 2 - 0.01, verticalAngle));
          updateCamera();
        } else if (e.touches.length === 2) {
          // Pinch to zoom
          const dx = e.touches[0].clientX - e.touches[1].clientX;
          const dy = e.touches[0].clientY - e.touches[1].clientY;
          const currentDistance = Math.sqrt(dx * dx + dy * dy);

          if (lastDistance > 0) {
            const scale = currentDistance / lastDistance;
            const ZOOM_SPEED = -1;
            distance *= 1 + (scale - 1) * ZOOM_SPEED;
            distance = Math.max(1, Math.min(MAX_DISTANCE, distance));
            updateCamera();
          }

          lastDistance = currentDistance;
        }
      };

      onTouchEnd = (e: TouchEvent) => {
        e.preventDefault();
        isDragging = false;
        isTouch = false;
        lastDistance = 0; // Reset distance for next gesture
      };

      // endDrag replaced by onMouseUp per-button tracking

      onWheel = (e: WheelEvent) => {
        e.preventDefault();
        const delta = e.deltaY;
        const ZOOM_SPEED = 0.1;
        // adjust distance exponentially for smooth zoom
        distance *= 1 + (delta > 0 ? ZOOM_SPEED : -ZOOM_SPEED);
        distance = Math.max(1, Math.min(MAX_DISTANCE, distance));
        updateCamera();
      };

      if (canvas) {
        canvas.addEventListener('mousedown', onMouseDown);
        canvas.addEventListener('wheel', onWheel!, { passive: false });
        canvas.addEventListener('touchstart', onTouchStart, { passive: false });
        canvas.addEventListener('touchmove', onTouchMove, { passive: false });
        canvas.addEventListener('touchend', onTouchEnd, { passive: false });
      }

      window.addEventListener('resize', resizeCanvas);
      window.addEventListener('mousemove', onMouseMove!);
      window.addEventListener('mouseup', onMouseUp!);
    })();

    return () => {
      cancelled = true;
      if (canvas && onMouseDown && onWheel && onMouseMove && onMouseUp && resizeCanvas && onTouchStart && onTouchMove && onTouchEnd) {
        canvas.removeEventListener('mousedown', onMouseDown);
        canvas.removeEventListener('wheel', onWheel);
        canvas.removeEventListener('touchstart', onTouchStart);
        canvas.removeEventListener('touchmove', onTouchMove);
        canvas.removeEventListener('touchend', onTouchEnd);
        window.removeEventListener('mousemove', onMouseMove);
        window.removeEventListener('mouseup', onMouseUp!);
        window.removeEventListener('resize', resizeCanvas!);
      }
      if (modelInstance) {
        scene.removeInstance(modelInstance);
      }
      // Clean up collision visuals
      for (const inst of collisionInstancesRef.current) {
        try { inst.hide?.(); } catch {
          // ignore
        }
      }
      collisionInstancesRef.current = [];
      if (instanceRef.current === modelInstance) {
        instanceRef.current = null;
      }
    };
  }, [modelPath, canvasRef.current, viewer, scene]);

  const [progress, setProgress] = useState(0);

  // Apply sequence when currentSeq updates
  useEffect(() => {
    const inst = instanceRef.current;
    if (inst) {
      inst.setSequence(currentSeq);
      // 0 = loop based on model, 1 = never loop, 2 = always loop (see mdx impl)
      inst.sequenceLoopMode = 0;
      setProgress(0);
    }
  }, [currentSeq]);

  // Keep a live progress percentage for the active sequence
  useEffect(() => {
    let raf = 0;
    const tick = () => {
      const inst = instanceRef.current;
      const seq = sequences[currentSeq];
      if (inst && seq) {
        const start = seq.interval[0];
        const end = seq.interval[1];
        const length = Math.max(1, end - start);
        const local = Math.max(0, inst.frame - start);
        const modelLoops = seq.nonLooping === 0;
        const effectiveLoops = inst.sequenceLoopMode === 2 ? true : inst.sequenceLoopMode === 1 ? false : modelLoops;
        if (effectiveLoops) {
          setProgress((local % length) / length);
        } else {
          setProgress(Math.min(1, local / length));
        }
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [sequences, currentSeq]);

  const [isFullscreen, setIsFullscreen] = useState(alwaysFullscreen ?? false);
  const scrollPositionRef = useRef<{ x: number; y: number } | null>(null);
  const [copied, setCopied] = useState(false);

  const handleFullscreenToggle = () => {
    if (!isFullscreen) {
      // Entering fullscreen - save current scroll position
      scrollPositionRef.current = { x: window.scrollX, y: window.scrollY };
    }
    setIsFullscreen(!isFullscreen);
  };

  const handleCopyLink = async () => {
    const viewerUrl = `${window.location.origin}/viewer?model=${encodeURIComponent(modelPath || '')}`;
    try {
      await navigator.clipboard.writeText(viewerUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy link:', err);
    }
  };

  const handleDownloadAssets = async () => {
    try {
      const files = Array.from(loadedFilesRef.current);
      if (files.length === 0) return;
      const source = baseUrlRef.current.includes('/api/browse-assets') ? 'browse' : 'export';
      await downloadAssetsZip({ files, source });
    } catch (e) {
      console.error('Failed to download assets', e);
    }
  };

  const handleToggleCollisions = () => {
    const next = !collisionsVisible;
    setCollisionsVisible(next);
    for (const inst of collisionInstancesRef.current) {
      if (next) inst.show?.();
      else inst.hide?.();
    }
  };

  const handleToggleGrid = () => {
    const next = !gridVisible;
    setGridVisible(next);
    for (const inst of gridInstancesRef.current) {
      try {
        if (next) inst.show?.();
        else inst.hide?.();
      } catch { /* noop */ }
    }
  };

  useEffect(() => {
    // If exiting fullscreen, immediately resize canvas and restore scroll
    if (!isFullscreen && scrollPositionRef.current) {
      // Force canvas resize immediately using same method as original resizeCanvas
      if (canvasRef.current) {
        const canvas = canvasRef.current;
        const width = canvas.clientWidth;
        const height = canvas.clientHeight;
        canvas.width = width;
        canvas.height = height;
      }
      // Restore scroll position
      window.scrollTo(scrollPositionRef.current.x, scrollPositionRef.current.y);
      // This is a hack to fix a bug where the canvas is not resized when exiting fullscreen
      setTimeout(() => {
        window.dispatchEvent(new Event('resize'));
      }, 100);
    }

    // Force a resize event to update the viewer
    window.dispatchEvent(new Event('resize'));
  }, [isFullscreen]);

  // Handle escape key to exit fullscreen
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isFullscreen && !alwaysFullscreen) {
        handleFullscreenToggle();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isFullscreen, alwaysFullscreen]);

  const handleSelectSequence = (idx: number) => {
    const inst = instanceRef.current;
    if (inst && idx === currentSeq) {
      try {
        inst.sequenceEnded = false;
        inst.frame = 0;
        inst.counter = 0;
        inst.setSequence(idx);
        inst.sequenceLoopMode = 0;
        setProgress(0);
      } catch {
        // ignore
      }
    } else {
      setCurrentSeq(idx);
    }
  };

  return (
    <div className={`flex flex-col lg:flex-row w-full h-full ${alwaysFullscreen ? 'fixed inset-0 z-50' : isFullscreen ? 'fixed inset-0 z-50' : ''}`}>
      <div className={`relative flex-1 min-h-0 ${alwaysFullscreen || isFullscreen ? 'h-full' : 'h-full'}`}>
        <div className="absolute top-2 left-2 z-10 flex gap-2">
          {!alwaysFullscreen && (
            <Button
              variant="secondary"
              size="sm"
              onClick={handleFullscreenToggle}
              className="bg-gray-800 text-white border border-gray-600 hover:bg-gray-700 focus:outline-none focus:border-gray-600 active:border-gray-600 w-10 h-10 text-2xl"
            >
              <TooltipHelp
                trigger={<span>{isFullscreen ? '✕' : '⛶'}</span>}
                tooltips={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'} asChild
              />
            </Button>
          )}
          <Button
            variant="secondary"
            size="sm"
            onClick={handleToggleCollisions}
            className="bg-gray-800 text-white border border-gray-600 hover:bg-gray-700 focus:outline-none focus:border-gray-600 active:border-gray-600 w-10 h-10 text-2xl font-mono"
          >
            <TooltipHelp
              trigger={<span>{collisionsVisible ? '•' : '◱'}</span>}
              tooltips={collisionsVisible ? 'Hide collision shapes' : 'Show collision shapes'} asChild
            />
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={handleToggleGrid}
            className="bg-gray-800 text-white border border-gray-600 hover:bg-gray-700 focus:outline-none focus:border-gray-600 active:border-gray-600 w-10 h-10 text-2xl font-mono"
          >
            <TooltipHelp
              trigger={<span>{gridVisible ? '⌗' : '⎕'}</span>}
              tooltips={gridVisible ? 'Hide grid' : 'Show grid'} asChild
            />
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void handleCopyLink()}
            className={'bg-gray-800 text-white border border-gray-600 hover:bg-gray-700 w-10 h-10 text-2xl'}
          >
            <TooltipHelp
              trigger={<span>{copied ? '✔' : '🔗'}</span>}
              tooltips={copied ? 'Copy viewer link' : 'Copy viewer link'} asChild
            />
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void handleDownloadAssets()}
            disabled={loadedCount === 0}
            className="bg-gray-800 text-white border border-gray-600 hover:bg-gray-700 focus:outline-none focus:border-gray-600 active:border-gray-600 w-max h-10 text-xl"
          >
            <TooltipHelp
              trigger={<span className="flex items-center gap-2">
                <Download className="h-4 w-4" />
              </span>}
              tooltips={loadedCount === 0 ? 'No model loaded' : 'Download model'} asChild
            />
          </Button>
        </div>
        <div className="absolute bottom-2 left-2 z-10 flex items-end gap-3">
          <TooltipHelp
            trigger={<span className="inline-flex"><Mouse className="w-6 h-6 text-white/90 drop-shadow" /></span>}
            tooltips={(
              <div className="text-sm leading-5">
                <div><span className="font-semibold">Left click</span>: orientation (rotate)</div>
                <div><span className="font-semibold">Right click</span>: forward/backward/left/right</div>
                <div><span className="font-semibold">Middle click</span>: up/down/left/right</div>
              </div>
            )}
            asChild
          />
        </div>
        <canvas
          ref={canvasRef}
          width={1}
          height={1}
          className="bg-gray-800 shadow-inner w-full h-full min-h-0"
          onClick={(e) => {
            if (e.detail === 2) { // double click to enter fullscreen
              handleFullscreenToggle();
            }
          }}
        />
      </div>
      <div className={'lg:w-60 w-full lg:h-full h-[200px] min-h-[200px] bg-gray-800/90 lg:border-l lg:border-t-0 border-t border-gray-600 flex-shrink-0 relative z-10 flex flex-col'}>
          <div className="bg-gray-900 px-3 py-2 text-white font-semibold border-b border-black shrink-0">
            Animations ({sequences.length})
          </div>
          <div className="flex-1 min-h-0 overflow-y-auto">
            <ul className="divide-y divide-gray-600">
              {sequences.length === 0 ? (
                <div className="p-3 text-gray-400">Loading animations...</div>
              ) : (
                sequences.map((seq, idx) => (
                <li
                  key={idx}
                  onClick={() => handleSelectSequence(idx)}
                  className={`px-3 py-2 cursor-pointer text-white relative ${idx === currentSeq ? 'bg-gray-800' : 'hover:bg-gray-700'}`}
                >
                  {seq.name || `Sequence ${idx}`}
                  <span className="text-gray-500 text-xs flex items-center gap-1">
                    ({idx})
                    Duration: {((seq.interval[1] - seq.interval[0]) / 1000).toFixed(3)} s
                    {!seq.nonLooping ? ', looping' : ''}
                  </span>
                  {idx === currentSeq ? (
                    <div className="absolute left-0 bottom-0 h-[2px] bg-blue-600" style={{ width: `${Math.round(progress * 100)}%` }} />
                  ) : null}
                </li>
                ))
              )}
            </ul>
          </div>
        </div>
    </div>
  );
}

async function createGridModel(viewer: ModelViewer, scene: Scene, size: number, step: number): Promise<MdxModelInstance[]> {
  const thickness = 1;
  const lineWidth = mdlx.primitives.createCube(size * step, thickness, thickness / 2);
  const lineHeight = mdlx.primitives.createCube(thickness, size * step, thickness / 2);

  const colorDefault = [0.25, 0.25, 0.25];
  const colorRed = [1, 0, 0];
  const colorGreen = [0, 1, 0];

  const whiteLineWidthMdx = (await mdlx.createPrimitive(viewer, lineWidth, { color: new Float32Array(colorDefault) }))!;
  const whiteLineHeightMdx = (await mdlx.createPrimitive(viewer, lineHeight, { color: new Float32Array(colorDefault) }))!;
  const instances: MdxModelInstance[] = [];
  // White lines at the grid steps
  for (let i = -size * step; i <= size * step; i += step) {
    const widthLine = whiteLineWidthMdx.addInstance();
    const heightLine = whiteLineHeightMdx.addInstance();
    scene.addInstance(widthLine);
    scene.addInstance(heightLine);
    widthLine.setLocation([0, i, 0]);
    heightLine.setLocation([i, 0, 0]);
    instances.push(widthLine, heightLine);
  }

  // Red and green lines at the origin
  const redLineMdx = (await mdlx.createPrimitive(viewer, lineWidth, { color: new Float32Array(colorRed) }))!;
  const redLine = redLineMdx.addInstance();
  scene.addInstance(redLine);
  instances.push(redLine);

  const greenLineMdx = (await mdlx.createPrimitive(viewer, lineHeight, { color: new Float32Array(colorGreen) }))!;
  const greenLine = greenLineMdx.addInstance();
  scene.addInstance(greenLine);
  instances.push(greenLine);

  return instances;
}

'use client';

import mdlx from '@pqhuy98/mdx-m3-viewer/dist/cjs/utils/mdlx';
import Camera from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/camera';
import blpHandler from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/blp/handler';
import mdxHandler from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/handler';
import MdxModel from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/model';
import MdxModelInstance from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/modelinstance';
import Scene from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/scene';
import ModelViewer from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/viewer';
import { vec3 } from 'gl-matrix';
import {
  useEffect, useRef, useState,
} from 'react';

import { Button } from '@/components/ui/button';

import { host } from './config';

interface ModelViewerProps {
  modelPath?: string
  alwaysFullscreen?: boolean
}

// Normalises backslashes to forward slashes for safe URL usage
const normalizePath = (p: string) => p.replace(/\\+/g, '/').replace(/\/+/, '/');

export default function ModelViewerUi({ modelPath, alwaysFullscreen }: ModelViewerProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [sequences, setSequences] = useState<string[]>([]);
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
  useEffect(() => {
    if (!canvasRef.current) return undefined;
    const viewer = new ModelViewer(canvasRef.current);
    viewer.addHandler(mdxHandler);
    viewer.addHandler(blpHandler);
    setViewer(viewer);

    const scene = viewer.addScene();
    setScene(scene);
    scene.color.fill(0.15);
    void createGridModel(viewer, scene, 10, 100);

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
      console.log(`[Viewer] Loading ${e.fetchUrl}`);
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

  // Keep track of which model path has already been loaded to avoid the extra
  // invocation that happens under React 18 Strict Mode while still allowing
  // re-loading when the `modelPath` prop actually changes.
  const lastLoadedPathRef = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (!modelPath || !canvasRef.current || !viewer || !scene || !camera) return undefined;
    // if (lastLoadedPathRef.current === modelPath) {
    //   console.log('Model already loaded, skipping');
    //   return
    // }
    lastLoadedPathRef.current = modelPath;

    // references for cleanup
    const canvas = canvasRef.current;
    let onMouseDown: ((e: MouseEvent) => void) | null = null;
    let onMouseMove: ((e: MouseEvent) => void) | null = null;
    let endDrag: (() => void) | null = null;
    let onWheel: ((e: WheelEvent) => void) | null = null;
    let resizeCanvas: (() => void) | null = null;

    let modelInstance: MdxModelInstance | null = null;

    void (async () => {
      // Ensure collision primitives are ready before loading shapes
      if (primsReadyRef.current) {
        try { await primsReadyRef.current; } catch {
          // ignore
        }
      }
      // Path solver so the viewer fetches every dependant file via our /asset route
      const pathSolver = (src: unknown) => `${host}/assets/${normalizePath(src as string)}`;

      // Load the model (assumed to be in MDX|MDL format)
      const model = await viewer.load(`${normalizePath(modelPath)}`, pathSolver);
      if (!(model instanceof MdxModel)) return;
      modelInstance = model.addInstance();

      instanceRef.current = modelInstance;
      modelInstance.setSequence(0);
      modelInstance.sequenceLoopMode = 2; // always loop
      setSequences(model.sequences.map((s) => s.name || `Sequence ${model.sequences.indexOf(s)}`));

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
      let horizontalAngle = 0;
      let verticalAngle = Math.PI / 6;
      let distance = 500;
      const target = vec3.fromValues(0, 0, 0);
      target[2] = modelInstance.getBounds().z;

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
            camera.farClipPlane,
          );
        }
      };
      resizeCanvas();

      // Mouse & wheel controls
      let button = 0;
      onMouseDown = (e: MouseEvent) => {
        e.preventDefault();
        button = e.button;
        if (e.button === 0 || e.button === 2) {
          isDragging = true;
          lastX = e.clientX;
          lastY = e.clientY;
        }
      };

      onMouseMove = (e: MouseEvent) => {
        if (!isDragging) return;
        if (button === 0) { // left click to rotate
          const dx = e.clientX - lastX;
          const dy = e.clientY - lastY;
          lastX = e.clientX;
          lastY = e.clientY;
          const ROT_SPEED = Math.PI / 360; // radians per pixel
          horizontalAngle -= dx * ROT_SPEED;
          verticalAngle += dy * ROT_SPEED;
          verticalAngle = Math.max(-Math.PI / 2 + 0.01, Math.min(Math.PI / 2 - 0.01, verticalAngle));
        } else if (button === 2) { // right click to move target
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

      endDrag = () => {
        isDragging = false;
      };

      onWheel = (e: WheelEvent) => {
        e.preventDefault();
        const delta = e.deltaY;
        const ZOOM_SPEED = 0.1;
        // adjust distance exponentially for smooth zoom
        distance *= 1 + (delta > 0 ? ZOOM_SPEED : -ZOOM_SPEED);
        distance = Math.max(1, Math.min(20000, distance));
        updateCamera();
      };

      if (canvas) {
        canvas.addEventListener('mousedown', onMouseDown);
        canvas.addEventListener('wheel', onWheel!, { passive: false });
      }

      window.addEventListener('resize', resizeCanvas);
      window.addEventListener('mousemove', onMouseMove!);
      window.addEventListener('mouseup', endDrag!);
    })();

    return () => {
      if (canvas && onMouseDown && onWheel && onMouseMove && endDrag && resizeCanvas) {
        canvas.removeEventListener('mousedown', onMouseDown);
        canvas.removeEventListener('wheel', onWheel);
        window.removeEventListener('mousemove', onMouseMove);
        window.removeEventListener('mouseup', endDrag!);
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
    };
  }, [modelPath, canvasRef.current, viewer, scene]);

  // Apply sequence when currentSeq updates
  useEffect(() => {
    const inst = instanceRef.current;
    if (inst) {
      inst.setSequence(currentSeq);
      inst.sequenceLoopMode = 2;
    }
  }, [currentSeq, instanceRef.current]);

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

  const handleToggleCollisions = () => {
    const next = !collisionsVisible;
    setCollisionsVisible(next);
    for (const inst of collisionInstancesRef.current) {
      try {
        if (next) inst.show?.();
        else inst.hide?.();
      } catch {
        // ignore
      }
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

  return (
    <div className={`flex w-full h-full ${alwaysFullscreen ? 'fixed inset-0 z-50' : isFullscreen ? 'fixed inset-0 z-50' : ''}`}>
      <div className={`relative ${alwaysFullscreen || isFullscreen ? 'flex-1 h-full' : 'flex-grow h-full'}`}>
        {!alwaysFullscreen && (
          <div className="absolute top-2 left-2 z-10 flex gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={handleFullscreenToggle}
              className="bg-gray-800 text-white border border-gray-600 hover:bg-gray-700 focus:outline-none focus:border-gray-600 active:border-gray-600 w-10 h-10 text-2xl"
              title={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
            >
              {isFullscreen ? '✕' : '⛶'}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleToggleCollisions}
              className="bg-gray-800 text-white border border-gray-600 hover:bg-gray-700 focus:outline-none focus:border-gray-600 active:border-gray-600 w-10 h-10 text-2xl font-mono"
              title={collisionsVisible ? 'Hide collisions' : 'Show collisions'}
            >
              {collisionsVisible ? '•' : '◱'}
            </Button>
            <Button
                variant="secondary"
                size="sm"
                onClick={() => void handleCopyLink()}
                className={'bg-gray-800 text-white border border-gray-600 hover:bg-gray-700 focus:outline-none focus:border-gray-600 active:border-gray-600 w-10 h-10 text-2xl'}
                title={copied ? 'Link copied!' : 'Copy viewer link'}
              >
                {copied ? '✔' : '🔗'}
              </Button>
          </div>
        )}
        <canvas
          ref={canvasRef}
          width={1}
          height={1}
          className="bg-gray-800 shadow-inner w-full h-full"
        />
      </div>
      <div className={`w-60 ${alwaysFullscreen || isFullscreen ? 'h-full' : 'h-full'} overflow-y-auto bg-gray-800/90 border-l border-gray-600`}>
        <div className="sticky top-0 z-10 bg-gray-900 px-3 py-2 text-white font-semibold border-b border-black">
          Animations
        </div>
        <ul className="divide-y divide-gray-600">
          {sequences.length === 0 ? (
            <div className="p-3 text-gray-400">Loading...</div>
          ) : (
            sequences.map((name, idx) => (
            <li
              key={idx}
              onClick={() => setCurrentSeq(idx)}
              className={`px-3 py-2 cursor-pointer text-white ${idx === currentSeq ? 'bg-gray-800' : 'hover:bg-gray-700'}`}
            >
              {name || `Sequence ${idx}`}
            </li>
            ))
          )}
        </ul>
      </div>
    </div>
  );
}

async function createGridModel(viewer: ModelViewer, scene: Scene, size: number, step: number) {
  const thickness = 1;
  const lineWidth = mdlx.primitives.createCube(size * step, thickness, thickness / 2);
  const lineHeight = mdlx.primitives.createCube(thickness, size * step, thickness / 2);

  const colorDefault = [0.25, 0.25, 0.25];
  const colorRed = [1, 0, 0];
  const colorGreen = [0, 1, 0];

  const whiteLineWidthMdx = (await mdlx.createPrimitive(viewer, lineWidth, { color: new Float32Array(colorDefault) }))!;
  const whiteLineHeightMdx = (await mdlx.createPrimitive(viewer, lineHeight, { color: new Float32Array(colorDefault) }))!;
  // White lines at the grid steps
  for (let i = -size * step; i <= size * step; i += step) {
    const widthLine = whiteLineWidthMdx.addInstance();
    const heightLine = whiteLineHeightMdx.addInstance();
    scene.addInstance(widthLine);
    scene.addInstance(heightLine);
    widthLine.setLocation([0, i, 0]);
    heightLine.setLocation([i, 0, 0]);
  }

  // Red and green lines at the origin
  const redLineMdx = (await mdlx.createPrimitive(viewer, lineWidth, { color: new Float32Array(colorRed) }))!;
  const redLine = redLineMdx.addInstance();
  scene.addInstance(redLine);

  const greenLineMdx = (await mdlx.createPrimitive(viewer, lineHeight, { color: new Float32Array(colorGreen) }))!;
  const greenLine = greenLineMdx.addInstance();
  scene.addInstance(greenLine);
}

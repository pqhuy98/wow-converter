'use client';

import { downloadAssetsZip } from '@api/download';
import mdlx from '@pqhuy98/mdx-m3-viewer/dist/cjs/utils/mdlx';
import blpHandler from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/blp/handler';
import Camera from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/camera';
import mdxHandler from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/handler';
import MdxModel from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/model';
import MdxModelInstance from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/modelinstance';
import Sequence from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/sequence';
import Scene from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/scene';
import ModelViewer from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/viewer';
import { vec3 } from 'gl-matrix';
import { Download, Mouse } from 'lucide-react';
import {
  useEffect, useMemo, useRef, useState,
} from 'react';

import { Button } from '@/components/ui/button';

import { useServerConfig } from '../../server-config';
import { TooltipHelp } from '../tooltip-help';
import { CutBox, cutBoxFromInstanceBounds, CutBoxOrthoView } from './cut-box-view';

interface ModelViewerProps {
  modelPath?: string
  /** When unchanged across reloads (e.g. skin swap), orbit camera is preserved. */
  cameraSessionKey?: string
  alwaysFullscreen?: boolean
  source?: 'export' | 'browse'
}

// Normalises backslashes to forward slashes for safe URL usage
const normalizePath = (p: string) => p.replace(/\\+/g, '/').replace(/\/+/, '/');

const MAX_DISTANCE = 2000000;
const FAR_CLIP_PLANE = 100_000_000;

export default function ModelViewerUi({ modelPath, cameraSessionKey, alwaysFullscreen, source }: ModelViewerProps) {
  const serverConfig = useServerConfig();
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const canvasContainerRef = useRef<HTMLDivElement | null>(null);
  const [sequences, setSequences] = useState<Sequence[]>([]);
  const [currentSeq, setCurrentSeq] = useState<number>(0);
  const instanceRef = useRef<MdxModelInstance | null>(null);
  const modelRef = useRef<MdxModel | null>(null);
  const sceneRef = useRef<Scene | null>(null);
  const vecHeap = vec3.create();

  const [viewer, setViewer] = useState<ModelViewer | null>(null);
  // Collision toggling and primitive refs
  const [collisionsVisible, setCollisionsVisible] = useState(false);
  const collisionInstancesRef = useRef<MdxModelInstance[]>([]);
  const boxModelRef = useRef<MdxModel | undefined>(undefined);
  const sphereModelRef = useRef<MdxModel | undefined>(undefined);
  const cutBoxModelRef = useRef<MdxModel | undefined>(undefined);
  const primsReadyRef = useRef<Promise<void> | undefined>(undefined);
  const loadRequestIdRef = useRef(0);
  const [gridVisible, setGridVisible] = useState(true);
  const gridInstancesRef = useRef<MdxModelInstance[]>([]);
  const [cameras, setCameras] = useState<Camera[]>([]);
  const [currentCamera, setCurrentCamera] = useState<number | null>(null);
  const cutBoxOverlayRef = useRef<MdxModelInstance | null>(null);
  const orbitCameraRef = useRef({
    sessionKey: '',
    horizontalAngle: 0,
    verticalAngle: Math.PI / 6,
    distance: 500,
    target: vec3.create(),
  });
  const animationSessionRef = useRef({
    sessionKey: '',
    sequenceIndex: 0,
    sequenceName: '',
  });
  const modelCameraSessionRef = useRef({
    sessionKey: '',
    cameraIndex: null as number | null,
    cameraName: '',
  });

  const applyModelCameraIndex = (idx: number): boolean => {
    const scene = sceneRef.current;
    const model = modelRef.current;
    const canvas = canvasRef.current;
    if (!scene || !model || !canvas) return false;
    const cam = model.cameras[idx];
    if (!cam) return false;
    const width = canvas.width || canvas.clientWidth || 1;
    const height = canvas.height || canvas.clientHeight || 1;
    const aspect = width / Math.max(1, height);
    try {
      scene.camera.perspective(
        cam.fieldOfView,
        aspect,
        cam.nearClippingPlane,
        FAR_CLIP_PLANE,
      );
      const from = vec3.fromValues(cam.position[0], cam.position[1], cam.position[2]);
      const to = vec3.fromValues(cam.targetPosition[0], cam.targetPosition[1], cam.targetPosition[2]);
      scene.camera.moveToAndFace(from, to, [0, 0, 1]);
      setCurrentCamera(idx);
      if (cameraSessionKey) {
        const mc = modelCameraSessionRef.current;
        mc.sessionKey = cameraSessionKey;
        mc.cameraIndex = idx;
        mc.cameraName = cam.name ?? '';
      }
      return true;
    } catch (e) {
      console.error('Failed to set camera', e);
      return false;
    }
  };

  const syncOrbitCameraSelection = () => {
    if (cameraSessionKey) {
      const mc = modelCameraSessionRef.current;
      mc.sessionKey = cameraSessionKey;
      mc.cameraIndex = null;
      mc.cameraName = '';
    }
    setCurrentCamera(null);
  };

  const resizeViewerToCanvas = () => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const container = canvasContainerRef.current ?? canvas;
    const width = Math.max(1, Math.floor(container.clientWidth));
    const height = Math.max(1, Math.floor(container.clientHeight));

    if (canvas.width !== width) canvas.width = width;
    if (canvas.height !== height) canvas.height = height;

    const scene = sceneRef.current;
    if (!scene) return;
    scene.viewport[2] = width;
    scene.viewport[3] = height;

    const cam = scene.camera;
    const aspect = width / Math.max(1, height);
    try {
      cam.perspective(cam.fov, aspect, cam.nearClipPlane, FAR_CLIP_PLANE);
    } catch {
      // ignore
    }
  };

  // Keep the 3D canvas backing resolution in sync with layout changes (e.g. toggling Cut Box).
  useEffect(() => {
    const el = canvasContainerRef.current;
    if (!el) return undefined;

    let raf = 0;
    const schedule = () => {
      if (raf) cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() => resizeViewerToCanvas());
    };

    const ro = new ResizeObserver(schedule);
    ro.observe(el);
    schedule();
    window.addEventListener('resize', schedule);

    return () => {
      window.removeEventListener('resize', schedule);
      ro.disconnect();
      if (raf) cancelAnimationFrame(raf);
    };
  }, []);

  // Track loaded asset files for download
  const baseUrlRef = useRef<string>('/api/assets');
  const loadedFilesRef = useRef<Set<string>>(new Set());
  const [loadedCount, setLoadedCount] = useState<number>(0);
  useEffect(() => {
    baseUrlRef.current = source === 'browse' ? '/api/browse-assets' : '/api/assets';
  }, [source]);
  useEffect(() => {
    if (!canvasRef.current) return undefined;
    const viewer = new ModelViewer(canvasRef.current);
    viewer.addHandler(mdxHandler);
    viewer.addHandler(blpHandler);
    setViewer(viewer);

    // Prepare primitive models for collision visualization
    primsReadyRef.current = (async () => {
      try {
        const [boxM, sphereM, cutBoxM] = await Promise.all([
          mdlx.createPrimitive(viewer, mdlx.primitives.createUnitCube(), { lines: true }),
          mdlx.createPrimitive(viewer, mdlx.primitives.createUnitSphere(12, 12), { lines: true }),
          mdlx.createPrimitive(
            viewer,
            mdlx.primitives.createUnitCube(),
            // Solid (opaque) cube used as a cut region preview (occludes what's inside).
            { color: new Float32Array([0.95, 0.85, 0.15, 0.5]), twoSided: true },
          ),
        ]);
        boxModelRef.current = boxM;
        sphereModelRef.current = sphereM;
        cutBoxModelRef.current = cutBoxM;
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
            setLoadedCount((prev) => prev + 1);
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

  useEffect(() => {
    if (!modelPath || !canvasRef.current || !viewer) return undefined;

    // references for cleanup
    const canvas = canvasRef.current;
    // reset loaded asset tracker on new load
    loadedFilesRef.current = new Set();

    viewer.clear();
    const scene = viewer.addScene();
    scene.color.fill(0.15);
    const camera = scene.camera;
    sceneRef.current = scene;
    camera.perspective(camera.fov, camera.aspect, camera.nearClipPlane, FAR_CLIP_PLANE);
    resizeViewerToCanvas();

    setLoadedCount(0);
    void (async () => {
      const gridInsts = await createGridModel(viewer, scene, 50, 128);
      gridInstancesRef.current = gridInsts;
    })();

    const requestId = ++loadRequestIdRef.current;
    let cancelled = false;
    let onMouseDown: ((e: MouseEvent) => void) | null = null;
    let onMouseMove: ((e: MouseEvent) => void) | null = null;
    let onMouseUp: ((e: MouseEvent) => void) | null = null;
    let onWheel: ((e: WheelEvent) => void) | null = null;
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
      const base = baseUrlRef.current;
      const pathSolver = (src: unknown) => `${base}/${normalizePath(src as string)}`;

      // Load the model (assumed to be in MDX|MDL format)
      const model = await viewer.load(`${normalizePath(modelPath)}`, pathSolver);
      if (cancelled || loadRequestIdRef.current !== requestId) return;
      if (!(model instanceof MdxModel)) return;
      modelInstance = model.addInstance();
      modelRef.current = model;
      try {
        const cams = model.cameras;
        setCameras(cams);
      } catch {
        setCameras([]);
      }

      if (cancelled || loadRequestIdRef.current !== requestId) return;
      instanceRef.current = modelInstance;

      const anim = animationSessionRef.current;
      const preserveAnim = Boolean(
        cameraSessionKey && anim.sessionKey === cameraSessionKey,
      );
      let nextSeq = 0;
      if (preserveAnim) {
        if (anim.sequenceIndex < model.sequences.length) {
          nextSeq = anim.sequenceIndex;
        } else if (anim.sequenceName) {
          const byName = model.sequences.findIndex((seq) => seq.name === anim.sequenceName);
          if (byName >= 0) nextSeq = byName;
        }
      } else {
        anim.sessionKey = cameraSessionKey ?? modelPath ?? '';
        anim.sequenceIndex = 0;
        anim.sequenceName = model.sequences[0]?.name ?? '';
      }

      setSequences(model.sequences);
      setCurrentSeq(nextSeq);

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
      const orbit = orbitCameraRef.current;
      const preserveOrbit = Boolean(
        cameraSessionKey && orbit.sessionKey === cameraSessionKey,
      );
      if (!preserveOrbit) {
        orbit.sessionKey = cameraSessionKey ?? modelPath ?? '';
        orbit.horizontalAngle = 0;
        orbit.verticalAngle = Math.PI / 6;
        orbit.distance = Math.max(200, Math.min(1000, modelInstance.getBounds().r * 5));
        vec3.set(orbit.target, modelInstance.getBounds().x, modelInstance.getBounds().y, 0);
      }

      const modelCam = modelCameraSessionRef.current;
      const preserveModelCam = Boolean(
        cameraSessionKey && modelCam.sessionKey === cameraSessionKey,
      );
      let restoredCameraIndex: number | null = null;
      if (preserveModelCam && modelCam.cameraIndex != null) {
        if (modelCam.cameraIndex < model.cameras.length) {
          restoredCameraIndex = modelCam.cameraIndex;
        } else if (modelCam.cameraName) {
          const byName = model.cameras.findIndex((c) => c.name === modelCam.cameraName);
          if (byName >= 0) restoredCameraIndex = byName;
        }
      } else if (!preserveModelCam) {
        modelCam.sessionKey = cameraSessionKey ?? modelPath ?? '';
        modelCam.cameraIndex = null;
        modelCam.cameraName = '';
      }

      const updateCamera = () => {
        const x = orbit.distance * Math.cos(orbit.verticalAngle) * Math.cos(orbit.horizontalAngle);
        const y = orbit.distance * Math.cos(orbit.verticalAngle) * Math.sin(orbit.horizontalAngle);
        const z = orbit.distance * Math.sin(orbit.verticalAngle);
        const camPos = vec3.fromValues(orbit.target[0] + x, orbit.target[1] + y, orbit.target[2] + z);
        camera.moveToAndFace(camPos, orbit.target, [0, 0, 1]);
        syncOrbitCameraSelection();
      };
      if (restoredCameraIndex != null && !applyModelCameraIndex(restoredCameraIndex)) {
        updateCamera();
      } else if (restoredCameraIndex == null) {
        updateCamera();
      }

      // Mouse & wheel controls
      let isTouch = false;
      let leftDown = false;
      let middleDown = false;
      let rightDown = false;

      onMouseDown = (e: MouseEvent) => {
        e.preventDefault();
        if (e.button === 0) {
          leftDown = true;
        }
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
          const sw = -dx / w * orbit.distance;
          const sh = dy / h2 * orbit.distance;
          // Move target along camera right (directionX) and up (directionY)
          vec3.add(
            orbit.target,
            orbit.target,
            vec3.scale(
              vecHeap,
              vec3.normalize(vecHeap, vec3.set(vecHeap, camera.directionX[0], camera.directionX[1], camera.directionX[2])),
              sw,
            ),
          );
          vec3.add(
            orbit.target,
            orbit.target,
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
          orbit.horizontalAngle -= dx * ROT_SPEED;
          orbit.verticalAngle += dy * ROT_SPEED;
          orbit.verticalAngle = Math.max(-Math.PI / 2 + 0.01, Math.min(Math.PI / 2 - 0.01, orbit.verticalAngle));
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
          const sw = -dx / w * orbit.distance;
          const sh = dy / h2 * orbit.distance;
          vec3.add(
            orbit.target,
            orbit.target,
            vec3.scale(vecHeap, vec3.normalize(vecHeap, vec3.set(vecHeap, dirX[0], dirX[1], 0)), sw),
          );
          vec3.add(
            orbit.target,
            orbit.target,
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
          orbit.horizontalAngle -= dx * ROT_SPEED;
          orbit.verticalAngle += dy * ROT_SPEED;
          orbit.verticalAngle = Math.max(-Math.PI / 2 + 0.01, Math.min(Math.PI / 2 - 0.01, orbit.verticalAngle));
          updateCamera();
        } else if (e.touches.length === 2) {
          // Pinch to zoom
          const dx = e.touches[0].clientX - e.touches[1].clientX;
          const dy = e.touches[0].clientY - e.touches[1].clientY;
          const currentDistance = Math.sqrt(dx * dx + dy * dy);

          if (lastDistance > 0) {
            const scale = currentDistance / lastDistance;
            const ZOOM_SPEED = -1;
            orbit.distance *= 1 + (scale - 1) * ZOOM_SPEED;
            orbit.distance = Math.max(1, Math.min(MAX_DISTANCE, orbit.distance));
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
        orbit.distance *= 1 + (delta > 0 ? ZOOM_SPEED : -ZOOM_SPEED);
        orbit.distance = Math.max(1, Math.min(MAX_DISTANCE, orbit.distance));
        updateCamera();
      };

      if (canvas) {
        canvas.addEventListener('mousedown', onMouseDown);
        canvas.addEventListener('wheel', onWheel!, { passive: false });
        canvas.addEventListener('touchstart', onTouchStart, { passive: false });
        canvas.addEventListener('touchmove', onTouchMove, { passive: false });
        canvas.addEventListener('touchend', onTouchEnd, { passive: false });
      }
      window.addEventListener('mousemove', onMouseMove!);
      window.addEventListener('mouseup', onMouseUp!);
    })();

    return () => {
      cancelled = true;
      // Hide/detach cut box overlay instance on model unload
      try {
        cutBoxOverlayRef.current?.hide?.();
      } catch {
        // ignore
      }
      cutBoxOverlayRef.current = null;
      if (canvas && onMouseDown && onWheel && onMouseMove && onMouseUp && onTouchStart && onTouchMove && onTouchEnd) {
        canvas.removeEventListener('mousedown', onMouseDown);
        canvas.removeEventListener('wheel', onWheel);
        canvas.removeEventListener('touchstart', onTouchStart);
        canvas.removeEventListener('touchmove', onTouchMove);
        canvas.removeEventListener('touchend', onTouchEnd);
        window.removeEventListener('mousemove', onMouseMove);
        window.removeEventListener('mouseup', onMouseUp!);
      }
      viewer.resources.forEach((resource) => viewer.unload(resource));

      collisionInstancesRef.current = [];
      if (instanceRef.current === modelInstance) {
        instanceRef.current = null;
      }
      if (modelRef.current === (modelInstance?.model as (MdxModel | undefined))) {
        modelRef.current = null;
        setCameras([]);
        setCurrentCamera(null);
      }
    };
  }, [modelPath, cameraSessionKey, canvasRef.current, viewer]);

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
    if (cameraSessionKey) {
      const anim = animationSessionRef.current;
      anim.sessionKey = cameraSessionKey;
      anim.sequenceIndex = currentSeq;
      anim.sequenceName = sequences[currentSeq]?.name ?? anim.sequenceName;
    }
  }, [currentSeq, cameraSessionKey, sequences]);

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

  const [cutBoxEnabled, setCutBoxEnabled] = useState<boolean>(false);
  const [cutBox, setCutBox] = useState<CutBox>(() => ({
    minX: -50,
    minY: -50,
    minZ: -50,
    maxX: 50,
    maxY: 50,
    maxZ: 50,
  }));
  const [cutBoxCopied, setCutBoxCopied] = useState(false);

  const handleFullscreenToggle = () => {
    if (!isFullscreen) {
      // Entering fullscreen - save current scroll position
      scrollPositionRef.current = { x: window.scrollX, y: window.scrollY };
    }
    setIsFullscreen(!isFullscreen);
  };

  const handleCopyLink = async () => {
    const viewerUrl = `${window.location.origin}/viewer?${source === 'browse' ? 'source=browse&' : ''}model=${encodeURIComponent(modelPath || '')}`;
    try {
      await navigator.clipboard.writeText(viewerUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy link:', err);
    }
  };

  const handleSelectCamera = (idx: number) => {
    applyModelCameraIndex(idx);
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

  const resetCutBoxToModelBounds = () => {
    const inst = instanceRef.current;
    if (!inst) return;
    setCutBox(cutBoxFromInstanceBounds(inst));
  };

  // Keep the cut box overlay instance in sync (create lazily when enabled).
  useEffect(() => {
    const scene = sceneRef.current;
    const boxModel = cutBoxModelRef.current ?? boxModelRef.current;
    if (!scene || !boxModel) return;

    if (!cutBoxEnabled) {
      try {
        cutBoxOverlayRef.current?.hide?.();
      } catch {
        // ignore
      }
      return;
    }

    let overlay = cutBoxOverlayRef.current;
    if (!overlay) {
      try {
        overlay = boxModel.addInstance();
        overlay.setScene(scene);
        overlay.dontInheritScaling = false;
        cutBoxOverlayRef.current = overlay;
      } catch {
        cutBoxOverlayRef.current = null;
        return;
      }
    }

    try {
      const cx = (cutBox.minX + cutBox.maxX) / 2;
      const cy = (cutBox.minY + cutBox.maxY) / 2;
      const cz = (cutBox.minZ + cutBox.maxZ) / 2;
      const sx = (cutBox.maxX - cutBox.minX) / 2;
      const sy = (cutBox.maxY - cutBox.minY) / 2;
      const sz = (cutBox.maxZ - cutBox.minZ) / 2;
      overlay.setLocation([cx, cy, cz]);
      overlay.setScale([Math.max(0.001, sx), Math.max(0.001, sy), Math.max(0.001, sz)]);
      overlay.show?.();
    } catch {
      // ignore
    }
  }, [cutBoxEnabled, cutBox]);

  // When enabling cut box, default to model bounds (if available).
  useEffect(() => {
    if (!cutBoxEnabled) return;
    const inst = instanceRef.current;
    if (!inst) return;
    setCutBox((prev) => {
      // If the box looks like the default placeholder, snap to model bounds.
      const looksDefault = Math.abs(prev.minX + 50) < 1e-6
        && Math.abs(prev.maxX - 50) < 1e-6
        && Math.abs(prev.minY + 50) < 1e-6
        && Math.abs(prev.maxY - 50) < 1e-6
        && Math.abs(prev.minZ + 50) < 1e-6
        && Math.abs(prev.maxZ - 50) < 1e-6;
      return looksDefault ? cutBoxFromInstanceBounds(inst) : prev;
    });
  }, [cutBoxEnabled]);

  const cutBoxCode = useMemo(() => {
    const min = `[${formatNumberForCode(cutBox.minX)}, ${formatNumberForCode(cutBox.minY)}, ${formatNumberForCode(cutBox.minZ)}]`;
    const max = `[${formatNumberForCode(cutBox.maxX)}, ${formatNumberForCode(cutBox.maxY)}, ${formatNumberForCode(cutBox.maxZ)}]`;
    return `model.modify.deleteVerticesInsideBox(${min}, ${max});`;
  }, [cutBox]);

  const modelBounds = useMemo<CutBox | undefined>(() => {
    const inst = instanceRef.current;
    if (!inst) return undefined;
    return cutBoxFromInstanceBounds(inst);
  }, [cutBoxEnabled, modelPath]);

  const handleCopyCutBoxCode = async () => {
    try {
      await navigator.clipboard.writeText(cutBoxCode);
      setCutBoxCopied(true);
      setTimeout(() => setCutBoxCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy cut box code:', err);
    }
  };

  useEffect(() => {
    // If exiting fullscreen, immediately resize canvas and restore scroll
    if (!isFullscreen && scrollPositionRef.current) {
      resizeViewerToCanvas();
      // Restore scroll position
      window.scrollTo(scrollPositionRef.current.x, scrollPositionRef.current.y);
      // This is a hack to fix a bug where the canvas is not resized when exiting fullscreen
      setTimeout(() => {
        resizeViewerToCanvas();
      }, 100);
    }

    resizeViewerToCanvas();
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
      <div ref={canvasContainerRef} className={`relative flex-1 min-h-0 ${alwaysFullscreen || isFullscreen ? 'h-full' : 'h-full'}`}>
        <div className="absolute top-2 left-2 z-10 flex gap-2">
          {!alwaysFullscreen && (
            <Button
              variant="secondary"
              size="sm"
              onClick={handleFullscreenToggle}
              className="w-10 h-10 text-2xl bg-[hsl(var(--viewer-control-bg))] text-[hsl(var(--viewer-sidebar-fg))] border border-[hsl(var(--viewer-divider))] hover:bg-[hsl(var(--viewer-item-hover))] focus:outline-none"
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
            className="w-10 h-10 text-2xl bg-[hsl(var(--viewer-control-bg))] text-[hsl(var(--viewer-sidebar-fg))] border border-[hsl(var(--viewer-divider))] hover:bg-[hsl(var(--viewer-item-hover))] focus:outline-none"
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
            className="w-10 h-10 text-2xl bg-[hsl(var(--viewer-control-bg))] text-[hsl(var(--viewer-sidebar-fg))] border border-[hsl(var(--viewer-divider))] hover:bg-[hsl(var(--viewer-item-hover))] focus:outline-none"
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
            className={'w-10 h-10 text-2xl bg-[hsl(var(--viewer-control-bg))] text-[hsl(var(--viewer-sidebar-fg))] border border-[hsl(var(--viewer-divider))] hover:bg-[hsl(var(--viewer-item-hover))] focus:outline-none'}
          >
            <TooltipHelp
              trigger={<span>{copied ? '✔' : '🔗'}</span>}
              tooltips={copied ? 'Copy viewer link' : 'Copy viewer link'} asChild
            />
          </Button>
          {serverConfig.isSharedHosting && <Button
            variant="secondary"
            size="sm"
            onClick={() => void handleDownloadAssets()}
            disabled={loadedCount === 0}
            className="w-10 h-10 text-xl bg-[hsl(var(--viewer-control-bg))] text-[hsl(var(--viewer-sidebar-fg))] border border-[hsl(var(--viewer-divider))] hover:bg-[hsl(var(--viewer-item-hover))] focus:outline-none p-0"
          >
            <TooltipHelp
              trigger={<Download className="h-full w-full" />}
              tooltips={loadedCount === 0 ? 'No model loaded' : 'Download model'} asChild
            />
          </Button>}
          {serverConfig.isDev && <Button
            variant="secondary"
            size="sm"
            onClick={() => setCutBoxEnabled((v) => !v)}
            className="w-10 h-10 text-2xl bg-[hsl(var(--viewer-control-bg))] text-[hsl(var(--viewer-sidebar-fg))] border border-[hsl(var(--viewer-divider))] hover:bg-[hsl(var(--viewer-item-hover))] focus:outline-none"
          >
            <TooltipHelp
              trigger={<span>{cutBoxEnabled ? '▭' : '▢'}</span>}
              tooltips={cutBoxEnabled ? 'Disable Cut Box' : 'Enable Cut Box'} asChild
            />
          </Button>}
        </div>
        <div className="absolute bottom-2 left-2 z-10 flex items-end gap-3">
          <TooltipHelp
            trigger={<span className="inline-flex"><Mouse className="w-6 h-6 text-[hsl(var(--viewer-sidebar-fg))]/90 drop-shadow" /></span>}
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
          className="w-full h-full min-h-0 bg-secondary shadow-inner"
          onClick={(e) => {
            if (e.detail === 2) { // double click to enter fullscreen
              handleFullscreenToggle();
            }
          }}
        />
      </div>
      <div className="flex lg:flex-row flex-col lg:h-full h-[420px] min-h-[420px] lg:border-l border-t lg:border-t-0 border-[hsl(var(--viewer-divider))] flex-shrink-0 relative z-10 bg-[hsl(var(--viewer-sidebar-bg))] text-[hsl(var(--viewer-sidebar-fg))]">
        <div className={'lg:w-60 w-full lg:h-full h-[200px] min-h-[200px] lg:border-r border-b lg:border-b-0 border-[hsl(var(--viewer-divider))] flex flex-col'}>
          <div className="flex-1 lg:flex lg:flex-col lg:overflow-hidden overflow-y-auto viewer-scroll">
            <div className="px-3 py-2 font-semibold bg-[hsl(var(--viewer-item-active))] text-[hsl(var(--viewer-sidebar-fg))] border-b border-[hsl(var(--viewer-divider))] lg:sticky lg:top-0 lg:z-10">
              Cameras ({cameras.length})
            </div>
            <div className="lg:max-h-48 lg:overflow-y-auto lg:border-b border-b border-[hsl(var(--viewer-divider))]">
              <ul className="divide-y divide-[hsl(var(--viewer-divider))]">
                {cameras.length === 0 ? (
                  <div className="p-3 text-muted-foreground">No cameras</div>
                ) : (
                  cameras.map((cam, idx) => (
                  <li
                    key={idx}
                    onClick={() => handleSelectCamera(idx)}
                    className={`px-3 py-2 cursor-pointer ${idx === currentCamera ? 'bg-[hsl(var(--viewer-item-active))]' : 'hover:bg-[hsl(var(--viewer-item-hover))]'}`}
                  >
                    {cam.name || `Camera ${idx}`}
                    <span className="text-[hsl(var(--viewer-muted))] text-xs flex items-center gap-1">
                      FOV: {(((cam?.fieldOfView ?? 0) * 180) / Math.PI).toFixed(1)}°
                    </span>
                  </li>
                  ))
                )}
              </ul>
            </div>
            <div className="h-6" />
            <div className="px-3 py-2 font-semibold bg-[hsl(var(--viewer-item-active))] text-[hsl(var(--viewer-sidebar-fg))] border-b border-[hsl(var(--viewer-divider))] lg:sticky lg:top-[60px] lg:z-10">
              Animations ({sequences.length})
            </div>
            <div className="lg:flex-1 lg:min-h-0 lg:overflow-y-auto">
              <ul className="divide-y divide-[hsl(var(--viewer-divider))]">
                {sequences.length === 0 ? (
                  <div className="p-3 text-muted-foreground">Loading animations...</div>
                ) : (
                  sequences.map((seq, idx) => (
                  <li
                    key={idx}
                    onClick={() => handleSelectSequence(idx)}
                    className={`px-3 py-2 cursor-pointer relative ${idx === currentSeq ? 'bg-[hsl(var(--viewer-item-active))]' : 'hover:bg-[hsl(var(--viewer-item-hover))]'}`}
                  >
                    {seq.name || `Sequence ${idx}`}
                    <span className="text-[hsl(var(--viewer-muted))] text-xs flex items-center gap-1">
                      ({idx})
                      Duration: {((seq.interval[1] - seq.interval[0]) / 1000).toFixed(3)} s
                      {!seq.nonLooping ? ', looping' : ''}
                    </span>
                    {idx === currentSeq ? (
                      <div className="absolute left-0 bottom-0 h-[2px] bg-[hsl(var(--animation-progress))]" style={{ width: `${Math.round(progress * 100)}%` }} />
                    ) : null}
                  </li>
                  ))
                )}
              </ul>
            </div>
            <div className="h-6" />
          </div>
        </div>

        {cutBoxEnabled ? (
        <div className="lg:w-[540px] w-full lg:h-full flex flex-col min-h-0">
          <div className="px-3 py-2 font-semibold bg-[hsl(var(--viewer-item-active))] border-b border-[hsl(var(--viewer-divider))] lg:sticky lg:top-0 lg:z-10 flex items-center justify-between">
            <div className="flex items-center gap-2">
              Cut Box
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={resetCutBoxToModelBounds}
                disabled={!instanceRef.current}
                className="h-8 px-2 bg-[hsl(var(--viewer-control-bg))] text-[hsl(var(--viewer-sidebar-fg))] border border-[hsl(var(--viewer-divider))] hover:bg-[hsl(var(--viewer-item-hover))]"
              >
                Reset
              </Button>
              <Button
                variant="secondary"
                size="sm"
                onClick={() => void handleCopyCutBoxCode()}
                className="h-8 px-2 bg-[hsl(var(--viewer-control-bg))] text-[hsl(var(--viewer-sidebar-fg))] border border-[hsl(var(--viewer-divider))] hover:bg-[hsl(var(--viewer-item-hover))]"
              >
                {cutBoxCopied ? 'Copied' : 'Copy code'}
              </Button>
            </div>
          </div>

          <div className="flex-1 min-h-0 overflow-hidden">
            <div className="p-3 h-full flex flex-col min-h-0 gap-3">
              <div className="text-xs text-[hsl(var(--viewer-muted))] leading-5 flex-shrink-0">
                Drag rectangle sides in the ortho views to edit min/max. Use mouse wheel to zoom (won’t scroll this panel).
              </div>

              <div className="flex-1 min-h-0 flex flex-col gap-3">
                <div className="flex-1 min-h-0">
                  <CutBoxOrthoView
                    title="Top (X / Y)"
                    enabled={cutBoxEnabled}
                    box={cutBox}
                    modelBounds={modelBounds}
                    plane="xy"
                    onChangeBox={setCutBox}
                  />
                </div>

                <div className="flex-1 min-h-0">
                  <CutBoxOrthoView
                    title="Side (X / Z)"
                    enabled={cutBoxEnabled}
                    box={cutBox}
                    modelBounds={modelBounds}
                    plane="xz"
                    onChangeBox={setCutBox}
                  />
                </div>
              </div>

              <div className="pt-2 border-t border-[hsl(var(--viewer-divider))] flex-shrink-0">
                <div className="text-xs font-mono text-[hsl(var(--viewer-muted))] break-words">
                  {cutBoxCode}
                </div>
              </div>
            </div>
          </div>
        </div>
        ) : null}
      </div>
    </div>
  );
}

async function createGridModel(viewer: ModelViewer, scene: Scene, size: number, step: number): Promise<MdxModelInstance[]> {
  const thickness1 = 0.2;
  const lineWidth = mdlx.primitives.createCube(size * step, thickness1, thickness1);
  const lineHeight = mdlx.primitives.createCube(thickness1, size * step, thickness1);
  const thickness2 = 1;
  const lineWidth2 = mdlx.primitives.createCube(size * step, thickness2, thickness2);
  const lineHeight2 = mdlx.primitives.createCube(thickness2, size * step, thickness2);

  const colorDefault = [0.5, 0.5, 0.5];
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
  const redLineMdx = (await mdlx.createPrimitive(viewer, lineWidth2, { color: new Float32Array(colorRed) }))!;
  const redLine = redLineMdx.addInstance();
  scene.addInstance(redLine);
  instances.push(redLine);

  const greenLineMdx = (await mdlx.createPrimitive(viewer, lineHeight2, { color: new Float32Array(colorGreen) }))!;
  const greenLine = greenLineMdx.addInstance();
  scene.addInstance(greenLine);
  instances.push(greenLine);

  return instances;
}

function formatNumberForCode(n: number): string {
  const s = n.toFixed(3);
  return s.replace(/\.?0+$/, '');
}

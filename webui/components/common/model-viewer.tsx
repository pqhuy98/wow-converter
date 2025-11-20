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
  useEffect, useRef, useState,
} from 'react';

import { Button } from '@/components/ui/button';

import { useServerConfig } from '../server-config';
import { TooltipHelp } from './tooltip-help';

interface ModelViewerProps {
  modelPath?: string
  alwaysFullscreen?: boolean
  source?: 'export' | 'browse'
}

// Normalises backslashes to forward slashes for safe URL usage
const normalizePath = (p: string) => p.replace(/\\+/g, '/').replace(/\/+/, '/');

const MAX_DISTANCE = 2000000;

interface CutBoxState {
  visible: boolean;
  min: [number, number, number];
  max: [number, number, number];
}

export default function ModelViewerUi({ modelPath, alwaysFullscreen, source }: ModelViewerProps) {
  const serverConfig = useServerConfig();
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
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
  // Cut box primitives and instances
  const cutBoxWireModelRef = useRef<MdxModel | undefined>(undefined);
  const cutHandleModelRef = useRef<MdxModel | undefined>(undefined);
  const cutFaceModelRef = useRef<MdxModel | undefined>(undefined);
  const cutBoxInstanceRef = useRef<MdxModelInstance | null>(null);
  const cutHandleInstancesRef = useRef<MdxModelInstance[]>([]);
  const cutFaceInstancesRef = useRef<MdxModelInstance[]>([]);
  const cutHandleDefsRef = useRef<ReadonlyArray<{ axis: 0 | 1 | 2; side: 'min' | 'max' }>>([
    { axis: 0, side: 'min' },
    { axis: 0, side: 'max' },
    { axis: 1, side: 'min' },
    { axis: 1, side: 'max' },
    { axis: 2, side: 'min' },
    { axis: 2, side: 'max' },
  ]);
  const [cutBox, setCutBox] = useState<CutBoxState>({ visible: false, min: [0, 0, 0], max: [0, 0, 0] });
  const cutBoxRef = useRef<CutBoxState>(cutBox);
  cutBoxRef.current = cutBox;
  const [copiedCutCode, setCopiedCutCode] = useState<boolean>(false);
  const hoveredFaceIdxRef = useRef<number>(-1);
  const draggingCutHandleRef = useRef<{
    axis: 0 | 1 | 2;
    side: 'min' | 'max';
    grabOffset: number; // difference between initial hit[axis] and current face coordinate
  } | null>(null);
  const primsReadyRef = useRef<Promise<void> | undefined>(undefined);
  const loadRequestIdRef = useRef(0);
  const [gridVisible, setGridVisible] = useState(true);
  const gridInstancesRef = useRef<MdxModelInstance[]>([]);
  const [cameras, setCameras] = useState<Camera[]>([]);
  const [currentCamera, setCurrentCamera] = useState<number | null>(null);
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
        const colorCut = new Float32Array([1, 0.5, 0]); // orange wireframe
        const colorHandle = new Float32Array([0, 0.9, 1]); // cyan solid
        const faceColor = new Float32Array([0.95, 0.55, 0.1]); // face default
        const [boxM, sphereM, cutWire, handleSphere, faceSolid] = await Promise.all([
          mdlx.createPrimitive(viewer, mdlx.primitives.createUnitCube(), { lines: true }),
          mdlx.createPrimitive(viewer, mdlx.primitives.createUnitSphere(12, 12), { lines: true }),
          mdlx.createPrimitive(viewer, mdlx.primitives.createUnitCube(), { lines: true, color: colorCut }),
          mdlx.createPrimitive(viewer, mdlx.primitives.createUnitSphere(10, 10), { color: colorHandle }),
          mdlx.createPrimitive(viewer, mdlx.primitives.createUnitCube(), { color: faceColor }),
        ]);
        boxModelRef.current = boxM;
        sphereModelRef.current = sphereM;
        cutBoxWireModelRef.current = cutWire;
        cutHandleModelRef.current = handleSphere;
        cutFaceModelRef.current = faceSolid;
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
    let resizeCanvas: (() => void) | null = null;
    let onTouchStart: ((e: TouchEvent) => void) | null = null;
    let onTouchMove: ((e: TouchEvent) => void) | null = null;
    let onTouchEnd: ((e: TouchEvent) => void) | null = null;

    let modelInstance: MdxModelInstance | null = null;
    let cutSyncInterval: number | undefined;

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
        setCurrentCamera(null);
      } catch {
        setCameras([]);
        setCurrentCamera(null);
      }

      if (cancelled || loadRequestIdRef.current !== requestId) return;
      instanceRef.current = modelInstance;
      modelInstance.setSequence(0);
      modelInstance.sequenceLoopMode = 2; // always loop
      setSequences(model.sequences);
      setCurrentSeq(0);

      // Add scene and basic camera, grid setup
      scene.addInstance(modelInstance);

      // Initialize/reset Cut Box state to model bounds (approximate using bounding sphere)
      const initCutBox = () => {
        const b = modelInstance!.getBounds();
        const min: [number, number, number] = [b.x - b.r, b.y - b.r, -b.r];
        const max: [number, number, number] = [b.x + b.r, b.y + b.r, b.r];
        setCutBox((prev) => ({ visible: prev.visible, min, max }));
      };
      initCutBox();

      // Helpers for Cut Box instances
      const ensureCutBoxInstances = () => {
        if (!cutBoxWireModelRef.current || !cutHandleModelRef.current) return;
        if (!cutBoxInstanceRef.current) {
          const inst = cutBoxWireModelRef.current.addInstance();
          inst.setScene(scene);
          cutBoxInstanceRef.current = inst;
        }
        if (cutHandleInstancesRef.current.length === 0) {
          cutHandleInstancesRef.current = cutHandleDefsRef.current.map(() => {
            const h = cutHandleModelRef.current!.addInstance();
            h.setScene(scene);
            return h;
          });
        }
        if (cutFaceInstancesRef.current.length === 0 && cutFaceModelRef.current) {
          cutFaceInstancesRef.current = cutHandleDefsRef.current.map(() => {
            const f = cutFaceModelRef.current!.addInstance();
            f.setScene(scene);
            return f;
          });
        }
      };

      const showHideCutBoxInstances = (show: boolean) => {
        if (cutBoxInstanceRef.current) {
          try { show ? cutBoxInstanceRef.current.show?.() : cutBoxInstanceRef.current.hide?.(); } catch { /* noop */ }
        }
        for (const h of cutHandleInstancesRef.current) {
          try { show ? h.show?.() : h.hide?.(); } catch { /* noop */ }
        }
        for (const f of cutFaceInstancesRef.current) {
          try { show ? f.show?.() : f.hide?.(); } catch { /* noop */ }
        }
      };

      const updateCutBoxTransforms = () => {
        ensureCutBoxInstances();
        const inst = cutBoxInstanceRef.current;
        const defs = cutHandleDefsRef.current;
        if (!inst || cutHandleInstancesRef.current.length !== defs.length || cutFaceInstancesRef.current.length !== defs.length) return;
        const [minX, minY, minZ] = cutBoxRef.current.min;
        const [maxX, maxY, maxZ] = cutBoxRef.current.max;
        const cx = (minX + maxX) / 2;
        const cy = (minY + maxY) / 2;
        const cz = (minZ + maxZ) / 2;
        const sx = (maxX - minX) / 2 || 0.0001;
        const sy = (maxY - minY) / 2 || 0.0001;
        const sz = (maxZ - minZ) / 2 || 0.0001;
        inst.setLocation([cx, cy, cz]);
        inst.setScale([sx, sy, sz]);
        // Handle size proportional to smallest dimension
        const diag = Math.max(0.0001, Math.min(sx, sy, sz));
        const handleScale = Math.max(0.02, Math.min(0.2, diag * 0.3));
        cutHandleInstancesRef.current.forEach((h, i) => {
          const def = defs[i]!;
          const hx = def.axis === 0 ? (def.side === 'min' ? minX : maxX) : cx;
          const hy = def.axis === 1 ? (def.side === 'min' ? minY : maxY) : cy;
          const hz = def.axis === 2 ? (def.side === 'min' ? minZ : maxZ) : cz;
          h.setLocation([hx, hy, hz]);
          h.uniformScale(handleScale);
        });
        // Opaque faces (thin cubes)
        const faceThickness = Math.max(0.01, Math.min(0.2, diag * 0.1));
        cutFaceInstancesRef.current.forEach((f, i) => {
          const def = defs[i]!;
          if (def.axis === 0) {
            const x = def.side === 'min' ? minX : maxX;
            f.setLocation([x, cy, cz]);
            f.setScale([faceThickness, sy, sz]);
          } else if (def.axis === 1) {
            const y = def.side === 'min' ? minY : maxY;
            f.setLocation([cx, y, cz]);
            f.setScale([sx, faceThickness, sz]);
          } else {
            const z = def.side === 'min' ? minZ : maxZ;
            f.setLocation([cx, cy, z]);
            f.setScale([sx, sy, faceThickness]);
          }
        });
        if (cutBoxRef.current.visible) {
          showHideCutBoxInstances(true);
        } else {
          showHideCutBoxInstances(false);
        }
      };

      const setFaceHighlight = (hoverIdx: number) => {
        if (cutFaceInstancesRef.current.length === 0) return;
        const defaultColor: number[] = [0.95, 0.55, 0.1];
        const highlightColor: number[] = [1.0, 1.0, 0.0];
        cutFaceInstancesRef.current.forEach((f, i) => {
          try {
            f.setVertexColor(i === hoverIdx ? highlightColor : defaultColor);
          } catch { /* noop */ }
        });
      };

      const pickFace = (ro: [number, number, number], rd: [number, number, number]) => {
        const [minX, minY, minZ] = cutBoxRef.current.min;
        const [maxX, maxY, maxZ] = cutBoxRef.current.max;
        const eps = 1e-8;
        let bestT = Number.POSITIVE_INFINITY;
        let bestIdx = -1;
        const testFace = (axis: 0 | 1 | 2, side: 'min' | 'max') => {
          const defs = cutHandleDefsRef.current;
          const idx = defs.findIndex((d) => d.axis === axis && d.side === side);
          const faceCoord = axis === 0 ? (side === 'min' ? minX : maxX)
            : axis === 1 ? (side === 'min' ? minY : maxY)
              : (side === 'min' ? minZ : maxZ);
          const rdA = rd[axis];
          if (Math.abs(rdA) < eps) return; // parallel to face plane
          const t = (faceCoord - ro[axis]) / rdA;
          if (t <= 0 || t >= bestT) return;
          const hitX = ro[0] + rd[0] * t;
          const hitY = ro[1] + rd[1] * t;
          const hitZ = ro[2] + rd[2] * t;
          if (axis === 0) {
            if (hitY >= minY - eps && hitY <= maxY + eps && hitZ >= minZ - eps && hitZ <= maxZ + eps) {
              bestT = t; bestIdx = idx;
            }
          } else if (axis === 1) {
            if (hitX >= minX - eps && hitX <= maxX + eps && hitZ >= minZ - eps && hitZ <= maxZ + eps) {
              bestT = t; bestIdx = idx;
            }
          } else {
            if (hitX >= minX - eps && hitX <= maxX + eps && hitY >= minY - eps && hitY <= maxY + eps) {
              bestT = t; bestIdx = idx;
            }
          }
        };
        testFace(0, 'min'); testFace(0, 'max');
        testFace(1, 'min'); testFace(1, 'max');
        testFace(2, 'min'); testFace(2, 'max');
        return bestIdx;
      };

      // Keep transforms in sync when state changes
      cutSyncInterval = window.setInterval(updateCutBoxTransforms, 50);

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
      let isDraggingCutHandle = false;
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
        setCurrentCamera(null);
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
        if (e.button === 0) {
          leftDown = true;
          // Shift+Left: start Cut Box drag (prefer face; fallback to handle)
          if (e.shiftKey && cutBoxRef.current.visible) {
            ensureCutBoxInstances();
            // Build a picking ray
            const vp = scene.viewport;
            const x = e.clientX - (canvas?.getBoundingClientRect().left || 0);
            const y = e.clientY - (canvas?.getBoundingClientRect().top || 0);
            const sy = (canvas?.height || 0) - y;
            const ray = new Float32Array(6);
            try {
              camera.screenToWorldRay(ray, new Float32Array([x, sy]), vp);
              const ro: [number, number, number] = [ray[0], ray[1], ray[2]];
              const rf: [number, number, number] = [ray[3], ray[4], ray[5]];
              const rd: [number, number, number] = [rf[0] - ro[0], rf[1] - ro[1], rf[2] - ro[2]];
              const rdn = vec3.normalize(vecHeap, vec3.fromValues(rd[0], rd[1], rd[2]));
              const [minX, minY, minZ] = cutBoxRef.current.min;
              const [maxX, maxY, maxZ] = cutBoxRef.current.max;
              const cx = (minX + maxX) / 2;
              const cy = (minY + maxY) / 2;
              const cz = (minZ + maxZ) / 2;
              const sx = (maxX - minX) / 2 || 0.0001;
              const syz = (maxY - minY) / 2 || 0.0001;
              const szz = (maxZ - minZ) / 2 || 0.0001;
              const diag = Math.max(0.0001, Math.min(sx, syz, szz));
              const handleRadius = Math.max(0.02, Math.min(0.2, diag * 0.3));
              // Prefer face under cursor
              let chosenIdx = pickFace([ro[0], ro[1], ro[2]], [rd[0], rd[1], rd[2]]);
              if (chosenIdx < 0 && cutHandleInstancesRef.current.length > 0) {
                // fallback to handle spheres
                let bestT = Number.POSITIVE_INFINITY;
                cutHandleInstancesRef.current.forEach((_h, i) => {
                  const def = cutHandleDefsRef.current[i]!;
                  const hx = def.axis === 0 ? (def.side === 'min' ? minX : maxX) : cx;
                  const hy = def.axis === 1 ? (def.side === 'min' ? minY : maxY) : cy;
                  const hz = def.axis === 2 ? (def.side === 'min' ? minZ : maxZ) : cz;
                  const ocx = ro[0] - hx;
                  const ocy = ro[1] - hy;
                  const ocz = ro[2] - hz;
                  const b = 2 * (ocx * rdn[0] + ocy * rdn[1] + ocz * rdn[2]);
                  const c = ocx * ocx + ocy * ocy + ocz * ocz - handleRadius * handleRadius;
                  const disc = b * b - 4 * c;
                  if (disc >= 0) {
                    const t = (-b - Math.sqrt(disc)) / 2;
                    if (t > 0 && t < bestT) {
                      bestT = t;
                      chosenIdx = i;
                    }
                  }
                });
              }
              if (chosenIdx >= 0) {
                const def = cutHandleDefsRef.current[chosenIdx]!;
                // Compute camera-facing plane at current face center, and store grab offset
                const planeNormal: [number, number, number] = [camera.directionZ[0], camera.directionZ[1], camera.directionZ[2]];
                const faceCenter: [number, number, number] = def.axis === 0
                  ? [(def.side === 'min' ? minX : maxX), cy, cz]
                  : def.axis === 1
                    ? [cx, (def.side === 'min' ? minY : maxY), cz]
                    : [cx, cy, (def.side === 'min' ? minZ : maxZ)];
                // Intersect current ray with this plane to compute grab offset
                const denom0 = planeNormal[0] * rdn[0] + planeNormal[1] * rdn[1] + planeNormal[2] * rdn[2];
                let grabOffset = 0;
                if (Math.abs(denom0) > 1e-6) {
                  const planeW0 = planeNormal[0] * faceCenter[0] + planeNormal[1] * faceCenter[1] + planeNormal[2] * faceCenter[2];
                  const t0 = (planeW0 - (planeNormal[0] * ro[0] + planeNormal[1] * ro[1] + planeNormal[2] * ro[2])) / denom0;
                  const hit0: [number, number, number] = [ro[0] + rdn[0] * t0, ro[1] + rdn[1] * t0, ro[2] + rdn[2] * t0];
                  const currentFaceCoord = def.axis === 0 ? faceCenter[0] : def.axis === 1 ? faceCenter[1] : faceCenter[2];
                  grabOffset = hit0[def.axis] - currentFaceCoord;
                }
                draggingCutHandleRef.current = { axis: def.axis, side: def.side, grabOffset };
                isDraggingCutHandle = true;
              }
            } catch {
              // ignore picking failures
            }
          }
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
        // Hover highlight
        if (!isTouch && cutBoxRef.current.visible && canvas) {
          try {
            const vp = scene.viewport;
            const x = e.clientX - (canvas.getBoundingClientRect().left || 0);
            const y = e.clientY - (canvas.getBoundingClientRect().top || 0);
            const sy = (canvas.height || 0) - y;
            const ray = new Float32Array(6);
            camera.screenToWorldRay(ray, new Float32Array([x, sy]), vp);
            const ro: [number, number, number] = [ray[0], ray[1], ray[2]];
            const rf: [number, number, number] = [ray[3], ray[4], ray[5]];
            const rd: [number, number, number] = [rf[0] - ro[0], rf[1] - ro[1], rf[2] - ro[2]];
            const idx = pickFace([ro[0], ro[1], ro[2]], [rd[0], rd[1], rd[2]]);
            if (idx !== hoveredFaceIdxRef.current) {
              hoveredFaceIdxRef.current = idx;
              setFaceHighlight(idx);
            }
          } catch { /* noop */ }
        }
        if (!isDragging || isTouch) return;
        // Cut Box dragging takes precedence over camera controls
        if (isDraggingCutHandle && draggingCutHandleRef.current && canvas) {
          const vp = scene.viewport;
          const x = e.clientX - (canvas.getBoundingClientRect().left || 0);
          const y = e.clientY - (canvas.getBoundingClientRect().top || 0);
          const sy = (canvas.height || 0) - y;
          const ray = new Float32Array(6);
          try {
            camera.screenToWorldRay(ray, new Float32Array([x, sy]), vp);
            const ro: [number, number, number] = [ray[0], ray[1], ray[2]];
            const rf: [number, number, number] = [ray[3], ray[4], ray[5]];
            const rd: [number, number, number] = [rf[0] - ro[0], rf[1] - ro[1], rf[2] - ro[2]];
            // Dynamic camera-facing plane through current face center
            const axis = draggingCutHandleRef.current.axis;
            const side = draggingCutHandleRef.current.side;
            const camNormal: [number, number, number] = [camera.directionZ[0], camera.directionZ[1], camera.directionZ[2]];
            const [minX, minY, minZ] = cutBoxRef.current.min;
            const [maxX, maxY, maxZ] = cutBoxRef.current.max;
            const cx = (minX + maxX) / 2;
            const cy = (minY + maxY) / 2;
            const cz = (minZ + maxZ) / 2;
            const faceCenter: [number, number, number] = axis === 0
              ? [(side === 'min' ? minX : maxX), cy, cz]
              : axis === 1
                ? [cx, (side === 'min' ? minY : maxY), cz]
                : [cx, cy, (side === 'min' ? minZ : maxZ)];
            const denom = camNormal[0] * rd[0] + camNormal[1] * rd[1] + camNormal[2] * rd[2];
            if (Math.abs(denom) > 1e-6) {
              const planeW = camNormal[0] * faceCenter[0] + camNormal[1] * faceCenter[1] + camNormal[2] * faceCenter[2];
              const t = (planeW - (camNormal[0] * ro[0] + camNormal[1] * ro[1] + camNormal[2] * ro[2])) / denom;
              const hitPoint: [number, number, number] = [ro[0] + rd[0] * t, ro[1] + rd[1] * t, ro[2] + rd[2] * t];
              const desiredCoord = hitPoint[axis] - draggingCutHandleRef.current.grabOffset;
              const eps = 1e-4;
              const nextMin: [number, number, number] = [minX, minY, minZ];
              const nextMax: [number, number, number] = [maxX, maxY, maxZ];
              if (side === 'min') {
                nextMin[axis] = Math.min(desiredCoord, nextMax[axis] - eps);
              } else {
                nextMax[axis] = Math.max(desiredCoord, nextMin[axis] + eps);
              }
              setCutBox((prev) => ({
                visible: prev.visible,
                min: nextMin,
                max: nextMax,
              }));
            }
          } catch {
            // ignore
          }
          return;
        }
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
        if (!leftDown) {
          isDraggingCutHandle = false;
          draggingCutHandleRef.current = null;
        }
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
      try { if (typeof cutSyncInterval === 'number') clearInterval(cutSyncInterval); } catch { /* noop */ }
      viewer.resources.forEach((resource) => viewer.unload(resource));

      try {
        cutBoxInstanceRef.current = null;
        cutHandleInstancesRef.current = [];
        cutFaceInstancesRef.current = [];
      } catch { /* noop */ }

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
  }, [modelPath, canvasRef.current, viewer]);

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
    const scene = sceneRef.current;
    const model = modelRef.current;
    const canvas = canvasRef.current;
    if (!scene || !model || !canvas) return;
    const cams = model.cameras;
    const cam = cams[idx];
    if (!cam) return;
    const width = canvas.width || canvas.clientWidth || 1;
    const height = canvas.height || canvas.clientHeight || 1;
    const aspect = width / Math.max(1, height);
    try {
      scene.camera.perspective(
        cam.fieldOfView,
        aspect,
        cam.nearClippingPlane,
        cam.farClippingPlane,
      );
      const from = vec3.fromValues(cam.position[0], cam.position[1], cam.position[2]);
      const to = vec3.fromValues(cam.targetPosition[0], cam.targetPosition[1], cam.targetPosition[2]);
      scene.camera.moveToAndFace(from, to, [0, 0, 1]);
      setCurrentCamera(idx);
    } catch (e) {
      console.error('Failed to set camera', e);
    }
  };

  const handleToggleCutBox = () => {
    setCutBox((prev) => ({ ...prev, visible: !prev.visible }));
    // Instances visibility will be toggled by the render loop sync
  };
  const handleCopyCutCode = async () => {
    const m = cutBox.min.map((v) => Number(v.toFixed(2)));
    const M = cutBox.max.map((v) => Number(v.toFixed(2)));
    const code = `model.modify.deleteVerticesInsideBox([${m[0]}, ${m[1]}, ${m[2]}], [${M[0]}, ${M[1]}, ${M[2]}]);\n`;
    try {
      await navigator.clipboard.writeText(code);
      setCopiedCutCode(true);
      setTimeout(() => setCopiedCutCode(false), 2000);
    } catch {
      // ignore
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
            onClick={handleToggleCutBox}
            className="w-10 h-10 text-2xl bg-[hsl(var(--viewer-control-bg))] text-[hsl(var(--viewer-sidebar-fg))] border border-[hsl(var(--viewer-divider))] hover:bg-[hsl(var(--viewer-item-hover))] focus:outline-none"
          >
            <TooltipHelp
              trigger={<span>{cutBox.visible ? '✂□' : '□'}</span>}
              tooltips={cutBox.visible ? 'Disable Cut Box' : 'Enable Cut Box'} asChild
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
      <div className={'lg:w-60 w-full lg:h-full h-[200px] min-h-[200px] bg-[hsl(var(--viewer-sidebar-bg))] text-[hsl(var(--viewer-sidebar-fg))] lg:border-l lg:border-t-0 border-t border-[hsl(var(--viewer-divider))] flex-shrink-0 relative z-10 flex flex-col'}>
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
          {cutBox.visible && <><div className="px-3 py-2 font-semibold bg-[hsl(var(--viewer-item-active))] text-[hsl(var(--viewer-sidebar-fg))] border-b border-[hsl(var(--viewer-divider))] lg:sticky lg:top-[60px] lg:z-10">
            Cut Box
          </div>
          <div className="p-3 space-y-3 border-b border-[hsl(var(--viewer-divider))]">
            <div className="flex gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => void handleCopyCutCode()}
                className="flex-1 bg-[hsl(var(--viewer-control-bg))] text-[hsl(var(--viewer-sidebar-fg))] border border-[hsl(var(--viewer-divider))] hover:bg-[hsl(var(--viewer-item-hover))]"
              >
                {copiedCutCode ? 'Copied' : 'Copy Code'}
              </Button>
            </div>
          </div></>}
        </div>
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

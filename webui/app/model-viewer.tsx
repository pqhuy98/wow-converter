"use client"

import { useEffect, useRef, useState } from "react"
import { vec3 } from "gl-matrix";
import ModelViewer from "@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/viewer";
import mdxHandler from "@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/handler";
import MdxModel from "@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/model";
import blpHandler from "@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/blp/handler";
import { host } from "./config";

interface ModelViewerProps {
  /**
   * Relative path (within the exporter output directory) to the MDX|MDL model.
   * Example: "highlord-darion-mograine.mdx"
   */
  modelPath?: string
}

// Normalises backslashes to forward slashes for safe URL usage
const normalizePath = (p: string) => p.replace(/\\+/g, "/").replace(/\/+/, "/")

export default function ModelViewerUi({ modelPath }: ModelViewerProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const [sequences, setSequences] = useState<string[]>([])
  const [currentSeq, setCurrentSeq] = useState<number>(0)
  const instanceRef = useRef<any>(null)
  const targetRef = useRef(vec3.fromValues(0,0,0));
  const vecHeap = vec3.create();

  useEffect(() => {
    if (!modelPath || !canvasRef.current) return

    let viewer: ModelViewer
    let animationFrameId: number
    let lastTime = performance.now();

    // references for cleanup
    let canvasEl: HTMLCanvasElement | null = null;
    let onMouseDown: ((e: MouseEvent) => void) | null = null;
    let onMouseMove: ((e: MouseEvent) => void) | null = null;
    let endDrag: (() => void) | null = null;
    let onWheel: ((e: WheelEvent) => void) | null = null;
    // Orbit camera state
    let isDragging = false;
    let lastX = 0;
    let lastY = 0;
    let horizontalAngle = 0
    let verticalAngle = Math.PI / 4
    let distance = 1000;

    (async () => {
      // Canvas & viewer setup
      const canvas = canvasRef.current!
      viewer = new ModelViewer(canvas)

      // Register handlers so the viewer knows how to parse MDX and BLP
      viewer.addHandler(mdxHandler)
      viewer.addHandler(blpHandler)

      // Path solver so the viewer fetches every dependant file via our /asset route
      const pathSolver = (src: unknown) => {
        return `${host}/assets/${normalizePath(src as string)}`
        }

      // Load the model (assumed to be in MDX|MDL format)
      const model = await viewer.load(`${normalizePath(modelPath)}`, pathSolver, {reforged: true, hd: true})
      if (!(model instanceof MdxModel)) return
      model.materials.forEach(m => { m.shader = 'Shader_HD_DefaultUnit'; });
      model.hd = true

      const scene = viewer.addScene()
      const camera = scene.camera;
      scene.color.fill(0.15);

      // Utility to update camera position from spherical coords
      const target = targetRef.current;
      distance = model.bounds.r * 3;
      target[2] = model.bounds.z;
      console.log(model.bounds, target)

      const updateCamera = () => {
        const x = distance * Math.cos(verticalAngle) * Math.cos(horizontalAngle);
        const y = distance * Math.cos(verticalAngle) * Math.sin(horizontalAngle);
        const z = distance * Math.sin(verticalAngle);
        const camPos = vec3.fromValues(target[0]+x, target[1]+y, target[2]+z);
        camera.moveToAndFace(camPos, target, [0, 0, 1]);
      };

      updateCamera();

      const instance = model.addInstance()
      scene.addInstance(instance)

      // expose sequences
      setSequences(model.sequences.map((s) => s.name || `Sequence ${model.sequences.indexOf(s)}`))

      // init animation
      instance.setSequence(0)
      instance.sequenceLoopMode = 2 // always loop
      instanceRef.current = instance

      // Start render loop
      const step = () => {
        const now = performance.now();
        const dt = now - lastTime;
        lastTime = now;
        viewer.updateAndRender(dt);
        animationFrameId = requestAnimationFrame(step);
      };
      step();

      // ----------------------------
      // Mouse & wheel controls
      // ----------------------------
      canvasEl = canvas;
      // ensure canvas matches element size
      const resizeCanvas = () => {
        if (canvasEl) {
          const width = canvasEl.clientWidth;
          const height = canvasEl.clientHeight;
          canvasEl.width = width;
          canvasEl.height = height;
          scene.viewport[2] = width;
          scene.viewport[3] = height;
          camera.perspective(
            camera.fov,
            width / height,
            camera.nearClipPlane,
            camera.farClipPlane
          );
        }
      };
      resizeCanvas();
      window.addEventListener('resize', resizeCanvas);

      let button = 0
      onMouseDown = (e: MouseEvent) => {
        e.preventDefault()
        button = e.button;
        if (e.button === 0 || e.button === 2) {
          isDragging = true;
          lastX = e.clientX;
          lastY = e.clientY;
        }
      };

      onMouseMove = (e: MouseEvent) => {
        if (!isDragging) return;
        // left click to rotate, right click to move target
        if (button === 0) {
          const dx = e.clientX - lastX;
          const dy = e.clientY - lastY;
          lastX = e.clientX;
          lastY = e.clientY;
          const ROT_SPEED = Math.PI / 180; // radians per pixel
          horizontalAngle -= dx * ROT_SPEED;
          verticalAngle += dy * ROT_SPEED;
          verticalAngle = Math.max(-Math.PI / 2, Math.min(Math.PI / 2, verticalAngle));
        } else if (button === 2) {
          // move target
          
          const dx = e.clientX - lastX;
          const dy = e.clientY - lastY;
          lastX = e.clientX;
          lastY = e.clientY;

          const dirX = camera.directionX;
          const dirY = camera.directionY;
          if (!canvasEl) return;
          const w = canvasEl.width;
          const h2 = canvasEl.height;
          const sw = -dx / w * distance;
          const sh = dy / h2 * distance;
          vec3.add(target, target,
            vec3.scale(vecHeap, vec3.normalize(vecHeap, vec3.set(vecHeap, dirX[0], dirX[1], 0)), sw));
          vec3.add(target, target,
            vec3.scale(vecHeap, vec3.normalize(vecHeap, vec3.set(vecHeap, dirY[0], dirY[1], 0)), sh));
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

      if (canvasEl) {
        canvasEl.addEventListener('mousedown', onMouseDown);
        canvasEl.addEventListener('contextmenu', (e)=> e.preventDefault());
      }
      
      window.addEventListener('mousemove', onMouseMove!);
      window.addEventListener('mouseup', endDrag!);
      canvasEl?.addEventListener('wheel', onWheel!, { passive: false });

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

      // cleanup to remove
      return () => {
        if (animationFrameId) cancelAnimationFrame(animationFrameId);
        if (canvasEl) {
         if (onMouseDown) canvasEl.removeEventListener('mousedown', onMouseDown);
         if (onWheel) canvasEl.removeEventListener('wheel', onWheel);
       }
       window.removeEventListener('resize', resizeCanvas);
        if (onMouseMove) window.removeEventListener('mousemove', onMouseMove);
        if (endDrag) window.removeEventListener('mouseup', endDrag);
      }
    })()

  }, [modelPath])

  // Apply sequence when currentSeq updates
  useEffect(() => {
    const inst = instanceRef.current
    if (inst) {
      inst.setSequence(currentSeq)
      inst.sequenceLoopMode = 2
    }
  }, [currentSeq])

  return (
    <div className="flex w-full">
      <canvas
        ref={canvasRef}
        width={1}
        height={1}
        className="flex-grow bg-gray-800 shadow-inner h-[600px] w-full"
      />

      <div className="w-60 h-[600px] overflow-y-auto bg-gray-800/90 border-l border-gray-600">
          <div className="sticky top-0 z-10 bg-gray-900 px-3 py-2 text-white font-semibold border-b border-gray-700">
          Animations
        </div>
        <ul className="divide-y divide-gray-700">
          {sequences.length === 0 ? (
            <div className="p-3 text-gray-400">Loading...</div>
          ) : (
            sequences.map((name, idx) => (
            <li
              key={idx}
              onClick={() => setCurrentSeq(idx)}
              className={`px-3 py-2 cursor-pointer hover:bg-gray-700 ${idx === currentSeq ? 'bg-gray-700 text-white' : 'text-gray-200'}`}
            >
              {name || `Sequence ${idx}`}
            </li>
          ))
          )}
        </ul>
      </div>
    </div>
  )
} 
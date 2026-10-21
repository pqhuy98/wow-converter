import { mat4, vec3 } from 'gl-matrix';

import type { GenerateWc3FormValues } from '@/lib/models/map-generate.model';
import type {
  MapPreviewData, MapPreviewInstance, MapPreviewPart, MapPreviewProgress,
} from '@/lib/models/map-preview.model';
import { previewIsWater, previewNPCPlacement, previewWaterRange } from '@/lib/models/map-preview.model';

interface Mesh {
  vao: WebGLVertexArrayObject;
  buffer: WebGLBuffer;
  vertices: number;
}

export type MapPreviewHit =
  | { kind: 'npc'; name: string; placement: ReturnType<typeof previewNPCPlacement> }
  | { kind: 'terrain'; playable: boolean };

const VERTEX_SHADER = `#version 300 es
precision highp float;
precision highp int;
layout(location=0) in vec3 a_position;
layout(location=1) in vec3 a_normal;
layout(location=2) in vec2 a_uv;
layout(location=3) in vec4 a_marker;
layout(location=4) in vec3 a_color;
uniform mat4 u_viewProjection;
uniform mat4 u_model;
uniform vec3 u_center;
uniform vec2 u_clamp;
uniform float u_angle;
uniform bool u_terrain;
uniform bool u_wireframe;
uniform bool u_markers;
uniform highp int u_water;
flat out int v_pickId;
out vec3 v_normal;
out vec2 v_uv;
out vec3 v_color;
out float v_height;
out vec3 v_barycentric;
void main() {
  vec3 world = u_markers ? a_marker.xyz+a_position*a_marker.w : (u_model*vec4(a_position,1.0)).xyz;
  v_height=world.z;
  v_pickId=gl_InstanceID+3;
  int corner=gl_VertexID%3;
  v_barycentric=vec3(corner==0 ? 1.0 : 0.0,corner==1 ? 1.0 : 0.0,corner==2 ? 1.0 : 0.0);
  if (u_terrain && !u_wireframe) world.z=clamp(world.z,u_clamp.x,u_clamp.y);
  if (u_water==2) world.z=clamp(world.z,u_clamp.x,u_clamp.y);
  world-=u_center;
  mat2 rotation=mat2(cos(u_angle),sin(u_angle),-sin(u_angle),cos(u_angle));
  world.xy=rotation*world.xy;
  vec3 normal=u_markers ? a_normal : mat3(u_model)*a_normal;
  normal.xy=rotation*normal.xy;
  v_normal=normalize(normal); v_uv=a_uv; v_color=a_color;
  gl_Position=u_viewProjection*vec4(world,1.0);
}`;

const FRAGMENT_SHADER = `#version 300 es
precision highp float;
precision highp int;
uniform sampler2D u_texture;
uniform bool u_picking;
flat in int v_pickId;
uniform bool u_textured;
uniform bool u_markers;
uniform bool u_terrain;
uniform bool u_wireframe;
uniform vec2 u_clamp;
uniform highp int u_water;
uniform vec2 u_waterRange;
in vec3 v_normal;
in vec2 v_uv;
in vec3 v_color;
in float v_height;
in vec3 v_barycentric;
out vec4 color;
void main() {
  bool outside=v_height<u_clamp.x || v_height>u_clamp.y;
  if (u_picking) {
    if (u_textured && !(u_terrain && outside) && texture(u_texture,v_uv).a<0.25) discard;
    int id=u_markers ? v_pickId : (u_terrain && outside ? 2 : 1);
    color=vec4(float(id&255),float((id>>8)&255),float((id>>16)&255),255.0)/255.0;
    return;
  }
  if (u_wireframe) {
    if (!outside) discard;
    vec3 edge=smoothstep(vec3(0.0),fwidth(v_barycentric)*1.2,v_barycentric);
    float alpha=1.0-min(min(edge.x,edge.y),edge.z);
    if (alpha<0.05) discard;
    color=vec4(0.8,0.85,0.9,alpha*0.8);
    return;
  }
  if (u_water>0) {
    bool fits=v_height>=u_waterRange.x && v_height<=u_waterRange.y;
    if ((u_water==1 && !fits) || (u_water==2 && fits)) discard;
    color=u_water==1 ? vec4(0.08,0.55,0.95,0.65) : vec4(1.0,0.45,0.08,0.4);
    return;
  }
  if (u_terrain && outside) {
    color=vec4(140.0/255.0,101.0/255.0,44.0/255.0,1.0);
    return;
  }
  vec4 base=u_textured ? texture(u_texture,v_uv) : vec4(0.58,0.63,0.69,1.0);
  if (base.a<0.25) discard;
  if (u_markers) base=vec4(v_color,1.0);
  float light=0.45+0.55*abs(dot(normalize(v_normal),normalize(vec3(-0.3,-0.4,0.85))));
  color=vec4(base.rgb*light,1.0);
}`;

/** Static map renderer. Camera gestures are copied from ModelViewerUi, with
 * animation, model inspection, emitters, downloads and continuous updates removed. */
export class MapExportPreviewViewer {
  private readonly gl: WebGL2RenderingContext;

  private readonly program: WebGLProgram;

  private readonly uniforms = new Map<string, WebGLUniformLocation | null>();

  private readonly meshes = new Map<string, Mesh>();

  private readonly textures = new Map<string, WebGLTexture>();

  private readonly defaultTexture: WebGLTexture;

  private readonly sphere: Mesh;

  private readonly markerBuffer: WebGLBuffer;

  private readonly observer: ResizeObserver;

  private scene: MapPreviewData | null = null;

  private form: GenerateWc3FormValues | null = null;

  private controller = new AbortController();

  private disposed = false;

  private frame = 0;

  private markerCount = 0;

  private pickTarget: { framebuffer: WebGLFramebuffer; texture: WebGLTexture; depth: WebGLRenderbuffer } | null = null;

  private target = vec3.create();

  private horizontalAngle = -Math.PI / 2;

  private verticalAngle = Math.PI / 6;

  private distance = 500;

  private view = mat4.create();

  private projection = mat4.create();

  private viewProjection = mat4.create();

  private lastX = 0;

  private lastY = 0;

  private buttons = [false, false, false];

  private touching = false;

  private touchDistance = 0;

  constructor(private readonly canvas: HTMLCanvasElement) {
    const gl = canvas.getContext('webgl2', { alpha: false, antialias: true, preserveDrawingBuffer: false });
    if (!gl) throw new Error('WebGL 2 is required for the map preview.');
    this.gl = gl;
    this.program = this.createProgram();
    const defaultTexture = gl.createTexture();
    if (!defaultTexture) throw new Error('Could not allocate the preview texture.');
    this.defaultTexture = defaultTexture;
    gl.bindTexture(gl.TEXTURE_2D, defaultTexture);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([255, 255, 255, 255]));
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    for (const name of ['viewProjection', 'model', 'center', 'clamp', 'angle', 'terrain', 'wireframe', 'markers', 'textured', 'texture', 'water', 'waterRange', 'picking']) {
      this.uniforms.set(name, gl.getUniformLocation(this.program, `u_${name}`));
    }
    this.sphere = this.createMesh(this.sphereVertices());
    const markerBuffer = gl.createBuffer();
    if (!markerBuffer) throw new Error('Could not allocate NPC markers.');
    this.markerBuffer = markerBuffer;
    gl.bindVertexArray(this.sphere.vao);
    gl.bindBuffer(gl.ARRAY_BUFFER, markerBuffer);
    gl.enableVertexAttribArray(3); gl.vertexAttribPointer(3, 4, gl.FLOAT, false, 28, 0); gl.vertexAttribDivisor(3, 1);
    gl.enableVertexAttribArray(4); gl.vertexAttribPointer(4, 3, gl.FLOAT, false, 28, 16); gl.vertexAttribDivisor(4, 1);
    gl.bindVertexArray(null);
    gl.enable(gl.DEPTH_TEST);
    this.observer = new ResizeObserver(() => this.requestRender());
    this.observer.observe(canvas);
    canvas.addEventListener('mousedown', this.onMouseDown);
    canvas.addEventListener('wheel', this.onWheel, { passive: false });
    canvas.addEventListener('contextmenu', this.preventDefault);
    canvas.addEventListener('touchstart', this.onTouchStart, { passive: false });
    canvas.addEventListener('touchmove', this.onTouchMove, { passive: false });
    canvas.addEventListener('touchend', this.onTouchEnd, { passive: false });
    canvas.addEventListener('touchcancel', this.onTouchEnd, { passive: false });
    window.addEventListener('mousemove', this.onMouseMove);
    window.addEventListener('mouseup', this.onMouseUp);
    window.addEventListener('blur', this.resetButtons);
    this.requestRender();
  }

  async setScene(scene: MapPreviewData | null, onProgress?: (progress: MapPreviewProgress) => void): Promise<void> {
    this.controller.abort();
    this.controller = new AbortController();
    const { signal } = this.controller;
    this.scene = scene;
    this.markerCount = 0;
    this.requestRender();
    if (!scene) return;
    const usedMeshes = new Set(scene.assets.flatMap((asset) => asset.parts.map((part) => part.mesh)));
    const usedTextures = new Set(scene.assets.flatMap((asset) => asset.parts.map((part) => part.texture)));
    for (const [path, mesh] of this.meshes) {
      if (!usedMeshes.has(path)) {
        this.gl.deleteBuffer(mesh.buffer); this.gl.deleteVertexArray(mesh.vao); this.meshes.delete(path);
      }
    }
    for (const [path, texture] of this.textures) {
      if (!usedTextures.has(path)) {
        this.gl.deleteTexture(texture); this.textures.delete(path);
      }
    }
    this.fitCamera();
    this.updateMarkers();
    const parts = scene.assets.flatMap((asset) => asset.parts);
    const textureLoads = new Map<string, Promise<void>>();
    let next = 0;
    let completed = 0;
    onProgress?.({ message: 'Loading preview assets', completed, total: parts.length });
    await Promise.all(Array.from({ length: Math.min(4, parts.length) }, async () => {
      while (next < parts.length && !signal.aborted) {
        const part = parts[next++];
        await this.loadPart(part, signal, textureLoads);
        if (!signal.aborted) {
          onProgress?.({ message: 'Loading preview assets', completed: ++completed, total: parts.length });
          this.requestRender();
        }
      }
    }));
    if (!signal.aborted) this.requestRender();
  }

  setOptions(form: GenerateWc3FormValues): void {
    this.form = form;
    this.updateMarkers();
    this.requestRender();
  }

  fitCamera(): void {
    if (!this.scene) return;
    this.target = vec3.create();
    // Geometry already has the export axis conversion (-WoW Y, WoW X).
    // View north along +Y, like WC3, rather than along -X.
    this.horizontalAngle = -Math.PI / 2;
    this.verticalAngle = Math.PI / 6;
    const scene = this.scene;
    const size = scene.max.map((v, i) => v - scene.min[i]);
    const aspect = Math.min(1, Math.max(1, this.canvas.clientWidth) / Math.max(1, this.canvas.clientHeight));
    const halfFov = Math.atan(Math.tan(Math.PI / 8) * aspect);
    this.distance = Math.max(10, Math.hypot(...size) / 2 / Math.sin(halfFov) * 1.1);
    this.requestRender();
  }

  /** Resolve only the hovered pixel on the GPU, using the same transforms,
   * clamped triangles and sphere instances as the visible render. */
  pick(x: number, y: number): MapPreviewHit | null {
    const scene = this.scene; const form = this.form; const gl = this.gl;
    if (this.disposed || !scene || !form || x < 0 || y < 0 || x >= this.canvas.clientWidth || y >= this.canvas.clientHeight) return null;
    if (this.frame) {
      cancelAnimationFrame(this.frame); this.frame = 0; this.render();
    }
    if (!this.pickTarget) {
      const framebuffer = gl.createFramebuffer(); const texture = gl.createTexture(); const depth = gl.createRenderbuffer();
      if (!framebuffer || !texture || !depth) {
        gl.deleteFramebuffer(framebuffer); gl.deleteTexture(texture); gl.deleteRenderbuffer(depth);
        return null;
      }
      gl.bindTexture(gl.TEXTURE_2D, texture);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
      gl.bindRenderbuffer(gl.RENDERBUFFER, depth);
      gl.renderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT16, 1, 1);
      gl.bindFramebuffer(gl.FRAMEBUFFER, framebuffer);
      gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, texture, 0);
      gl.framebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, depth);
      const complete = gl.checkFramebufferStatus(gl.FRAMEBUFFER) === gl.FRAMEBUFFER_COMPLETE;
      gl.bindRenderbuffer(gl.RENDERBUFFER, null); gl.bindFramebuffer(gl.FRAMEBUFFER, null);
      if (!complete) {
        gl.deleteFramebuffer(framebuffer); gl.deleteTexture(texture); gl.deleteRenderbuffer(depth);
        return null;
      }
      this.pickTarget = { framebuffer, texture, depth };
    }
    const pixel = new Uint8Array(4);
    gl.bindFramebuffer(gl.FRAMEBUFFER, this.pickTarget.framebuffer);
    // Offset a normal-sized viewport onto a 1x1 target. The GPU rasterizes only
    // the cursor pixel; no CPU mesh copies, ray scans or full-size buffers.
    gl.viewport(
      -Math.floor(x * this.canvas.width / this.canvas.clientWidth),
      -(this.canvas.height - 1 - Math.floor(y * this.canvas.height / this.canvas.clientHeight)),
      this.canvas.width,
      this.canvas.height,
    );
    gl.disable(gl.BLEND); gl.disable(gl.DITHER); gl.enable(gl.DEPTH_TEST); gl.depthMask(true);
    gl.clearColor(0, 0, 0, 0); gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    gl.useProgram(this.program);
    gl.activeTexture(gl.TEXTURE0); gl.bindTexture(gl.TEXTURE_2D, this.defaultTexture);
    gl.uniform1i(this.uniform('picking'), 1); gl.uniform1i(this.uniform('markers'), 0);
    gl.uniform1i(this.uniform('wireframe'), 0); gl.uniform1i(this.uniform('water'), 0);
    try {
      const assets = new Map(scene.assets.map((asset) => [asset.id, asset]));
      for (const instance of scene.instances) this.drawGeometry(instance, assets.get(instance.asset)?.parts ?? []);
      if (this.markerCount) {
        gl.uniform1i(this.uniform('markers'), 1); gl.uniform1i(this.uniform('terrain'), 0); gl.uniform1i(this.uniform('textured'), 0);
        gl.bindVertexArray(this.sphere.vao);
        gl.drawArraysInstanced(gl.TRIANGLES, 0, this.sphere.vertices, this.markerCount);
      }
      gl.readPixels(0, 0, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, pixel);
    } finally {
      gl.uniform1i(this.uniform('picking'), 0); gl.uniform1i(this.uniform('markers'), 0);
      gl.bindVertexArray(null); gl.bindFramebuffer(gl.FRAMEBUFFER, null);
      gl.viewport(0, 0, this.canvas.width, this.canvas.height); gl.enable(gl.DITHER);
    }
    const id = pixel[0] + pixel[1] * 256 + pixel[2] * 65536;
    if (id === 1) return { kind: 'terrain', playable: true };
    if (id === 2) return { kind: 'terrain', playable: false };
    const npc = id >= 3 ? scene.npcs[id - 3] : null;
    return npc ? { kind: 'npc', name: npc.name || `#${npc.guid}`, placement: previewNPCPlacement(npc, scene, form) } : null;
  }

  dispose(): void {
    this.disposed = true;
    this.controller.abort();
    this.observer.disconnect();
    if (this.frame) cancelAnimationFrame(this.frame);
    this.canvas.removeEventListener('mousedown', this.onMouseDown);
    this.canvas.removeEventListener('wheel', this.onWheel);
    this.canvas.removeEventListener('contextmenu', this.preventDefault);
    this.canvas.removeEventListener('touchstart', this.onTouchStart);
    this.canvas.removeEventListener('touchmove', this.onTouchMove);
    this.canvas.removeEventListener('touchend', this.onTouchEnd);
    this.canvas.removeEventListener('touchcancel', this.onTouchEnd);
    window.removeEventListener('mousemove', this.onMouseMove);
    window.removeEventListener('mouseup', this.onMouseUp);
    window.removeEventListener('blur', this.resetButtons);
    for (const mesh of [...this.meshes.values(), this.sphere]) {
      this.gl.deleteBuffer(mesh.buffer); this.gl.deleteVertexArray(mesh.vao);
    }
    for (const texture of this.textures.values()) this.gl.deleteTexture(texture);
    this.gl.deleteTexture(this.defaultTexture);
    this.gl.deleteBuffer(this.markerBuffer);
    if (this.pickTarget) {
      this.gl.deleteFramebuffer(this.pickTarget.framebuffer);
      this.gl.deleteTexture(this.pickTarget.texture);
      this.gl.deleteRenderbuffer(this.pickTarget.depth);
    }
    this.gl.deleteProgram(this.program);
    this.gl.getExtension('WEBGL_lose_context')?.loseContext();
  }

  private uniform(name: string): WebGLUniformLocation | null { return this.uniforms.get(name) ?? null; }

  private createProgram(): WebGLProgram {
    const gl = this.gl;
    const program = gl.createProgram();
    if (!program) throw new Error('Could not allocate map preview shaders.');
    for (const [type, source] of [[gl.VERTEX_SHADER, VERTEX_SHADER], [gl.FRAGMENT_SHADER, FRAGMENT_SHADER]] as const) {
      const shader = gl.createShader(type);
      if (!shader) throw new Error('Could not allocate map preview shader.');
      gl.shaderSource(shader, source); gl.compileShader(shader);
      if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
        const error = gl.getShaderInfoLog(shader); gl.deleteShader(shader); gl.deleteProgram(program);
        throw new Error(error ?? 'Could not compile map preview shader.');
      }
      gl.attachShader(program, shader); gl.deleteShader(shader);
    }
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) throw new Error(gl.getProgramInfoLog(program) ?? 'Could not link map preview shaders.');
    return program;
  }

  private createMesh(vertices: Float32Array): Mesh {
    const gl = this.gl;
    const vao = gl.createVertexArray(); const buffer = gl.createBuffer();
    if (!vao || !buffer) throw new Error('Could not allocate map preview geometry.');
    gl.bindVertexArray(vao); gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
    gl.bufferData(gl.ARRAY_BUFFER, vertices, gl.STATIC_DRAW);
    for (const [index, count, offset] of [[0, 3, 0], [1, 3, 12], [2, 2, 24]]) {
      gl.enableVertexAttribArray(index); gl.vertexAttribPointer(index, count, gl.FLOAT, false, 32, offset);
    }
    gl.bindVertexArray(null);
    return { vao, buffer, vertices: vertices.length / 8 };
  }

  private assetUrl(path: string): string { return `/api/browse-assets/${path.split('/').map(encodeURIComponent).join('/')}`; }

  private async loadPart(part: MapPreviewPart, signal: AbortSignal, textureLoads: Map<string, Promise<void>>): Promise<void> {
    if (!this.meshes.has(part.mesh)) {
      const response = await fetch(this.assetUrl(part.mesh), { signal });
      if (!response.ok) throw new Error('Could not load a map preview model.');
      const bytes = await response.arrayBuffer();
      if (signal.aborted) return;
      if (bytes.byteLength !== part.vertices * 32) throw new Error('Incomplete map preview model; reopen the dialog to retry.');
      this.meshes.set(part.mesh, this.createMesh(new Float32Array(bytes)));
    }
    if (part.texture && !this.textures.has(part.texture)) {
      let pending = textureLoads.get(part.texture);
      if (!pending) {
        pending = this.loadTexture(part.texture, signal);
        textureLoads.set(part.texture, pending);
      }
      await pending;
    }
  }

  private async loadTexture(path: string, signal: AbortSignal): Promise<void> {
    const response = await fetch(this.assetUrl(path), { signal });
    if (!response.ok) throw new Error('Could not load a map preview texture.');
    const bitmap = await createImageBitmap(await response.blob());
    if (signal.aborted) { bitmap.close(); return; }
    const gl = this.gl; const texture = gl.createTexture();
    if (!texture) { bitmap.close(); throw new Error('Could not allocate a map preview texture.'); }
    gl.bindTexture(gl.TEXTURE_2D, texture);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, bitmap);
    gl.generateMipmap(gl.TEXTURE_2D);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    bitmap.close(); this.textures.set(path, texture);
  }

  private updateMarkers(): void {
    const scene = this.scene; const form = this.form;
    this.markerCount = 0;
    if (!scene || !form || !form.creatures.enable) return;
    const radius = Math.max(0.8, (scene.max[0] - scene.min[0]) / 900) * form.unitScale;
    const data = new Float32Array(scene.npcs.length * 7);
    scene.npcs.forEach((npc, i) => {
      const placement = previewNPCPlacement(npc, scene, form);
      const color = placement === 'unit' ? [0.15, 1, 0.15] : placement === 'destructible' ? [1, 0.85, 0.1] : [1, 0.12, 0.15];
      data.set([npc.position[0], npc.position[1], npc.position[2] + radius, radius, ...color], i * 7);
    });
    this.gl.bindBuffer(this.gl.ARRAY_BUFFER, this.markerBuffer);
    this.gl.bufferData(this.gl.ARRAY_BUFFER, data, this.gl.DYNAMIC_DRAW);
    this.markerCount = scene.npcs.length;
  }

  private requestRender(): void {
    if (this.disposed || this.frame) return;
    this.frame = requestAnimationFrame(() => { this.frame = 0; this.render(); });
  }

  private drawGeometry(instance: MapPreviewInstance, parts: MapPreviewPart[], wireframe = false): void {
    const gl = this.gl;
    gl.uniformMatrix4fv(this.uniform('model'), false, new Float32Array(instance.transform));
    gl.uniform1i(this.uniform('terrain'), instance.kind === 'adt' || instance.root ? 1 : 0);
    for (const part of parts) {
      if (part.liquid) continue;
      const mesh = this.meshes.get(part.mesh); if (!mesh) continue;
      const texture = !wireframe && part.texture ? this.textures.get(part.texture) : null;
      gl.uniform1i(this.uniform('textured'), texture ? 1 : 0);
      gl.activeTexture(gl.TEXTURE0); gl.bindTexture(gl.TEXTURE_2D, texture ?? this.defaultTexture);
      gl.bindVertexArray(mesh.vao); gl.drawArrays(gl.TRIANGLES, 0, mesh.vertices);
    }
  }

  private render(): void {
    const gl = this.gl; const { canvas } = this;
    const width = Math.max(1, canvas.clientWidth); const height = Math.max(1, canvas.clientHeight);
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    if (canvas.width !== Math.round(width * ratio) || canvas.height !== Math.round(height * ratio)) {
      canvas.width = Math.round(width * ratio); canvas.height = Math.round(height * ratio);
    }
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.clearColor(0.055, 0.075, 0.095, 1); gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
    const scene = this.scene; const form = this.form;
    if (!scene || !form) return;
    const x = this.distance * Math.cos(this.verticalAngle) * Math.cos(this.horizontalAngle);
    const y = this.distance * Math.cos(this.verticalAngle) * Math.sin(this.horizontalAngle);
    const z = this.distance * Math.sin(this.verticalAngle);
    mat4.lookAt(this.view, vec3.fromValues(this.target[0] + x, this.target[1] + y, this.target[2] + z), this.target, [0, 0, 1]);
    mat4.perspective(this.projection, Math.PI / 4, width / height, Math.max(0.01, this.distance / 100), Math.max(10000, this.distance * 20));
    mat4.multiply(this.viewProjection, this.projection, this.view);
    gl.useProgram(this.program);
    gl.uniformMatrix4fv(this.uniform('viewProjection'), false, this.viewProjection);
    gl.uniform3f(this.uniform('center'), (scene.min[0] + scene.max[0]) / 2, (scene.min[1] + scene.max[1]) / 2, (scene.min[2] + scene.max[2]) / 2);
    const span = scene.max[2] - scene.min[2];
    gl.uniform2f(this.uniform('clamp'), scene.min[2] + span * form.clampLower, scene.min[2] + span * form.clampUpper);
    gl.uniform1f(this.uniform('angle'), form.mapAngleDeg * Math.PI / 180);
    gl.uniform1i(this.uniform('picking'), 0);
    gl.uniform1i(this.uniform('markers'), 0); gl.uniform1i(this.uniform('texture'), 0);
    gl.uniform1i(this.uniform('water'), 0);
    gl.uniform1i(this.uniform('wireframe'), 0);
    gl.uniform2f(this.uniform('waterRange'), ...previewWaterRange(scene, form));
    const assets = new Map(scene.assets.map((asset) => [asset.id, asset]));
    const roots = scene.instances.filter((instance) => instance.kind === 'adt' || instance.root);
    const children = scene.instances.filter((instance) => instance.kind !== 'adt' && !instance.root);
    for (const instance of roots) this.drawGeometry(instance, assets.get(instance.asset)?.parts ?? []);
    if (form.clampLower > 0 || form.clampUpper < 1) {
      // Reuse the original triangles as an overlay, so geometry below the
      // flattened floor also stays visible. Opaque children are drawn after it.
      gl.uniform1i(this.uniform('wireframe'), 1);
      gl.disable(gl.DEPTH_TEST); gl.depthMask(false);
      gl.enable(gl.BLEND); gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
      for (const instance of roots) this.drawGeometry(instance, assets.get(instance.asset)?.parts ?? [], true);
      gl.uniform1i(this.uniform('wireframe'), 0);
      gl.enable(gl.DEPTH_TEST); gl.depthMask(true); gl.disable(gl.BLEND);
    }
    for (const instance of children) this.drawGeometry(instance, assets.get(instance.asset)?.parts ?? []);
    // Translucent source water respects terrain/bridge depth. The separate
    // orange guide stays visible when the clamp would bury or exclude water.
    gl.enable(gl.BLEND); gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA); gl.depthMask(false);
    gl.uniform1i(this.uniform('terrain'), 0); gl.uniform1i(this.uniform('textured'), 0);
    for (const pass of [1, 2]) {
      gl.uniform1i(this.uniform('water'), pass);
      if (pass === 2) gl.disable(gl.DEPTH_TEST);
      for (const instance of scene.instances) {
        gl.uniformMatrix4fv(this.uniform('model'), false, new Float32Array(instance.transform));
        for (const part of assets.get(instance.asset)?.parts ?? []) {
          if (!previewIsWater(part, scene)) continue;
          const mesh = this.meshes.get(part.mesh); if (!mesh) continue;
          gl.bindVertexArray(mesh.vao); gl.drawArrays(gl.TRIANGLES, 0, mesh.vertices);
        }
      }
    }
    gl.enable(gl.DEPTH_TEST); gl.depthMask(true); gl.disable(gl.BLEND);
    gl.uniform1i(this.uniform('water'), 0);
    if (this.markerCount) {
      gl.uniform1i(this.uniform('markers'), 1); gl.uniform1i(this.uniform('terrain'), 0); gl.uniform1i(this.uniform('textured'), 0);
      gl.bindTexture(gl.TEXTURE_2D, this.defaultTexture);
      gl.bindVertexArray(this.sphere.vao);
      gl.drawArraysInstanced(gl.TRIANGLES, 0, this.sphere.vertices, this.markerCount);
    }
    gl.bindVertexArray(null);
  }

  private sphereVertices(): Float32Array {
    const vertices: number[] = [];
    const point = (x: number, y: number) => {
      const latitude = y / 6 * Math.PI; const longitude = x / 8 * Math.PI * 2;
      const p = [Math.sin(latitude) * Math.cos(longitude), Math.sin(latitude) * Math.sin(longitude), Math.cos(latitude)];
      vertices.push(...p, ...p, 0, 0);
    };
    for (let y = 0; y < 6; y++) {
      for (let x = 0; x < 8; x++) {
        point(x, y); point(x + 1, y); point(x + 1, y + 1);
        point(x, y); point(x + 1, y + 1); point(x, y + 1);
      }
    }
    return new Float32Array(vertices);
  }

  // Match ModelViewerUi: left orbit, right ground-plane pan, middle or L+R
  // screen-plane pan, wheel zoom, one-finger orbit and two-finger pinch zoom.
  private preventDefault = (event: Event): void => { event.preventDefault(); };

  private resetButtons = (): void => { this.buttons = [false, false, false]; this.touching = false; };

  private onMouseDown = (e: MouseEvent): void => { e.preventDefault(); this.buttons[e.button] = true; this.lastX = e.clientX; this.lastY = e.clientY; };

  private onMouseUp = (e: MouseEvent): void => { this.buttons[e.button] = false; };

  private onMouseMove = (e: MouseEvent): void => {
    if (this.touching || !this.buttons.some(Boolean)) return;
    const dx = e.clientX - this.lastX; const dy = e.clientY - this.lastY;
    this.lastX = e.clientX; this.lastY = e.clientY;
    if (this.buttons[1] || (this.buttons[0] && this.buttons[2])) this.pan(dx, dy, false);
    else if (this.buttons[0]) this.orbit(dx, dy);
    else if (this.buttons[2]) this.pan(dx, dy, true);
    this.requestRender();
  };

  private orbit(dx: number, dy: number): void {
    this.horizontalAngle -= dx * Math.PI / 360;
    this.verticalAngle = Math.max(-Math.PI / 2 + 0.01, Math.min(Math.PI / 2 - 0.01, this.verticalAngle + dy * Math.PI / 360));
  }

  private pan(dx: number, dy: number, ground: boolean): void {
    const right = vec3.fromValues(this.view[0], this.view[4], ground ? 0 : this.view[8]);
    const up = vec3.fromValues(this.view[1], this.view[5], ground ? 0 : this.view[9]);
    vec3.normalize(right, right); vec3.normalize(up, up);
    vec3.scaleAndAdd(this.target, this.target, right, -dx / Math.max(1, this.canvas.clientWidth) * this.distance);
    vec3.scaleAndAdd(this.target, this.target, up, dy / Math.max(1, this.canvas.clientHeight) * this.distance);
  }

  private onWheel = (e: WheelEvent): void => {
    e.preventDefault(); this.distance = Math.max(1, Math.min(2_000_000, this.distance * (e.deltaY > 0 ? 1.1 : 0.9))); this.requestRender();
  };

  private onTouchStart = (e: TouchEvent): void => {
    e.preventDefault(); this.touching = true;
    if (e.touches.length === 1) { this.lastX = e.touches[0].clientX; this.lastY = e.touches[0].clientY; }
    if (e.touches.length === 2) this.touchDistance = Math.hypot(e.touches[0].clientX - e.touches[1].clientX, e.touches[0].clientY - e.touches[1].clientY);
  };

  private onTouchMove = (e: TouchEvent): void => {
    e.preventDefault(); if (!this.touching) return;
    if (e.touches.length === 1) {
      this.orbit(e.touches[0].clientX - this.lastX, e.touches[0].clientY - this.lastY);
      this.lastX = e.touches[0].clientX; this.lastY = e.touches[0].clientY;
    } else if (e.touches.length === 2) {
      const distance = Math.hypot(e.touches[0].clientX - e.touches[1].clientX, e.touches[0].clientY - e.touches[1].clientY);
      if (this.touchDistance > 0) this.distance = Math.max(1, Math.min(2_000_000, this.distance * (2 - distance / this.touchDistance)));
      this.touchDistance = distance;
    }
    this.requestRender();
  };

  private onTouchEnd = (e: TouchEvent): void => { e.preventDefault(); this.touching = false; this.touchDistance = 0; };
}

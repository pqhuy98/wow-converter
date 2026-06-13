/**
 * CPU port of wow.export's CharMaterialRenderer (src/js/3D/renderers/CharMaterialRenderer.js).
 *
 * The original bakes character customization textures using a WebGL canvas and
 * the char.vertex/char.fragment shaders. This port replicates the GL pipeline
 * (rasterization of section rects, fragment blend math and framebuffer
 * blending, including its quirks) on the CPU so it can run headless under
 * Bun/Node. Output is verified against wow.export with pixel tolerance.
 */
import { write } from '@/lib/wow/log';
import { getCasc } from '@/lib/wow/server/runtime';

import * as listfile from '../archive/casc/listfile';
import { BLPImage } from '../formats/blp/blp';
import { PNGWriter } from '../formats/png-writer';
import { wowConfig } from '../server/config';

export interface CharComponentTextureSection {
  SectionType?: number;
  X: number;
  Y: number;
  Width: number;
  Height: number;
  OverlapSectionMask?: number;
}

export interface ChrModelMaterialRow {
  TextureType: number;
  Width: number;
  Height: number;
  Flags?: number;
}

export interface ChrModelTextureLayerRow {
  TextureType: number;
  Layer: number;
  Flags: number;
  BlendMode: number;
  TextureSectionTypeBitMask: number;
  ChrModelTextureTargetID: number[];
}

export interface ChrCustomizationMaterialEntry {
  ChrModelTextureTargetID: number;
  FileDataID: number;
}

interface CPUTexture {
  data: Uint8Array; // RGBA, row 0 = image top
  width: number;
  height: number;
}

interface TextureTarget {
  id: number;
  section: CharComponentTextureSection;
  material: ChrModelMaterialRow;
  textureLayer: ChrModelTextureLayerRow;
  custMaterial: ChrCustomizationMaterialEntry;
  texture: CPUTexture;
  filename: string | undefined;
}

/**
 * Un-premultiply an RGBA buffer the way Chromium serialises a WebGL canvas
 * created with premultipliedAlpha=true: raw bytes are interpreted as
 * premultiplied, so rgb' = clamp(round(rgb * 255 / a)); a=0 zeroes rgb.
 */
function unpremultiply(src: Uint8ClampedArray | Uint8Array): Uint8Array {
  const out = new Uint8Array(src.length);
  for (let i = 0; i < src.length; i += 4) {
    const a = src[i + 3];
    if (a === 0) {
      out[i + 3] = 0;
    } else if (a === 255) {
      out[i] = src[i];
      out[i + 1] = src[i + 1];
      out[i + 2] = src[i + 2];
      out[i + 3] = 255;
    } else {
      out[i] = Math.min(255, Math.round((src[i] * 255) / a));
      out[i + 1] = Math.min(255, Math.round((src[i + 1] * 255) / a));
      out[i + 2] = Math.min(255, Math.round((src[i + 2] * 255) / a));
      out[i + 3] = a;
    }
  }
  return out;
}

/** Sample a texture with CLAMP_TO_EDGE + GL filter selection (NEAREST min / LINEAR mag). */
function sampleTexture(tex: CPUTexture, u: number, v: number, minified: boolean, out: Float64Array): void {
  const {
    data, width, height,
  } = tex;

  if (minified) {
    // NEAREST
    let tx = Math.floor(u * width);
    let ty = Math.floor(v * height);
    tx = Math.min(width - 1, Math.max(0, tx));
    ty = Math.min(height - 1, Math.max(0, ty));
    const i = (ty * width + tx) * 4;
    out[0] = data[i] / 255;
    out[1] = data[i + 1] / 255;
    out[2] = data[i + 2] / 255;
    out[3] = data[i + 3] / 255;
    return;
  }

  // LINEAR (bilinear, clamp-to-edge)
  const fx = u * width - 0.5;
  const fy = v * height - 0.5;
  const x0 = Math.floor(fx);
  const y0 = Math.floor(fy);
  const dx = fx - x0;
  const dy = fy - y0;

  const cx0 = Math.min(width - 1, Math.max(0, x0));
  const cx1 = Math.min(width - 1, Math.max(0, x0 + 1));
  const cy0 = Math.min(height - 1, Math.max(0, y0));
  const cy1 = Math.min(height - 1, Math.max(0, y0 + 1));

  const i00 = (cy0 * width + cx0) * 4;
  const i10 = (cy0 * width + cx1) * 4;
  const i01 = (cy1 * width + cx0) * 4;
  const i11 = (cy1 * width + cx1) * 4;

  const w00 = (1 - dx) * (1 - dy);
  const w10 = dx * (1 - dy);
  const w01 = (1 - dx) * dy;
  const w11 = dx * dy;

  for (let c = 0; c < 4; c++) {
    out[c] = (data[i00 + c] * w00 + data[i10 + c] * w10 + data[i01 + c] * w01 + data[i11 + c] * w11) / 255;
  }
}

export class CharMaterialRenderer {
  textureTargets: TextureTarget[] = [];

  width: number;

  height: number;

  /** RGBA canvas, row 0 = image top (matches canvas.toDataURL orientation). */
  canvas: Uint8ClampedArray;

  /** Mirrors CharMaterialRenderer.init(); shader sources are compiled into this port. */
  static init(): void {
    // No-op: blend math from char.fragment.shader is implemented natively.
  }

  constructor(_textureLayer: string | number, width: number, height: number, _headless = true) {
    this.width = width;
    this.height = height;
    this.canvas = new Uint8ClampedArray(width * height * 4);
  }

  init(): void {
    this.reset();
  }

  /** Get the baked canvas as a PNG data URI (replaces canvas.toDataURL()). */
  getURI(): string {
    return `data:image/png;base64,${this.getPNG().toString('base64')}`;
  }

  /** Encode the canvas to a PNG buffer. */
  getPNG(): Buffer {
    const png = new PNGWriter(this.width, this.height);
    // wow.export's WebGL canvas uses premultipliedAlpha=true: the drawing
    // buffer bytes are interpreted as premultiplied, so canvas.toDataURL()
    // un-premultiplies them (rgb * 255 / a, clamped). Replicate that here.
    png.getPixelData().set(unpremultiply(this.canvas));
    return png.getBuffer().raw;
  }

  reset(): void {
    this.textureTargets = [];
    this.clearCanvas();
  }

  /** Loads a specific texture to a target. */
  async setTextureTarget(
    chrCustomizationMaterial: ChrCustomizationMaterialEntry,
    charComponentTextureSection: CharComponentTextureSection,
    chrModelMaterial: ChrModelMaterialRow,
    chrModelTextureLayer: ChrModelTextureLayerRow,
    useAlpha: boolean = true,
    filenameOverride: string | undefined = undefined,
  ): Promise<void> {
    // The converter process has no listfile; the direct pipeline passes the
    // server-resolved name instead.
    const filename = filenameOverride ?? listfile.getByID(chrCustomizationMaterial.FileDataID);
    // write('Loading texture %s for target %d with alpha %s', filename, chrCustomizationMaterial.ChrModelTextureTargetID, useAlpha);

    this.textureTargets.push({
      id: chrCustomizationMaterial.ChrModelTextureTargetID,
      section: charComponentTextureSection,
      material: chrModelMaterial,
      textureLayer: chrModelTextureLayer,
      custMaterial: chrCustomizationMaterial,
      texture: await this.loadTexture(chrCustomizationMaterial.FileDataID, useAlpha),
      filename,
    });

    this.update();
  }

  dispose(): void {
    this.textureTargets = [];
  }

  /** Load a texture from CASC and decode it to RGBA. */
  async loadTexture(fileDataID: number, useAlpha = true): Promise<CPUTexture> {
    const blp = new BLPImage(await getCasc().getFile(fileDataID));
    const data = blp.toUInt8Array(0, useAlpha ? 0b1111 : 0b0111);
    return { data, width: blp.width, height: blp.height };
  }

  clearCanvas(): void {
    this.canvas.fill(0);
  }

  /** Re-bake the canvas from all texture targets (port of update()). */
  update(): void {
    this.clearCanvas();

    // Order texture targets by id (stable) and remove duplicates by id+section rect.
    const sortedTargets = [...this.textureTargets].sort((a, b) => a.id - b.id);
    const seen = new Set<string>();
    const uniqueTargets = sortedTargets.filter((target) => {
      const key = `${target.id}_${target.section.X}_${target.section.Y}_${target.section.Width}_${target.section.Height}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });

    for (const layer of uniqueTargets) {
      // Hide underwear based on settings.
      if (!wowConfig.chrIncludeBaseClothing && (layer.textureLayer.ChrModelTextureTargetID[0] === 13 || layer.textureLayer.ChrModelTextureTargetID[0] === 14)) continue;

      this.drawLayer(layer);
    }
  }

  /** Rasterize one texture target onto the canvas, replicating the GL draw. */
  private drawLayer(layer: TextureTarget): void {
    const { section, material } = layer;
    const blendMode = layer.textureLayer.BlendMode;

    // Destination rect in image coordinates (top-left origin). The original
    // computes clip-space coords from section X/Y relative to material size;
    // since the canvas is created with the material dimensions this reduces to
    // a straight top-left blit of the section rect.
    const rectX = Math.round((section.X / material.Width) * this.width);
    const rectY = Math.round((section.Y / material.Height) * this.height);
    const rectW = Math.round((section.Width / material.Width) * this.width);
    const rectH = Math.round((section.Height / material.Height) * this.height);

    if (rectW <= 0 || rectH <= 0) return;

    // write(
    //   '[%d] Placing texture %s of blend mode %d for target %d with offset %dx%d of size %dx%d',
    //   material.TextureType,
    //   layer.filename,
    //   blendMode,
    //   layer.id,
    //   section.X,
    //   section.Y,
    //   section.Width,
    //   section.Height,
    // );

    // For multiply/overlay/screen, snapshot the current canvas as the base
    // texture (replicates TEXTURE1 upload, including wow.export's
    // bottom-origin readPixels for partial sections).
    let baseTex: CPUTexture | null = null;
    if (blendMode === 4 || blendMode === 6 || blendMode === 7) {
      if (material.Width === section.Width && material.Height === section.Height) {
        // texImage2D(canvas) un-premultiplies the (premultiplied-interpreted)
        // canvas bytes; readPixels (the partial path below) does not.
        baseTex = { data: unpremultiply(this.canvas), width: this.width, height: this.height };
      } else {
        const snap = new Uint8Array(rectW * rectH * 4);
        for (let fy = 0; fy < rectH; fy++) {
          // flipped[fy] = image row (CanvasH - sectionY - H + fy), per the
          // readPixels (bottom-origin) + manual flip in wow.export.
          const srcRow = this.height - rectY - rectH + fy;
          if (srcRow < 0 || srcRow >= this.height) continue; // readPixels outside framebuffer -> zeros
          for (let x = 0; x < rectW; x++) {
            const srcCol = rectX + x;
            if (srcCol < 0 || srcCol >= this.width) continue;
            const src = (srcRow * this.width + srcCol) * 4;
            const dst = (fy * rectW + x) * 4;
            snap[dst] = this.canvas[src];
            snap[dst + 1] = this.canvas[src + 1];
            snap[dst + 2] = this.canvas[src + 2];
            snap[dst + 3] = this.canvas[src + 3];
          }
        }
        baseTex = { data: snap, width: rectW, height: rectH };
      }
    }

    // GL filter selection: LOD > 0 (minification) uses MIN_FILTER (NEAREST,
    // set in loadTexture); otherwise MAG_FILTER (LINEAR, default).
    const tex = layer.texture;
    const texMinified = Math.max(tex.width / rectW, tex.height / rectH) > 1;
    const baseMinified = baseTex ? Math.max(baseTex.width / rectW, baseTex.height / rectH) > 1 : false;

    const src = new Float64Array(4);
    const base = new Float64Array(4);
    const frag = new Float64Array(4);

    const x0 = Math.max(0, rectX);
    const y0 = Math.max(0, rectY);
    const x1 = Math.min(this.width, rectX + rectW);
    const y1 = Math.min(this.height, rectY + rectH);

    for (let py = y0; py < y1; py++) {
      const v = (py - rectY + 0.5) / rectH;
      for (let px = x0; px < x1; px++) {
        const u = (px - rectX + 0.5) / rectW;

        sampleTexture(tex, u, v, texMinified, src);

        // --- Fragment shader (char.fragment.shader) ---
        if (blendMode === 0 || blendMode === 1 || blendMode === 9 || blendMode === 15) {
          frag.set(src);
        } else if (blendMode === 4) { // MULTIPLY
          sampleTexture(baseTex!, u, v, baseMinified, base);
          frag[0] = base[0] * src[0];
          frag[1] = base[1] * src[1];
          frag[2] = base[2] * src[2];
          frag[3] = base[3] * src[3];
        } else if (blendMode === 7) { // SCREEN
          sampleTexture(baseTex!, u, v, baseMinified, base);
          frag[0] = 1 - (1 - base[0]) * (1 - src[0]);
          frag[1] = 1 - (1 - base[1]) * (1 - src[1]);
          frag[2] = 1 - (1 - base[2]) * (1 - src[2]);
          frag[3] = src[3];
        } else if (blendMode === 6) { // OVERLAY
          sampleTexture(baseTex!, u, v, baseMinified, base);
          for (let c = 0; c < 3; c++) {
            frag[c] = src[c] < 0.5
              ? 2 * base[c] * src[c]
              : 1 - 2 * (1 - base[c]) * (1 - src[c]);
          }
          frag[3] = src[3];
        } else {
          // Shader fallback: magenta (unused blend modes).
          frag[0] = 1; frag[1] = 0; frag[2] = 1; frag[3] = 1;
        }

        // --- Framebuffer blending ---
        const di = (py * this.width + px) * 4;
        const sa = frag[3];
        if (blendMode === 0) {
          // Blending disabled: direct write.
          this.canvas[di] = frag[0] * 255;
          this.canvas[di + 1] = frag[1] * 255;
          this.canvas[di + 2] = frag[2] * 255;
          this.canvas[di + 3] = sa * 255;
        } else if (blendMode === 9) {
          // blendFuncSeparate(SRC_ALPHA, ONE_MINUS_SRC_ALPHA, ONE, ONE_MINUS_SRC_ALPHA)
          this.canvas[di] = (frag[0] * sa + (this.canvas[di] / 255) * (1 - sa)) * 255;
          this.canvas[di + 1] = (frag[1] * sa + (this.canvas[di + 1] / 255) * (1 - sa)) * 255;
          this.canvas[di + 2] = (frag[2] * sa + (this.canvas[di + 2] / 255) * (1 - sa)) * 255;
          this.canvas[di + 3] = (sa + (this.canvas[di + 3] / 255) * (1 - sa)) * 255;
        } else {
          // blendFunc(SRC_ALPHA, ONE_MINUS_SRC_ALPHA) -- applies to alpha too.
          this.canvas[di] = (frag[0] * sa + (this.canvas[di] / 255) * (1 - sa)) * 255;
          this.canvas[di + 1] = (frag[1] * sa + (this.canvas[di + 1] / 255) * (1 - sa)) * 255;
          this.canvas[di + 2] = (frag[2] * sa + (this.canvas[di + 2] / 255) * (1 - sa)) * 255;
          this.canvas[di + 3] = (sa * sa + (this.canvas[di + 3] / 255) * (1 - sa)) * 255;
        }
      }
    }

    if (![0, 1, 4, 6, 7, 9, 15].includes(blendMode)) {
      write('Warning: encountered previously unused blendmode %d during character texture baking, poke a dev', blendMode);
    }
  }
}

export default CharMaterialRenderer;

/**
 * ADT exporter, ported from wow.export (src/js/3D/exporters/ADTExporter.js).
 *
 * The WebGL terrain bake is replaced by the CPU rasterizer in
 * map/terrain-baker.ts (pixel-tolerance verified). All other artifacts
 * (OBJ/MTL/CSV/JSON) are byte-identical ports.
 */
import path from 'path';
import util from 'util';

import { DB2Row, WDCReader } from '@/lib/wow/db/wdc-reader';
import { write } from '@/lib/wow/log';
import { getCasc } from '@/lib/wow/server/runtime';

import * as listfile from '../../archive/casc/listfile';
import { ADTLoader, DoodadEntry } from '../../formats/adt/adt-loader';
import { WDTLoader } from '../../formats/adt/wdt-loader';
import { BLPImage } from '../../formats/blp/blp';
import { constants } from '../../formats/constants';
import { PNGWriter } from '../../formats/png-writer';
import { wowConfig } from '../../server/config';
import type { ExportProgress } from '../export-progress';
import { M2Exporter } from '../m2/m2-exporter';
import { modelReferencePath, resolveModelStoragePath } from '../model-reference-path';
import { WMOExporter } from '../wmo/wmo-exporter';
import { CSVWriter } from '../writers/csv-writer';
import {
  getExportPath, replaceExtension, win32ToPosix,
} from '../writers/export-helper';
import { JSONWriter } from '../writers/json-writer';
import { MTLWriter } from '../writers/mtl-writer';
import { OBJWriter } from '../writers/obj-writer';
import { outputFileExists, writeOutputFile } from '../writers/output-sink';
import { ADTExportOptions } from './map-export-utils';
import {
  bakeChunk, BakeMaterial, loadBakeTexture, resizeBilinear, rotate180,
} from './terrain-baker';

const MAP_SIZE = constants.GAME.MAP_SIZE;
const TILE_SIZE = constants.GAME.TILE_SIZE;
const CHUNK_SIZE = TILE_SIZE / 16;
const UNIT_SIZE = CHUNK_SIZE / 8;
const UNIT_SIZE_HALF = UNIT_SIZE / 2;

type ADTExportObject = DoodadEntry | DB2Row;

function isDoodadEntry(model: ADTExportObject): model is DoodadEntry {
  return typeof (model as DoodadEntry).mmidEntry === 'number' && Array.isArray((model as DoodadEntry).position);
}

function getPlacementFileDataID(model: ADTExportObject): number {
  if (isDoodadEntry(model)) return model.mmidEntry;
  const fdid = model.FileDataID;
  return typeof fdid === 'number' ? fdid : 0;
}

function getPlacementCoord(model: ADTExportObject, axis: 'position' | 'rotation', index: number): number {
  if (isDoodadEntry(model)) {
    const arr = axis === 'position' ? model.position : model.rotation;
    return arr[index] ?? 0;
  }
  const arr = axis === 'position' ? model.Position : model.Rotation;
  return Array.isArray(arr) ? (arr[index] as number | undefined) ?? 0 : 0;
}

function getPlacementScaleFactor(model: ADTExportObject): number {
  if (isDoodadEntry(model)) return model.scale / 1024;
  return 1;
}

function getPlacementUniqueId(model: ADTExportObject): number {
  if (isDoodadEntry(model)) return model.uniqueId;
  const uid = model.uniqueId;
  return typeof uid === 'number' ? uid : 0;
}

const wdtCache = new Map<string, WDTLoader>();

let isFoliageAvailable = false;
let hasLoadedFoliage = false;
let dbTextures: WDCReader | undefined;
let dbDoodads: WDCReader | undefined;

/** Load and cache GroundEffectDoodad and GroundEffectTexture data tables. */
const loadFoliageTables = async (): Promise<void> => {
  if (!hasLoadedFoliage) {
    try {
      dbDoodads = new WDCReader('DBFilesClient/GroundEffectDoodad.db2');
      dbTextures = new WDCReader('DBFilesClient/GroundEffectTexture.db2');

      await dbDoodads.parse();
      await dbTextures.parse();

      hasLoadedFoliage = true;
      isFoliageAvailable = true;
    } catch (e) {
      isFoliageAvailable = false;
      write('Unable to load foliage tables, foliage exporting will be unavailable for all tiles.');
    }

    hasLoadedFoliage = true;
  }
};

/** Convert an RGBA object into an integer. */
const rgbaToInt = (rgba: { r: number; g: number; b: number; a: number }): number => {
  let intval = rgba.r;
  intval = (intval << 8) + rgba.g;
  intval = (intval << 8) + rgba.b;
  return (intval << 8) + rgba.a;
};

/** Write an RGBA pixel buffer as a PNG through the output sink. */
const writePNG = async (outPath: string, pixels: Uint8ClampedArray | Uint8Array, width: number, height: number): Promise<void> => {
  const png = new PNGWriter(width, height);
  png.getPixelData().set(pixels);
  await writeOutputFile(outPath, png.getBuffer().raw);
};

interface AlphaMapMaterial {
  scale: number;
  fileDataID: number;
  file?: string;
  heightFile?: string;
  heightFileDataID?: number;
  heightScale?: number;
  heightOffset?: number;
}

interface ADTExportResult {
  type: 'ADT_RAW' | 'ADT_OBJ';
  path: string;
}

export class ADTExporter {
  mapID: number;

  mapDir: string;

  tileX: number;

  tileY: number;

  tileID: string;

  tileIndex: number;

  constructor(mapID: number, mapDir: string, tileIndex: number) {
    this.mapID = mapID;
    this.mapDir = mapDir;
    this.tileX = tileIndex % MAP_SIZE;
    this.tileY = Math.floor(tileIndex / MAP_SIZE);
    this.tileID = `${this.tileY}_${this.tileX}`;
    this.tileIndex = tileIndex;
  }

  /** Calculate UV bounds for single-texture-mode normalization. */
  calculateUVBounds(rootAdt: ADTLoader, firstChunkX: number, firstChunkY: number): { minU: number; maxU: number; minV: number; maxV: number } {
    let minU = Infinity; let maxU = -Infinity;
    let minV = Infinity; let maxV = -Infinity;

    for (let x = 0; x < 16; x++) {
      for (let y = 0; y < 16; y++) {
        const chunk = rootAdt.chunks[x * 16 + y];
        if (!chunk || !chunk.vertices) continue;

        const chunkX = chunk.position[0];
        const chunkY = chunk.position[1];

        for (let row = 0; row < 17; row++) {
          const isShort = !!(row % 2);
          const colCount = isShort ? 8 : 9;

          for (let col = 0; col < colCount; col++) {
            let vx = chunkY - (col * UNIT_SIZE);
            const vz = chunkX - (row * UNIT_SIZE_HALF);

            if (isShort) vx -= UNIT_SIZE_HALF;

            const u = -(vx - firstChunkX) / TILE_SIZE;
            const v = (vz - firstChunkY) / TILE_SIZE;

            minU = Math.min(minU, u);
            maxU = Math.max(maxU, u);
            minV = Math.min(minV, v);
            maxV = Math.max(maxV, v);
          }
        }
      }
    }

    return {
      minU, maxU, minV, maxV,
    };
  }

  /**
   * Export the ADT tile.
   * @param dir Directory to export the tile into.
   * @param quality Texture resolution (-1 alpha maps, 0 none, <=512 minimap, 513+ bake).
   * @param gameObjects Additional game objects to export.
   * @param options Request-specific export options.
   * @param progress Optional progress reporter for batch exports.
   */
  async export(
    dir: string,
    quality: number,
    gameObjects: Set<DB2Row> | undefined,
    options?: ADTExportOptions,
    progress?: ExportProgress,
  ): Promise<ADTExportResult> {
    const casc = getCasc();
    // Prefer caller-provided options; fall back to global config.
    const config = options ?? (wowConfig as unknown as ADTExportOptions);

    const out: ADTExportResult = { type: config.mapsExportRaw ? 'ADT_RAW' : 'ADT_OBJ', path: '' };

    const usePosix = config.pathFormat === 'posix';
    const prefix = util.format('world/maps/%s/%s', this.mapDir, this.mapDir);

    // Load the WDT. We cache this to speed up exporting large amounts of tiles
    // from the same map. Make sure ADTExporter.clearCache() is called after exporting.
    let wdt = wdtCache.get(this.mapDir);
    if (!wdt) {
      const wdtFile = await casc.getFileByName(`${prefix}.wdt`);

      wdt = new WDTLoader(wdtFile);
      wdt.load();
      wdtCache.set(this.mapDir, wdt);

      if (config.mapsExportRaw) {
        await writeOutputFile(path.join(dir, `${this.mapDir}.wdt`), wdtFile.raw);

        const rawAux: [number | undefined, string][] = [
          [wdt.lgtFileDataID, '_lgt.wdt'],
          [wdt.occFileDataID, '_occ.wdt'],
          [wdt.fogsFileDataID, '_fogs.wdt'],
          [wdt.mpvFileDataID, '_mpv.wdt'],
          [wdt.texFileDataID, '.tex'],
          [wdt.wdlFileDataID, '.wdl'],
          [wdt.pd4FileDataID, '.pd4'],
        ];

        for (const [fileDataID, suffix] of rawAux) {
          if (fileDataID && fileDataID > 0) {
            const file = await casc.getFile(fileDataID);
            await writeOutputFile(path.join(dir, this.mapDir + suffix), file.raw);
          }
        }
      }
    }

    const tilePrefix = `${prefix}_${this.tileID}`;

    const maid = wdt.entries![this.tileIndex];
    const rootFileDataID = maid.rootADT > 0 ? maid.rootADT : listfile.getByFilename(`${tilePrefix}.adt`) ?? 0;
    const tex0FileDataID = maid.tex0ADT > 0 ? maid.tex0ADT : listfile.getByFilename(`${tilePrefix}_tex0.adt`) ?? 0;
    const obj0FileDataID = maid.obj0ADT > 0 ? maid.obj0ADT : listfile.getByFilename(`${tilePrefix}_obj0.adt`) ?? 0;
    const obj1FileDataID = maid.obj1ADT > 0 ? maid.obj1ADT : listfile.getByFilename(`${tilePrefix}_obj1.adt`) ?? 0;

    // Ensure we actually have the fileDataIDs for the files we need. LOD is not available on Classic.
    if (rootFileDataID === 0 || tex0FileDataID === 0 || obj0FileDataID === 0 || obj1FileDataID === 0) throw new Error(`Missing fileDataID for ADT files: ${[rootFileDataID, tex0FileDataID, obj0FileDataID].join(', ')}`);

    const rootFile = await casc.getFile(rootFileDataID);
    const texFile = await casc.getFile(tex0FileDataID);
    const objFile = await casc.getFile(obj0FileDataID);

    if (config.mapsExportRaw) {
      await writeOutputFile(path.join(dir, `${this.mapDir}_${this.tileID}.adt`), rootFile.raw);
      await writeOutputFile(path.join(dir, `${this.mapDir}_${this.tileID}_tex0.adt`), texFile.raw);
      await writeOutputFile(path.join(dir, `${this.mapDir}_${this.tileID}_obj0.adt`), objFile.raw);

      // We only care about these when exporting raw files.
      const obj1File = await casc.getFile(obj1FileDataID);
      await writeOutputFile(path.join(dir, `${this.mapDir}_${this.tileID}_obj1.adt`), obj1File.raw);

      // LOD is not available on Classic.
      if (maid.lodADT > 0) {
        const lodFile = await casc.getFile(maid.lodADT);
        await writeOutputFile(path.join(dir, `${this.mapDir}_${this.tileID}_lod.adt`), lodFile.raw);
      }
    }

    const rootAdt = new ADTLoader(rootFile);
    rootAdt.loadRoot();

    const texAdt = new ADTLoader(texFile);
    texAdt.loadTex(wdt);

    const objAdt = new ADTLoader(objFile);
    objAdt.loadObj();
    progress?.advance();
    progress?.setLabel(`Tile ${this.tileID}, terrain mesh`);

    if (!config.mapsExportRaw) {
      const vertices = new Array<number>(16 * 16 * 145 * 3);
      const normals = new Array<number>(16 * 16 * 145 * 3);
      const uvs = new Array<number>(16 * 16 * 145 * 2);
      const uvsBake = new Array<number>(16 * 16 * 145 * 2);
      const vertexColors = new Array<number>(16 * 16 * 145 * 4);

      const chunkMeshes = new Array<number[]>(256);

      const objOut = path.join(dir, `adt_${this.tileID}.obj`);
      out.path = objOut;

      const obj = new OBJWriter(objOut);
      const mtl = new MTLWriter(path.join(dir, `adt_${this.tileID}.mtl`));

      const firstChunk = rootAdt.chunks[0];
      const firstChunkX = firstChunk.position[0];
      const firstChunkY = firstChunk.position[1];

      const isAlphaMaps = quality === -1;
      const isLargeBake = quality >= 8192;
      // NOTE: wow.export reads these three from the global config even when
      // request options are provided (quirk preserved for parity).
      const isSplittingAlphaMaps = isAlphaMaps && wowConfig.splitAlphaMaps;
      const isSplittingTextures = isLargeBake && wowConfig.splitLargeTerrainBakes;
      const includeHoles = wowConfig.mapsIncludeHoles;

      // Calculate UV bounds for single texture mode normalization.
      let uvBounds: { minU: number; maxU: number; minV: number; maxV: number } | null = null;
      if (quality !== 0 && !isSplittingTextures && !isSplittingAlphaMaps) uvBounds = this.calculateUVBounds(rootAdt, firstChunkX, firstChunkY);

      let ofs = 0;
      let chunkID = 0;
      for (let x = 0, midX = 0; x < 16; x++) {
        for (let y = 0; y < 16; y++) {
          const indices: number[] = [];

          const chunkIndex = (x * 16) + y;
          const chunk = rootAdt.chunks[chunkIndex];

          const chunkX = chunk.position[0];
          const chunkY = chunk.position[1];
          const chunkZ = chunk.position[2];

          for (let row = 0, idx = 0; row < 17; row++) {
            const isShort = !!(row % 2);
            const colCount = isShort ? 8 : 9;

            for (let col = 0; col < colCount; col++) {
              let vx = chunkY - (col * UNIT_SIZE);
              const vy = chunk.vertices![idx] + chunkZ;
              const vz = chunkX - (row * UNIT_SIZE_HALF);

              if (isShort) vx -= UNIT_SIZE_HALF;

              const vIndex = midX * 3;
              vertices[vIndex + 0] = vx;
              vertices[vIndex + 1] = vy;
              vertices[vIndex + 2] = vz;

              const normal = chunk.normals![idx];
              normals[vIndex + 0] = normal[0] / 127;
              normals[vIndex + 1] = normal[1] / 127;
              normals[vIndex + 2] = normal[2] / 127;

              const cIndex = midX * 4;
              if (chunk.vertexShading) {
                // Store vertex shading in BGRA format.
                const color = chunk.vertexShading[idx];
                vertexColors[cIndex + 0] = color.b / 255;
                vertexColors[cIndex + 1] = color.g / 255;
                vertexColors[cIndex + 2] = color.r / 255;
                vertexColors[cIndex + 3] = color.a / 255;
              } else {
                // No vertex shading, default to this.
                vertexColors[cIndex + 0] = 0.5;
                vertexColors[cIndex + 1] = 0.5;
                vertexColors[cIndex + 2] = 0.5;
                vertexColors[cIndex + 3] = 1;
              }

              const uvIdx = isShort ? col + 0.5 : col;
              const uvIndex = midX * 2;

              const uRaw = -(vx - firstChunkX) / TILE_SIZE;
              const vRaw = (vz - firstChunkY) / TILE_SIZE;

              uvsBake[uvIndex + 0] = uRaw;
              uvsBake[uvIndex + 1] = vRaw;

              if (quality === 0) {
                uvs[uvIndex + 0] = uvIdx / 8;
                uvs[uvIndex + 1] = (row * 0.5) / 8;
              } else if (isSplittingTextures || isSplittingAlphaMaps) {
                uvs[uvIndex + 0] = uvIdx / 8;
                uvs[uvIndex + 1] = 1 - (row / 16);
              } else {
                // Single texture mode - apply normalization.
                if (uvBounds) {
                  uvs[uvIndex + 0] = (uRaw - uvBounds.minU) / (uvBounds.maxU - uvBounds.minU);
                  uvs[uvIndex + 1] = (vRaw - uvBounds.minV) / (uvBounds.maxV - uvBounds.minV);
                } else {
                  // Fallback to raw values if bounds calculation failed.
                  uvs[uvIndex + 0] = uRaw;
                  uvs[uvIndex + 1] = vRaw;
                }
              }

              idx++;
              midX++;
            }
          }

          const holesHighRes = chunk.holesHighRes;
          for (let j = 9, xx = 0, yy = 0; j < 145; j++, xx++) {
            if (xx >= 8) {
              xx = 0;
              yy++;
            }

            let isHole = true;
            if (includeHoles === true) {
              if (!(chunk.flags & 0x10000)) {
                const current = Math.trunc(2 ** (Math.floor(xx / 2) + Math.floor(yy / 2) * 4));

                if (!(chunk.holesLowRes & current)) isHole = false;
              } else {
                if (!((holesHighRes[yy] >> xx) & 1)) isHole = false;
              }
            } else {
              isHole = false;
            }

            if (!isHole) {
              const indOfs = ofs + j;
              indices.push(indOfs, indOfs - 9, indOfs + 8);
              indices.push(indOfs, indOfs - 8, indOfs - 9);
              indices.push(indOfs, indOfs + 9, indOfs - 8);
              indices.push(indOfs, indOfs + 8, indOfs + 9);
            }

            if (!((j + 1) % (9 + 8))) j += 9;
          }

          ofs = midX;

          if (isSplittingTextures || isSplittingAlphaMaps) {
            const objName = `${this.tileID}_${chunkID}`;
            const matName = `tex_${objName}`;
            mtl.addMaterial(matName, `${matName}.png`);
            obj.addMesh(objName, indices, matName);
          } else {
            obj.addMesh(String(chunkID), indices, `tex_${this.tileID}`);
          }
          chunkMeshes[chunkIndex] = indices;

          chunkID++;
        }
      }

      if (quality !== 0 && ((!isAlphaMaps && !isSplittingTextures) || (isAlphaMaps && !isSplittingAlphaMaps))) mtl.addMaterial(`tex_${this.tileID}`, `tex_${this.tileID}.png`);

      obj.setVertArray(vertices);
      obj.setNormalArray(normals);
      obj.addUVArray(uvs);

      if (!mtl.isEmpty) obj.setMaterialLibrary(path.basename(mtl.out));

      await obj.write(config.overwriteFiles);
      await mtl.write(config.overwriteFiles);
      progress?.advance();
      progress?.setLabel(`Tile ${this.tileID}, textures`);

      if (quality !== 0) {
        if (isAlphaMaps) {
          // Export alpha maps.
          const materialIDs = texAdt.diffuseTextureFileDataIDs!;
          const heightIDs = texAdt.heightTextureFileDataIDs ?? [];
          const texParams = texAdt.texParams;

          const saveLayerTexture = async (fileDataID: number): Promise<string> => {
            const blp = new BLPImage(await casc.getFile(fileDataID));
            let fileName = listfile.getByID(fileDataID);
            if (fileName !== undefined) fileName = replaceExtension(fileName, '.png');
            else fileName = listfile.formatUnknownFile(fileDataID, '.png');

            let texFileOut: string;
            let texPath: string;

            if (config.enableSharedTextures) {
              texPath = getExportPath(fileName);
              texFileOut = path.relative(dir, texPath);
            } else {
              texPath = path.join(dir, path.basename(fileName));
              texFileOut = path.basename(texPath);
            }

            await writeOutputFile(texPath, blp.toPNG().raw);

            return usePosix ? win32ToPosix(texFileOut) : texFileOut;
          };

          // Export the raw diffuse textures to disk.
          const materials = new Array<AlphaMapMaterial | undefined>(materialIDs.length);
          for (let i = 0, n = materials.length; i < n; i++) {
            const diffuseFileDataID = materialIDs[i];
            const heightFileDataID = heightIDs[i] ?? 0;
            if (diffuseFileDataID === 0) continue;

            const mat: AlphaMapMaterial = { scale: 1, fileDataID: diffuseFileDataID };
            materials[i] = mat;
            mat.file = await saveLayerTexture(diffuseFileDataID);

            // Include a reference to the height map texture if it exists.
            if (heightFileDataID > 0) {
              mat.heightFile = await saveLayerTexture(heightFileDataID);
              mat.heightFileDataID = heightFileDataID;
            }

            if (texParams && texParams[i]) {
              const params = texParams[i];
              mat.scale = 2 ** ((params.flags & 0xF0) >> 4);

              if (params.height !== 0 || params.offset !== 1) {
                mat.heightScale = params.height;
                mat.heightOffset = params.offset;
              }
            }
          }

          // Alpha maps are 64x64, we're not up-scaling here.
          const canvasSize = isSplittingAlphaMaps ? 64 : 64 * 16;
          const canvas = new Uint8ClampedArray(canvasSize * canvasSize * 4);

          const chunks = texAdt.texChunks;
          const chunkCount = chunks.length;

          const layers: Record<string, unknown>[] = [];
          const chunkVertexColors: { chunkIndex: number; shading: number[] }[] = [];

          for (let chunkIndex = 0; chunkIndex < chunkCount; chunkIndex++) {
            progress?.setLabel(`Tile ${this.tileID}, alpha maps`, chunkIndex + 1, chunkCount);
            const texChunk = texAdt.texChunks[chunkIndex];
            const rootChunk = rootAdt.chunks[chunkIndex];

            const fixAlphaMap = !(rootChunk.flags & (1 << 15));

            const alphaLayers = texChunk.alphaLayers || [];
            const imageData = new Uint8ClampedArray(64 * 64 * 4);

            // Write each layer as RGB.
            for (let i = 1; i < alphaLayers.length; i++) {
              const layer = alphaLayers[i];

              for (let j = 0; j < layer.length; j++) {
                const isLastColumn = (j % 64) === 63;
                const isLastRow = j >= 63 * 64;

                // fixAlphaMap: layer is 63x63, fill last column/row.
                if (fixAlphaMap) {
                  if (isLastColumn && !isLastRow) {
                    imageData[(j * 4) + (i - 1)] = layer[j - 1];
                  } else if (isLastRow) {
                    const prevRowIndex = j - 64;
                    imageData[(j * 4) + (i - 1)] = layer[prevRowIndex];
                  } else {
                    imageData[(j * 4) + (i - 1)] = layer[j];
                  }
                } else {
                  imageData[(j * 4) + (i - 1)] = layer[j];
                }
              }
            }

            // Set all the alpha values to max.
            for (let i = 0; i < 64 * 64; i++) imageData[(i * 4) + 3] = 255;

            if (isSplittingAlphaMaps) {
              // Export tile as an individual file.
              const filePrefix = `${this.tileID}_${chunkIndex}`;
              const tilePath = path.join(dir, `tex_${filePrefix}.png`);

              await writePNG(tilePath, imageData, 64, 64);

              const texLayers = texChunk.layers ?? [];
              for (let i = 0, n = texLayers.length; i < n; i++) {
                const layer = texLayers[i];
                const mat = materials[layer.textureId];
                if (mat !== undefined) layers.push({ index: i, effectID: layer.effectID, ...mat });
              }

              const json = new JSONWriter(path.join(dir, `tex_${filePrefix}.json`));
              json.addProperty('layers', layers);

              if (rootChunk.vertexShading) json.addProperty('vertexColors', rootChunk.vertexShading.map((e) => rgbaToInt(e)));

              await json.write();

              layers.length = 0;
            } else {
              const chunkX = chunkIndex % 16;
              const chunkY = Math.floor(chunkIndex / 16);

              // Export as part of a merged alpha map (putImageData equivalent).
              for (let row = 0; row < 64; row++) {
                const srcOfs = row * 64 * 4;
                const dstOfs = ((chunkY * 64 + row) * canvasSize + chunkX * 64) * 4;
                canvas.set(imageData.subarray(srcOfs, srcOfs + 64 * 4), dstOfs);
              }

              const texLayers = texChunk.layers ?? [];
              for (let i = 0, n = texLayers.length; i < n; i++) {
                const layer = texLayers[i];
                const mat = materials[layer.textureId];
                if (mat !== undefined) {
                  layers.push({
                    index: i, chunkIndex, effectID: layer.effectID, ...mat,
                  });
                }
              }

              if (rootChunk.vertexShading) chunkVertexColors.push({ chunkIndex, shading: rootChunk.vertexShading.map((e) => rgbaToInt(e)) });
            }
          }

          // For combined alpha maps, export everything together once done.
          if (!isSplittingAlphaMaps) {
            const mergedPath = path.join(dir, `tex_${this.tileID}.png`);
            await writePNG(mergedPath, canvas, canvasSize, canvasSize);

            const json = new JSONWriter(path.join(dir, `tex_${this.tileID}.json`));
            json.addProperty('layers', layers);

            if (chunkVertexColors.length > 0) json.addProperty('vertexColors', chunkVertexColors);

            await json.write();
          }
        } else if (quality <= 512) {
          // Use minimaps for cheap textures.
          const paddedX = this.tileY.toString().padStart(2, '0');
          const paddedY = this.tileX.toString().padStart(2, '0');
          const tilePath = util.format('world/minimaps/%s/map%s_%s.blp', this.mapDir, paddedX, paddedY);
          const tileOutPath = path.join(dir, `tex_${this.tileID}.png`);

          if (config.overwriteFiles || !await outputFileExists(tileOutPath)) {
            const data = await casc.getFileByName(tilePath, false, true);
            const blp = new BLPImage(data);

            // Decode the BLP and scale the image down (replaces the canvas
            // drawImage scaling in wow.export).
            const raw = blp.toUInt8Array(0, 0b0111);
            const scaled = resizeBilinear(raw, blp.scaledWidth, blp.scaledHeight, quality, quality);

            await writePNG(tileOutPath, scaled, quality, quality);
          } else {
            write('Skipping ADT bake of %s (file exists, overwrite disabled)', tileOutPath);
          }
        } else {
          const tileOutPath = path.join(dir, `tex_${this.tileID}.png`);

          const chunkSizePx = quality / 16;

          let composite: Uint8ClampedArray | undefined;
          if (!isSplittingTextures) composite = new Uint8ClampedArray(quality * quality * 4);

          if (isSplittingTextures || config.overwriteFiles || !await outputFileExists(tileOutPath)) {
            // Materials. Note: height textures are loaded by wow.export but the
            // bake shader never samples them, so we skip loading them entirely.
            const materialIDs = texAdt.diffuseTextureFileDataIDs!;
            const texParams = texAdt.texParams;

            const materials = new Array<BakeMaterial | undefined>(materialIDs.length);
            progress?.setLabel(`Tile ${this.tileID}, loading textures`, 0, materialIDs.length);
            for (let i = 0, n = materials.length; i < n; i++) {
              progress?.setLabel(`Tile ${this.tileID}, loading textures`, i + 1, n);
              const diffuseFileDataID = materialIDs[i];

              if (diffuseFileDataID === 0) continue;

              const mat: BakeMaterial = { scale: 1, heightScale: 0, heightOffset: 1 };
              materials[i] = mat;
              mat.diffuseTex = await loadBakeTexture(diffuseFileDataID);

              if (texParams && texParams[i]) {
                const params = texParams[i];
                mat.scale = 2 ** ((params.flags & 0xF0) >> 4);

                if (params.height !== 0 || params.offset !== 1) {
                  mat.heightScale = params.height;
                  mat.heightOffset = params.offset;
                }
              }
            }

            // Persistent chunk canvas (the GL canvas is cleared once to black
            // and never between chunk draws).
            const chunkCanvas = new Uint8ClampedArray(chunkSizePx * chunkSizePx * 4);
            for (let i = 0; i < chunkCanvas.length; i += 4) {
              chunkCanvas[i + 3] = 255;
            }

            const deltaX = firstChunk.position[1] - TILE_SIZE;
            const deltaY = firstChunk.position[0] - TILE_SIZE;

            let bakeChunkID = 0;
            let bakedChunkIndex = 0;
            progress?.setLabel(`Tile ${this.tileID}, baking textures`, 0, 256);
            for (let x = 0; x < 16; x++) {
              for (let y = 0; y < 16; y++) {
                progress?.setLabel(`Tile ${this.tileID}, baking textures`, bakedChunkIndex + 1, 256);
                bakedChunkIndex++;
                const ofsX = -deltaX - (CHUNK_SIZE * 7.5) + (y * CHUNK_SIZE);
                const ofsY = -deltaY - (CHUNK_SIZE * 7.5) + (x * CHUNK_SIZE);

                const chunkIndex = (x * 16) + y;
                const texChunk = texAdt.texChunks[chunkIndex];
                const indices = chunkMeshes[chunkIndex];

                const alphaLayersRaw = texChunk.alphaLayers || [];

                // If MCNK do_not_fix_alpha_map flag is not set, duplicate last
                // row/column for 63x63 alpha maps to avoid seams (Noggit behavior).
                const fixAlphaMap = !(rootAdt.chunks[chunkIndex].flags & (1 << 15));

                const alphaLayers = new Array<number[] | Uint8Array | undefined>(alphaLayersRaw.length);
                for (let i = 1; i < alphaLayersRaw.length; i++) {
                  let source: number[] | Uint8Array = alphaLayersRaw[i];
                  if (fixAlphaMap && source && source.length === 64 * 64) {
                    const fixed = new Uint8Array(64 * 64);
                    for (let j = 0; j < 64 * 64; j++) {
                      const isLastColumn = (j % 64) === 63;
                      const isLastRow = j >= 63 * 64;
                      if (isLastColumn && !isLastRow) {
                        fixed[j] = source[j - 1];
                      } else if (isLastRow) {
                        fixed[j] = source[j - 64];
                      } else {
                        fixed[j] = source[j];
                      }
                    }
                    source = fixed;
                  }
                  alphaLayers[i] = source;
                }

                // Map texture layers to bake materials (slots 0-3).
                const texLayers = texChunk.layers ?? [];
                const layerMats = new Array<BakeMaterial | undefined>(4);
                for (let i = 0, n = texLayers.length; i < n; i++) {
                  const mat = materials[texLayers[i].textureId];
                  if (mat === undefined) continue;
                  if (i < 4) layerMats[i] = mat;
                }

                bakeChunk({
                  canvas: chunkCanvas,
                  canvasSize: chunkSizePx,
                  indices,
                  vertices,
                  uvsBake,
                  vertexColors,
                  translation: [ofsX, ofsY],
                  tileSize: TILE_SIZE,
                  zoom: 0.0625,
                  layers: layerMats,
                  alphaLayers,
                });

                const rotated = rotate180(chunkCanvas, chunkSizePx);

                if (isSplittingTextures) {
                  // Save this individual chunk.
                  const tilePath = path.join(dir, `tex_${this.tileID}_${bakeChunkID++}.png`);

                  if (config.overwriteFiles || !await outputFileExists(tilePath)) {
                    await writePNG(tilePath, rotated, chunkSizePx, chunkSizePx);
                  }
                } else {
                  // Store as part of a larger composite.
                  const chunkX = chunkIndex % 16;
                  const chunkY = Math.floor(chunkIndex / 16);

                  for (let row = 0; row < chunkSizePx; row++) {
                    const srcOfs = row * chunkSizePx * 4;
                    const dstOfs = ((chunkY * chunkSizePx + row) * quality + chunkX * chunkSizePx) * 4;
                    composite!.set(rotated.subarray(srcOfs, srcOfs + chunkSizePx * 4), dstOfs);
                  }
                }
              }
            }

            // Save the completed composite tile.
            if (!isSplittingTextures) {
              progress?.setLabel(`Tile ${this.tileID}, saving terrain texture`);
              await writePNG(tileOutPath, composite!, quality, quality);
            }
          }
        }
        progress?.advance();
      }
    } else {
      const saveRawLayerTexture = async (fileDataID: number): Promise<string | undefined> => {
        if (fileDataID === 0) return undefined;

        const blp = await casc.getFile(fileDataID);

        let fileName = listfile.getByID(fileDataID);
        if (fileName !== undefined) fileName = replaceExtension(fileName, '.blp');
        else fileName = listfile.formatUnknownFile(fileDataID, '.blp');

        let texFileOut: string;
        let texPath: string;

        if (config.enableSharedTextures) {
          texPath = getExportPath(fileName);
          texFileOut = path.relative(dir, texPath);
        } else {
          texPath = path.join(dir, path.basename(fileName));
          texFileOut = path.basename(texPath);
        }

        await writeOutputFile(texPath, blp.raw);

        return usePosix ? win32ToPosix(texFileOut) : texFileOut;
      };

      const materialIDs = texAdt.diffuseTextureFileDataIDs ?? [];
      for (const fileDataID of materialIDs) await saveRawLayerTexture(fileDataID);

      const heightIDs = texAdt.heightTextureFileDataIDs ?? [];
      for (const fileDataID of heightIDs) await saveRawLayerTexture(fileDataID);
    }

    // Export doodads / WMOs.
    if (config.mapsIncludeWMO || config.mapsIncludeM2 || config.mapsIncludeGameObjects) {
      progress?.setLabel(`Tile ${this.tileID}, model placements`);
      const objectCache = new Set<number | string>();

      const csvPath = path.join(dir, `adt_${this.tileID}_ModelPlacementInformation.csv`);
      if (config.overwriteFiles || !await outputFileExists(csvPath)) {
        const csv = new CSVWriter(csvPath);
        csv.addField('ModelFile', 'PositionX', 'PositionY', 'PositionZ', 'RotationX', 'RotationY', 'RotationZ', 'RotationW', 'ScaleFactor', 'ModelId', 'Type', 'FileDataID', 'DoodadSetIndexes', 'DoodadSetNames');

        const exportObjects = async (exportType: string, objects: Iterable<ADTExportObject> & { length?: number; size?: number }, csvName: string): Promise<void> => {
          const nObjects = (objects as { length?: number }).length ?? (objects as { size?: number }).size ?? 0;
          if (config.mapsDirectModels) {
            write('Writing %d %s placements to CSV...', nObjects, exportType);
          } else {
            write('Exporting %d %s for ADT...', nObjects, exportType);
          }

          let objectIndex = 0;
          for (const model of objects) {
            progress?.setLabel(`Tile ${this.tileID}, ${exportType}`, objectIndex + 1, nObjects);
            const fileDataID = getPlacementFileDataID(model);
            const kind = csvName === 'wmo' ? 'wmo' : 'm2';

            try {
              let modelFile: string;
              if (!config.mapsDirectModels) {
                let fileName = listfile.getByID(fileDataID);

                if (!config.mapsExportRaw) {
                  if (fileName !== undefined) {
                    fileName = replaceExtension(fileName, '.obj');
                  } else {
                    fileName = listfile.formatUnknownFile(fileDataID, '.obj');
                  }
                }

                let modelPath: string;
                if (config.enableSharedChildren) modelPath = getExportPath(fileName!);
                else modelPath = path.join(dir, path.basename(fileName!));

                if (!objectCache.has(fileDataID)) {
                  const data = await casc.getFile(fileDataID);
                  const m2 = new M2Exporter(data, undefined, fileDataID);

                  if (config.mapsExportRaw) throw new Error('Raw M2 export is not supported by the native ADT exporter');
                  await m2.exportAsOBJ(modelPath, undefined, config.modelsExportCollision, progress);

                  objectCache.add(fileDataID);
                }

                modelFile = path.relative(dir, modelPath);
              } else {
                const fileName = modelReferencePath(fileDataID, kind);
                const modelPath = resolveModelStoragePath(fileName, dir, config.enableSharedChildren, getExportPath);
                objectCache.add(fileDataID);
                modelFile = path.relative(dir, modelPath);
              }

              if (usePosix) modelFile = win32ToPosix(modelFile);

              csv.addRow({
                ModelFile: modelFile,
                PositionX: getPlacementCoord(model, 'position', 0),
                PositionY: getPlacementCoord(model, 'position', 1),
                PositionZ: getPlacementCoord(model, 'position', 2),
                RotationX: getPlacementCoord(model, 'rotation', 0),
                RotationY: getPlacementCoord(model, 'rotation', 1),
                RotationZ: getPlacementCoord(model, 'rotation', 2),
                RotationW: getPlacementCoord(model, 'rotation', 3),
                ScaleFactor: getPlacementScaleFactor(model),
                ModelId: getPlacementUniqueId(model),
                Type: csvName,
                FileDataID: fileDataID,
                DoodadSetIndexes: 0,
                DoodadSetNames: '',
              });
              if (!config.mapsDirectModels) progress?.advance(1);
              objectIndex++;
            } catch (e) {
              write('Failed to export model [%d]: %s', fileDataID, (e as Error).message);
              objectIndex++;
            }
          }
        };

        if (config.mapsIncludeGameObjects === true && gameObjects !== undefined && gameObjects.size > 0) await exportObjects('game objects', gameObjects, 'gobj');

        if (config.mapsIncludeM2) {
          await exportObjects('doodads', objAdt.models ?? [], 'm2');
        }

        if (config.mapsIncludeWMO) {
          if (config.mapsDirectModels) {
            write('Writing %d WMO placements to CSV...', objAdt.worldModels?.length ?? 0);
          } else {
            write('Exporting %d WMOs for ADT...', objAdt.worldModels?.length ?? 0);
          }

          const setNameCache = new Map<number, string[]>();

          const usingNames = !!objAdt.wmoNames;
          const worldModels = objAdt.worldModels ?? [];
          let worldModelIndex = 0;
          for (const model of worldModels) {
            progress?.setLabel(`Tile ${this.tileID}, WMO objects`, worldModelIndex + 1, worldModels.length);
            const useADTSets = (model as unknown as number) & 0x80;

            let fileDataID: number | undefined;
            let fileName: string | undefined;

            try {
              if (usingNames) {
                fileName = objAdt.wmoNames![objAdt.wmoOffsets![model.mwidEntry]];
                fileDataID = listfile.getByFilename(fileName);
              } else {
                fileDataID = model.mwidEntry;
                fileName = listfile.getByID(fileDataID);
              }

              const doodadSets = useADTSets ? objAdt.doodadSets! : [model.doodadSet];
              const cacheID = `${fileDataID}-${doodadSets.join(',')}`;

              let modelPath: string;
              if (config.mapsDirectModels) {
                const refName = modelReferencePath(fileDataID!, 'wmo', model.doodadSet);
                modelPath = resolveModelStoragePath(refName, dir, config.enableSharedChildren, getExportPath);
              } else {
                if (!config.mapsExportRaw) {
                  if (fileName !== undefined) {
                    fileName = replaceExtension(fileName, `_set${model.doodadSet}.obj`);
                  } else {
                    fileName = listfile.formatUnknownFile(fileDataID!, `_set${model.doodadSet}.obj`);
                  }
                }
                if (config.enableSharedChildren) modelPath = getExportPath(fileName!);
                else modelPath = path.join(dir, path.basename(fileName!));
              }

              if (!objectCache.has(cacheID)) {
                const data = await casc.getFile(fileDataID!);
                const wmoLoader = new WMOExporter(data, fileDataID!);

                wmoLoader.wmo.load();

                setNameCache.set(fileDataID!, wmoLoader.wmo.doodadSets!.map((e) => e.name));

                if (config.mapsIncludeWMOSets) {
                  const mask: Record<number, { checked: boolean }> = { 0: { checked: true } };
                  if (useADTSets) {
                    for (const setIndex of objAdt.doodadSets!) mask[setIndex] = { checked: true };
                  } else {
                    mask[model.doodadSet] = { checked: true };
                  }

                  wmoLoader.setDoodadSetMask(mask as unknown as { checked: boolean }[]);
                }

                if (config.mapsExportRaw) throw new Error('Raw WMO export is not supported by the native ADT exporter');
                if (config.mapsDirectModels) {
                  await wmoLoader.exportDoodadPlacementCsv(modelPath, config, progress, true);
                } else {
                  await wmoLoader.exportAsOBJ(modelPath, undefined, progress);
                }

                objectCache.add(cacheID);
              }

              const doodadNames = setNameCache.get(fileDataID!)!;

              let modelFile = path.relative(dir, modelPath);
              if (usePosix) modelFile = win32ToPosix(modelFile);

              csv.addRow({
                ModelFile: modelFile,
                PositionX: model.position[0],
                PositionY: model.position[1],
                PositionZ: model.position[2],
                RotationX: model.rotation[0],
                RotationY: model.rotation[1],
                RotationZ: model.rotation[2],
                RotationW: 0,
                ScaleFactor: model.scale / 1024,
                ModelId: model.uniqueId,
                Type: 'wmo',
                FileDataID: fileDataID!,
                DoodadSetIndexes: doodadSets.join(','),
                DoodadSetNames: doodadSets.map((e) => doodadNames[e]).join(','),
              });
              if (!config.mapsDirectModels) progress?.advance(1);
              worldModelIndex++;
            } catch (e) {
              write('Failed to export WMO [%d]: %s', fileDataID, (e as Error).message);
              worldModelIndex++;
            }
          }

          WMOExporter.clearCache();
        }

        await csv.write();
      } else {
        write('Skipping model placement export %s (file exists, overwrite disabled)', csvPath);
      }

      if (config.mapsDirectModels) progress?.advance(1);
    }

    // Export liquids.
    if (config.mapsIncludeLiquid && rootAdt.liquidChunks) {
      const liquidFile = path.join(dir, `liquid_${this.tileID}.json`);
      write('Exporting liquid data to %s', liquidFile);
      progress?.setLabel(`Tile ${this.tileID}, liquid`);
      progress?.advance();

      const enhancedLiquidChunks = rootAdt.liquidChunks.map((chunk, chunkIndex) => {
        if (!chunk || !chunk.instances) return chunk;

        const terrainChunk = rootAdt.chunks[chunkIndex];
        const enhancedInstances = chunk.instances.map((instance) => {
          if (!instance) return instance;

          const chunkX = terrainChunk.position[0];
          const chunkY = terrainChunk.position[1];
          const chunkZ = terrainChunk.position[2];

          const centerX = instance.xOffset + instance.width / 2;
          const centerY = instance.yOffset + instance.height / 2;

          const worldX = chunkY - (centerX * UNIT_SIZE);
          const worldY = (instance.minHeightLevel + instance.maxHeightLevel) / 2 + chunkZ;
          const worldZ = chunkX - (centerY * UNIT_SIZE);

          return {
            ...instance,
            worldPosition: [worldX, worldY, worldZ],
            terrainChunkPosition: [chunkX, chunkY, chunkZ],
          };
        });

        return {
          ...chunk,
          instances: enhancedInstances,
        };
      });

      const liquidJSON = new JSONWriter(liquidFile);
      liquidJSON.addProperty('liquidChunks', enhancedLiquidChunks);
      await liquidJSON.write();
    }

    // Prepare foliage data tables if needed.
    if (config.mapsIncludeFoliage && !hasLoadedFoliage) await loadFoliageTables();

    // Export foliage.
    if (config.mapsIncludeFoliage && isFoliageAvailable) {
      const foliageExportCache = new Set<number>();
      const foliageEffectCache = new Set<number>();
      const foliageDir = path.join(dir, 'foliage');

      write('Exporting foliage to %s', foliageDir);
      progress?.setLabel(`Tile ${this.tileID}, foliage`);

      for (const chunk of texAdt.texChunks) {
        // Skip chunks that have no layers?
        if (!chunk.layers) continue;

        for (const layer of chunk.layers) {
          // Skip layers with no effect.
          if (!layer.effectID) continue;

          const groundEffectTexture = dbTextures!.getRow(layer.effectID);
          if (!groundEffectTexture || !Array.isArray(groundEffectTexture.DoodadID)) continue;

          // Create a foliage metadata JSON packed with the table data.
          // NOTE: wow.export reads exportFoliageMeta from the global config (quirk preserved).
          let foliageJSON: JSONWriter | undefined;
          if (wowConfig.exportFoliageMeta && !foliageEffectCache.has(layer.effectID)) {
            foliageJSON = new JSONWriter(path.join(foliageDir, `${layer.effectID}.json`));
            foliageJSON.data = groundEffectTexture as Record<string, unknown>;

            foliageEffectCache.add(layer.effectID);
          }

          const doodadModelIDs: Record<number, { fileDataID: number; fileName?: string }> = {};
          for (const doodadEntryID of groundEffectTexture.DoodadID as number[]) {
            // Skip empty fields.
            if (!doodadEntryID) continue;

            const groundEffectDoodad = dbDoodads!.getRow(doodadEntryID);
            if (groundEffectDoodad) {
              const modelID = groundEffectDoodad.ModelFileID as number;
              doodadModelIDs[doodadEntryID] = { fileDataID: modelID };
              if (!modelID || foliageExportCache.has(modelID)) continue;

              foliageExportCache.add(modelID);
            }
          }

          if (foliageJSON) {
            for (const entry of Object.values(doodadModelIDs)) {
              const fileName = listfile.getByID(entry.fileDataID)!;

              if (config.mapsExportRaw) entry.fileName = path.basename(fileName);
              else if (config.mapsDirectModels) entry.fileName = path.basename(modelReferencePath(entry.fileDataID, 'm2'));
              else entry.fileName = replaceExtension(path.basename(fileName), '.obj');
            }

            foliageJSON.addProperty('DoodadModelIDs', doodadModelIDs);
            await foliageJSON.write();
          }
        }
      }

      // Export foliage after collecting to give an accurate progress count.
      const foliageModels = [...foliageExportCache];
      progress?.setLabel(`Tile ${this.tileID}, foliage doodads`, 0, foliageModels.length);
      if (!config.mapsDirectModels) {
        for (let foliageIndex = 0; foliageIndex < foliageModels.length; foliageIndex++) {
          const modelID = foliageModels[foliageIndex];
          progress?.setLabel(`Tile ${this.tileID}, foliage doodads`, foliageIndex + 1, foliageModels.length);
          const modelName = path.basename(listfile.getByID(modelID)!);

          const data = await casc.getFile(modelID);
          const m2 = new M2Exporter(data, undefined, modelID);

          if (config.mapsExportRaw) {
            throw new Error('Raw foliage export is not supported by the native ADT exporter');
          } else {
            const modelPath = replaceExtension(modelName, '.obj');
            await m2.exportAsOBJ(path.join(foliageDir, modelPath), undefined, config.modelsExportCollision, progress);
          }
        }
      }
      progress?.advance(1);
    }

    progress?.syncTileComplete();
    return out;
  }

  /** Clear internal tile-loading cache. */
  static clearCache(): void {
    wdtCache.clear();
  }
}

export default ADTExporter;

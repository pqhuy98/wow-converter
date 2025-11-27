import chalk from 'chalk';
import { mkdir, statfs, writeFile } from 'fs/promises';
import { exists } from 'fs-extra';
import path from 'path';
import sharp from 'sharp';

import { maxConcurrency } from '@/lib/constants';
import { pngsToBlps, readBlpSizeSync } from '@/lib/formats/blp/blp';
import { resizePng } from '@/lib/formats/png';
import { workerPool } from '@/lib/utils';

import { Config } from '../../global-config';
import { EulerRotation, Vector3 } from '../../math/common';
import { calculateChildAbsoluteEulerRotation } from '../../math/rotation';
import { V3 } from '../../math/vector';
import { convertWowExportModel } from '../../objmdl';
import { Model, WowObject } from './models';

export class AssetManager {
  models = new Map<string, Model>();

  textures = new Set<string>();

  texturesOverwrite = new Set<string>();

  constructor(private config: Config) {
  }

  async parse(objectPath: string, noCache: boolean): Promise<Model> {
    if (this.models.has(objectPath) && !noCache) {
      return this.models.get(objectPath)!;
    }

    const objRelativePath = objectPath.endsWith('.obj') ? objectPath : `${objectPath}.obj`;
    const objFullPath = path.join(this.config.wowExportAssetDir, objRelativePath);
    const { mdl, texturePaths } = await convertWowExportModel(objFullPath, this.config);
    const model: Model = {
      relativePath: mdl.model.name,
      mdl,
    };
    if (!noCache) {
      this.models.set(objectPath, model);
    }
    texturePaths.forEach((p) => this.textures.add(p));
    return model;
  }

  async exportModels(assetPath: string) {
    console.log('Exporting models to', assetPath, '...');
    const start = performance.now();
    await mkdir(assetPath, { recursive: true });
    let writeCount = 0;
    // Before exporting, harmonize normals across adjacent ADT tiles to avoid
    // lighting seams between tile models.
    this.smoothAdtTileBorders();
    await workerPool(maxConcurrency, Array.from(this.models.entries()).map(([relativePath, model]) => async () => {
      const fullPath = `${path.join(assetPath, this.config.assetPrefix, relativePath)}.${this.config.mdx ? 'mdx' : 'mdl'}`;

      if (!this.config.overrideModels && await exists(fullPath)) {
        // console.log('Skipping model already exists', fullPath);
        return;
      }

      const mdl = model.mdl;
      if (mdl.model.boundsRadius > this.config.infiniteExtentBoundRadiusThreshold) {
        mdl.modify.setLargeBounds();
      }
      await mkdir(path.dirname(fullPath), { recursive: true });
      const data = this.config.mdx ? model.mdl.toMdx() : model.mdl.toMdl();
      await writeFile(fullPath, data);
      writeCount++;
    }));
    const durationS = (performance.now() - start) / 1000;
    console.log(
      `Models export took ${chalk.yellow(durationS.toFixed(2))} s`,
      `(${chalk.gray((writeCount / durationS).toFixed(2))} models/s)`,
    );
  }

  addPngTexture(texturePath: string, overwrite = false) {
    this.textures.add(texturePath);
    if (overwrite) {
      this.texturesOverwrite.add(texturePath);
    }
  }

  async exportTextures(assetPath: string) {
    console.log('Exporting textures to', assetPath, '...');
    const start = performance.now();

    const exportedTexturePaths: string[] = [];
    await mkdir(assetPath, { recursive: true });
    let writeCount = 0;

    // Collect all textures that need processing
    const texturesToProcess: Array<{
      png: string | Buffer;
      blpPath: string;
    }> = [];
    for (const texturePath of this.textures) {
      const fromPath = path.join(this.config.wowExportAssetDir, texturePath);
      if (!await exists(fromPath)) {
        console.warn('Skipping texture not found', fromPath);
        continue;
      }

      // Read source PNG dimensions once so we can compute the target size for current limit
      const maxSize = this.config.maxTextureSize ?? Infinity;
      let width = 0;
      let height = 0;
      try {
        const meta = await sharp(fromPath).metadata();
        width = meta.width ?? 0;
        height = meta.height ?? 0;
      } catch (err) {
        console.warn('Failed to read PNG metadata, proceeding without resize:', fromPath, err);
        console.log(await statfs(fromPath));
      }

      // Compute target size for current limit; if limit increased, target grows accordingly
      const scale = Math.min(1, maxSize / Math.max(width, height));
      const targetWidth = Math.round(width * scale);
      const targetHeight = Math.round(height * scale);

      // Skip only if the existing BLP exactly matches the target size
      const debug = false;
      const blpPath = path.join(assetPath, this.config.assetPrefix, texturePath.replace('.png', '.blp'));
      exportedTexturePaths.push(blpPath);
      if (await exists(blpPath) && !this.texturesOverwrite.has(texturePath)) {
        const size = readBlpSizeSync(blpPath);
        if (size && size.width === targetWidth && size.height === targetHeight) {
          debug && console.log('Skipping existing texture', blpPath);
          continue;
        }
      }

      // Now we need to export the texture again and resize it if needed
      let pngInput: string | Buffer = fromPath;
      if (this.config.maxTextureSize) {
        try {
          if ((width > targetWidth) || (height > targetHeight)) {
            debug && console.log('Resizing texture', fromPath, width, height, 'to', targetWidth, targetHeight);
            pngInput = await resizePng(fromPath, targetWidth, targetHeight);
          }
        } catch (err) {
          console.warn('Failed to read PNG metadata, proceeding without resize:', fromPath, err);
        }
      }
      writeCount++;
      texturesToProcess.push({ png: pngInput, blpPath });
    }

    // Process textures in parallel using the new non-blocking conversion
    if (texturesToProcess.length > 0) {
      await pngsToBlps(texturesToProcess);
      const durationS = (performance.now() - start) / 1000;
      console.log(
        `Texture BLP conversion took ${chalk.yellow(durationS.toFixed(2))} s`,
        `(${chalk.gray((texturesToProcess.length / durationS).toFixed(2))} textures/s)`,
      );
    }

    console.log(`Wrote ${chalk.yellow(writeCount)}, skipped ${chalk.gray(this.textures.size - writeCount)} textures. Total: ${chalk.yellow(exportedTexturePaths.length)}`);
    return exportedTexturePaths;
  }

  purgeTextures(usedTexturePaths: string[]) {
    const removeExt = (p: string) => p.replace('.blp', '').replace('.png', '');

    const usedTextures = new Set(usedTexturePaths.map(
      (p) => path.relative(this.config.assetPrefix, removeExt(p)),
    ));
    this.textures.forEach((texturePath) => {
      if (!usedTextures.has(removeExt(texturePath))) {
        this.textures.delete(texturePath);
      }
    });
  }

  // -------- ADT helpers ---------------------------------------------------
  // Average normals on shared borders between adjacent ADT tiles to remove
  // visible lighting seams once in game. Operates in-place on stored models.
  smoothAdtTileBorders() {
    // Collect ADT models by tile indices
    type TileKey = `${number}_${number}`;
    const tiles = new Map<TileKey, Model>();
    const re = /adt_(\d+)_(\d+)$/i;

    this.models.forEach((model) => {
      // model.mdl.model.name looks like ".../adt_27_32"
      const base = path.basename(model.mdl.model.name);
      const m = base.match(re);
      if (m) {
        const key = `${Number(m[1])}_${Number(m[2])}` as TileKey;
        tiles.set(key, model);
      }
    });

    const get = (x: number, y: number) => tiles.get(`${x}_${y}` as TileKey);

    const getMinMax = (mdl: Model) => {
      let minX = Infinity; let maxX = -Infinity;
      let minZ = Infinity; let maxZ = -Infinity;
      mdl.mdl.geosets.forEach((g) => g.vertices.forEach((v) => {
        minX = Math.min(minX, v.position[0]);
        maxX = Math.max(maxX, v.position[0]);
        minZ = Math.min(minZ, v.position[2]);
        maxZ = Math.max(maxZ, v.position[2]);
      }));
      return {
        minX, maxX, minZ, maxZ,
      };
    };

    const POS_EPS = 1e-2; // match recompute-normals tolerance
    const BORDER_EPS = 100; // select a small ring (~half unit) around border
    const q = (x: number) => Math.round(x / POS_EPS) * POS_EPS;

    const averageBorder = (a: Model, b: Model, axis: 'x' | 'z') => {
      const {
        /* minX: _aMinX, */ maxX: aMaxX, /* minZ: _aMinZ, */ maxZ: aMaxZ,
      } = getMinMax(a);
      const {
        minX: bMinX, /* maxX: _bMaxX, */ minZ: bMinZ, /* maxZ: _bMaxZ, */
      } = getMinMax(b);

      // Determine shared line coordinate and iterate along the orthogonal axis.
      let aSel: (v: { position: [number, number, number] }) => boolean;
      let bSel: (v: { position: [number, number, number] }) => boolean;
      let keyFrom: (v: { position: [number, number, number] }) => number;

      if (axis === 'x') {
        const ax = aMaxX; // east border of A
        const bx = bMinX; // west border of B
        const borderX = (ax + bx) * 0.5;
        aSel = (v) => Math.abs(v.position[0] - borderX) <= BORDER_EPS;
        bSel = (v) => Math.abs(v.position[0] - borderX) <= BORDER_EPS;
        keyFrom = (v) => q(v.position[2]); // use Z coordinate to pair
      } else {
        const az = aMaxZ; // north border of A
        const bz = bMinZ; // south border of B
        const borderZ = (az + bz) * 0.5;
        aSel = (v) => Math.abs(v.position[2] - borderZ) <= BORDER_EPS;
        bSel = (v) => Math.abs(v.position[2] - borderZ) <= BORDER_EPS;
        keyFrom = (v) => q(v.position[0]); // use X coordinate to pair
      }

      const aMap = new Map<number, GeosetVertex[]>();
      const bMap = new Map<number, GeosetVertex[]>();

      a.mdl.geosets.forEach((g) => g.vertices.forEach((v) => {
        if (aSel(v)) {
          const k = keyFrom(v);
          const arr = aMap.get(k) ?? [];
          arr.push(v);
          aMap.set(k, arr);
        }
      }));
      b.mdl.geosets.forEach((g) => g.vertices.forEach((v) => {
        if (bSel(v)) {
          const k = keyFrom(v);
          const arr = bMap.get(k) ?? [];
          arr.push(v);
          bMap.set(k, arr);
        }
      }));

      // For each matched position along the border (ring), average normals
      const keys = new Set<number>([...aMap.keys(), ...bMap.keys()]);
      keys.forEach((k) => {
        const aVerts = aMap.get(k);
        const bVerts = bMap.get(k);
        if (!aVerts || !bVerts) return;
        let nx = 0; let ny = 0; let nz = 0;
        const add = (v: GeosetVertex) => { nx += v.normal[0]; ny += v.normal[1]; nz += v.normal[2]; };
        aVerts.forEach(add);
        bVerts.forEach(add);
        const len = Math.hypot(nx, ny, nz) || 1;
        const avg: [number, number, number] = [nx / len, ny / len, nz / len];
        const set = (v: GeosetVertex) => { v.normal = [avg[0], avg[1], avg[2]]; };
        aVerts.forEach(set);
        bVerts.forEach(set);
      });
    };

    // Types used in helper above
    type GeosetVertex = { position: [number, number, number], normal: [number, number, number] };

    // Enumerate all tiles and harmonize with east and north neighbors
    tiles.forEach((model, key) => {
      const [sx, sy] = key.split('_').map(Number);
      const east = get(sx + 1, sy);
      if (east) averageBorder(model, east, 'x');
      const north = get(sx, sy + 1);
      if (north) averageBorder(model, north, 'z');
    });
  }
}

export function computeAbsoluteMinMaxExtents(objs: WowObject[]) {
  let min = V3.all(Infinity);
  let max = V3.all(-Infinity);

  function isEmpty(obj: WowObject) {
    return obj.model!.mdl.geosets.every((geoset) => geoset.vertices.length === 0);
  }

  objs.forEach((obj) => {
    let nodes = [obj];
    let basePosition: Vector3 = [0, 0, 0];
    let baseRotation: EulerRotation = [0, 0, 0];

    if (isEmpty(obj)) {
      nodes = obj.children;
      basePosition = obj.position;
      baseRotation = obj.rotation;
    }

    nodes.forEach((node) => {
      const position = V3.sum(basePosition, V3.rotate(node.position, baseRotation));
      const rotation = calculateChildAbsoluteEulerRotation(baseRotation, node.rotation);
      if (!node.model) {
        return;
      }
      node.model.mdl.geosets.forEach((geoset) => {
        geoset.vertices.forEach((v) => {
          const rotatedV = V3.rotate(v.position, rotation);
          const positionV = V3.sum(position, rotatedV);
          min = V3.min(min, positionV);
          max = V3.max(max, positionV);
        });
      });
    });
  });
  return { min, max };
}

export function computeModelMinMaxExtents(objs: WowObject[]) {
  let min = V3.all(Infinity);
  let max = V3.all(-Infinity);
  objs.forEach((obj) => {
    obj.model!.mdl.geosets.forEach((geoset) => {
      geoset.vertices.forEach((v) => {
        min = V3.min(min, v.position);
        max = V3.max(max, v.position);
      });
    });
  });
  return { min, max };
}

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
      node.model!.mdl.geosets.forEach((geoset) => {
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

import { existsSync, mkdirSync, writeFileSync } from 'fs';
import path from 'path';

import { pngToBlp } from '../../blp/blp';
import { Config } from '../../global-config';
import { EulerRotation, Vector3 } from '../../math/common';
import { calculateChildAbsoluteEulerRotation } from '../../math/rotation';
import { V3 } from '../../math/vector';
import { convertWowExportModel } from '../../objmdl';
import { Model, WowObject } from './models';

export class AssetManager {
  models = new Map<string, Model>();

  textures = new Set<string>();

  constructor(private config: Config) {
  }

  parse(objectPath: string, noCache: boolean): Model {
    if (this.models.has(objectPath) && !noCache) {
      return this.models.get(objectPath)!;
    }

    const objRelativePath = objectPath.endsWith('.obj') ? objectPath : `${objectPath}.obj`;
    const objFullPath = path.join(this.config.wowExportAssetDir, objRelativePath);
    // console.log('Parsing model', objFullPath);
    const { mdl, texturePaths } = convertWowExportModel(objFullPath, this.config);
    const model: Model = {
      relativePath: path.join(this.config.assetPrefix, `${objectPath}.mdl`),
      mdl,
    };
    if (!noCache) {
      this.models.set(objectPath, model);
    }
    texturePaths.forEach((p) => this.textures.add(p));
    return model;
  }

  exportModels(assetPath: string) {
    console.log('Exporting models to', assetPath);
    mkdirSync(assetPath, { recursive: true });
    for (const [relativePath, model] of this.models.entries()) {
      const fullPath = `${path.join(assetPath, this.config.assetPrefix, relativePath)}.${this.config.mdx ? 'mdx' : 'mdl'}`;

      if (!this.config.overrideModels && existsSync(fullPath)) {
        // console.log('Skipping model already exists', fullPath);
        continue;
      }

      const mdl = model.mdl;
      if (mdl.model.boundsRadius > this.config.infiniteExtentBoundRadiusThreshold) {
        mdl.modify.setLargeExtents();
      }
      mkdirSync(path.dirname(fullPath), { recursive: true });
      const data = this.config.mdx ? model.mdl.toMdx() : model.mdl.toMdl();
      writeFileSync(fullPath, data);
    }
  }

  addPngTexture(texturePath: string) {
    this.textures.add(texturePath);
  }

  async exportTextures(assetPath: string) {
    const exportedTexturePaths: string[] = [];
    console.log('Exporting textures to', assetPath);
    mkdirSync(assetPath, { recursive: true });
    await Promise.all(Array.from(this.textures).map(async (texturePath) => {
      const toPath = path.join(assetPath, this.config.assetPrefix, texturePath.replace('.png', '.blp'));
      if (existsSync(toPath)) {
        exportedTexturePaths.push(toPath);
        return;
      }
      const fromPath = path.join(this.config.wowExportAssetDir, texturePath);
      if (!existsSync(fromPath)) {
        console.warn('Skipping texture not found', fromPath);
        return;
      }
      await pngToBlp(fromPath, toPath);
      exportedTexturePaths.push(toPath);
    }));
    console.log(`Exported ${exportedTexturePaths.length} textures`);
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

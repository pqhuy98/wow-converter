import { existsSync, mkdirSync, writeFileSync } from 'fs';
import path from 'path';

import { blp2Image } from '../blp/blp';
import { V3 } from '../math/vector';
import { convertObjMdl } from '../objmdl';
import { Config, Model, WowObject } from './common';

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
    const objFullPath = path.join(this.config.wowExportPath, objRelativePath);
    // console.log('Parsing model', objFullPath);
    const { mdl, texturePaths } = convertObjMdl(objFullPath, this.config.wowExportPath, this.config.assetPrefix, this.config);
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
      const fullPath = `${path.join(assetPath, this.config.assetPrefix, relativePath)}.${this.config.release ? 'mdx' : 'mdl'}`;

      if (!this.config.overrideModels && existsSync(fullPath)) {
        console.log('Skipping model already exists', fullPath);
        continue;
      }

      const mdl = model.mdl;
      if (mdl.model.boundsRadius > this.config.infiniteExtentBoundRadiusThreshold * this.config.rawModelScaleUp) {
        mdl.modify.setLargeExtents();
      }
      mkdirSync(path.dirname(fullPath), { recursive: true });
      const data = this.config.release ? model.mdl.toMdx() : model.mdl.toString();
      writeFileSync(fullPath, data);
    }
  }

  addPngTexture(texturePath: string) {
    this.textures.add(texturePath);
  }

  exportTextures(assetPath: string) {
    const exportedTexturePaths: string[] = [];
    console.log('Exporting textures to', assetPath);
    mkdirSync(assetPath, { recursive: true });
    for (const texturePath of this.textures) {
      const toPath = path.join(assetPath, this.config.assetPrefix, texturePath.replace('.png', '.blp'));
      if (existsSync(toPath)) {
        continue;
      }
      const fromPath = path.join(this.config.wowExportPath, texturePath);
      if (!existsSync(fromPath)) {
        console.warn('Skipping texture not found', fromPath);
        continue;
      }
      blp2Image(fromPath, toPath, 'blp');
      exportedTexturePaths.push(toPath);
    }
    console.log('Exported textures to', assetPath);
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
  objs.forEach((obj) => {
    obj.model!.mdl.geosets.forEach((geoset) => {
      geoset.vertices.forEach((v) => {
        const rotatedV = V3.rotate(v.position, obj.rotation);
        const position = V3.sum(obj.position, rotatedV);
        min = V3.min(min, position);
        max = V3.max(max, position);
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

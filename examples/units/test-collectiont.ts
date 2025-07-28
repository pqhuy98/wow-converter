import { writeFileSync } from 'fs';
import path from 'path';

import { AssetManager } from '@/lib/converter/common/model-manager';
import { getDefaultConfig } from '@/lib/global-config';
import { V3 } from '@/lib/math/vector';
import { AnimationFile } from '@/lib/objmdl/animation/animation';
import { ANIM_NAMES, getWc3AnimName, getWowAnimName } from '@/lib/objmdl/animation/animation_mapper';
import { isUnknownBone } from '@/lib/objmdl/animation/bones_mapper';
import { M2MetadataFile } from '@/lib/objmdl/metadata/m2_metadata';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';

const config = await getDefaultConfig();

async function character() {
  const excludedAnimIds: number[] = [];
  for (let animId = 0; animId < ANIM_NAMES.length; animId++) {
    const wc3Anim = getWc3AnimName(getWowAnimName(animId));

    if (wc3Anim.wc3Name !== 'Walk') {
      excludedAnimIds.push(animId);
    }
  }
  console.log(excludedAnimIds.length);
  await wowExportClient.waitUntilReady();

  const result = await wowExportClient.exportCharacter({
    race: 37,
    gender: 0,
    customizations: {
      360: 8938,
      361: 8966,
      362: 3495,
      363: 3501,
      364: 3508,
      366: 3534,
      367: 3539,
      415: 3524,
      794: 8912,
      797: 8998,
    },
    geosetIds: [],
    format: 'obj',
    include_animations: true,
    include_base_clothing: false,
    excludeAnimationIds: excludedAnimIds,
  });

  const objPath = result.fileManifest.find((f) => f.type === 'OBJ')!.file;
  const assetManager = new AssetManager(config);
  const model = assetManager.parse(path.relative(config.wowExportAssetDir, objPath), true);
  writeFileSync(path.join('exported-assets', 'test.mdl'), model.mdl.toString());
  await assetManager.exportTextures('exported-assets');
  console.log(result);
  const metadata = new M2MetadataFile(objPath.replace(/\.obj$/, '.json'), config);
  const animation = new AnimationFile(objPath.replace(/\.obj$/, '_bones.json'));

  return { model, metadata, animation };
}

export async function collection() {
  await wowExportClient.waitUntilReady();
  const objPath = path.join(config.wowExportAssetDir, 'item\\objectcomponents\\collections\\collections_mechagnome_mg_m_collections_plate_mechagnome_c_01_gn_2618164.obj');
  const assetManager = new AssetManager(config);
  const model = assetManager.parse(path.relative(config.wowExportAssetDir, objPath), true);
  writeFileSync(path.join('exported-assets', 'collection.mdl'), model.mdl.toString());
  await assetManager.exportTextures('exported-assets');
  const metadata = new M2MetadataFile(objPath.replace(/\.obj$/, '.json'), config);
  const animation = new AnimationFile(objPath.replace(/\.obj$/, '_bones.json'));
  return { model, metadata, animation };
}

export async function main() {
  const characterResult = await character();
  const collectionResult = await collection();
  const { model: model1, metadata: meta1, animation: anim1 } = characterResult;
  const { model: model2, metadata: meta2, animation: anim2 } = collectionResult;
  const mdl1 = model1.mdl;
  const mdl2 = model2.mdl;

  const bone1Map = new Map(mdl1.bones.map((bone) => [bone.name, bone]));
  for (const bone of mdl2.bones) {
    let bone1 = bone1Map.get(bone.name);
    if (isUnknownBone(bone)) {
      // find the closest bone in mdl1
      bone1 = mdl1.bones.reduce((closest, bone) => {
        const dist = Math.hypot(...V3.sub(bone.pivotPoint, bone.pivotPoint));
        const closestDist = Math.hypot(...V3.sub(closest.pivotPoint, bone.pivotPoint));
        return dist < 0.0001 && dist < closestDist ? bone : closest;
      }, mdl1.bones[0]);
    }
    if (!bone1) {
      console.log(bone.name, 'not found');
      continue;
    }
    if (bone1.name === bone.name) {
      const match = Math.hypot(...V3.sub(bone.pivotPoint, bone1.pivotPoint)) < 0.0001;
      if (!match) {
        console.log(bone.name, 'not match', bone.pivotPoint, bone1.pivotPoint);
      }
    }
  }
  console.log(true);
}

void main().then(() => process.exit(0));

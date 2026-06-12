import chalk from 'chalk';

import { profileScope } from '@/lib/export-profile';
import { Config } from '@/lib/global-config';

import { assembleWowModel } from '../assemble';
import { AnimationFile } from '../bundle/animation';
import { M2MetadataFile } from '../bundle/metadata';
import { MTLFile } from '../bundle/mtl';
import { OBJFile } from '../bundle/obj';

/** Legacy pipeline: parse wow.export OBJ/MTL/_bones.json/.json files, then assemble MDL. */
export async function convertWowExportModel(objFilePath: string, config: Config): Promise<{ mdl: import('@/lib/formats/mdl/mdl').MDL; texturePaths: Set<string> }> {
  return profileScope('converter/objToMdx', async () => {
    !config.isBulkExport && console.log('Converting OBJ model:', chalk.blue(objFilePath));
    const obj = await profileScope('parseOBJ', () => new OBJFile(objFilePath, config).parse());
    const mtl = await profileScope('parseMTL', () => new MTLFile(objFilePath.replace(/\.obj$/, '.mtl'), config).parse());
    const animation = await profileScope('parseAnimation', () => new AnimationFile(objFilePath.replace(/\.obj$/, '_bones.json'), config).parse());
    const metadata = await profileScope('parseMetadata', () => new M2MetadataFile(objFilePath.replace(/\.obj$/, '.json'), config, animation).parse());
    return assembleWowModel({
      objFilePath, obj, mtl, animation, metadata,
    }, config);
  });
}

import { Config } from '@/lib/global-config';

import { assembleWowModel } from './assemble';
import { AnimationFile } from './bundle/animation';
import { M2MetadataFile } from './bundle/metadata';
import { MTLFile } from './bundle/mtl';
import { OBJFile } from './bundle/obj';

/** Parse ADT terrain OBJ/MTL from map export, then assemble MDL. */
export async function convertAdtTerrainObjToMdl(
  objFilePath: string,
  config: Config,
): Promise<{ mdl: import('@/lib/formats/mdl/mdl').MDL; texturePaths: Set<string> }> {
  const obj = await new OBJFile(objFilePath, config).parse();
  const mtl = await new MTLFile(objFilePath.replace(/\.obj$/, '.mtl'), config).parse();
  const animation = await new AnimationFile(objFilePath.replace(/\.obj$/, '_bones.json'), config).parse();
  const metadata = await new M2MetadataFile(objFilePath.replace(/\.obj$/, '.json'), config, animation).parse();
  return assembleWowModel({
    objFilePath, obj, mtl, animation, metadata,
  }, config);
}

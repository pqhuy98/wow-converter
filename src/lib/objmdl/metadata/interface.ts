import { AnimationFile } from '../animation/animation';
import {
  Geoset, GlobalSequence, Material, MDL,
} from '../mdl/mdl';

export interface MetadataFile {
  extractMDLGeosetAnim(animFile: AnimationFile, geosets: Geoset[]): MDL['geosetAnims']
  extractMDLTexturesMaterials(
    texturePrefix: string, numGeosets: number, animFile: AnimationFile,
    globalSequences: GlobalSequence[]
  ): Pick<MDL, 'textures' | 'materials' | 'textureAnims'> & {geosetToMat: Map<number, Material>}
}

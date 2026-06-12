import type { M2Loader } from '@/lib/wow/formats/m2/m2-loader';
import type { SKELLoader } from '@/lib/wow/formats/m2/skel-loader';

/** Creature variant texture: numeric fileDataID or a data-texture key. */
export type VariantTexture = number | string;

export interface GeosetMaskEntry {
  checked: boolean;
}

export interface TextureManifestEntry {
  matName: string;
  matPathRelative: string;
  matPath: string;
}

export interface FileManifestEntry {
  type: string;
  fileDataID: number | string;
  file: string;
}

export interface SkeletonLike {
  bones?: NonNullable<SKELLoader['bones'] | M2Loader['bones']>;
  animations: SKELLoader['animations'];
}

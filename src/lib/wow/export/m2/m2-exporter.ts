/**
 * M2 export utilities retained for the direct M2 -> MDL converter path.
 */
import type { M2Track } from '../../formats/m2/m2-generics';
import type { SkeletonLike } from '../types/model-export';

export type {
  FileManifestEntry, GeosetMaskEntry, SkeletonLike, TextureManifestEntry, VariantTexture,
} from '../types/model-export';

/** Strip animation keyframes belonging to excluded animation IDs from bones. */
export function bonesExcludeAnimations(skel: SkeletonLike, excludedAnimIds: Set<number>): SkeletonLike['bones'] {
  if (!excludedAnimIds || excludedAnimIds.size === 0) return skel.bones;
  if (!skel.bones) return skel.bones;

  const animations = skel.animations;
  const animIdxMap = new Map(animations.map((anim, idx) => [anim.id, idx]));

  const excludedIndices = new Set(
    [...excludedAnimIds]
      .map((id) => animIdxMap.get(id))
      .filter((idx): idx is number => idx !== undefined),
  );

  if (excludedIndices.size === 0) return skel.bones;

  const noGlobalSeq = 65535;
  const modifyArr = <T>(arr: T[][], globalSeq: number): T[][] => arr.map((a, idx) => (globalSeq === noGlobalSeq && excludedIndices.has(idx) ? [] : a));

  const modifyTrack = <V>(track: M2Track<V>): M2Track<V> => ({
    ...track,
    timestamps: modifyArr(track.timestamps, track.globalSeq),
    values: modifyArr(track.values, track.globalSeq),
  });

  return skel.bones.map((bone) => ({
    ...bone,
    translation: modifyTrack(bone.translation),
    rotation: modifyTrack(bone.rotation),
    scale: modifyTrack(bone.scale),
  }));
}

/**
 * Direct port of M2Exporter's writeBones (src/lib/wow/export/m2-exporter.ts):
 * builds the `_bones.json`-equivalent object in memory instead of writing it.
 * Runs in the converter process; SKEL/ANIM files resolve through the
 * remote-CASC adapter (raw cache + REST fallback).
 *
 * The pre-exclusion skeleton graph (bones + animations + attachments) is
 * cached per skeleton/model file: parent skeletons are shared across every
 * character of a race, so repeated character exports skip the SKEL/.anim
 * loading and track parsing entirely. Exclusions are applied per request via
 * bonesExcludeAnimations, which never mutates the cached arrays.
 */
import { normalizeJsonValues } from '@/lib/converter/wow-model/direct/m2/json-normalize';
import { bonesExcludeAnimations, SkeletonLike } from '@/lib/wow/export/m2/m2-exporter';
import { M2Loader } from '@/lib/wow/formats/m2/m2-loader';
import type { M2Animation, M2Attachment } from '@/lib/wow/formats/m2/m2-types';
import { SKELLoader } from '@/lib/wow/formats/m2/skel-loader';
import { getCasc } from '@/lib/wow/server/runtime';
import { wowDataClient } from '@/lib/wow-data-client/wow-data-client';

export interface BonesData {
  bones: unknown;
  animations?: unknown;
  boneWeights: number[];
  boneIndicies: number[];
  attachments: M2Attachment[];
}

interface SkeletonGraph {
  bones: SkeletonLike['bones'];
  animations?: M2Animation[];
  /** Attachments from the SKEL chain (fallback when the M2 has none). */
  skelAttachments?: M2Attachment[];
}

const GRAPH_CACHE_MAX = 8;
const graphCache = new Map<string, SkeletonGraph>();

/** Drop cached skeleton graphs (e.g. when the active CASC build changes). */
export function clearSkeletonGraphCache(): void {
  graphCache.clear();
}

async function currentBuildKey(): Promise<string> {
  await wowDataClient.waitUntilReady();
  const buildKey = wowDataClient.cascInfo?.buildKey;
  if (!buildKey) throw new Error('No CASC build key available from data server');
  return buildKey;
}

function cacheGet(key: string): SkeletonGraph | undefined {
  const hit = graphCache.get(key);
  if (hit) {
    // Refresh LRU recency.
    graphCache.delete(key);
    graphCache.set(key, hit);
  }
  return hit;
}

function cacheSet(key: string, value: SkeletonGraph): void {
  graphCache.set(key, value);
  if (graphCache.size > GRAPH_CACHE_MAX) {
    graphCache.delete(graphCache.keys().next().value!);
  }
}

/** Load (or reuse) the full pre-exclusion skeleton graph for an M2. */
async function loadSkeletonGraph(m2: M2Loader, m2FileDataID: number | undefined): Promise<SkeletonGraph> {
  const buildKey = await currentBuildKey();
  const cacheKey = m2.skeletonFileID
    ? `${buildKey}:skel:${m2.skeletonFileID}`
    : (m2FileDataID !== undefined ? `${buildKey}:m2:${m2FileDataID}` : undefined);
  if (cacheKey) {
    const hit = cacheGet(cacheKey);
    if (hit) return hit;
  }

  let graph: SkeletonGraph;

  if (m2.skeletonFileID) {
    const skelFile = await getCasc().getFile(m2.skeletonFileID);
    const skel = new SKELLoader(skelFile);
    skel.load();
    await skel.loadAnims();

    if (skel.parent_skel_file_id && skel.parent_skel_file_id > 0) {
      const parentSkelFile = await getCasc().getFile(skel.parent_skel_file_id);
      const parentSkel = new SKELLoader(parentSkelFile);
      parentSkel.load();
      await parentSkel.loadAnims();

      // Map of animation indices from child to parent.
      const animIndexMap = new Map<number, number>();
      for (let i = 0; i < skel.animations.length; i++) {
        const anim = skel.animations[i];
        for (let j = 0; j < parentSkel.animations.length; j++) {
          const parentAnim = parentSkel.animations[j];
          if (parentAnim.id === anim.id && parentAnim.variationIndex === anim.variationIndex) {
            animIndexMap.set(i, j);
            break;
          }
        }
      }

      // Override parent bone animation data with child skeleton animation data if animation is present on both.
      for (let i = 0; i < skel.bones!.length; i++) {
        if (i >= parentSkel.bones!.length) break;

        const bone = skel.bones![i];
        const parentBone = parentSkel.bones![i];

        for (const anim of animIndexMap) {
          if (bone.translation.timestamps.length > anim[0] && parentBone.translation.timestamps.length > anim[1]
            && bone.translation.values.length > anim[0] && parentBone.translation.values.length > anim[1]) {
            parentSkel.bones![i].translation.timestamps[anim[1]] = bone.translation.timestamps[anim[0]];
            parentSkel.bones![i].translation.values[anim[1]] = bone.translation.values[anim[0]];
          }

          if (bone.rotation.timestamps.length > anim[0] && parentBone.rotation.timestamps.length > anim[1]
            && bone.rotation.values.length > anim[0] && parentBone.rotation.values.length > anim[1]) {
            parentSkel.bones![i].rotation.timestamps[anim[1]] = bone.rotation.timestamps[anim[0]];
            parentSkel.bones![i].rotation.values[anim[1]] = bone.rotation.values[anim[0]];
          }

          if (bone.scale.timestamps.length > anim[0] && parentBone.scale.timestamps.length > anim[1]
            && bone.scale.values.length > anim[0] && parentBone.scale.values.length > anim[1]) {
            parentSkel.bones![i].scale.timestamps[anim[1]] = bone.scale.timestamps[anim[0]];
            parentSkel.bones![i].scale.values[anim[1]] = bone.scale.values[anim[0]];
          }
        }
      }

      graph = {
        bones: parentSkel.bones!,
        animations: parentSkel.animations,
        // Legacy parity: attachment fallback always reads the child SKEL.
        skelAttachments: skel.attachments,
      };
    } else {
      graph = {
        bones: skel.bones!,
        animations: skel.animations,
        skelAttachments: skel.attachments,
      };
    }
  } else {
    await m2.loadAnims();
    graph = {
      bones: (m2 as unknown as SkeletonLike).bones,
      animations: m2.animations,
    };
  }

  // Normalize once (replaces the legacy JSON file round-trip: -0 -> 0,
  // NaN/Inf -> null). Idempotent, so doing it at cache-fill time is safe.
  normalizeJsonValues(graph.bones);
  if (graph.animations) normalizeJsonValues(graph.animations);
  if (graph.skelAttachments) normalizeJsonValues(graph.skelAttachments);

  if (cacheKey) cacheSet(cacheKey, graph);
  return graph;
}

export async function buildBonesData(m2: M2Loader, excludedAnimIds: Set<number>, m2FileDataID?: number): Promise<BonesData> {
  const graph = await loadSkeletonGraph(m2, m2FileDataID);

  const data: Partial<BonesData> = {};
  // bonesExcludeAnimations is non-mutating: it returns new bone wrappers
  // sharing the cached track arrays (or the cached bones directly when
  // nothing is excluded).
  data.bones = bonesExcludeAnimations(
    { bones: graph.bones, animations: graph.animations ?? [] } as SkeletonLike,
    excludedAnimIds,
  );
  if (graph.animations) data.animations = graph.animations;

  data.boneWeights = m2.boneWeights;
  data.boneIndicies = m2.boneIndices;

  // Same fallthrough quirk as the legacy exporter: prefer M2 attachments when
  // non-empty, otherwise the SKEL's. M2 attachments come from the fresh
  // per-request loader, so normalize them here (cached ones already are).
  const m2Attachments = m2.attachments?.length ? normalizeJsonValues(m2.attachments) : undefined;
  data.attachments = m2Attachments ?? graph.skelAttachments ?? [];

  return data as BonesData;
}

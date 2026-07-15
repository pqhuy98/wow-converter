import { applyReplaceableTextures, ExportContext, exportModelFileIdAsMdl } from '@/lib/converter/character/utils';
import { MDL } from '@/lib/formats/mdl/mdl';
import { ensureConverterCasc } from '@/lib/wow/archive/client/remote-casc';
import {
  getCreatureDisplaysByFileDataID,
  getFileDataIDByDisplayID,
  initializeCreatureData,
} from '@/lib/wow/db/caches/db-creatures';
import { wowDataClient } from '@/lib/wow-data-client/wow-data-client';
import { CharacterData } from '@/lib/wowhead-client/objects';

/** Wowhead uses Model=0 as a sentinel for chr-model NPCs; only positive IDs are creature M2s. */
export function hasResolvedModel(model?: number): model is number {
  return model != null && model > 0;
}

export async function resolveNpcMetaFromDB(displayId: number): Promise<CharacterData | null> {
  try {
    const remote = await wowDataClient.resolveNpcDisplayMeta(displayId);
    if (remote.found && remote.model != null) {
      return {
        Model: remote.model,
        Textures: remote.textures ?? {},
        Creature: remote.geosets?.length
          ? {
            CreatureCustomizations: [],
            CreatureGeosetData: remote.geosets.map(({ geosetIndex, geosetValue }) => ({
              GeosetIndex: geosetIndex,
              GeosetValue: geosetValue,
            })),
          }
          : undefined,
      };
    }
  } catch {
    // Match Go: fall back to converter-side DB2 caches when the data client fails.
  }

  ensureConverterCasc();
  await initializeCreatureData();
  const fileDataId = getFileDataIDByDisplayID(displayId);
  if (fileDataId == null) return null;

  const meta: CharacterData = {
    Model: fileDataId,
    Textures: {},
  };
  const displays = getCreatureDisplaysByFileDataID(fileDataId);
  if (!displays) return meta;

  const display = displays.find((d) => d.ID === displayId);
  if (!display) return meta;

  display.textures.forEach((texId, i) => {
    meta.Textures![String(i + 1)] = texId;
  });
  if (display.extraGeosets?.length) {
    meta.Creature = {
      CreatureCustomizations: [],
      CreatureGeosetData: display.extraGeosets.map((geoset) => ({
        GeosetIndex: Math.floor(geoset / 100) - 1,
        GeosetValue: geoset % 100,
      })),
    };
  }
  return meta;
}

export function mergeNpcMeta(
  primary: CharacterData,
  fallback: CharacterData,
  preferFallback = false,
): CharacterData {
  if (preferFallback) {
    const out: CharacterData = { ...primary };
    if (hasResolvedModel(fallback.Model)) out.Model = fallback.Model;
    if (fallback.Textures && Object.keys(fallback.Textures).length > 0) out.Textures = fallback.Textures;
    if (fallback.Creature) {
      if (!out.Creature) out.Creature = fallback.Creature;
      else if (!out.Creature.CreatureGeosetData?.length && fallback.Creature.CreatureGeosetData?.length) {
        out.Creature.CreatureGeosetData = fallback.Creature.CreatureGeosetData;
      }
    }
    return out;
  }
  const out: CharacterData = { ...primary };
  if (out.Model == null && hasResolvedModel(fallback.Model)) out.Model = fallback.Model;
  if ((!out.Textures || Object.keys(out.Textures).length === 0) && fallback.Textures && Object.keys(fallback.Textures).length > 0) {
    out.Textures = fallback.Textures;
  }
  if (!out.Creature && fallback.Creature) out.Creature = fallback.Creature;
  return out;
}

export async function exportCreatureNpcAsMdl(ctx: ExportContext, meta: CharacterData): Promise<MDL> {
  if (!hasResolvedModel(meta.Model)) throw new Error('Creature NPC must contain Model');
  const modelId = meta.Model;
  const extraGeosets = meta.Creature?.CreatureGeosetData?.map((g) => (g.GeosetIndex + 1) * 100 + g.GeosetValue) || [];
  const model = await exportModelFileIdAsMdl(ctx, modelId, {
    textureIds: Object.values(meta.Textures || {}),
    extraGeosets,
  });

  await applyReplaceableTextures(ctx, model.mdl, meta.Textures || {});

  return model.mdl;
}

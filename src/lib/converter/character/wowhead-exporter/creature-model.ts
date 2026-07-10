import { applyReplaceableTextures, ExportContext, exportModelFileIdAsMdl } from '@/lib/converter/character/utils';
import { MDL } from '@/lib/formats/mdl/mdl';
import { CharacterData } from '@/lib/wowhead-client/objects';
import {
  getCreatureDisplaysByFileDataID,
  getFileDataIDByDisplayID,
  initializeCreatureData,
} from '@/lib/wow/db/caches/db-creatures';

export async function resolveNpcMetaFromDB(displayId: number): Promise<CharacterData | null> {
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

export function mergeNpcMeta(primary: CharacterData, fallback: CharacterData): CharacterData {
  const out: CharacterData = { ...primary };
  if (!out.Model) out.Model = fallback.Model;
  if (!out.Textures || Object.keys(out.Textures).length === 0) out.Textures = fallback.Textures;
  if (!out.Creature && fallback.Creature) out.Creature = fallback.Creature;
  return out;
}

export async function exportCreatureNpcAsMdl(ctx: ExportContext, meta: CharacterData): Promise<MDL> {
  if (!meta.Model) throw new Error('Creature NPC must contain Model');
  const modelId = meta.Model;
  const extraGeosets = meta.Creature?.CreatureGeosetData?.map((g) => (g.GeosetIndex + 1) * 100 + g.GeosetValue) || [];
  const model = await exportModelFileIdAsMdl(ctx, modelId, {
    textureIds: Object.values(meta.Textures || {}),
    extraGeosets,
  });

  await applyReplaceableTextures(ctx, model.mdl, meta.Textures || {});

  return model.mdl;
}

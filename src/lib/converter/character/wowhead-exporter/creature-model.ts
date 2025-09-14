import { MDL } from '@/lib/formats/mdl/mdl';
import { CharacterData } from '@/lib/wowhead-client/objects';

import { applyReplaceableTextures, ExportContext, exportModelFileIdAsMdl } from '../utils';

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

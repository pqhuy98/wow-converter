import { MDL } from '@/lib/formats/mdl/mdl';
import { fetchNpcMeta } from '@/lib/wowhead-client/npc';
import { NpcZamUrl } from '@/lib/wowhead-client/zam-url';

import { applyReplaceableTextrures, ExportContext, exportModelFileIdAsMdl } from '../utils';

export async function exportCreatureNpcAsMdl(ctx: ExportContext, zam: NpcZamUrl): Promise<MDL> {
  const meta = await fetchNpcMeta(zam);
  if (!meta.Model) throw new Error('Creature NPC must contain Model');
  const modelId = meta.Model;
  const extraGeosets = meta.Creature?.CreatureGeosetData?.map((g) => (g.GeosetIndex + 1) * 100 + g.GeosetValue) || [];
  const mdl = await exportModelFileIdAsMdl(ctx, modelId, {
    textureIds: Object.values(meta.Textures || {}),
    extraGeosets,
  });

  await applyReplaceableTextrures(ctx, mdl, meta.Textures || {});

  return mdl;
}

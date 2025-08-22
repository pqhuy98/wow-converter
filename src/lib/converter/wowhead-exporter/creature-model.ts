
import { fetchNpcMeta } from '@/lib/wowhead-client/npc';
import { NpcZamUrl, ZamUrl } from '@/lib/wowhead-client/zam-url';
import { MDL } from '@/lib/objmdl/mdl/mdl';
import { ExportContext, exportModelFileIdAsMdl } from './utils';


export async function exportCreatureNpcAsMdl(ctx: ExportContext, zam: NpcZamUrl): Promise<MDL> {
  const meta = await fetchNpcMeta(zam);
  if (!meta.Model) throw new Error('Creature NPC must contain Model');
  const modelId = meta.Model;
  const textureIds = Object.values(meta.Textures || {});
  return  exportModelFileIdAsMdl(ctx, modelId, textureIds);
}

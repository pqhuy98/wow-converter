import {
  getCreatureDisplaysByFileDataID,
  getFileDataIDByDisplayID,
} from '@/lib/wow/db/caches/db-creatures';
import { ensureModelCachesInitialized } from '@/lib/wow/db/caches/init-cache';

export interface NpcDisplayGeoset {
  geosetIndex: number;
  geosetValue: number;
}

export interface NpcDisplayMeta {
  found: boolean;
  model?: number;
  textures?: Record<string, number>;
  geosets?: NpcDisplayGeoset[];
}

/** Resolve creature model, textures, and geosets from DB2 tables (mirrors Go service). */
export async function resolveNpcDisplayMeta(displayID: number): Promise<NpcDisplayMeta> {
  await ensureModelCachesInitialized();
  const fileDataID = getFileDataIDByDisplayID(displayID);
  if (fileDataID == null) {
    return { found: false };
  }

  const meta: NpcDisplayMeta = {
    found: true,
    model: fileDataID,
    textures: {},
    geosets: [],
  };

  const displays = getCreatureDisplaysByFileDataID(fileDataID);
  const display = displays?.find((d) => d.ID === displayID);
  if (!display) {
    return meta;
  }

  display.textures.forEach((texID, i) => {
    meta.textures![String(i + 1)] = texID;
  });
  if (display.extraGeosets?.length) {
    meta.geosets = display.extraGeosets.map((geoset) => ({
      geosetIndex: Math.floor(geoset / 100) - 1,
      geosetValue: geoset % 100,
    }));
  }
  return meta;
}

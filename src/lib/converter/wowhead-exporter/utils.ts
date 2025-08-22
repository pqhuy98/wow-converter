import { AssetManager } from "../common/model-manager";
import { wowExportClient } from "@/lib/wowexport-client/wowexport-client";
import { MDL } from "@/lib/objmdl/mdl/mdl";
import path from "path";
import { Config } from "@/lib/global-config";

export interface ExportContext {
  assetManager: AssetManager;
  config: Config;
}

export async function exportModelFileIdAsMdl(ctx: ExportContext, modelFileId: number, textureIds?: number[]): Promise<MDL> {
  let skinName: string | undefined;
  const skins = await wowExportClient.getModelSkins(modelFileId);
  const match = skins.find((s: { textureIDs: number[] }) => textureIds?.every((id) => s.textureIDs.includes(id)));
  skinName = (match || skins[0])?.id;

  const exported = (await wowExportClient.exportModels([{ fileDataID: modelFileId, skinName }]))[0];
  const obj = exported.files.find((f) => f.type === 'OBJ')?.file;
  if (!obj) throw new Error('Failed to export model OBJ');
  const baseDir = await wowExportClient.getAssetDir();
  const relative = path.relative(baseDir, obj);
  return ctx.assetManager.parse(relative, true).mdl;
}

export async function exportTexture(textureId: number): Promise<string> {
  const tex = await wowExportClient.exportTextures([textureId]);
  return await relativeToExport(tex[0].file);
}

async function relativeToExport(p: string): Promise<string> {
  const baseDir = await wowExportClient.getAssetDir();
  return path.relative(baseDir, p);
}
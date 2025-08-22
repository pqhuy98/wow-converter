import { CharacterModelZamUrl, getZamBaseUrl, ZamUrl } from "./zam-url";

// https://wow.zamimg.com/modelviewer/wrath/meta/character/3.json
export interface CharacterModelMeta {
  Model: number;
}

export async function fetchCharacterModel(zam: CharacterModelZamUrl): Promise<CharacterModelMeta> {
  if (zam.type !== "character") throw new Error('fetchItemMeta expects a ZamUrl of type item');
  const base = getZamBaseUrl(zam.expansion);
  const url = `${base}/meta/character/${zam.modelId}.json`;
  const res = await fetch(url);
  if (!res.ok) throw new Error(`Failed to fetch character model meta: ${res.status} ${res.statusText}`);
  return await res.json() as unknown as CharacterModelMeta;
}

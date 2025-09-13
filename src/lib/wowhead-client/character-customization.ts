import chalk from 'chalk';

import {
  CharacterCustomizationZamUrl, fetchWithCache, getLatestExpansionHavingUrl, getZamBaseUrl,
} from './zam-url';

export interface CharacterCustomization {
  Options: {
    Id: number
    Name: string
    OrderIndex: number
    Choices: {
      Id: number
      Name: string
      OrderIndex: number
      CustReqId: number
      Elements: {
        Id: number
        VariationIndex: number
        VariationChoiceID: number
        Geoset?: {
          GeosetType: number
          GeosetID: number
          Modifier: number
        }
        SkinnedModel?: {
          CollectionFileDataID: number
          GeosetType: number
          GeosetID: number
          Modifier: number
          Flags: number
        }
        Material?: {
          TextureTarget: number
          MaterialResourcesID: number
        }
        BoneSet?: {
          BoneFileDataID: number
          ModelFileDataID: number
        }
        CondModelFileDataId: number
        ChrCustItemGeoModifyID: number
      }[]
    }[]
  }[]
  TextureFiles: {
    [k: string]: {
      FileDataId: number
      Gender: number
      Class: number
      Race: number
      ExtraData: number
    }[]
  }
  Materials: {
    TextureType: number
    Width: number
    Height: number
    Flags: number
    // Unk_34615_1: number
  }[]
  TextureLayers: {
    TextureType: number;
    Layer: number;
    BlendMode: number;
    ChrModelTextureTargetID: number;
    TextureSection: number;
  }[];
  TextureSections: {
    SectionType: number;
    X: number;
    Y: number;
    Width: number;
    Height: number;
  }[];
}

const debug = true;

export async function fetchCharacterCustomization(zam: CharacterCustomizationZamUrl): Promise<CharacterCustomization> {
  if (zam.type !== 'character-customization') throw new Error('fetchCharacterCustomization expects a ZamUrl of type character-customization');

  const path = `meta/charactercustomization/${zam.chrModelId}.json`;

  let expansion = zam.expansion;
  if (expansion === 'latest-available') {
    expansion = await getLatestExpansionHavingUrl(path);
  }
  const base = getZamBaseUrl(expansion);
  const url = `${base}/${path}`;
  debug && console.log('Get character customization from', chalk.blue(url));
  try {
    const res = await fetchWithCache(url);
    return JSON.parse(res) as unknown as CharacterCustomization;
  } catch (e) {
    console.log(
      chalk.red('Failed to fetch character customization from'),
      chalk.blue(url),
      chalk.red(e),
      chalk.red('falling back to latest available expansion'),
    );
    const base2 = getZamBaseUrl(await getLatestExpansionHavingUrl(path));
    const url2 = `${base2}/${path}`;
    const res2 = await fetchWithCache(url2);
    return JSON.parse(res2) as unknown as CharacterCustomization;
  }
}

import { existsSync, readFileSync, writeFileSync } from 'fs';
import path from 'path';

import {
  Camera, Doodad, Info, ObjectModificationTable, ObjectType, Region, SpecialDoodad, Terrain, Unit,
} from '../data';
import { CamerasTranslator } from './CamerasTranslator';
import { DoodadsTranslator } from './DoodadsTranslator';
import { InfoTranslator } from './InfoTranslator';
import { ObjectsTranslator } from './ObjectsTranslator';
import { RegionsTranslator } from './RegionsTranslator';
import { TerrainTranslator } from './TerrainTranslator';
import { UnitsTranslator } from './UnitsTranslator';

type FilePath = 'info'
  | 'terrain'
  | 'units'
  | 'doodads'
  | 'cameras'
  | 'regions'
  | 'unitData'
  | 'itemData'
  | 'destructibleData'
  | 'doodadData'
  | 'abilityData'
  | 'buffData'
  | 'upgradeData';

export class MapTranslator {
  public info: Info;

  public terrain: Terrain;

  public units: Unit[] = [];

  public doodads: Doodad[] = [];

  public specialDoodads: SpecialDoodad[] = [];

  public cameras: Camera[] = [];

  public regions: Region[] = [];

  public unitData: ObjectModificationTable = { original: {}, custom: {} };

  public destructibleData: ObjectModificationTable = { original: {}, custom: {} };

  public doodadData: ObjectModificationTable = { original: {}, custom: {} };

  public itemData: ObjectModificationTable = { original: {}, custom: {} };

  public abilityData: ObjectModificationTable = { original: {}, custom: {} };

  public buffData: ObjectModificationTable = { original: {}, custom: {} };

  public upgradeData: ObjectModificationTable = { original: {}, custom: {} };

  public filePaths: Record<FilePath, string>;

  constructor() {
  }

  public setMapDir(mapDir: string) {
    this.filePaths = {
      info: path.join(mapDir, 'war3map.w3i'),
      terrain: path.join(mapDir, 'war3map.w3e'),
      units: path.join(mapDir, 'war3mapUnits.doo'),
      doodads: path.join(mapDir, 'war3map.doo'),
      cameras: path.join(mapDir, 'war3map.w3c'),
      regions: path.join(mapDir, 'war3map.w3r'),
      unitData: path.join(mapDir, 'war3map.w3u'),
      itemData: path.join(mapDir, 'war3map.w3t'),
      destructibleData: path.join(mapDir, 'war3map.w3b'),
      doodadData: path.join(mapDir, 'war3map.w3d'),
      abilityData: path.join(mapDir, 'war3map.w3a'),
      buffData: path.join(mapDir, 'war3map.w3h'),
      upgradeData: path.join(mapDir, 'war3map.w3q'),
    };
  }

  load(mapDir: string) {
    this.setMapDir(mapDir);
    this.info = InfoTranslator.warToJson(readFileSync(this.filePaths.info)).json;
    this.terrain = TerrainTranslator.warToJson(readFileSync(this.filePaths.terrain)).json;

    console.log('Loading units from', this.filePaths.unitData);
    this.units = UnitsTranslator.warToJson(readFileSync(this.filePaths.units)).json;
    console.log('Units loaded', this.units.length);

    const allDoodads = DoodadsTranslator.warToJson(readFileSync(this.filePaths.doodads)).json;
    this.doodads = allDoodads[0];
    this.specialDoodads = allDoodads[1];

    if (existsSync(this.filePaths.cameras)) {
      this.cameras = CamerasTranslator.warToJson(readFileSync(this.filePaths.cameras)).json;
    }

    if (existsSync(this.filePaths.regions)) {
      this.regions = RegionsTranslator.warToJson(readFileSync(this.filePaths.regions)).json;
    }

    if (existsSync(this.filePaths.unitData)) {
      this.unitData = ObjectsTranslator.warToJson(ObjectType.Units, readFileSync(this.filePaths.unitData)).json;
    }
    if (existsSync(this.filePaths.itemData)) {
      this.itemData = ObjectsTranslator.warToJson(ObjectType.Items, readFileSync(this.filePaths.itemData)).json;
    }
    if (existsSync(this.filePaths.destructibleData)) {
      this.destructibleData = ObjectsTranslator.warToJson(ObjectType.Destructables, readFileSync(this.filePaths.destructibleData)).json;
    }
    if (existsSync(this.filePaths.doodadData)) {
      this.doodadData = ObjectsTranslator.warToJson(ObjectType.Doodads, readFileSync(this.filePaths.doodadData)).json;
    }
    if (existsSync(this.filePaths.abilityData)) {
      this.abilityData = ObjectsTranslator.warToJson(ObjectType.Abilities, readFileSync(this.filePaths.abilityData)).json;
    }
    if (existsSync(this.filePaths.buffData)) {
      this.buffData = ObjectsTranslator.warToJson(ObjectType.Buffs, readFileSync(this.filePaths.buffData)).json;
    }
    if (existsSync(this.filePaths.upgradeData)) {
      this.upgradeData = ObjectsTranslator.warToJson(ObjectType.Upgrades, readFileSync(this.filePaths.upgradeData)).json;
    }
  }

  public save(filePath: FilePath) {
    switch (filePath) {
      case 'info':
        writeFileSync(this.filePaths.info, InfoTranslator.jsonToWar(this.info).buffer);
        break;
      case 'terrain':
        writeFileSync(this.filePaths.terrain, TerrainTranslator.jsonToWar(this.terrain).buffer);
        break;
      case 'units':
        writeFileSync(this.filePaths.units, UnitsTranslator.jsonToWar(this.units).buffer);
        break;
      case 'doodads':
        writeFileSync(this.filePaths.doodads, DoodadsTranslator.jsonToWar([this.doodads, this.specialDoodads]).buffer);
        break;
      case 'cameras':
        writeFileSync(this.filePaths.cameras, CamerasTranslator.jsonToWar(this.cameras).buffer);
        break;
      case 'regions':
        writeFileSync(this.filePaths.regions, RegionsTranslator.jsonToWar(this.regions).buffer);
        break;
      case 'unitData':
        writeFileSync(this.filePaths.unitData, ObjectsTranslator.jsonToWar(ObjectType.Units, this.unitData).buffer);
        break;
      case 'itemData':
        writeFileSync(this.filePaths.itemData, ObjectsTranslator.jsonToWar(ObjectType.Items, this.itemData).buffer);
        break;
      case 'destructibleData':
        writeFileSync(this.filePaths.destructibleData, ObjectsTranslator.jsonToWar(ObjectType.Destructables, this.destructibleData).buffer);
        break;
      case 'doodadData':
        writeFileSync(this.filePaths.doodadData, ObjectsTranslator.jsonToWar(ObjectType.Doodads, this.doodadData).buffer);
        break;
      case 'abilityData':
        writeFileSync(this.filePaths.abilityData, ObjectsTranslator.jsonToWar(ObjectType.Abilities, this.abilityData).buffer);
        break;
      case 'buffData':
        writeFileSync(this.filePaths.buffData, ObjectsTranslator.jsonToWar(ObjectType.Buffs, this.buffData).buffer);
        break;
      case 'upgradeData':
        writeFileSync(this.filePaths.upgradeData, ObjectsTranslator.jsonToWar(ObjectType.Upgrades, this.upgradeData).buffer);
        break;
      default:
        // eslint-disable-next-line @typescript-eslint/restrict-template-expressions
        throw new Error(`Unknown file path ${filePath}`);
    }
  }
}

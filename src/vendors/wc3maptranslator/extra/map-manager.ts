import {
  Doodad, Modification, ObjectModificationTable, Terrain, Unit,
} from '../data';
import { MapTranslator } from '../translators';
import { FourCCGenerator } from './war3-fourcc';

export const baseDoodadType = 'YOlb'; // Lightning Bolt
export const baseDestructibleType = 'OTds'; // Demon Storm

export interface IUnitType {
  code: string
  parent: string
  data: Modification[]
}

export interface IUnit extends Omit<Unit, 'type'> {
  type: IUnitType
}

export interface IDoodadType {
  code: string
  parent: string
  data: Modification[]
  isDestructible: boolean
}

export interface IDoodad extends Omit<Doodad, 'type'> {
  type: IDoodadType
}

export class MapManager {
  private mapData: MapTranslator;

  private fourCCGenerator: FourCCGenerator;

  unitTypes: IUnitType[] = [];

  doodadTypes: IDoodadType[] = [];

  destructibleTypes: IDoodadType[] = [];

  units: IUnit[] = [];

  doodads: IDoodad[] = [];

  constructor(mapDir: string) {
    this.mapData = new MapTranslator(mapDir);

    // Initialise FourCC generator and mark all already-used IDs as taken.
    this.fourCCGenerator = new FourCCGenerator();

    const registerTableFourCCs = (table: ObjectModificationTable) => {
      [...Object.keys(table.original), ...Object.keys(table.custom)].forEach((key) => {
        if (key.length >= 4) this.fourCCGenerator.addUsed(key.slice(0, 4));
      });
    };

    registerTableFourCCs(this.mapData.unitData);
    registerTableFourCCs(this.mapData.itemData);
    registerTableFourCCs(this.mapData.destructibleData);
    registerTableFourCCs(this.mapData.doodadData);
    registerTableFourCCs(this.mapData.abilityData);
    registerTableFourCCs(this.mapData.buffData);
    registerTableFourCCs(this.mapData.upgradeData);
  }

  get terrain() {
    return this.mapData.terrain;
  }

  set terrain(terrain: Terrain) {
    this.mapData.terrain = terrain;
  }

  addUnitType(isHero: 'hero' | 'unit', parentCode: string, data: Modification[]) {
    this.unitTypes.push({
      code: this.fourCCGenerator.generate(isHero === 'hero' ? 'upper' : 'lower').codeString,
      parent: parentCode,
      data,
    });
    return this.unitTypes[this.unitTypes.length - 1];
  }

  addDoodadType(data: Modification[], isDestructible: boolean) {
    const doodadType: IDoodadType = {
      code: this.fourCCGenerator.generate().codeString,
      parent: isDestructible ? baseDestructibleType : baseDoodadType,
      data,
      isDestructible,
    };
    if (isDestructible) {
      this.destructibleTypes.push(doodadType);
    } else {
      this.doodadTypes.push(doodadType);
    }
    return doodadType;
  }

  addUnit(type: IUnitType, unit: Omit<Unit, 'type'>) {
    this.units.push({
      ...unit,
      type,
    });
    return this.units[this.units.length - 1];
  }

  addDoodad(type: IDoodadType, doodad: Omit<Doodad, 'type'>) {
    this.doodads.push({
      ...doodad,
      type,
    });
    return this.doodads[this.doodads.length - 1];
  }

  save() {
    this.unitTypes.forEach((unitType) => {
      this.mapData.unitData.custom[`${unitType.code}:${unitType.parent}`] = unitType.data;
    });
    this.doodadTypes.forEach((doodadType) => {
      this.mapData.doodadData.custom[`${doodadType.code}:${doodadType.parent}`] = doodadType.data;
    });
    this.destructibleTypes.forEach((destructibleType) => {
      this.mapData.destructibleData.custom[`${destructibleType.code}:${destructibleType.parent}`] = destructibleType.data;
    });
    this.units.forEach((unit) => {
      this.mapData.units.push({
        ...unit,
        type: unit.type.code,
      });
    });
    this.doodads.forEach((doodad) => {
      this.mapData.doodads.push({
        ...doodad,
        type: doodad.type.code,
      });
    });
    this.mapData.save('units');
    this.mapData.save('doodads');
    this.mapData.save('terrain');
    this.mapData.save('unitData');
    this.mapData.save('doodadData');
    this.mapData.save('destructibleData');
  }
}

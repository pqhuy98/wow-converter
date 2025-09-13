import {
  Doodad, Modification, ObjectModificationTable, Terrain, Unit,
} from '../data';
import { MapTranslator } from '../translators';
import { FourCCGenerator } from './war3-fourcc';

export const baseDoodadType = 'YOlb'; // Lightning Bolt
export const baseDestructibleType = 'OTds'; // Demon Storm

export interface IObjectData {
  code: string
  parent: string
  data: Modification[]
}

export interface IUnitType extends IObjectData {
}

export interface IUnit extends Omit<Unit, 'type'> {
  type: IUnitType | string
}

export interface IDoodadType extends IObjectData {
  isDestructible: boolean
}

export interface IDoodad extends Omit<Doodad, 'type'> {
  type: IDoodadType | string
}

export interface IAbilityType extends IObjectData {
}

export class MapManager {
  private mapData: MapTranslator;

  private fourCCGenerator: FourCCGenerator;

  unitTypes: IUnitType[] = [];

  doodadTypes: IDoodadType[] = [];

  destructibleTypes: IDoodadType[] = [];

  units: IUnit[] = [];

  doodads: IDoodad[] = [];

  abilities: IAbilityType[] = [];

  constructor() {
    this.mapData = new MapTranslator();

    // Initialise FourCC generator and mark all already-used IDs as taken.
    this.fourCCGenerator = new FourCCGenerator();
  }

  private registerTableFourCCs(table: ObjectModificationTable) {
    [...Object.keys(table.original), ...Object.keys(table.custom)].forEach((key) => {
      if (key.length >= 4) this.fourCCGenerator.addUsed(key.slice(0, 4));
    });
  }

  load(mapDir: string) {
    this.mapData.load(mapDir);

    this.registerTableFourCCs(this.mapData.unitData);
    this.registerTableFourCCs(this.mapData.itemData);
    this.registerTableFourCCs(this.mapData.destructibleData);
    this.registerTableFourCCs(this.mapData.doodadData);
    this.registerTableFourCCs(this.mapData.abilityData);
    this.registerTableFourCCs(this.mapData.buffData);
    this.registerTableFourCCs(this.mapData.upgradeData);

    Object.entries(this.mapData.unitData.custom).forEach(([key, value]) => {
      const [code, parent] = key.split(':');
      this.unitTypes.push({
        code,
        parent,
        data: value,
      });
    });
    Object.entries(this.mapData.doodadData.custom).forEach(([key, value]) => {
      const [code, parent] = key.split(':');
      this.doodadTypes.push({
        code,
        parent,
        data: value,
        isDestructible: false,
      });
    });
    Object.entries(this.mapData.destructibleData.custom).forEach(([key, value]) => {
      const [code, parent] = key.split(':');
      this.destructibleTypes.push({
        code,
        parent,
        data: value,
        isDestructible: true,
      });
    });

    this.units = this.mapData.units.map((unit) => {
      const type = this.unitTypes.find((type) => type.code === unit.type) ?? unit.type;
      return { ...unit, type };
    });
    this.doodads = this.mapData.doodads.map((doodad) => {
      const type = this.doodadTypes.find((type) => type.code === doodad.type) ?? doodad.type;
      return { ...doodad, type };
    });
    Object.entries(this.mapData.abilityData.custom).forEach(([key, value]) => {
      const [code, parent] = key.split(':');
      this.abilities.push({
        code,
        parent,
        data: value,
      });
    });
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

  addAbility(parentCode: string, data: Modification[]) {
    this.abilities.push({
      code: this.fourCCGenerator.generate().codeString,
      parent: parentCode,
      data,
    });
    return this.abilities[this.abilities.length - 1];
  }

  save(mapDir: string) {
    this.mapData.unitData.custom = {};
    this.unitTypes.forEach((unitType) => {
      this.mapData.unitData.custom[`${unitType.code}:${unitType.parent}`] = unitType.data;
    });
    this.mapData.doodadData.custom = {};
    this.doodadTypes.forEach((doodadType) => {
      this.mapData.doodadData.custom[`${doodadType.code}:${doodadType.parent}`] = doodadType.data;
    });
    this.mapData.destructibleData.custom = {};
    this.destructibleTypes.forEach((destructibleType) => {
      this.mapData.destructibleData.custom[`${destructibleType.code}:${destructibleType.parent}`] = destructibleType.data;
    });
    this.mapData.units = [];
    this.units.forEach((unit) => {
      this.mapData.units.push({
        ...unit,
        type: typeof unit.type === 'string' ? unit.type : unit.type.code,
      });
    });
    this.mapData.doodads = [];
    this.doodads.forEach((doodad) => {
      this.mapData.doodads.push({
        ...doodad,
        type: typeof doodad.type === 'string' ? doodad.type : doodad.type.code,
      });
    });
    this.mapData.abilityData.custom = {};
    this.abilities.forEach((ability) => {
      this.mapData.abilityData.custom[`${ability.code}:${ability.parent}`] = ability.data;
    });
    this.mapData.setMapDir(mapDir);
    this.mapData.save('units');
    this.mapData.save('doodads');
    this.mapData.save('terrain');
    this.mapData.save('unitData');
    this.mapData.save('doodadData');
    this.mapData.save('destructibleData');
    this.mapData.save('abilityData');
  }
}

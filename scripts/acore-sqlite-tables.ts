export const ACORE_SQLITE_OUTPUT = 'bin/azerothcore-world.sqlite';

/** MySQL UNSIGNED INT columns stored as signed INT32 in SQLite (Prisma Int limit). */
export const ACORE_UNSIGNED_INT32_COLUMNS: Record<string, readonly string[]> = {
  creature: ['phaseMask'],
  gameobject: ['phaseMask'],
};

export function normalizeSqliteColumnValue(table: string, column: string, value: unknown): unknown {
  if (value == null) return value;
  if (!ACORE_UNSIGNED_INT32_COLUMNS[table]?.includes(column)) return value;
  const n = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(n) || n <= 2147483647) return n;
  return n | 0;
}

export interface AcoreTableSpec {
  /** AzerothCore world table name. */
  name: string;
  /** Columns to copy into SQLite (must exist in the source table). */
  columns: readonly string[];
  createSql: string;
  indexes?: readonly string[];
}

/**
 * Tables exported from acore_world into the bundled SQLite file.
 * Add entries here when new AzerothCore data is needed, then rerun
 * `bun run generate:acore-sqlite`.
 */
export const ACORE_SQLITE_TABLES: readonly AcoreTableSpec[] = [
  {
    name: 'game_event_creature',
    columns: ['eventEntry', 'guid'],
    createSql: `CREATE TABLE game_event_creature (
      eventEntry INTEGER NOT NULL, guid INTEGER NOT NULL,
      PRIMARY KEY (guid, eventEntry)
    )`,
  },
  {
    name: 'game_event_gameobject',
    columns: ['eventEntry', 'guid'],
    createSql: `CREATE TABLE game_event_gameobject (
      eventEntry INTEGER NOT NULL, guid INTEGER NOT NULL,
      PRIMARY KEY (guid, eventEntry)
    )`,
  },
  {
    name: 'gameobject_addon',
    columns: ['guid', 'invisibilityType', 'invisibilityValue'],
    createSql: `CREATE TABLE gameobject_addon (
      guid INTEGER PRIMARY KEY, invisibilityType INTEGER NOT NULL, invisibilityValue INTEGER NOT NULL
    )`,
  },
  {
    name: 'gameobject',
    columns: ['guid', 'id', 'map', 'spawnMask', 'phaseMask', 'position_x', 'position_y', 'position_z', 'orientation', 'rotation0', 'rotation1', 'rotation2', 'rotation3', 'state', 'animprogress'],
    createSql: `CREATE TABLE gameobject (
      guid INTEGER PRIMARY KEY, id INTEGER NOT NULL, map INTEGER NOT NULL,
      spawnMask INTEGER NOT NULL, phaseMask INTEGER NOT NULL,
      position_x REAL NOT NULL, position_y REAL NOT NULL, position_z REAL NOT NULL,
      orientation REAL NOT NULL, rotation0 REAL NOT NULL, rotation1 REAL NOT NULL,
      rotation2 REAL NOT NULL, rotation3 REAL NOT NULL, state INTEGER NOT NULL, animprogress INTEGER NOT NULL
    )`,
    indexes: ['CREATE INDEX idx_gameobject_map_position ON gameobject(map, position_x, position_y)'],
  },
  {
    name: 'gameobject_template',
    columns: ['entry', 'type', 'displayId', 'name', 'size'],
    createSql: `CREATE TABLE gameobject_template (
      entry INTEGER PRIMARY KEY, type INTEGER NOT NULL, displayId INTEGER NOT NULL,
      name TEXT NOT NULL, size REAL NOT NULL
    )`,
  },
  {
    name: 'creature',
    columns: [
      'guid', 'id1', 'map', 'position_x', 'position_y', 'position_z',
      'orientation', 'phaseMask', 'curhealth', 'curmana',
    ],
    createSql: `
      CREATE TABLE creature (
        guid INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
        id1 INTEGER NOT NULL DEFAULT 0,
        map INTEGER NOT NULL DEFAULT 0,
        position_x REAL NOT NULL DEFAULT 0,
        position_y REAL NOT NULL DEFAULT 0,
        position_z REAL NOT NULL DEFAULT 0,
        orientation REAL NOT NULL DEFAULT 0,
        phaseMask INTEGER NOT NULL DEFAULT 1,
        curhealth INTEGER NOT NULL DEFAULT 1,
        curmana INTEGER NOT NULL DEFAULT 0
      )`,
    indexes: [
      'CREATE INDEX idx_creature_id1 ON creature(id1)',
      'CREATE INDEX idx_creature_map ON creature(map)',
    ],
  },
  {
    name: 'creature_template',
    columns: ['entry', 'name', 'subname', 'maxlevel', 'flags_extra', 'minlevel', 'exp', 'unit_class', 'type', 'family', 'rank', 'faction', 'npcflag', 'unit_flags', 'type_flags', 'speed_walk', 'speed_run', 'speed_swim', 'speed_flight', 'detection_range', 'BaseAttackTime', 'RangeAttackTime', 'BaseVariance', 'RangeVariance', 'DamageModifier', 'HealthModifier', 'ManaModifier', 'ArmorModifier', 'HoverHeight', 'RegenHealth', 'dmgschool'],
    createSql: `
      CREATE TABLE creature_template (
        entry INTEGER PRIMARY KEY NOT NULL,
        name TEXT NOT NULL DEFAULT '0',
        subname TEXT,
        maxlevel INTEGER NOT NULL DEFAULT 1,
        flags_extra INTEGER NOT NULL DEFAULT 0,
        minlevel INTEGER NOT NULL DEFAULT 1, exp INTEGER NOT NULL DEFAULT 0,
        unit_class INTEGER NOT NULL DEFAULT 1, type INTEGER NOT NULL DEFAULT 0,
        family INTEGER NOT NULL DEFAULT 0, rank INTEGER NOT NULL DEFAULT 0,
        faction INTEGER NOT NULL DEFAULT 0, npcflag INTEGER NOT NULL DEFAULT 0,
        unit_flags INTEGER NOT NULL DEFAULT 0, type_flags INTEGER NOT NULL DEFAULT 0,
        speed_walk REAL NOT NULL DEFAULT 1, speed_run REAL NOT NULL DEFAULT 1,
        speed_swim REAL NOT NULL DEFAULT 1, speed_flight REAL NOT NULL DEFAULT 1,
        detection_range REAL NOT NULL DEFAULT 20,
        BaseAttackTime INTEGER NOT NULL DEFAULT 2000, RangeAttackTime INTEGER NOT NULL DEFAULT 2000,
        BaseVariance REAL NOT NULL DEFAULT 1, RangeVariance REAL NOT NULL DEFAULT 1,
        DamageModifier REAL NOT NULL DEFAULT 1, HealthModifier REAL NOT NULL DEFAULT 1,
        ManaModifier REAL NOT NULL DEFAULT 1, ArmorModifier REAL NOT NULL DEFAULT 1,
        HoverHeight REAL NOT NULL DEFAULT 1, RegenHealth INTEGER NOT NULL DEFAULT 1,
        dmgschool INTEGER NOT NULL DEFAULT 0
      )`,
    indexes: [
      'CREATE INDEX idx_creature_template_name ON creature_template(name)',
    ],
  },
  {
    name: 'creature_equip_template',
    columns: ['CreatureID', 'ID', 'ItemID1', 'ItemID2', 'ItemID3'],
    createSql: `
      CREATE TABLE creature_equip_template (
        CreatureID INTEGER NOT NULL DEFAULT 0,
        ID INTEGER NOT NULL DEFAULT 1,
        ItemID1 INTEGER NOT NULL DEFAULT 0,
        ItemID2 INTEGER NOT NULL DEFAULT 0,
        ItemID3 INTEGER NOT NULL DEFAULT 0,
        PRIMARY KEY (CreatureID, ID)
      )`,
  },
  {
    name: 'creature_template_model',
    columns: ['CreatureID', 'Idx', 'CreatureDisplayID', 'DisplayScale'],
    createSql: `
      CREATE TABLE creature_template_model (
        CreatureID INTEGER NOT NULL,
        Idx INTEGER NOT NULL DEFAULT 0,
        CreatureDisplayID INTEGER NOT NULL,
        DisplayScale REAL NOT NULL DEFAULT 1,
        PRIMARY KEY (CreatureID, Idx)
      )`,
  },
  {
    name: 'item_template',
    columns: ['entry', 'InventoryType', 'class', 'subclass', 'SoundOverrideSubclass', 'delay'],
    createSql: `
      CREATE TABLE item_template (
        entry INTEGER PRIMARY KEY NOT NULL,
        InventoryType INTEGER NOT NULL DEFAULT 0,
        class INTEGER NOT NULL DEFAULT 0, subclass INTEGER NOT NULL DEFAULT 0,
        SoundOverrideSubclass INTEGER NOT NULL DEFAULT -1, delay INTEGER NOT NULL DEFAULT 0
      )`,
    indexes: [
      'CREATE INDEX idx_item_template_inventory ON item_template(InventoryType)',
    ],
  },
  {
    name: 'creature_classlevelstats',
    columns: ['level', 'class', 'basehp0', 'basehp1', 'basehp2', 'basemana', 'basearmor', 'attackpower', 'rangedattackpower', 'damage_base', 'damage_exp1', 'damage_exp2'],
    createSql: `CREATE TABLE creature_classlevelstats (
      level INTEGER NOT NULL, class INTEGER NOT NULL,
      basehp0 REAL NOT NULL, basehp1 REAL NOT NULL, basehp2 REAL NOT NULL,
      basemana REAL NOT NULL, basearmor REAL NOT NULL, attackpower REAL NOT NULL,
      rangedattackpower REAL NOT NULL, damage_base REAL NOT NULL,
      damage_exp1 REAL NOT NULL, damage_exp2 REAL NOT NULL, PRIMARY KEY(level,class)
    )`,
  },
  {
    name: 'creature_model_info',
    columns: ['DisplayID', 'BoundingRadius', 'CombatReach'],
    createSql: 'CREATE TABLE creature_model_info (DisplayID INTEGER PRIMARY KEY, BoundingRadius REAL NOT NULL, CombatReach REAL NOT NULL)',
  },
  {
    name: 'creature_template_movement',
    columns: ['CreatureId', 'Ground', 'Swim', 'Flight', 'Rooted'],
    createSql: 'CREATE TABLE creature_template_movement (CreatureId INTEGER PRIMARY KEY, Ground INTEGER, Swim INTEGER, Flight INTEGER, Rooted INTEGER)',
  },
  {
    name: 'creature_movement_override',
    columns: ['SpawnId', 'Ground', 'Swim', 'Flight', 'Rooted'],
    createSql: 'CREATE TABLE creature_movement_override (SpawnId INTEGER PRIMARY KEY, Ground INTEGER, Swim INTEGER, Flight INTEGER, Rooted INTEGER)',
  },
  {
    name: 'factiontemplate_dbc',
    columns: ['ID', 'FactionGroup', 'FriendGroup', 'EnemyGroup'],
    createSql: 'CREATE TABLE factiontemplate_dbc (ID INTEGER PRIMARY KEY, FactionGroup INTEGER NOT NULL, FriendGroup INTEGER NOT NULL, EnemyGroup INTEGER NOT NULL)',
  },
];

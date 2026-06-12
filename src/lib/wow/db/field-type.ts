/**
 * DB2 field types, ported from wow.export (src/js/db/FieldType.js).
 */
export enum FieldType {
  String = 'String',
  Int8 = 'Int8',
  UInt8 = 'UInt8',
  Int16 = 'Int16',
  UInt16 = 'UInt16',
  Int32 = 'Int32',
  UInt32 = 'UInt32',
  Int64 = 'Int64',
  UInt64 = 'UInt64',
  Float = 'Float',
  Relation = 'Relation',
  NonInlineID = 'NonInlineID',
}

export default FieldType;

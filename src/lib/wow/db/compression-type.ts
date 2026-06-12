/**
 * DB2 field compression types, ported from wow.export (src/js/db/CompressionType.js).
 */
export enum CompressionType {
  None = 0,
  Bitpacked = 1,
  CommonData = 2,
  BitpackedIndexed = 3,
  BitpackedIndexedArray = 4,
  BitpackedSigned = 5,
}

export default CompressionType;

package db

// FieldType describes a DB2 schema field type.
type FieldType int

const (
	FieldTypeString FieldType = iota
	FieldTypeInt8
	FieldTypeUInt8
	FieldTypeInt16
	FieldTypeUInt16
	FieldTypeInt32
	FieldTypeUInt32
	FieldTypeInt64
	FieldTypeUInt64
	FieldTypeFloat
	FieldTypeRelation
	FieldTypeNonInlineID
)

// SchemaEntry is either a single field type or a typed array.
type SchemaEntry struct {
	Type  FieldType
	Count int // 0 = scalar, >0 = array length
}

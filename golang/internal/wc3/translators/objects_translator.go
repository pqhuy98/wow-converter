package translators

import (
	"fmt"
	"sort"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

var (
	objectsTranslatorsMu sync.Mutex
	objectsTranslators   = map[data.ObjectType]*ObjectsTranslator{}
)

// ObjectsTranslator handles w3u/w3d/w3b/w3a object data tables.
type ObjectsTranslator struct {
	objectType data.ObjectType
}

var varTypes = map[data.ModificationType]int{
	data.ModificationInt:    0,
	data.ModificationReal:   1,
	data.ModificationUnreal: 2,
	data.ModificationString: 3,
}

var varTypesReverse = map[int]data.ModificationType{
	0: data.ModificationInt,
	1: data.ModificationReal,
	2: data.ModificationUnreal,
	3: data.ModificationString,
}

// GetObjectsTranslator returns a translator for the given object type.
func GetObjectsTranslator(objectType data.ObjectType) *ObjectsTranslator {
	objectsTranslatorsMu.Lock()
	defer objectsTranslatorsMu.Unlock()
	if t, ok := objectsTranslators[objectType]; ok {
		return t
	}
	t := &ObjectsTranslator{objectType: objectType}
	objectsTranslators[objectType] = t
	return t
}

// JSONToWarObjects serializes an object modification table.
func JSONToWarObjects(objectType data.ObjectType, table data.ObjectModificationTable) wc3.WarResult {
	return GetObjectsTranslator(objectType).jsonToWar(table)
}

// WarToJSONObjects parses object modification bytes.
func WarToJSONObjects(objectType data.ObjectType, buffer []byte) wc3.JsonResult[data.ObjectModificationTable] {
	return GetObjectsTranslator(objectType).warToJSON(buffer)
}

func (t *ObjectsTranslator) jsonToWar(json data.ObjectModificationTable) wc3.WarResult {
	out := wc3.NewHexBufferWriter()
	out.AddInt(2)
	t.writeTable(out, data.TableOriginal, json.Original)
	t.writeTable(out, data.TableCustom, json.Custom)
	return wc3.WarResult{Buffer: out.GetBuffer()}
}

func (t *ObjectsTranslator) writeTable(out *wc3.HexBuffer, tableType data.TableType, tableData map[string][]data.Modification) {
	if tableData == nil {
		out.AddInt(0)
		return
	}
	keys := make([]string, 0, len(tableData))
	for k := range tableData {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out.AddInt(len(keys))
	for _, defKey := range keys {
		obj := tableData[defKey]
		if tableType == data.TableOriginal {
			out.AddChars(defKey)
			out.AddByte(0)
			out.AddByte(0)
			out.AddByte(0)
			out.AddByte(0)
		} else if len(defKey) >= 9 {
			out.AddChars(defKey[5:9])
			out.AddChars(defKey[0:4])
		} else {
			out.AddChars(defKey)
			out.AddByte(0)
			out.AddByte(0)
			out.AddByte(0)
			out.AddByte(0)
		}

		out.AddInt(len(obj))
		for _, mod := range obj {
			out.AddChars(mod.ID)
			modType := resolveModType(mod)
			out.AddInt(modType)

			if t.objectType == data.ObjectDoodads || t.objectType == data.ObjectAbilities || t.objectType == data.ObjectUpgrades {
				levelOrVariation := mod.Level
				if levelOrVariation == 0 && mod.Variation != 0 {
					levelOrVariation = mod.Variation
				}
				out.AddInt(levelOrVariation)
				out.AddInt(mod.Column)
			}

			switch varTypesReverse[modType] {
			case data.ModificationInt:
				out.AddInt(asInt(mod.Value))
			case data.ModificationReal, data.ModificationUnreal:
				out.AddFloat(asFloat(mod.Value))
			case data.ModificationString:
				out.AddString(asString(mod.Value))
			}

			if tableType == data.TableOriginal {
				out.AddChars(defKey)
			} else {
				out.AddByte(0)
				out.AddByte(0)
				out.AddByte(0)
				out.AddByte(0)
			}
		}
	}
}

func (t *ObjectsTranslator) warToJSON(buffer []byte) wc3.JsonResult[data.ObjectModificationTable] {
	result := data.NewObjectModificationTable()
	buf := wc3.NewW3Buffer(buffer)
	fileVersion := buf.ReadInt()
	t.readModificationTable(buf, fileVersion, true, &result)
	t.readModificationTable(buf, fileVersion, false, &result)
	return wc3.JsonResult[data.ObjectModificationTable]{JSON: result}
}

func (t *ObjectsTranslator) readModificationTable(buf *wc3.W3Buffer, fileVersion int32, isOriginal bool, result *data.ObjectModificationTable) {
	numTableModifications := int(buf.ReadInt())
	for i := 0; i < numTableModifications; i++ {
		objectDefinition := []data.Modification{}
		originalID := buf.ReadChars(4)
		customID := buf.ReadChars(4)

		sets := 1
		if fileVersion >= 3 {
			sets = int(buf.ReadInt())
		}

		for j := 0; j < sets; j++ {
			if fileVersion >= 3 {
				buf.ReadInt()
			}
			modificationCount := int(buf.ReadInt())
			for k := 0; k < modificationCount; k++ {
				modification := data.Modification{Type: data.ModificationString}
				modification.ID = buf.ReadChars(4)
				modTypeInt := int(buf.ReadInt())
				modification.Type = varTypesReverse[modTypeInt]

				if t.objectType == data.ObjectDoodads || t.objectType == data.ObjectAbilities || t.objectType == data.ObjectUpgrades {
					modification.Level = int(buf.ReadInt())
					modification.Column = int(buf.ReadInt())
				}

				switch modification.Type {
				case data.ModificationInt:
					modification.Value = buf.ReadInt()
				case data.ModificationReal, data.ModificationUnreal:
					modification.Value = buf.ReadFloat()
				default:
					modification.Value = buf.ReadString()
				}

				if isOriginal {
					buf.ReadInt()
				} else {
					buf.ReadChars(4)
				}
				objectDefinition = append(objectDefinition, modification)
			}
		}

		if isOriginal {
			result.Original[originalID] = objectDefinition
		} else {
			result.Custom[customID+":"+originalID] = objectDefinition
		}
	}
}

func resolveModType(mod data.Modification) int {
	if mod.Type != "" {
		if v, ok := varTypes[mod.Type]; ok {
			return v
		}
	}
	switch mod.Value.(type) {
	case int, int32, int64, float64, float32:
		if mod.Type == data.ModificationReal || mod.Type == data.ModificationUnreal {
			return varTypes[data.ModificationReal]
		}
		return varTypes[data.ModificationInt]
	case string:
		return varTypes[data.ModificationString]
	default:
		panic(fmt.Errorf("no type specified and cannot infer type for mod %s", mod.ID))
	}
}

func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	default:
		return 0
	}
}

func asFloat(v any) float32 {
	switch n := v.(type) {
	case float32:
		return n
	case float64:
		return float32(n)
	case int:
		return float32(n)
	case int32:
		return float32(n)
	default:
		return 0
	}
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

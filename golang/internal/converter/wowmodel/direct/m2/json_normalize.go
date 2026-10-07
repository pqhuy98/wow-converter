package directm2

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// NormalizeJSONValues round-trips v through JSON to normalize undefined/NaN/BigInt like the TS pipeline.
func NormalizeJSONValues(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		b, err = json.Marshal(sanitize(reflect.ValueOf(v)))
		if err != nil {
			return v
		}
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}

func sanitize(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		return sanitize(v.Elem())
	case reflect.Map:
		if v.IsNil() {
			return nil
		}
		out := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			key := iter.Key()
			var name string
			switch key.Kind() {
			case reflect.String:
				name = key.String()
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				name = strconv.FormatInt(key.Int(), 10)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				name = strconv.FormatUint(key.Uint(), 10)
			default:
				continue
			}
			out[name] = sanitize(iter.Value())
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return nil
		}
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return v.Interface()
		}
		fallthrough
	case reflect.Array:
		out := make([]any, v.Len())
		for i := range out {
			out[i] = sanitize(v.Index(i))
		}
		return out
	case reflect.Struct:
		if v.CanInterface() {
			if marshaler, ok := v.Interface().(json.Marshaler); ok {
				if data, err := marshaler.MarshalJSON(); err == nil {
					var out any
					if json.Unmarshal(data, &out) == nil {
						return sanitize(reflect.ValueOf(out))
					}
				}
			}
		}
		out := make(map[string]any, v.NumField())
		for i := range v.NumField() {
			field := v.Type().Field(i)
			if field.PkgPath != "" {
				continue
			}
			tag := field.Tag.Get("json")
			if tag == "-" {
				continue
			}
			name, options, _ := strings.Cut(tag, ",")
			if name == "" {
				name = field.Name
			}
			if strings.Contains(options, "omitempty") && v.Field(i).IsZero() {
				continue
			}
			out[name] = sanitize(v.Field(i))
		}
		return out
	case reflect.Float32, reflect.Float64:
		n := v.Float()
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return nil
		}
		return n
	case reflect.Bool:
		return v.Bool()
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	default:
		if v.CanInterface() {
			return v.Interface()
		}
		return nil
	}
}

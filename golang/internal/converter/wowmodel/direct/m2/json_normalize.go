package directm2

import (
	"encoding/json"
	"math"
)

// NormalizeJSONValues round-trips v through JSON to normalize undefined/NaN/BigInt like the TS pipeline.
func NormalizeJSONValues(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return sanitize(out)
}

func sanitize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			x[k] = sanitize(val)
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = sanitize(val)
		}
		return x
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil
		}
		if x == 0 {
			return 0.0
		}
		return x
	default:
		return v
	}
}

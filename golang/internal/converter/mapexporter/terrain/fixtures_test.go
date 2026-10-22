package terrain

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

func fieldValues(t *testing.T, mods []data.Modification) map[string]interface{} {
	t.Helper()
	out := map[string]interface{}{}
	for _, m := range mods {
		if _, exists := out[m.ID]; exists {
			t.Fatalf("duplicate WE field %s", m.ID)
		}
		out[m.ID] = m.Value
	}
	return out
}

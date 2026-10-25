package creature

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestShadowSkinOverridesPreserveEditorVersionsAndUnrelatedBytes(t *testing.T) {
	for _, version := range []uint32{2, 3} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			root := t.TempDir()
			mm := extra.NewMapManager()
			var mods []data.Modification
			values := map[string]float32{"ushw": 146, "ushh": 146, "ushx": 73, "ushy": 73}
			for _, id := range []string{"ushw", "ushh", "ushx", "ushy"} {
				mods = append(mods, data.Modification{ID: id, Type: data.ModificationReal, Value: values[id]})
			}
			unit := mm.AddUnitType(false, "hfoo", mods)
			var b bytes.Buffer
			putInt := func(v uint32) { _ = binary.Write(&b, binary.LittleEndian, v) }
			type replacement struct {
				offset int
				value  float32
			}
			var replacements []replacement
			putObject := func(code string, replace bool) {
				b.WriteString("hfoo" + code)
				sets := uint32(1)
				if version == 3 {
					sets = 2
					putInt(sets)
				}
				for set := uint32(0); set < sets; set++ {
					if version == 3 {
						putInt(set * 2)
					}
					putInt(6)
					for _, id := range []string{"ushw", "ushh", "ushx", "ushy"} {
						b.WriteString(id)
						putInt(1)
						if replace {
							replacements = append(replacements, replacement{b.Len(), values[id]})
						}
						putInt(math.Float32bits(999))
						putInt(0)
					}
					b.WriteString("unam")
					putInt(3)
					b.WriteString("preserve ushw text and skin flags\x00")
					putInt(0)
					b.WriteString("utco")
					putInt(0)
					putInt(17)
					putInt(0)
				}
			}
			putInt(version)
			putInt(1)
			putObject("\x00\x00\x00\x00", false)
			putInt(2)
			putObject(unit.Code, true)
			putObject("zzzz", false)
			original := append([]byte(nil), b.Bytes()...)
			path := filepath.Join(root, "war3mapSkin.w3u")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			if err := mm.Save(root); err != nil {
				t.Fatal(err)
			}
			if err := RefreshShadowSkinOverrides(root, mm); err != nil {
				t.Fatal(err)
			}
			expected := append([]byte(nil), original...)
			for _, update := range replacements {
				binary.LittleEndian.PutUint32(expected[update.offset:], math.Float32bits(update.value))
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, expected) {
				t.Fatal("shadow refresh changed editor format/sets or unrelated fields")
			}
			if err := RefreshShadowSkinOverrides(root, mm); err != nil {
				t.Fatal(err)
			}
			// A malformed skin must fail without writing partially patched bytes.
			truncated := original[:len(original)-1]
			if err := os.WriteFile(path, truncated, 0600); err != nil {
				t.Fatal(err)
			}
			if err := RefreshShadowSkinOverrides(root, mm); err == nil {
				t.Fatal("truncated skin accepted")
			}
			actual, err = os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, truncated) {
				t.Fatal("failed parse changed the skin file")
			}
		})
	}
}

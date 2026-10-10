package translators

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

func TestModernPlacementFields(t *testing.T) {
	state := byte(5)
	doodads := data.DoodadList{Doodads: []data.Doodad{{Type: "LTlt", SkinID: "LTlt", GroupID: 123, Unknown1: -7, Angle: 37.5, Roll: 0.25, Pitch: -0.75, State: &state, Life: 0, ID: 17, Scale: [3]float32{1, 2, 3}, Lights: []data.DoodadLight{{Index: 2, ShadowCasting: 1, Color: -1, Intensity: 3.5, ShadowCastingStart: 1, ShadowCastingEnd: 2, QuadraticFalloff: 4, LinearFalloff: 5, Damping: 6}}}}}
	for _, version := range []int{12, 13} {
		encoded := DoodadsTranslator{}.JSONToWar(doodads, version).Buffer
		parsed := DoodadsTranslator{}.WarToJSON(encoded)
		if parsed.FormatVersion != version || parsed.JSON.Doodads[0].Life != 0 || *parsed.JSON.Doodads[0].State != state || len(parsed.JSON.Doodads[0].Lights) != 1 {
			t.Fatal("lost modern doodad fields")
		}
		if !bytes.Equal(encoded, DoodadsTranslator{}.JSONToWar(parsed.JSON, version).Buffer) {
			t.Fatal("doodad bytes changed")
		}
	}
	units := []data.Unit{{Type: "hfoo", Skin: "hfoo", GroupID: 41, Flags: 9, UnknownBytes: [2]byte{1, 2}, UnknownTail: [3]int32{3, 4, 5}, Player: 1, Color: 0, ID: 18, Random: data.UnitRandom{Level: 0xffffff}}}
	encoded := UnitsTranslator{}.JSONToWar(units, 13).Buffer
	parsed := UnitsTranslator{}.WarToJSON(encoded)
	if parsed.JSON[0].Random.Level != 0xffffff || !bytes.Equal(encoded, UnitsTranslator{}.JSONToWar(parsed.JSON, 13).Buffer) {
		t.Fatal("unit bytes changed")
	}
	regions := []data.Region{{Name: "Region", CameraBlocker: 2, AlphaTileMinimapColor: -1}}
	encoded = RegionsTranslator{}.JSONToWar(regions, 7).Buffer
	if !bytes.Equal(encoded, RegionsTranslator{}.JSONToWar(RegionsTranslator{}.WarToJSON(encoded).JSON, 7).Buffer) {
		t.Fatal("region bytes changed")
	}
	cameras := []data.Camera{{Name: "Camera", DofDistance: 123.456, DofScale: 0.25, PosAbsoluteZ: 37.5, CameraType: 2}}
	encoded = CamerasTranslator{}.JSONToWar(cameras, 3).Buffer
	if !bytes.Equal(encoded, CamerasTranslator{}.JSONToWar(CamerasTranslator{}.WarToJSON(encoded).JSON, 3).Buffer) {
		t.Fatal("camera bytes changed")
	}
	info := DefaultInfo()
	info.FileVersion = 39
	info.RaceHud = 3
	info.UnknownFlags = 1 << 25
	info.Players[0].RaceHud = 4
	info.Fog3 = data.Fog3{HeightStart: 1.23456, HeightEnd: 2, LinearStart: 3, LinearEnd: 4, MaxOpacity: 0.25, DrawOverSky: 1}
	info.Water3 = data.Water3{MinOpacity: 1, MaxOpacity: 2, Reflectivity: 3, Emissivity: 4, EdgeSoftness: 5, WavesVertexDisplacement: 6, WavesNormalMapStrength: 7, OverrideColor: -1, EnvmapReflectivity: 9, AlphaTileMinimapColor: -2}
	encoded = InfoTranslator{}.JSONToWar(info).Buffer
	if !bytes.Equal(encoded, InfoTranslator{}.JSONToWar(InfoTranslator{}.WarToJSON(encoded).JSON).Buffer) {
		t.Fatal("info bytes changed")
	}
}

// Optional real-map acceptance check: WC3_MAP_DIR points to an unpacked map.
func TestMapFormatRoundTrip(t *testing.T) {
	dir := os.Getenv("WC3_MAP_DIR")
	if dir == "" {
		t.Skip("set WC3_MAP_DIR to check a real map")
	}
	checks := map[string]func([]byte) wc3.WarResult{
		"war3map.w3e": func(b []byte) wc3.WarResult {
			return TerrainTranslator{}.JSONToWar(TerrainTranslator{}.WarToJSON(b).JSON)
		},
		"war3map.w3i": func(b []byte) wc3.WarResult { return InfoTranslator{}.JSONToWar(InfoTranslator{}.WarToJSON(b).JSON) },
		"war3map.doo": func(b []byte) wc3.WarResult {
			r := DoodadsTranslator{}.WarToJSON(b)
			return DoodadsTranslator{}.JSONToWar(r.JSON, r.FormatVersion)
		},
		"war3mapUnits.doo": func(b []byte) wc3.WarResult {
			r := UnitsTranslator{}.WarToJSON(b)
			return UnitsTranslator{}.JSONToWar(r.JSON, r.FormatVersion)
		},
		"war3map.w3r": func(b []byte) wc3.WarResult {
			r := RegionsTranslator{}.WarToJSON(b)
			return RegionsTranslator{}.JSONToWar(r.JSON, r.FormatVersion)
		},
		"war3map.w3c": func(b []byte) wc3.WarResult {
			r := CamerasTranslator{}.WarToJSON(b)
			return CamerasTranslator{}.JSONToWar(r.JSON, r.FormatVersion)
		},
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			out := check(b).Buffer
			if !bytes.Equal(b, out) {
				for i := 0; i < len(b) && i < len(out); i++ {
					if b[i] != out[i] {
						t.Fatalf("different byte at %d: %02x vs %02x (sizes %d/%d)", i, b[i], out[i], len(b), len(out))
					}
				}
				t.Fatalf("size %d/%d", len(b), len(out))
			}
			t.Logf("round-trip %d bytes", len(b))
		})
	}
}

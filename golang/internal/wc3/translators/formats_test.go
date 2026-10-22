package translators

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

func TestUnitsUseNativeFormatVersion(t *testing.T) {
	// Two consecutive World Editor v8/11 placements, including Reforged skins.
	// These bytes come from a native save, not this translator's writer.
	native, err := hex.DecodeString("5733646f080000000b000000020000006f777976000000005deb9ec3281f41c4c0d10bc2c66b5a400000803f0000803f0000803f6f77797602000000000000ffffffffffffffffffffffff00000000d4300000000080bf0100000000000000000000000000000000000000000000000000000001000000ffffffffffffffff890000006f77797600000000f85da1c306b9aac4142252c26118f83f0000803f0000803f0000803f6f77797602000000000000ffffffffffffffffffffffff00000000d4300000000080bf0100000000000000000000000000000000000000000000000000000001000000ffffffffffffffff88000000")
	if err != nil {
		t.Fatal(err)
	}
	parsed := UnitsTranslator{}.WarToJSON(native)
	if len(parsed.JSON) != 2 || parsed.JSON[0].Type != "owyv" || parsed.JSON[1].ID != 136 {
		t.Fatal("failed to read native placements")
	}
	for _, version := range []int{0, 8, 9} {
		encoded := UnitsTranslator{}.JSONToWar(parsed.JSON, version).Buffer
		if !bytes.Equal(encoded, native) {
			t.Fatalf("version %d did not produce the native v8/11 layout", version)
		}
	}
	m := NewMapTranslator()
	m.SetMapDir(t.TempDir())
	m.Units = parsed.JSON
	m.formatVersions = map[FilePath]int{FileUnits: 9}
	if err := m.Save(FileUnits); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(m.filePaths[FileUnits])
	if err != nil || !bytes.Equal(saved, native) {
		t.Fatalf("cached map save did not repair the invalid version: %v", err)
	}
}

func TestModernPlacementFields(t *testing.T) {
	state := byte(5)
	groupID := int32(123)
	doodads := data.DoodadList{Doodads: []data.Doodad{{Type: "LTlt", SkinID: "LTlt", GroupID: &groupID, Unknown1: -7, Angle: 37.5, Roll: 0.25, Pitch: -0.75, State: &state, Life: 0, ID: 17, Scale: [3]float32{1, 2, 3}, Lights: []data.DoodadLight{{Index: 2, ShadowCasting: 1, Color: -1, Intensity: 3.5, ShadowCastingStart: 1, ShadowCastingEnd: 2, QuadraticFalloff: 4, LinearFalloff: 5, Damping: 6}}}}}
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

func TestAlphaTileMinimapColorFlagCanBeDisabled(t *testing.T) {
	info := DefaultInfo()
	info.FileVersion = 39
	// Native bit 24 enables the custom alpha-tile color; bit 25 is unrelated.
	info.UnknownFlags = 1<<24 | 1<<25
	parsed := InfoTranslator{}.WarToJSON(InfoTranslator{}.JSONToWar(info).Buffer).JSON
	if !parsed.Map.Flags.UseAlphaTileMinimapColor || parsed.UnknownFlags != 1<<25 {
		t.Fatal("alpha-tile color enable bit was not decoded separately")
	}
	parsed.Map.Flags.UseAlphaTileMinimapColor = false
	parsed = InfoTranslator{}.WarToJSON(InfoTranslator{}.JSONToWar(parsed).Buffer).JSON
	if parsed.Map.Flags.UseAlphaTileMinimapColor || parsed.UnknownFlags != 1<<25 {
		t.Fatal("clearing alpha-tile color enable bit changed unrelated flags")
	}
}

func TestDoodadTiltPromotesOlderFormats(t *testing.T) {
	for _, version := range []int{0, 8, 12, 13} {
		for _, tilt := range []data.Doodad{{Roll: .2}, {Pitch: -.4}} {
			tilt.Type = "LTlt"
			encoded := DoodadsTranslator{}.JSONToWar(data.DoodadList{Doodads: []data.Doodad{tilt}}, version)
			parsed := DoodadsTranslator{}.WarToJSON(encoded.Buffer)
			wantVersion := max(12, version)
			if parsed.FormatVersion != wantVersion || len(parsed.JSON.Doodads) != 1 || parsed.JSON.Doodads[0].Roll != tilt.Roll || parsed.JSON.Doodads[0].Pitch != tilt.Pitch {
				t.Fatalf("version %d lost native tilt", version)
			}
		}
	}
	// Ordinary legacy placements retain their existing binary layout.
	parsed := DoodadsTranslator{}.WarToJSON(DoodadsTranslator{}.JSONToWar(data.DoodadList{}).Buffer)
	if parsed.FormatVersion != 8 {
		t.Fatal("legacy format changed without modern fields")
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

func TestNativeTerrainVariationBitFields(t *testing.T) {
	for _, version := range []int{11, 12} {
		// Four native corners: ground variation 17 in low five bits,
		// cliff variation 5 in the high three bits. Independent byte fixture.
		w := wc3.NewHexBufferWriter()
		w.AddChars("W3E!")
		w.AddInt(version)
		w.AddChar("L")
		w.AddInt(1)
		w.AddInt(1)
		w.AddChars("Ldrt")
		w.AddInt(1)
		w.AddChars("CLdi")
		w.AddInt(2)
		w.AddInt(2)
		w.AddFloat(0)
		w.AddFloat(0)
		for i := 0; i < 4; i++ {
			w.AddShort(8192)
			w.AddShort(0)
			if version == 12 {
				w.AddShort(0)
			} else {
				w.AddByte(0)
			}
			w.AddByte(0xb1)
			w.AddByte(7)
		}
		terrain := TerrainTranslator{}.WarToJSON(w.GetBuffer()).JSON
		if terrain.GroundVariation[0][0] != 17 || terrain.CliffVariation[0][0] != 0xa0 {
			t.Fatalf("v%d: ground/cliff %d/%d", version, terrain.GroundVariation[0][0], terrain.CliffVariation[0][0])
		}
	}
}

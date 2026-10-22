package translators

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

func TestDoodadsWithoutGroupsStayUngrouped(t *testing.T) {
	for _, version := range []int{8, 12, 13} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			doodads := data.DoodadList{Doodads: []data.Doodad{
				{Type: "LTlt", Roll: .2}, {Type: "LTlt"},
			}}
			encoded := DoodadsTranslator{}.JSONToWar(doodads, version).Buffer
			// The native v12+ group field follows type/variation/transform/skin.
			if group := int32(binary.LittleEndian.Uint32(encoded[56:60])); group != -1 {
				t.Fatalf("omitted group became editor group %d; want -1 (ungrouped)", group)
			}
			parsed := DoodadsTranslator{}.WarToJSON(encoded)
			for i, doodad := range parsed.JSON.Doodads {
				one := DoodadsTranslator{}.JSONToWar(data.DoodadList{Doodads: []data.Doodad{doodad}}, parsed.FormatVersion).Buffer
				if group := int32(binary.LittleEndian.Uint32(one[56:60])); group != -1 {
					t.Fatalf("placement %d became editor group %d", i, group)
				}
			}
		})
	}
	// A legacy placement has no group field; adding tilt must not invent group 0.
	legacy := DoodadsTranslator{}.JSONToWar(data.DoodadList{Doodads: []data.Doodad{{Type: "LTlt"}}}, 8)
	parsed := DoodadsTranslator{}.WarToJSON(legacy.Buffer)
	parsed.JSON.Doodads[0].Pitch = .3
	promoted := DoodadsTranslator{}.JSONToWar(parsed.JSON, 8).Buffer
	if group := int32(binary.LittleEndian.Uint32(promoted[56:60])); group != -1 {
		t.Fatalf("legacy upgrade invented editor group %d", group)
	}
}

func TestDoodadExplicitGroupsSurviveRoundTrip(t *testing.T) {
	for _, version := range []int{12, 13} {
		for _, groupID := range []int32{-1, 0, 123} {
			doodads := data.DoodadList{Doodads: []data.Doodad{{Type: "LTlt", GroupID: &groupID}}}
			encoded := DoodadsTranslator{}.JSONToWar(doodads, version).Buffer
			if got := int32(binary.LittleEndian.Uint32(encoded[56:60])); got != groupID {
				t.Fatalf("version %d changed group %d to %d", version, groupID, got)
			}
			parsed := DoodadsTranslator{}.WarToJSON(encoded)
			if !bytes.Equal(encoded, DoodadsTranslator{}.JSONToWar(parsed.JSON, parsed.FormatVersion).Buffer) {
				t.Fatalf("version %d changed group %d on round trip", version, groupID)
			}
		}
	}
}

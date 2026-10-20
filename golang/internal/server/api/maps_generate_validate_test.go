package api

import "testing"

func validGenerateWc3Body() map[string]any {
	return map[string]any{
		"tiles":       []any{map[string]any{"x": float64(1), "y": float64(2)}},
		"quality":     float64(4096),
		"mapSaveName": "test-map",
		"clampLower":  float64(0),
		"clampUpper":  float64(1),
		"mapAngleDeg": float64(0),
		"unitScale":   float64(1),
		"freshExport": true,
		"creatures": map[string]any{
			"enable":        false,
			"allAreDoodads": false,
		},
	}
}

func TestParseGenerateWc3BodyAppliesDefaults(t *testing.T) {
	body, issues := parseGenerateWc3Body(validGenerateWc3Body())
	if len(issues) > 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if !body.AutoClampPercent {
		t.Fatal("autoClampPercent should default to true")
	}
	if body.PlacePathBlockers {
		t.Fatal("placePathBlockers should default to false")
	}
	if body.IncludeBuildingInteriors == nil || !*body.IncludeBuildingInteriors {
		t.Fatal("includeBuildingInteriors should default to true")
	}
	if body.MapSaveName != "test-map.w3x" {
		t.Fatalf("unexpected normalized map name: %q", body.MapSaveName)
	}
}

func TestParseGenerateWc3BodyRejectsInvalidContractValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown field", func(body map[string]any) { body["extra"] = true }},
		{"unsupported quality", func(body map[string]any) { body["quality"] = float64(256) }},
		{"zero unit scale", func(body map[string]any) { body["unitScale"] = float64(0) }},
		{"out of range tile", func(body map[string]any) {
			body["tiles"] = []any{map[string]any{"x": float64(64), "y": float64(2)}}
		}},
		{"missing tile coordinate", func(body map[string]any) {
			body["tiles"] = []any{map[string]any{"x": float64(1)}}
		}},
		{"missing creature flag", func(body map[string]any) {
			body["creatures"] = map[string]any{"enable": false}
		}},
		{"null required boolean", func(body map[string]any) { body["freshExport"] = nil }},
		{"null optional boolean", func(body map[string]any) { body["autoClampPercent"] = nil }},
		{"null path blocker flag", func(body map[string]any) { body["placePathBlockers"] = nil }},
		{"string path blocker flag", func(body map[string]any) { body["placePathBlockers"] = "true" }},
		{"inverted clamp", func(body map[string]any) {
			body["clampLower"] = float64(0.8)
			body["clampUpper"] = float64(0.2)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := validGenerateWc3Body()
			tt.mutate(body)
			if _, issues := parseGenerateWc3Body(body); len(issues) == 0 {
				t.Fatal("expected validation issue")
			}
		})
	}
}

func TestParseGenerateWc3BodyEnablesPathBlockersWithoutNPCExport(t *testing.T) {
	raw := validGenerateWc3Body()
	raw["placePathBlockers"] = true
	body, issues := parseGenerateWc3Body(raw)
	if len(issues) != 0 || !body.PlacePathBlockers || body.Creatures.Enable {
		t.Fatalf("blockers must be independently selectable: %+v, %v", body, issues)
	}
}

func TestMapNavigationOptions(t *testing.T) {
	raw := validGenerateWc3Body()
	raw["navigation"] = map[string]any{"jumpHeight": float64(0), "radius": float64(.5)}
	body, issues := parseGenerateWc3Body(raw)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	if body.Navigation.JumpHeight != 0 || body.Navigation.Radius != .5 || body.Navigation.Height != 1.8 {
		t.Fatal("partial overrides must keep defaults and preserve explicit zero jump height")
	}
	for _, object := range []any{nil, map[string]any{"radius": float64(-1)}, map[string]any{"jumpHeight": nil}, map[string]any{"typo": true}} {
		raw := validGenerateWc3Body()
		raw["navigation"] = object
		if _, issues := parseGenerateWc3Body(raw); len(issues) == 0 {
			t.Fatalf("invalid navigation config accepted: %v", object)
		}
	}
}

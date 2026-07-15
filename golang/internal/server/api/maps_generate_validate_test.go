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

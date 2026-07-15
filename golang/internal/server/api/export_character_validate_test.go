package api

import "testing"

func validExportCharacterBody() map[string]any {
	return map[string]any{
		"character": map[string]any{
			"base": map[string]any{
				"type":  "displayID",
				"value": "123",
			},
			"inGameMovespeed": float64(270),
		},
		"outputFileName": "test-character",
		"optimization":   map[string]any{},
		"format":         "mdx",
	}
}

func TestParseExportCharacterRequestAppliesOptimizationDefaults(t *testing.T) {
	body, issues := parseExportCharacterRequest(validExportCharacterBody())
	if len(issues) > 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if body.Optimization.SortSequences == nil || !*body.Optimization.SortSequences {
		t.Fatal("sortSequences should default to true")
	}
	if body.Optimization.AllMaterialsUnshaded == nil || *body.Optimization.AllMaterialsUnshaded {
		t.Fatal("allMaterialsUnshaded should default to false")
	}
	if body.Optimization.RemoveUnusedVertices == nil || !*body.Optimization.RemoveUnusedVertices ||
		body.Optimization.RemoveUnusedNodes == nil || !*body.Optimization.RemoveUnusedNodes ||
		body.Optimization.RemoveUnusedMaterialsTextures == nil || !*body.Optimization.RemoveUnusedMaterialsTextures {
		t.Fatal("remove-unused optimization flags should default to true")
	}
}

func TestParseExportCharacterRequestRejectsInvalidContractValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing movement speed", func(body map[string]any) {
			delete(body["character"].(map[string]any), "inGameMovespeed")
		}},
		{"bad format", func(body map[string]any) { body["format"] = "obj" }},
		{"bad format version", func(body map[string]any) { body["formatVersion"] = "900" }},
		{"bad texture size", func(body map[string]any) {
			body["optimization"].(map[string]any)["maxTextureSize"] = "2048"
		}},
		{"null optimization flag", func(body map[string]any) {
			body["optimization"].(map[string]any)["sortSequences"] = nil
		}},
		{"bad base type", func(body map[string]any) {
			body["character"].(map[string]any)["base"].(map[string]any)["type"] = "unknown"
		}},
		{"unsafe local path", func(body map[string]any) {
			body["character"].(map[string]any)["base"] = map[string]any{
				"type": "local", "value": "../secret.m2",
			}
		}},
		{"bad size", func(body map[string]any) {
			body["character"].(map[string]any)["size"] = "colossal"
		}},
		{"bad attack tag", func(body map[string]any) {
			body["character"].(map[string]any)["attackTag"] = "Sword"
		}},
		{"bad mount seat", func(body map[string]any) {
			body["character"].(map[string]any)["mount"] = map[string]any{
				"path":       map[string]any{"type": "displayID", "value": "456"},
				"seatOffset": []any{float64(1), float64(2)},
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := validExportCharacterBody()
			tt.mutate(body)
			if _, issues := parseExportCharacterRequest(body); len(issues) == 0 {
				t.Fatal("expected validation issue")
			}
		})
	}
}

func TestExportCharacterResponseUsesJobIDAsVersionID(t *testing.T) {
	response := newExportCharacterResponse("job-version-123")
	if response.VersionID != "job-version-123" {
		t.Fatalf("versionId = %q, want job ID", response.VersionID)
	}
}

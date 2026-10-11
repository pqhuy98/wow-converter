package api

import (
	"encoding/json"
	"testing"
)

func TestSequenceSelectionRequestContract(t *testing.T) {
	raw := validExportCharacterBody()
	character := raw["character"].(map[string]any)
	character["sequenceSelection"] = map[string]any{"fileDataID": 42.0, "buildKey": "build", "indices": []any{2.0, 0.0}}
	body, issues := parseExportCharacterRequest(raw)
	if len(issues) != 0 || body.Character.SequenceSelection.Indices[0] != 0 {
		t.Fatalf("selection contract: %+v %v", body.Character.SequenceSelection, issues)
	}
	for _, indices := range []any{nil, []any{}, []any{-1.0}, []any{1.5}, []any{1.0, 1.0}, []any{"1"}} {
		character["sequenceSelection"].(map[string]any)["indices"] = indices
		if _, issues := parseExportCharacterRequest(raw); len(issues) == 0 {
			t.Fatalf("accepted indices %v", indices)
		}
	}
}

func TestExportMetadataRequestCanBeReplayed(t *testing.T) {
	body, issues := parseExportCharacterRequest(validExportCharacterBody())
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var replay map[string]any
	if err := json.Unmarshal(encoded, &replay); err != nil {
		t.Fatal(err)
	}
	if _, issues := parseExportCharacterRequest(replay); len(issues) > 0 {
		t.Fatalf("saved request cannot be re-exported: %s: %v", encoded, issues)
	}
	replay["optimization"].(map[string]any)["maxTextureSize"] = ""
	if _, issues := parseExportCharacterRequest(replay); len(issues) > 0 {
		t.Fatalf("legacy saved request cannot be re-exported: %v", issues)
	}
}

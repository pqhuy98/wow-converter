package character

import (
	"context"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	wowchar "github.com/pqhuy98/wow-converter/internal/wow/character"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

type metadataClient struct {
	client.Client
	response casc.CharacterMetaResponse
}

func (c metadataClient) GetCharMeta(context.Context, casc.CharacterMetaParams) (casc.CharacterMetaResponse, error) {
	return c.response, nil
}

func TestResolveCharMetaPreservesEveryTypedChoice(t *testing.T) {
	ctx := &ExportContext{WowClient: metadataClient{response: casc.CharacterMetaResponse{
		FileDataID: 123, Choices: map[string]wowchar.ChoiceMeta{
			"11": {Geosets: []int{101}},
			"22": {Materials: []wowchar.CharMetaChoiceMaterial{{CustMaterial: wowchar.ChrCustomizationMaterialEntry{FileDataID: 234}}}},
		},
	}}}
	meta, choices, err := resolveCharMeta(ctx, ExportCharacterParams{})
	if err != nil {
		t.Fatal(err)
	}
	if meta.FileDataID != 123 || len(meta.Choices) != 2 || len(choices) != 2 || meta.Choices[11].Geosets[0] != 101 || choices[22].Materials[0].CustMaterial.FileDataID != 234 {
		t.Fatalf("metadata lost choices: %#v, %#v", meta, choices)
	}
}

func TestResolveCharMetaRejectsInvalidChoiceIdentity(t *testing.T) {
	ctx := &ExportContext{WowClient: metadataClient{response: casc.CharacterMetaResponse{Choices: map[string]wowchar.ChoiceMeta{"11-bad": {}}}}}
	if _, _, err := resolveCharMeta(ctx, ExportCharacterParams{}); err == nil {
		t.Fatal("invalid choice key became a partial integer identity")
	}
}

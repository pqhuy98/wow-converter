//go:build integration_tests

package directm2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/transport"
)

func TestAdvancedTextureAnimationRealM2Variants(t *testing.T) {
	if os.Getenv("WOW_DATA_SERVER_URL") == "" {
		transport.ConfigureBundled()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	base := cascSource{client: client.NewHTTPClient("")}
	src := textureAnimationIntegrationSource{cascSource: base}
	readyCtx, stopWaiting := context.WithTimeout(ctx, 60*time.Second)
	defer stopWaiting()
	for {
		attemptCtx, stopAttempt := context.WithTimeout(readyCtx, 3*time.Second)
		_, err := base.client.GetCASCInfo(attemptCtx)
		stopAttempt()
		if err == nil {
			break
		}
		select {
		case <-readyCtx.Done():
			t.Fatalf("existing wow-data-server did not become ready: %v", err)
		case <-time.After(time.Second):
		}
	}

	testCases := []struct {
		name                 string
		fileDataID           int
		skinName             string
		skinNamePrefix       string
		mustContainTexture   int
		requireGeoset3       bool
		primaryTextureCount  int
		requiredExtraGeosets []int
	}{
		{name: "Akilzon", fileDataID: 6225101, skinName: "amanieagleloa_akilzon_skin", mustContainTexture: 6436414, requireGeoset3: true},
		{name: "Aether Serpent", fileDataID: 3040716, skinName: "aetherserpentmount"},
		{name: "Elemental Primalist", fileDataID: 4497519, skinName: "elementalprimalist_4517805_display143934", primaryTextureCount: 3},
		{name: "Thunder Lizard", fileDataID: 4004565, skinNamePrefix: "thunderlizardprimal_black", primaryTextureCount: 1, requiredExtraGeosets: []int{103, 203, 302}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			skins, err := src.GetModelSkins(ctx, testCase.fileDataID)
			if err != nil {
				t.Fatalf("load model skins: %v", err)
			}
			var selectedSkin *ModelSkin
			for i := range skins {
				matchesName := testCase.skinName != "" && skins[i].ID == testCase.skinName
				matchesPrefix := testCase.skinNamePrefix != "" && strings.HasPrefix(skins[i].ID, testCase.skinNamePrefix)
				if (!matchesName && !matchesPrefix) || !containsAllInts(skins[i].ExtraGeosets, testCase.requiredExtraGeosets) {
					continue
				}
				if testCase.primaryTextureCount > 0 && !hasPositiveTexturePrefix(skins[i].Textures, testCase.primaryTextureCount) {
					continue
				}
				if selectedSkin != nil {
					t.Fatalf("skin selection is ambiguous for %d (%q / %q)", testCase.fileDataID, testCase.skinName, testCase.skinNamePrefix)
				}
				selectedSkin = &skins[i]
			}
			if selectedSkin == nil {
				t.Fatalf("model %d has no skin matching %q / %q and selection metadata; available skins: %v", testCase.fileDataID, testCase.skinName, testCase.skinNamePrefix, modelSkinSummaries(skins))
			}
			if testCase.mustContainTexture != 0 && !containsInt(selectedSkin.Textures, testCase.mustContainTexture) {
				t.Fatalf("selected skin %q textures %v do not include replaceable source %d", selectedSkin.ID, selectedSkin.Textures, testCase.mustContainTexture)
			}
			if testCase.primaryTextureCount > 0 {
				if !hasPositiveTexturePrefix(selectedSkin.Textures, testCase.primaryTextureCount) {
					t.Fatalf("selected skin %q textures %v do not have %d positive primary file data IDs", selectedSkin.ID, selectedSkin.Textures, testCase.primaryTextureCount)
				}
			}
			for _, geosetID := range testCase.requiredExtraGeosets {
				if !containsInt(selectedSkin.ExtraGeosets, geosetID) {
					t.Fatalf("selected skin %q extra geosets %v do not contain %d", selectedSkin.ID, selectedSkin.ExtraGeosets, geosetID)
				}
			}
			t.Logf("selected skin %q textures=%v extraGeosets=%v", selectedSkin.ID, selectedSkin.Textures, selectedSkin.ExtraGeosets)

			animate := true
			cfg := config.DefaultConfig()
			cfg.ExportAssetDir = filepath.Join(t.TempDir(), "sources")
			cfg.TextureBaking = config.TextureBakingOptions{
				Enabled: true, Animate: &animate, FPS: 15, WindowMS: 4000, ResolutionScale: 1,
			}
			result, err := ConvertM2ToMdl(ctx, cfg, src, ConvertOptions{
				FileDataID: testCase.fileDataID,
				SkinName:   selectedSkin.ID,
			})
			if err != nil {
				t.Fatalf("convert M2 with advanced animated texture baking: %v", err)
			}

			var animatedGeoset3 bool
			animatedLayers := 0
			for _, geoset := range result.MDL.Geosets {
				if geoset == nil || len(geoset.Faces) == 0 || geoset.Material == nil {
					continue
				}
				for _, layer := range geoset.Material.Layers {
					if layer.Texture == nil || !hasNonconstantGlobalLayerAnimation(layer) {
						continue
					}
					animatedLayers++
					if geoset.Name == "Geoset3" {
						animatedGeoset3 = true
					}
				}
			}
			if animatedLayers == 0 {
				t.Fatal("final rendered layers contain no nonconstant, globally bound UV or texture-ID animation")
			}
			if testCase.requireGeoset3 && !animatedGeoset3 {
				t.Fatal("Akilzon Geoset3 has no animated rendered layer; expected the selected skin's fourth texture to resolve rather than the static fallback")
			}

			assertSerializedTextureAnimations(t, result.MDL)
		})
	}
}

type textureAnimationIntegrationSource struct {
	cascSource
}

func (s textureAnimationIntegrationSource) GetModelSkins(ctx context.Context, fileDataID int) ([]ModelSkin, error) {
	skins, err := s.client.GetModelSkins(ctx, fileDataID)
	if err != nil {
		return nil, err
	}
	result := make([]ModelSkin, 0, len(skins))
	for _, skin := range skins {
		result = append(result, ModelSkin{ID: skin.ID, ExtraGeosets: skin.ExtraGeosets, Textures: skin.Textures})
	}
	return result, nil
}

func hasNonconstantGlobalLayerAnimation(layer components.Layer) bool {
	if layer.TVertexAnim != nil && (hasNonconstantGlobalTrack(layer.TVertexAnim.Translation) || hasNonconstantGlobalTrack(layer.TVertexAnim.Rotation) || hasNonconstantGlobalTrack(layer.TVertexAnim.Scaling)) {
		return true
	}
	return hasNonconstantGlobalTrack(layer.TextureIDAnim)
}

func hasNonconstantGlobalTrack(track *components.Animation) bool {
	if track == nil || track.GlobalSeq == nil || track.GlobalSeq.Duration <= 0 || len(track.KeyFrames) < 2 {
		return false
	}

	var first any
	firstSet := false
	for _, timestamp := range components.SortedKeyInts(track.KeyFrames) {
		value := track.KeyFrames[timestamp]
		if !firstSet {
			first = value
			firstSet = true
			continue
		}
		if !reflect.DeepEqual(first, value) {
			return true
		}
	}
	return false
}

func assertSerializedTextureAnimations(t *testing.T, model *mdl.MDL) {
	t.Helper()
	model.Sync()
	serialized := mdx.NewModel()
	if err := serialized.LoadMdl(model.ToMdl()); err != nil {
		t.Fatalf("serialized MDL did not reload: %v", err)
	}

	boundAnimatedLayers := 0
	for _, material := range serialized.Materials {
		for _, layer := range material.Layers {
			if layer.TextureAnimationID >= 0 {
				if int(layer.TextureAnimationID) >= len(serialized.TextureAnimations) {
					t.Fatalf("layer texture-animation reference %d exceeds %d animations", layer.TextureAnimationID, len(serialized.TextureAnimations))
				}
				animation := serialized.TextureAnimations[layer.TextureAnimationID]
				for _, track := range animation.Animations {
					if track.GlobalSequenceID >= 0 && int(track.GlobalSequenceID) < len(serialized.GlobalSequences) && hasVaryingParserValues(track.Values) {
						boundAnimatedLayers++
					}
				}
			}
			for _, track := range layer.Animations {
				if track.Name == "KMTF" && track.GlobalSequenceID >= 0 && int(track.GlobalSequenceID) < len(serialized.GlobalSequences) && hasVaryingParserValues(track.Values) {
					boundAnimatedLayers++
				}
			}
		}
	}
	if boundAnimatedLayers == 0 {
		t.Fatal("serialized MDL has no valid layer binding to a global texture animation")
	}
}

func hasVaryingParserValues(values []interface{}) bool {
	if len(values) < 2 {
		return false
	}
	for _, value := range values[1:] {
		if !reflect.DeepEqual(values[0], value) {
			return true
		}
	}
	return false
}

func modelSkinSummaries(skins []ModelSkin) []string {
	result := make([]string, 0, len(skins))
	for _, skin := range skins {
		result = append(result, skin.ID+" textures="+fmt.Sprint(skin.Textures)+" extraGeosets="+fmt.Sprint(skin.ExtraGeosets))
	}
	return result
}

func containsInt(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsAllInts(values, want []int) bool {
	for _, value := range want {
		if !containsInt(values, value) {
			return false
		}
	}
	return true
}

func hasPositiveTexturePrefix(textures []int, count int) bool {
	if len(textures) < count {
		return false
	}
	for _, texture := range textures[:count] {
		if texture <= 0 {
			return false
		}
	}
	return true
}

//go:build integration_tests

package directm2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/transport"
)

const trollShoulderModelFileDataID = 4423476
const trollShoulderSkinFileDataID = 4423553

// This checks a global-only UV animation from a real equipment M2. It uses the
// already-running data server; WOW_DATA_TRANSPORT only selects its socket and
// does not start another CASC runtime.
func TestTrollShoulderGlobalUVBakeAnimatedAndStill(t *testing.T) {
	if os.Getenv("WOW_DATA_SERVER_URL") == "" {
		transport.ConfigureBundled()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	src := cascSource{client: client.NewHTTPClient("")}

	readyCtx, stopWaiting := context.WithTimeout(ctx, 60*time.Second)
	defer stopWaiting()
	for {
		attemptCtx, stopAttempt := context.WithTimeout(readyCtx, 3*time.Second)
		_, err := src.client.GetCASCInfo(attemptCtx)
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

	loader, err := loadTrollShoulderModel(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	skin, err := loader.GetSkin(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if skin.FileDataID != trollShoulderSkinFileDataID {
		t.Fatalf("first shoulder skin FID = %d, want %d", skin.FileDataID, trollShoulderSkinFileDataID)
	}
	if len(loader.TextureTransforms) == 0 {
		t.Fatal("shoulder source M2 has no UV transforms")
	}
	replaceableTextures := trollShoulderReplaceableTextures(t)

	animated := true
	animatedResult := convertTrollShoulder(ctx, t, src, t.TempDir(), &animated, replaceableTextures)
	animatedAtlas, animatedGlobal := inspectShoulderBake(t, animatedResult.MDL, true)
	if animatedAtlas == 0 || animatedGlobal == 0 {
		t.Fatalf("animated conversion produced %d baked atlases and %d global flipbooks", animatedAtlas, animatedGlobal)
	}

	still := false
	stillResult := convertTrollShoulder(ctx, t, src, t.TempDir(), &still, replaceableTextures)
	stillAtlas, stillGlobal := inspectShoulderBake(t, stillResult.MDL, false)
	if stillAtlas == 0 {
		t.Fatal("still conversion did not bake any shoulder atlases")
	}
	if stillGlobal != 0 {
		t.Fatalf("still conversion emitted %d animated atlas layers", stillGlobal)
	}
}

func loadTrollShoulderModel(ctx context.Context, src cascSource) (*m2.Loader, error) {
	raw, err := src.GetRawFile(ctx, trollShoulderModelFileDataID)
	if err != nil {
		return nil, err
	}
	loader := m2.NewLoader(buffer.From(raw), func(ctx context.Context, fileDataID uint32) ([]byte, error) {
		return src.GetRawFile(ctx, int(fileDataID))
	})
	if err := loader.Load(ctx); err != nil {
		return nil, err
	}
	return loader, nil
}

func trollShoulderReplaceableTextures(t *testing.T) map[int]int {
	t.Helper()
	// Verified against item display 660000 metadata for this dressing-room URL:
	// https://www.wowhead.com/dressing-room#fR80k0zg89c8ze8NH8zg8NA8zv8MxW8zE8Aw8zl8Ap8dM8Myp8zYx8dLh8Mta808dd8Mtr8dr8MtF8da8Mxg8dk8Mxb8df8MtA8dw8Mgz877wzzeq8m0i87Mzzga8m0i87m6kk8m0i8zzgI8m0i86oS8MIE8zztP8MIE8zMMZ8MIE8zoBI87M6oO8MIE87V
	// These are the item's component IDs, keyed by M2 texture type.
	return map[int]int{2: 4528821, 3: 4298889, 4: 4298893, 24: 4382185}
}

func convertTrollShoulder(ctx context.Context, t *testing.T, src cascSource, outputDir string, animate *bool, replaceableTextures map[int]int) ConvertResult {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.ExportAssetDir = filepath.Join(outputDir, "sources")
	cfg.TextureBaking = config.TextureBakingOptions{Enabled: true, Animate: animate, FPS: 15, WindowMS: 4000}
	result, err := ConvertM2ToMdl(ctx, cfg, src, ConvertOptions{
		FileDataID:          trollShoulderModelFileDataID,
		ReplaceableTextures: replaceableTextures,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func inspectShoulderBake(t *testing.T, model *mdl.MDL, wantAnimated bool) (atlasLayers, globalLayers int) {
	t.Helper()
	for _, geoset := range model.Geosets {
		if geoset == nil || geoset.Material == nil {
			continue
		}
		for _, layer := range geoset.Material.Layers {
			if layer.Texture == nil || !strings.Contains(strings.ReplaceAll(layer.Texture.WowData.PngPath, "\\", "/"), "baked/uv2/") {
				continue
			}
			atlasLayers++
			if layer.TVertexAnim == nil {
				continue
			}
			globalLayers++
			track := layer.TVertexAnim.Translation
			if track == nil || track.GlobalSeq == nil || len(track.KeyFrames) < 3 || track.GlobalSeq.Duration <= 0 {
				t.Fatalf("baked flipbook has no valid global translation track: %+v", track)
			}
		}
	}
	if atlasLayers == 0 {
		t.Fatal("conversion did not produce any UV2 shoulder atlas layers")
	}
	if wantAnimated && globalLayers == 0 {
		t.Fatal("animated conversion lost the source global UV animation")
	}
	if !wantAnimated && globalLayers != 0 {
		t.Fatalf("static conversion retained %d baked texture animations", globalLayers)
	}

	model.Modify.RemoveUnusedMaterialsTextures()
	model.Sync()
	serialized := mdx.NewModel()
	if err := serialized.LoadMdl(model.ToMdl()); err != nil {
		t.Fatalf("serialized shoulder MDL did not reload: %v", err)
	}
	for _, material := range serialized.Materials {
		for _, layer := range material.Layers {
			if layer.TextureAnimationID < 0 {
				continue
			}
			animation := serialized.TextureAnimations[layer.TextureAnimationID]
			if len(animation.Animations) == 0 || animation.Animations[0].GlobalSequenceID < 0 {
				t.Fatalf("serialized shoulder flipbook lost global sequence: %+v", animation)
			}
		}
	}
	return atlasLayers, globalLayers
}

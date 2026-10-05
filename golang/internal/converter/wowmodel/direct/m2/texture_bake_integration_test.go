//go:build integration_tests

package directm2

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/transport"
)

// Uses the existing data server, including the bundled Unix socket. Never loads
// a second CASC runtime. UV2_TEST_OUTPUT optionally keeps an inspectable export.
func TestFirehawkUV2Bake(t *testing.T) {
	if os.Getenv("WOW_DATA_SERVER_URL") == "" {
		transport.ConfigureBundled()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	src := cascSource{client: client.NewHTTPClient("")}
	// The bundled socket disappears briefly during development hot reload.
	readyCtx, stopWaiting := context.WithTimeout(ctx, 45*time.Second)
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
	cfg := config.DefaultConfig()
	cfg.TextureBaking.Enabled = true
	out := os.Getenv("UV2_TEST_OUTPUT")
	if out == "" {
		out = t.TempDir()
	}
	cfg.ExportAssetDir = filepath.Join(out, "sources")
	started := time.Now()
	result, err := ConvertM2ToMdl(ctx, cfg, src, ConvertOptions{FileDataID: 514089, VariantTextures: []int{512368, 512371, 512374}})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("conversion %s", time.Since(started))
	m := result.MDL
	baked, animated := 0, 0
	effectTextures := map[string]bool{}
	for _, g := range m.Geosets {
		if g.Material == nil || len(g.Material.Layers) != 1 {
			continue
		}
		layer := g.Material.Layers[0]
		if !strings.Contains(layer.Texture.Image, "baked/uv2/") {
			continue
		}
		// The general shader baker also converts the two opaque body batches.
		// Only the source effect-card sections have the following draw flags.
		if g.Name == "Geoset0" || g.Name == "Geoset4" || strings.HasSuffix(g.Name, "_Emission") || strings.HasSuffix(g.Name, "_EnvironmentFactor") {
			continue
		}
		baked++
		effectTextures[layer.Texture.WowData.PngPath] = true
		if !layer.Unshaded || !layer.TwoSided || !layer.NoDepthSet {
			t.Fatalf("effect card %s lost material flags: %+v", g.Name, layer)
		}
		if layer.TVertexAnim != nil {
			animated++
			if layer.TVertexAnim.Translation == nil || len(layer.TVertexAnim.Translation.KeyFrames) < 3 {
				t.Fatalf("effect card %s has no flipbook", g.Name)
			}
		}
		for _, v := range g.Vertices {
			if v.TexPosition2 != nil {
				t.Fatal("baked card still requires UV2")
			}
			if v.TexPosition[0] < 0 || v.TexPosition[1] < 0 || v.TexPosition[0] >= 1 || v.TexPosition[1] >= 1 {
				t.Fatalf("invalid atlas UV: %v", v.TexPosition)
			}
		}
	}
	if baked != 9 || animated != 8 {
		t.Fatalf("baked %d effect sections (%d animated), want 9 (8 animated)", baked, animated)
	}
	m.Modify.ConvertToSd800()
	m.Modify.RemoveUnusedMaterialsTextures()
	m.Sync()
	var textureBytes int64
	for _, texture := range m.Textures {
		rel := texture.WowData.PngPath
		source, ok := texturesource.Get(rel)
		if !ok {
			continue
		}
		input := blp.EncodeInput{PNG: source.PNG, PreserveAlpha: source.PreserveAlpha}
		if source.Kind == texturesource.KindBLP {
			input.BLP2, err = src.GetRawFile(ctx, source.FileDataID)
			if err != nil {
				t.Fatal(err)
			}
		}
		if source.Kind == texturesource.KindPNG {
			img, err := png.Decode(bytes.NewReader(source.PNG))
			if err != nil {
				t.Fatal(err)
			}
			transparent, soft, visible := false, false, false
			for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
				for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
					_, _, _, a := img.At(x, y).RGBA()
					transparent = transparent || a == 0
					soft = soft || (a > 0 && a < 65535)
					visible = visible || a > 0
				}
			}
			if effectTextures[rel] && (!transparent || !soft || !visible) {
				t.Fatalf("atlas %s lacks visible, soft-alpha, or transparent pixels", rel)
			}
			pngPath := filepath.Join(out, cfg.AssetPrefix, rel)
			if err = os.MkdirAll(filepath.Dir(pngPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(pngPath, source.PNG, 0644); err != nil {
				t.Fatal(err)
			}
		}
		blpPath := filepath.Join(out, cfg.AssetPrefix, strings.TrimSuffix(rel, ".png")+".blp")
		if err = blp.ConvertTextureToBlp(input, blpPath); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(blpPath)
		if err != nil {
			t.Fatal(err)
		}
		textureBytes += info.Size()
	}
	mdlPath := filepath.Join(out, "firehawk-uv2.mdl")
	if err = os.WriteFile(mdlPath, []byte(m.ToMdl()), 0644); err != nil {
		t.Fatal(err)
	}
	serialized := mdx.NewModel()
	if err := serialized.LoadMdl(m.ToMdl()); err != nil {
		t.Fatal(err)
	}
	hasPriority := false
	for _, material := range serialized.Materials {
		hasPriority = hasPriority || material.PriorityPlane > 0
		for _, layer := range material.Layers {
			if !strings.Contains(serialized.Textures[layer.TextureID].Path, "baked/uv2/") || layer.TextureAnimationID < 0 {
				continue
			}
			tracks := serialized.TextureAnimations[layer.TextureAnimationID].Animations
			if len(tracks) != 1 || tracks[0].Name != "KTAT" || tracks[0].InterpolationType != mdx.InterpolationDontInterp || tracks[0].GlobalSequenceID < 0 {
				t.Fatalf("serialized flipbook is invalid: %+v", tracks)
			}
			track := tracks[0]
			period := serialized.GlobalSequences[track.GlobalSequenceID]
			if period < 1100 || period > 1567 || len(track.Frames) < 3 || len(track.Frames) > bakeFrameCount(int(period))+1 || track.Frames[len(track.Frames)-1] != int32(period) {
				t.Fatalf("serialized Firehawk loop: period %d frames %v", period, track.Frames)
			}
		}
	}
	if !hasPriority {
		t.Fatal("serialized effect materials lost their sorting priorities")
	}
	if textureBytes > 32*1024*1024 {
		t.Fatalf("Firehawk textures exceed map budget: %d bytes", textureBytes)
	}
	mdxBytes, err := m.ToMdx()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(out, "firehawk-uv2.mdx"), mdxBytes, 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %d baked sections, %d texture bytes, %d MDX bytes", mdlPath, baked, textureBytes, len(mdxBytes))
}

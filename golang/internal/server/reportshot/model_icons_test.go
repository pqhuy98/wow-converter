package reportshot

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
)

func iconModelFixture(t *testing.T, root string) {
	t.Helper()
	raw, err := mdx.ConvertMdlToMdx(`Version { FormatVersion 1000, }
 Model "icon" { BlendTime 150, }
 Sequences 1 { Anim "Stand 1" { Interval { 0, 1000 }, } }
 Textures 1 { Bitmap { Image "skin.blp", } }
 Camera "Portrait_Camera" { Position { 10, 0, 3 }, FieldOfView 0.4712, FarClip 1000, NearClip 0.1, Target { Position { 0, 0, 2 }, } }
 `)
	if err != nil {
		t.Fatal(err)
	}
	os.Mkdir(filepath.Join(root, "npcs"), 0755)
	os.WriteFile(filepath.Join(root, "npcs", "wolf.mdx"), raw, 0644)
	os.WriteFile(filepath.Join(root, "skin.blp"), []byte("skin-a"), 0644)
}

func TestModelIconCacheCleanupKeepsCurrentAndUnrelatedIcons(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "npcs"), 0755)
	for _, model := range []string{"npcs/old.mdx", "npcs/current.mdx", "npcs/handmade.mdx"} {
		active, disabled := modelIconPaths(model)
		for _, path := range []string{active, disabled} {
			full := filepath.Join(root, filepath.FromSlash(path))
			os.MkdirAll(filepath.Dir(full), 0755)
			os.WriteFile(full, []byte("BLP1"), 0644)
		}
	}
	cache := map[string]string{"npcs/old.mdx": "old", "npcs/current.mdx": "current"}
	saveModelIconCache(root, cache, []string{"npcs/current.mdx"})
	active, _ := modelIconPaths("npcs/old.mdx")
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(active))); !os.IsNotExist(err) {
		t.Fatal("stale generated icon retained")
	}
	for _, model := range []string{"npcs/current.mdx", "npcs/handmade.mdx"} {
		active, _ := modelIconPaths(model)
		if !validIconBLP(root, active) {
			t.Fatal("current or unrelated icon deleted")
		}
	}
}

func TestModelIconCameraAndTextureCacheInvalidation(t *testing.T) {
	root := t.TempDir()
	iconModelFixture(t, root)
	job, err := prepareModelIcon(context.Background(), root, "npcs/wolf.mdx", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	distance := 10 * math.Tan(.4712/2) / math.Tan(math.Pi/8) / 1.7
	if math.Abs(job.camera.Eye[0]-distance) > .00001 || job.sequence != "Stand 1" {
		t.Fatalf("wrong portrait camera: %+v", job)
	}
	os.WriteFile(filepath.Join(root, "skin.blp"), []byte("skin-b"), 0644)
	changed, err := prepareModelIcon(context.Background(), root, "npcs/wolf.mdx", map[string]string{})
	if err != nil || changed.key == job.key {
		t.Fatal("texture update did not invalidate icon")
	}
	if _, err = prepareModelIcon(context.Background(), root, "../outside.mdx", map[string]string{}); err == nil {
		t.Fatal("unsafe path")
	}
	active, disabled := modelIconPaths("npcs/a-wolf_604.mdx")
	if active != "ReplaceableTextures/CommandButtons/BTN_a-wolf_604.blp" || disabled != "ReplaceableTextures/CommandButtonsDisabled/DISBTN_a-wolf_604.blp" {
		t.Fatal("icon not tied to model name")
	}
}

func TestModelIconEncodingCachingAndCancellation(t *testing.T) {
	root := t.TempDir()
	iconModelFixture(t, root)
	img := image.NewRGBA(image.Rect(0, 0, 384, 384))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{38, 38, 38, 255}), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(80, 60, 300, 320), image.NewUniform(color.RGBA{180, 150, 70, 255}), image.Point{}, draw.Src)
	var raw bytes.Buffer
	png.Encode(&raw, img)
	if err := writeModelIcon(context.Background(), root, "npcs/wolf.mdx", raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	active, disabled := modelIconPaths("npcs/wolf.mdx")
	if !validIconBLP(root, active) || !validIconBLP(root, disabled) {
		t.Fatal("BTN/DISBTN BLP missing")
	}
	job, err := prepareModelIcon(context.Background(), root, "npcs/wolf.mdx", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	cache := []byte(`{"npcs/wolf.mdx":"` + job.key + `"}`)
	os.WriteFile(filepath.Join(root, "npcs", "icons.json"), cache, 0644)
	t.Setenv("WOW_CONVERTER_URL", "http://127.0.0.1:1")
	icons, err := RenderModelIcons(context.Background(), root, []string{"npcs/wolf.mdx"}, false)
	if err != nil || icons["npcs/wolf.mdx"] != active {
		t.Fatalf("cached icon should not launch browser: %v", err)
	}
	_, err = RenderModelIcons(context.Background(), root, []string{"npcs/wolf.mdx"}, true)
	if err == nil {
		t.Fatal("missing viewer did not report optional failure")
	}
	icons, err = RenderModelIcons(context.Background(), root, []string{"npcs/missing.mdx"}, false)
	if err != nil || len(icons) != 0 {
		t.Fatal("missing icon model blocked batch")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = RenderModelIcons(ctx, root, []string{"npcs/wolf.mdx"}, false); err == nil {
		t.Fatal("ignored cancellation")
	}
}

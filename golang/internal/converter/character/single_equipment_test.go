package character

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

func TestSingleEquipmentItem185188Metadata(t *testing.T) {
	wowClient := client.NewHTTPClient(os.Getenv("WOW_DATA_SERVER_URL"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := wowClient.WaitUntilReady(ctx); err != nil {
		t.Skip("wow-data-server not ready:", err)
	}
	http := wowhead.NewHTTPClient()
	hash := "fa80o0zN89c8zZ8jY8z18nw8zZ8jY8z28nj8z38n18z48yo8aM8z5P8Mz8yt8MM8yC8sW8z3g8zYv8dLv8Mtr808og8yh8M08yL877g3MZ8Maz87r"
	meta, err := wowhead.DecodeDressingRoom(http, wowhead.ExpansionLive, hash)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("equipment: %#v", meta.Equipment)
	cfg := config.DefaultConfig()
	exp := NewCharacterExporter(cfg, wowClient)
	exportCtx := exp.newExportContext("frost-single", Character{
		Base: Ref{Type: "wowhead", Value: "https://www.wowhead.com/dressing-room?frost-prince#" + hash},
	}, "")
	for slotStr, displayID := range meta.Equipment {
		slot := 0
		fmt.Sscanf(slotStr, "%d", &slot)
		zam := wowhead.ZamURL{Expansion: wowhead.ExpansionLive, Type: wowhead.ZamTypeItem, DisplayID: displayID, SlotID: &slot}
		item, err := ProcessItemData(exportCtx.WowheadHTTP(), wowhead.ExpansionLive, zam, 10, 0, 6)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("slot %s display %d models=%v tex0=%v", slotStr, displayID, item.ModelFiles, item.ModelTextureFiles[0])
		model, err := ExportModelFileIDAsMdl(&exportCtx, item.ModelFiles[0].FileDataID, ExportModelOptions{
			TextureIDs: textureIDs(item.ModelTextureFiles[0]),
		})
		if err != nil {
			t.Fatal(err)
		}
		replaceable := itemReplaceableTextures(item.ModelTextureFiles)
		t.Logf("replaceable: %#v", replaceable)
		if len(replaceable) == 0 {
			t.Fatal("expected replaceable textures from Textures2")
		}
		if err := ApplyReplaceableTextures(&exportCtx, model.MDL, replaceable); err != nil {
			t.Fatal(err)
		}
		for i, tex := range model.MDL.Textures {
			t.Logf("  tex[%d] type=%d image=%q png=%q", i, tex.WowData.Type, tex.Image, tex.WowData.PngPath)
		}
		for _, g := range model.MDL.Geosets {
			if g == nil || g.Material == nil {
				continue
			}
			for li, layer := range g.Material.Layers {
				if layer.Texture == nil {
					continue
				}
				if layer.Texture.Image == "" {
					t.Errorf("geoset %q layer %d empty type=%d", g.Name, li, layer.Texture.WowData.Type)
				}
			}
		}
	}
}

func textureIDs(files []FileWithComponent) []int {
	out := make([]int, 0, len(files))
	for _, f := range files {
		out = append(out, f.FileDataID)
	}
	return out
}

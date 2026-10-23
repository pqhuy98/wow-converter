package preview

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

type wmoProgressClient struct {
	client.Client
	rootReads, groupReads int
}

func (c *wmoProgressClient) GetFileByID(_ context.Context, id int) (casc.ListfileEntry, error) {
	return casc.ListfileEntry{FileDataID: id, FileName: fmt.Sprintf("model-%d.wmo", id)}, nil
}

func (c *wmoProgressClient) DownloadCascFile(_ context.Context, id int) ([]byte, error) {
	groups := map[int]int{101: 2, 202: 3, 303: 1}[id]
	if groups == 0 {
		c.groupReads++
		return []byte{}, nil // Empty display group; geometry is irrelevant to progress.
	}
	c.rootReads++
	raw := make([]byte, 72+8+groups*4)
	binary.LittleEndian.PutUint32(raw, 0x4d4f4844) // MOHD
	binary.LittleEndian.PutUint32(raw[4:], 64)
	binary.LittleEndian.PutUint32(raw[12:], uint32(groups))
	binary.LittleEndian.PutUint32(raw[72:], 0x47464944) // GFID
	binary.LittleEndian.PutUint32(raw[76:], uint32(groups*4))
	for i := 0; i < groups; i++ {
		binary.LittleEndian.PutUint32(raw[80+i*4:], uint32(id*10+i))
	}
	return raw, nil
}

func TestWMOGroupProgressUsesOneTotalAcrossAssetsAndCacheHits(t *testing.T) {
	for _, cancelDuringGroup := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%t", cancelDuringGroup), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := &wmoProgressClient{}
			var groups []PreviewProgress
			opts := PreviewOptions{Client: c, CacheDir: t.TempDir(), MapDir: "azeroth", Tiles: []settings.TileCoord{{X: 0, Y: 0}}}
			opts.Progress = func(p PreviewProgress) {
				if p.Message == "Preparing WMO groups" {
					groups = append(groups, p)
					if cancelDuringGroup && p.Completed == 1 {
						cancel()
					}
				}
			}
			b := previewBuilder{ctx: ctx, options: opts, prefix: MapPreviewCacheDirectory + "/" + previewHash("")}
			stem := b.prefix + "/adt-bounds-v2-" + previewHash("azeroth") + "-0-0"
			tile := previewTile{Min: imath.Vector3{}, Max: imath.Vector3{100, 100, 100}, Chunks: []previewChunk{{Heights: make([]float32, 145)}}, Asset: PreviewAsset{ID: stem, Parts: []PreviewPart{{Mesh: stem + ".bin", Vertices: 3}}}}
			for i, id := range []int{101, 202, 101, 303} {
				tile.Placements = append(tile.Placements, previewPlacement{ID: id, UID: uint32(i + 1), Scale: 1})
			}
			writeJSON := func(path string, value any) {
				t.Helper()
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err := b.write(path, raw); err != nil {
					t.Fatal(err)
				}
			}
			if err := b.write(stem+".bin", make([]byte, 96)); err != nil {
				t.Fatal(err)
			}
			writeJSON(stem+".json", tile)
			cached := previewWMO{Asset: PreviewAsset{ID: b.prefix + "/wmo-303"}, FloorFile: b.prefix + "/wmo-303-floors.bin"}
			if err := b.write(cached.FloorFile, []byte{}); err != nil {
				t.Fatal(err)
			}
			writeJSON(cached.Asset.ID+".json", cached)
			_, err := BuildMapPreview(ctx, opts)
			if cancelDuringGroup {
				if !errors.Is(err, context.Canceled) || c.groupReads != 1 {
					t.Fatalf("group cancellation did not stop work: %v, downloads=%d", err, c.groupReads)
				}
			} else if err != nil || c.groupReads != 5 {
				t.Fatalf("cached/repeated WMO was rebuilt: %v, downloads=%d", err, c.groupReads)
			}
			previous := 0
			for _, p := range groups {
				if p.Total != 6 || p.Completed < previous || p.Completed > 6 {
					t.Fatalf("progress reset or changed totals: %+v", groups)
				}
				previous = p.Completed
			}
			if len(groups) == 0 || (!cancelDuringGroup && previous != 6) || c.rootReads != 3 {
				t.Fatalf("incomplete progress or duplicate root reads: %+v, roots=%d", groups, c.rootReads)
			}
		})
	}
}

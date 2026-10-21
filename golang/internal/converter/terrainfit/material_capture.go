package terrainfit

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	bake "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	adtfmt "github.com/pqhuy98/wow-converter/internal/wow/formats/adt"
)

// CaptureMaterials snapshots original, unbaked material textures and authored
// layer weights. Baked reference fragments are never used as material exemplars.
func CaptureMaterials(ctx context.Context, source casc.Source, root string, region Region, x, y int) error {
	dir := filepath.Join(root, fmt.Sprintf("%s-%d_%d", region.Map, x, y))
	path := filepath.Join(dir, "layers.json")
	if data, err := os.ReadFile(path); err == nil {
		var cached materialCapture
		if json.Unmarshal(data, &cached) == nil && cached.Version == 2 && cached.Build == source.GetBuildName() && len(cached.Chunks) == 256 {
			fmt.Println("CACHED MATERIALS", filepath.Base(dir), len(cached.Materials))
			return nil
		}
	}
	load := func(id uint32, name string) ([]byte, error) {
		if id == 0 {
			found, ok := archivecasc.GetByFilename(name)
			if !ok {
				return nil, fmt.Errorf("missing terrain file: %s", name)
			}
			id = uint32(found)
		}
		return source.GetFile(ctx, int(id))
	}
	prefix := fmt.Sprintf("world/maps/%s/%s", region.Map, region.Map)
	data, err := load(0, prefix+".wdt")
	if err != nil {
		return err
	}
	wdt := adtfmt.NewWDTLoader(buffer.From(data))
	wdt.Load()
	index := x*64 + y
	if index >= len(wdt.Entries) {
		return fmt.Errorf("missing WDT entry: %s %d_%d", region.Map, x, y)
	}
	entry := wdt.Entries[index]
	tilePrefix := fmt.Sprintf("%s_%d_%d", prefix, x, y)
	data, err = load(entry.RootADT, tilePrefix+".adt")
	if err != nil {
		return err
	}
	adt := adtfmt.NewADTLoader(buffer.From(data))
	adt.LoadRoot()
	data, err = load(entry.Tex0ADT, tilePrefix+"_tex0.adt")
	if err != nil {
		return err
	}
	tex := adtfmt.NewADTLoader(buffer.From(data))
	tex.LoadTex(wdt)
	if len(adt.Chunks) != 256 || len(tex.TexChunks) != 256 {
		return fmt.Errorf("incomplete material chunks: %s", tilePrefix)
	}
	capture := materialCapture{Version: 2, Build: source.GetBuildName(), HeightTexturing: wdt.Flags&0x80 != 0}
	materialsDir := filepath.Join(root, "materials", capture.Build)
	if err = os.MkdirAll(materialsDir, 0755); err != nil {
		return err
	}
	saveTexture := func(id uint32) error {
		if id == 0 {
			return nil
		}
		file := filepath.Join(materialsDir, fmt.Sprintf("%d.png", id))
		if info, e := os.Stat(file); e == nil && info.Size() > 0 {
			return nil
		}
		texture, e := bake.LoadBakeTexture(ctx, func(ctx context.Context, id uint32) ([]byte, error) { return source.GetFile(ctx, int(id)) }, id)
		if e != nil {
			return e
		}
		base := texture.Mips[0]
		return savePNG(file, &image.NRGBA{Pix: base.Data, Stride: base.Width * 4, Rect: image.Rect(0, 0, base.Width, base.Height)})
	}
	for i, id := range tex.DiffuseTextureFileDataIDs {
		m := terrainMaterial{ID: id, Scale: 1, HeightOffset: 1}
		if i < len(tex.HeightTextureFileDataIDs) {
			m.HeightID = tex.HeightTextureFileDataIDs[i]
		}
		if i < len(tex.TexParams) {
			p := tex.TexParams[i]
			m.Scale = int(math.Pow(2, float64((p.Flags&0xf0)>>4)))
			m.HeightScale, m.HeightOffset = float64(p.Height), float64(p.Offset)
		}
		if err = saveTexture(m.ID); err != nil {
			return err
		}
		if err = saveTexture(m.HeightID); err != nil {
			return err
		}
		capture.Materials = append(capture.Materials, m)
	}
	scales := map[int]int{}
	for i, chunk := range tex.TexChunks {
		row := materialChunk{X: i % 16, Y: i / 16, Alpha: bake.FixChunkAlphaLayers(chunk.AlphaLayers, adt.Chunks[i].Flags&(1<<15) == 0)}
		for _, layer := range chunk.Layers {
			if int(layer.TextureID) >= len(capture.Materials) {
				return fmt.Errorf("invalid material index: %d", layer.TextureID)
			}
			row.Layers = append(row.Layers, int(layer.TextureID))
			scales[capture.Materials[layer.TextureID].Scale]++
		}
		for _, color := range adt.Chunks[i].VertexShading {
			row.Tint = append(row.Tint, rgb{float32(color.B) / 127.5, float32(color.G) / 127.5, float32(color.R) / 127.5})
		}
		capture.Chunks = append(capture.Chunks, row)
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err = writeJSON(path, capture); err != nil {
		return err
	}
	fmt.Println("MATERIALS", filepath.Base(dir), len(capture.Materials), "scales", scales)
	return nil
}

type materialCapture struct {
	Version         int               `json:"version"`
	Build           string            `json:"build"`
	HeightTexturing bool              `json:"heightTexturing"`
	Materials       []terrainMaterial `json:"materials"`
	Chunks          []materialChunk   `json:"chunks"`
}

type terrainMaterial struct {
	ID, HeightID              uint32
	Scale                     int
	HeightScale, HeightOffset float64
}

type materialChunk struct {
	X, Y   int
	Layers []int
	Alpha  [][]byte
	Tint   []rgb
}

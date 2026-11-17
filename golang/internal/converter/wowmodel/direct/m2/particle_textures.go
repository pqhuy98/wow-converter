package directm2

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"strconv"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func orientM2ParticleHeads(ctx context.Context, cfg config.Config, result *ConvertResult, loader *m2.Loader, loadTexture func(int) (*image.NRGBA, error)) error {
	type atlasKey struct {
		texture       *components.Texture
		columns, rows int
	}
	converted := map[atlasKey]*components.Texture{}
	for _, node := range result.MDL.ParticleEmitter2s {
		if node.HeadOrTail == components.HeadOrTailTail || node.Texture == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		key := atlasKey{node.Texture, max(1, node.Columns), max(1, node.Rows)}
		texture := converted[key]
		if texture == nil {
			var source image.Image
			if registered, ok := texturesource.Get(node.Texture.WowData.PngPath); ok && registered.Kind == texturesource.KindPNG {
				decoded, err := png.Decode(bytes.NewReader(registered.PNG))
				if err != nil {
					return err
				}
				source = decoded
			} else {
				indexText, _, _ := strings.Cut(strings.TrimPrefix(node.Name, "ParticleEmitter_"), "_")
				index, err := strconv.Atoi(indexText)
				if err != nil || index < 0 || index >= len(loader.ParticleEmitters) {
					return fmt.Errorf("particle texture: invalid emitter %q", node.Name)
				}
				p := loader.ParticleEmitters[index]
				textureIndex := int(p.TexturePacked)
				if p.Flags&0x10000000 != 0 {
					textureIndex &= 31
				}
				decoded, err := loadTexture(textureIndex)
				if err != nil {
					return fmt.Errorf("particle %d texture orientation: %w", index, err)
				}
				source = decoded
			}
			atlas, err := transposeParticleCells(source, key.columns, key.rows)
			if err != nil {
				return err
			}
			// Preserve the atlas resolution; Advanced Textures already applied its scale.
			textureConfig := cfg
			textureConfig.TextureBaking.ResolutionScale = 1
			texture, err = registerBakeTexture(textureConfig, result, atlas)
			if err != nil {
				return err
			}
			converted[key] = texture
		}
		if node.HeadOrTail == components.HeadOrTailBoth {
			tail := mdl.CloneParticleEmitter2(node)
			tail.Name += "_tail"
			tail.HeadOrTail = components.HeadOrTailTail
			result.MDL.ParticleEmitter2s = append(result.MDL.ParticleEmitter2s, tail)
			node.HeadOrTail = components.HeadOrTailHead
		}
		node.Texture = texture
	}
	return nil
}

func transposeParticleCells(source image.Image, columns, rows int) (*image.NRGBA, error) {
	bound := source.Bounds()
	// An unbound replaceable sampler is constant across every atlas cell.
	if bound.Dx() == 1 && bound.Dy() == 1 && columns > 0 && rows > 0 {
		columns, rows = 1, 1
	}
	if columns < 1 || rows < 1 || bound.Dx()%columns != 0 || bound.Dy()%rows != 0 || bound.Empty() {
		return nil, fmt.Errorf("particle atlas %dx%d has invalid grid %dx%d", bound.Dx(), bound.Dy(), columns, rows)
	}
	width, height := bound.Dx()/columns, bound.Dy()/rows
	atlas := image.NewNRGBA(image.Rect(0, 0, height*columns, width*rows))
	// WoW maps (u,v) to (right,down). PRE2 head corners map it to (down,right).
	// Swap the axes within each cell, retaining its index and all UV intervals.
	for row := range rows {
		for column := range columns {
			for y := range width {
				for x := range height {
					atlas.Set(column*height+x, row*width+y, source.At(bound.Min.X+column*width+y, bound.Min.Y+row*height+x))
				}
			}
		}
	}
	return atlas, nil
}

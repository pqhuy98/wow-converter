package directm2

import (
	"context"
	"fmt"
	"image"
	"log"
	"math"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func particleTrackAt(track m2.PartTrack, age float64, fallback []float64) []float64 {
	if len(track.Values) == 0 || len(track.Timestamps) == 0 {
		return fallback
	}
	t := age * 32767
	for i, stamp := range track.Timestamps {
		if float64(stamp) >= t && i < len(track.Values) {
			if i == 0 || stamp == track.Timestamps[i-1] {
				return track.Values[i]
			}
			a, b := track.Values[i-1], track.Values[i]
			ratio := (t - float64(track.Timestamps[i-1])) / float64(stamp-track.Timestamps[i-1])
			out := make([]float64, min(len(a), len(b)))
			for k := range out {
				out[k] = a[k] + (b[k]-a[k])*ratio
			}
			return out
		}
	}
	return track.Values[len(track.Values)-1]
}

func particleBakeRGBIndependent(mode components.ParticleFilterMode) bool {
	return mode == components.PFilterModulate || mode == components.PFilterModulate2x
}

// PRE2 has one texture and square sprites. Bake the full three-texture shader,
// both scroll rates and the source's anisotropic sprite shape over particle age.
// A few deterministic phase variants approximate WoW's random per-particle UV
// seeds without synchronizing every particle to one pattern.
func bakeM2Particles(ctx context.Context, cfg config.Config, result *ConvertResult, loader *m2.Loader, loadTexture func(int) (*image.NRGBA, error)) error {
	byName := map[string]*components.ParticleEmitter2{}
	for _, node := range result.MDL.ParticleEmitter2s {
		byName[node.Name] = node
	}
	for index, p := range loader.ParticleEmitters {
		node := byName[fmt.Sprintf("ParticleEmitter_%d", index)]
		if node == nil {
			continue
		}
		original := *node
		multi := p.Flags&0x10000000 != 0
		maxScale := 0.0
		anisotropic := false
		for _, v := range p.ScaleTrack.Values {
			if len(v) >= 2 {
				maxScale = max(maxScale, v[0], v[1])
				anisotropic = anisotropic || math.Abs(v[0]-v[1]) > max(v[0], v[1])*.05
			}
		}
		if !multi && !anisotropic {
			continue
		}
		count, variants := 1, 1
		if multi {
			count, variants = 3, 2
		}
		var images [3]*image.NRGBA
		var flags [3]uint32
		for k := range count {
			texIndex := int(p.TexturePacked)
			if multi {
				texIndex = int(p.TexturePacked >> (5 * k) & 31)
			}
			img, err := loadTexture(texIndex)
			if err != nil {
				return fmt.Errorf("particle %d texture %d: %w", index, texIndex, err)
			}
			images[k] = img
			flags[k] = loader.Textures[texIndex].Flags
		}
		if maxScale <= 0 {
			maxScale = 1
		}
		factor := float64(p.TwinkleScale.Min+p.TwinkleScale.Max) / 2
		if factor <= 0 {
			factor = 1
		}
		frames := bakeFrameCount(int(node.LifeSpan * 1000))
		referenceFrames := frames
		if !cfg.TextureBaking.Flipbook() {
			frames = 1
		} else if cfg.TextureBaking.FPS > 0 || cfg.TextureBaking.WindowMS > 0 {
			fps := bakeFPS
			if cfg.TextureBaking.FPS > 0 {
				fps = cfg.TextureBaking.FPS
			}
			// PRE2 advances its atlas over the full particle lifetime. Cropping
			// that lifetime would lower the requested FPS or age particles early.
			frames = max(2, int(math.Ceil(node.LifeSpan*float64(fps))))
		}
		cols := bakePowerOfTwo(int(math.Ceil(math.Sqrt(float64(frames)))))
		rows := bakePowerOfTwo((frames + cols - 1) / cols)
		tile := 64
		budget := bakeParticlePixels / max(1, len(loader.ParticleEmitters))
		referenceCols := bakePowerOfTwo(int(math.Ceil(math.Sqrt(float64(referenceFrames)))))
		referenceRows := (referenceFrames + referenceCols - 1) / referenceCols
		for tile > 16 && tile*tile*referenceCols*referenceRows*variants > budget {
			tile /= 2
		}
		for variant := range variants {
			if err := ctx.Err(); err != nil {
				return err
			}
			atlas := image.NewNRGBA(image.Rect(0, 0, tile*cols, tile*rows))
			for frame := range frames {
				age := 0.0
				if frames > 1 {
					age = float64(frame) / float64(frames-1)
				}
				life := age * node.LifeSpan
				scale := particleTrackAt(p.ScaleTrack, age, []float64{1, 1})
				sx, sy := scale[0]/maxScale, scale[1]/maxScale
				cell := int(math.Floor(particleTrackAt(p.HeadCellTrack, age, []float64{0})[0]))
				for y := range tile {
					for x := range tile {
						uv := imath.Vector2{(float64(x) + .5) / float64(tile), (float64(y) + .5) / float64(tile)}
						if sx <= 1e-9 || sy <= 1e-9 {
							continue
						}
						corner := imath.Vector2{(uv[0]-.5)/sx + .5, (uv[1]-.5)/sy + .5}
						if corner[0] < 0 || corner[0] > 1 || corner[1] < 0 || corner[1] > 1 {
							continue
						}
						base := imath.Vector2{(float64(cell%max(1, int(p.TextureCols))) + corner[0]) / float64(max(1, int(p.TextureCols))), (float64(cell/max(1, int(p.TextureCols))) + corner[1]) / float64(max(1, int(p.TextureRows)))}
						samples := [3][4]float64{sampleBakeTexture(images[0], base, flags[0])}
						for k := 1; k < count; k++ {
							layer := k - 1
							coord := imath.Vector2{}
							for axis := range 2 {
								seed := math.Mod(float64((variant+1)*(layer+2)*(axis+3))*.61803398875, 1)
								rate := float64(p.MultiTextureScrollMid[layer][axis]) + (.5-float64(variant)/float64(max(1, variants-1)))*float64(p.MultiTextureScrollRange[layer][axis])
								coord[axis] = seed + corner[axis]*float64(p.MultiTextureScale[layer]) + life*rate
							}
							samples[k] = sampleBakeTexture(images[k], coord, flags[k]|3)
						}
						color := samples[0]
						if multi {
							color = combineParticleSamples(samples, p.Flags)
						}
						at := ((frame/cols)*tile+y)*atlas.Stride + ((frame%cols)*tile+x)*4
						for channel := range 4 {
							atlas.Pix[at+channel] = uint8(math.Round(clampM2(color[channel]) * 255))
						}
					}
				}
			}
			tex, err := registerBakeTexture(cfg, result, atlas, bakeTextureOptions{independentRGB: particleBakeRGBIndependent(original.FilterMode)})
			if err != nil {
				return err
			}
			baked := original
			baked.Texture = tex
			baked.Columns, baked.Rows = cols, rows
			// Assembly already converted the native emitters into WC3 units.
			// ScaleTrack is still in M2 units; use the same unit conversion when
			// replacing their size, otherwise the baked gas becomes 56x smaller.
			size := maxScale * factor * cfg.RawModelScaleUp
			baked.SegmentScaling = [3]float64{size, size, size}
			middle := math.Round(clampM2(node.TimeMiddle) * float64(frames-1))
			baked.HeadIntervals = [3]float64{0, middle, 1}
			baked.DecayIntervals = [3]float64{middle, float64(frames - 1), 1}
			baked.TailIntervals, baked.TailDecayIntervals = baked.HeadIntervals, baked.DecayIntervals
			baked.EmissionRate = scaleBakeRate(original.EmissionRate, 1/float64(variants))
			if multi {
				baked.Flags2 = append(append([]components.ParticleEmitter2Flag(nil), node.Flags2...), components.PE2Unshaded)
			}
			if variant == 0 {
				*node = baked
			} else {
				baked.Name = fmt.Sprintf("%s_Phase%d", node.Name, variant)
				result.MDL.ParticleEmitter2s = append(result.MDL.ParticleEmitter2s, &baked)
			}
		}
		log.Printf("M2 particle bake %d: %d textures, %d age frames, %d phase variants", index, count, frames, variants)
	}
	return nil
}

func combineParticleSamples(t [3][4]float64, flags uint32) [4]float64 {
	color := [4]float64{}
	colorFactor, alphaFactor := 2.0, 2.0
	if flags&0x40000000 != 0 {
		colorFactor = 4
	}
	if flags&0x20000000 != 0 {
		alphaFactor = 4
	}
	for k := range 3 {
		color[k] = t[0][k] * t[1][k] * colorFactor
		if flags&0x40000000 != 0 {
			color[k] *= t[2][k]
		}
	}
	color[3] = t[0][3] * t[1][3] * t[2][3] * alphaFactor
	return color
}

func scaleBakeRate(rate components.AnimatedOrStatic[float64], scale float64) components.AnimatedOrStatic[float64] {
	if rate.Static {
		rate.Value *= scale
	} else if rate.Anim != nil {
		anim := *rate.Anim
		anim.KeyFrames = map[int]any{}
		for k, v := range rate.Anim.KeyFrames {
			if n, ok := v.(float64); ok {
				anim.KeyFrames[k] = n * scale
			}
		}
		if rate.Anim.InOutTans != nil {
			anim.InOutTans = map[int]components.InOutTan{}
			for t, tan := range rate.Anim.InOutTans {
				tan.InTan[0] *= scale
				tan.OutTan[0] *= scale
				anim.InOutTans[t] = tan
			}
		}
		rate.Anim = &anim
	}
	return rate
}

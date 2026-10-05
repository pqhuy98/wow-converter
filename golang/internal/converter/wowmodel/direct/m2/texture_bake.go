package directm2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	bundlemeta "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/metadata"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// Classic maps need bounded imports. Long native loops repeat their first four
// seconds; spatial detail and temporal sampling share a per-model pixel budget.
const bakeFPS = 8
const bakeWindowMS = 4000
const bakeMaterialPixels = 6 * 1024 * 1024
const bakeParticlePixels = 2 * 1024 * 1024
const bakePadding = 3

var errUV2BakeUnsupported = errors.New("UV2 baking unavailable")

type bakePixel struct {
	uv1, uv2 imath.Vector2
	env      imath.Vector2
	set      bool
	covered  bool
	face     int
	bary     [3]float64
}

type bakeChart struct {
	pixels                          []bakePixel
	active                          []int
	x, y, minX, minY, width, height int
}

type bakeChartOptions struct {
	conflict func(bakePixel, bakePixel) bool
	sourceUV map[*components.GeosetVertex][2]imath.Vector2
}

// bakeUV2Materials evaluates the WoW fragment combiner before framebuffer
// blending. WC3 layers cannot multiply two independently addressed alpha values.
// Baking in UV space keeps the original mesh, skinning and camera independence.
func bakeUV2Materials(ctx context.Context, cfg config.Config, src FileSource, loader *m2.Loader, skin *m2.Skin, mask []m2export.GeosetMaskEntry, resolved ResolvedTextures, result *ConvertResult) error {
	return bakeM2Materials(ctx, cfg, src, loader, skin, mask, resolved, nil, result)
}

func bakeM2Materials(ctx context.Context, cfg config.Config, src FileSource, loader *m2.Loader, skin *m2.Skin, mask []m2export.GeosetMaskEntry, resolved ResolvedTextures, meta *bundlemeta.File, result *ConvertResult) error {
	geosets := map[int]*components.Geoset{}
	targets := map[int]*components.Geoset{}
	gi := 0
	for si := range skin.SubMeshes {
		if mask != nil && (si >= len(mask) || !mask[si].Checked) {
			continue
		}
		if gi < len(result.MDL.Geosets) {
			geosets[si] = result.MDL.Geosets[gi]
			targets[si] = result.MDL.Geosets[gi]
		}
		gi++
	}
	unitsPerSection := map[uint16]int{}
	bakedGlobals := map[*components.GlobalSequence]bool{}
	budgets := m2BakeBudgets(loader, skin, geosets)
	decoded := map[int]*image.NRGBA{}
	loadTexture := func(index int) (*image.NRGBA, error) {
		if img := decoded[index]; img != nil {
			return img, nil
		}
		if index < 0 || index >= len(loader.Textures) {
			return nil, fmt.Errorf("texture index %d out of range", index)
		}
		tex := loader.Textures[index]
		entry, found := resolved.ValidTextures[tex.FileDataID]
		if !found && tex.FileName != "" {
			entry, found = resolved.ValidTextures[tex.FileName]
		}
		if !found && index < len(loader.TextureTypes) {
			entry, found = resolved.ValidTextures[fmt.Sprintf("data-%d", loader.TextureTypes[index])]
		}
		var img *image.NRGBA
		if found {
			rel, _ := filepath.Rel(cfg.ExportAssetDir, entry.MatPath)
			if source, ok := texturesource.Get(rel); ok && source.Kind == texturesource.KindPNG {
				decodedPNG, err := png.Decode(bytes.NewReader(source.PNG))
				if err != nil {
					return nil, err
				}
				img = image.NewNRGBA(decodedPNG.Bounds())
				draw.Draw(img, img.Bounds(), decodedPNG, decodedPNG.Bounds().Min, draw.Src)
			}
		}
		if img == nil && tex.FileDataID > 0 {
			raw, err := src.GetRawFile(ctx, int(tex.FileDataID))
			if err != nil {
				return nil, err
			}
			blpImage, err := blp.NewBLPImage(buffer.From(raw))
			if err != nil {
				return nil, err
			}
			pixels, err := blpImage.ToUInt8Array(0, 15)
			if err != nil {
				return nil, err
			}
			img = image.NewNRGBA(image.Rect(0, 0, int(blpImage.Width), int(blpImage.Height)))
			copy(img.Pix, pixels)
		}
		if img == nil && found {
			raw, err := os.ReadFile(entry.MatPath)
			if err != nil {
				return nil, err
			}
			decodedPNG, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				return nil, err
			}
			img = image.NewNRGBA(decodedPNG.Bounds())
			draw.Draw(img, img.Bounds(), decodedPNG, decodedPNG.Bounds().Min, draw.Src)
		}
		if img == nil {
			return nil, fmt.Errorf("%w: deferred or unresolved M2 texture %d", errUV2BakeUnsupported, index)
		}
		decoded[index] = img
		return img, nil
	}
units:
	for unitIndex, unit := range skin.TextureUnits {
		original := geosets[int(unit.SkinSectionIndex)]
		if original == nil || len(original.Faces) == 0 || original.Material == nil || len(original.Material.Layers) == 0 || int(unit.MaterialIndex) >= len(loader.Materials) {
			continue
		}
		description, err := decodeM2Shader(unit.ShaderID, int(unit.TextureCount))
		if err != nil {
			return err
		}
		// Each source batch is a draw. Keep separate passes even when they share
		// a skin section: compositing their raw textures as one material is wrong.
		g := cloneBakeGeoset(original)
		material := loader.Materials[unit.MaterialIndex]
		layer := original.Material.Layers[0]
		layer.FilterMode = m2BakeBlend(material.BlendingMode)
		layer.Unshaded, layer.Unlit = material.Flags&1 != 0, material.Flags&1 != 0
		if material.BlendingMode == 5 || material.BlendingMode == 6 {
			layer.Unshaded = true
		}
		layer.Unfogged, layer.TwoSided = material.Flags&2 != 0, material.Flags&4 != 0
		layer.NoDepthSet = material.Flags&16 != 0
		layer.NoDepthTest = material.Flags&8 != 0
		layer.Alpha = components.AnimatedOrStatic[float64]{Static: true, Value: 1}
		if meta != nil && unit.Flags&0x40 == 0 {
			layer.Alpha = meta.TextureWeight(int(unit.TextureWeightComboIndex))
		}
		g.Material = &components.Material{PriorityPlane: int(unit.Priority), TwoSided: layer.TwoSided, Layers: []components.Layer{layer}}
		program := bakeProgram{shader: description, count: m2SamplerCounts[description.pixel], blend: material.BlendingMode, weights: [4]components.AnimatedOrStatic[float64]{}, pixelBudget: budgets[unitIndex]}
		for k := range 4 {
			program.weights[k] = components.AnimatedOrStatic[float64]{Static: true, Value: 1}
			if meta != nil {
				program.weights[k] = meta.TextureWeight(int(unit.TextureWeightComboIndex) + k)
			}
		}
		combo := int(unit.TextureComboIndex)
		if combo+program.count > len(loader.TextureCombos) {
			return fmt.Errorf("UV2 bake section %d: invalid texture combo", unit.SkinSectionIndex)
		}
		for sampler := range program.count {
			index := int(loader.TextureCombos[combo+sampler])
			img, err := loadTexture(index)
			if err != nil {
				if errors.Is(err, errUV2BakeUnsupported) {
					log.Printf("UV2 bake: preserving section %d: %v", unit.SkinSectionIndex, err)
					continue units
				}
				return fmt.Errorf("UV2 bake section %d texture %d: %w", unit.SkinSectionIndex, index, err)
			}
			program.images[sampler], program.flags[sampler] = img, loader.Textures[index].Flags
		}
		for matrix := range 2 {
			ti := int(unit.TextureTransformComboIndex) + matrix
			if ti < len(loader.TextureTransformsLookup) {
				ai := int(loader.TextureTransformsLookup[ti])
				if ai < len(loader.TextureTransforms) && ai < len(result.MDL.TextureAnims) {
					program.transforms[matrix] = &result.MDL.TextureAnims[ai]
				}
			}
		}
		if description.edge {
			log.Printf("M2 bake section %d: camera-dependent edge fade uses neutral opacity in Classic", unit.SkinSectionIndex)
		}
		bakeErr := error(nil)
		didBake := false
		var environmentFactor *components.Geoset
		var alphaAttenuation *components.Geoset
		var alphaMasks []nativeMaskDraw
		var nativeSourceGA components.GeosetAnim
		var nativeMasks []nativeMaskDraw
		var opaqueRemainder *components.Geoset
		var opaqueBase *components.Geoset
		if unit.ColorIndex == 65535 && layer.Alpha.Static && layer.Alpha.Value == 1 && !layer.NoDepthSet && !layer.NoDepthTest {
			opaqueRemainder, opaqueBase, environmentFactor = nativeOpaqueProduct(g, program)
		}
		if layer.NoDepthSet {
			// The opacity and radiance draws must not write depth between their
			// different mask subdivisions. Depth-writing batches use one bake.
			nativeMasks = nativeModulateMask(g, program)
		}
		if opaqueBase == nil && len(nativeMasks) == 0 && unit.ColorIndex == 65535 && layer.Alpha.Static && layer.Alpha.Value == 1 {
			opaqueRemainder, nativeMasks = nativeOpaqueMasks(g, program)
		}
		if opaqueBase != nil {
			if len(opaqueRemainder.Faces) > 0 {
				opaqueRemainder.Name += "_BakedCoverageEdges"
				if err := bakeM2Geoset(ctx, cfg, result, opaqueRemainder, program); err != nil {
					return err
				}
				didBake = true
			}
			g = opaqueBase
			texture, err := registerBakeTexture(cfg, result, program.images[1])
			if err != nil {
				return err
			}
			texture.WrapWidth, texture.WrapHeight = program.flags[1]&1 != 0, program.flags[1]&2 != 0
			g.Material.Layers[0].Texture, g.Material.Layers[0].TVertexAnim = texture, program.transforms[1]
			g.Material.Layers[0].FilterMode = components.BlendNone
			factorTexture, err := registerBakeTexture(cfg, result, nativeModulateColor(program.images[0]))
			if err != nil {
				return err
			}
			factorTexture.WrapWidth, factorTexture.WrapHeight = program.flags[0]&1 != 0, program.flags[0]&2 != 0
			factorLayer := layer
			factorLayer.Texture, factorLayer.TVertexAnim = factorTexture, program.transforms[0]
			factorLayer.FilterMode = components.BlendModulate
			factorLayer.Unshaded, factorLayer.NoDepthSet = true, true
			environmentFactor.Material = &components.Material{PriorityPlane: -129, Layers: []components.Layer{factorLayer}}
			result.MDL.Materials = append(result.MDL.Materials, g.Material, environmentFactor.Material)
			log.Printf("M2 native opaque product %s: %d original triangles, %d baked coverage triangles", g.Name, len(g.Faces), len(opaqueRemainder.Faces))
		} else if len(nativeMasks) > 0 {
			if opaqueRemainder != nil && len(opaqueRemainder.Faces) > 0 {
				opaqueRemainder.Name += "_BakedMaskEdges"
				if err := bakeM2Geoset(ctx, cfg, result, opaqueRemainder, program); err != nil {
					return err
				}
				didBake = true
			}
			if program.blend == 2 {
				alphaLayer := layer
				alphaLayer.FilterMode = components.BlendBlend
				alphaAttenuation = cloneBakeGeoset(g)
				alphaAttenuation.Material = &components.Material{PriorityPlane: g.Material.PriorityPlane, Layers: []components.Layer{alphaLayer}}
				alphaAttenuation.Name += "_MaskAlpha"
				if nativeDetailOpaque(g, program) {
					var alphaImage *image.NRGBA
					alphaAttenuation, alphaImage = nativeMaskAlpha(alphaAttenuation, program)
					texture, err := registerBakeTexture(cfg, result, alphaImage)
					if err != nil {
						return err
					}
					alphaAttenuation.Material.Layers[0].Texture, alphaAttenuation.Material.Layers[0].TVertexAnim = texture, nil
					texture.WrapWidth, texture.WrapHeight = program.flags[0]&1 != 0, program.flags[0]&2 != 0
					result.MDL.Materials = append(result.MDL.Materials, alphaAttenuation.Material)
				} else {
					alphaProgram := program
					alphaProgram.alphaOnly = true
					alphaMasks = nativeModulateMask(alphaAttenuation, alphaProgram)
					if len(alphaMasks) > 0 {
						alphaImage := image.NewNRGBA(program.images[1].Bounds())
						for i := 3; i < len(alphaImage.Pix); i += 4 {
							alphaImage.Pix[i] = program.images[1].Pix[i]
						}
						texture, err := registerBakeTexture(cfg, result, alphaImage)
						if err != nil {
							return err
						}
						texture.WrapWidth, texture.WrapHeight = program.flags[1]&1 != 0, program.flags[1]&2 != 0
						alphaLayer.Texture, alphaLayer.TVertexAnim = texture, program.transforms[1]
						alphaMaterial := &components.Material{PriorityPlane: g.Material.PriorityPlane, Layers: []components.Layer{alphaLayer}}
						for i, draw := range alphaMasks {
							draw.geoset.Material = alphaMaterial
							draw.geoset.Name = fmt.Sprintf("%s_MaskAlpha%d", g.Name, i)
						}
						result.MDL.Materials = append(result.MDL.Materials, alphaMaterial)
					} else if err := bakeM2Geoset(ctx, cfg, result, alphaAttenuation, alphaProgram); err != nil {
						return err
					} else {
						didBake = true
					}
				}
			}
			nativeImage := program.images[1]
			if program.blend == 1 {
				nativeImage = nativeCutoutAlpha(nativeImage)
			}
			texture, err := registerBakeTexture(cfg, result, nativeImage)
			if err != nil {
				return err
			}
			texture.WrapWidth, texture.WrapHeight = program.flags[1]&1 != 0, program.flags[1]&2 != 0
			g = nativeMasks[0].geoset
			g.Material.Layers[0].Texture = texture
			g.Material.Layers[0].TVertexAnim = program.transforms[1]
			if alphaAttenuation != nil {
				g.Material.Layers[0].FilterMode = components.BlendAddAlpha
			}
			result.MDL.Materials = append(result.MDL.Materials, g.Material)
			log.Printf("M2 native detail %s: %d mask groups, full-resolution %dx%d texture", g.Name, len(nativeMasks), program.images[1].Bounds().Dx(), program.images[1].Bounds().Dy())
		} else if description.pixel == 12 && material.BlendingMode == 0 {
			// Opaque_EnvMetal factorizes exactly as diffuse * reflectionFactor.
			// Keep the full-resolution source diffuse; only the smoother factor
			// needs a chart atlas and Classic's framebuffer Modulate2x pass.
			environmentFactor = cloneBakeGeoset(g)
			environmentFactor.Name += "_EnvironmentFactor"
			factorLayer := layer
			factorLayer.FilterMode = components.BlendModulate2x
			factorLayer.Unshaded, factorLayer.NoDepthSet = true, true
			factorLayer.Alpha = components.AnimatedOrStatic[float64]{Static: true, Value: 1}
			// Finish opaque modulation before any alpha/additive effect draws.
			// Otherwise the factor also darkens effects rendered over the surface.
			environmentFactor.Material = &components.Material{PriorityPlane: -129, Layers: []components.Layer{factorLayer}}
			program.environmentFactor, program.blend = true, 6
			didBake = true
			bakeErr = bakeM2Geoset(ctx, cfg, result, environmentFactor, program)
			g.Material.Layers[0].TVertexAnim = program.transforms[0]
			for _, v := range g.Vertices {
				v.TexPosition2 = nil
			}
			result.MDL.Materials = append(result.MDL.Materials, g.Material)
		} else if unitsPerSection[unit.SkinSectionIndex] == 0 && program.count == 1 && !description.edge && description.coords[0] != coordEnv && material.BlendingMode != 3 && material.BlendingMode != 6 && material.BlendingMode != 7 && (description.pixel == 1 || description.pixel == 0 && material.BlendingMode == 0) {
			coord := description.coords[0]
			matrix := 0
			if coord == coordT1M1 || coord == coordT2M1 {
				matrix = 1
			}
			g.Material.Layers[0].TVertexAnim = program.transforms[matrix]
			if coord == coordT2M0 || coord == coordT2M1 || coord == coordT2 {
				for _, v := range g.Vertices {
					if v.TexPosition2 != nil {
						v.TexPosition = *v.TexPosition2
					}
				}
			}
			for _, v := range g.Vertices {
				v.TexPosition2 = nil
			}
			result.MDL.Materials = append(result.MDL.Materials, g.Material)
		} else {
			didBake = true
			bakeErr = bakeM2Geoset(ctx, cfg, result, g, program)
		}
		if err := bakeErr; err != nil {
			if errors.Is(err, errUV2BakeUnsupported) {
				log.Printf("UV2 bake: preserving section %d: %v", unit.SkinSectionIndex, err)
				continue
			}
			return fmt.Errorf("UV2 bake section %d: %w", unit.SkinSectionIndex, err)
		}
		if didBake {
			for _, ta := range program.transforms {
				if ta != nil {
					for _, track := range []*components.Animation{ta.Translation, ta.Rotation, ta.Scaling} {
						if track != nil && track.GlobalSeq != nil && !constantBakeTrack(track) {
							bakedGlobals[track.GlobalSeq] = true
						}
					}
				}
			}
		}
		if unitsPerSection[unit.SkinSectionIndex] == 0 {
			// Keep the source pointer alive for GeosetAnim links. Save a pristine
			// geometry copy for any subsequent batch before replacing its data.
			geosets[int(unit.SkinSectionIndex)] = cloneBakeGeoset(original)
			*original = *g
			g = original
			if meta != nil {
				kept := result.MDL.GeosetAnims[:0]
				for _, ga := range result.MDL.GeosetAnims {
					if ga.Geoset != original {
						kept = append(kept, ga)
					}
				}
				result.MDL.GeosetAnims = kept
			}
		} else {
			result.MDL.Geosets = append(result.MDL.Geosets, g)
			if meta == nil {
				for _, ga := range result.MDL.GeosetAnims {
					if ga.Geoset == targets[int(unit.SkinSectionIndex)] {
						copyGA := ga
						copyGA.Geoset = g
						result.MDL.GeosetAnims = append(result.MDL.GeosetAnims, copyGA)
						break
					}
				}
			}
		}
		if meta != nil {
			ga := meta.GeosetAnimation(int(unit.ColorIndex), g)
			if material.BlendingMode == 5 || material.BlendingMode == 6 {
				ga.Color = nil
			}
			if ga.Color != nil || ga.Alpha != nil {
				result.MDL.GeosetAnims = append(result.MDL.GeosetAnims, ga)
			}
		}
		if len(nativeMasks) > 0 {
			sourceGA := components.GeosetAnim{Geoset: g}
			gaIndex := -1
			for i, ga := range result.MDL.GeosetAnims {
				if ga.Geoset == g {
					sourceGA, gaIndex = ga, i
					break
				}
			}
			first := tintMaskAnimation(sourceGA, nativeMasks[0].mask)
			nativeSourceGA = sourceGA
			if gaIndex >= 0 {
				result.MDL.GeosetAnims[gaIndex] = first
			} else {
				result.MDL.GeosetAnims = append(result.MDL.GeosetAnims, first)
			}
			for i, draw := range nativeMasks[1:] {
				extra := draw.geoset
				extra.Name = fmt.Sprintf("%s_NativeMask%d", g.Name, i+1)
				extra.Material = g.Material
				result.MDL.Geosets = append(result.MDL.Geosets, extra)
				ga := tintMaskAnimation(sourceGA, draw.mask)
				ga.Geoset = extra
				result.MDL.GeosetAnims = append(result.MDL.GeosetAnims, ga)
			}
		}
		if environmentFactor != nil {
			result.MDL.Geosets = append(result.MDL.Geosets, environmentFactor)
			for _, ga := range result.MDL.GeosetAnims {
				if ga.Geoset == g {
					ga.Geoset, ga.Color = environmentFactor, nil
					result.MDL.GeosetAnims = append(result.MDL.GeosetAnims, ga)
					break
				}
			}
		}
		if len(alphaMasks) > 0 {
			for _, draw := range alphaMasks {
				result.MDL.Geosets = append(result.MDL.Geosets, draw.geoset)
				ga := tintMaskAnimation(nativeSourceGA, draw.mask)
				ga.Geoset, ga.Color = draw.geoset, nil
				result.MDL.GeosetAnims = append(result.MDL.GeosetAnims, ga)
			}
		} else if alphaAttenuation != nil {
			result.MDL.Geosets = append(result.MDL.Geosets, alphaAttenuation)
			for _, ga := range result.MDL.GeosetAnims {
				if ga.Geoset == g {
					ga.Geoset, ga.Color = alphaAttenuation, nil
					result.MDL.GeosetAnims = append(result.MDL.GeosetAnims, ga)
					break
				}
			}
		}
		if opaqueRemainder != nil && len(opaqueRemainder.Faces) > 0 {
			result.MDL.Geosets = append(result.MDL.Geosets, opaqueRemainder)
			ga := nativeSourceGA
			ga.Geoset = opaqueRemainder
			result.MDL.GeosetAnims = append(result.MDL.GeosetAnims, ga)
		}
		// Emission bypasses diffuse lighting and batch RGB, but keeps visibility.
		// A separate geoset is needed because WC3 colours every layer together.
		if len(g.Material.Layers) > 1 {
			for li, extraLayer := range g.Material.Layers[1:] {
				extra := cloneBakeGeoset(g)
				isDiffuse := program.blend == 7 && li == 0
				if isDiffuse {
					extra.Name += "_BlendAdd"
				} else {
					extra.Name += "_Emission"
				}
				extra.Material = &components.Material{PriorityPlane: g.Material.PriorityPlane, Layers: []components.Layer{extraLayer}}
				result.MDL.Geosets = append(result.MDL.Geosets, extra)
				result.MDL.Materials = append(result.MDL.Materials, extra.Material)
				for _, ga := range result.MDL.GeosetAnims {
					if ga.Geoset == g {
						ga.Geoset = extra
						if !isDiffuse {
							ga.Color = nil
						}
						result.MDL.GeosetAnims = append(result.MDL.GeosetAnims, ga)
						break
					}
				}
			}
			g.Material.Layers = g.Material.Layers[:1]
		}
		unitsPerSection[unit.SkinSectionIndex]++
	}
	boundNativeUVLoops(result, bakedGlobals, cfg.TextureBaking.WindowMS)
	if err := bakeM2Particles(ctx, cfg, result, loader, loadTexture); err != nil {
		return err
	}
	result.MDL.Sync()
	return nil
}

type bakeProgram struct {
	alphaOnly         bool
	opaqueMask        bool
	shader            m2Shader
	count             int
	blend             uint16
	images            [4]*image.NRGBA
	flags             [4]uint32
	transforms        [2]*components.TextureAnim
	weights           [4]components.AnimatedOrStatic[float64]
	shade             func(bakePixel, bakeMoment) m2Fragment
	conflict          func(bakePixel, bakePixel) bool
	hasEmission       bool
	screen            bool
	pixelBudget       int
	environmentFactor bool
}

func m2HasEmission(pixel int) bool {
	switch pixel {
	case 8, 10, 13, 14, 15, 16, 17, 20, 21, 23, 24, 25, 34:
		return true
	}
	return false
}

func registerBakeTexture(cfg config.Config, result *ConvertResult, atlas *image.NRGBA) (*components.Texture, error) {
	var encoded bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&encoded, atlas); err != nil {
		return nil, err
	}
	hash := sha256.Sum256(encoded.Bytes())
	rel := fmt.Sprintf("baked/uv2/%x.png", hash[:16])
	texturesource.Register(rel, texturesource.Source{Kind: texturesource.KindPNG, PNG: encoded.Bytes(), PreserveAlpha: true})
	result.TexturePaths[rel] = struct{}{}
	tex := &components.Texture{Image: filepath.ToSlash(filepath.Join(cfg.AssetPrefix, strings.TrimSuffix(rel, ".png")+".blp")), WowData: components.TextureWowData{PngPath: rel}}
	result.MDL.Textures = append(result.MDL.Textures, tex)
	return tex, nil
}

func m2BakeBlend(mode uint16) components.BlendMode {
	return [8]components.BlendMode{components.BlendNone, components.BlendTransparent, components.BlendBlend, components.BlendAdditive, components.BlendAddAlpha, components.BlendModulate, components.BlendModulate2x, components.BlendBlend}[min(7, int(mode))]
}

func m2BakeBudgets(loader *m2.Loader, skin *m2.Skin, geosets map[int]*components.Geoset) []int {
	areas := make([]float64, len(skin.TextureUnits))
	valid := make([]bool, len(areas))
	areaSum, overlays := 0.0, 0
	for i, unit := range skin.TextureUnits {
		g := geosets[int(unit.SkinSectionIndex)]
		if g == nil || int(unit.MaterialIndex) >= len(loader.Materials) {
			continue
		}
		valid[i] = true
		shader, _ := decodeM2Shader(unit.ShaderID, int(unit.TextureCount))
		if loader.Materials[unit.MaterialIndex].BlendingMode <= 1 && shader.pixel != 12 {
			for _, f := range g.Faces {
				a, b, c := f.Vertices[0].Position, f.Vertices[1].Position, f.Vertices[2].Position
				u, v := imath.Vector3{b[0] - a[0], b[1] - a[1], b[2] - a[2]}, imath.Vector3{c[0] - a[0], c[1] - a[1], c[2] - a[2]}
				x, y, z := u[1]*v[2]-u[2]*v[1], u[2]*v[0]-u[0]*v[2], u[0]*v[1]-u[1]*v[0]
				areas[i] += math.Sqrt(x*x+y*y+z*z) / 2
			}
		}
		if areas[i] > 0 {
			areaSum += areas[i]
		} else {
			overlays++
		}
	}
	// Reserve detail for the main painted surface rather than giving a tiny
	// effect card the same space as an entire body. Smooth environment factors
	// share the overlay reserve; their source albedo remains native.
	primaryBudget, overlayBudget := 0, bakeMaterialPixels
	if areaSum > 0 {
		// Painted surfaces need more spatial detail than smooth effect masks.
		// This remains a per-model reserve, shared by physical surface area.
		primaryBudget = bakeMaterialPixels * 4 / 3
		overlayBudget = 0
		if overlays > 0 {
			overlayBudget = bakeMaterialPixels / 3
		}
	}
	budgets := make([]int, len(areas))
	for i, area := range areas {
		if !valid[i] {
			continue
		}
		if area > 0 {
			budgets[i] = max(1, int(float64(primaryBudget)*area/areaSum))
		} else {
			budgets[i] = max(1, overlayBudget/max(1, overlays))
		}
	}
	return budgets
}

func cloneBakeGeoset(g *components.Geoset) *components.Geoset {
	copyG := *g
	copyG.Vertices = nil
	copyG.Faces = append([]components.Face(nil), g.Faces...)
	vertices := map[*components.GeosetVertex]*components.GeosetVertex{}
	for _, v := range g.Vertices {
		cloned := *v
		vertices[v] = &cloned
		copyG.Vertices = append(copyG.Vertices, &cloned)
	}
	for fi := range copyG.Faces {
		for vi, v := range copyG.Faces[fi].Vertices {
			copyG.Faces[fi].Vertices[vi] = vertices[v]
		}
	}
	return &copyG
}

func bakeUV2Geoset(ctx context.Context, cfg config.Config, result *ConvertResult, g *components.Geoset, shader uint16, images [2]*image.NRGBA, flags [2]uint32, transforms [2]*components.TextureAnim) error {
	description, err := decodeM2Shader(shader, 2)
	if err != nil {
		return err
	}
	program := bakeProgram{shader: description, count: 2, blend: 2, images: [4]*image.NRGBA{images[0], images[1]}, flags: [4]uint32{flags[0], flags[1]}, transforms: transforms}
	for k := range 4 {
		program.weights[k] = components.AnimatedOrStatic[float64]{Static: true, Value: 1}
	}
	return bakeM2Geoset(ctx, cfg, result, g, program)
}

func bakeM2Geoset(ctx context.Context, cfg config.Config, result *ConvertResult, g *components.Geoset, program bakeProgram) error {
	transforms := program.transforms
	usedMatrix := [2]bool{}
	usesEnv := false
	for _, coord := range program.shader.coords[:program.count] {
		if coord == coordT1M0 || coord == coordT2M0 {
			usedMatrix[0] = true
		}
		if coord == coordT1M1 || coord == coordT2M1 {
			usedMatrix[1] = true
		}
		usesEnv = usesEnv || coord == coordEnv
	}
	for k := range 2 {
		if !usedMatrix[k] && program.shade == nil {
			transforms[k] = nil
		}
	}
	plan := makeBakeTimeline(transforms, program.weights, program.shader.pixel, result.MDL.Sequences)
	frames := len(plan.moments)
	minUV, maxUV := imath.Vector2{math.Inf(1), math.Inf(1)}, imath.Vector2{math.Inf(-1), math.Inf(-1)}
	minUV2, maxUV2 := minUV, maxUV
	for _, f := range g.Faces {
		for _, v := range f.Vertices {
			if v.TexPosition2 == nil {
				uv := v.TexPosition
				v.TexPosition2 = &uv
			}
			for axis := range 2 {
				minUV[axis] = min(minUV[axis], v.TexPosition[axis])
				maxUV[axis] = max(maxUV[axis], v.TexPosition[axis])
				minUV2[axis] = min(minUV2[axis], (*v.TexPosition2)[axis])
				maxUV2[axis] = max(maxUV2[axis], (*v.TexPosition2)[axis])
			}
		}
	}
	span := imath.Vector2{maxUV[0] - minUV[0], maxUV[1] - minUV[1]}
	span2 := imath.Vector2{maxUV2[0] - minUV2[0], maxUV2[1] - minUV2[1]}
	// Triangle coverage detects compressed gradient swatches even when a few
	// outlying UVs make their overall bounding box look like a full unwrap.
	domain2 := span2[0] > 1e-9 && span2[1] > 1e-9 && (span[0] <= 1e-9 || span[1] <= 1e-9 || bakePreferUV2(g, program))
	if domain2 {
		minUV, maxUV, span = minUV2, maxUV2, span2
	}
	chartOptions := bakeChartOptions{conflict: program.conflict}
	if span[0] <= 1e-9 || span[1] <= 1e-9 {
		// Uniform/gradient swatches can have no surface unwrap at all. Make a
		// planar raster domain while preserving the UVs used by the shader.
		boundsMin, boundsMax := imath.Vector3{math.Inf(1), math.Inf(1), math.Inf(1)}, imath.Vector3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
		for _, f := range g.Faces {
			for _, v := range f.Vertices {
				for k := range 3 {
					boundsMin[k] = min(boundsMin[k], v.Position[k])
					boundsMax[k] = max(boundsMax[k], v.Position[k])
				}
			}
		}
		axes := []int{0, 1, 2}
		sort.SliceStable(axes, func(i, j int) bool {
			return boundsMax[axes[i]]-boundsMin[axes[i]] > boundsMax[axes[j]]-boundsMin[axes[j]]
		})
		chartOptions.sourceUV = map[*components.GeosetVertex][2]imath.Vector2{}
		for _, v := range g.Vertices {
			chartOptions.sourceUV[v] = [2]imath.Vector2{v.TexPosition, *v.TexPosition2}
			v.TexPosition = imath.Vector2{v.Position[axes[0]], v.Position[axes[1]]}
		}
		minUV = imath.Vector2{boundsMin[axes[0]], boundsMin[axes[1]]}
		maxUV = imath.Vector2{boundsMax[axes[0]], boundsMax[axes[1]]}
		span = imath.Vector2{maxUV[0] - minUV[0], maxUV[1] - minUV[1]}
		domain2 = false
		log.Printf("Shader bake %s: generated a planar surface domain for degenerate UVs", g.Name)
	}
	if span[0] <= 1e-9 || span[1] <= 1e-9 || !finiteBakeUV(minUV) || !finiteBakeUV(maxUV) {
		return fmt.Errorf("%w: degenerate or nonfinite primary UV domain", errUV2BakeUnsupported)
	}
	limit := 512
	if frames == 1 {
		limit = 1024
	}
	w, h := bakeRasterSize(span, program, domain2, limit)
	spatialW, spatialH := w, h
	constantUV2 := [2]bool{}
	other := 1
	if domain2 {
		other = 0
	}
	if program.count == 2 && program.shader.coords[0] == coordT1M0 && program.shader.coords[1] == coordT2M1 && (transforms[other] == nil || transforms[other].Rotation == nil) {
		constantUV2 = bakeConstantAxes(program.images[other])
	}
	var env map[*components.GeosetVertex]imath.Vector2
	// Classic cannot update edge fading with the camera or animated normals.
	// Neutral opacity keeps effect shells visible from every view, matching the
	// reference renderer's fallback until a runtime edge-fade representation exists.
	if usesEnv {
		log.Printf("M2 bake %s: environment combiner uses bind-normal reference projection (camera dependent)", g.Name)
		env = map[*components.GeosetVertex]imath.Vector2{}
		for _, v := range g.Vertices {
			env[v] = m2SphereCoord(imath.Vector3{0, 0, -1}, imath.Vector3{v.Normal[1], v.Normal[2], v.Normal[0]})
		}
	}
	charts, faceCharts := buildBakeCharts(g, minUV, span, w, h, constantUV2, domain2, env, chartOptions)
	frameCols := bakePowerOfTwo(int(math.Ceil(math.Sqrt(float64(frames)))))
	frameRows := bakePowerOfTwo((frames + frameCols - 1) / frameCols)
	// Balance the complete atlas, including rectangular tiles and frame grids.
	frameW, frameH := packBakeCharts(charts, w, h)
	width, height := frameW*frameCols, frameH*frameRows
	passes := 1
	if program.hasEmission || m2HasEmission(program.shader.pixel) {
		passes++
	}
	if program.blend == 7 || program.screen {
		passes++
	}
	budget := program.pixelBudget
	if budget <= 0 {
		budget = 1024 * 1024
	}
	for frameW > 2048 || frameH > 2048 || width*height*passes > budget {
		// Preserve surface detail before spending the budget on temporal samples.
		// Crowded effect meshes use fewer frames rather than tiny, blurred charts.
		if w <= 256 && h <= 256 && frames > 4 {
			plan = reduceBakeTimeline(plan)
			frames = len(plan.moments)
			frameCols = bakePowerOfTwo(int(math.Ceil(math.Sqrt(float64(frames)))))
			frameRows = bakePowerOfTwo((frames + frameCols - 1) / frameCols)
		} else if w <= 16 && h <= 16 {
			if frames > 1 {
				reduced := reduceBakeTimeline(plan)
				if len(reduced.moments) == frames {
					break // Every sequence needs at least one distinct material state.
				}
				plan = reduced
				frames = len(plan.moments)
				frameCols = bakePowerOfTwo(int(math.Ceil(math.Sqrt(float64(frames)))))
				frameRows = bakePowerOfTwo((frames + frameCols - 1) / frameCols)
			} else {
				break
			}
		} else if w*spatialH >= h*spatialW && w > 16 {
			w /= 2
		} else if h > 16 {
			h /= 2
		} else {
			w /= 2
		}
		charts, faceCharts = buildBakeCharts(g, minUV, span, w, h, constantUV2, domain2, env, chartOptions)
		frameW, frameH = packBakeCharts(charts, w, h)
		width, height = frameW*frameCols, frameH*frameRows
	}
	// Explicit requests keep the requested sampling rate. Choose spatial detail
	// with the compact reference above, then grow/page the atlas in time. This
	// keeps FPS comparisons at identical spatial quality instead of silently
	// dropping frames or blurring the higher-FPS variants further.
	if cfg.TextureBaking.FPS > 0 || cfg.TextureBaking.WindowMS > 0 {
		plan = makeBakeTimeline(transforms, program.weights, program.shader.pixel, result.MDL.Sequences, cfg.TextureBaking)
		frames = len(plan.moments)
		frameCols = bakePowerOfTwo(int(math.Ceil(math.Sqrt(float64(frames)))))
		frameRows = bakePowerOfTwo((frames + frameCols - 1) / frameCols)
	}
	// Paging is a safety valve for unusually complex static chart layouts.
	frameCols = min(frameCols, max(1, 2048/frameW))
	frameRows = min(frameRows, max(1, 2048/frameH))
	pageFrames := frameCols * frameRows
	width, height = frameW*frameCols, frameH*frameRows
	if frameW > 4096 || frameH > 4096 {
		return fmt.Errorf("%w: one frame is %dx%d", errUV2BakeUnsupported, frameW, frameH)
	}
	atlas := image.NewNRGBA(image.Rect(0, 0, width, height))
	var emissionAtlas *image.NRGBA
	var radianceAtlas *image.NRGBA
	if program.blend == 7 || program.screen {
		radianceAtlas = image.NewNRGBA(atlas.Bounds())
	}
	if program.hasEmission || m2HasEmission(program.shader.pixel) {
		emissionAtlas = image.NewNRGBA(atlas.Bounds())
	}
	var pages, emissionPages, radiancePages []*components.Texture
	textureBytes := 0
	flushPage := func() error {
		tex, err := registerBakeTexture(cfg, result, atlas)
		if err != nil {
			return err
		}
		pages = append(pages, tex)
		if source, ok := texturesource.Get(tex.WowData.PngPath); ok {
			textureBytes += len(source.PNG)
		}
		if emissionAtlas != nil {
			tex, err = registerBakeTexture(cfg, result, emissionAtlas)
			if err != nil {
				return err
			}
			emissionPages = append(emissionPages, tex)
		}
		if radianceAtlas != nil {
			tex, err = registerBakeTexture(cfg, result, radianceAtlas)
			if err != nil {
				return err
			}
			radiancePages = append(radiancePages, tex)
		}
		return nil
	}
	for frame := range frames {
		if err := ctx.Err(); err != nil {
			return err
		}
		moment := plan.moments[frame]
		matrices := [2][6]float64{bakeTransformAt(transforms[0], moment), bakeTransformAt(transforms[1], moment)}
		weights := [4]float64{}
		for k, weight := range program.weights {
			weights[k] = weight.Value
			if !weight.Static {
				weights[k] = bakeAnimationAt(weight.Anim, moment, []float64{1})[0]
			}
		}
		for _, chart := range charts {
			localFrame := frame % pageFrames
			ox := (localFrame%frameCols)*frameW + chart.x - chart.minX
			oy := (localFrame/frameCols)*frameH + chart.y - chart.minY
			for _, pi := range chart.active {
				p := chart.pixels[pi]
				samples := [4][4]float64{}
				for k := 0; program.shade == nil && k < program.count; k++ {
					uv := p.uv1
					switch program.shader.coords[k] {
					case coordT1M0:
						uv = transformBakeUV(p.uv1, matrices[0])
					case coordT1M1:
						uv = transformBakeUV(p.uv1, matrices[1])
					case coordT2M0:
						uv = transformBakeUV(p.uv2, matrices[0])
					case coordT2M1:
						uv = transformBakeUV(p.uv2, matrices[1])
					case coordT2:
						uv = p.uv2
					case coordEnv:
						uv = p.env
					}
					samples[k] = sampleBakeTexture(program.images[k], uv, program.flags[k])
				}
				fragment := evaluateM2Combiner(program.shader.pixel, samples, weights)
				if program.environmentFactor {
					for k := range 3 {
						fragment.diffuse[k] = samples[1][k]*2*(1-samples[0][3]) + samples[0][3]
					}
				}
				if program.shade != nil {
					fragment = program.shade(p, moment)
				}
				color := [4]float64{fragment.diffuse[0], fragment.diffuse[1], fragment.diffuse[2], fragment.alpha}
				if program.alphaOnly {
					color[0], color[1], color[2] = 0, 0, 0
				}
				if program.blend == 0 || program.blend == 3 || program.blend == 5 || program.blend == 6 {
					color[3] = 1
				}
				if program.blend == 6 {
					for k := range 3 {
						color[k] *= .5
					}
				}
				if program.blend == 1 {
					if fragment.alpha >= .501960814 {
						color[3] = 1
					} else {
						color[3] = 0
					}
				}
				if program.blend == 7 {
					color[0], color[1], color[2] = 0, 0, 0
				}
				if program.screen {
					for k := range 3 {
						color[k] = 1 - clampM2(fragment.diffuse[k]+fragment.emission[k])
					}
					color[3] = 1
				}
				at := (oy+pi/w)*atlas.Stride + (ox+pi%w)*4
				for channel := range 4 {
					atlas.Pix[at+channel] = uint8(math.Round(min(1, max(0, color[channel])) * 255))
					if radianceAtlas != nil {
						value := 1.0
						if channel < 3 {
							value = fragment.diffuse[channel]
							if program.screen {
								value += fragment.emission[channel]
							}
						}
						radianceAtlas.Pix[at+channel] = uint8(math.Round(clampM2(value) * 255))
					}
					if emissionAtlas != nil {
						value := fragment.alpha
						if program.blend == 0 || program.blend == 3 || program.blend == 7 {
							value = 1
						}
						if channel < 3 {
							value = fragment.emission[channel]
						}
						emissionAtlas.Pix[at+channel] = uint8(math.Round(clampM2(value) * 255))
					}
				}
			}
		}
		if (frame+1)%pageFrames == 0 || frame+1 == frames {
			if err := flushPage(); err != nil {
				return err
			}
			clear(atlas.Pix)
			if emissionAtlas != nil {
				clear(emissionAtlas.Pix)
			}
			if radianceAtlas != nil {
				clear(radianceAtlas.Pix)
			}
		}
	}
	var anim *components.TextureAnim
	if frames > 1 {
		var globalSeq *components.GlobalSequence
		if plan.period > 0 {
			gs := components.NewGlobalSequence(len(result.MDL.GlobalSequences), plan.period)
			// This generated sequence has no M2 raw ID.
			gs.HasRawID = false
			result.MDL.GlobalSequences = append(result.MDL.GlobalSequences, &gs)
			globalSeq = &gs
		}
		anim = &components.TextureAnim{Translation: &components.Animation{GlobalSeq: globalSeq, Type: components.AnimTypeTVertexAnim, Interpolation: components.InterpDontInterp, KeyFrames: map[int]any{}}}
		for t, frame := range plan.keys {
			frame %= pageFrames
			anim.Translation.KeyFrames[t] = imath.Vector3{float64(frame%frameCols) / float64(frameCols), float64(frame/frameCols) / float64(frameRows), 0}
		}
		anim.ID = len(result.MDL.TextureAnims)
		result.MDL.TextureAnims = append(result.MDL.TextureAnims, *anim)
	}
	oldMaterial := g.Material
	layer := oldMaterial.Layers[0]
	layer.Texture, layer.TVertexAnim, layer.CoordID = pages[0], anim, nil
	pageAnimation := func(textures []*components.Texture) *components.Animation {
		if len(textures) <= 1 {
			return nil
		}
		track := &components.Animation{GlobalSeq: anim.Translation.GlobalSeq, Type: components.AnimTypeOthers, Interpolation: components.InterpDontInterp, KeyFrames: map[int]any{}}
		for t, frame := range plan.keys {
			track.KeyFrames[t] = textures[frame/pageFrames]
		}
		return track
	}
	layer.TextureIDAnim = pageAnimation(pages)
	layer.Unshaded = layer.Unshaded || layer.Unlit
	if program.screen {
		layer.FilterMode = components.BlendModulate
		layer.Unshaded = true
	}
	material := *oldMaterial
	material.Layers = []components.Layer{layer}
	if len(radiancePages) > 0 {
		// WoW BlendAdd uses ONE, ONE_MINUS_SRC_ALPHA. A black alpha pass
		// attenuates the destination; a lit additive pass then adds source RGB.
		// Unlike straight-alpha blending this preserves zero-alpha radiance.
		radiance := layer
		radiance.Texture, radiance.TextureIDAnim = radiancePages[0], pageAnimation(radiancePages)
		radiance.FilterMode, radiance.NoDepthSet = components.BlendAddAlpha, true
		if program.screen {
			radiance.Unshaded = true
		}
		material.Layers = append(material.Layers, radiance)
	}
	if len(emissionPages) > 0 {
		emissive := layer
		emissive.Texture = emissionPages[0]
		emissive.TextureIDAnim = pageAnimation(emissionPages)
		emissive.FilterMode = components.BlendAddAlpha
		emissive.Unshaded = true
		emissive.NoDepthSet = true
		material.Layers = append(material.Layers, emissive)
	}
	g.Material = &material
	result.MDL.Materials = append(result.MDL.Materials, g.Material)
	vertices := make([]map[*components.GeosetVertex]*components.GeosetVertex, len(charts))
	for ci := range vertices {
		vertices[ci] = map[*components.GeosetVertex]*components.GeosetVertex{}
	}
	g.Vertices = nil
	for fi := range g.Faces {
		ci := faceCharts[fi]
		for vi, original := range g.Faces[fi].Vertices {
			v := vertices[ci][original]
			if v == nil {
				copyVertex := *original
				uv := original.TexPosition
				if domain2 {
					uv = *original.TexPosition2
				}
				copyVertex.TexPosition = imath.Vector2{
					(float64(charts[ci].x-charts[ci].minX+bakePadding) + (uv[0]-minUV[0])/span[0]*float64(w-2*bakePadding)) / float64(width),
					(float64(charts[ci].y-charts[ci].minY+bakePadding) + (uv[1]-minUV[1])/span[1]*float64(h-2*bakePadding)) / float64(height),
				}
				copyVertex.TexPosition2 = nil
				v = &copyVertex
				vertices[ci][original] = v
				g.Vertices = append(g.Vertices, v)
			}
			g.Faces[fi].Vertices[vi] = v
		}
	}
	log.Printf("UV2 bake %s: %d charts on %dx%d grid, %d frames/%d ms, %d pages of %dx%d, %d PNG bytes", g.Name, len(charts), w, h, frames, plan.period, len(pages), width, height, textureBytes)
	return nil
}

func bakePreferUV2(g *components.Geoset, p bakeProgram) bool {
	weight := [2]float64{}
	for i := range p.count {
		axis := -1
		switch p.shader.coords[i] {
		case coordT1M0, coordT1M1, coordT1:
			axis = 0
		case coordT2M0, coordT2M1, coordT2:
			axis = 1
		}
		if axis >= 0 && p.images[i] != nil {
			weight[axis] = max(weight[axis], float64(p.images[i].Bounds().Dx()*p.images[i].Bounds().Dy()))
		}
	}
	area := [2]float64{}
	for _, f := range g.Faces {
		for axis := range 2 {
			uv := [3]imath.Vector2{}
			for i, v := range f.Vertices {
				uv[i] = v.TexPosition
				if axis == 1 && v.TexPosition2 != nil {
					uv[i] = *v.TexPosition2
				}
			}
			area[axis] += math.Abs((uv[1][0]-uv[0][0])*(uv[2][1]-uv[0][1]) - (uv[1][1]-uv[0][1])*(uv[2][0]-uv[0][0]))
		}
	}
	return area[1]*weight[1] > area[0]*weight[0]*1.01
}

// Preserve source texel proportions when shrinking charts. Atlas packing can
// be wide even for a narrow UV strip; using its shape repeatedly halves the
// same surface axis and turns round details into horizontal smears.
func bakeRasterSize(span imath.Vector2, p bakeProgram, domain2 bool, limit int) (int, int) {
	w, h := 64.0, 64.0
	for i := range p.count {
		coord := p.shader.coords[i]
		secondary := coord == coordT2 || coord == coordT2M0 || coord == coordT2M1
		if coord == coordEnv || secondary != domain2 || p.images[i] == nil {
			continue
		}
		w = max(w, span[0]*float64(p.images[i].Bounds().Dx()))
		h = max(h, span[1]*float64(p.images[i].Bounds().Dy()))
	}
	scale := min(1, float64(limit)/max(w, h))
	return bakePowerOfTwo(max(16, int(math.Ceil(w*scale)))), bakePowerOfTwo(max(16, int(math.Ceil(h*scale))))
}

func buildBakeCharts(g *components.Geoset, minUV, span imath.Vector2, w, h int, constantUV2 [2]bool, domain2 bool, env map[*components.GeosetVertex]imath.Vector2, options ...bakeChartOptions) ([]bakeChart, []int) {
	opt := bakeChartOptions{}
	if len(options) > 0 {
		opt = options[0]
	}
	charts := []bakeChart{{pixels: make([]bakePixel, w*h)}}
	faceCharts := make([]int, len(g.Faces))
	for fi, face := range g.Faces {
		var points [3]imath.Vector2
		for i, v := range face.Vertices {
			uv := v.TexPosition
			if domain2 {
				uv = *v.TexPosition2
			}
			points[i] = imath.Vector2{float64(bakePadding) + (uv[0]-minUV[0])/span[0]*float64(w-2*bakePadding), float64(bakePadding) + (uv[1]-minUV[1])/span[1]*float64(h-2*bakePadding)}
		}
		x0 := max(0, int(math.Floor(min(points[0][0], points[1][0], points[2][0]))))
		x1 := min(w-1, int(math.Ceil(max(points[0][0], points[1][0], points[2][0]))))
		y0 := max(0, int(math.Floor(min(points[0][1], points[1][1], points[2][1]))))
		y1 := min(h-1, int(math.Ceil(max(points[0][1], points[1][1], points[2][1]))))
		type pixelWrite struct {
			index int
			pixel bakePixel
		}
		var writes []pixelWrite
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				weights, inside := bakeBarycentric(points, imath.Vector2{float64(x) + 0.5, float64(y) + 0.5})
				if !inside {
					// Conservative coverage keeps sub-pixel triangles and boundary
					// texels from becoming transparent holes at reduced tile sizes.
					sum := 0.0
					for k := range 3 {
						weights[k] = max(0, weights[k])
						sum += weights[k]
					}
					if sum <= 0 {
						continue
					}
					closest := imath.Vector2{}
					for k := range 3 {
						weights[k] /= sum
						for axis := range 2 {
							closest[axis] += weights[k] * points[k][axis]
						}
					}
					if math.Hypot(closest[0]-(float64(x)+.5), closest[1]-(float64(y)+.5)) > .8 {
						continue
					}
				}
				p := bakePixel{set: true, covered: inside, face: fi, bary: weights}
				for k, v := range face.Vertices {
					coords := [2]imath.Vector2{v.TexPosition, *v.TexPosition2}
					if original, ok := opt.sourceUV[v]; ok {
						coords = original
					}
					for axis := range 2 {
						p.uv1[axis] += weights[k] * coords[0][axis]
						p.uv2[axis] += weights[k] * coords[1][axis]
						p.env[axis] += weights[k] * env[v][axis]
					}
				}
				// A constant mask axis cannot cause a sampling conflict. Collapsing
				// it shares charts without changing any baked pixels.
				for axis := range 2 {
					if constantUV2[axis] {
						if domain2 {
							p.uv1[axis] = 0
						} else {
							p.uv2[axis] = 0
						}
					}
				}
				writes = append(writes, pixelWrite{y*w + x, p})
			}
		}
		ci := 0
		for ; ci < len(charts); ci++ {
			conflict := false
			for _, write := range writes {
				old := charts[ci].pixels[write.index]
				a, b := old.uv2, write.pixel.uv2
				if domain2 {
					a, b = old.uv1, write.pixel.uv1
				}
				if old.set && old.covered && write.pixel.covered && (math.Abs(a[0]-b[0]) > 1e-4 || math.Abs(a[1]-b[1]) > 1e-4 || math.Abs(old.env[0]-write.pixel.env[0]) > 1e-4 || math.Abs(old.env[1]-write.pixel.env[1]) > 1e-4) {
					conflict = true
					break
				}
				if old.set && old.covered && write.pixel.covered && opt.conflict != nil && opt.conflict(old, write.pixel) {
					conflict = true
					break
				}
			}
			if !conflict {
				break
			}
		}
		if ci == len(charts) {
			charts = append(charts, bakeChart{pixels: make([]bakePixel, w*h)})
		}
		for _, write := range writes {
			old := charts[ci].pixels[write.index]
			if !old.set || write.pixel.covered {
				charts[ci].pixels[write.index] = write.pixel
			}
		}
		faceCharts[fi] = ci
	}
	return charts, faceCharts
}

func bakeConstantAxes(img *image.NRGBA) [2]bool {
	constant := [2]bool{true, true}
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			at := y*img.Stride + x*4
			for c := range 4 {
				constant[0] = constant[0] && img.Pix[at+c] == img.Pix[y*img.Stride+c]
				constant[1] = constant[1] && img.Pix[at+c] == img.Pix[x*4+c]
			}
		}
	}
	return constant
}

// Crop each UV chart to the pixels it actually occupies before packing. A
// small island must not reserve an entire copy of the surface UV rectangle.
func packBakeCharts(charts []bakeChart, w, h int) (int, int) {
	area, widest := 0, 1
	order := make([]int, len(charts))
	for ci := range charts {
		chart := &charts[ci]
		padBakeChart(chart.pixels, w, h)
		chart.active = chart.active[:0]
		x0, y0, x1, y1 := w, h, -1, -1
		for pi, p := range chart.pixels {
			if p.set {
				chart.active = append(chart.active, pi)
				x, y := pi%w, pi/w
				x0, y0 = min(x0, x), min(y0, y)
				x1, y1 = max(x1, x), max(y1, y)
			}
		}
		if x1 < 0 {
			x0, y0, x1, y1 = 0, 0, w-1, h-1
		}
		chart.minX, chart.minY, chart.width, chart.height = x0, y0, x1-x0+1, y1-y0+1
		area += chart.width * chart.height
		widest = max(widest, chart.width)
		order[ci] = ci
	}
	sort.SliceStable(order, func(i, j int) bool { return charts[order[i]].height > charts[order[j]].height })
	rowWidth := bakePowerOfTwo(max(widest, int(math.Ceil(math.Sqrt(float64(area))))))
	x, y, rowHeight, maxX := 0, 0, 0, 0
	for _, ci := range order {
		chart := &charts[ci]
		if x+chart.width > rowWidth {
			x = 0
			y += rowHeight
			rowHeight = 0
		}
		chart.x, chart.y = x, y
		x += chart.width
		rowHeight = max(rowHeight, chart.height)
		maxX = max(maxX, x)
	}
	return bakePowerOfTwo(maxX), bakePowerOfTwo(y + rowHeight)
}

func bakeBarycentric(points [3]imath.Vector2, p imath.Vector2) ([3]float64, bool) {
	a, b, c := points[0], points[1], points[2]
	det := (b[1]-c[1])*(a[0]-c[0]) + (c[0]-b[0])*(a[1]-c[1])
	if math.Abs(det) < 1e-12 {
		return [3]float64{}, false
	}
	x := ((b[1]-c[1])*(p[0]-c[0]) + (c[0]-b[0])*(p[1]-c[1])) / det
	y := ((c[1]-a[1])*(p[0]-c[0]) + (a[0]-c[0])*(p[1]-c[1])) / det
	z := 1 - x - y
	return [3]float64{x, y, z}, x >= -1e-8 && y >= -1e-8 && z >= -1e-8
}

func padBakeChart(pixels []bakePixel, w, h int) {
	// Extend sample coordinates beyond triangle edges, avoiding dark filtering
	// seams while retaining the shader's straight (unpremultiplied) RGB.
	for range bakePadding {
		next := append([]bakePixel(nil), pixels...)
		for y := range h {
			for x := range w {
				at := y*w + x
				if pixels[at].set {
					continue
				}
				for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
					x2, y2 := x+d[0], y+d[1]
					if x2 >= 0 && x2 < w && y2 >= 0 && y2 < h && pixels[y2*w+x2].set {
						next[at] = pixels[y2*w+x2]
						break
					}
				}
			}
		}
		copy(pixels, next)
	}
}

func bakePeriod(transforms [2]*components.TextureAnim) (int, error) {
	period := 0
	longest := 0
	approximate := false
	for _, transform := range transforms {
		if transform == nil {
			continue
		}
		for _, track := range []*components.Animation{transform.Translation, transform.Rotation, transform.Scaling} {
			if track == nil || len(track.KeyFrames) <= 1 || constantBakeTrack(track) {
				continue
			}
			if track.GlobalSeq == nil {
				return 0, fmt.Errorf("%w: sequence-local UV animation requires per-sequence baking", errUV2BakeUnsupported)
			}
			duration := track.GlobalSeq.Duration
			if duration <= 0 {
				return 0, fmt.Errorf("invalid UV loop duration %d", duration)
			}
			if duration > 60000 {
				return 0, fmt.Errorf("%w: UV loop %d ms exceeds 60 seconds", errUV2BakeUnsupported, duration)
			}
			longest = max(longest, duration)
			if period == 0 {
				period = duration
			} else {
				a, b := period, duration
				for b != 0 {
					a, b = b, a%b
				}
				period = period / a * duration
			}
			if period > 60000 {
				approximate = true
				period = longest
			}
		}
	}
	if approximate {
		log.Printf("UV2 bake: independent loops use a %d ms window; their phase resets at the window boundary", longest)
		period = longest
	}
	return period, nil
}

func constantBakeTrack(track *components.Animation) bool {
	var first []float64
	sampled := *track
	sampled.GlobalSeq = nil
	times := components.SortedKeyInts(track.KeyFrames)
	if len(times) > 1 && (track.Interpolation == components.InterpHermite || track.Interpolation == components.InterpBezier) && (len(track.InOutTans) != len(times) || track.Type == components.AnimTypeRotation) {
		return false
	}
	probes := make([]float64, 0, len(times)*3)
	for i, t := range times {
		probes = append(probes, float64(t))
		if i+1 < len(times) && (track.Interpolation == components.InterpHermite || track.Interpolation == components.InterpBezier) {
			// Equal cubic endpoints can still move between keys. Two interior
			// probes plus the endpoints determine whether a cubic is constant.
			delta := float64(times[i+1]-t) / 3
			probes = append(probes, float64(t)+delta, float64(t)+2*delta)
		}
	}
	for _, t := range probes {
		value := sampleBakeAnimation(&sampled, t, nil)
		if first == nil {
			first = value
			continue
		}
		if len(first) != len(value) {
			return false
		}
		for i := range value {
			if math.Abs(first[i]-value[i]) > 1e-6 {
				return false
			}
		}
	}
	return true
}

func bakeTransform(anim *components.TextureAnim, timeMS float64) [6]float64 {
	if anim == nil {
		return [6]float64{1, 0, 0, 0, 1, 0}
	}
	t := sampleBakeAnimation(anim.Translation, timeMS, []float64{0, 0, 0})
	s := sampleBakeAnimation(anim.Scaling, timeMS, []float64{1, 1, 1})
	q := sampleBakeAnimation(anim.Rotation, timeMS, []float64{0, 0, 0, 1})
	length := math.Sqrt(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3])
	if length > 0 {
		for i := range q {
			q[i] /= length
		}
	}
	a, b := (1-2*(q[1]*q[1]+q[2]*q[2]))*s[0], 2*(q[0]*q[1]-q[2]*q[3])*s[1]
	c, d := 2*(q[0]*q[1]+q[2]*q[3])*s[0], (1-2*(q[0]*q[0]+q[2]*q[2]))*s[1]
	return [6]float64{a, b, 0.5 + t[0] - 0.5*a - 0.5*b, c, d, 0.5 + t[1] - 0.5*c - 0.5*d}
}

func sampleBakeAnimation(anim *components.Animation, timeMS float64, fallback []float64) []float64 {
	if anim == nil || len(anim.KeyFrames) == 0 {
		return fallback
	}
	if anim.GlobalSeq != nil && anim.GlobalSeq.Duration > 0 {
		timeMS = math.Mod(timeMS, float64(anim.GlobalSeq.Duration))
	}
	keys := components.SortedKeyInts(anim.KeyFrames)
	index := sort.Search(len(keys), func(i int) bool { return float64(keys[i]) > timeMS })
	left := max(0, index-1)
	value := func(key int) []float64 {
		switch v := anim.KeyFrames[key].(type) {
		case imath.Vector3:
			return v[:]
		case imath.QuaternionRotation:
			return v[:]
		case []float64:
			return v
		case float64:
			return []float64{v}
		default:
			return fallback
		}
	}
	a := value(keys[left])
	if anim.Interpolation == components.InterpDontInterp || index >= len(keys) || index == 0 {
		return a
	}
	b := value(keys[index])
	t := (timeMS - float64(keys[left])) / float64(keys[index]-keys[left])
	if anim.Type == components.AnimTypeRotation {
		q := imath.V3Slerp(imath.QuaternionRotation(a), imath.QuaternionRotation(b), t)
		return q[:]
	}
	out := make([]float64, len(a))
	for i := range out {
		out[i] = a[i] + (b[i]-a[i])*t
		if i < 3 {
			leftTan, hasLeft := anim.InOutTans[keys[left]]
			rightTan, hasRight := anim.InOutTans[keys[index]]
			if hasLeft && hasRight {
				u := 1 - t
				if anim.Interpolation == components.InterpHermite {
					out[i] = a[i]*(2*t*t*t-3*t*t+1) + leftTan.OutTan[i]*(t*t*t-2*t*t+t) + rightTan.InTan[i]*(t*t*t-t*t) + b[i]*(-2*t*t*t+3*t*t)
				} else if anim.Interpolation == components.InterpBezier {
					out[i] = a[i]*u*u*u + 3*leftTan.OutTan[i]*t*u*u + 3*rightTan.InTan[i]*t*t*u + b[i]*t*t*t
				}
			}
		}
	}
	return out
}

func transformBakeUV(uv imath.Vector2, m [6]float64) imath.Vector2 {
	return imath.Vector2{m[0]*uv[0] + m[1]*uv[1] + m[2], m[3]*uv[0] + m[4]*uv[1] + m[5]}
}

func sampleBakeTexture(img *image.NRGBA, uv imath.Vector2, flags uint32) [4]float64 {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	x, y := uv[0]*float64(w)-0.5, uv[1]*float64(h)-0.5
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	xf, yf := x-float64(x0), y-float64(y0)
	address := func(i, size int, wrap bool) int {
		if wrap {
			return (i%size + size) % size
		}
		return min(size-1, max(0, i))
	}
	var result [4]float64
	for yi := range 2 {
		for xi := range 2 {
			weight := [2]float64{1 - xf, xf}[xi] * [2]float64{1 - yf, yf}[yi]
			at := address(y0+yi, h, flags&2 != 0)*img.Stride + address(x0+xi, w, flags&1 != 0)*4
			for channel := range 4 {
				result[channel] += float64(img.Pix[at+channel]) / 255 * weight
			}
		}
	}
	return result
}

func combineBakeSamples(shader uint16, a, b [4]float64) [4]float64 {
	op, mod := shader&7, shader&0x70 != 0
	factor := 1.0
	if op == 4 || op == 6 {
		factor = 2
	}
	var color [4]float64
	for c := range 3 {
		color[c] = a[c] * b[c] * factor
	}
	color[3] = 1
	if mod {
		color[3] = a[3]
	}
	if op == 1 || op == 2 || op == 5 || op == 4 {
		color[3] *= b[3] * factor
	}
	if op == 3 || op == 7 {
		for c := range 3 {
			if mod {
				color[c] = a[c] + b[c]
			} else {
				color[c] = a[c] + b[c]*b[3]
			}
		}
		if mod && op == 3 {
			color[3] = a[3] + b[3]
		}
	}
	return color
}

func bakePowerOfTwo(n int) int {
	p := 1
	for p < n {
		p *= 2
	}
	return p
}
func finiteBakeUV(v imath.Vector2) bool {
	return !math.IsInf(v[0], 0) && !math.IsInf(v[1], 0) && !math.IsNaN(v[0]) && !math.IsNaN(v[1])
}

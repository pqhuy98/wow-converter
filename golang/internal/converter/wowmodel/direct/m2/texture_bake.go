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

// Bound temporary shader samples, independently of the final RGBA atlas.
// Each sample carries UVs, barycentrics and a source face (88 bytes before map overhead).
const bakeChartPixels = 1024 * 1024

var errUV2BakeUnsupported = errors.New("UV2 baking unavailable")
var errBakeChartLimit = errors.New("shader chart working-memory limit")

func unboundOptionalReplaceableTexture(loader *m2.Loader, index int, resolved ResolvedTextures) bool {
	if index < 0 || index >= len(loader.Textures) || index >= len(loader.TextureTypes) {
		return false
	}
	tex := loader.Textures[index]
	textureType := loader.TextureTypes[index]
	if textureType <= 0 || textureType == 1 || tex.FileDataID != 0 || tex.FileName != "" {
		return false
	}
	if _, found := resolved.ValidTextures[tex.FileDataID]; found {
		return false
	}
	if _, found := resolved.ValidTextures[tex.FileName]; found {
		return false
	}
	_, found := resolved.ValidTextures[fmt.Sprintf("data-%d", textureType)]
	return !found
}

type bakePixel struct {
	uv1, uv2 imath.Vector2
	env      imath.Vector2
	set      bool
	covered  bool
	prepared bool
	face     int
	bary     [3]float64
	color    [6]uint8
}

type bakeChart struct {
	pixels                          map[int]bakePixel
	active                          []int
	x, y, minX, minY, width, height int
}

type bakeChartOptions struct {
	conflict      func(bakePixel, bakePixel) bool
	sourceUV      map[*components.GeosetVertex][2]imath.Vector2
	domain2ByFace []bool
	prepare       func(*bakePixel)
}

// bakeUV2Materials evaluates the WoW fragment combiner before framebuffer
// blending. WC3 layers cannot multiply two independently addressed alpha values.
// Baking in UV space keeps the original mesh, skinning and camera independence.
func bakeUV2Materials(ctx context.Context, cfg config.Config, src FileSource, loader *m2.Loader, skin *m2.Skin, mask []m2export.GeosetMaskEntry, resolved ResolvedTextures, result *ConvertResult) error {
	return bakeM2Materials(ctx, cfg, src, loader, skin, mask, resolved, nil, result)
}

func bakeM2Materials(ctx context.Context, cfg config.Config, src FileSource, loader *m2.Loader, skin *m2.Skin, mask []m2export.GeosetMaskEntry, resolved ResolvedTextures, meta *bundlemeta.File, result *ConvertResult) error {
	geosets, targets := mapBakeSkinGeosets(skin, mask, result.MDL.Geosets)
	unitsPerSection := map[uint16]int{}
	bakedGlobals := map[*components.GlobalSequence]bool{}
	budgets := m2BakeBudgets(loader, skin, geosets)
	remaining, spare := 0, 0
	for _, budget := range budgets {
		if budget > 0 {
			remaining++
		}
	}
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
		if unboundOptionalReplaceableTexture(loader, index, resolved) {
			// WCpp binds an unbound replaceable sampler to a transparent black
			// texture. Keep that behavior for optional shader slots. Type 1 is the
			// deferred character skin and must remain unsupported until baking has
			// access to the selected skin image.
			img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
			decoded[index] = img
			return img, nil
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
		// Reuse space that earlier batches did not need. A native body texture
		// must not strand the painted-surface reserve while detailed effects
		// shrink to a handful of texels. Account at the common reference FPS so
		// explicit FPS/window comparisons retain the same spatial quality.
		share := spare / max(1, remaining)
		program.pixelBudget += share
		spare -= share
		remaining--
		referenceBefore := result.bakeReferencePixels
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
					spare += program.pixelBudget
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
			log.Printf("M2 bake section %d: camera-dependent edge fade uses an all-angle average in Classic", unit.SkinSectionIndex)
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
		allTexturesBound := true
		for sampler := range program.count {
			index := int(loader.TextureCombos[combo+sampler])
			if unboundOptionalReplaceableTexture(loader, index, resolved) {
				allTexturesBound = false
			}
		}
		if allTexturesBound && cfg.TextureBaking.Flipbook() {
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
			texture, err := registerBakeTexture(cfg, result, program.images[1], bakeTextureOptions{independentRGB: true})
			if err != nil {
				return err
			}
			texture.WrapWidth, texture.WrapHeight = program.flags[1]&1 != 0, program.flags[1]&2 != 0
			g.Material.Layers[0].Texture, g.Material.Layers[0].TVertexAnim = texture, program.transforms[1]
			g.Material.Layers[0].FilterMode = components.BlendNone
			factorTexture, err := registerBakeTexture(cfg, result, nativeModulateColor(program.images[0]), bakeTextureOptions{independentRGB: true})
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
			texture, err := registerBakeTexture(cfg, result, nativeImage, bakeTextureOptions{independentRGB: m2BakeRGBIndependent(program.blend, false)})
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
		} else if allTexturesBound && description.pixel == 12 && material.BlendingMode == 0 {
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
		} else if allTexturesBound && unitsPerSection[unit.SkinSectionIndex] == 0 && program.count == 1 && !description.edge && description.coords[0] != coordEnv && material.BlendingMode != 3 && material.BlendingMode != 6 && material.BlendingMode != 7 && (description.pixel == 1 || description.pixel == 0 && material.BlendingMode == 0) {
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
		if didBake && cfg.TextureBaking.Flipbook() {
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
		spare += max(0, program.pixelBudget-(result.bakeReferencePixels-referenceBefore))
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
	prepare           func(*bakePixel)
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

type bakeTextureOptions struct {
	ignoreAlpha    bool
	independentRGB bool
}

func registerBakeTexture(cfg config.Config, result *ConvertResult, atlas *image.NRGBA, options ...bakeTextureOptions) (*components.Texture, error) {
	textureOptions := bakeTextureOptions{}
	if len(options) > 0 {
		textureOptions = options[0]
	}
	if cfg.TextureBaking.ResolutionFactor() == .5 {
		atlas = halfBakeTexture(atlas, textureOptions.independentRGB)
	}
	var encoded bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&encoded, atlas); err != nil {
		return nil, err
	}
	hash := sha256.Sum256(encoded.Bytes())
	if textureOptions.ignoreAlpha {
		// Identical PNGs used by opaque and blended draws must not alias an
		// output file with different alpha semantics.
		hasher := sha256.New()
		hasher.Write(encoded.Bytes())
		hasher.Write([]byte("alpha-unused"))
		copy(hash[:], hasher.Sum(nil))
	}
	stem := result.BakeStem
	if stem == "" {
		stem = BakeStemFromListfile(result.MDL.Model.Name)
	}
	rel := fmt.Sprintf("baked/uv2/%s_%x.png", stem, hash[:6])
	for _, tex := range result.MDL.Textures {
		if tex.WowData.PngPath == rel {
			return tex, nil
		}
	}
	texturesource.Register(rel, texturesource.Source{Kind: texturesource.KindPNG, PNG: encoded.Bytes(), PreserveAlpha: true, IgnoreAlpha: textureOptions.ignoreAlpha})
	result.TexturePaths[rel] = struct{}{}
	tex := &components.Texture{Image: filepath.ToSlash(filepath.Join(cfg.AssetPrefix, strings.TrimSuffix(rel, ".png")+".blp")), WowData: components.TextureWowData{Type: -1, PngPath: rel}}
	result.MDL.Textures = append(result.MDL.Textures, tex)
	return tex, nil
}

// BakeStemFromListfile is the WoW file basename used in baked texture paths.
func BakeStemFromListfile(fileName string) string {
	stem := strings.ToLower(stripModelExt(filepath.Base(strings.ReplaceAll(fileName, "\\", "/"))))
	if stem == "" || stem == "." {
		return "baked"
	}
	return stem
}

// mapBakeSkinGeosets pairs skin sections with assembled geosets. OBJ assemble
// only emits groups that have faces, so empty checked placeholders must not
// consume a geoset slot or baking writes into the next real mesh.
func mapBakeSkinGeosets(skin *m2.Skin, mask []m2export.GeosetMaskEntry, mdlGeosets []*components.Geoset) (mapped, targets map[int]*components.Geoset) {
	mapped = map[int]*components.Geoset{}
	targets = map[int]*components.Geoset{}
	if skin == nil {
		return mapped, targets
	}
	gi := 0
	for si, section := range skin.SubMeshes {
		if mask != nil && (si >= len(mask) || !mask[si].Checked) {
			continue
		}
		if section.TriangleCount < 3 {
			continue
		}
		if gi < len(mdlGeosets) {
			mapped[si] = mdlGeosets[gi]
			targets[si] = mdlGeosets[gi]
		}
		gi++
	}
	return mapped, targets
}

func m2BakeBlend(mode uint16) components.BlendMode {
	return [8]components.BlendMode{components.BlendNone, components.BlendTransparent, components.BlendBlend, components.BlendAdditive, components.BlendAddAlpha, components.BlendModulate, components.BlendModulate2x, components.BlendBlend}[min(7, int(mode))]
}

func m2BakeRGBIndependent(blend uint16, screen bool) bool {
	return blend == 0 || blend == 5 || blend == 6 || screen
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
	if !cfg.TextureBaking.Flipbook() {
		plan = stillBakeTimeline()
	}
	frames := len(plan.moments)
	minUV, maxUV := imath.Vector2{math.Inf(1), math.Inf(1)}, imath.Vector2{math.Inf(-1), math.Inf(-1)}
	minUV2, maxUV2 := minUV, maxUV
	for _, f := range g.Faces {
		for _, v := range f.Vertices {
			for axis := range 2 {
				minUV[axis] = min(minUV[axis], v.TexPosition[axis])
				maxUV[axis] = max(maxUV[axis], v.TexPosition[axis])
				uv2 := v.TexPosition
				if v.TexPosition2 != nil {
					uv2 = *v.TexPosition2
				}
				minUV2[axis] = min(minUV2[axis], uv2[axis])
				maxUV2[axis] = max(maxUV2[axis], uv2[axis])
			}
		}
	}
	span := imath.Vector2{maxUV[0] - minUV[0], maxUV[1] - minUV[1]}
	span2 := imath.Vector2{maxUV2[0] - minUV2[0], maxUV2[1] - minUV2[1]}
	// Triangle coverage detects compressed gradient swatches even when a few
	// outlying UVs make their overall bounding box look like a full unwrap.
	domain2 := span2[0] > 1e-9 && span2[1] > 1e-9 && (span[0] <= 1e-9 || span[1] <= 1e-9 || bakePreferUV2(g, program))
	domain2ByFace := bakeDomainsByFace(g, program, domain2)
	mixedDomains := false
	for _, faceDomain2 := range domain2ByFace {
		if faceDomain2 != domain2 {
			mixedDomains = true
			break
		}
	}
	if mixedDomains {
		minUV, maxUV = imath.Vector2{math.Inf(1), math.Inf(1)}, imath.Vector2{math.Inf(-1), math.Inf(-1)}
		for fi, f := range g.Faces {
			for _, v := range f.Vertices {
				uv := v.TexPosition
				if domain2ByFace[fi] && v.TexPosition2 != nil {
					uv = *v.TexPosition2
				}
				for axis := range 2 {
					minUV[axis] = min(minUV[axis], uv[axis])
					maxUV[axis] = max(maxUV[axis], uv[axis])
				}
			}
		}
		span = imath.Vector2{maxUV[0] - minUV[0], maxUV[1] - minUV[1]}
	} else if domain2 {
		minUV, maxUV, span = minUV2, maxUV2, span2
	}
	chartOptions := bakeChartOptions{conflict: program.conflict, prepare: program.prepare}
	if mixedDomains {
		chartOptions.domain2ByFace = domain2ByFace
	}
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
			uv2 := v.TexPosition
			if v.TexPosition2 != nil {
				uv2 = *v.TexPosition2
			}
			chartOptions.sourceUV[v] = [2]imath.Vector2{v.TexPosition, uv2}
			v.TexPosition = imath.Vector2{v.Position[axes[0]], v.Position[axes[1]]}
		}
		minUV = imath.Vector2{boundsMin[axes[0]], boundsMin[axes[1]]}
		maxUV = imath.Vector2{boundsMax[axes[0]], boundsMax[axes[1]]}
		span = imath.Vector2{maxUV[0] - minUV[0], maxUV[1] - minUV[1]}
		domain2 = false
		domain2ByFace = nil
		chartOptions.domain2ByFace = nil
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
	if mixedDomains {
		otherW, otherH := bakeRasterSize(span, program, !domain2, limit)
		w, h = max(w, otherW), max(h, otherH)
	}
	constantUV2 := [2]bool{}
	other := 1
	if domain2 {
		other = 0
	}
	if program.count == 2 && program.shader.coords[0] == coordT1M0 && program.shader.coords[1] == coordT2M1 && (transforms[other] == nil || transforms[other].Rotation == nil) {
		constantUV2 = bakeConstantAxes(program.images[other])
	}
	var env map[*components.GeosetVertex]imath.Vector2
	if usesEnv {
		log.Printf("M2 bake %s: environment combiner uses bind-normal reference projection (camera dependent)", g.Name)
		env = map[*components.GeosetVertex]imath.Vector2{}
		for _, v := range g.Vertices {
			env[v] = m2SphereCoord(imath.Vector3{0, 0, -1}, imath.Vector3{v.Normal[1], v.Normal[2], v.Normal[0]})
		}
	}
	var charts []bakeChart
	var faceCharts []int
	var frameW, frameH int
	shrinkRaster := func() {
		// Raster grids need not be powers of two; only the packed atlas does.
		// Halve area uniformly instead of halving one surface axis at a time.
		scale := math.Sqrt(.5)
		w = max(16, int(float64(w)*scale))
		h = max(16, int(float64(h)*scale))
	}
	buildCharts := func() error {
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			var err error
			charts, faceCharts, err = buildBakeCharts(g, minUV, span, w, h, constantUV2, domain2, env, chartOptions)
			if err == nil {
				frameW, frameH, err = packBakeCharts(charts, w, h)
			}
			if err == nil {
				return nil
			}
			charts, faceCharts = nil, nil
			if !errors.Is(err, errBakeChartLimit) || w <= 16 && h <= 16 {
				return fmt.Errorf("%w: %s at %dx%d: %v", errUV2BakeUnsupported, g.Name, w, h, err)
			}
			shrinkRaster()
		}
	}
	if err := buildCharts(); err != nil {
		return err
	}
	frameCols := bakePowerOfTwo(int(math.Ceil(math.Sqrt(float64(frames)))))
	frameRows := bakePowerOfTwo((frames + frameCols - 1) / frameCols)
	// Balance the complete atlas, including rectangular tiles and frame grids.
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
		reduced := plan
		if w <= 256 && h <= 256 && (frames > 4 || w <= 16 && h <= 16) {
			reduced = reduceBakeTimeline(plan)
		}
		if len(reduced.moments) < frames {
			plan = reduced
			frames = len(plan.moments)
			frameCols = bakePowerOfTwo(int(math.Ceil(math.Sqrt(float64(frames)))))
			frameRows = bakePowerOfTwo((frames + frameCols - 1) / frameCols)
		} else if w <= 16 && h <= 16 {
			break // Every local sequence needs at least one material state.
		} else {
			// Reduction can reach one frame per sequence while frames is still
			// greater than four. Shrink space then; retrying the same timeline
			// would loop forever. Temporal changes do not require new charts.
			shrinkRaster()
			if err := buildCharts(); err != nil {
				return err
			}
		}
		width, height = frameW*frameCols, frameH*frameRows
	}
	result.bakeReferencePixels += width * height * passes
	// Explicit requests keep the requested sampling rate. Choose spatial detail
	// with the compact reference above, then grow/page the atlas in time. This
	// keeps FPS comparisons at identical spatial quality instead of silently
	// dropping frames or blurring the higher-FPS variants further.
	if cfg.TextureBaking.Flipbook() && (cfg.TextureBaking.FPS > 0 || cfg.TextureBaking.WindowMS > 0) {
		plan = makeBakeTimeline(transforms, program.weights, program.shader.pixel, result.MDL.Sequences, cfg.TextureBaking)
		frames = len(plan.moments)
		frameCols = bakePowerOfTwo(int(math.Ceil(math.Sqrt(float64(frames)))))
		frameRows = bakePowerOfTwo((frames + frameCols - 1) / frameCols)
	}
	if frameW > 4096 || frameH > 4096 {
		return fmt.Errorf("%w: one frame is %dx%d", errUV2BakeUnsupported, frameW, frameH)
	}
	// Only the final texture needs power-of-two dimensions. Repeating each
	// frame's rounded rectangle repeats its empty margin too. Retain every
	// chart sample and gutter, then choose the cheapest uniform page layout.
	// Spatial budgeting above deliberately uses the old reference dimensions.
	layout := planBakePages(charts, frames)
	frameW, frameH = layout.cellW, layout.cellH
	frameCols = layout.cols
	pageFrames := layout.capacity()
	width, height = layout.width, layout.height
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
		tex, err := registerBakeTexture(cfg, result, atlas, bakeTextureOptions{
			ignoreAlpha:    program.blend == 0 && !program.screen,
			independentRGB: m2BakeRGBIndependent(program.blend, program.screen),
		})
		if err != nil {
			return err
		}
		pages = append(pages, tex)
		if source, ok := texturesource.Get(tex.WowData.PngPath); ok {
			textureBytes += len(source.PNG)
		}
		if emissionAtlas != nil {
			tex, err = registerBakeTexture(cfg, result, compactBakeRadiance(emissionAtlas))
			if err != nil {
				return err
			}
			emissionPages = append(emissionPages, tex)
		}
		if radianceAtlas != nil {
			tex, err = registerBakeTexture(cfg, result, compactBakeRadiance(radianceAtlas))
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
				if program.shader.edge {
					fragment = averageM2EdgeFade(fragment, program.blend)
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
			anim.Translation.KeyFrames[t] = layout.translation(frame)
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
	type bakeVertexKey struct {
		vertex  *components.GeosetVertex
		domain2 bool
	}
	vertices := make([]map[bakeVertexKey]*components.GeosetVertex, len(charts))
	for ci := range vertices {
		vertices[ci] = map[bakeVertexKey]*components.GeosetVertex{}
	}
	g.Vertices = nil
	for fi := range g.Faces {
		ci := faceCharts[fi]
		faceDomain2 := domain2
		if len(domain2ByFace) > fi {
			faceDomain2 = domain2ByFace[fi]
		}
		for vi, original := range g.Faces[fi].Vertices {
			key := bakeVertexKey{vertex: original, domain2: faceDomain2}
			v := vertices[ci][key]
			if v == nil {
				copyVertex := *original
				uv := original.TexPosition
				if faceDomain2 && original.TexPosition2 != nil {
					uv = *original.TexPosition2
				}
				copyVertex.TexPosition = imath.Vector2{
					(float64(charts[ci].x-charts[ci].minX+bakePadding) + (uv[0]-minUV[0])/span[0]*float64(w-2*bakePadding)) / float64(width),
					(float64(charts[ci].y-charts[ci].minY+bakePadding) + (uv[1]-minUV[1])/span[1]*float64(h-2*bakePadding)) / float64(height),
				}
				copyVertex.TexPosition2 = nil
				v = &copyVertex
				vertices[ci][key] = v
				g.Vertices = append(g.Vertices, v)
			}
			g.Faces[fi].Vertices[vi] = v
		}
	}
	log.Printf("UV2 bake %s: %d charts on %dx%d grid, %d frames/%d ms, %d pages of %dx%d, %d PNG bytes", g.Name, len(charts), w, h, frames, plan.period, len(pages), width, height, textureBytes)
	return nil
}

func bakePreferUV2(g *components.Geoset, p bakeProgram) bool {
	weight := bakeUVTexelWeights(p)
	area := [2]float64{}
	for _, f := range g.Faces {
		area[0] += bakeFaceUVArea(f, false)
		area[1] += bakeFaceUVArea(f, true)
	}
	return area[1]*weight[1] > area[0]*weight[0]*1.01
}

func bakeUVTexelWeights(p bakeProgram) [2]float64 {
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
	return weight
}

func bakeFaceUVArea(face components.Face, domain2 bool) float64 {
	var uv [3]imath.Vector2
	for i, v := range face.Vertices {
		uv[i] = v.TexPosition
		if domain2 && v.TexPosition2 != nil {
			uv[i] = *v.TexPosition2
		}
	}
	return math.Abs((uv[1][0]-uv[0][0])*(uv[2][1]-uv[0][1]) - (uv[1][1]-uv[0][1])*(uv[2][0]-uv[0][0]))
}

func bakeDomainsByFace(g *components.Geoset, p bakeProgram, preferUV2 bool) []bool {
	weights := bakeUVTexelWeights(p)
	domains := make([]bool, len(g.Faces))
	for i, face := range g.Faces {
		domains[i] = preferUV2
		area1, area2 := bakeFaceUVArea(face, false)*weights[0], bakeFaceUVArea(face, true)*weights[1]
		if area1 > area2*1.01 {
			domains[i] = false
		} else if area2 > area1*1.01 {
			domains[i] = true
		}
	}
	return domains
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

func buildBakeCharts(g *components.Geoset, minUV, span imath.Vector2, w, h int, constantUV2 [2]bool, domain2 bool, env map[*components.GeosetVertex]imath.Vector2, options ...bakeChartOptions) ([]bakeChart, []int, error) {
	opt := bakeChartOptions{}
	if len(options) > 0 {
		opt = options[0]
	}
	// Overlapping UV islands often occupy only a few pixels of a large domain.
	// Keep samples sparse until packing instead of allocating w*h for every chart.
	charts := []bakeChart{{pixels: map[int]bakePixel{}}}
	sampleCount := 0
	faceCharts := make([]int, len(g.Faces))
	// A conflict layer is not necessarily one UV island. Track connectivity
	// while sampling so distant compatible islands can be packed separately.
	parents := make([]int, len(g.Faces))
	for fi := range parents {
		parents[fi] = fi
	}
	find := func(fi int) int {
		for parents[fi] != fi {
			parents[fi] = parents[parents[fi]]
			fi = parents[fi]
		}
		return fi
	}
	join := func(a, b int) {
		a, b = find(a), find(b)
		parents[max(a, b)] = min(a, b)
	}
	type chartVertex struct {
		chart  int
		vertex *components.GeosetVertex
	}
	owners := map[chartVertex]int{}
	occupancy := newBakeChartOccupancy(w, h)
	type pixelWrite struct {
		index int
		pixel bakePixel
	}
	var writes []pixelWrite
	mixedDomains := false
	if len(opt.domain2ByFace) == len(g.Faces) {
		for _, faceDomain2 := range opt.domain2ByFace {
			if faceDomain2 != domain2 {
				mixedDomains = true
				break
			}
		}
	}
	for fi, face := range g.Faces {
		var points [3]imath.Vector2
		var coordinates [3][2]imath.Vector2
		var environment [3]imath.Vector2
		faceDomain2 := domain2
		if len(opt.domain2ByFace) == len(g.Faces) {
			faceDomain2 = opt.domain2ByFace[fi]
		}
		for i, v := range face.Vertices {
			uv2 := v.TexPosition
			if v.TexPosition2 != nil {
				uv2 = *v.TexPosition2
			}
			coordinates[i] = [2]imath.Vector2{v.TexPosition, uv2}
			if original, ok := opt.sourceUV[v]; ok {
				coordinates[i] = original
			}
			environment[i] = env[v]
			uv := v.TexPosition
			if faceDomain2 {
				uv = uv2
			}
			points[i] = imath.Vector2{float64(bakePadding) + (uv[0]-minUV[0])/span[0]*float64(w-2*bakePadding), float64(bakePadding) + (uv[1]-minUV[1])/span[1]*float64(h-2*bakePadding)}
		}
		x0 := max(0, int(math.Floor(min(points[0][0], points[1][0], points[2][0]))))
		x1 := min(w-1, int(math.Ceil(max(points[0][0], points[1][0], points[2][0]))))
		y0 := max(0, int(math.Floor(min(points[0][1], points[1][1], points[2][1]))))
		y1 := min(h-1, int(math.Ceil(max(points[0][1], points[1][1], points[2][1]))))
		writes = writes[:0]
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
				for k, coords := range coordinates {
					for axis := range 2 {
						p.uv1[axis] += weights[k] * coords[0][axis]
						p.uv2[axis] += weights[k] * coords[1][axis]
						p.env[axis] += weights[k] * environment[k][axis]
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
		generation := occupancy.beginFace(len(charts))
		for wi, write := range writes {
			for node := occupancy.headAt(write.index); node != 0; node = occupancy.nodes[node-1].next {
				ci := occupancy.nodes[node-1].chart
				occupancy.addCandidate(ci, wi, generation)
			}
		}
		ci := len(charts)
		for candidateChart := range len(charts) {
			head := occupancy.candidateHead(candidateChart, generation)
			if head == 0 {
				// No occupied sample can conflict with this face, so the first
				// chart without candidates is immediately compatible.
				ci = candidateChart
				break
			}
			conflict := false
			for candidate := head; candidate != 0; candidate = occupancy.candidateWriteNext[candidate-1].next {
				write := &writes[occupancy.candidateWriteNext[candidate-1].write]
				old := charts[candidateChart].pixels[write.index]
				// Conservative samples still represent a surface. Unrelated tiny
				// islands must not overwrite each other when neither covers a texel
				// centre. Adjacent faces may share their conservative edge samples.
				checkConflict := old.set && old.covered && write.pixel.covered
				if old.set && !checkConflict {
					shared := 0
					for _, a := range g.Faces[old.face].Vertices {
						for _, b := range face.Vertices {
							if a == b {
								shared++
							}
						}
					}
					checkConflict = shared < 2
				}
				a, b := old.uv2, write.pixel.uv2
				uvMismatch := math.Abs(a[0]-b[0]) > 1e-4 || math.Abs(a[1]-b[1]) > 1e-4
				if mixedDomains {
					uvMismatch = uvMismatch || math.Abs(old.uv1[0]-write.pixel.uv1[0]) > 1e-4 || math.Abs(old.uv1[1]-write.pixel.uv1[1]) > 1e-4
				} else if domain2 {
					a, b = old.uv1, write.pixel.uv1
					uvMismatch = math.Abs(a[0]-b[0]) > 1e-4 || math.Abs(a[1]-b[1]) > 1e-4
				}
				if checkConflict && opt.conflict == nil && (uvMismatch || math.Abs(old.env[0]-write.pixel.env[0]) > 1e-4 || math.Abs(old.env[1]-write.pixel.env[1]) > 1e-4) {
					conflict = true
					break
				}
				if checkConflict && opt.conflict != nil {
					// Most samples never overlap. Shade only an actual comparison,
					// then reuse its color when other faces visit the same sample.
					if opt.prepare != nil {
						if !old.prepared {
							opt.prepare(&old)
							old.prepared = true
							charts[candidateChart].pixels[write.index] = old
						}
						if !write.pixel.prepared {
							opt.prepare(&write.pixel)
							write.pixel.prepared = true
						}
					}
					if opt.conflict(old, write.pixel) {
						conflict = true
						break
					}
				}
			}
			if !conflict {
				ci = candidateChart
				break
			}
		}
		if ci == len(charts) {
			charts = append(charts, bakeChart{pixels: map[int]bakePixel{}})
		}
		for _, write := range writes {
			old := charts[ci].pixels[write.index]
			if old.set {
				join(fi, old.face) // Compatible overlapping islands may reuse samples.
			}
			if !old.set || write.pixel.covered {
				if !old.set {
					if sampleCount == bakeChartPixels {
						return nil, nil, errBakeChartLimit
					}
					sampleCount++
					occupancy.add(write.index, ci)
				}
				charts[ci].pixels[write.index] = write.pixel
			}
		}
		for _, vertex := range face.Vertices {
			key := chartVertex{ci, vertex}
			if owner, ok := owners[key]; ok {
				join(fi, owner)
			} else {
				owners[key] = fi
			}
		}
		faceCharts[fi] = ci
	}
	if len(g.Faces) == 0 {
		return charts, faceCharts, nil
	}
	islands := []bakeChart{}
	rootCharts := map[int]int{}
	for fi := range g.Faces {
		root := find(fi)
		ci, ok := rootCharts[root]
		if !ok {
			ci = len(islands)
			rootCharts[root] = ci
			islands = append(islands, bakeChart{pixels: map[int]bakePixel{}})
		}
		faceCharts[fi] = ci
	}
	for ci := range charts {
		for index, pixel := range charts[ci].pixels {
			islands[faceCharts[pixel.face]].pixels[index] = pixel
		}
		charts[ci].pixels = nil
	}
	return islands, faceCharts, nil
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
func packBakeCharts(charts []bakeChart, w, h int) (int, int, error) {
	area, widest := 0, 1
	order := make([]int, len(charts))
	sampleCount := 0
	for _, chart := range charts {
		sampleCount += len(chart.pixels)
	}
	if sampleCount > bakeChartPixels {
		return 0, 0, errBakeChartLimit
	}
	for ci := range charts {
		chart := &charts[ci]
		previousCount := len(chart.pixels)
		if err := padBakeChart(chart.pixels, w, h, bakeChartPixels-sampleCount+previousCount); err != nil {
			return 0, 0, err
		}
		sampleCount += len(chart.pixels) - previousCount
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
		sort.Ints(chart.active)
		if x1 < 0 {
			x0, y0, x1, y1 = 0, 0, w-1, h-1
		}
		chart.minX, chart.minY, chart.width, chart.height = x0, y0, x1-x0+1, y1-y0+1
		area += chart.width * chart.height
		widest = max(widest, chart.width)
		order[ci] = ci
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := charts[order[i]], charts[order[j]]
		if a.height != b.height {
			return a.height > b.height
		}
		return a.width > b.width
	})
	// Pack padded silhouettes rather than bounding rectangles. Triangle islands
	// can interlock without sharing any texel used by the mesh or its gutter.
	type silhouette struct{ top, bottom []int }
	shapes := make([]silhouette, len(charts))
	for ci, chart := range charts {
		s := silhouette{make([]int, chart.width), make([]int, chart.width)}
		for x := range s.top {
			s.top[x], s.bottom[x] = h, -1
		}
		for _, pi := range chart.active {
			x, y := pi%w-chart.minX, pi/w-chart.minY
			s.top[x], s.bottom[x] = min(s.top[x], y), max(s.bottom[x], y)
		}
		shapes[ci] = s
	}
	bestW, bestH, bestArea := 0, 0, math.MaxInt
	positions, bestPositions := make([][2]int, len(charts)), make([][2]int, len(charts))
	firstWidth := bakePowerOfTwo(widest)
	lastWidth := max(firstWidth, min(4096, bakePowerOfTwo(int(math.Ceil(math.Sqrt(float64(area)))))*2))
	for width := firstWidth; width <= lastWidth; width *= 2 {
		skyline := make([]int, width)
		maxX, maxY := 0, 0
		for _, ci := range order {
			chart, shape := charts[ci], shapes[ci]
			bestX, bestY := 0, math.MaxInt
			for x := 0; x+chart.width <= width; x++ {
				y := 0
				for col, top := range shape.top {
					if shape.bottom[col] >= 0 {
						y = max(y, skyline[x+col]-top)
					}
					if y >= bestY {
						break
					}
				}
				if y < bestY {
					bestX, bestY = x, y
				}
				if bestY == 0 {
					break
				}
			}
			positions[ci] = [2]int{bestX, bestY}
			for col, bottom := range shape.bottom {
				if bottom >= 0 {
					skyline[bestX+col] = max(skyline[bestX+col], bestY+bottom+1)
				}
			}
			maxX, maxY = max(maxX, bestX+chart.width), max(maxY, bestY+chart.height)
		}
		packedW, packedH := bakePowerOfTwo(maxX), bakePowerOfTwo(maxY)
		if packedW*packedH < bestArea || packedW*packedH == bestArea && max(packedW, packedH) < max(bestW, bestH) {
			bestW, bestH, bestArea = packedW, packedH, packedW*packedH
			copy(bestPositions, positions)
		}
	}
	for ci := range charts {
		charts[ci].x, charts[ci].y = bestPositions[ci][0], bestPositions[ci][1]
	}
	return bestW, bestH, nil
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

func padBakeChart(pixels map[int]bakePixel, w, h, limit int) error {
	// Extend sample coordinates beyond triangle edges, avoiding dark filtering
	// seams while retaining the shader's straight (unpremultiplied) RGB.
	var frontier []int
	for wave := range bakePadding {
		next := map[int]bakePixel{}
		directions := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
		visit := func(pi int) error {
			x, y := pi%w, pi/w
			for _, d := range directions {
				x2, y2 := x+d[0], y+d[1]
				at := y2*w + x2
				if x2 < 0 || x2 >= w || y2 < 0 || y2 >= h || pixels[at].set || next[at].set {
					continue
				}
				if len(pixels)+len(next) == limit {
					return errBakeChartLimit
				}
				// Choose the same left/right/up/down neighbor as the dense raster,
				// regardless of map iteration order. Read only the previous wave.
				for _, neighbor := range directions {
					nx, ny := x2+neighbor[0], y2+neighbor[1]
					if nx >= 0 && nx < w && ny >= 0 && ny < h {
						if sample := pixels[ny*w+nx]; sample.set {
							next[at] = sample
							break
						}
					}
				}
			}
			return nil
		}
		if wave == 0 {
			for pi := range pixels {
				if err := visit(pi); err != nil {
					return err
				}
			}
		} else {
			// Earlier samples already have all four neighbors filled. Only the
			// previous wave's new boundary can expose another empty texel.
			for _, pi := range frontier {
				if err := visit(pi); err != nil {
					return err
				}
			}
		}
		frontier = frontier[:0]
		for pi, p := range next {
			pixels[pi] = p
			frontier = append(frontier, pi)
		}
	}
	return nil
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

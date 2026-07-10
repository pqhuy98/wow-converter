package utils

import (
	"log"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

// WowToWc3Interpolation maps M2 interpolation to MDL.
func WowToWc3Interpolation(wowInterpolation uint16) components.Interpolation {
	switch wowInterpolation {
	case 0:
		return components.InterpDontInterp
	case 1:
		return components.InterpLinear
	default:
		return components.InterpLinear
	}
}

func m2BlendModeToWc3FilterMode(m2BlendMode uint16) components.BlendMode {
	switch m2BlendMode {
	case 0:
		return components.BlendNone
	case 1:
		return components.BlendTransparent
	case 2, 3:
		return components.BlendBlend
	case 4, 7:
		return components.BlendAdditive
	case 5:
		return components.BlendModulate
	case 6:
		return components.BlendModulate2x
	default:
		return components.BlendBlend
	}
}

// GetLayerFilterMode maps M2 shader combiner to WC3 per-layer filter.
func GetLayerFilterMode(blendingMode uint16, shaderID uint16, layerIndex int, textureImage string) *components.BlendMode {
	if layerIndex == 0 {
		mode := m2BlendModeToWc3FilterMode(blendingMode)
		return &mode
	}
	opaquePath := (shaderID & 0x70) == 0
	op := shaderID & 7
	if opaquePath {
		if strings.Contains(textureImage, "reflect") || strings.Contains(textureImage, "shine") {
			return nil
		}
		if op == 0 {
			return nil
		}
		if op == 3 {
			texturePath := strings.ReplaceAll(strings.ReplaceAll(textureImage, ".png", ""), ".blp", "")
			if strings.Contains(texturePath, "glow") {
				mode := components.BlendAdditive
				return &mode
			}
			return nil
		}
		mode := components.BlendAdditive
		return &mode
	}
	return nil
}

// WmoBlendModeToWc3FilterMode maps WMO EGxBlendEnum to WC3 filter mode.
func WmoBlendModeToWc3FilterMode(wmoBlendMode uint16) components.BlendMode {
	switch wmoBlendMode {
	case 0:
		return components.BlendNone
	case 1:
		return components.BlendTransparent
	case 2:
		return components.BlendBlend
	case 3:
		return components.BlendAdditive
	case 4:
		return components.BlendModulate
	case 5:
		return components.BlendModulate2x
	case 6, 7, 10, 12, 13:
		return components.BlendAdditive
	case 8, 9, 11:
		return components.BlendBlend
	default:
		return components.BlendBlend
	}
}

// GuessFilterMode guesses filter mode from texture path when metadata is unavailable.
func GuessFilterMode(texturePath string) components.BlendMode {
	log.Printf("%s %s", ansi.Red("Warning: guessFilterMode"), texturePath)

	lower := strings.ToLower(strings.ReplaceAll(texturePath, "\\", "/"))
	noneFilterPatterns := []string{
		"textures/walls",
		"textures/trim",
		"textures/floor",
	}
	transparentFilterPatterns := []string{
		"bush", "_bush", "branch", "_branch", "tree", "_tree", "treetall", "_vfx_fire_",
		"vines", "treebranch", "floornets", "spells/", "environment/doodad/", "gate10.",
		"interface/glue", "fence", "haypiles", "plant", "alpha", "ash04", "glow",
		"elwynnmiscrope03", "textures/decoration", "_glow", "jlo_worc_chainsr", "hay/",
		"sc_brazier", "hangnets", "flare05", "lightbeam", "jlo_worc_grate", "sc_chain",
	}
	additiveFilterPatterns := []string{
		"genericglow", "swordinice", "_fog_", "icecrown_rays", "blueglow", "treeweb01", "_web",
	}

	for _, pattern := range noneFilterPatterns {
		if strings.Contains(lower, pattern) {
			return components.BlendNone
		}
	}
	for _, pattern := range additiveFilterPatterns {
		if strings.Contains(lower, pattern) {
			return components.BlendAdditive
		}
	}
	for _, pattern := range transparentFilterPatterns {
		if strings.Contains(lower, pattern) {
			return components.BlendTransparent
		}
	}
	return components.BlendNone
}

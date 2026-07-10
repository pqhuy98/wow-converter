package icon

import (
	"path/filepath"
	"strings"
)

// MergeOptions applies TS-equivalent defaults.
func MergeOptions(opts ConversionOptions) MergedOptions {
	out := MergedOptions{
		Size:       opts.Size,
		Style:      opts.Style,
		Frame:      opts.Frame,
		ResizeMode: opts.ResizeMode,
	}
	if out.Style == "" {
		out.Style = StyleClassicHD20
	}
	if out.Frame == "" {
		out.Frame = FrameNone
	}
	if out.ResizeMode == "" {
		out.ResizeMode = ResizeNormal
	}
	if opts.Extras != nil {
		out.Extras = *opts.Extras
	}
	return out
}

type customFrameEntry struct {
	Pos  [2]int
	Size [2]int
}

var customFrameData = map[Frame]map[Size]map[Style]customFrameEntry{
	FrameAtt: {
		Size64:  {StyleClassicSD: {Pos: [2]int{4, 4}, Size: [2]int{48, 48}}, StyleReforgedHD: {Pos: [2]int{2, 2}, Size: [2]int{51, 51}}, StyleClassicHD20: {Pos: [2]int{4, 4}, Size: [2]int{48, 48}}},
		Size128: {StyleClassicSD: {Pos: [2]int{8, 8}, Size: [2]int{96, 96}}, StyleReforgedHD: {Pos: [2]int{5, 5}, Size: [2]int{101, 101}}, StyleClassicHD20: {Pos: [2]int{8, 8}, Size: [2]int{96, 96}}},
		Size256: {StyleClassicSD: {Pos: [2]int{16, 16}, Size: [2]int{192, 192}}, StyleReforgedHD: {Pos: [2]int{10, 10}, Size: [2]int{202, 202}}, StyleClassicHD20: {Pos: [2]int{16, 16}, Size: [2]int{192, 192}}},
	},
	FrameUpg: {
		Size64:  {StyleClassicSD: {Pos: [2]int{4, 4}, Size: [2]int{48, 48}}, StyleReforgedHD: {Pos: [2]int{2, 2}, Size: [2]int{51, 51}}, StyleClassicHD20: {Pos: [2]int{4, 4}, Size: [2]int{48, 48}}},
		Size128: {StyleClassicSD: {Pos: [2]int{8, 8}, Size: [2]int{96, 96}}, StyleReforgedHD: {Pos: [2]int{5, 5}, Size: [2]int{101, 101}}, StyleClassicHD20: {Pos: [2]int{8, 8}, Size: [2]int{96, 96}}},
		Size256: {StyleClassicSD: {Pos: [2]int{16, 16}, Size: [2]int{192, 192}}, StyleReforgedHD: {Pos: [2]int{10, 10}, Size: [2]int{202, 202}}, StyleClassicHD20: {Pos: [2]int{16, 16}, Size: [2]int{192, 192}}},
	},
	FrameSSH: {
		Size64:  {StyleClassicSD: {Pos: [2]int{2, 16}, Size: [2]int{32, 32}}, StyleReforgedHD: {Pos: [2]int{2, 16}, Size: [2]int{32, 32}}, StyleClassicHD20: {Pos: [2]int{2, 16}, Size: [2]int{32, 32}}},
		Size128: {StyleClassicSD: {Pos: [2]int{4, 32}, Size: [2]int{64, 64}}, StyleReforgedHD: {Pos: [2]int{4, 32}, Size: [2]int{64, 64}}, StyleClassicHD20: {Pos: [2]int{4, 32}, Size: [2]int{64, 64}}},
		Size256: {StyleClassicSD: {Pos: [2]int{8, 64}, Size: [2]int{128, 128}}, StyleReforgedHD: {Pos: [2]int{8, 64}, Size: [2]int{128, 128}}, StyleClassicHD20: {Pos: [2]int{8, 64}, Size: [2]int{128, 128}}},
	},
	FrameSSP: {
		Size64:  {StyleClassicSD: {Pos: [2]int{2, 16}, Size: [2]int{32, 32}}, StyleReforgedHD: {Pos: [2]int{2, 16}, Size: [2]int{32, 32}}, StyleClassicHD20: {Pos: [2]int{2, 16}, Size: [2]int{32, 32}}},
		Size128: {StyleClassicSD: {Pos: [2]int{4, 32}, Size: [2]int{64, 64}}, StyleReforgedHD: {Pos: [2]int{4, 32}, Size: [2]int{64, 64}}, StyleClassicHD20: {Pos: [2]int{4, 32}, Size: [2]int{64, 64}}},
		Size256: {StyleClassicSD: {Pos: [2]int{8, 64}, Size: [2]int{128, 128}}, StyleReforgedHD: {Pos: [2]int{8, 64}, Size: [2]int{128, 128}}, StyleClassicHD20: {Pos: [2]int{8, 64}, Size: [2]int{128, 128}}},
	},
}

func getCustomFrameData(frame Frame, size Size, style Style) *customFrameEntry {
	bySize, ok := customFrameData[frame]
	if !ok {
		return nil
	}
	byStyle, ok := bySize[size]
	if !ok {
		return nil
	}
	entry, ok := byStyle[style]
	if !ok {
		return nil
	}
	return &entry
}

// GetWc3Path generates the WC3 output path for an icon.
func GetWc3Path(texturePath string, frame Frame) string {
	filename := texturePath
	if i := strings.LastIndex(filename, "/"); i >= 0 {
		filename = filename[i+1:]
	}
	baseName := filename
	if dot := strings.LastIndex(baseName, "."); dot >= 0 {
		baseName = baseName[:dot]
	}
	switch frame {
	case FrameBtn:
		return "ReplaceableTextures/CommandButtons/BTN" + baseName + ".blp"
	case FrameDisBtn:
		return "ReplaceableTextures/CommandButtonsDisabled/DISBTN" + baseName + ".blp"
	case FramePas:
		return "ReplaceableTextures/PassiveButtons/PAS" + baseName + ".blp"
	case FrameDisPas:
		return "ReplaceableTextures/CommandButtonsDisabled/DISPAS" + baseName + ".blp"
	case FrameAtc:
		return "ReplaceableTextures/CommandButtons/ATC" + baseName + ".blp"
	case FrameDisAtc:
		return "ReplaceableTextures/CommandButtonsDisabled/DISATC" + baseName + ".blp"
	case FrameUpg:
		return "ReplaceableTextures/CommandButtons/UPG" + baseName + ".blp"
	case FrameAtt:
		return "ReplaceableTextures/CommandButtons/ATT" + baseName + ".blp"
	case FrameSSH:
		return "SSH" + baseName + ".blp"
	case FrameSSP:
		return "SSP" + baseName + ".blp"
	default:
		return filename
	}
}

func validateOutputPath(outputPath string, frame Frame) (string, error) {
	trimmed := strings.TrimSpace(outputPath)
	if trimmed == "" {
		return "", errInvalidOutputPath("output path cannot be empty")
	}
	if strings.Contains(trimmed, "..") {
		return "", errInvalidOutputPath("path traversal detected")
	}
	if frame != FrameNone && strings.ContainsAny(trimmed, `/\`) {
		return "", errInvalidOutputPath("path separators are not allowed for this frame type")
	}
	clean := filepath.ToSlash(filepath.Clean(trimmed))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "/") {
		return "", errInvalidOutputPath("absolute path not allowed")
	}
	return GetWc3Path(clean, frame), nil
}

type pathError string

func (e pathError) Error() string           { return "Invalid output path: " + string(e) }
func errInvalidOutputPath(msg string) error { return pathError(msg) }

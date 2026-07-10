package icon

// Size names for icon output dimensions.
type Size string

const (
	Size64   Size = "64x64"
	Size128  Size = "128x128"
	Size256  Size = "256x256"
	SizeOrig Size = "original"
)

// Style names for frame asset folders.
type Style string

const (
	StyleClassicSD   Style = "classic-sd"
	StyleReforgedHD  Style = "reforged-hd"
	StyleClassicHD20 Style = "classic-hd-2.0"
)

// Frame names for WC3 icon overlays.
type Frame string

const (
	FrameBtn    Frame = "btn"
	FrameDisBtn Frame = "disbtn"
	FramePas    Frame = "pas"
	FrameDisPas Frame = "dispas"
	FrameAtc    Frame = "atc"
	FrameDisAtc Frame = "disatc"
	FrameAtt    Frame = "att"
	FrameUpg    Frame = "upg"
	FrameSSH    Frame = "ssh"
	FrameSSP    Frame = "ssp"
	FrameNone   Frame = "none"
)

// ResizeMode controls pre-frame resizing strategy.
type ResizeMode string

const (
	ResizeNormal ResizeMode = "normal"
	ResizeAI     ResizeMode = "ai"
)

// Extras holds optional icon processing flags.
type Extras struct {
	Crop bool `json:"crop"`
}

// ConversionOptions mirrors the TS icon conversion schema.
type ConversionOptions struct {
	Size       Size       `json:"size"`
	Style      Style      `json:"style"`
	Frame      Frame      `json:"frame"`
	Extras     *Extras    `json:"extras,omitempty"`
	ResizeMode ResizeMode `json:"resizeMode,omitempty"`
}

// MergedOptions has defaults applied.
type MergedOptions struct {
	Size       Size
	Style      Style
	Frame      Frame
	Extras     Extras
	ResizeMode ResizeMode
}

// ExportItem is a single icon export request.
type ExportItem struct {
	TexturePath string             `json:"texturePath"`
	Options     *ConversionOptions `json:"options,omitempty"`
	OutputPath  string             `json:"outputPath,omitempty"`
}

var sizeMapping = map[Size]struct{ Width, Height int }{
	Size64:  {64, 64},
	Size128: {128, 128},
	Size256: {256, 256},
}

var styleFolderMap = map[Style]string{
	StyleClassicSD:   "ClassicSD",
	StyleReforgedHD:  "ReforgedHD",
	StyleClassicHD20: "ClassicHD2.0",
}

var frameFileMap = map[Frame]string{
	FrameBtn:    "BTN",
	FrameDisBtn: "DISBTN",
	FramePas:    "PAS",
	FrameDisPas: "DISPAS",
	FrameAtc:    "ATC",
	FrameDisAtc: "DISATC",
	FrameAtt:    "ATT",
	FrameUpg:    "UPG",
	FrameSSH:    "SSH",
	FrameSSP:    "SSP",
	FrameNone:   "NONE",
}

var hdDesaturationFrames = map[Frame]struct{}{
	FrameDisBtn: {},
	FrameDisPas: {},
	FrameDisAtc: {},
}

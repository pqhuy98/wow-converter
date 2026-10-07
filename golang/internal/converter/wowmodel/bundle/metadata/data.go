package metadata

import (
	bundleanim "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

// Data is the typed assembly contract. Direct conversion builds it in memory;
// companion JSON is decoded into it only by Parse. LoadFromData takes an owned
// snapshot so later loader/assembly mutations cannot affect one another.
type Data struct {
	FileType                string                    `json:"fileType"`
	Textures                []Texture                 `json:"textures"`
	Materials               []m2.MaterialEntry        `json:"-"`
	WMOMaterials            []wmo.Material            `json:"-"`
	TextureTypes            []uint32                  `json:"textureTypes"`
	TextureCombos           []uint16                  `json:"textureCombos"`
	TextureTransforms       []TextureTransform        `json:"textureTransforms"`
	TextureTransformsLookup []uint16                  `json:"textureTransformsLookup"`
	TransparencyLookup      []uint16                  `json:"transparencyLookup"`
	TextureWeights          []m2.Track                `json:"textureWeights"`
	GlobalLoops             []uint32                  `json:"globalLoops"`
	M2Animations            []m2.AnimationEntry       `json:"m2Animations"`
	Colors                  []Color                   `json:"colors"`
	Cameras                 []m2.CameraEntry          `json:"cameras"`
	Lights                  []m2.LightEntry           `json:"lights"`
	RibbonEmitters          []m2.RibbonEmitterEntry   `json:"ribbonEmitters"`
	ParticleEmitters        []m2.ParticleEmitterEntry `json:"particleEmitters"`
	Skin                    Skin                      `json:"skin"`
}

// TextureTransform retains nullable timestamps from companion files as well as
// concrete timestamps from native M2 tracks.
type TextureTransform struct {
	Translation Track `json:"translation"`
	Rotation    Track `json:"rotation"`
	Scaling     Track `json:"scaling"`
}

// Color carries native geoset color and visibility tracks.
type Color struct {
	Color Track `json:"color"`
	Alpha Track `json:"alpha"`
}

// Track shares the bone animation contract, including nullable timestamps.
// Its companion-file decoder supplies the legacy no-global-sequence default.
type Track bundleanim.TrackData

// Texture is a resolved source texture used by assembly.
type Texture struct {
	FileNameExternal string `json:"fileNameExternal"`
	MtlName          string `json:"mtlName"`
	Flags            int    `json:"flags"`
	FileDataID       int    `json:"fileDataID"`
}

// Skin is the selected geometry and material layout.
type Skin struct {
	SubMeshes    []SubMesh     `json:"subMeshes"`
	TextureUnits []TextureUnit `json:"textureUnits"`
}

// SubMesh describes the source vertices belonging to an enabled section.
type SubMesh struct {
	Enabled     bool `json:"enabled"`
	SubmeshID   int  `json:"submeshID"`
	VertexStart int  `json:"vertexStart"`
	VertexCount int  `json:"vertexCount"`
}

// TextureUnit links a source draw to its material, texture and animation lookups.
type TextureUnit struct {
	Flags                      int `json:"flags"`
	Priority                   int `json:"priority"`
	ShaderID                   int `json:"shaderID"`
	SkinSectionIndex           int `json:"skinSectionIndex"`
	MaterialIndex              int `json:"materialIndex"`
	TextureCount               int `json:"textureCount"`
	TextureComboIndex          int `json:"textureComboIndex"`
	TextureTransformComboIndex int `json:"textureTransformComboIndex"`
	TextureWeightComboIndex    int `json:"textureWeightComboIndex"`
	ColorIndex                 int `json:"colorIndex"`
}

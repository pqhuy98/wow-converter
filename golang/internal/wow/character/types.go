package character

// CharComponentTextureSection mirrors ChrComponentTextureSections.db2 row fields used by charMeta.
type CharComponentTextureSection struct {
	ID                           *int `json:"ID,omitempty"`
	CharComponentTextureLayoutID *int `json:"CharComponentTextureLayoutID,omitempty"`
	SectionType                  *int `json:"SectionType,omitempty"`
	X                            int  `json:"X"`
	Y                            int  `json:"Y"`
	Width                        int  `json:"Width"`
	Height                       int  `json:"Height"`
	OverlapSectionMask           *int `json:"OverlapSectionMask,omitempty"`
}

// ChrModelMaterialRow mirrors ChrModelMaterial.db2 row fields used by charMeta.
type ChrModelMaterialRow struct {
	ID                            int `json:"ID"`
	CharComponentTextureLayoutsID int `json:"CharComponentTextureLayoutsID"`
	TextureType                   int `json:"TextureType"`
	Width                         int `json:"Width"`
	Height                        int `json:"Height"`
	Flags                         int `json:"Flags"`
	Field90134615006              int `json:"Field_9_0_1_34615_006"`
}

// ChrModelTextureLayerRow mirrors ChrModelTextureLayer.db2 row fields used by charMeta.
type ChrModelTextureLayerRow struct {
	ID                            int   `json:"ID"`
	CharComponentTextureLayoutsID int   `json:"CharComponentTextureLayoutsID"`
	TextureType                   int   `json:"TextureType"`
	Layer                         int   `json:"Layer"`
	Flags                         int   `json:"Flags"`
	BlendMode                     int   `json:"BlendMode"`
	TextureSectionTypeBitMask     int   `json:"TextureSectionTypeBitMask"`
	TextureSectionTypeBitMask2    int   `json:"TextureSectionTypeBitMask2"`
	ChrModelTextureTargetID       []int `json:"ChrModelTextureTargetID"`
	Field90134365006              []int `json:"Field_9_0_1_34365_006"`
}

// ChrCustomizationMaterialEntry mirrors resolved customization material data.
type ChrCustomizationMaterialEntry struct {
	ChrModelTextureTargetID int `json:"ChrModelTextureTargetID"`
	FileDataID              int `json:"FileDataID"`
}

// CharMetaChoiceMaterial is one resolved bake layer for a customization choice.
type CharMetaChoiceMaterial struct {
	CustMaterial ChrCustomizationMaterialEntry `json:"custMaterial"`
	TextureLayer ChrModelTextureLayerRow       `json:"textureLayer"`
	Material     ChrModelMaterialRow           `json:"material"`
	Section      *CharComponentTextureSection  `json:"section"`
	Filename     *string                       `json:"filename"`
}

// ChoiceMeta holds geosets and materials for one customization choice.
type ChoiceMeta struct {
	Geosets   []int                    `json:"geosets"`
	Materials []CharMetaChoiceMaterial `json:"materials"`
}

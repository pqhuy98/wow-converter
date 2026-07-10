package wowhead

import (
	"encoding/json"
	"fmt"
)

// CharacterCustomization is zam character customization meta JSON.
type CharacterCustomization struct {
	Options         []CustomizationOption   `json:"Options"`
	TextureFiles    map[string][]FileEntry  `json:"TextureFiles"`
	Materials       []CustomizationMaterial `json:"Materials"`
	TextureLayers   []TextureLayerEntry     `json:"TextureLayers"`
	TextureSections []TextureSectionEntry   `json:"TextureSections"`
}

// CustomizationOption is one customization category.
type CustomizationOption struct {
	ID         int                   `json:"Id"`
	Name       string                `json:"Name"`
	OrderIndex int                   `json:"OrderIndex"`
	Choices    []CustomizationChoice `json:"Choices"`
}

// CustomizationChoice is one customization choice.
type CustomizationChoice struct {
	ID         int                    `json:"Id"`
	Name       string                 `json:"Name"`
	OrderIndex int                    `json:"OrderIndex"`
	CustReqID  int                    `json:"CustReqId"`
	Elements   []CustomizationElement `json:"Elements"`
}

// CustomizationElement is a customization visual element.
type CustomizationElement struct {
	ID                     int                  `json:"Id"`
	VariationIndex         int                  `json:"VariationIndex"`
	VariationChoiceID      int                  `json:"VariationChoiceID"`
	Geoset                 *ElementGeoset       `json:"Geoset"`
	SkinnedModel           *ElementSkinnedModel `json:"SkinnedModel"`
	Material               *ElementMaterial     `json:"Material"`
	BoneSet                *ElementBoneSet      `json:"BoneSet"`
	CondModelFileDataId    int                  `json:"CondModelFileDataId"`
	ChrCustItemGeoModifyID int                  `json:"ChrCustItemGeoModifyID"`
}

// ElementGeoset is a geoset customization element.
type ElementGeoset struct {
	GeosetType int `json:"GeosetType"`
	GeosetID   int `json:"GeosetID"`
	Modifier   int `json:"Modifier"`
}

// ElementSkinnedModel is a skinned collection element.
type ElementSkinnedModel struct {
	CollectionFileDataID int `json:"CollectionFileDataID"`
	GeosetType           int `json:"GeosetType"`
	GeosetID             int `json:"GeosetID"`
	Modifier             int `json:"Modifier"`
	Flags                int `json:"Flags"`
}

// ElementMaterial is a material customization element.
type ElementMaterial struct {
	TextureTarget       int `json:"TextureTarget"`
	MaterialResourcesID int `json:"MaterialResourcesID"`
}

// ElementBoneSet is a bone set customization element.
type ElementBoneSet struct {
	BoneFileDataID  int `json:"BoneFileDataID"`
	ModelFileDataID int `json:"ModelFileDataID"`
}

// CustomizationMaterial describes a material texture type.
type CustomizationMaterial struct {
	TextureType int `json:"TextureType"`
	Width       int `json:"Width"`
	Height      int `json:"Height"`
	Flags       int `json:"Flags"`
}

// TextureLayerEntry is a texture layer definition.
type TextureLayerEntry struct {
	TextureType             int `json:"TextureType"`
	Layer                   int `json:"Layer"`
	BlendMode               int `json:"BlendMode"`
	ChrModelTextureTargetID int `json:"ChrModelTextureTargetID"`
	TextureSection          int `json:"TextureSection"`
}

// TextureSectionEntry is a texture section rectangle.
type TextureSectionEntry struct {
	SectionType int     `json:"SectionType"`
	X           float64 `json:"X"`
	Y           float64 `json:"Y"`
	Width       float64 `json:"Width"`
	Height      float64 `json:"Height"`
}

// FetchCharacterCustomization fetches chr customization meta for a model id.
func FetchCharacterCustomization(client *HTTPClient, expansion Expansion, chrModelID int) (CharacterCustomization, error) {
	path := fmt.Sprintf("meta/charactercustomization/%d.json", chrModelID)
	if expansion == ExpansionLatestAvailable {
		var err error
		expansion, err = GetLatestExpansionHavingURL(client, path)
		if err != nil {
			return CharacterCustomization{}, err
		}
	}
	url := GetZamBaseURL(expansion) + "/" + path
	text, err := FetchWithCache(client, url)
	if err != nil {
		fallback, err2 := GetLatestExpansionHavingURL(client, path)
		if err2 != nil {
			return CharacterCustomization{}, err
		}
		url = GetZamBaseURL(fallback) + "/" + path
		text, err = FetchWithCache(client, url)
		if err != nil {
			return CharacterCustomization{}, err
		}
	}
	var data CharacterCustomization
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return CharacterCustomization{}, err
	}
	return data, nil
}

package adt

// PlacementRow is one model placement (ADT tile CSV row or WMO interior row).
type PlacementRow struct {
	ModelFile   string `json:"modelFile"`
	PositionX   string `json:"positionX"`
	PositionY   string `json:"positionY"`
	PositionZ   string `json:"positionZ"`
	RotationX   string `json:"rotationX"`
	RotationY   string `json:"rotationY"`
	RotationZ   string `json:"rotationZ"`
	RotationW   string `json:"rotationW"`
	ScaleFactor string `json:"scaleFactor"`
	ModelId     string `json:"modelId"`
	Type        string `json:"type"`
	FileDataID  string `json:"fileDataID"`
}

// BakedTexture is a terrain texture PNG kept in memory for WC3 conversion.
type BakedTexture struct {
	RelPath string `json:"relPath"`
	PNG     []byte `json:"png"`
}

// ConversionOutput holds in-memory ADT export artifacts for map conversion.
type ConversionOutput struct {
	ExportAssetDir string                       `json:"exportAssetDir"`
	TileX          int                          `json:"tileX"`
	TileY          int                          `json:"tileY"`
	ObjectPath     string                       `json:"objectPath"`
	ObjFilePath    string                       `json:"objFilePath"`
	ObjText        string                       `json:"objText"`
	MtlText        string                       `json:"mtlText"`
	Textures       []BakedTexture               `json:"textures"`
	Placements     []PlacementRow               `json:"placements"`
	WmoPlacements  map[string][]PlacementRow    `json:"wmoPlacements"`
}

package casc

// NpcDisplayGeoset is a creature geoset selection from DB2 tables.
type NpcDisplayGeoset struct {
	GeosetIndex int `json:"geosetIndex"`
	GeosetValue int `json:"geosetValue"`
}

// NpcDisplayMeta is creature display metadata resolved from CASC DB2 tables.
type NpcDisplayMeta struct {
	Found    bool               `json:"found"`
	Model    int                `json:"model,omitempty"`
	Textures map[string]int     `json:"textures,omitempty"`
	Geosets  []NpcDisplayGeoset `json:"geosets,omitempty"`
}

package server

// SettableConfigKeys are the only ReaderConfig fields writable via /rest/setConfig.
// Mirrors client.DesiredConfig — export-shaping knobs the converter syncs before export.
var SettableConfigKeys = map[string]struct{}{
	"copyMode":                   {},
	"listfileShowFileDataIDs":    {},
	"enableM2Skins":              {},
	"enableSharedTextures":       {},
	"enableSharedChildren":       {},
	"enableAbsoluteMTLPaths":     {},
	"enableAbsoluteCSVPaths":     {},
	"removePathSpaces":           {},
	"removePathSpacesCopy":       {},
	"exportTextureFormat":        {},
	"exportModelFormat":          {},
	"exportM2Bones":              {},
	"exportM2Meta":               {},
	"exportWMOMeta":              {},
	"modelsExportSkin":           {},
	"modelsExportSkel":           {},
	"modelsExportBone":           {},
	"modelsExportAnim":           {},
	"modelsExportUV2":            {},
	"modelsExportTextures":       {},
	"modelsExportAlpha":          {},
	"modelsExportAnimations":     {},
	"modelsExportCollision":      {},
}

// IsSettableConfigKey reports whether key may be changed over HTTP.
func IsSettableConfigKey(key string) bool {
	_, ok := SettableConfigKeys[key]
	return ok
}

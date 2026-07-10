package wowmodel

import "github.com/pqhuy98/wow-converter/internal/formats/mdl"

// ConvertResult is assembled MDL output from OBJ/MTL or direct conversion.
type ConvertResult struct {
	MDL          *mdl.MDL
	TexturePaths []string
}

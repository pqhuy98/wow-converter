package service

import (
	"regexp"
	"strings"

	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	apicasc "github.com/pqhuy98/wow-converter/internal/wow/casc"
)

const maxRegexPatternLen = 256

// ListfileService adapts archive listfile to REST interface.
type ListfileService struct{}

func (ListfileService) IsLoaded() bool {
	return archivecasc.IsListfileLoaded()
}

func (ListfileService) GetFilteredEntries(search string, useRegex bool) []apicasc.ListfileEntry {
	var query any = search
	if useRegex && search != "" && len(search) <= maxRegexPatternLen && !nestedQuantifierPattern(search) {
		re, err := regexp.Compile("(?i)" + search)
		if err == nil {
			query = re
		}
	}
	entries := archivecasc.GetFilteredEntries(query)
	out := make([]apicasc.ListfileEntry, len(entries))
	for i, e := range entries {
		out[i] = apicasc.ListfileEntry{FileDataID: e.FileDataID, FileName: e.FileName}
	}
	return out
}

func (ListfileService) GetByID(fileDataID int) string {
	name, _ := archivecasc.GetByID(fileDataID)
	return name
}

func (ListfileService) GetByFilename(fileName string) int {
	id, _ := archivecasc.GetByFilename(fileName)
	return id
}

func (ListfileService) CollectBrowseFileIndex() (models, textures []apicasc.ListfileEntry) {
	m, t := archivecasc.CollectBrowseFileIndex()
	models = make([]apicasc.ListfileEntry, len(m))
	for i, e := range m {
		models[i] = apicasc.ListfileEntry{FileDataID: e.FileDataID, FileName: e.FileName}
	}
	textures = make([]apicasc.ListfileEntry, len(t))
	for i, e := range t {
		textures[i] = apicasc.ListfileEntry{FileDataID: e.FileDataID, FileName: e.FileName}
	}
	return models, textures
}

func nestedQuantifierPattern(pattern string) bool {
	for i := 0; i < len(pattern)-2; i++ {
		if pattern[i] != '(' {
			continue
		}
		depth := 1
		for j := i + 1; j < len(pattern); j++ {
			switch pattern[j] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					inner := pattern[i+1 : j]
					if strings.ContainsAny(inner, "+*") && j+1 < len(pattern) && strings.ContainsAny(pattern[j+1:], "+*{") {
						return true
					}
					break
				}
			}
		}
	}
	return false
}

func (ListfileService) CollectMapTileFileIndex() []apicasc.ListfileEntry {
	entries := archivecasc.CollectMapTileFileIndex()
	out := make([]apicasc.ListfileEntry, len(entries))
	for i, e := range entries {
		out[i] = apicasc.ListfileEntry{FileDataID: e.FileDataID, FileName: e.FileName}
	}
	return out
}

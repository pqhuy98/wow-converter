package api

import (
	"encoding/json"
	"fmt"

	"github.com/pqhuy98/wow-converter/internal/server/pathsafe"
)

var characterSizes = map[string]struct{}{
	"small": {}, "medium": {}, "large": {}, "hero": {}, "semi-giant": {}, "giant": {},
}

var characterAttackTags = map[string]struct{}{
	"": {}, "Auto": {}, "1H": {}, "2H": {}, "2HL": {}, "Unarmed": {},
	"Bow": {}, "Rifle": {}, "Thrown": {},
}

func parseExportCharacterRequest(raw map[string]any) (exportCharacterRequest, []string) {
	issues := validateExportCharacterShape(raw)
	applyExportCharacterDefaults(raw)

	data, err := json.Marshal(raw)
	if err != nil {
		return exportCharacterRequest{}, append(issues, "request body is not valid JSON")
	}
	var body exportCharacterRequest
	if err := json.Unmarshal(data, &body); err != nil {
		return exportCharacterRequest{}, append(issues, err.Error())
	}
	return body, issues
}

func applyExportCharacterDefaults(raw map[string]any) {
	optimization, ok := raw["optimization"].(map[string]any)
	if !ok {
		return
	}
	defaults := map[string]bool{
		"sortSequences":                 true,
		"allMaterialsUnshaded":          false,
		"removeUnusedVertices":          true,
		"removeUnusedNodes":             true,
		"removeUnusedMaterialsTextures": true,
	}
	for key, value := range defaults {
		if _, exists := optimization[key]; !exists {
			optimization[key] = value
		}
	}
}

func validateExportCharacterShape(raw map[string]any) []string {
	issues := make([]string, 0)
	for _, field := range []string{"character", "outputFileName", "optimization", "format"} {
		if _, ok := raw[field]; !ok {
			issues = append(issues, fmt.Sprintf("%s is required", field))
		}
	}

	if value, ok := raw["outputFileName"]; ok {
		name, valid := value.(string)
		if !valid || name == "" {
			issues = append(issues, "outputFileName must be a non-empty string")
		} else if err := pathsafe.ValidateRelativeRef(name); err != nil {
			issues = append(issues, "outputFileName must be a safe relative path")
		}
	}
	if value, ok := raw["format"]; ok {
		format, valid := value.(string)
		if !valid || (format != "mdx" && format != "mdl") {
			issues = append(issues, `format must be "mdx" or "mdl"`)
		}
	}
	if value, ok := raw["formatVersion"]; ok {
		version, valid := value.(string)
		if !valid || (version != "800" && version != "1000") {
			issues = append(issues, `formatVersion must be "800" or "1000"`)
		}
	}
	validateOptionalBool(raw, "isBrowse", &issues)
	validateOptionalString(raw, "skinId", &issues)
	validateExportOptimization(raw["optimization"], &issues)
	validateCharacterObject(raw["character"], "character", &issues)
	return issues
}

func validateExportOptimization(value any, issues *[]string) {
	optimization, ok := value.(map[string]any)
	if !ok {
		*issues = append(*issues, "optimization must be an object")
		return
	}
	for _, field := range []string{
		"sortSequences", "allMaterialsUnshaded", "removeUnusedVertices",
		"removeUnusedNodes", "removeUnusedMaterialsTextures",
	} {
		validateOptionalBool(optimization, field, issues)
	}
	if value, exists := optimization["maxTextureSize"]; exists {
		size, valid := value.(string)
		if !valid || (size != "256" && size != "512" && size != "1024") {
			*issues = append(*issues, `optimization.maxTextureSize must be "256", "512", or "1024"`)
		}
	}
}

func validateCharacterObject(value any, field string, issues *[]string) {
	character, ok := value.(map[string]any)
	if !ok {
		*issues = append(*issues, field+" must be an object")
		return
	}
	validateRefObject(character["base"], field+".base", true, issues)
	validateRequiredNumber(character, "inGameMovespeed", field, issues)
	for _, name := range []string{"scale", "particlesDensity"} {
		validateOptionalNumber(character, name, field, issues)
	}
	for _, name := range []string{"keepCinematic", "noDecay", "forceSheathed", "withCollision"} {
		validateOptionalBoolWithPrefix(character, name, field, issues)
	}
	validateOptionalStringWithPrefix(character, "portraitCameraSequenceName", field, issues)

	if value, exists := character["size"]; exists {
		size, valid := value.(string)
		if _, allowed := characterSizes[size]; !valid || !allowed {
			*issues = append(*issues, field+".size is not supported")
		}
	}
	if value, exists := character["attackTag"]; exists {
		tag, valid := value.(string)
		if _, allowed := characterAttackTags[tag]; !valid || !allowed {
			*issues = append(*issues, field+".attackTag is not supported")
		}
	}
	if value, exists := character["attachItems"]; exists {
		items, valid := value.(map[string]any)
		if !valid {
			*issues = append(*issues, field+".attachItems must be an object")
		} else {
			for key, itemValue := range items {
				item, itemValid := itemValue.(map[string]any)
				itemField := fmt.Sprintf("%s.attachItems.%s", field, key)
				if !itemValid {
					*issues = append(*issues, itemField+" must be an object")
					continue
				}
				validateRefObject(item["path"], itemField+".path", true, issues)
				validateOptionalNumber(item, "scale", itemField, issues)
			}
		}
	}
	if value, exists := character["mount"]; exists {
		mount, valid := value.(map[string]any)
		if !valid {
			*issues = append(*issues, field+".mount must be an object")
		} else {
			validateRefObject(mount["path"], field+".mount.path", true, issues)
			validateOptionalNumber(mount, "scale", field+".mount", issues)
			validateOptionalStringWithPrefix(mount, "animation", field+".mount", issues)
			if seatValue, seatExists := mount["seatOffset"]; seatExists {
				seat, seatValid := seatValue.([]any)
				if !seatValid || len(seat) != 3 {
					*issues = append(*issues, field+".mount.seatOffset must contain three numbers")
				} else {
					for _, coordinate := range seat {
						if _, valid := coordinate.(float64); !valid {
							*issues = append(*issues, field+".mount.seatOffset must contain three numbers")
							break
						}
					}
				}
			}
		}
	}
}

func validateRefObject(value any, field string, required bool, issues *[]string) {
	ref, ok := value.(map[string]any)
	if !ok {
		if required || value != nil {
			*issues = append(*issues, field+" must be an object")
		}
		return
	}
	refType, typeOK := ref["type"].(string)
	refValue, valueOK := ref["value"].(string)
	if !typeOK || (refType != "local" && refType != "wowhead" && refType != "displayID") {
		*issues = append(*issues, field+".type is not supported")
	}
	if !valueOK {
		*issues = append(*issues, field+".value must be a string")
	} else if refType == "local" {
		if err := pathsafe.ValidateRelativeRef(refValue); err != nil {
			*issues = append(*issues, field+".value must be a safe relative path")
		}
	}
}

func validateRequiredNumber(object map[string]any, name, prefix string, issues *[]string) {
	value, exists := object[name]
	if !exists {
		*issues = append(*issues, prefix+"."+name+" is required")
		return
	}
	if _, ok := value.(float64); !ok {
		*issues = append(*issues, prefix+"."+name+" must be a number")
	}
}

func validateOptionalNumber(object map[string]any, name, prefix string, issues *[]string) {
	if value, exists := object[name]; exists {
		if _, ok := value.(float64); !ok {
			*issues = append(*issues, prefix+"."+name+" must be a number")
		}
	}
}

func validateOptionalBool(object map[string]any, name string, issues *[]string) {
	if value, exists := object[name]; exists {
		if _, ok := value.(bool); !ok {
			*issues = append(*issues, name+" must be a boolean")
		}
	}
}

func validateOptionalBoolWithPrefix(object map[string]any, name, prefix string, issues *[]string) {
	if value, exists := object[name]; exists {
		if _, ok := value.(bool); !ok {
			*issues = append(*issues, prefix+"."+name+" must be a boolean")
		}
	}
}

func validateOptionalString(object map[string]any, name string, issues *[]string) {
	if value, exists := object[name]; exists {
		if _, ok := value.(string); !ok {
			*issues = append(*issues, name+" must be a string")
		}
	}
}

func validateOptionalStringWithPrefix(object map[string]any, name, prefix string, issues *[]string) {
	if value, exists := object[name]; exists {
		if _, ok := value.(string); !ok {
			*issues = append(*issues, prefix+"."+name+" must be a string")
		}
	}
}

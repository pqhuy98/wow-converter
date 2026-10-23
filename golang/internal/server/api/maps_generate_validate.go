package api

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter"
)

var generateWc3AllowedFields = map[string]struct{}{
	"tiles": {}, "quality": {}, "mapSaveName": {}, "clampLower": {},
	"clampUpper": {}, "autoClampPercent": {}, "mapAngleDeg": {},
	"unitScale": {}, "includeBuildingInteriors": {}, "freshExport": {},
	"creatures": {}, "navigation": {}, "placePathBlockers": {},
	"terrainAsWc3Tilesets": {}, "playerFaction": {}, "doodadMaxTextureSize": {},
}

var generateWc3AllowedQualities = map[int]struct{}{
	0: {}, 512: {}, 1024: {}, 2048: {}, 4096: {}, 8192: {}, 16384: {},
}

func parseGenerateWc3Body(raw map[string]any) (generateWc3Body, []string) {
	issues := validateGenerateWc3Shape(raw)
	if _, ok := raw["autoClampPercent"]; !ok {
		raw["autoClampPercent"] = true
	}
	if _, ok := raw["includeBuildingInteriors"]; !ok {
		raw["includeBuildingInteriors"] = true
	}

	if _, ok := raw["playerFaction"]; !ok {
		raw["playerFaction"] = "Alliance"
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return generateWc3Body{}, append(issues, "request body is not valid JSON")
	}
	defaults := mapexporter.DefaultNavigationConfig()
	body := generateWc3Body{Navigation: &defaults}
	if err := json.Unmarshal(data, &body); err != nil {
		return generateWc3Body{}, append(issues, err.Error())
	}

	if body.PlayerFaction != "Horde" && body.PlayerFaction != "Alliance" {
		issues = append(issues, "playerFaction must be Horde or Alliance")
	}
	if len(body.Tiles) == 0 {
		issues = append(issues, "tiles must contain at least one tile")
	}
	for i, tile := range body.Tiles {
		if tile.X < 0 || tile.X > 63 || tile.Y < 0 || tile.Y > 63 {
			issues = append(issues, fmt.Sprintf("tiles[%d] coordinates must be between 0 and 63", i))
		}
	}
	if _, ok := generateWc3AllowedQualities[body.Quality]; !ok {
		issues = append(issues, "quality is not supported")
	}
	body.MapSaveName = mapexporter.NormalizeMapSaveName(body.MapSaveName)
	if body.MapSaveName == "" || len(body.MapSaveName) > 128 {
		issues = append(issues, "mapSaveName must produce a valid name of at most 128 characters")
	}
	if body.ClampLower < 0 || body.ClampLower > 1 {
		issues = append(issues, "clampLower must be between 0 and 1")
	}
	if body.ClampUpper < 0 || body.ClampUpper > 1 {
		issues = append(issues, "clampUpper must be between 0 and 1")
	}
	if body.ClampUpper <= body.ClampLower {
		issues = append(issues, "clampUpper must be > clampLower")
	}
	if body.UnitScale <= 0 {
		issues = append(issues, "unitScale must be greater than 0")
	}
	if size := body.Creatures.MaxTextureSize; size != nil && *size != 0 && *size != 256 && *size != 512 && *size != 1024 {
		issues = append(issues, "creatures.maxTextureSize must be 0, 256, 512 or 1024")
	}
	if size := body.DoodadMaxTextureSize; size != nil && *size != 0 && *size != 256 && *size != 512 && *size != 1024 {
		issues = append(issues, "doodadMaxTextureSize must be 0, 256, 512 or 1024")
	}
	if body.Navigation != nil {
		if err := body.Navigation.Validate(); err != nil {
			issues = append(issues, err.Error())
		}
	}
	return body, issues
}

func validateGenerateWc3Shape(raw map[string]any) []string {
	issues := make([]string, 0)
	if value, exists := raw["navigation"]; exists {
		if object, ok := value.(map[string]any); !ok {
			issues = append(issues, "navigation must be an object")
		} else {
			allowed := map[string]bool{"disabled": true, "radius": true, "height": true, "maxSlopeDegrees": true, "stepHeight": true, "jumpHeight": true, "jumpDistance": true, "maxDrop": true, "seedSnap": true, "airBlockHeight": true}
			for key, v := range object {
				if !allowed[key] {
					issues = append(issues, "unknown navigation field "+key)
				}
				if key == "disabled" {
					if _, ok := v.(bool); !ok {
						issues = append(issues, "navigation.disabled must be a boolean")
					}
				} else {
					if _, ok := v.(float64); !ok {
						issues = append(issues, "navigation."+key+" must be a number")
					}
				}
			}
		}
	}
	for field := range raw {
		if _, ok := generateWc3AllowedFields[field]; !ok {
			issues = append(issues, fmt.Sprintf("unknown field %q", field))
		}
	}
	for _, field := range []string{
		"tiles", "quality", "mapSaveName", "clampLower", "clampUpper",
		"mapAngleDeg", "unitScale", "freshExport", "creatures",
	} {
		if _, ok := raw[field]; !ok {
			issues = append(issues, fmt.Sprintf("%s is required", field))
		}
	}

	for _, field := range []string{"quality", "clampLower", "clampUpper", "mapAngleDeg", "unitScale", "doodadMaxTextureSize"} {
		if value, exists := raw[field]; exists {
			if _, ok := value.(float64); !ok {
				issues = append(issues, fmt.Sprintf("%s must be a number", field))
			}
		}
	}
	if value, exists := raw["mapSaveName"]; exists {
		if _, ok := value.(string); !ok {
			issues = append(issues, "mapSaveName must be a string")
		}
	}
	for _, field := range []string{"autoClampPercent", "includeBuildingInteriors", "freshExport", "placePathBlockers", "terrainAsWc3Tilesets"} {
		if value, exists := raw[field]; exists {
			if _, ok := value.(bool); !ok {
				issues = append(issues, fmt.Sprintf("%s must be a boolean", field))
			}
		}
	}

	tiles, ok := raw["tiles"].([]any)
	if !ok {
		if _, exists := raw["tiles"]; exists {
			issues = append(issues, "tiles must be an array")
		}
	} else {
		for i, value := range tiles {
			tile, isObject := value.(map[string]any)
			if !isObject {
				issues = append(issues, fmt.Sprintf("tiles[%d] must be an object", i))
				continue
			}
			for _, field := range []string{"x", "y"} {
				coordinate, exists := tile[field]
				if !exists {
					issues = append(issues, fmt.Sprintf("tiles[%d].%s is required", i, field))
				} else if number, isNumber := coordinate.(float64); !isNumber || math.Trunc(number) != number {
					issues = append(issues, fmt.Sprintf("tiles[%d].%s must be an integer", i, field))
				}
			}
		}
	}

	creatures, ok := raw["creatures"].(map[string]any)
	if !ok {
		if _, exists := raw["creatures"]; exists {
			issues = append(issues, "creatures must be an object")
		}
	} else {
		if value, exists := creatures["maxTextureSize"]; exists {
			if _, ok := value.(float64); !ok {
				issues = append(issues, "creatures.maxTextureSize must be a number")
			}
		}
		for _, field := range []string{"enable", "allAreDoodads"} {
			value, exists := creatures[field]
			if !exists {
				issues = append(issues, fmt.Sprintf("creatures.%s is required", field))
			} else if _, isBool := value.(bool); !isBool {
				issues = append(issues, fmt.Sprintf("creatures.%s must be a boolean", field))
			}
		}
	}
	return issues
}

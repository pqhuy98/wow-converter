package mtl

import (
	"os"
	"strconv"
	"strings"
)

// ParseFile reads and parses an MTL export artifact.
func ParseFile(filePath string) ([]Material, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return Parse(string(data)), nil
}

// Parse parses MTL text content.
func Parse(text string) []Material {
	var materials []Material
	var current *Material
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "newmtl":
			if current != nil {
				materials = append(materials, *current)
			}
			name := ""
			if len(fields) > 1 {
				name = fields[1]
			}
			current = &Material{Name: name}
		case "map_Kd":
			if current != nil && len(fields) > 1 {
				current.MapKd = strings.Join(fields[1:], " ")
			}
		}
	}
	if current != nil {
		materials = append(materials, *current)
	}
	return materials
}

func parseFloat(fields []string) float64 {
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[1], 64)
	return v
}

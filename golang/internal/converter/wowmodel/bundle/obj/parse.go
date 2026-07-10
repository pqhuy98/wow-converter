package obj

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ParseFile reads and parses an OBJ export artifact.
func ParseFile(filePath string) (Result, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Result{}, err
	}
	return Parse(string(data)), nil
}

// Parse parses OBJ text content.
func Parse(text string) Result {
	res := Result{}
	currentMaterial := ""
	currentGroup := Group{Name: ""}
	nextGroupID := 1
	smoothingGroup := 0
	defaultModelName := "untitled"

	currentModel := func() *Model {
		if len(res.Models) == 0 {
			res.Models = append(res.Models, Model{
				Name:  defaultModelName,
				Group: []Group{{Name: ""}},
			})
			smoothingGroup = 0
		}
		return &res.Models[len(res.Models)-1]
	}

	for _, line := range strings.Split(text, "\n") {
		stripped := stripComments(line)
		fields := strings.Fields(stripped)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "o":
			name := defaultModelName
			if len(fields) >= 2 {
				name = fields[1]
			}
			res.Models = append(res.Models, Model{
				Name:  name,
				Group: []Group{{Name: ""}},
			})
			currentGroup = Group{Name: ""}
			smoothingGroup = 0
		case "g":
			if len(fields) != 2 {
				continue
			}
			currentGroup = Group{Name: fields[1], ID: nextGroupID}
			nextGroupID++
		case "v":
			m := currentModel()
			x, y, z := parseFloatDefault(fields, 1, 3)
			m.Vertices = append(m.Vertices, Vertex{X: x, Y: y, Z: z})
		case "vt":
			m := currentModel()
			u, v, w := parseFloatDefault(fields, 1, 3)
			m.TextureCoords = append(m.TextureCoords, TextureVertex{U: u, V: v, W: w})
		case "vt2":
			m := currentModel()
			u, v, w := parseFloatDefault(fields, 1, 3)
			m.TextureCoords2 = append(m.TextureCoords2, TextureVertex{U: u, V: v, W: w})
		case "vn":
			m := currentModel()
			x, y, z := parseFloatDefault(fields, 1, 3)
			m.VertexNormals = append(m.VertexNormals, Vertex{X: x, Y: y, Z: z})
		case "s":
			if len(fields) >= 2 {
				if fields[1] == "off" {
					smoothingGroup = 0
				} else if v, err := strconv.Atoi(fields[1]); err == nil {
					smoothingGroup = v
				}
			}
		case "f":
			m := currentModel()
			if len(fields) < 4 {
				continue
			}
			face := Face{
				Group:          currentGroup,
				Material:       currentMaterial,
				SmoothingGroup: smoothingGroup,
			}
			for i := 1; i <= 3 && i < len(fields); i++ {
				face.Vertices[i-1] = parseFaceVertex(fields[i])
			}
			m.Faces = append(m.Faces, face)
		case "mtllib":
			if len(fields) >= 2 {
				res.MaterialLibraries = append(res.MaterialLibraries, fields[1])
			}
		case "usemtl":
			if len(fields) >= 2 {
				currentMaterial = fields[1]
			}
		}
	}
	_ = filepath.Base
	return res
}

func stripComments(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func parseFloatDefault(fields []string, start, count int) (float64, float64, float64) {
	vals := []float64{0, 0, 0}
	for i := 0; i < count && start+i < len(fields); i++ {
		if v, err := strconv.ParseFloat(fields[start+i], 64); err == nil {
			vals[i] = v
		}
	}
	return vals[0], vals[1], vals[2]
}

func parseFaceVertex(token string) FaceVertex {
	parts := strings.Split(token, "/")
	readIdx := func(s string) int {
		if s == "" {
			return 0
		}
		v, _ := strconv.Atoi(s)
		return v
	}
	fv := FaceVertex{VertexIndex: readIdx(parts[0])}
	if len(parts) > 1 {
		fv.TextureCoordsIndex = readIdx(parts[1])
	}
	if len(parts) > 2 {
		fv.VertexNormalIndex = readIdx(parts[2])
	}
	return fv
}

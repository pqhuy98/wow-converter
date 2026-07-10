package writers

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

// OBJMesh is a mesh section in an OBJ file.
type OBJMesh struct {
	Name, MatName string
	Triangles     []int
}

// OBJWriter builds Wavefront OBJ content.
type OBJWriter struct {
	Out      string
	Name     string
	MTL      string
	Verts    []float32
	Normals  []float32
	UVs      [][]float32
	Meshes   []OBJMesh
}

// SetMaterialLibrary sets the mtllib name.
func (w *OBJWriter) SetMaterialLibrary(name string) { w.MTL = name }

// SetName sets the object name.
func (w *OBJWriter) SetName(name string) { w.Name = name }

// SetVertArray sets vertex positions.
func (w *OBJWriter) SetVertArray(verts []float32) { w.Verts = verts }

// SetNormalArray sets vertex normals.
func (w *OBJWriter) SetNormalArray(normals []float32) { w.Normals = normals }

// AddUVArray adds a UV layer.
func (w *OBJWriter) AddUVArray(uv []float32) { w.UVs = append(w.UVs, uv) }

// AddMesh adds a mesh group.
func (w *OBJWriter) AddMesh(name string, triangles []int, matName string) {
	w.Meshes = append(w.Meshes, OBJMesh{Name: name, Triangles: triangles, MatName: matName})
}

// Content returns OBJ text.
func (w *OBJWriter) Content() string {
	name := w.Name
	if name == "" {
		name = "Mesh"
	}
	lines := []string{
		fmt.Sprintf("# Exported using wow-converter v%s", constants.Version),
		fmt.Sprintf("o %s", name),
	}
	if w.MTL != "" {
		lines = append(lines, "mtllib "+w.MTL)
	}
	used := make(map[int]struct{})
	for _, mesh := range w.Meshes {
		for _, idx := range mesh.Triangles {
			used[idx] = struct{}{}
		}
	}
	vertMap := make(map[int]int)
	normalMap := make(map[int]int)
	uvMap := make(map[int]int)
	u := 0
	for j := 0; j < len(w.Verts)/3; j++ {
		if _, ok := used[j]; ok {
			vertMap[j] = u
			i := j * 3
			lines = append(lines, fmt.Sprintf("v %s %s %s", fmtFloat32(w.Verts[i]), fmtFloat32(w.Verts[i+1]), fmtFloat32(w.Verts[i+2])))
			u++
		}
	}
	u = 0
	for j := 0; j < len(w.Normals)/3; j++ {
		if _, ok := used[j]; ok {
			normalMap[j] = u
			i := j * 3
			lines = append(lines, fmt.Sprintf("vn %s %s %s", fmtFloat32(w.Normals[i]), fmtFloat32(w.Normals[i+1]), fmtFloat32(w.Normals[i+2])))
			u++
		}
	}
	hasUV := len(w.UVs) > 0
	if hasUV {
		for uvIndex, uv := range w.UVs {
			prefix := "vt"
			if uvIndex > 0 {
				prefix += fmt.Sprintf("%d", uvIndex+1)
			}
			u = 0
			for j := 0; j < len(uv)/2; j++ {
				if _, ok := used[j]; ok {
					if uvIndex == 0 {
						uvMap[j] = u
					}
					i := j * 2
					lines = append(lines, fmt.Sprintf("%s %s %s", prefix, fmtFloat32(uv[i]), fmtFloat32(uv[i+1])))
					u++
				}
			}
		}
	}
	for _, mesh := range w.Meshes {
		lines = append(lines, "g "+mesh.Name, "s 1")
		if mesh.MatName != "" {
			lines = append(lines, "usemtl "+mesh.MatName)
		}
		tris := mesh.Triangles
		for i := 0; i < len(tris); i += 3 {
			point := func(idx int) string {
				v := vertMap[tris[idx]] + 1
				n := normalMap[tris[idx]] + 1
				if hasUV {
					return fmt.Sprintf("%d/%d/%d", v, uvMap[tris[idx]]+1, n)
				}
				return fmt.Sprintf("%d//%d", v, n)
			}
			lines = append(lines, fmt.Sprintf("f %s %s %s", point(i), point(i+1), point(i+2)))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func fmtFloat32(v float32) string {
	return strconv.FormatFloat(float64(v), 'g', -1, 64)
}

// Write writes the OBJ file.
func (w *OBJWriter) Write(overwrite bool) error {
	if !overwrite && OutputFileExists(w.Out) {
		return nil
	}
	return WriteOutputFile(w.Out, []byte(w.Content()))
}

// MTLMaterial is a material entry.
type MTLMaterial struct{ Name, File string }

// MTLWriter builds MTL material libraries.
type MTLWriter struct {
	Out       string
	Materials []MTLMaterial
}

// AddMaterial adds a material.
func (m *MTLWriter) AddMaterial(name, file string) {
	m.Materials = append(m.Materials, MTLMaterial{Name: name, File: file})
}

// IsEmpty reports whether the library has no materials.
func (m *MTLWriter) IsEmpty() bool { return len(m.Materials) == 0 }

// Content returns MTL text.
func (m *MTLWriter) Content() string {
	cfg := server.GetConfig()
	mtlDir := filepath.Dir(m.Out)
	lines := make([]string, 0, len(m.Materials)*3)
	for _, mat := range m.Materials {
		lines = append(lines, "newmtl "+mat.Name, "illum 1")
		file := mat.File
		if cfg.EnableAbsoluteMTLPaths {
			file = filepath.Join(mtlDir, file)
		}
		lines = append(lines, "map_Kd "+file)
	}
	return strings.Join(lines, "\n") + "\n"
}

// Write writes the MTL file.
func (m *MTLWriter) Write(overwrite bool) error {
	if m.IsEmpty() {
		return nil
	}
	if !overwrite && OutputFileExists(m.Out) {
		return nil
	}
	return WriteOutputFile(m.Out, []byte(m.Content()))
}

// CSVWriter builds semicolon-separated CSV files.
type CSVWriter struct {
	Out    string
	Fields []string
	Rows   []map[string]any
}

// AddField adds CSV columns.
func (c *CSVWriter) AddField(fields ...string) { c.Fields = append(c.Fields, fields...) }

// AddRow adds a CSV row.
func (c *CSVWriter) AddRow(row map[string]any) { c.Rows = append(c.Rows, row) }

func escapeCSVField(value any) string {
	if value == nil {
		return ""
	}
	str := fmt.Sprint(value)
	if strings.ContainsAny(str, ";\"") || strings.Contains(str, "\n") {
		return `"` + strings.ReplaceAll(str, `"`, `""`) + `"`
	}
	return str
}

// Content returns CSV text.
func (c *CSVWriter) Content() string {
	lines := []string{strings.Join(c.Fields, ";")}
	for _, row := range c.Rows {
		parts := make([]string, len(c.Fields))
		for i, field := range c.Fields {
			parts[i] = escapeCSVField(row[field])
		}
		lines = append(lines, strings.Join(parts, ";"))
	}
	return strings.Join(lines, "\n") + "\n"
}

// Write writes the CSV file.
func (c *CSVWriter) Write(overwrite bool) error {
	if len(c.Rows) == 0 {
		return nil
	}
	if !overwrite && OutputFileExists(c.Out) {
		return nil
	}
	return WriteOutputFile(c.Out, []byte(c.Content()))
}

// JSONWriter builds JSON export files.
type JSONWriter struct {
	Out  string
	Data map[string]any
}

// NewJSONWriter creates a JSON writer.
func NewJSONWriter(out string) *JSONWriter {
	return &JSONWriter{Out: out, Data: make(map[string]any)}
}

// AddProperty adds a JSON property.
func (j *JSONWriter) AddProperty(name string, data any) { j.Data[name] = data }

// Write writes JSON to disk (pretty-printed).
func (j *JSONWriter) Write(overwrite bool) error {
	if !overwrite && OutputFileExists(j.Out) {
		return nil
	}
	raw, err := json.MarshalIndent(j.Data, "", "\t")
	if err != nil {
		return err
	}
	return WriteOutputFile(j.Out, append(raw, '\n'))
}

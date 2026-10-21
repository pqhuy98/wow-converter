package terrainfit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/wow/export/adt"
)

// Region uses inclusive ADT file coordinates, not the exporter's transposed index.
type Region struct {
	Name  string `json:"name"`
	Map   string `json:"map"`
	MapID int    `json:"mapID"`
	X0    int    `json:"x0"`
	Y0    int    `json:"y0"`
	X1    int    `json:"x1"`
	Y1    int    `json:"y1"`
}

// Mesh stores normalized top-left UV coordinates and original WoW heights.
// Each original ADT triangle is retained, including missing hole triangles.
type Mesh struct {
	Positions [][3]float64 `json:"positions"`
	Indices   []uint32     `json:"indices"`
}

type captureInfo struct {
	Version   int    `json:"version"`
	Build     string `json:"build"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Triangles int    `json:"triangles"`
}

func CaptureValid(dir, build string) bool {
	b, e := os.ReadFile(filepath.Join(dir, "capture.json"))
	if e != nil {
		return false
	}
	var info captureInfo
	if json.Unmarshal(b, &info) != nil || info.Version != 1 || info.Build != build || info.Width != 8192 {
		return false
	}
	for _, n := range []string{"wow.png", "mesh.json"} {
		if f, e := os.Stat(filepath.Join(dir, n)); e != nil || f.Size() == 0 {
			return false
		}
	}
	return true
}

func SaveCapture(dir, build string, conv *adt.ConversionOutput) error {
	if len(conv.Textures) != 1 {
		return fmt.Errorf("expected one complete 8K bake, got %d", len(conv.Textures))
	}
	cfg, e := png.DecodeConfig(bytes.NewReader(conv.Textures[0].PNG))
	if e != nil {
		return e
	}
	if cfg.Width != 8192 || cfg.Height != 8192 {
		return fmt.Errorf("reference is %dx%d", cfg.Width, cfg.Height)
	}
	mesh, e := parseOBJ(conv.ObjText)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(dir, "wow.png"), conv.Textures[0].PNG, 0644); e != nil {
		return e
	}
	if e = writeJSON(filepath.Join(dir, "mesh.json"), mesh); e != nil {
		return e
	}
	return writeJSON(filepath.Join(dir, "capture.json"), captureInfo{1, build, 8192, 8192, len(mesh.Indices) / 3})
}

func parseOBJ(s string) (Mesh, error) {
	var p [][3]float64
	var uv [][2]float64
	var indices []uint32
	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "v":
			if len(f) != 4 {
				return Mesh{}, fmt.Errorf("invalid OBJ vertex")
			}
			var v [3]float64
			for j := range v {
				n, e := strconv.ParseFloat(f[j+1], 64)
				if e != nil {
					return Mesh{}, e
				}
				v[j] = n
			}
			p = append(p, v)
		case "vt":
			if len(f) < 3 {
				return Mesh{}, fmt.Errorf("invalid OBJ UV")
			}
			u, e := strconv.ParseFloat(f[1], 64)
			if e != nil {
				return Mesh{}, e
			}
			v, e := strconv.ParseFloat(f[2], 64)
			if e != nil {
				return Mesh{}, e
			}
			uv = append(uv, [2]float64{u, 1 - v})
		case "f":
			if len(f) != 4 {
				return Mesh{}, fmt.Errorf("non-triangle OBJ")
			}
			for _, v := range f[1:] {
				parts := strings.Split(v, "/")
				n, e := strconv.Atoi(parts[0])
				if e != nil || n < 1 || n > len(p) {
					return Mesh{}, fmt.Errorf("invalid face index")
				}
				if len(parts) < 2 || parts[0] != parts[1] {
					return Mesh{}, fmt.Errorf("unshared OBJ UV indexing")
				}
				indices = append(indices, uint32(n-1))
			}
		}
	}
	if e := scanner.Err(); e != nil {
		return Mesh{}, e
	}
	if len(p) != len(uv) || len(indices) == 0 {
		return Mesh{}, fmt.Errorf("missing mesh/UV data")
	}
	for i := range p {
		p[i] = [3]float64{uv[i][0], uv[i][1], p[i][1]}
	}
	return Mesh{p, indices}, nil
}

func writeJSON(path string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return os.WriteFile(path, b, 0644)
}

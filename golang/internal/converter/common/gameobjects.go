package common

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

// ReadGameObjects adds server spawns to their ADT parents using world coordinates.
// Geometry is shared by file ID; placements remain distinct by server GUID.
func (m *WowObjectManager) ReadGameObjects(ctx context.Context, mapID int) error {
	type tileSpawns struct {
		tile   *WowObject
		spawns []azerothcore.GameObject
	}
	var tiles []tileSpawns
	wanted := map[int]bool{}
	for _, tile := range m.Terrains {
		x, y, ok := AsAdt(tile)
		if !ok {
			continue
		}
		spawns, err := azerothcore.GetGameObjectsInTile(ctx, mapID, [2]int{x, y})
		if err != nil {
			return err
		}
		tiles = append(tiles, tileSpawns{tile, spawns})
		for _, g := range spawns {
			wanted[g.DisplayID] = true
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	models, err := m.AssetManager.gameObjectModels(ctx, wanted)
	if err != nil {
		return err
	}
	count := 0
	skipped := 0
	for _, tile := range tiles {
		for _, g := range tile.spawns {
			if err := ctx.Err(); err != nil {
				return err
			}
			id := fmt.Sprintf("gameobject-%d", g.GUID)
			if _, exists := m.Objects[id]; exists {
				continue
			}
			fid := models[g.DisplayID]
			model, err := m.AssetManager.gameObjectModel(ctx, g, fid)
			if err != nil {
				return err
			}
			if model == nil {
				skipped++
				continue
			}
			position, rotation := gameObjectTransform(g, tile.tile, m.config.RawModelScaleUp)
			obj := &WowObject{ID: id, Type: WowObjectGobj, FileDataID: fid, Model: model, Position: position, Rotation: rotation, ScaleFactor: g.Scale, GameObject: &g}
			tile.tile.Children = append(tile.tile.Children, obj)
			m.Objects[id] = obj
			m.Doodads = append(m.Doodads, obj)
			count++
		}
	}
	log.Printf("Loaded %d AzerothCore gameobject spawns; skipped %d without usable models", count, skipped)
	return nil
}

func (a *AssetManager) gameObjectModel(ctx context.Context, g azerothcore.GameObject, fid int) (*Model, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if fid <= 0 {
		log.Printf("Skipping gameobject %d (%s), display %d: no model in loaded client", g.GUID, g.Name, g.DisplayID)
		return nil, nil
	}
	model, err := a.ResolveModel(ctx, fmt.Sprintf("gameobjects/%d", fid), fid, WowObjectGobj, false)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Printf("Skipping gameobject %d (%s), display %d, file %d: %v", g.GUID, g.Name, g.DisplayID, fid, err)
		return nil, nil
	}
	return model, nil
}

func gameObjectTransform(g azerothcore.GameObject, parent *WowObject, scale float64) ([3]float64, [3]float64) {
	position := imath.V3Scale([3]float64{-g.Position[0], -g.Position[1], g.Position[2]}, scale)
	position = imath.V3Rotate(imath.V3Sub(position, parent.Position), imath.V3Negative(parent.Rotation))
	q := g.Rotation
	norm := math.Sqrt(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3])
	rotation := [3]float64{0, 0, g.Orientation}
	// Match GameObject::Create: AzerothCore's default spawns use orientation
	// because stored quaternions are unreliable; only explicit exceptions use them.
	if g.UseQuaternion && norm > 0 && !math.IsNaN(norm) && !math.IsInf(norm, 0) {
		for i := range q {
			q[i] /= norm
		}
		rotation = imath.QuaternionToEuler(q)
	}
	// The server-to-export basis negates world X/Y: a half turn about Z.
	rotation = imath.CalculateChildAbsoluteEulerRotation([3]float64{0, 0, math.Pi}, rotation)
	rotation = imath.CalculateChildAbsoluteEulerRotation(imath.V3Negative(parent.Rotation), rotation)
	return position, rotation
}

type gameObjectFileSource struct {
	db.RuntimeFileSource
	client client.Client
	build  string
}

func (s gameObjectFileSource) GetBuildName() string { return s.build }
func (s gameObjectFileSource) GetFile(ctx context.Context, id int) ([]byte, error) {
	if tables, ok := s.client.(client.DBTableClient); ok {
		return tables.DownloadCascTable(ctx, id)
	}
	return s.client.DownloadCascFile(ctx, id)
}
func (s gameObjectFileSource) GetFileByName(ctx context.Context, name string) ([]byte, error) {
	entry, err := s.client.GetFileByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if entry.FileDataID <= 0 {
		return nil, fmt.Errorf("client file %s not found", name)
	}
	return s.GetFile(ctx, entry.FileDataID)
}

func (a *AssetManager) gameObjectModels(ctx context.Context, wanted map[int]bool) (map[int]int, error) {
	var source db.FileSource = db.RuntimeFileSource{}
	if a.wowClient != nil {
		info, err := a.wowClient.GetCASCInfo(ctx)
		if err != nil {
			return nil, err
		}
		source = gameObjectFileSource{client: a.wowClient, build: info.BuildName}
	}
	path := "DBFilesClient/GameObjectDisplayInfo.db2"
	raw, err := source.GetFileByName(ctx, path)
	if err != nil {
		path = "DBFilesClient/GameObjectDisplayInfo.dbc"
		raw, err = source.GetFileByName(ctx, path)
	}
	if err != nil {
		return nil, fmt.Errorf("load gameobject display table: %w", err)
	}
	result := map[int]int{}
	if wanted[0] {
		result[0] = 0
	}
	names := map[int]string{}
	if len(raw) >= 4 && string(raw[:4]) == "WDBC" {
		names, err = gameObjectDBCModels(raw)
		if err != nil {
			return nil, err
		}
		for id := range wanted {
			if name, exists := names[id]; exists && name == "" {
				result[id] = 0
			}
		}
	} else {
		reader := db.NewWDCReader(path, source)
		if err := reader.Parse(ctx, raw); err != nil {
			return nil, err
		}
		for id := range wanted {
			row := reader.GetRow(uint32(id))
			if row == nil {
				continue
			}
			result[id] = gameObjectFileID(row["FileDataID"])
			if name, ok := row["ModelName"].(string); ok {
				names[id] = name
			}
		}
	}
	for id := range wanted {
		if result[id] > 0 {
			continue
		}
		name := names[id]
		if name == "" {
			continue
		}
		name = strings.ReplaceAll(name, "\\", "/")
		// Old DBCs name M2 assets with the original MDX extension.
		if strings.HasSuffix(strings.ToLower(name), ".mdx") {
			name = name[:len(name)-4] + ".m2"
		}
		if a.wowClient != nil {
			entry, err := a.wowClient.GetFileByName(ctx, name)
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				log.Printf("Skipping gameobject display %d model %s: %v", id, name, err)
				continue
			}
			result[id] = entry.FileDataID
		} else {
			if fid, ok := archivecasc.GetByFilename(name); ok {
				result[id] = fid
			}
		}
	}
	return result, nil
}

func gameObjectFileID(value any) int {
	switch v := value.(type) {
	case uint32:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint64:
		return int(v)
	case int:
		return v
	case []uint32:
		if len(v) > 0 {
			return int(v[0])
		}
	case []int64:
		if len(v) > 0 {
			return int(v[0])
		}
	}
	return 0
}

// WotLK GameObjectDisplayInfo.dbc stores ID and ModelName in its first two fields.
func gameObjectDBCModels(raw []byte) (map[int]string, error) {
	if len(raw) < 20 || string(raw[:4]) != "WDBC" {
		return nil, fmt.Errorf("invalid GameObjectDisplayInfo DBC header")
	}
	count := uint64(binary.LittleEndian.Uint32(raw[4:8]))
	fields := uint64(binary.LittleEndian.Uint32(raw[8:12]))
	size := uint64(binary.LittleEndian.Uint32(raw[12:16]))
	stringsSize := uint64(binary.LittleEndian.Uint32(raw[16:20]))
	end := uint64(20) + count*size
	if fields < 2 || size < 8 || end+stringsSize > uint64(len(raw)) {
		return nil, fmt.Errorf("truncated GameObjectDisplayInfo DBC")
	}
	stringsData := raw[end : end+stringsSize]
	models := map[int]string{}
	for i := uint64(0); i < count; i++ {
		record := raw[20+i*size : 20+(i+1)*size]
		id := int(binary.LittleEndian.Uint32(record[:4]))
		offset := uint64(binary.LittleEndian.Uint32(record[4:8]))
		if offset >= stringsSize {
			return nil, fmt.Errorf("invalid model string offset for display %d", id)
		}
		tail := stringsData[offset:]
		terminator := strings.IndexByte(string(tail), 0)
		if terminator < 0 {
			return nil, fmt.Errorf("unterminated model name for display %d", id)
		}
		models[id] = string(tail[:terminator])
	}
	return models, nil
}

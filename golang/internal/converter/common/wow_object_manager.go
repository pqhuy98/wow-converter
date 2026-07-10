package common

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

const maxWoWSize = 51200.0 / 3.0

type placementInfoRow struct {
	ModelFile   string
	PositionX   string
	PositionY   string
	PositionZ   string
	RotationX   string
	RotationY   string
	RotationZ   string
	RotationW   string
	ScaleFactor string
	ModelId     string
	FileDataID  string
	Type        string
}

// WowObjectManager owns the WoW object tree and asset cache.
type WowObjectManager struct {
	config       config.Config
	AssetManager *AssetManager
	Objects      map[string]*WowObject
	Terrains     []*WowObject
	Doodads      []*WowObject
	Roots        []*WowObject
}

// NewWowObjectManager creates a manager.
func NewWowObjectManager(cfg config.Config, wowClient client.Client) *WowObjectManager {
	return &WowObjectManager{
		config:       cfg,
		AssetManager: NewAssetManager(cfg, wowClient),
		Objects:      map[string]*WowObject{},
	}
}

// IterateObjects walks all objects with absolute placement.
func (m *WowObjectManager) IterateObjects(fn func(obj *WowObject, abs ObjectAbsolute)) {
	var visit func(o *WowObject, parent *ObjectAbsolute)
	visit = func(o *WowObject, parent *ObjectAbsolute) {
		abs := ObjectAbsolute{
			Position:    math.Vector3(o.Position),
			Rotation:    o.Rotation,
			ScaleFactor: o.ScaleFactor,
		}
		if parent != nil {
			relPos := math.V3Rotate(abs.Position, parent.Rotation)
			abs.Position = math.V3Sum(parent.Position, relPos)
			abs.Rotation = math.CalculateChildAbsoluteEulerRotation(parent.Rotation, abs.Rotation)
			abs.ScaleFactor *= parent.ScaleFactor
		}
		fn(o, abs)
		childAbs := abs
		for _, c := range o.Children {
			visit(c, &childAbs)
		}
	}
	for _, r := range m.Roots {
		visit(r, nil)
	}
}

// ReadTerrainsDoodads parses exported OBJ roots matching patterns.
func (m *WowObjectManager) ReadTerrainsDoodads(patterns []string, filter func(id string, typ WowObjectType) bool) error {
	start := time.Now()
	rootSet := map[*WowObject]struct{}{}

	var files []string
	for _, pattern := range patterns {
		matches, err := GlobExportFiles(m.config.ExportAssetDir, pattern)
		if err != nil {
			return err
		}
		files = append(files, matches...)
	}
	log.Printf("Parsing root files %v", files)

	for _, file := range files {
		rel := m.relative(file)
		typ := WowObjectWMO
		if strings.Contains(rel, "adt") {
			typ = WowObjectADT
		}
		if filter != nil && !filter(rel, typ) {
			continue
		}
		fileName := StripModelReferenceExt(rel)
		root := &WowObject{
			ID:          fileName,
			Type:        typ,
			Position:    [3]float64{0, 0, 0},
			Rotation:    [3]float64{0, 0, math.Radians(-90)},
			ScaleFactor: 1,
		}
		if typ == WowObjectADT {
			parts := strings.Split(fileName, "_")
			if len(parts) >= 3 {
				root.TileX, _ = strconv.Atoi(parts[1])
				root.TileY, _ = strconv.Atoi(parts[2])
			}
		}
		m.Roots = append(m.Roots, root)
		rootSet[root] = struct{}{}
		if err := m.parseRecursive(fileName, root, filter); err != nil {
			return err
		}
		if isEmptyModel(root) {
			log.Printf("Empty root model: %s", root.ID)
			children := root.Children
			root.Children = nil
			for _, child := range children {
				if _, ok := rootSet[child]; ok {
					continue
				}
				log.Printf("Adding child as root %s", child.ID)
				rootSet[child] = struct{}{}
				m.Roots = append(m.Roots, child)
				child.Position = math.V3Sum(math.V3Rotate(child.Position, root.Rotation), root.Position)
				child.Rotation = math.CalculateChildAbsoluteEulerRotation(root.Rotation, child.Rotation)
			}
		}
	}

	durationS := time.Since(start).Seconds()
	log.Printf("Converted all terrains and doodads took %.2f s (%.2f objects/s)",
		durationS, float64(len(m.Objects))/durationS)
	return nil
}

// ReadCreatures loads creatures for each ADT tile from AzerothCore DB.
func (m *WowObjectManager) ReadCreatures(mapID int) error {
	for _, adt := range m.Terrains {
		tx, ty, ok := AsAdt(adt)
		if !ok {
			continue
		}
		log.Printf("Reading creatures in tile %d %d", tx, ty)
		creatures, err := azerothcore.GetCreaturesInTile(mapID, [2]int{tx, ty}, nil)
		if err != nil {
			return err
		}
		log.Printf("Found %d creatures in tile %d %d", len(creatures), tx, ty)
		for i := range creatures {
			c := creatures[i]
			pos := math.V3Rotate(
				math.V3Sub(
					math.V3Scale([3]float64{-c.Creature.PositionX, -c.Creature.PositionY, c.Creature.PositionZ}, m.config.RawModelScaleUp),
					adt.Position,
				),
				math.V3Negative(adt.Rotation),
			)
			unit := &WowObject{
				ID:          fmt.Sprintf("unit-%d-%d", c.Creature.GUID, c.Model.CreatureDisplayID),
				Type:        WowObjectUnit,
				Creature:    &c,
				Position:    pos,
				Rotation:    math.V3Sum([3]float64{0, 0, c.Creature.Orientation}, adt.Rotation),
				ScaleFactor: c.Model.DisplayScale,
			}
			adt.Children = append(adt.Children, unit)
			m.Objects[unit.ID] = unit
		}
	}
	return nil
}

// RotateRootsAtCenter rotates all roots around their combined center.
func (m *WowObjectManager) RotateRootsAtCenter(rotation [3]float64) {
	ext := ComputeAbsoluteMinMaxExtents(m.Roots)
	center := math.V3Mean(ext.Min, ext.Max)
	for _, obj := range m.Roots {
		diff := math.V3Sub(obj.Position, center)
		rotatedDiff := math.V3Rotate(diff, rotation)
		obj.Position = math.V3Sum(center, rotatedDiff)
		obj.Rotation = math.CalculateChildAbsoluteEulerRotation(obj.Rotation, rotation)
	}
}

func (m *WowObjectManager) relative(fullPath string) string {
	rel, err := filepath.Rel(m.config.ExportAssetDir, fullPath)
	if err != nil {
		return filepath.ToSlash(fullPath)
	}
	return filepath.ToSlash(rel)
}

func (m *WowObjectManager) full(relativePath string) string {
	return filepath.Join(m.config.ExportAssetDir, relativePath)
}

func (m *WowObjectManager) parseRecursive(objectPath string, current *WowObject, filter func(string, WowObjectType) bool) error {
	if _, ok := m.Objects[current.ID]; ok {
		return nil
	}
	m.Objects[current.ID] = current

	model, err := m.AssetManager.ResolveModel(objectPath, current.FileDataID, current.Type, false)
	if err != nil {
		return err
	}
	current.Model = model

	if IsWowAdt(current) {
		m.Terrains = append(m.Terrains, current)
		ext := ComputeAbsoluteMinMaxExtents([]*WowObject{current})
		center := math.V3Mean(ext.Min, ext.Max)
		if !isNaN3(center) {
			for _, geoset := range current.Model.MDL.Geosets {
				for _, v := range geoset.Vertices {
					vAbs := math.V3Rotate(v.Position, current.Rotation)
					vAbsCentered := math.V3Sum(vAbs, math.V3Negative(center))
					vRel := math.V3Rotate(vAbsCentered, math.V3Negative(current.Rotation))
					v.Position = vRel
				}
			}
			current.Position = center
		}
	} else {
		m.Doodads = append(m.Doodads, current)
	}

	if current.Type == WowObjectGobj {
		log.Printf("Found gobj %s %v %v", current.ID, current.Position, current.Rotation)
	}

	csvPath := m.full(filepath.Join(objectPath+"_ModelPlacementInformation.csv"))
	if !ExportAssetExists(csvPath) {
		return nil
	}
	rows, err := parsePlacementCSV(csvPath)
	if err != nil {
		return err
	}
	for _, row := range rows {
		id := fmt.Sprintf("%s:%s:%s:%s:%s", row.FileDataID, row.ModelFile, row.PositionX, row.PositionY, row.PositionZ)
		fileName := StripModelReferenceExt(row.ModelFile)
		fileDataID, _ := strconv.Atoi(row.FileDataID)
		if row.Type == "" {
			row.Type = "m2"
		}
		if !IsWowObjectType(row.Type) {
			log.Printf("Invalid object type %s with id: %s", row.Type, id)
			continue
		}
		if _, ok := m.Objects[id]; ok {
			continue
		}
		if filter != nil && !filter(fileName, WowObjectType(row.Type)) {
			continue
		}
		pos, rot := convertRowPositionRotation(row, current.Type)
		child := &WowObject{
			ID:          id,
			FileDataID:  fileDataID,
			Type:        WowObjectType(row.Type),
			Position:    pos,
			Rotation:    rot,
			ScaleFactor: parseFloat(row.ScaleFactor),
			Children:    nil,
		}
		for i := range child.Position {
			child.Position[i] *= m.config.RawModelScaleUp
		}
		if current.Type == WowObjectADT {
			wmoParentFixedRotation := [3]float64{0, 0, math.Radians(-90)}
			childAbsPos := math.V3Rotate(child.Position, wmoParentFixedRotation)
			delta := math.V3Sub(childAbsPos, current.Position)
			child.Position = math.V3Rotate(delta, math.V3Negative(wmoParentFixedRotation))
		}
		current.Children = append(current.Children, child)
		childObjectPath := filepath.ToSlash(filepath.Join(filepath.Dir(objectPath), fileName))
		if err := m.parseRecursive(childObjectPath, child, filter); err != nil {
			return err
		}
	}
	return nil
}

func parsePlacementCSV(file string) ([]placementInfoRow, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = ';'
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	col := map[string]int{}
	for i, name := range header {
		col[name] = i
	}
	get := func(rec []string, name string) string {
		if idx, ok := col[name]; ok && idx < len(rec) {
			return rec[idx]
		}
		return ""
	}
	var rows []placementInfoRow
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) <= 1 {
			continue
		}
		rows = append(rows, placementInfoRow{
			ModelFile:   get(rec, "ModelFile"),
			PositionX:   get(rec, "PositionX"),
			PositionY:   get(rec, "PositionY"),
			PositionZ:   get(rec, "PositionZ"),
			RotationX:   get(rec, "RotationX"),
			RotationY:   get(rec, "RotationY"),
			RotationZ:   get(rec, "RotationZ"),
			RotationW:   get(rec, "RotationW"),
			ScaleFactor: get(rec, "ScaleFactor"),
			ModelId:     get(rec, "ModelId"),
			FileDataID:  get(rec, "FileDataID"),
			Type:        get(rec, "Type"),
		})
	}
	return rows, nil
}

func convertRowPositionRotation(row placementInfoRow, parentType WowObjectType) ([3]float64, [3]float64) {
	if parentType == WowObjectADT {
		px, _ := strconv.ParseFloat(row.PositionX, 64)
		py, _ := strconv.ParseFloat(row.PositionY, 64)
		pz, _ := strconv.ParseFloat(row.PositionZ, 64)
		rz, _ := strconv.ParseFloat(row.RotationZ, 64)
		rx, _ := strconv.ParseFloat(row.RotationX, 64)
		ry, _ := strconv.ParseFloat(row.RotationY, 64)
		return [3]float64{
				maxWoWSize - px,
				-(maxWoWSize - pz),
				py,
			}, [3]float64{
				math.Radians(rz),
				math.Radians(rx),
				math.Radians(ry + 90),
			}
	}
	return [3]float64{
			parseFloat(row.PositionX),
			parseFloat(row.PositionY),
			parseFloat(row.PositionZ),
		}, math.QuaternionToEuler([4]float64{
			parseFloat(row.RotationX),
			parseFloat(row.RotationY),
			parseFloat(row.RotationZ),
			parseFloat(row.RotationW),
		})
}

func isEmptyModel(obj *WowObject) bool {
	if obj.Model == nil || obj.Model.MDL == nil {
		return true
	}
	mdlModel := obj.Model.MDL.Model
	for _, geoset := range obj.Model.MDL.Geosets {
		if len(geoset.Vertices) > 0 {
			size := math.V3Sub(mdlModel.MaximumExtent, mdlModel.MinimumExtent)
			for _, v := range size {
				if v == 0 {
					return true
				}
			}
			return false
		}
	}
	return true
}

func isNaN3(v [3]float64) bool {
	return v[0] != v[0] || v[1] != v[1] || v[2] != v[2]
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

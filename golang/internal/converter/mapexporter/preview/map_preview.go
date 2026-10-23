package preview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/liquid"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

// MapPreviewCacheDirectory is persistent across ordinary browse-session cleanup.
const MapPreviewCacheDirectory = "_map-preview-v5"

// PreviewPart is an immutable static triangle buffer (position/normal/UV,
// eight little-endian float32 values per vertex), with an optional small PNG.
type PreviewPart struct {
	Mesh     string         `json:"mesh"`
	Texture  string         `json:"texture,omitempty"`
	Vertices int            `json:"vertices"`
	Liquid   *PreviewLiquid `json:"liquid,omitempty"`
}

// PreviewLiquid bounds share the containing part's local coordinate system.
type PreviewLiquid struct {
	TypeID uint16        `json:"typeId"`
	Min    imath.Vector3 `json:"min"`
	Max    imath.Vector3 `json:"max"`
}

type PreviewAsset struct {
	ID    string        `json:"id"`
	Parts []PreviewPart `json:"parts"`
}

type PreviewInstance struct {
	Asset     string      `json:"asset"`
	Kind      string      `json:"kind"`
	Root      bool        `json:"root,omitempty"` // ADTs are implicit roots.
	Transform [16]float64 `json:"transform"`
}

type PreviewNPC struct {
	Faction  int           `json:"faction"`
	GUID     int64         `json:"guid"`
	Name     string        `json:"name"`
	Position imath.Vector3 `json:"position"`
	GroundZ  *float64      `json:"groundZ"`
	TerrainZ *float64      `json:"terrainZ"`
}

// MapPreview contains source-space static geometry. Form edits never rebuild it.
type MapPreview struct {
	Assets            []PreviewAsset    `json:"assets"`
	Instances         []PreviewInstance `json:"instances"`
	NPCs              []PreviewNPC      `json:"npcs"`
	Min               imath.Vector3     `json:"min"`
	Max               imath.Vector3     `json:"max"`
	RawModelScale     float64           `json:"rawModelScale"`
	GroundHeightRange float64           `json:"groundHeightRange"`
	GroundMinZ        float64           `json:"groundMinZ"`
	WaterHeightRange  [2]float64        `json:"waterHeightRange"`
	LiquidTypes       map[uint16]int    `json:"liquidTypes"`
	Warnings          []string          `json:"warnings"`
}

type PreviewOptions struct {
	Client                     client.Client
	CacheDir, BuildKey, MapDir string
	MapID                      int
	Tiles                      []settings.TileCoord
	NPCsSupported              bool
	SuggestOnly                bool // ADT bounds and NPC elevations; skip WMO work.
	Progress                   func(PreviewProgress)
}

type PreviewProgress struct {
	Message   string `json:"message"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
}

type previewChunk struct {
	Position  [3]float32
	Heights   []float32
	Holes     uint16
	HighHoles []uint8
	Flags     uint32
}

type previewPlacement struct {
	ID                 int
	UID                uint32
	Position, Rotation [3]float32
	Scale              float64
}

type previewTile struct {
	Min, Max   imath.Vector3
	Origin     imath.Vector3
	Chunks     []previewChunk
	Placements []previewPlacement
	Asset      PreviewAsset
}

type previewBuilder struct {
	ctx                                context.Context
	options                            PreviewOptions
	prefix                             string
	wdt                                *adt.WDTLoader
	warnings                           []string
	wmoRoots                           map[int]*wmo.Loader
	wmoGroupsCompleted, wmoGroupsTotal int
}

var previewAssetLocks sync.Map

// BuildMapPreview reads only static ADT/WMO source chunks and small texture
// mipmaps. It never exports NPC/M2 models or invokes the conversion/bake path.
func BuildMapPreview(ctx context.Context, options PreviewOptions) (MapPreview, error) {
	options.MapDir = strings.ToLower(options.MapDir)
	ordered := append([]settings.TileCoord(nil), options.Tiles...)
	settings.SortTileCoords(ordered)
	options.Tiles = ordered
	b := previewBuilder{ctx: ctx, options: options, prefix: filepath.ToSlash(filepath.Join(MapPreviewCacheDirectory, previewHash(options.BuildKey)))}
	key := fmt.Sprintf("terrain-bounds-v3:%s:%v:%t:%t:%s", options.MapDir, ordered, options.NPCsSupported, options.SuggestOnly, azerothcore.CreatureDataRevision())
	var result MapPreview
	if err := b.progress("Checking preview cache", 0, 1); err != nil {
		return result, err
	}
	err := b.cached(b.prefix+"/scene-"+previewHash(key)+".json", &result, func() error { var err error; result, err = buildMapPreview(ctx, options); return err })
	if err == nil {
		err = b.checkBuild()
	}
	if err == nil {
		err = b.progress("Preview ready", 1, 1)
	}
	return result, err
}

// SuggestPreviewClamp uses the same clamp algorithm as the production exporter.
func SuggestPreviewClamp(p MapPreview, unitScale float64) settings.TerrainClampPercent {
	heights := make([]float64, len(p.NPCs))
	for i, n := range p.NPCs {
		heights[i] = n.Position[2] * p.RawModelScale
	}
	return mapterrain.SuggestTerrainClamp(imath.V3Scale(p.Min, p.RawModelScale), imath.V3Scale(p.Max, p.RawModelScale), heights, unitScale, settings.TerrainClampPercent{Lower: 0, Upper: 1})
}

func buildMapPreview(ctx context.Context, options PreviewOptions) (MapPreview, error) {
	b := previewBuilder{ctx: ctx, options: options, prefix: filepath.ToSlash(filepath.Join(MapPreviewCacheDirectory, previewHash(options.BuildKey)))}
	var result MapPreview
	result.Min, result.Max = imath.V3All(math.Inf(1)), imath.V3All(math.Inf(-1))
	result.Assets, result.Instances, result.NPCs, result.Warnings = []PreviewAsset{}, []PreviewInstance{}, []PreviewNPC{}, []string{}
	result.RawModelScale, result.GroundHeightRange = config.DefaultConfig().RawModelScaleUp, config.MaxGameHeightDiff()
	result.GroundMinZ = config.DataHeightToGameZ(float64(config.DataHeightMin()))
	result.WaterHeightRange = [2]float64{config.WaterHeightToGameZ(0), config.WaterHeightToGameZ(0x3fff)}
	result.LiquidTypes = map[uint16]int{}
	ordered := append([]settings.TileCoord(nil), options.Tiles...)
	settings.SortTileCoords(ordered)
	var tiles []previewTile
	placements := map[uint32]bool{}
	assetIndex := map[string]bool{}
	var floors []previewFloor
	for tileIndex, tile := range ordered {
		if err := b.progress("Preparing terrain", tileIndex, len(ordered)); err != nil {
			return result, err
		}
		t, err := b.tile(tile)
		if err != nil {
			return result, fmt.Errorf("preview tile %d,%d: %w", tile.X, tile.Y, err)
		}
		tiles = append(tiles, t)
		result.Min = imath.V3Min(result.Min, t.Min)
		result.Max = imath.V3Max(result.Max, t.Max)
		if t.Asset.ID != "" {
			result.Assets = append(result.Assets, t.Asset)
			result.Instances = append(result.Instances, PreviewInstance{Asset: t.Asset.ID, Kind: "adt", Transform: previewTransform(t.Origin, imath.Vector3{}, 1)})
		}
	}
	if err := b.progress("Preparing terrain", len(ordered), len(ordered)); err != nil {
		return result, err
	}
	var models []previewPlacement
	for _, t := range tiles {
		if options.SuggestOnly {
			break
		}
		for _, p := range t.Placements {
			if placements[p.UID] {
				continue
			}
			placements[p.UID] = true
			models = append(models, p)
		}
	}
	// Count unique source groups before processing so the denominator stays fixed
	// across WMOs, repeated placements and per-asset cache hits.
	var wmoIDs []int
	seenWMO := map[int]bool{}
	for _, p := range models {
		if !seenWMO[p.ID] {
			seenWMO[p.ID] = true
			wmoIDs = append(wmoIDs, p.ID)
		}
	}
	for i, id := range wmoIDs {
		if err := b.progress("Counting WMO groups", i, len(wmoIDs)); err != nil {
			return result, err
		}
		root, err := b.wmoRoot(id)
		if err != nil {
			return result, fmt.Errorf("preview WMO %d: %w", id, err)
		}
		b.wmoGroupsTotal += int(root.GroupCount)
	}
	if b.wmoGroupsTotal > 0 {
		if err := b.progress("Preparing WMO groups", 0, b.wmoGroupsTotal); err != nil {
			return result, err
		}
	}
	for _, p := range models {
		a, err := b.wmo(p.ID)
		if err != nil {
			return result, fmt.Errorf("preview WMO %d: %w", p.ID, err)
		}
		if !assetIndex[a.Asset.ID] {
			b.wmoGroupsCompleted += int(b.wmoRoots[p.ID].GroupCount)
			if err := b.progress("Preparing WMO groups", b.wmoGroupsCompleted, b.wmoGroupsTotal); err != nil {
				return result, err
			}
			assetIndex[a.Asset.ID] = true
			result.Assets = append(result.Assets, a.Asset)
		}
		position := imath.Vector3{float64(p.Position[0]) - 51200./3, 51200./3 - float64(p.Position[2]), float64(p.Position[1])}
		rotation := imath.Vector3{imath.Radians(float64(p.Rotation[2])), imath.Radians(float64(p.Rotation[0])), imath.Radians(float64(p.Rotation[1]) + 90)}
		matrix := previewTransform(position, rotation, p.Scale)
		// ADT placement axes contain a 180-degree root rotation at map angle 0.
		for _, i := range []int{0, 1, 4, 5, 8, 9} {
			matrix[i] = -matrix[i]
		}
		result.Instances = append(result.Instances, PreviewInstance{Asset: a.Asset.ID, Kind: "wmo", Transform: matrix})
		for _, f := range a.Floors {
			for i, p := range f.V {
				f.V[i] = previewApply(matrix, p)
			}
			floors = append(floors, f)
		}
	}
	if len(tiles) == 0 || math.IsInf(result.Min[0], 1) {
		return result, fmt.Errorf("no preview terrain found")
	}
	// Resolve types once per build, only when the scene contains liquids. The
	// suggestion path never needs another client table or WMO processing.
	if !options.SuggestOnly {
		if err := b.progress("Preparing water", 0, 1); err != nil {
			return result, err
		}
		for _, a := range result.Assets {
			for _, p := range a.Parts {
				if p.Liquid != nil && len(result.LiquidTypes) == 0 {
					err := b.cached(b.prefix+"/liquid-types.json", &result.LiquidTypes, func() error {
						var err error
						result.LiquidTypes, err = common.NewAssetManager(config.DefaultConfig(), options.Client, nil).LiquidSoundBanks(ctx)
						return err
					})
					if err != nil {
						return result, err
					}
				}
			}
		}
		if err := b.progress("Preparing water", 1, 1); err != nil {
			return result, err
		}
	}
	floorBins := map[[2]int][]int{}
	for i, f := range floors {
		if i%max(1, len(floors)/100) == 0 {
			if err := b.progress("Preparing walkable surfaces", i, len(floors)); err != nil {
				return result, err
			}
		}
		if n := mapterrain.NavigationCross(imath.V3Sub(f.V[1], f.V[0]), imath.V3Sub(f.V[2], f.V[0])); n[2] <= 0 {
			continue
		}
		lo, hi := f.V[0], f.V[0]
		for _, v := range f.V {
			lo = imath.V3Min(lo, v)
			hi = imath.V3Max(hi, v)
		}
		for y := int(math.Floor(lo[1] / 32)); y <= int(math.Floor(hi[1]/32)); y++ {
			for x := int(math.Floor(lo[0] / 32)); x <= int(math.Floor(hi[0]/32)); x++ {
				floorBins[[2]int{x, y}] = append(floorBins[[2]int{x, y}], i)
			}
		}
	}
	if len(floors) > 0 {
		if err := b.progress("Preparing walkable surfaces", len(floors), len(floors)); err != nil {
			return result, err
		}
	}
	if options.NPCsSupported && azerothcore.CreatureDataRevision() == "missing" {
		b.warnings = append(b.warnings, "NPC data is unavailable; showing geometry only")
	} else if options.NPCsSupported {
		seen := map[int64]bool{}
		for i, tile := range ordered {
			if err := b.progress("Loading NPC data", i, len(ordered)); err != nil {
				return result, err
			}
			creatures, err := azerothcore.GetCreaturesInTile(options.MapID, [2]int{tile.X, tile.Y}, nil)
			if err != nil {
				return result, err
			}
			for _, c := range creatures {
				if seen[c.Creature.GUID] {
					continue
				}
				seen[c.Creature.GUID] = true
				p := imath.Vector3{-c.Creature.PositionY, c.Creature.PositionX, c.Creature.PositionZ}
				z, ok := previewGround(tiles[i].Chunks, p)
				var ground *float64
				if ok {
					ground = &z
				}
				terrainZ := ground
				for _, id := range floorBins[[2]int{int(math.Floor(p[0] / 32)), int(math.Floor(p[1] / 32))}] {
					f := floors[id]
					h, inside := mapterrain.NavigationHeight(f.V, p[0], p[1])
					if !inside {
						continue
					}
					if ground == nil || math.Abs(h-p[2]) < math.Abs(*ground-p[2]) {
						value := h
						ground = &value
					}
				}
				name := c.Template.Name
				if name == "" {
					name = c.Template.SubName
				}
				faction := 0
				if c.Gameplay != nil {
					faction = c.Gameplay.Faction
				}
				result.NPCs = append(result.NPCs, PreviewNPC{GUID: c.Creature.GUID, Name: name, Position: p, GroundZ: ground, TerrainZ: terrainZ, Faction: faction})
			}
		}
		if err := b.progress("Loading NPC data", len(ordered), len(ordered)); err != nil {
			return result, err
		}
	}
	sort.Slice(result.NPCs, func(i, j int) bool { return result.NPCs[i].GUID < result.NPCs[j].GUID })
	result.Warnings = append(result.Warnings, b.warnings...)
	return result, ctx.Err()
}

func (b *previewBuilder) progress(message string, completed, total int) error {
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if b.options.Progress != nil {
		b.options.Progress(PreviewProgress{Message: message, Completed: completed, Total: total})
	}
	return b.ctx.Err()
}

func previewHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:12])
}

func (b *previewBuilder) full(path string) string {
	return filepath.Join(b.options.CacheDir, filepath.FromSlash(path))
}

func (b *previewBuilder) cached(path string, out any, build func() error) error {
	lock, _ := previewAssetLocks.LoadOrStore(b.full(path), &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if raw, err := os.ReadFile(b.full(path)); err == nil && json.Unmarshal(raw, out) == nil && b.assetsPresent(out) {
		return nil
	}
	if err := build(); err != nil {
		return err
	}
	if err := b.ctx.Err(); err != nil {
		return err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return b.write(path, raw)
}

// Manifests are reusable only while their binary/texture dependencies exist.
// Clearing part of browse assets must not leave a permanently broken preview.
func (b *previewBuilder) assetsPresent(out any) bool {
	var assets []PreviewAsset
	switch value := out.(type) {
	case *MapPreview:
		if len(value.Instances) == 0 {
			return false
		}
		assets = value.Assets
	case *previewTile:
		if len(value.Chunks) == 0 || value.Asset.ID == "" {
			return false
		}
		assets = []PreviewAsset{value.Asset}
	case *previewWMO:
		if value.Asset.ID == "" {
			return false
		}
		assets = []PreviewAsset{value.Asset}
		if info, err := os.Stat(b.full(value.FloorFile)); value.FloorFile == "" || err != nil || info.Size() != int64(value.FloorCount)*36 {
			return false
		}
	}
	for _, asset := range assets {
		for _, part := range asset.Parts {
			info, err := os.Stat(b.full(part.Mesh))
			if err != nil || info.Size() != int64(part.Vertices)*32 {
				return false
			}
			if part.Texture != "" {
				if info, err = os.Stat(b.full(part.Texture)); err != nil || info.Size() == 0 {
					return false
				}
			}
		}
	}
	return true
}

func (b *previewBuilder) write(path string, raw []byte) error {
	if err := b.checkBuild(); err != nil {
		return err
	}
	full := b.full(path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(full), ".preview-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(raw)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = b.ctx.Err(); err != nil {
		return err
	}
	return os.Rename(f.Name(), full)
}

func (b *previewBuilder) file(id uint32, name string) ([]byte, error) {
	if err := b.checkBuild(); err != nil {
		return nil, err
	}
	if id == 0 {
		entry, err := b.options.Client.GetFileByName(b.ctx, name)
		if err != nil {
			return nil, err
		}
		id = uint32(entry.FileDataID)
	}
	return b.options.Client.DownloadCascFile(b.ctx, int(id))
}

// A build switch must never publish mixed-build files into the old cache prefix.
func (b *previewBuilder) checkBuild() error {
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if b.options.Client == nil || b.options.BuildKey == "" {
		return nil
	}
	info, err := b.options.Client.GetCASCInfo(b.ctx)
	if err != nil {
		return err
	}
	if info.BuildKey != b.options.BuildKey {
		return fmt.Errorf("WoW build changed; reopen the map preview")
	}
	return nil
}

func (b *previewBuilder) descriptor() error {
	if b.wdt != nil {
		return nil
	}
	var descriptor adt.WDTLoader
	path := b.prefix + "/map-" + previewHash(b.options.MapDir) + ".json"
	err := b.cached(path, &descriptor, func() error {
		raw, err := b.file(0, fmt.Sprintf("world/maps/%s/%s.wdt", b.options.MapDir, b.options.MapDir))
		if err != nil {
			return err
		}
		l := adt.NewWDTLoader(buffer.From(raw))
		l.Load()
		descriptor = *l
		descriptor.Data = nil
		return nil
	})
	b.wdt = &descriptor
	return err
}

func (b *previewBuilder) tile(tile settings.TileCoord) (previewTile, error) {
	var t previewTile
	stem := fmt.Sprintf("%s/adt-bounds-v2-%s-%d-%d", b.prefix, previewHash(b.options.MapDir), tile.X, tile.Y)
	err := b.cached(stem+".json", &t, func() error {
		t = previewTile{}
		if err := b.descriptor(); err != nil {
			return err
		}
		entry := adt.WDTEntry{}
		if i := tile.X*64 + tile.Y; i < len(b.wdt.Entries) {
			entry = b.wdt.Entries[i]
		}
		base := fmt.Sprintf("world/maps/%s/%s_%d_%d", b.options.MapDir, b.options.MapDir, tile.X, tile.Y)
		raw, err := b.file(entry.RootADT, base+".adt")
		if err != nil {
			return err
		}
		root := adt.NewADTLoader(buffer.From(raw))
		root.LoadRoot()
		t.Min, t.Max = imath.V3All(math.Inf(1)), imath.V3All(math.Inf(-1))
		for _, ch := range root.Chunks {
			if len(ch.Vertices) != 145 {
				continue
			}
			t.Chunks = append(t.Chunks, previewChunk{Position: ch.Position, Heights: ch.Vertices, Holes: ch.HolesLowRes, HighHoles: ch.HolesHighRes, Flags: ch.Flags})
			used := previewTerrainVertices(t.Chunks[len(t.Chunks)-1])
			for row := 0; row < 17; row++ {
				n := 9
				if row%2 == 1 {
					n = 8
				}
				for x := 0; x < n; x++ {
					if !used[row/2*17+(row%2)*9+x] {
						continue
					}
					p := previewADTVertex(ch, row, x)
					t.Min = imath.V3Min(t.Min, p)
					t.Max = imath.V3Max(t.Max, p)
				}
			}
		}
		if len(t.Chunks) == 0 {
			return fmt.Errorf("ADT contains no terrain vertices")
		}
		t.Origin = imath.V3Mean(t.Min, t.Max)
		t.Asset = PreviewAsset{ID: stem, Parts: []PreviewPart{{Mesh: stem + ".bin", Vertices: 0}}}
		vertices := previewTerrainMesh(t)
		t.Asset.Parts[0].Vertices = len(vertices) / 8
		if err := b.write(stem+".bin", previewVertexBytes(vertices)); err != nil {
			return err
		}
		surfaces, err := root.LiquidSurfaces()
		if err != nil {
			return err
		}
		// ADT liquid heights are absolute; unlike terrain they never add chunk Z.
		surfaces = liquid.Transform(surfaces, func(p imath.Vector3) imath.Vector3 {
			return imath.V3Sub(imath.Vector3{-p[1], p[0], p[2]}, t.Origin)
		})
		if err := b.addLiquids(&t.Asset, surfaces); err != nil {
			return err
		}
		texture, err := b.texture(entry.MinimapTexture, fmt.Sprintf("world/minimaps/%s/map%02d_%02d.blp", b.options.MapDir, tile.X, tile.Y), stem+".png")
		if err == nil {
			t.Asset.Parts[0].Texture = texture
		} else {
			b.warnings = append(b.warnings, "Minimap texture unavailable; showing terrain geometry")
		}
		objRaw := raw
		if entry.Obj0ADT != 0 {
			objRaw, err = b.file(entry.Obj0ADT, "")
			if err != nil {
				return err
			}
		} else if split, e := b.file(0, base+"_obj0.adt"); e == nil {
			objRaw = split
		}
		objects := adt.NewADTLoader(buffer.From(objRaw))
		objects.LoadObj()
		for _, p := range objects.WorldModels {
			id := int(p.MwidEntry)
			if len(objects.WmoNames) > 0 {
				if int(p.MwidEntry) >= len(objects.WmoOffsets) {
					return fmt.Errorf("invalid WMO name index")
				}
				name, ok := objects.WmoNames[int(objects.WmoOffsets[p.MwidEntry])]
				if !ok {
					return fmt.Errorf("missing WMO name")
				}
				f, err := b.options.Client.GetFileByName(b.ctx, name)
				if err != nil {
					return err
				}
				id = f.FileDataID
			}
			scale := float64(p.Scale) / 1024
			if scale == 0 {
				scale = 1
			}
			t.Placements = append(t.Placements, previewPlacement{ID: id, UID: p.UniqueID, Position: p.Position, Rotation: p.Rotation, Scale: scale})
		}
		return nil
	})
	return t, err
}

func previewTransform(position, rotation imath.Vector3, scale float64) (m [16]float64) {
	for axis := 0; axis < 3; axis++ {
		v := imath.Vector3{}
		v[axis] = scale
		v = imath.V3Rotate(v, rotation)
		for row := 0; row < 3; row++ {
			m[axis*4+row] = v[row]
		}
	}
	copy(m[12:15], position[:])
	m[15] = 1
	return
}

func previewApply(m [16]float64, p imath.Vector3) imath.Vector3 {
	return imath.Vector3{m[0]*p[0] + m[4]*p[1] + m[8]*p[2] + m[12], m[1]*p[0] + m[5]*p[1] + m[9]*p[2] + m[13], m[2]*p[0] + m[6]*p[1] + m[10]*p[2] + m[14]}
}

func previewGround(chunks []previewChunk, p imath.Vector3) (float64, bool) {
	for _, ch := range chunks {
		x, y := (-p[0]-float64(ch.Position[1]))/(-533.3333333333333/128), (float64(ch.Position[0])-p[1])/(533.3333333333333/128)
		if x < 0 || y < 0 || x >= 8 || y >= 8 {
			continue
		}
		xi, yi := int(x), int(y)
		if previewHole(ch, xi, yi) {
			return 0, false
		}
		u, v := x-float64(xi), y-float64(yi)
		base := float64(ch.Position[2])
		a := float64(ch.Heights[yi*17+xi])
		bb := float64(ch.Heights[yi*17+xi+1])
		c := float64(ch.Heights[(yi+1)*17+xi+1])
		d := float64(ch.Heights[(yi+1)*17+xi])
		mid := float64(ch.Heights[yi*17+9+xi])
		// WoW uses four triangles around the center vertex, not a bilinear quad.
		verts := [5]imath.Vector3{{0, 0, a}, {1, 0, bb}, {1, 1, c}, {0, 1, d}, {.5, .5, mid}}
		for _, ids := range [][3]int{{0, 1, 4}, {1, 2, 4}, {2, 3, 4}, {3, 0, 4}} {
			if z, ok := mapterrain.NavigationHeight([3]imath.Vector3{verts[ids[0]], verts[ids[1]], verts[ids[2]]}, u, v); ok {
				return base + z, true
			}
		}
	}
	return 0, false
}

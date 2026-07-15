package common

import (
	"sync"

	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
)

// TileRegistry holds in-memory ADT tile snapshots for direct map conversion.
type TileRegistry struct {
	mu              sync.RWMutex
	byObjectPath    map[string]*exportadt.ConversionOutput
	placementsByObj map[string][]exportadt.PlacementRow
}

// NewTileRegistry creates an empty tile registry.
func NewTileRegistry() *TileRegistry {
	return &TileRegistry{
		byObjectPath:    map[string]*exportadt.ConversionOutput{},
		placementsByObj: map[string][]exportadt.PlacementRow{},
	}
}

// Register adds a loaded tile snapshot.
func (r *TileRegistry) Register(snapshot *exportadt.ConversionOutput) {
	if snapshot == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byObjectPath[snapshot.ObjectPath] = snapshot
	if len(snapshot.Placements) > 0 {
		r.placementsByObj[snapshot.ObjectPath] = append([]exportadt.PlacementRow(nil), snapshot.Placements...)
	}
	for wmoPath, rows := range snapshot.WmoPlacements {
		r.placementsByObj[wmoPath] = append([]exportadt.PlacementRow(nil), rows...)
	}
}

// Snapshot returns the terrain snapshot for an ADT object path.
func (r *TileRegistry) Snapshot(objectPath string) *exportadt.ConversionOutput {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byObjectPath[objectPath]
}

// Placements returns inline placement rows for an object path (ADT tile or WMO).
func (r *TileRegistry) Placements(objectPath string) []exportadt.PlacementRow {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.placementsByObj[objectPath]
}

// HasTiles reports whether any tiles were registered.
func (r *TileRegistry) HasTiles() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byObjectPath) > 0
}

// RegisterTerrainTextures registers baked terrain PNGs in the texture source registry.
// PNG bytes are moved out of tile snapshots so they are not held twice in memory.
func (r *TileRegistry) RegisterTerrainTextures() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, snap := range r.byObjectPath {
		moveTerrainTexturesToSource(snap)
	}
}

// RegisterTerrainTexturesFor moves baked terrain PNGs for one loaded tile into texturesource.
func (r *TileRegistry) RegisterTerrainTexturesFor(objectPath string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	moveTerrainTexturesToSource(r.byObjectPath[objectPath])
}

func moveTerrainTexturesToSource(snap *exportadt.ConversionOutput) {
	if snap == nil {
		return
	}
	for i := range snap.Textures {
		tex := &snap.Textures[i]
		if len(tex.PNG) == 0 {
			continue
		}
		texturesource.Register(tex.RelPath, texturesource.Source{Kind: texturesource.KindPNG, PNG: tex.PNG})
		tex.PNG = nil
	}
}

// TrimAfterParse drops tile mesh/placement payloads kept on snapshots after models are parsed.
func (r *TileRegistry) TrimAfterParse() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, snap := range r.byObjectPath {
		if snap == nil {
			continue
		}
		snap.ObjText = ""
		snap.MtlText = ""
		snap.Textures = nil
		snap.Placements = nil
		snap.WmoPlacements = nil
	}
	r.byObjectPath = map[string]*exportadt.ConversionOutput{}
}

// Release drops tile snapshots and in-memory terrain PNG registry entries.
func (r *TileRegistry) Release() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var paths []string
	for _, snap := range r.byObjectPath {
		for _, tex := range snap.Textures {
			paths = append(paths, tex.RelPath)
		}
		if snap != nil {
			snap.Textures = nil
			snap.ObjText = ""
			snap.MtlText = ""
			snap.Placements = nil
			snap.WmoPlacements = nil
		}
	}
	texturesource.ReleasePaths(paths)
	r.byObjectPath = map[string]*exportadt.ConversionOutput{}
	r.placementsByObj = map[string][]exportadt.PlacementRow{}
}

// ObjectPaths returns registered ADT object paths in stable order.
func (r *TileRegistry) ObjectPaths() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byObjectPath))
	for path := range r.byObjectPath {
		out = append(out, path)
	}
	return out
}

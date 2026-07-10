package export

import (
	"sync"

	apicasc "github.com/pqhuy98/wow-converter/internal/wow/casc"
)

const FixedOverheadSteps = 2

// ADTExportOptions are normalized ADT export settings.
type ADTExportOptions struct {
	PathFormat             string
	EnableSharedTextures   bool
	OverwriteFiles         bool
	SplitAlphaMaps         bool
	SplitLargeTerrainBakes bool
	MapsIncludeHoles       bool
	EnableSharedChildren   bool
	EnableAbsoluteCSVPaths bool
	ModelsExportCollision  bool
	MapsIncludeWMO         bool
	MapsIncludeM2          bool
	MapsIncludeWMOSets     bool
	ExportFoliageMeta      bool
	MapsIncludeFoliage     bool
	MapsIncludeLiquid      bool
	MapsIncludeGameObjects bool
}

// ComputeStepsPerTile returns the fixed phase budget per tile.
func ComputeStepsPerTile(quality int, options ADTExportOptions) int {
	steps := FixedOverheadSteps
	if quality != 0 {
		steps++
	}
	if options.MapsIncludeM2 || options.MapsIncludeWMO || options.MapsIncludeGameObjects {
		steps++
	}
	if options.MapsIncludeLiquid {
		steps++
	}
	if options.MapsIncludeFoliage {
		steps++
	}
	return steps
}

var progressStore sync.Map

// BatchExportProgressParams configures batch export progress tracking.
type BatchExportProgressParams struct {
	Key          string
	TileIndex    int
	TileCount    int
	StepsPerTile int
	CurrentTile  apicasc.TileCoord
}

// ProgressReporter tracks export progress for a single tile.
type ProgressReporter struct {
	key, taskName              string
	tileIndex, tileCount       int
	stepsPerTile, tileSteps    int
	totalSteps, tileBase       int
	currentTile                apicasc.TileCoord
	taskValue, taskMax         *int
}

// CreateBatchExportProgress creates a progress reporter.
func CreateBatchExportProgress(params BatchExportProgressParams) *ProgressReporter {
	totalSteps := params.TileCount * params.StepsPerTile
	tileBase := params.TileIndex * params.StepsPerTile
	p := &ProgressReporter{
		key: params.Key, tileIndex: params.TileIndex, tileCount: params.TileCount,
		stepsPerTile: params.StepsPerTile, totalSteps: totalSteps, tileBase: tileBase,
		currentTile: params.CurrentTile,
	}
	p.publish(tileBase)
	return p
}

func (p *ProgressReporter) publish(completed int) {
	snap := apicasc.ExportProgressSnapshot{
		CompletedSteps: completed,
		TotalSteps:     p.totalSteps,
		TileIndex:      p.tileIndex,
		TileCount:      p.tileCount,
		StepsPerTile:   p.stepsPerTile,
		CurrentTile:    &p.currentTile,
	}
	if p.taskName != "" {
		snap.TaskName = p.taskName
	}
	if p.taskValue != nil {
		snap.TaskValue = *p.taskValue
	}
	if p.taskMax != nil {
		snap.TaskMax = *p.taskMax
	}
	progressStore.Store(p.key, snap)
}

// Advance increments completed steps within the current tile.
func (p *ProgressReporter) Advance(steps ...int) {
	n := 1
	if len(steps) > 0 {
		n = steps[0]
	}
	max := p.stepsPerTile - 1
	if max < 0 {
		max = 0
	}
	p.tileSteps += n
	if p.tileSteps > max {
		p.tileSteps = max
	}
	p.publish(p.tileBase + p.tileSteps)
}

// SetLabel updates the current task label.
func (p *ProgressReporter) SetLabel(name string, value ...int) {
	p.taskName = name
	if len(value) > 0 {
		v := value[0]
		p.taskValue = &v
	}
	if len(value) > 1 {
		v := value[1]
		p.taskMax = &v
	}
	p.publish(p.tileBase + p.tileSteps)
}

// SyncTileComplete marks the tile as fully complete.
func (p *ProgressReporter) SyncTileComplete() {
	p.tileSteps = p.stepsPerTile
	p.publish(p.tileBase + p.stepsPerTile)
}

// GetExportProgressSnapshot returns progress for a key.
func GetExportProgressSnapshot(key string) (*apicasc.ExportProgressSnapshot, bool) {
	if v, ok := progressStore.Load(key); ok {
		snap := v.(apicasc.ExportProgressSnapshot)
		return &snap, true
	}
	return nil, false
}

// FinalizeExportProgress marks a batch export complete.
func FinalizeExportProgress(key string) {
	if v, ok := progressStore.Load(key); ok {
		snap := v.(apicasc.ExportProgressSnapshot)
		snap.CompletedSteps = snap.TotalSteps
		snap.TaskName = "Complete"
		snap.TaskValue = 0
		snap.TaskMax = 0
		progressStore.Store(key, snap)
	}
}

// ClearExportProgressSnapshot removes a progress entry.
func ClearExportProgressSnapshot(key string) {
	progressStore.Delete(key)
}

// ExportProgressService implements casc.ExportProgress.
type ExportProgressService struct{}

func (ExportProgressService) GetSnapshot(key string) (*apicasc.ExportProgressSnapshot, bool) {
	return GetExportProgressSnapshot(key)
}

func (ExportProgressService) Finalize(key string) (*apicasc.ExportProgressSnapshot, bool) {
	FinalizeExportProgress(key)
	return GetExportProgressSnapshot(key)
}

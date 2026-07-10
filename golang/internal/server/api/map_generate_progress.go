package api

import (
	"sync"
)

type mapGeneratePhase string

const (
	phaseADT     mapGeneratePhase = "adt"
	phaseConvert mapGeneratePhase = "convert"
)

type mapGenerateSnapshot struct {
	CompletedSteps   int
	TotalSteps       int
	Phase            mapGeneratePhase
	TaskName         string
	TileIndex        *int
	TileCount        *int
	StepsPerTile     *int
	CurrentTile      *tileCoord
	CreatureCompleted *int
	CreatureTotal    *int
}

type tileCoord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

var (
	mapGenerateMu    sync.Mutex
	mapGenerateStore = map[string]*mapGenerateSnapshot{}
)

func initMapGenerateProgress(key string, adtTotalSteps, convertSteps, tileCount, stepsPerTile int) {
	mapGenerateMu.Lock()
	defer mapGenerateMu.Unlock()
	ti := 0
	tc := tileCount
	spt := stepsPerTile
	mapGenerateStore[key] = &mapGenerateSnapshot{
		CompletedSteps: 0,
		TotalSteps:     adtTotalSteps + convertSteps,
		Phase:          phaseADT,
		TaskName:       "Exporting tiles",
		TileIndex:      &ti,
		TileCount:      &tc,
		StepsPerTile:   &spt,
	}
}

func updateMapGenerateTotalSteps(key string, convertSteps, adtTotalSteps int) {
	mapGenerateMu.Lock()
	defer mapGenerateMu.Unlock()
	if snap, ok := mapGenerateStore[key]; ok {
		snap.TotalSteps = adtTotalSteps + convertSteps
	}
}

func setMapGeneratePhase(key string, phase mapGeneratePhase, taskName string) {
	mapGenerateMu.Lock()
	defer mapGenerateMu.Unlock()
	if snap, ok := mapGenerateStore[key]; ok {
		snap.Phase = phase
		snap.TaskName = taskName
		if phase == phaseConvert {
			snap.TileIndex = nil
			snap.CurrentTile = nil
		}
	}
}

func syncAdtProgress(key string, completedSteps, tileIndex, tileCount, stepsPerTile int, currentTile *tileCoord, taskName string) {
	mapGenerateMu.Lock()
	defer mapGenerateMu.Unlock()
	snap, ok := mapGenerateStore[key]
	if !ok {
		return
	}
	snap.Phase = phaseADT
	snap.CompletedSteps = completedSteps
	ti := tileIndex
	tc := tileCount
	spt := stepsPerTile
	snap.TileIndex = &ti
	snap.TileCount = &tc
	snap.StepsPerTile = &spt
	snap.CurrentTile = currentTile
	if taskName != "" {
		snap.TaskName = taskName
	} else {
		snap.TaskName = "Exporting tiles"
	}
}

func advanceMapGenerateProgress(key string, adtTotalSteps, convertCompleted int, taskName string, creatureCompleted, creatureTotal *int) {
	mapGenerateMu.Lock()
	defer mapGenerateMu.Unlock()
	snap, ok := mapGenerateStore[key]
	if !ok {
		return
	}
	snap.CompletedSteps = adtTotalSteps + convertCompleted
	snap.TaskName = taskName
	snap.CreatureCompleted = creatureCompleted
	snap.CreatureTotal = creatureTotal
}

func getMapGenerateProgress(key string) *mapGenerateSnapshot {
	mapGenerateMu.Lock()
	defer mapGenerateMu.Unlock()
	return mapGenerateStore[key]
}

func clearMapGenerateProgress(key string) {
	mapGenerateMu.Lock()
	defer mapGenerateMu.Unlock()
	delete(mapGenerateStore, key)
}

func toProgressPercent(snap *mapGenerateSnapshot) int {
	if snap == nil || snap.TotalSteps <= 0 {
		return 0
	}
	pct := (snap.CompletedSteps * 100) / snap.TotalSteps
	if pct > 100 {
		return 100
	}
	return pct
}

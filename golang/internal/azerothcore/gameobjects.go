package azerothcore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
)

// GameObject is a server spawn joined to its model template. GUID identifies
// the placement; DisplayID identifies client geometry, not a CASC file ID.
type GameObject struct {
	GUID                        int64
	Entry, Map, Type, DisplayID int
	Name                        string
	Position                    [3]float64
	Orientation                 float64
	Rotation                    [4]float64
	UseQuaternion               bool
	Scale                       float64
	SpawnMask, PhaseMask        uint32
	State, AnimProgress         int
}

// GetGameObjectsInTile reads gameobjects with half-open tile bounds.
// Server world X corresponds to ADT tile Y, and world Y to ADT tile X.
func GetGameObjectsInTile(ctx context.Context, mapID int, tile [2]int) ([]GameObject, error) {
	path := resolveDatabasePath()
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("azerothcore database: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return queryGameObjects(ctx, db, mapID, tile)
}

func queryGameObjects(ctx context.Context, db *sql.DB, mapID int, tile [2]int) ([]GameObject, error) {
	const size = 1600.0 / 3
	minX, maxX := (31-float64(tile[1]))*size, (32-float64(tile[1]))*size
	minY, maxY := (31-float64(tile[0]))*size, (32-float64(tile[0]))*size
	rows, err := db.QueryContext(ctx, `SELECT g.guid,g.id,g.map,t.type,t.displayId,t.name,
 g.position_x,g.position_y,g.position_z,g.orientation,
 g.rotation0,g.rotation1,g.rotation2,g.rotation3,t.size,g.spawnMask,g.phaseMask,g.state,g.animprogress,
 CASE WHEN g.id IN (181233,181575,20992,21042) OR (a.invisibilityType=0 AND a.invisibilityValue=0) THEN 1 ELSE 0 END
 FROM gameobject g JOIN gameobject_template t ON t.entry=g.id
 LEFT JOIN gameobject_addon a ON a.guid=g.guid
 WHERE g.map=? AND g.position_x>=? AND g.position_x<? AND g.position_y>=? AND g.position_y<?
 ORDER BY g.guid`, mapID, minX, maxX, minY, maxY)
	if err != nil {
		return nil, fmt.Errorf("query AzerothCore gameobjects (regenerate bin/azerothcore-world.sqlite with gameobject tables): %w", err)
	}
	defer rows.Close()
	var result []GameObject
	for rows.Next() {
		var g GameObject
		var phase int64
		if err := rows.Scan(&g.GUID, &g.Entry, &g.Map, &g.Type, &g.DisplayID, &g.Name,
			&g.Position[0], &g.Position[1], &g.Position[2], &g.Orientation,
			&g.Rotation[0], &g.Rotation[1], &g.Rotation[2], &g.Rotation[3], &g.Scale, &g.SpawnMask, &phase, &g.State, &g.AnimProgress, &g.UseQuaternion); err != nil {
			return nil, err
		}
		g.PhaseMask = uint32(phase)
		result = append(result, g)
	}
	return result, rows.Err()
}

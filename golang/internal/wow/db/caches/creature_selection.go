package caches

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

var selectionTables = struct {
	sync.Mutex
	tables map[string]*db.WDCReader
}{tables: map[string]*db.WDCReader{}}

// CreatureSelectionRadius follows the unmounted 3.3.5a client's
// GetStandBox/GetModelScale/GetTrueScale/UpdateSelectionRadius chain.
// The value is a world-space WoW radius, before export-only resizing.
func CreatureSelectionRadius(ctx context.Context, displayID int, o casc.NpcSelectionOptions) (float64, error) {
	selectionTables.Lock()
	defer selectionTables.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	row := func(table string, id uint32) (db.DB2Row, error) {
		r := selectionTables.tables[table]
		if r == nil {
			r = db.NewWDCReader("DBFilesClient/"+table+".db2", nil)
			if err := r.Parse(ctx, nil); err != nil {
				return nil, fmt.Errorf("load NPC selection %s: %w", table, err)
			}
			// Never retain failed/canceled loads across jobs.
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			selectionTables.tables[table] = r
		}
		return r.GetRow(id), nil
	}
	display, err := row("CreatureDisplayInfo", uint32(displayID))
	if err != nil {
		return 0, err
	}
	if display == nil {
		return 1.2, nil
	}
	model, err := row("CreatureModelData", toUint32(display["ModelID"]))
	if err != nil {
		return 0, err
	}
	if model == nil {
		return 1.2, nil
	}
	extraScale := 1.0
	if extraID := toUint32(display["ExtendedDisplayInfoID"]); extraID != 0 {
		extra, err := row("CreatureDisplayInfoExtra", extraID)
		if err != nil {
			return 0, err
		}
		if extra != nil {
			race, err := row("ChrRaces", toUint32(extra["DisplayRaceID"]))
			if err != nil {
				return 0, err
			}
			key := "MaleDisplayID"
			if toUint32(extra["DisplaySexID"]) == 1 {
				key = "FemaleDisplayID"
			}
			if race != nil {
				base, err := row("CreatureDisplayInfo", toUint32(race[key]))
				if err != nil {
					return 0, err
				}
				if base != nil {
					extraScale = selectionNumber(base["CreatureModelScale"])
				}
			}
		}
	}
	var family db.DB2Row
	if o.Family > 0 {
		family, err = row("CreatureFamily", uint32(o.Family))
		if err != nil {
			return 0, err
		}
	}
	return selectionRadiusFromRows(display, model, family, extraScale, o)
}

func resetCreatureSelectionCache() {
	selectionTables.Lock()
	selectionTables.tables = map[string]*db.WDCReader{}
	selectionTables.Unlock()
}

func selectionRadiusFromRows(display, model, family db.DB2Row, extraScale float64, o casc.NpcSelectionOptions) (float64, error) {
	minBox, maxBox, ok := selectionBounds(model)
	if !ok {
		return 0, fmt.Errorf("CreatureModelData has no supported GeoBox fields for NPC selection")
	}
	dx, dy := maxBox[0]-minBox[0], maxBox[1]-minBox[1]
	if math.Abs(dx) < .00000023841858 && math.Abs(dy) < .00000023841858 {
		return 1.2, nil
	}
	nativeScale := extraScale * selectionNumber(display["CreatureModelScale"]) * selectionNumber(model["ModelScale"])
	if nativeScale <= 0 {
		nativeScale = 1
	}
	if family != nil {
		lo, hi := selectionNumber(family["MinScaleLevel"]), selectionNumber(family["MaxScaleLevel"])
		fraction := 0.0
		if hi > lo {
			fraction = math.Max(0, math.Min(1, (float64(o.Level)-lo)/(hi-lo)))
		}
		familyScale := selectionNumber(family["MinScale"]) + fraction*(selectionNumber(family["MaxScale"])-selectionNumber(family["MinScale"]))
		nativeScale = math.Max(nativeScale, familyScale)
	}
	objectScale := o.ObjectScale
	if objectScale <= 0 {
		objectScale = 1
	}
	scaled := math.Hypot(dx, dy) * .5 * nativeScale * objectScale
	radius := math.Sqrt(scaled)
	if scaled > 5 {
		radius += (scaled - 5) * (scaled - 5) * .059999999
	}
	return math.Min(10, radius), nil
}

func selectionBounds(row db.DB2Row) ([3]float64, [3]float64, bool) {
	if lo, hi := selectionVector(row["GeoBoxMin"]), selectionVector(row["GeoBoxMax"]); len(lo) >= 3 && len(hi) >= 3 {
		return [3]float64{lo[0], lo[1], lo[2]}, [3]float64{hi[0], hi[1], hi[2]}, true
	}
	if box := selectionVector(row["GeoBox"]); len(box) >= 6 {
		return [3]float64{box[0], box[1], box[2]}, [3]float64{box[3], box[4], box[5]}, true
	}
	if _, ok := row["GeoBoxMinX"]; ok {
		return [3]float64{selectionNumber(row["GeoBoxMinX"]), selectionNumber(row["GeoBoxMinY"]), selectionNumber(row["GeoBoxMinZ"])}, [3]float64{selectionNumber(row["GeoBoxMaxX"]), selectionNumber(row["GeoBoxMaxY"]), selectionNumber(row["GeoBoxMaxZ"])}, true
	}
	return [3]float64{}, [3]float64{}, false
}

func selectionVector(v any) []float64 {
	switch a := v.(type) {
	case []float64:
		return a
	case []float32:
		out := make([]float64, len(a))
		for i, n := range a {
			out[i] = float64(n)
		}
		return out
	case []any:
		out := make([]float64, len(a))
		for i, n := range a {
			out[i] = selectionNumber(n)
		}
		return out
	}
	return nil
}

func selectionNumber(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case uint32:
		return float64(n)
	case uint64:
		return float64(n)
	case uint8:
		return float64(n)
	case int8:
		return float64(n)
	case uint16:
		return float64(n)
	case int16:
		return float64(n)
	}
	return 0
}

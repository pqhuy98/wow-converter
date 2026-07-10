package azerothcore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/workspace"
	_ "modernc.org/sqlite"
)

const tileSize = 533.3333333

const (
	creatureFlagExtraTrigger         = 0x00000080
	creatureFlagExtraGhostVisibility = 0x00000400
	invisibleDisplayID               = 11686
)

// CreatureRecord mirrors AzerothCore creature row fields used by export.
type CreatureRecord struct {
	GUID        int64
	ID1         int
	Map         int
	PositionX   float64
	PositionY   float64
	PositionZ   float64
	Orientation float64
	CurHealth   int
	CurMana     int
	PhaseMask   int32
}

// CreatureTemplate holds template metadata.
type CreatureTemplate struct {
	Entry      int
	Name       string
	SubName    string
	MaxLevel   int
	FlagsExtra int
}

// CreatureTemplateModel holds display model info.
type CreatureTemplateModel struct {
	CreatureDisplayID int
	DisplayScale      float64
}

// ItemTemplate holds item metadata for equipment.
type ItemTemplate struct {
	Entry         int
	InventoryType int
}

// Equipment holds optional equipped items.
type Equipment struct {
	Item1 *ItemTemplate
	Item2 *ItemTemplate
	Item3 *ItemTemplate
}

// Creature is a spawned creature with template and model.
type Creature struct {
	Creature  CreatureRecord
	Template  CreatureTemplate
	Equipment *Equipment
	Model     CreatureTemplateModel
}

func resolveDatabasePath() string {
	root := workspace.FindRepoRoot()
	candidates := []string{
		filepath.Join(root, "bin", "azerothcore-world.sqlite"),
	}
	if v := os.Getenv("DATABASE_URL"); v != "" && strings.HasPrefix(v, "file:") {
		p := strings.TrimPrefix(v, "file:")
		p = strings.TrimPrefix(p, "./")
		if filepath.IsAbs(p) {
			candidates = append(candidates, p)
		} else {
			candidates = append(candidates,
				filepath.Clean(filepath.Join(root, p)),
				filepath.Clean(filepath.Join(root, "bin", filepath.Base(p))),
			)
		}
	}
	candidates = append(candidates,
		filepath.Join("bin", "azerothcore-world.sqlite"),
		filepath.Join("..", "bin", "azerothcore-world.sqlite"),
	)
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

// GetCreaturesInTile queries creatures within an ADT tile.
func GetCreaturesInTile(mapID int, tileXY [2]int, _ map[string]any) ([]Creature, error) {
	dbPath := resolveDatabasePath()
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("azerothcore database not found: %s", dbPath)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	tileX, tileY := tileXY[0], tileXY[1]
	worldMinX := (32 - float64(tileY) - 1) * tileSize
	worldMaxX := (32 - float64(tileY)) * tileSize
	worldMinY := (32 - float64(tileX) - 1) * tileSize
	worldMaxY := (32 - float64(tileX)) * tileSize
	minX, maxX := worldMinX, worldMaxX
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := worldMinY, worldMaxY
	if minY > maxY {
		minY, maxY = maxY, minY
	}

	rows, err := db.Query(`
		SELECT guid, id1, map, position_x, position_y, position_z, orientation,
		       curhealth, curmana, phaseMask
		FROM creature
		WHERE map = ? AND position_x >= ? AND position_x <= ?
		  AND position_y >= ? AND position_y <= ?`,
		mapID, minX, maxX, minY, maxY,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creatures []Creature
	templateIDs := map[int]struct{}{}
	for rows.Next() {
		var c Creature
		if err := rows.Scan(
			&c.Creature.GUID, &c.Creature.ID1, &c.Creature.Map,
			&c.Creature.PositionX, &c.Creature.PositionY, &c.Creature.PositionZ,
			&c.Creature.Orientation, &c.Creature.CurHealth, &c.Creature.CurMana, &c.Creature.PhaseMask,
		); err != nil {
			return nil, err
		}
		c.Template.Entry = c.Creature.ID1
		templateIDs[c.Creature.ID1] = struct{}{}
		creatures = append(creatures, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	templates := loadTemplates(db, templateIDs)
	models := loadModels(db, templateIDs)
	equipment := loadEquipment(db, templateIDs)
	items := loadItems(db, equipment)

	var out []Creature
	for _, c := range creatures {
		t, ok := templates[c.Template.Entry]
		if !ok {
			continue
		}
		m, ok := models[c.Template.Entry]
		if !ok {
			continue
		}
		flagsExtra := t.FlagsExtra
		if flagsExtra&creatureFlagExtraTrigger != 0 {
			continue
		}
		if flagsExtra&creatureFlagExtraGhostVisibility != 0 {
			continue
		}
		if m.CreatureDisplayID == invisibleDisplayID {
			continue
		}
		c.Template = t
		c.Model = m
		if eq, ok := equipment[c.Template.Entry]; ok {
			e := Equipment{}
			if eq.ItemID1 > 0 {
				if it, ok := items[eq.ItemID1]; ok {
					e.Item1 = &it
				}
			}
			if eq.ItemID2 > 0 {
				if it, ok := items[eq.ItemID2]; ok {
					e.Item2 = &it
				}
			}
			if eq.ItemID3 > 0 {
				if it, ok := items[eq.ItemID3]; ok {
					e.Item3 = &it
				}
			}
			c.Equipment = &e
		}
		out = append(out, c)
	}
	return out, nil
}

// CountCreaturesInTiles returns visible creature spawns across ADT tiles.
func CountCreaturesInTiles(mapID int, tiles [][2]int) (int, error) {
	total := 0
	for _, tile := range tiles {
		creatures, err := GetCreaturesInTile(mapID, tile, nil)
		if err != nil {
			return 0, err
		}
		total += len(creatures)
	}
	return total, nil
}

type equipRow struct {
	CreatureID int
	ItemID1    int
	ItemID2    int
	ItemID3    int
}

func loadTemplates(db *sql.DB, ids map[int]struct{}) map[int]CreatureTemplate {
	out := map[int]CreatureTemplate{}
	if len(ids) == 0 {
		return out
	}
	query := "SELECT entry, name, subname, maxlevel, flags_extra FROM creature_template WHERE entry IN (" + intSetSQL(ids) + ")"
	rows, err := db.Query(query)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var t CreatureTemplate
		var subname sql.NullString
		if err := rows.Scan(&t.Entry, &t.Name, &subname, &t.MaxLevel, &t.FlagsExtra); err != nil {
			continue
		}
		if subname.Valid {
			t.SubName = subname.String
		}
		out[t.Entry] = t
	}
	return out
}

func loadModels(db *sql.DB, ids map[int]struct{}) map[int]CreatureTemplateModel {
	out := map[int]CreatureTemplateModel{}
	if len(ids) == 0 {
		return out
	}
	query := "SELECT CreatureID, CreatureDisplayID, DisplayScale FROM creature_template_model WHERE CreatureID IN (" + intSetSQL(ids) + ")"
	rows, err := db.Query(query)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var creatureID int
		var m CreatureTemplateModel
		if err := rows.Scan(&creatureID, &m.CreatureDisplayID, &m.DisplayScale); err != nil {
			continue
		}
		out[creatureID] = m
	}
	return out
}

func loadEquipment(db *sql.DB, ids map[int]struct{}) map[int]equipRow {
	out := map[int]equipRow{}
	if len(ids) == 0 {
		return out
	}
	query := "SELECT CreatureID, ItemID1, ItemID2, ItemID3 FROM creature_equip_template WHERE CreatureID IN (" + intSetSQL(ids) + ")"
	rows, err := db.Query(query)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var row equipRow
		if err := rows.Scan(&row.CreatureID, &row.ItemID1, &row.ItemID2, &row.ItemID3); err != nil {
			continue
		}
		out[row.CreatureID] = row
	}
	return out
}

func loadItems(db *sql.DB, equipment map[int]equipRow) map[int]ItemTemplate {
	itemIDs := map[int]struct{}{}
	for _, eq := range equipment {
		if eq.ItemID1 > 0 {
			itemIDs[eq.ItemID1] = struct{}{}
		}
		if eq.ItemID2 > 0 {
			itemIDs[eq.ItemID2] = struct{}{}
		}
		if eq.ItemID3 > 0 {
			itemIDs[eq.ItemID3] = struct{}{}
		}
	}
	out := map[int]ItemTemplate{}
	if len(itemIDs) == 0 {
		return out
	}
	query := "SELECT entry, InventoryType FROM item_template WHERE entry IN (" + intSetSQL(itemIDs) + ")"
	rows, err := db.Query(query)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var it ItemTemplate
		if err := rows.Scan(&it.Entry, &it.InventoryType); err != nil {
			continue
		}
		out[it.Entry] = it
	}
	return out
}

func intSetSQL(ids map[int]struct{}) string {
	vals := make([]int, 0, len(ids))
	for id := range ids {
		vals = append(vals, id)
	}
	sort.Ints(vals)
	if len(vals) == 0 {
		return "0"
	}
	s := fmt.Sprintf("%d", vals[0])
	for _, v := range vals[1:] {
		s += fmt.Sprintf(",%d", v)
	}
	return s
}

package creature

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

type deathSoundFileSource struct {
	db.RuntimeFileSource
	client client.Client
	build  string
}

func (s deathSoundFileSource) GetBuildName() string { return s.build }
func (s deathSoundFileSource) GetFile(ctx context.Context, id int) ([]byte, error) {
	if tables, ok := s.client.(client.DBTableClient); ok {
		return tables.DownloadCascTable(ctx, id)
	}
	return s.client.DownloadCascFile(ctx, id)
}
func (s deathSoundFileSource) GetFileByName(ctx context.Context, name string) ([]byte, error) {
	entry, err := s.client.GetFileByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if entry.FileDataID <= 0 {
		return nil, fmt.Errorf("client table %s not found", name)
	}
	return s.GetFile(ctx, entry.FileDataID)
}

func loadDeathSoundFiles(ctx context.Context, source db.FileSource, wanted map[int]bool) (map[int]int, error) {
	tables := make([]*db.WDCReader, 4)
	for i, name := range []string{"CreatureDisplayInfo", "CreatureModelData", "CreatureSoundData", "SoundKitEntry"} {
		r := db.NewWDCReader("DBFilesClient/"+name+".db2", source)
		if err := r.Parse(ctx, nil); err != nil {
			return nil, fmt.Errorf("NPC death sounds: load %s: %w", name, err)
		}
		tables[i] = r
	}
	first, err := firstDeathSoundFiles(ctx, tables[3].GetAllRows())
	if err != nil {
		return nil, err
	}
	files := map[int]int{}
	for display := range wanted {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files[display] = creatureDeathSoundFile(tables[0].GetRow(uint32(display)), tables[1].GetRow, tables[2].GetRow, first)
	}
	return files, nil
}

// Select by entry ID rather than file ID; WDC section/map iteration order must
// not change which variant is exported.
func firstDeathSoundFiles(ctx context.Context, rows map[uint32]db.DB2Row) (map[int]int, error) {
	first := map[int]int{}
	entryIDs := map[int]uint32{}
	for id, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		kit, file := deathSoundInt(row["SoundKitID"]), deathSoundInt(row["FileDataID"])
		if kit > 0 && file > 0 && (first[kit] == 0 || id < entryIDs[kit]) {
			first[kit], entryIDs[kit] = file, id
		}
	}
	return first, nil
}

func creatureDeathSoundFile(display db.DB2Row, model, sound func(uint32) db.DB2Row, first map[int]int) int {
	if display == nil {
		return 0
	}
	data := sound(uint32(deathSoundInt(display["SoundID"])))
	// Match the client's display override, falling back only when its record
	// is absent. An existing sound record with no death kit is intentionally silent.
	if data == nil {
		m := model(uint32(deathSoundInt(display["ModelID"])))
		data = sound(uint32(deathSoundInt(m["SoundID"])))
	}
	return first[deathSoundInt(data["SoundDeathID"])]
}

func deathSoundInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case uint32:
		return int(n)
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case uint16:
		return int(n)
	case uint8:
		return int(n)
	default:
		return 0
	}
}

func ExportCreatureDeathSounds(ctx context.Context, wowClient client.Client, outputDir string, units []PlacedUnit) (map[string]string, error) {
	wanted := map[int]bool{}
	for _, u := range units {
		if u.TypeCode != "" && u.Creature.Template.FlagsExtra&0x80 == 0 {
			wanted[u.Creature.Model.CreatureDisplayID] = true
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}
	info, err := wowClient.GetCASCInfo(ctx)
	if err != nil {
		return nil, err
	}
	files, err := loadDeathSoundFiles(ctx, deathSoundFileSource{client: wowClient, build: info.BuildName}, wanted)
	if err != nil {
		return nil, err
	}
	return SaveCreatureDeathSounds(ctx, wowClient, outputDir, units, files)
}

func SaveCreatureDeathSounds(ctx context.Context, wowClient client.Client, outputDir string, units []PlacedUnit, files map[int]int) (map[string]string, error) {
	paths := map[int]string{}
	byType := map[string]string{}
	for _, u := range units {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if u.TypeCode == "" || u.Creature.Template.FlagsExtra&0x80 != 0 {
			continue
		}
		id := files[u.Creature.Model.CreatureDisplayID]
		if id == 0 {
			continue
		}
		path := paths[id]
		if path == "" {
			raw, err := wowClient.DownloadCascFile(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("NPC %s death sound %d: %w", u.Creature.Template.Name, id, err)
			}
			wav, err := deathSoundWAV(ctx, raw)
			if err != nil {
				return nil, fmt.Errorf("NPC %s death sound %d: %w", u.Creature.Template.Name, id, err)
			}
			path = fmt.Sprintf("Sounds/WoW/Death/%d.wav", id)
			target := filepath.Join(outputDir, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(target, wav, 0644); err != nil {
				return nil, err
			}
			paths[id] = path
		}
		if previous, ok := byType[u.TypeCode]; ok && previous != path {
			return nil, fmt.Errorf("conflicting death sounds for unit type %s", u.TypeCode)
		}
		byType[u.TypeCode] = path
	}
	log.Printf("NPC death sounds: %d unit types, %d unique audio files", len(byType), len(paths))
	return byType, nil
}

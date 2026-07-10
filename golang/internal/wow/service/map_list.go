package service

import (
	"context"
	"path"
	"strings"

	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

// MapListService implements casc.MapList.
type MapListService struct{}

func (MapListService) GetMaps(ctx context.Context) ([]casc.MapEntry, error) {
	reader := db.NewWDCReader("DBFilesClient/Map.db2", nil)
	if err := reader.Parse(ctx, nil); err != nil {
		return nil, err
	}
	var maps []casc.MapEntry
	rows := reader.GetAllRows()
	for _, id := range reader.RowIDsInOrder() {
		entry := rows[id]
		dir, _ := entry["Directory"].(string)
		if dir == "" {
			continue
		}
		wdtPath := path.Join("world/maps", dir, dir+".wdt")
		wdtPath = strings.ReplaceAll(wdtPath, "\\", "/")
		if fileID, ok := archivecasc.GetByFilename(wdtPath); !ok || fileID == 0 {
			continue
		}
		maps = append(maps, casc.MapEntry{
			ID: int(id), Name: entry["MapName_lang"], Dir: dir, ExpansionID: entry["ExpansionID"],
		})
	}
	return maps, nil
}

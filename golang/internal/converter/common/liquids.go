package common

import (
	"context"
	"log"

	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

// LiquidSoundBanks resolves water/ocean (0/1), magma (2), and slime (3) using
// the loaded client's table. Unknown IDs are kept for explicit mesh fallback.
func (a *AssetManager) LiquidSoundBanks(ctx context.Context) (map[uint16]int, error) {
	banks := map[uint16]int{1: 0, 2: 1, 3: 2, 4: 3, 5: 0, 13: 0, 14: 1, 19: 2, 20: 3}
	var source db.FileSource = db.RuntimeFileSource{}
	if a.wowClient != nil {
		info, err := a.wowClient.GetCASCInfo(ctx)
		if err != nil {
			return nil, err
		}
		source = gameObjectFileSource{client: a.wowClient, build: info.BuildName}
	}
	r := db.NewWDCReader("DBFilesClient/LiquidType.db2", source)
	if err := r.Parse(ctx, nil); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Printf("Liquid types unavailable; retaining legacy IDs and using mesh fallback for unknown types: %v", err)
		return banks, nil
	}
	for id, row := range r.GetAllRows() {
		if value, ok := row["SoundBank"]; ok && id <= 65535 {
			banks[uint16(id)] = gameObjectFileID(value)
		}
	}
	return banks, nil
}

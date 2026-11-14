//go:build integration_tests

package db_test

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/wow/bootstrap"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

func TestRealProjectedModelCacheTables(t *testing.T) {
	if os.Getenv("WOW_DB_PROJECTION_VALIDATION") != "1" {
		t.Skip("set WOW_DB_PROJECTION_VALIDATION=1 and CASC_LOCAL_WOW")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := bootstrap.StartWowDataServer(ctx); err != nil {
		t.Fatal(err)
	}
	for _, table := range []struct {
		name   string
		fields []string
	}{
		{"ModelFileData", []string{"ModelResourcesID"}},
		{"TextureFileData", []string{"UsageType", "MaterialResourcesID"}},
		{"ItemDisplayInfo", []string{"ModelResourcesID", "ModelMaterialResourcesID"}},
		{"CreatureDisplayInfoGeosetData", []string{"CreatureDisplayInfoID", "GeosetIndex", "GeosetValue"}},
		{"CreatureDisplayInfo", []string{"ModelID", "TextureVariationFileDataID"}},
		{"CreatureModelData", []string{"FileDataID", "CreatureGeosetDataID"}},
	} {
		t.Run(table.name, func(t *testing.T) {
			full := db.NewWDCReader("DBFilesClient/"+table.name+".db2", nil)
			if err := full.Parse(ctx, nil); err != nil {
				t.Fatal(err)
			}
			projected := db.NewWDCReader(full.FileName, nil)
			if err := projected.Parse(ctx, nil, table.fields...); err != nil {
				t.Fatal(err)
			}
			want, got := full.GetAllRows(), projected.GetAllRows()
			if len(want) != len(got) {
				t.Fatal("row count changed")
			}
			for id, row := range want {
				for _, field := range append([]string{"ID"}, table.fields...) {
					if !reflect.DeepEqual(row[field], got[id][field]) {
						t.Fatalf("row %d field %s changed", id, field)
					}
				}
			}
			t.Logf("verified %d projected rows", len(want))
		})
	}
}

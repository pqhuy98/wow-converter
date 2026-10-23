package mapexporter

import (
	"context"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
)

type wmoChildrenClient struct {
	client.Client
	params casc.ADTExportParams
}

func (c *wmoChildrenClient) ExportADTForConversion(_ context.Context, params casc.ADTExportParams) (*exportadt.ConversionOutput, error) {
	c.params = params
	return &exportadt.ConversionOutput{ObjectPath: "adt/test_1_2"}, nil
}

func TestInteriorFlagAlwaysLoadsExteriorWMOChildren(t *testing.T) {
	for _, include := range []bool{false, true} {
		c := &wmoChildrenClient{}
		cfg := DefaultMapExportConfig()
		if err := LoadADTTilesForConversion(context.Background(), c, "", cfg, 0, []TileCoord{{X: 1, Y: 2}}, include, common.NewTileRegistry(), nil); err != nil {
			t.Fatal(err)
		}
		if c.params.IncludeWMOSets == nil || !*c.params.IncludeWMOSets || c.params.ExcludeWMOInteriors == include {
			t.Fatalf("include interiors %v: WMO sets %v, exclude interiors %v", include, c.params.IncludeWMOSets, c.params.ExcludeWMOInteriors)
		}
	}
}

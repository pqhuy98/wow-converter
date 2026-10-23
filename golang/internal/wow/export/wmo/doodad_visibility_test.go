package wmoexport

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	exportpkg "github.com/pqhuy98/wow-converter/internal/wow/export"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

func groupFixture(flags uint32, refs ...uint16) []byte {
	var raw bytes.Buffer
	// MOGP's fixed header is 68 bytes; MODR is a nested chunk.
	for _, word := range []uint32{0x4D4F4750, uint32(68 + 8 + 2*len(refs)), 0, 0, flags} {
		binary.Write(&raw, binary.LittleEndian, word)
	}
	raw.Write(make([]byte, 56))
	binary.Write(&raw, binary.LittleEndian, uint32(0x4D4F4452))
	binary.Write(&raw, binary.LittleEndian, uint32(2*len(refs)))
	binary.Write(&raw, binary.LittleEndian, refs)
	return raw.Bytes()
}

func placementFixture() *Exporter {
	root := wmo.NewLoader(buffer.From(nil), 1, "", false)
	root.GroupCount = 4
	root.GroupIDs = []uint32{10, 11, 12, 13}
	root.FileDataIDs = []uint32{123}
	root.DoodadSets = []wmo.DoodadSet{{DoodadCount: 7}}
	for i := 0; i < 7; i++ {
		// Every instance uses the same model: classification must be per placement.
		root.Doodads = append(root.Doodads, wmo.Doodad{Position: []float32{float32(i), 2, 3}, Scale: 1})
	}
	return &Exporter{Loader: root}
}

func TestInteriorFilterKeepsExteriorSharedUnknownAndUnreferencedChildren(t *testing.T) {
	e := placementFixture()
	groups := map[uint32][]byte{
		10: groupFixture(0x2000, 0, 2, 4, 5, 99),
		11: groupFixture(0x8, 1, 2, 6),
		12: groupFixture(0, 4),
		13: groupFixture(0x2008, 5),
	}
	if err := e.ExcludeInteriorDoodads(context.Background(), func(_ context.Context, id uint32) ([]byte, error) {
		return groups[id], nil
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := e.CollectDoodadPlacements("building.obj", exportpkg.ADTExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var positions []string
	for _, row := range rows {
		positions = append(positions, row.PositionX)
	}
	if !reflect.DeepEqual(positions, []string{"1", "2", "3", "4", "5", "6"}) {
		t.Fatalf("kept placements %v", positions)
	}
	out := filepath.Join(t.TempDir(), "building.obj")
	if err := e.ExportDoodadPlacementCsv(out, exportpkg.ADTExportOptions{OverwriteFiles: true}, nil); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(exportpkg.PlacementCsvPath(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	csvRows, err := csv.NewReader(f).ReadAll()
	if err != nil || len(csvRows) != len(rows)+1 {
		t.Fatalf("CSV differs from direct placement filtering: %d, %v", len(csvRows), err)
	}
	// The checked UI path does not request the filter and retains every child.
	all, err := placementFixture().CollectDoodadPlacements("building.obj", exportpkg.ADTExportOptions{})
	if err != nil || len(all) != 7 {
		t.Fatalf("all children: %d, %v", len(all), err)
	}
	e.SetDoodadSetMask([]DoodadSetMaskEntry{{Checked: false}})
	rows, err = e.CollectDoodadPlacements("building.obj", exportpkg.ADTExportOptions{})
	if err != nil || len(rows) != 0 {
		t.Fatalf("disabled doodad set was included: %v, %v", rows, err)
	}
}

func TestIncompleteGroupDataKeepsAllChildrenAndCancellationPropagates(t *testing.T) {
	e := placementFixture()
	err := e.ExcludeInteriorDoodads(context.Background(), func(_ context.Context, id uint32) ([]byte, error) {
		if id == 10 {
			return groupFixture(0x2000, 0), nil
		}
		return nil, errors.New("missing group")
	})
	if err == nil || len(e.excludeDoodads) != 0 {
		t.Fatalf("incomplete classification applied a partial filter: %v", err)
	}
	rows, err := e.CollectDoodadPlacements("building.obj", exportpkg.ADTExportOptions{})
	if err != nil || len(rows) != 7 {
		t.Fatalf("fallback lost children: %d, %v", len(rows), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = e.ExcludeInteriorDoodads(ctx, func(context.Context, uint32) ([]byte, error) {
		t.Fatal("read after cancellation")
		return nil, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

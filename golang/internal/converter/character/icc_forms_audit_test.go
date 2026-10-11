package character

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

type iccAuditSource struct {
	db.RuntimeFileSource
	client *client.HTTPClient
	build  string
}

func (s iccAuditSource) GetBuildName() string { return s.build }
func (s iccAuditSource) GetFile(ctx context.Context, id int) ([]byte, error) {
	return s.client.DownloadCascTable(ctx, id)
}
func (s iccAuditSource) GetFileByName(ctx context.Context, name string) ([]byte, error) {
	f, err := s.client.GetFileByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return s.GetFile(ctx, f.FileDataID)
}

// Opt-in source-data audit. Reads client tables without changing the loaded build.
func TestICCPutricideFormsAudit(t *testing.T) {
	output := os.Getenv("ICC_FORMS_AUDIT_OUTPUT")
	if output == "" {
		t.Skip("set ICC_FORMS_AUDIT_OUTPUT")
	}
	ctx := context.Background()
	c := client.NewHTTPClient("")
	info, err := c.GetCASCInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	source := iccAuditSource{client: c, build: info.BuildName}
	load := func(name string) *db.WDCReader {
		p := "DBFilesClient/" + name + ".db2"
		raw, err := source.GetFileByName(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		r := db.NewWDCReader(p, source)
		if err := r.Parse(ctx, raw); err != nil {
			t.Fatal(err)
		}
		return r
	}
	num := func(value any) int {
		switch v := value.(type) {
		case int16:
			return int(v)
		case uint16:
			return int(v)
		case int32:
			return int(v)
		case uint32:
			return int(v)
		case int64:
			return int(v)
		case uint64:
			return int(v)
		case int:
			return v
		}
		return 0
	}
	result := map[string]any{"build": info.BuildName}
	effects := load("SpellEffect")
	var rows []db.DB2Row
	wanted := map[int]bool{71621: true, 71893: true}
	for changed := true; changed; {
		changed = false
		for _, row := range effects.GetAllRows() {
			if wanted[num(row["SpellID"])] {
				trigger := num(row["EffectTriggerSpell"])
				if trigger > 0 && !wanted[trigger] {
					wanted[trigger] = true
					changed = true
				}
			}
		}
	}
	for _, row := range effects.GetAllRows() {
		if wanted[num(row["SpellID"])] {
			rows = append(rows, row)
		}
	}
	result["effects"] = rows
	displays := load("CreatureDisplayInfo")
	models := load("CreatureModelData")
	var forms []map[string]any
	for _, id := range []uint32{30881, 30641, 30993, 22769} {
		d := displays.GetRow(id)
		m := models.GetRow(uint32(num(d["ModelID"])))
		fid := num(m["FileDataID"])
		f, err := c.GetFileByID(ctx, fid)
		if err != nil {
			t.Fatal(err)
		}
		forms = append(forms, map[string]any{"display": id, "displayRow": d, "modelRow": m, "file": f.FileName, "fileDataID": fid})
	}
	result["forms"] = forms
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, data, 0644); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Wrote Putricide client-data audit to %s\n", output)
}

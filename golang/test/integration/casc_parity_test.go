//go:build integration

package integration

import (
	"fmt"
	"testing"

	"github.com/pqhuy98/wow-converter/test/internal/snapshot"
)

type cascSample struct {
	Name     string
	FileName string
	Check    func(t *testing.T, data []byte)
}

func TestCascFileParity(t *testing.T) {
	env := testServer(t)
	golden := loadGoldenCascManifest(t)

	samples := []cascSample{
		{
			Name:     "m2",
			FileName: "creature/murloc/murloc.m2",
			Check:    assertM2Magic,
		},
		{
			Name:     "blp",
			FileName: "interface/icons/inv_misc_questionmark.blp",
			Check:    assertBLPMagic,
		},
		{
			Name:     "db2",
			FileName: "dbfilesclient/map.db2",
			Check:    assertDB2Magic,
		},
	}

	for _, sample := range samples {
		sample := sample
		t.Run(sample.Name, func(t *testing.T) {
			fileDataID := env.resolveFileDataID(t, sample.FileName)
			data := env.fetchCascFile(t, fileDataID)
			sample.Check(t, data)

			key := fmt.Sprintf("%s:%d", sample.Name, fileDataID)
			if rec, ok := golden.Files[key]; ok && !snapshot.IsPlaceholderSHA(rec.SHA256) {
				got := sha256Hex(data)
				if got != rec.SHA256 {
					t.Fatalf("golden sha256 mismatch for %s: got %s want %s", key, got, rec.SHA256)
				}
				if int64(len(data)) != rec.Size {
					t.Fatalf("golden size mismatch for %s: got %d want %d", key, len(data), rec.Size)
				}
			}

			compareAgainstTSReference(t, env, fileDataID, data)
		})
	}
}

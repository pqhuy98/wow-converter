package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/server/util"
)

func TestLoadRecentExportsUsesEmptyArrayForMissingOrNullFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recent-exports.json")
	queue := &util.JobQueue[exportCharacterRequest, exportCharacterResponse]{}

	loadRecentExports(queue, path)
	assertRecentExportsJSONIsArray(t, queue)

	if err := os.WriteFile(path, []byte("null"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadRecentExports(queue, path)
	assertRecentExportsJSONIsArray(t, queue)
}

func assertRecentExportsJSONIsArray(t *testing.T, queue *util.JobQueue[exportCharacterRequest, exportCharacterResponse]) {
	t.Helper()
	if queue.RecentCompletedJobs == nil {
		t.Fatal("recent exports must not be nil")
	}
	data, err := json.Marshal(queue.RecentCompletedJobs)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[]" {
		t.Fatalf("recent exports JSON = %s, want []", data)
	}
}

package icon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/workspace"
)

func TestRunUpscalerWithRelativeOutputPath(t *testing.T) {
	if !UpscalerAvailable() {
		t.Skip("upscayl runtime unavailable")
	}

	_, _ = workspace.ChdirRepoRoot()
	src := filepath.Join(workspace.DefaultExportDir(), "interface", "icons", "10_2_raidability_blue.png")
	if _, err := os.Stat(src); err != nil {
		t.Skip("sample icon png missing")
	}

	relOut := filepath.Join(".cache", "wow-export", "interface", "icons", "test_ai_relative_out.png")
	_ = os.Remove(relOut)
	t.Cleanup(func() { _ = os.Remove(relOut) })

	if err := runUpscaler(src, relOut, 2); err != nil {
		t.Fatalf("runUpscaler: %v", err)
	}
	if _, err := os.Stat(relOut); err != nil {
		t.Fatalf("expected output at cwd-relative path: %v", err)
	}
}

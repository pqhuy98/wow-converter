package icon

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/workspace"
)

func TestResizeAiPngConcurrentWaiters(t *testing.T) {
	if !UpscalerAvailable() {
		t.Skip("upscayl runtime unavailable")
	}

	_, _ = workspace.ChdirRepoRoot()
	src := filepath.Join(workspace.DefaultExportDir(), "interface", "icons", "inv_sword_146.png")
	if _, err := os.Stat(src); err != nil {
		t.Skip("sample icon png missing")
	}

	_ = os.Remove(src + "__ai128.png")

	const waiters = 6
	var wg sync.WaitGroup
	errs := make(chan error, waiters)

	for i := 0; i < waiters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := resizeAiPngWithCache(src, Size128)
			if err != nil {
				errs <- err
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Minute):
		t.Fatal("concurrent AI resize waiters did not all complete")
	}

	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("resizeAiPngWithCache: %v", err)
		}
	}
}

// Package runtime provides minimal runtime state for the WoW reader.
package runtime

import "github.com/pqhuy98/wow-converter/internal/wow/log"

// LoadFunc is executed when a CASC source finishes loading.
type LoadFunc func() error

var loadFuncs []LoadFunc

// RegisterLoadFunc registers a load function.
func RegisterLoadFunc(fn LoadFunc) {
	loadFuncs = append(loadFuncs, fn)
}

// RunLoadFuncs runs all registered load functions.
func RunLoadFuncs() error {
	for _, fn := range loadFuncs {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

// RuntimeState holds selected CDN region and active CASC source.
var RuntimeState = struct {
	SelectedCDNRegionTag string
	Casc                 any
}{
	SelectedCDNRegionTag: "us",
}

// Progress is a progress shim replacing core.createProgress().
type Progress interface {
	Step(name string) error
}

type progress struct{}

// CreateProgress creates a progress tracker.
func CreateProgress(_ int) Progress {
	return progress{}
}

func (progress) Step(name string) error {
	if name != "" {
		log.Write("Progress: %s", name)
	}
	return nil
}

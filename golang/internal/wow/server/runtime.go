package server

import (
	"fmt"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
)

// LoadFunc runs after a CASC source finishes loading.
type LoadFunc func() error

var (
	loadFuncsMu sync.Mutex
	loadFuncs   []LoadFunc
)

// RegisterLoadFunc registers a function to run when CASC finishes loading.
func RegisterLoadFunc(fn LoadFunc) {
	loadFuncsMu.Lock()
	defer loadFuncsMu.Unlock()
	loadFuncs = append(loadFuncs, fn)
}

// RunLoadFuncs executes all registered load functions.
func RunLoadFuncs() error {
	loadFuncsMu.Lock()
	funcs := append([]LoadFunc(nil), loadFuncs...)
	loadFuncsMu.Unlock()

	for _, fn := range funcs {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

// RuntimeState mirrors src/lib/wow/server/runtime.ts runtimeState.
type RuntimeState struct {
	mu sync.RWMutex

	SelectedCDNRegionTag string
	Casc                 casc.Source
}

// GlobalRuntime is the process-wide runtime singleton.
var GlobalRuntime = &RuntimeState{
	SelectedCDNRegionTag: "us",
}

// GetCasc returns the active CASC source or an error if none is loaded.
func (r *RuntimeState) GetCasc() (casc.Source, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.Casc == nil {
		return nil, fmt.Errorf("no CASC source has been loaded")
	}
	return r.Casc, nil
}

// SetCasc sets the active CASC source.
func (r *RuntimeState) SetCasc(source casc.Source) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Casc = source
}

// GetCascOptional returns the active CASC source, which may be nil.
func (r *RuntimeState) GetCascOptional() casc.Source {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Casc
}

// Progress mirrors core.createProgress() — logs steps for now.
type Progress interface {
	Step(name string) error
}

type logProgress struct{}

// CreateProgress returns a progress shim.
func CreateProgress(_ int) Progress {
	return logProgress{}
}

func (logProgress) Step(_ string) error {
	return nil
}

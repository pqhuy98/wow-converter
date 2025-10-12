package core

import (
	"sync"
	"wowexportd/internal/casc"
)

type CoreView struct {
	mu      sync.RWMutex
	Active  casc.Source
	Pending casc.Source
}

func New() *CoreView { return &CoreView{} }

func (v *CoreView) SetPending(s casc.Source) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Pending = s
}

func (v *CoreView) GetPending() casc.Source {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Pending
}

func (v *CoreView) ActivatePending() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Active = v.Pending
	v.Pending = nil
}

func (v *CoreView) GetActive() casc.Source {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Active
}

package api

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/ansi"
)

type startupAnnouncer struct {
	mu           sync.Mutex
	port         int
	browseOK     bool
	mapsDone     bool
	modelCount   int
	textureCount int
	announced    bool
}

func newStartupAnnouncer(port int) *startupAnnouncer {
	return &startupAnnouncer{port: port}
}

func (a *startupAnnouncer) browseFinished(ok bool, modelCount, textureCount int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.browseOK = ok
	if ok {
		a.modelCount = modelCount
		a.textureCount = textureCount
	}
	a.maybeAnnounceLocked()
}

func (a *startupAnnouncer) mapsFinished() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mapsDone = true
	a.maybeAnnounceLocked()
}

func (a *startupAnnouncer) maybeAnnounceLocked() {
	if a.announced || !a.browseOK || !a.mapsDone {
		return
	}
	a.announced = true
	clearTerminal()
	url := fmt.Sprintf("http://127.0.0.1:%d", a.port)
	log.Printf("%s indexed %s models and %s textures",
		ansi.Green("✅ WoW Converter is ready:"),
		ansi.Yellowf("%d", a.modelCount),
		ansi.Yellowf("%d", a.textureCount),
	)
	log.Printf("%s %s", ansi.Green("Open in your browser:"), ansi.Blue(url))
}

func clearTerminal() {
	_, _ = fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H")
}

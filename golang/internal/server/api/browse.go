package api

import (
	"context"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/stringsort"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

var (
	m2WmoRegex   = regexp.MustCompile(`(?i)\.(m2|wmo)$`)
	badWmoRegex  = regexp.MustCompile(`(?i)_([0-9]{3}|lod\d)\.wmo$`)
	textureRegex = regexp.MustCompile(`(?i)\.(blp|png|tga|dds)$`)
)

type browseIndexes struct {
	mu           sync.Mutex
	modelFiles   []casc.ListfileEntry
	textureFiles []casc.ListfileEntry
	loading      bool
	loadErr      error
}

func (b *browseIndexes) load(ctx context.Context, c client.Client) error {
	b.mu.Lock()
	if b.modelFiles != nil {
		b.mu.Unlock()
		return nil
	}
	if b.loading {
		b.mu.Unlock()
		for {
			time.Sleep(50 * time.Millisecond)
			b.mu.Lock()
			if b.modelFiles != nil {
				b.mu.Unlock()
				return nil
			}
			if !b.loading {
				err := b.loadErr
				b.mu.Unlock()
				return err
			}
			b.mu.Unlock()
		}
	}
	b.loading = true
	b.mu.Unlock()

	err := b.build(ctx, c)

	b.mu.Lock()
	b.loading = false
	b.loadErr = err
	b.mu.Unlock()
	return err
}

func (b *browseIndexes) build(ctx context.Context, c client.Client) error {
	start := time.Now()
	var modelFiles, textureFiles []casc.ListfileEntry

	if direct, ok := c.(client.DirectListfileClient); ok {
		log.Printf("Building browse index from listfile...")
		var err error
		modelFiles, textureFiles, err = direct.CollectBrowseFileIndex(ctx)
		if err == nil {
			log.Printf("Collected %d models and %d textures in %.1fs", len(modelFiles), len(textureFiles), time.Since(start).Seconds())
			b.mu.Lock()
			b.modelFiles = modelFiles
			b.textureFiles = textureFiles
			b.mu.Unlock()
			log.Printf("Total M2/WMO files: %d", len(modelFiles))
			log.Printf("Total texture files: %d", len(textureFiles))
			return nil
		}
		log.Printf("Direct browse index unavailable (%v), falling back to full listfile copy", err)
	}

	if modelFiles == nil {
		log.Printf("Building browse index (full listfile copy)...")
		files, err := getListFiles(ctx, c)
		if err != nil {
			return err
		}
		log.Printf("Loaded %d listfile entries in %.1fs", len(files), time.Since(start).Seconds())

		filterStart := time.Now()
		filtered := make([]casc.ListfileEntry, 0, len(files))
		for _, f := range files {
			if badWmoRegex.MatchString(f.FileName) {
				continue
			}
			filtered = append(filtered, f)
		}

		modelFiles = make([]casc.ListfileEntry, 0, len(filtered)/4)
		textureFiles = make([]casc.ListfileEntry, 0, len(filtered)/2)
		for _, f := range filtered {
			if m2WmoRegex.MatchString(f.FileName) {
				modelFiles = append(modelFiles, f)
			} else if textureRegex.MatchString(f.FileName) {
				textureFiles = append(textureFiles, f)
			}
		}
		log.Printf("Filtered browse index in %.1fs", time.Since(filterStart).Seconds())

		sortStart := time.Now()
		sortBrowseEntries(modelFiles, textureFiles)
		log.Printf("Sorted browse index in %.1fs", time.Since(sortStart).Seconds())
	}

	b.mu.Lock()
	b.modelFiles = modelFiles
	b.textureFiles = textureFiles
	b.mu.Unlock()

	log.Printf("Total M2/WMO files: %d", len(modelFiles))
	log.Printf("Total texture files: %d", len(textureFiles))
	return nil
}

func (b *browseIndexes) snapshot(q string) []casc.ListfileEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	if q == "model" {
		return b.modelFiles
	}
	return b.textureFiles
}

func (b *browseIndexes) modelFileName(fileDataID int) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, f := range b.modelFiles {
		if f.FileDataID == fileDataID {
			return f.FileName
		}
	}
	return ""
}

func sortBrowseEntries(modelFiles, textureFiles []casc.ListfileEntry) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		stringsort.SortBy(modelFiles, func(e casc.ListfileEntry) string { return e.FileName })
		wg.Done()
	}()
	go func() {
		stringsort.SortBy(textureFiles, func(e casc.ListfileEntry) string { return e.FileName })
		wg.Done()
	}()
	wg.Wait()
}

func waitForDataClient(ctx context.Context, c client.Client) {
	if err := c.WaitUntilReady(ctx); err != nil {
		log.Printf("browse preload: data client not ready: %v", err)
	}
}

func (b *browseIndexes) counts() (models, textures int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.modelFiles), len(b.textureFiles)
}

func registerBrowse(r Router, d *Deps) {
	indexes := &browseIndexes{}

	go func() {
		ctx := context.Background()
		waitForDataClient(ctx, d.Client)
		err := indexes.load(ctx, d.Client)
		if err != nil {
			log.Printf("browse index preload failed: %v", err)
			d.startup.browseFinished(false, 0, 0)
			return
		}
		models, textures := indexes.counts()
		d.startup.browseFinished(true, models, textures)
	}()

	r.Get("/browse", func(w http.ResponseWriter, req *http.Request) {
		if err := indexes.load(req.Context(), d.Client); err != nil {
			sendInternalError(w, err)
			return
		}
		q := req.URL.Query().Get("q")
		if q == "" {
			sendError(w, http.StatusBadRequest, "q is required")
			return
		}
		if q != "model" && q != "texture" {
			sendError(w, http.StatusBadRequest, `q must be "model" or "texture"`)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=60")
		sendJSON(w, http.StatusOK, indexes.snapshot(q))
	})

	r.Get("/browse/model-skins", func(w http.ResponseWriter, req *http.Request) {
		fileDataID, err := strconv.Atoi(req.URL.Query().Get("fileDataID"))
		if err != nil || fileDataID <= 0 {
			sendError(w, http.StatusBadRequest, "fileDataID is required")
			return
		}
		if err := indexes.load(req.Context(), d.Client); err != nil {
			sendInternalError(w, err)
			return
		}
		fileName := indexes.modelFileName(fileDataID)
		if fileName == "" {
			sendError(w, http.StatusNotFound, "Model not found")
			return
		}
		if err := d.Client.InitModelCaches(req.Context()); err != nil {
			sendInternalError(w, err)
			return
		}
		skins, err := d.Client.GetModelSkins(req.Context(), fileDataID)
		if err != nil {
			sendInternalError(w, err)
			return
		}
		options := character.GetModelSkinOptions(fileName, skins)
		w.Header().Set("Cache-Control", "no-store")
		sendJSON(w, http.StatusOK, map[string]any{
			"fileDataID": fileDataID,
			"fileName":   fileName,
			"skins":      options,
		})
	})
}

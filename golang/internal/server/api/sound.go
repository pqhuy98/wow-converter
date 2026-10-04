package api

import (
	"archive/zip"
	"bytes"
	"errors"
	"log"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/pqhuy98/wow-converter/internal/stringsort"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
)

const (
	soundRegex = `\.(mp3|ogg|wav)$`
	// maxSoundZipFiles bounds one zip response, which is built from files held in memory.
	maxSoundZipFiles = 1000
)

// soundIndex lists the raw sound files of the active CASC build, rebuilt when the build changes.
type soundIndex struct {
	mu       sync.Mutex
	buildKey string
	files    []casc.ListfileEntry
	nameByID map[int]string
}

func (s *soundIndex) load(r *http.Request, d *Deps) (files []casc.ListfileEntry, nameByID map[int]string, err error) {
	if err := d.Client.WaitUntilReady(r.Context()); err != nil {
		return nil, nil, err
	}
	buildKey := d.BuildKey(r.Context())
	if buildKey == "" {
		return nil, nil, errors.New("game data is not loaded")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.buildKey != buildKey {
		entries, err := d.Client.SearchFiles(r.Context(), soundRegex, true)
		if err != nil {
			return nil, nil, err
		}
		stringsort.SortBy(entries, func(e casc.ListfileEntry) string { return e.FileName })
		s.nameByID = make(map[int]string, len(entries))
		for _, e := range entries {
			s.nameByID[e.FileDataID] = e.FileName
		}
		s.files, s.buildKey = entries, buildKey
	}
	return s.files, s.nameByID, nil
}

func registerSound(r Router, d *Deps) {
	index := &soundIndex{}
	transcripts := loadSoundTranscripts()

	r.Get("/sound", func(w http.ResponseWriter, req *http.Request) {
		files, _, err := index.load(req, d)
		if err != nil {
			sendInternalError(w, err)
			return
		}
		search, err := listfileSearchQuery(req.URL.Query().Get("search"), d.Config.IsSharedHosting)
		if err != nil {
			sendError(w, http.StatusForbidden, err.Error())
			return
		}
		files = filterListfileSearch(files, search)
		buildKey := d.BuildKey(req.Context())
		etag := etagFromParts("sound-list", buildKey, search, strconv.Itoa(len(files)))
		if matchNotModified(req, etag) {
			writeNotModified(w, etag)
			return
		}
		applyCascBuildCache(w, req, d.Config, buildKey, etag, false)
		sendJSON(w, http.StatusOK, files)
	})

	r.Get("/sound/transcripts", func(w http.ResponseWriter, req *http.Request) {
		etag := etagFromParts("sound-transcripts", strconv.Itoa(len(transcripts)))
		if matchNotModified(req, etag) {
			writeNotModified(w, etag)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "private, max-age=3600")
		sendJSON(w, http.StatusOK, transcripts)
	})

	// Raw file bytes, playable inline by <audio>; ?download=1 forces a save dialog.
	r.Get("/sound/{fileDataID}", func(w http.ResponseWriter, req *http.Request) {
		fileDataID, err := strconv.Atoi(chi.URLParam(req, "fileDataID"))
		if err != nil || fileDataID <= 0 {
			sendError(w, http.StatusBadRequest, "Invalid fileDataID")
			return
		}
		_, nameByID, err := index.load(req, d)
		if err != nil {
			sendInternalError(w, err)
			return
		}
		fileName, ok := nameByID[fileDataID]
		if !ok {
			sendError(w, http.StatusNotFound, "Sound not found")
			return
		}
		data, err := d.Client.DownloadCascFile(req.Context(), fileDataID)
		if err != nil {
			sendError(w, http.StatusNotFound, "Sound data is missing")
			return
		}

		baseName := path.Base(fileName)
		w.Header().Set("Content-Type", soundContentType(fileName))
		if req.URL.Query().Get("download") == "1" {
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": baseName}))
		}
		w.Header().Set("ETag", etagFromParts("sound", d.BuildKey(req.Context()), strconv.Itoa(fileDataID)))
		w.Header().Set("Cache-Control", "private, max-age=3600")
		http.ServeContent(w, req, baseName, time.Time{}, bytes.NewReader(data)) // handles Range for seeking
	})

	// Zip of raw files, keeping their WoW folder structure.
	r.Post("/sound/zip", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			FileDataIDs []int `json:"fileDataIDs"`
		}
		if err := readJSONBody(req, &body); err != nil {
			if errors.Is(err, errRequestBodyTooLarge) {
				sendError(w, http.StatusRequestEntityTooLarge, "Request body too large")
				return
			}
			sendError(w, http.StatusBadRequest, "fileDataIDs is required")
			return
		}
		if len(body.FileDataIDs) == 0 || len(body.FileDataIDs) > maxSoundZipFiles {
			sendError(w, http.StatusBadRequest, "fileDataIDs must contain 1 to "+strconv.Itoa(maxSoundZipFiles)+" ids")
			return
		}
		// Temporary: one file per zip. Delete this check to restore bulk zip (limit stays maxSoundZipFiles).
		if len(body.FileDataIDs) > 1 {
			sendError(w, http.StatusBadRequest, "zip is limited to one file")
			return
		}
		_, nameByID, err := index.load(req, d)
		if err != nil {
			sendInternalError(w, err)
			return
		}

		// A file that is not in the list, or whose bytes cannot be read, is left out of the zip.
		type zipEntry struct {
			name string
			data []byte
		}
		entries := make([]zipEntry, 0, len(body.FileDataIDs))
		for _, id := range body.FileDataIDs {
			fileName, ok := nameByID[id]
			if !ok {
				log.Printf("sound zip skip %d: not in the sound list", id)
				continue
			}
			data, err := d.Client.DownloadCascFile(req.Context(), id)
			if err != nil {
				log.Printf("sound zip skip %d (%s): %v", id, fileName, err)
				continue
			}
			entries = append(entries, zipEntry{fileName, data})
		}

		// 2) Stream the zip. Audio is already compressed, so store it as is.
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="sounds.zip"`)
		zw := zip.NewWriter(w)
		defer zw.Close()
		for _, e := range entries {
			fw, err := zw.CreateHeader(&zip.FileHeader{Name: e.name, Method: zip.Store})
			if err != nil {
				return
			}
			if _, err := fw.Write(e.data); err != nil {
				return
			}
		}
	})
}

func soundContentType(fileName string) string {
	switch strings.ToLower(path.Ext(fileName)) {
	case ".mp3":
		return "audio/mpeg"
	case ".ogg":
		return "audio/ogg"
	default:
		return "audio/wav"
	}
}

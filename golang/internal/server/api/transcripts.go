package api

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/workspace"
)

const soundTranscriptZip = "resources/transcripts.final.zip"

// loadSoundTranscripts reads the transcript zip once at startup.
// The returned map is file id to subtitle. A missing zip leaves the map empty.
func loadSoundTranscripts() map[int]string {
	path := workspace.ResolveRepoPath(filepath.FromSlash(soundTranscriptZip))
	r, err := zip.OpenReader(path)
	if err != nil {
		log.Printf("sound transcripts: %v", err)
		return map[int]string{}
	}
	defer r.Close()
	return transcriptsFromZip(&r.Reader)
}

func transcriptsFromZip(r *zip.Reader) map[int]string {
	out := map[int]string{}
	for _, f := range r.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".jsonl") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			log.Printf("sound transcripts: %s: %v", f.Name, err)
			return out
		}
		sc := bufio.NewScanner(rc)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			var row struct {
				ID   int    `json:"id"`
				Text string `json:"text"`
			}
			if json.Unmarshal(sc.Bytes(), &row) != nil || row.ID <= 0 {
				continue
			}
			sub := strings.TrimSpace(row.Text)
			if sub == "" {
				continue
			}
			out[row.ID] = sub
		}
		rc.Close()
		if err := sc.Err(); err != nil {
			log.Printf("sound transcripts: %s: %v", f.Name, err)
		}
		return out
	}
	return out
}

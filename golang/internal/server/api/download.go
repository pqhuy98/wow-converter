package api

import (
	"archive/zip"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/server/pathsafe"
)

var versionSuffixRegex = regexp.MustCompile(`__([0-9a-fA-F]{32})\.(mdx|mdl)$`)

type downloadRequest struct {
	Files  []string `json:"files"`
	Source string   `json:"source"`
}

func registerDownload(r Router, d *Deps) {
	r.Post("/download", func(w http.ResponseWriter, req *http.Request) {
		var body downloadRequest
		if err := readJSONBody(req, &body); err != nil {
			if errors.Is(err, errRequestBodyTooLarge) {
				sendError(w, http.StatusRequestEntityTooLarge, "Request body too large")
				return
			}
			sendError(w, http.StatusBadRequest, "files is required")
			return
		}
		if len(body.Files) == 0 {
			sendError(w, http.StatusBadRequest, "files is required")
			return
		}

		baseDir := d.Config.OutputDir
		if body.Source == "browse" {
			baseDir = d.Config.OutputDirBrowse
		}

		zipName := "assets.zip"
		for _, p := range body.Files {
			if strings.HasSuffix(strings.ToLower(p), ".mdx") || strings.HasSuffix(strings.ToLower(p), ".mdl") {
				base := filepath.Base(p)
				base = versionSuffixRegex.ReplaceAllString(base, ".$2")
				zipName = strings.TrimSuffix(strings.TrimSuffix(base, ".mdx"), ".mdl") + ".zip"
				break
			}
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+zipName+`"`)

		zw := zip.NewWriter(w)
		defer zw.Close()

		for _, relativePath := range body.Files {
			f, err := pathsafe.OpenRegularFileUnderBase(baseDir, relativePath)
			if err != nil {
				if errors.Is(err, pathsafe.ErrInvalidPath) {
					sendError(w, http.StatusBadRequest, "Invalid path")
					return
				}
				if os.IsNotExist(err) {
					sendError(w, http.StatusBadRequest, "File not found")
					return
				}
				sendInternalError(w, err)
				return
			}

			archiveName := relativePath
			ext := strings.ToLower(filepath.Ext(relativePath))
			if ext == ".mdx" || ext == ".mdl" {
				archiveName = versionSuffixRegex.ReplaceAllString(archiveName, ".$2")
			}

			if err := addOpenFileToZip(zw, f, archiveName); err != nil {
				_ = f.Close()
				sendInternalError(w, err)
				return
			}
			_ = f.Close()
		}
	})
}

func addOpenFileToZip(zw *zip.Writer, f *os.File, archiveName string) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = strings.ReplaceAll(archiveName, `\`, "/")
	header.Method = zip.Deflate
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

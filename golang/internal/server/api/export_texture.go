package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/converter/icon"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
)

func registerExportTexture(r Router, d *Deps) {
	exporter := icon.NewExporter(d.Client, d.ExportAssetDir(), d.Config.OutputDir)

	r.Get("/texture/png/*", func(w http.ResponseWriter, req *http.Request) {
		texturePath := strings.TrimPrefix(req.URL.Path, "/api/texture/png/")
		if texturePath == "" {
			sendError(w, http.StatusBadRequest, "Texture path is required")
			return
		}

		isIconMode := req.URL.Query().Get("mode") == "icon"
		allowedPrefix := ""
		if isIconMode {
			allowedPrefix = "interface/icons/"
		}
		normalized, err := validateRelativePath(texturePath, allowedPrefix)
		if err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !strings.HasSuffix(strings.ToLower(normalized), ".png") {
			normalized = strings.TrimSuffix(normalized, ".blp") + ".png"
		}

		blpPath := strings.TrimSuffix(normalized, ".png") + ".blp"
		absPath, err := exporter.ExportPngByPath(req.Context(), blpPath)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				sendError(w, http.StatusNotFound, "Texture not found")
				return
			}
			sendInternalError(w, err)
			return
		}

		if isIconMode {
			if !strings.HasPrefix(strings.ToLower(normalized), "interface/icons/") {
				sendError(w, http.StatusBadRequest, "Icon conversion is only available for files in interface/icons directory")
				return
			}
			opts, err := parseIconQuery(req.URL.Query())
			if err != nil {
				sendError(w, http.StatusBadRequest, err.Error())
				return
			}
			iconBuffer, err := exporter.ConvertPngToIconBuffer(absPath, opts)
			if err != nil {
				sendInternalError(w, err)
				return
			}
			buildKey := d.BuildKey(req.Context())
			quotedETag := etagFromParts("texture-png-icon", buildKey, absPath, req.URL.RawQuery)
			if matchNotModified(req, quotedETag) {
				applyCascBuildCache(w, req, d.Config, buildKey, quotedETag, true)
				writeNotModified(w, quotedETag)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			applyCascBuildCache(w, req, d.Config, buildKey, quotedETag, true)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(iconBuffer)
			return
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			sendError(w, http.StatusNotFound, "Texture not found")
			return
		}
		buildKey := d.BuildKey(req.Context())
		quotedETag := etagFromParts("texture-png", buildKey, absPath)
		if matchNotModified(req, quotedETag) {
			applyCascBuildCache(w, req, d.Config, buildKey, quotedETag, true)
			writeNotModified(w, quotedETag)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		applyCascBuildCache(w, req, d.Config, buildKey, quotedETag, true)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	r.Post("/texture/blp", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Items []icon.ExportItem `json:"items"`
		}
		if err := readJSONBody(req, &body); err != nil {
			if errors.Is(err, errRequestBodyTooLarge) {
				sendError(w, http.StatusRequestEntityTooLarge, "Request body too large")
				return
			}
			sendError(w, http.StatusBadRequest, "No items provided")
			return
		}
		if len(body.Items) == 0 {
			sendError(w, http.StatusBadRequest, "No items provided")
			return
		}

		seen := map[string]struct{}{}
		var filtered []icon.ExportItem
		for _, item := range body.Items {
			prefix := ""
			if item.Options != nil {
				prefix = "interface/icons/"
			}
			texPath, err := validateRelativePath(item.TexturePath, prefix)
			if err != nil {
				sendError(w, http.StatusBadRequest, err.Error())
				return
			}
			item.TexturePath = texPath
			if item.OutputPath != "" {
				frame := icon.FrameNone
				if item.Options != nil {
					frame = icon.MergeOptions(*item.Options).Frame
				}
				outPath, err := validateIconOutputPath(item.OutputPath, frame)
				if err != nil {
					sendError(w, http.StatusBadRequest, err.Error())
					return
				}
				if _, ok := seen[outPath]; ok {
					continue
				}
				seen[outPath] = struct{}{}
				item.OutputPath = outPath
			} else if item.Options != nil {
				frame := icon.MergeOptions(*item.Options).Frame
				outPath := icon.GetWc3Path(texPath, frame)
				if _, ok := seen[outPath]; ok {
					continue
				}
				seen[outPath] = struct{}{}
				item.OutputPath = outPath
			}
			filtered = append(filtered, item)
		}

		count, paths, err := exporter.ExportToBlp(req.Context(), filtered)
		if err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		resp := map[string]any{"count": count, "paths": paths}
		if !d.Config.IsSharedHosting {
			resp["outputDirectory"] = filepath.Clean(d.Config.OutputDir)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func parseIconQuery(query url.Values) (icon.ConversionOptions, error) {
	opts := icon.ConversionOptions{
		Size:  icon.Size(query.Get("size")),
		Style: icon.Style(query.Get("style")),
		Frame: icon.Frame(query.Get("frame")),
	}
	if opts.Size == "" {
		return opts, fmt.Errorf("size is required for icon mode")
	}
	if query.Get("resizeMode") != "" {
		opts.ResizeMode = icon.ResizeMode(query.Get("resizeMode"))
	}
	if extrasRaw := query.Get("extras"); extrasRaw != "" {
		var extras icon.Extras
		if err := json.Unmarshal([]byte(extrasRaw), &extras); err != nil {
			return opts, fmt.Errorf("invalid extras JSON")
		}
		opts.Extras = &extras
	}
	return opts, nil
}

func validateIconOutputPath(outputPath string, frame icon.Frame) (string, error) {
	trimmed := strings.TrimSpace(outputPath)
	if trimmed == "" {
		return "", fmt.Errorf("invalid output path: output path cannot be empty")
	}
	if strings.Contains(trimmed, "..") {
		return "", fmt.Errorf("invalid output path: path traversal detected")
	}
	if frame != icon.FrameNone && strings.ContainsAny(trimmed, `/\`) {
		return "", fmt.Errorf("invalid output path: path separators are not allowed for this frame type")
	}
	clean := filepath.ToSlash(filepath.Clean(trimmed))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("invalid output path: absolute path not allowed")
	}
	return icon.GetWc3Path(clean, frame), nil
}

func ensureTexturePNG(ctx context.Context, d *Deps, relPNG, absPNG string) error {
	blpRel := strings.TrimSuffix(relPNG, ".png") + ".blp"
	files, err := d.Client.SearchFiles(ctx, strings.TrimSuffix(filepath.Base(blpRel), ".blp"), false)
	if err != nil {
		return err
	}
	var fileDataID int
	for _, f := range files {
		if strings.EqualFold(strings.ReplaceAll(f.FileName, "\\", "/"), blpRel) {
			fileDataID = f.FileDataID
			break
		}
	}
	if fileDataID == 0 {
		return fmt.Errorf("texture not found: %s", blpRel)
	}
	raw, err := d.Client.DownloadCascFile(ctx, fileDataID)
	if err != nil {
		return err
	}
	img, err := blp.NewBLPImage(buffer.From(raw))
	if err != nil {
		return err
	}
	pngBuf, err := img.ToPNG(0b1111, 0)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absPNG), 0o755); err != nil {
		return err
	}
	return os.WriteFile(absPNG, pngBuf.Raw(), 0o644)
}

func validateRelativePath(inputPath, allowedPrefix string) (string, error) {
	if strings.Contains(inputPath, "..") {
		return "", fmt.Errorf("invalid path: path traversal detected")
	}
	decoded, err := url.PathUnescape(inputPath)
	if err != nil {
		decoded = inputPath
	}
	if strings.Contains(decoded, "..") {
		return "", fmt.Errorf("invalid path: path traversal detected")
	}
	normalized := filepath.ToSlash(filepath.Clean(decoded))
	if filepath.IsAbs(normalized) || strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("invalid path: absolute path not allowed")
	}
	if allowedPrefix != "" {
		prefix := filepath.ToSlash(filepath.Clean(allowedPrefix))
		if !strings.HasPrefix(strings.ToLower(normalized), strings.ToLower(prefix)) {
			return "", fmt.Errorf("path must start with %s", allowedPrefix)
		}
	}
	return normalized, nil
}

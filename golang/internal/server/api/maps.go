package api

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

type tileInfo struct {
	X          int  `json:"x"`
	Y          int  `json:"y"`
	HasTexture bool `json:"hasTexture"`
}

type mapWithTiles struct {
	casc.MapEntry
	Tiles []tileInfo `json:"tiles"`
}

var (
	mapsMu         sync.RWMutex
	mapsIndexMu    sync.Mutex
	mapsWithTiles  []mapWithTiles
	mapsByDir     = map[string]*mapWithTiles{}
	fileNameIndex = map[string]casc.ListfileEntry{}

	tileBlpRegex = regexp.MustCompile(`^world/minimaps/([^/]+)/map(\d{1,2})_(\d{1,2})\.blp$`)
	tileAdtRegex = regexp.MustCompile(`(?i)^world/maps/([^/]+)/([^/]+)_(\d{2})_(\d{2})\.adt$`)
)

func resetMapsFileIndex() {
	mapsIndexMu.Lock()
	defer mapsIndexMu.Unlock()
	mapsMu.Lock()
	defer mapsMu.Unlock()
	mapsWithTiles = nil
	mapsByDir = map[string]*mapWithTiles{}
	fileNameIndex = map[string]casc.ListfileEntry{}
}

func init() {
	runtimecache.RegisterConverterClearHook(resetMapsFileIndex)
}

func buildMapsIndex(ctx context.Context, d *Deps) error {
	baseMaps, err := d.Client.GetMapList(ctx)
	if err != nil {
		baseMaps = nil
	}

	start := time.Now()
	var files []casc.ListfileEntry
	if direct, ok := d.Client.(client.DirectListfileClient); ok {
		log.Printf("Building maps index from listfile...")
		var err error
		files, err = direct.CollectMapTileFileIndex(ctx)
		if err == nil {
			log.Printf("Collected %d map tile files in %.1fs", len(files), time.Since(start).Seconds())
		} else {
			log.Printf("Direct map tile index unavailable (%v), falling back to full listfile copy", err)
			files = nil
		}
	}
	if files == nil {
		log.Printf("Building maps index (full listfile copy)...")
		var err error
		files, err = getListFiles(ctx, d.Client)
		if err != nil {
			return err
		}
		log.Printf("Loaded %d listfile entries for maps in %.1fs", len(files), time.Since(start).Seconds())
	}

	adtByDir := map[string]map[string]struct{}{}
	texByDir := map[string]map[string]struct{}{}
	localFileNameIndex := map[string]casc.ListfileEntry{}

	for _, entry := range files {
		fileName := strings.ToLower(strings.ReplaceAll(entry.FileName, `\`, `/`))
		if m := tileBlpRegex.FindStringSubmatch(fileName); m != nil {
			dir := m[1]
			x, _ := strconv.Atoi(m[2])
			y, _ := strconv.Atoi(m[3])
			if x >= 0 && x < 64 && y >= 0 && y < 64 {
				ensureSet(texByDir, dir)[tileKey(x, y)] = struct{}{}
				localFileNameIndex[fileName] = entry
			}
			continue
		}
		if m := tileAdtRegex.FindStringSubmatch(fileName); m != nil && strings.EqualFold(m[1], m[2]) {
			dir := strings.ToLower(m[1])
			x, _ := strconv.Atoi(m[3])
			y, _ := strconv.Atoi(m[4])
			if x >= 0 && x < 64 && y >= 0 && y < 64 {
				ensureSet(adtByDir, dir)[tileKey(x, y)] = struct{}{}
			}
		}
	}

	mapsMu.Lock()
	defer mapsMu.Unlock()
	mapsWithTiles = nil
	mapsByDir = map[string]*mapWithTiles{}
	fileNameIndex = localFileNameIndex

	for _, m := range baseMaps {
		dir := strings.ToLower(m.Dir)
		tilesMap := map[string]tileInfo{}

		for key := range adtByDir[dir] {
			x, y := parseTileKey(key)
			tilesMap[key] = tileInfo{X: x, Y: y, HasTexture: false}
		}
		for key := range texByDir[dir] {
			x, y := parseTileKey(key)
			if prev, ok := tilesMap[key]; ok {
				prev.HasTexture = true
				tilesMap[key] = prev
			} else {
				tilesMap[key] = tileInfo{X: x, Y: y, HasTexture: true}
			}
		}

		tiles := make([]tileInfo, 0, len(tilesMap))
		for _, t := range tilesMap {
			tiles = append(tiles, t)
		}
		withTiles := mapWithTiles{MapEntry: m, Tiles: tiles}
		mapsWithTiles = append(mapsWithTiles, withTiles)
		entry := withTiles
		mapsByDir[dir] = &entry
	}
	return nil
}

func ensureSet(m map[string]map[string]struct{}, dir string) map[string]struct{} {
	if m[dir] == nil {
		m[dir] = map[string]struct{}{}
	}
	return m[dir]
}

func tileKey(x, y int) string {
	return strconv.Itoa(x) + "," + strconv.Itoa(y)
}

func parseTileKey(key string) (int, int) {
	parts := strings.SplitN(key, ",", 2)
	x, _ := strconv.Atoi(parts[0])
	y, _ := strconv.Atoi(parts[1])
	return x, y
}

func ensureMapsIndex(ctx context.Context, d *Deps) error {
	mapsMu.RLock()
	if len(mapsWithTiles) > 0 {
		mapsMu.RUnlock()
		return nil
	}
	mapsMu.RUnlock()

	if err := d.Client.WaitUntilReady(ctx); err != nil {
		return err
	}

	mapsIndexMu.Lock()
	defer mapsIndexMu.Unlock()
	mapsMu.RLock()
	if len(mapsWithTiles) > 0 {
		mapsMu.RUnlock()
		return nil
	}
	mapsMu.RUnlock()
	return buildMapsIndex(ctx, d)
}

func resolveMap(key string) (id int, dir string, ok bool) {
	mapsMu.RLock()
	defer mapsMu.RUnlock()
	entry, found := mapsByDir[strings.ToLower(key)]
	if !found || entry == nil {
		return 0, "", false
	}
	return entry.ID, entry.Dir, true
}

func registerMaps(r Router, d *Deps) {
	go func() {
		defer d.startup.mapsFinished()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := ensureMapsIndex(ctx, d); err != nil {
			log.Printf("maps index preload failed: %v", err)
		}
	}()

	registerMapGenerateRoutes(r, d)

	r.Get("/maps", func(w http.ResponseWriter, req *http.Request) {
		if err := ensureMapsIndex(req.Context(), d); err != nil {
			sendInternalError(w, err)
			return
		}
		mapsMu.RLock()
		buildKey := d.BuildKey(req.Context())
		etag := etagFromParts("maps", buildKey, strconv.Itoa(len(mapsWithTiles)))
		if matchNotModified(req, etag) {
			mapsMu.RUnlock()
			applyCascBuildCache(w, req, d.Config, buildKey, etag, false)
			writeNotModified(w, etag)
			return
		}
		out := make([]map[string]any, 0, len(mapsWithTiles))
		for _, m := range mapsWithTiles {
			out = append(out, map[string]any{
				"id": m.ID, "name": m.Name, "dir": m.Dir, "expansionID": m.ExpansionID,
			})
		}
		mapsMu.RUnlock()
		applyCascBuildCache(w, req, d.Config, buildKey, etag, false)
		sendJSON(w, http.StatusOK, out)
	})

	r.Get("/maps/{map}/wdt-mask", func(w http.ResponseWriter, req *http.Request) {
		if err := ensureMapsIndex(req.Context(), d); err != nil {
			sendInternalError(w, err)
			return
		}
		mapKey := strings.ToLower(chi.URLParam(req, "map"))
		mapsMu.RLock()
		entry := mapsByDir[mapKey]
		var tiles []tileInfo
		if entry != nil {
			tiles = entry.Tiles
		}
		buildKey := d.BuildKey(req.Context())
		etag := etagFromParts("wdt-mask", buildKey, mapKey, strconv.Itoa(len(tiles)))
		if matchNotModified(req, etag) {
			mapsMu.RUnlock()
			applyCascBuildCache(w, req, d.Config, buildKey, etag, false)
			writeNotModified(w, etag)
			return
		}
		mapsMu.RUnlock()
		applyCascBuildCache(w, req, d.Config, buildKey, etag, false)
		sendJSON(w, http.StatusOK, map[string]any{
			"map": chi.URLParam(req, "map"), "size": 64, "tiles": tiles,
		})
	})

	r.Post("/maps/{map}/creatures-check", func(w http.ResponseWriter, req *http.Request) {
		if err := assertDesktopOnly(d.Config.IsSharedHosting); err != nil {
			sendError(w, http.StatusForbidden, err.Error())
			return
		}
		mapID, _, ok := resolveMap(chi.URLParam(req, "map"))
		if !ok {
			sendError(w, http.StatusNotFound, "Unknown map")
			return
		}
		var body struct {
			Tiles []exportAdtTile `json:"tiles"`
		}
		if err := readJSONBody(req, &body); err != nil || len(body.Tiles) == 0 {
			sendError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		ordered := dedupeTiles(body.Tiles)
		if len(ordered) == 0 {
			sendError(w, http.StatusBadRequest, "No valid tiles provided")
			return
		}

		tileCoords := make([][2]int, len(ordered))
		checkedTiles := make([]exportAdtTile, len(ordered))
		for i, tile := range ordered {
			if tile.X < 0 || tile.X >= 64 || tile.Y < 0 || tile.Y >= 64 {
				sendError(w, http.StatusBadRequest, "Tile coordinates must be within 0..63")
				return
			}
			tileCoords[i] = [2]int{tile.X, tile.Y}
			checkedTiles[i] = tile
		}

		count, err := azerothcore.CountCreaturesInTiles(mapID, tileCoords)
		if err != nil {
			if strings.Contains(err.Error(), "azerothcore database not found") {
				sendJSON(w, http.StatusOK, map[string]any{
					"hasCreatures":  false,
					"creatureCount": 0,
					"checkedTiles":  checkedTiles,
				})
				return
			}
			sendInternalError(w, err)
			return
		}

		sendJSON(w, http.StatusOK, map[string]any{
			"hasCreatures":  count > 0,
			"creatureCount": count,
			"checkedTiles":  checkedTiles,
		})
	})

	r.Get("/maps/{map}/minimap/{x}/{y}", func(w http.ResponseWriter, req *http.Request) {
		mapDir := strings.ToLower(chi.URLParam(req, "map"))
		x, errX := strconv.Atoi(chi.URLParam(req, "x"))
		y, errY := strconv.Atoi(chi.URLParam(req, "y"))
		if errX != nil || errY != nil || x < 0 || x >= 64 || y < 0 || y >= 64 {
			sendError(w, http.StatusBadRequest, "x and y must be within 0..63")
			return
		}

		xs := pad2(x)
		ys := pad2(y)

		buildKey := ""
		if info, err := d.Client.GetCASCInfo(req.Context()); err == nil {
			buildKey = info.BuildKey
		}
		etag := md5Hex(buildKey + "|" + mapDir + "|" + strconv.Itoa(x) + "|" + strconv.Itoa(y))
		quotedETag := `"` + etag + `"`
		if matchNotModified(req, quotedETag) {
			applyCascBuildCache(w, req, d.Config, buildKey, quotedETag, true)
			writeNotModified(w, quotedETag)
			return
		}

		assetDir := d.ExportAssetDir()
		preexisting := MinimapPngPath(assetDir, buildKey, mapDir, xs, ys)
		if data, err := os.ReadFile(preexisting); err == nil {
			servePNG(w, req, d.Config, buildKey, data, quotedETag)
			return
		}

		blpPath := "world/minimaps/" + mapDir + "/map" + xs + "_" + ys + ".blp"
		mapsMu.RLock()
		file, found := fileNameIndex[blpPath]
		mapsMu.RUnlock()
		if !found || file.FileDataID == 0 {
			sendError(w, http.StatusNotFound, "Minimap tile not found")
			return
		}

		raw, err := d.Client.DownloadCascFile(req.Context(), file.FileDataID)
		if err != nil {
			sendInternalError(w, err)
			return
		}
		img, err := blp.NewBLPImage(buffer.From(raw))
		if err != nil {
			sendInternalError(w, err)
			return
		}
		pngBuf, err := img.ToPNG(0b1111, 0)
		if err != nil {
			sendInternalError(w, err)
			return
		}
		if err := os.MkdirAll(filepath.Dir(preexisting), 0o755); err == nil {
			_ = os.WriteFile(preexisting, pngBuf.Raw(), 0o644)
		}
		servePNG(w, req, d.Config, buildKey, pngBuf.Raw(), quotedETag)
	})
}

func pad2(n int) string {
	return fmt.Sprintf("%02d", n)
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func servePNG(w http.ResponseWriter, req *http.Request, cfg Config, activeBuild string, data []byte, etag string) {
	w.Header().Set("Content-Type", "image/png")
	applyCascBuildCache(w, req, cfg, activeBuild, etag, true)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

package mapexporter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

var generatedTerrainAsset = regexp.MustCompile(`^(?:tex_\d+_\d+(?:_\d+)?\.blp|adt_\d+_\d+(?:-tileset-[0-9a-f]+)?(?:-chunks-v\d+-\d+x\d+-\d+-\d+)?(?:-edge-atlas-\d+\.blp|\.(?:mdl|mdx)))$`)

// prepareTerrainAssets hashes final encoded bytes, including every mip, before
// models are serialized. Equal terrain images share a deterministic path.
func (e *MapExporter) prepareTerrainAssets(ctx context.Context, outputDir string) error {
	dir, err := e.terrainAssetDir(outputDir)
	if err != nil {
		return err
	}
	e.terrainAssets = map[string]bool{}
	paths := map[string]bool{}
	models := e.WowObjectManager.AssetManager.Models()
	for _, model := range models {
		for _, tex := range model.MDL.Textures {
			path := strings.ReplaceAll(tex.Image, "\\", "/")
			if filepath.ToSlash(filepath.Dir(path)) == dir && generatedTerrainAsset.MatchString(filepath.Base(path)) {
				paths[path] = true
			}
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	canonical := map[[32]byte]string{}
	remap := map[string]string{}
	for _, path := range ordered {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(path)))
		if err != nil {
			return fmt.Errorf("read terrain texture %s: %w", path, err)
		}
		hash := sha256.Sum256(raw)
		if first, found := canonical[hash]; found {
			remap[path] = first
		} else {
			canonical[hash] = path
			e.terrainAssets[path] = true
		}
	}
	for _, model := range models {
		changed := false
		for _, tex := range model.MDL.Textures {
			if first := remap[strings.ReplaceAll(tex.Image, "\\", "/")]; first != "" {
				tex.Image = first
				// Retain source identity in WowData for future texture exports;
				// only the serialized import reference changes.
				changed = true
			}
		}
		if changed {
			model.AlwaysExport = true
		}
		path := strings.ReplaceAll(model.RelativePath, "\\", "/")
		if filepath.ToSlash(filepath.Dir(path)) == dir {
			ext := ".mdl"
			if e.Config.MDX {
				ext = ".mdx"
			}
			e.terrainAssets[strings.TrimSuffix(strings.TrimSuffix(path, ".mdx"), ".mdl")+ext] = true
		}
	}
	log.Printf("Terrain texture deduplication: %d redundant files", len(remap))
	return nil
}

// Only converter-owned terrain filenames in this map's direct asset directory
// are cleaned. This runs after successful map serialization, never on cancel.
func (e *MapExporter) cleanTerrainAssets(ctx context.Context, outputDir string) error {
	if e.terrainAssets == nil {
		return nil
	}
	dir, err := e.terrainAssetDir(outputDir)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(outputDir, filepath.FromSlash(dir)))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	obsolete := map[string]bool{}
	for _, entry := range entries {
		rel := dir + "/" + entry.Name()
		if !entry.IsDir() && generatedTerrainAsset.MatchString(entry.Name()) && !e.terrainAssets[rel] {
			obsolete[rel] = true
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Update imports first. Preserve unrelated entries and their original flags.
	existing, err := pruneTerrainImports(outputDir, dir, e.terrainAssets)
	if err != nil {
		return err
	}
	var imports []string
	for path := range e.terrainAssets {
		if !existing[path] {
			imports = append(imports, strings.ReplaceAll(path, "/", "\\"))
		}
	}
	if err := extra.EnsureCustomImports(outputDir, imports); err != nil {
		return err
	}
	for _, entry := range entries {
		rel := dir + "/" + entry.Name()
		if obsolete[rel] {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := os.Remove(filepath.Join(outputDir, filepath.FromSlash(rel))); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *MapExporter) terrainAssetDir(outputDir string) (string, error) {
	rel := filepath.Join(e.Config.AssetPrefix, "maps", e.MapExportConfig.WowExportFolder)
	if !filepath.IsLocal(rel) || !filepath.IsLocal(e.MapExportConfig.WowExportFolder) {
		return "", fmt.Errorf("invalid terrain asset directory")
	}
	// Refuse directory links, which could redirect cleanup outside this map.
	current := outputDir
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("linked terrain asset directory: %s", part)
		}
	}
	return filepath.ToSlash(rel), nil
}

func pruneTerrainImports(outputDir, terrainDir string, keep map[string]bool) (map[string]bool, error) {
	path := filepath.Join(outputDir, "war3map.imp")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) < 8 {
		return nil, fmt.Errorf("truncated map import list")
	}
	out := append([]byte(nil), raw[:8]...)
	count := uint32(0)
	existing := map[string]bool{}
	pos := 8
	for i := uint32(0); i < binary.LittleEndian.Uint32(raw[4:8]); i++ {
		start := pos
		if pos >= len(raw) {
			return nil, fmt.Errorf("truncated map import entry")
		}
		flag := raw[pos]
		pos++
		end := bytes.IndexByte(raw[pos:], 0)
		if end < 0 {
			return nil, fmt.Errorf("unterminated map import path")
		}
		name := strings.ReplaceAll(string(raw[pos:pos+end]), "\\", "/")
		pos += end + 1
		if flag&0xf == 13 && filepath.ToSlash(filepath.Dir(name)) == terrainDir && generatedTerrainAsset.MatchString(filepath.Base(name)) && !keep[name] {
			continue
		}
		if flag&0xf == 13 {
			existing[name] = true
		}
		out = append(out, raw[start:pos]...)
		count++
	}
	binary.LittleEndian.PutUint32(out[4:8], count)
	return existing, os.WriteFile(path, out, 0o644)
}

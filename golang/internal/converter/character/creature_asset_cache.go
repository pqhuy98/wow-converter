package character

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func exportCreatureAppearance(ctx context.Context, job *creatureExport, root string, cfg config.Config, wowClient client.Client) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	asset := job.asset
	destination := filepath.Join(root, filepath.FromSlash(asset.ModelPath))
	if job.cachedPath != "" {
		if err := os.Rename(job.cachedPath, destination); err != nil {
			return err
		}
		return fingerprintCreatureModel(ctx, root, asset)
	}
	ex := NewCharacterExporter(cfg, wowClient)
	name := strings.TrimSuffix(asset.ModelPath, ".mdx")
	model, err := ex.ExportCharacter(ctx, job.character, name, ExportOptions{resolvedBase: &job.metadata})
	if err != nil {
		return fmt.Errorf("export creature %s: %w", name, err)
	}
	ex.OptimizeModelsTextures(DefaultExportOptimization())
	if job.standReady {
		useNPCStandReady(model)
	}
	model.Modify.OptimizeNPCSequences()
	model.Sync()
	if _, err = ex.WriteAllTextures(ctx, root); err != nil {
		return err
	}
	for _, texture := range model.Textures {
		if texture.Image != "" {
			asset.Textures = append(asset.Textures, filepath.ToSlash(strings.ReplaceAll(texture.Image, "\\", "/")))
		}
	}
	if _, err = ex.WriteAllModels(ctx, root, "mdx"); err != nil {
		return err
	}
	return fingerprintCreatureModel(ctx, root, asset)
}

// Read the serialized chunks without changing them. MODL's fixed model-name
// field is identity only; every other byte, including materials/tracks, matters.
func fingerprintCreatureModel(ctx context.Context, root string, asset *CreatureAsset) error {
	file, err := os.Open(filepath.Join(root, filepath.FromSlash(asset.ModelPath)))
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	var magic [4]byte
	if _, err = io.ReadFull(file, magic[:]); err != nil {
		return err
	}
	if string(magic[:]) != "MDLX" {
		return fmt.Errorf("expected NPC MDX")
	}
	hash := sha256.New()
	asset.Textures = nil
	for offset := int64(4); offset < info.Size(); {
		if err = ctx.Err(); err != nil {
			return err
		}
		var header [8]byte
		if _, err = io.ReadFull(file, header[:]); err != nil {
			return err
		}
		offset += 8
		size := int64(binary.LittleEndian.Uint32(header[4:]))
		if size > info.Size()-offset {
			return io.ErrUnexpectedEOF
		}
		offset += size
		tag := string(header[:4])
		hash.Write(header[:])
		if tag == "MODL" {
			if size < 80 {
				return io.ErrUnexpectedEOF
			}
			if _, err = file.Seek(80, io.SeekCurrent); err != nil {
				return err
			}
			size -= 80
		}
		if tag == "TEXS" {
			if size%268 != 0 {
				return fmt.Errorf("invalid NPC texture chunk")
			}
			raw := make([]byte, size)
			if _, err = io.ReadFull(file, raw); err != nil {
				return err
			}
			hash.Write(raw)
			stream := mdx.NewBinaryStream(raw)
			for stream.Index() < len(raw) {
				t := mdx.NewTexture()
				if err = t.ReadMdx(stream); err != nil {
					return err
				}
				if t.Path != "" {
					asset.Textures = append(asset.Textures, strings.ReplaceAll(t.Path, "\\", "/"))
				}
			}
		} else if _, err = io.CopyN(hash, file, size); err != nil {
			return err
		}
	}
	asset.ContentKey = hex.EncodeToString(hash.Sum(nil))
	return ctx.Err()
}

// Snapshot reused files before writing any new names. Otherwise a renamed
// appearance could overwrite another job's old source file during a cache hit.
func stageCreatureCache(ctx context.Context, root string, jobs []*creatureExport, cfg config.Config, old map[string]*CreatureAsset) (func(), error) {
	noop := func() {}
	if cfg.OverrideModels {
		return noop, nil
	}
	stage, err := os.MkdirTemp(filepath.Join(root, "npcs"), ".models-")
	if err != nil {
		return noop, err
	}
	cleanup := func() { _ = os.RemoveAll(stage) }
	for i, job := range jobs {
		if err = ctx.Err(); err != nil {
			cleanup()
			return noop, err
		}
		source := ""
		if previous := old[job.asset.ConfigKey]; previous != nil && validCreatureAssetPath(previous.ModelPath) {
			source = previous.ModelPath
			for _, texture := range previous.Textures {
				path := filepath.Clean(filepath.FromSlash(texture))
				if filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
					source = ""
					break
				}
				if _, err = os.Stat(filepath.Join(root, path)); err != nil {
					source = ""
					break
				}
			}
			if source != "" {
				job.asset.ContentKey = previous.ContentKey
				job.asset.Textures = previous.Textures
			}
		}
		if source == "" {
			continue
		}
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(source)))
		if err != nil {
			continue
		}
		var magic [4]byte
		if _, err = io.ReadFull(file, magic[:]); err != nil || string(magic[:]) != "MDLX" {
			file.Close()
			continue
		}
		file.Seek(0, io.SeekStart)
		path := filepath.Join(stage, fmt.Sprintf("%d.mdx", i))
		copy, err := os.Create(path)
		if err != nil {
			file.Close()
			cleanup()
			return noop, err
		}
		_, err = io.Copy(copy, file)
		file.Close()
		closeErr := copy.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			cleanup()
			return noop, err
		}
		job.cachedPath = path
	}
	return cleanup, nil
}

func validCreatureAssetPath(path string) bool {
	return strings.HasPrefix(path, "npcs/") && filepath.ToSlash(filepath.Clean(path)) == path && !strings.Contains(path, "\\") && strings.HasSuffix(path, ".mdx")
}

func readCreatureAssetCache(root string) map[string]*CreatureAsset {
	result := map[string]*CreatureAsset{}
	raw, err := os.ReadFile(filepath.Join(root, "npcs", "models.json"))
	if err != nil {
		return result
	}
	if json.Unmarshal(raw, &result) != nil {
		return map[string]*CreatureAsset{}
	}
	for key, asset := range result {
		if asset == nil || !validCreatureAssetPath(asset.ModelPath) {
			delete(result, key)
		}
	}
	return result
}

func writeCreatureAssetCache(root string, assets map[string]*CreatureAsset) error {
	raw, err := json.MarshalIndent(assets, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Join(root, "npcs"), "models-*.tmp")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer os.Remove(path)
	if _, err = temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	return os.Rename(path, filepath.Join(root, "npcs", "models.json"))
}

func cleanCreatureAssets(root string, old, current map[string]*CreatureAsset) error {
	used := map[string]bool{}
	for _, asset := range current {
		used[asset.ModelPath] = true
	}
	for _, asset := range old {
		if !used[asset.ModelPath] && validCreatureAssetPath(asset.ModelPath) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(asset.ModelPath))); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	legacy := regexp.MustCompile(`^creature-[0-9]+\.(mdx|mdl)$`)
	for _, entry := range entries {
		if !entry.IsDir() && legacy.MatchString(entry.Name()) {
			if err = os.Remove(filepath.Join(root, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

package character

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"
	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

// ExportModelOptions configures skin guessing for model export.
type ExportModelOptions struct {
	TextureIDs   []int
	ExtraGeosets []int
}

// commonModel wraps an exported MDL.
type commonModel struct {
	MDL          *mdl.MDL
	RelativePath string
}

// ExportModelFileIDAsMdl exports an M2 by file data id.
func ExportModelFileIDAsMdl(ctx *ExportContext, modelFileID int, opts ExportModelOptions) (*commonModel, error) {
	skinName := pickSkin(ctx, modelFileID, opts.TextureIDs, opts.ExtraGeosets)
	m, err := ctx.AssetManager.ParseDirect(context.Background(), modelFileID, skinName, "")
	if err != nil {
		return nil, err
	}
	return &commonModel{MDL: m.MDL, RelativePath: m.RelativePath}, nil
}

func pickSkin(ctx *ExportContext, modelFileID int, textureIDs, extraGeosets []int) string {
	if len(textureIDs) == 0 && len(extraGeosets) == 0 {
		return ""
	}
	if ctx.WowClient == nil {
		return ""
	}
	skins, err := ctx.WowClient.GetModelSkins(context.Background(), modelFileID)
	if err != nil || len(skins) == 0 {
		return ""
	}
	bestIdx := 0
	bestScore := skinMatchScore(extraGeosets, textureIDs, skins[0].ExtraGeosets, skins[0].Textures)
	for i := 1; i < len(skins); i++ {
		score := skinMatchScore(extraGeosets, textureIDs, skins[i].ExtraGeosets, skins[i].Textures)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	skinName := skins[bestIdx].ID
	maxScore := skinMatchScore(extraGeosets, textureIDs, extraGeosets, textureIDs)
	confidence := float64(bestScore) / float64(maxScore)
	log.Printf("Chosen skin: %s with confidence: %.2f%% map[score:%d maxScore:%d skinIdx:%d]",
		skinName, confidence*100, bestScore, maxScore, bestIdx)
	return skinName
}

func skinMatchScore(wantGeosets, wantTextures, haveGeosets, haveTextures []int) int {
	textureScore := 0
	for _, id := range wantTextures {
		for _, h := range haveTextures {
			if id == h {
				textureScore++
			}
		}
	}
	geosetScore := 0
	for _, id := range wantGeosets {
		for _, h := range haveGeosets {
			if id == h {
				geosetScore++
			}
		}
	}
	penalty := 0
	for _, h := range haveGeosets {
		found := false
		for _, w := range wantGeosets {
			if w == h {
				found = true
				break
			}
		}
		if !found {
			penalty++
		}
	}
	return geosetScore*1000000 - penalty*1000 + textureScore
}

// ApplyReplaceableTextures swaps MDL textures using wowhead replaceable texture map.
func ApplyReplaceableTextures(ctx *ExportContext, m *mdl.MDL, replaceable map[string]int) error {
	cache := map[int]string{}
	replaceTexture := func(tex *components.Texture) error {
		if tex == nil {
			return nil
		}
		fileDataID, ok := replaceable[fmt.Sprintf("%d", tex.WowData.Type)]
		if !ok || fileDataID <= 0 {
			return nil
		}
		rel, ok := cache[fileDataID]
		if !ok {
			var err error
			rel, err = ExportTexture(ctx, fileDataID)
			if err != nil {
				return err
			}
			cache[fileDataID] = rel
		}
		ctx.AssetManager.AddPngTexture(rel, false)
		tex.Image = ctx.Config.AssetPrefix + "/" + rel
		tex.Image = replaceExt(tex.Image, ".png", ".blp")
		tex.WowData.PngPath = rel
		return nil
	}
	for i := range m.Textures {
		if err := replaceTexture(m.Textures[i]); err != nil {
			return err
		}
	}
	for i := range m.Geosets {
		if m.Geosets[i] == nil || m.Geosets[i].Material == nil {
			continue
		}
		for j := range m.Geosets[i].Material.Layers {
			if err := replaceTexture(m.Geosets[i].Material.Layers[j].Texture); err != nil {
				return err
			}
		}
	}
	for i := range m.ParticleEmitter2s {
		if err := replaceTexture(m.ParticleEmitter2s[i].Texture); err != nil {
			return err
		}
	}
	return nil
}

// ExportTexture exports a BLP texture to PNG and returns the relative path.
func ExportTexture(ctx *ExportContext, textureID int) (string, error) {
	rel, _, err := ExportTexturePNG(ctx, textureID)
	return rel, err
}

// ExportTexturePNG exports a BLP texture and returns the relative path with PNG bytes.
func ExportTexturePNG(ctx *ExportContext, textureID int) (string, []byte, error) {
	if textureID <= 0 {
		return "", nil, fmt.Errorf("invalid texture fileDataID: %d", textureID)
	}
	fileName := fmt.Sprintf("unknown/%d.blp", textureID)
	if ctx.WowClient != nil {
		if entry, err := ctx.WowClient.GetFileByID(context.Background(), textureID); err == nil && entry.FileName != "" {
			fileName = entry.FileName
		}
	}
	rel := normalizeTexturePath(replaceExt(fileName, ".blp", ".png"))
	if source, ok := texturesource.Get(rel); ok && source.Kind == texturesource.KindPNG && len(source.PNG) > 0 {
		return rel, source.PNG, nil
	}
	absPath := filepath.Join(ctx.Config.ExportAssetDir, rel)
	if common.ExportAssetExists(absPath) {
		data, err := os.ReadFile(absPath)
		if err != nil {
			return "", nil, err
		}
		texturesource.Register(rel, texturesource.Source{Kind: texturesource.KindBLP, FileDataID: textureID})
		return rel, data, nil
	}
	if ctx.WowClient == nil {
		return "", nil, fmt.Errorf("texture not found: %s", rel)
	}
	raw, err := ctx.WowClient.DownloadCascFile(context.Background(), textureID)
	if err != nil {
		return "", nil, err
	}
	img, err := blp.NewBLPImage(buffer.From(raw))
	if err != nil {
		return "", nil, err
	}
	pngBuf, err := img.ToPNG(0b1111, 0)
	if err != nil {
		return "", nil, err
	}
	png := pngBuf.Raw()
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(absPath, png, 0o644); err != nil {
		return "", nil, err
	}
	texturesource.Register(rel, texturesource.Source{Kind: texturesource.KindBLP, FileDataID: textureID})
	return rel, png, nil
}

// ResolveTexturePNGBytes loads PNG bytes from the texture-source registry or export cache.
func ResolveTexturePNGBytes(ctx *ExportContext, relPath string) ([]byte, error) {
	rel := filepath.ToSlash(relPath)
	if source, ok := texturesource.Get(rel); ok {
		switch source.Kind {
		case texturesource.KindPNG:
			return source.PNG, nil
		case texturesource.KindBLP:
			if ctx.WowClient == nil {
				return nil, fmt.Errorf("texture not found: %s", relPath)
			}
			raw, err := ctx.WowClient.DownloadCascFile(context.Background(), source.FileDataID)
			if err != nil {
				return nil, err
			}
			img, err := blp.NewBLPImage(buffer.From(raw))
			if err != nil {
				return nil, err
			}
			pngBuf, err := img.ToPNG(0b1111, 0)
			if err != nil {
				return nil, err
			}
			return pngBuf.Raw(), nil
		}
	}
	absPath := filepath.Join(ctx.Config.ExportAssetDir, filepath.FromSlash(rel))
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("texture not found: %s", relPath)
	}
	return data, nil
}

// TextureRelPath normalizes an absolute or relative export texture path.
func TextureRelPath(exportAssetDir, texturePath string) string {
	if filepath.IsAbs(texturePath) {
		rel, err := filepath.Rel(exportAssetDir, texturePath)
		if err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(texturePath)
}

func normalizeTexturePath(p string) string {
	return stringsReplaceAll(p, " ", "")
}

func stringsReplaceAll(s, old, new string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			continue
		}
		if s[i] == '\\' {
			out = append(out, '/')
		} else {
			out = append(out, s[i])
		}
	}
	return string(out)
}

func replaceExt(path, from, to string) string {
	if len(path) >= len(from) && path[len(path)-len(from):] == from {
		return path[:len(path)-len(from)] + to
	}
	return path + to
}

// LocalModelOptions configures local model export.
type LocalModelOptions struct {
	WithCollision  bool
	SkinIDOverride string
	KeepCinematic  bool
	AttackTag      animmap.AttackTag
}

// ExportLocalModelAsMdl resolves and converts a listfile local model reference.
func ExportLocalModelAsMdl(assetManager *common.AssetManager, cfg config.Config, wowClient client.Client, filePath string, opts LocalModelOptions) (*commonModel, *mdl.MDL, error) {
	fileName := common.NormalizeLocalModelRef(filePath)
	file, err := searchModelWithSkin(wowClient, fileName)
	if err != nil || file == nil {
		return nil, nil, fmt.Errorf("file %s not found in WoW assets", fileName)
	}
	skins, err := wowClient.GetModelSkins(context.Background(), file.FileDataID)
	if err != nil {
		return nil, nil, err
	}
	skin := pickLocalModelSkin(filePath, file.FileName, skins, opts.SkinIDOverride)
	skinName := ""
	if skin != nil {
		skinName = skin.ID
	}
	model, err := assetManager.ParseDirectOptions(context.Background(), common.DirectParseOptions{
		FileDataID:         file.FileDataID,
		SkinName:           skinName,
		ExportPathOverride: common.CachePathForLocalRef(cfg.ExportAssetDir, filePath, ".m2"),
	})
	if err != nil {
		return nil, nil, err
	}
	var collision *mdl.MDL
	if opts.WithCollision && wowClient != nil && !strings.HasSuffix(strings.ToLower(file.FileName), ".wmo") {
		collisionResult, err := directm2.ConvertM2CollisionToMdl(context.Background(), cfg, CascFileSource{Client: wowClient}, file.FileDataID)
		if err != nil {
			return nil, nil, err
		}
		collision = collisionResult.MDL
	}
	return &commonModel{MDL: model.MDL, RelativePath: model.RelativePath}, collision, nil
}

func searchModelWithSkin(wowClient client.Client, fileWithSkin string) (*listfileEntry, error) {
	dirName := filepath.Dir(strings.ReplaceAll(fileWithSkin, "\\", "/"))
	for i := len(fileWithSkin); i > len(dirName); i-- {
		searchPhrase := fileWithSkin[:i]
		files, err := wowClient.SearchFiles(context.Background(), searchPhrase, false)
		if err != nil {
			continue
		}
		for _, f := range files {
			if common.NormalizeLocalModelRef(f.FileName) == searchPhrase {
				return &listfileEntry{FileDataID: f.FileDataID, FileName: f.FileName}, nil
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

type listfileEntry struct {
	FileDataID int
	FileName   string
}

func localModelBasename(ref string) string {
	ref = strings.ReplaceAll(ref, "\\", "/")
	return filepath.Base(common.NormalizeLocalModelRef(ref))
}

func pickLocalModelSkin(filePath, modelFileName string, skins []casc.ModelSkin, override string) *casc.ModelSkin {
	if len(skins) == 0 {
		return nil
	}
	if override != "" {
		for i := range skins {
			if skins[i].ID == override {
				return &skins[i]
			}
		}
	}
	refBase := localModelBasename(filePath)
	modelBase := localModelBasename(modelFileName)
	for i := range skins {
		if skins[i].ID == refBase {
			return &skins[i]
		}
	}
	for i := range skins {
		id := skins[i].ID
		if strings.HasSuffix(refBase, "_"+id) || strings.HasSuffix(refBase, id) {
			return &skins[i]
		}
	}
	if refBase == modelBase {
		return &skins[0]
	}
	return &skins[0]
}

// ResolveLocalModelRef validates a local model ref against the WoW listfile.
func ResolveLocalModelRef(wowClient client.Client, localPath string) (ok bool, similarFiles []string, err error) {
	if wowClient == nil {
		return false, nil, nil
	}
	normalized := common.NormalizeLocalModelRef(localPath)
	file, err := searchModelWithSkin(wowClient, normalized)
	if err != nil || file == nil {
		return false, nil, err
	}
	skins, err := wowClient.GetModelSkins(context.Background(), file.FileDataID)
	if err != nil {
		return false, nil, err
	}
	return true, buildLocalRefVariants(file.FileName, skins), nil
}

func buildLocalRefVariants(listfilePath string, skins []casc.ModelSkin) []string {
	baseRef := common.NormalizeLocalModelRef(listfilePath)
	dir := filepath.Dir(strings.ReplaceAll(baseRef, "\\", "/"))
	modelBase := filepath.Base(baseRef)
	seen := map[string]struct{}{toLocalRefPath(dir, modelBase): {}}
	refs := []string{toLocalRefPath(dir, modelBase)}
	for _, skin := range skins {
		skinBase := skin.ID
		if idx := strings.Index(skinBase, ","); idx >= 0 {
			skinBase = skinBase[:idx]
		}
		if skinBase == modelBase {
			continue
		}
		candidates := []string{toLocalRefPath(dir, skinBase)}
		if !strings.HasPrefix(skinBase, modelBase) {
			candidates = append(candidates, toLocalRefPath(dir, modelBase+"_"+skinBase))
		}
		for _, ref := range candidates {
			if _, dup := seen[ref]; dup {
				continue
			}
			seen[ref] = struct{}{}
			refs = append(refs, ref)
		}
	}
	return refs
}

func toLocalRefPath(dir, base string) string {
	if dir == "." || dir == "" {
		return base
	}
	return dir + "/" + base
}

// LocalModelSkinOption is a browse skin picker entry.
type LocalModelSkinOption struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	LocalRef string `json:"localRef"`
}

// GetModelSkinOptions returns skin variants for the browse UI, matching TS getModelSkinOptions.
func GetModelSkinOptions(listfilePath string, skins []casc.ModelSkin) []LocalModelSkinOption {
	baseRef := common.NormalizeLocalModelRef(listfilePath)
	dir := filepath.Dir(strings.ReplaceAll(baseRef, "\\", "/"))
	modelBase := filepath.Base(baseRef)

	if len(skins) == 0 {
		return []LocalModelSkinOption{{
			ID: "", Label: "default", LocalRef: toBrowseLocalRefPath(dir, modelBase),
		}}
	}

	options := make([]LocalModelSkinOption, 0, len(skins))
	seen := map[string]struct{}{}
	for i, skin := range skins {
		option := LocalModelSkinOption{
			ID:       skin.ID,
			Label:    skin.Label,
			LocalRef: localRefForSkin(dir, modelBase, skin, i == 0),
		}
		if _, dup := seen[option.Label]; dup {
			continue
		}
		seen[option.Label] = struct{}{}
		options = append(options, option)
	}
	return options
}

func localRefForSkin(dir, modelBase string, skin casc.ModelSkin, isDefault bool) string {
	skinBase := skin.ID
	if idx := strings.Index(skinBase, ","); idx >= 0 {
		skinBase = skinBase[:idx]
	}
	if isDefault {
		return toBrowseLocalRefPath(dir, modelBase)
	}
	if strings.HasPrefix(skinBase, modelBase) {
		return toBrowseLocalRefPath(dir, skinBase)
	}
	if skinBase == modelBase {
		return toBrowseLocalRefPath(dir, modelBase)
	}
	return toBrowseLocalRefPath(dir, modelBase+"_"+skinBase)
}

func toBrowseLocalRefPath(dir, base string) string {
	if dir == "." || dir == "" {
		return base
	}
	return strings.ReplaceAll(dir+"/"+base, "/", "\\")
}

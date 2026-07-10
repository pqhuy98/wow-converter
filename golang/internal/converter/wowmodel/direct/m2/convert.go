package directm2

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/assemble"
	bundleanim "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	bundlemeta "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/metadata"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/mtl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// FileSource resolves CASC files for M2 conversion.
type FileSource interface {
	GetRawFile(ctx context.Context, fileDataID int) ([]byte, error)
	GetFileName(ctx context.Context, fileDataID int) (string, error)
	GetModelSkins(ctx context.Context, fileDataID int) ([]ModelSkin, error)
	GetBuildKey(ctx context.Context) (string, error)
}

// ModelSkin is a DB2 skin descriptor.
type ModelSkin struct {
	ID           string
	ExtraGeosets []int
	Textures     []int
}

// ConvertOptions configures direct M2 -> MDL conversion.
type ConvertOptions struct {
	FileDataID         int
	SkinName           string
	VariantTextures    []int
	GeosetMask         []m2export.GeosetMaskEntry
	GeosetMaskBuilder  func(*m2.Skin) []m2export.GeosetMaskEntry
	DataTextures       map[int]DirectDataTexture
	ExcludeAnimIDs     map[int]struct{}
	ExportPathOverride string
}

// ConvertResult is M2 conversion output.
type ConvertResult struct {
	MDL          *mdl.MDL
	TexturePaths map[string]struct{}
}

// ConvertM2ToMdl converts an M2 file to MDL via the direct pipeline.
func ConvertM2ToMdl(ctx context.Context, cfg config.Config, src FileSource, opts ConvertOptions) (ConvertResult, error) {
	raw, err := src.GetRawFile(ctx, opts.FileDataID)
	if err != nil {
		return ConvertResult{}, err
	}

	fileName, err := src.GetFileName(ctx, opts.FileDataID)
	if err != nil || fileName == "" {
		magic := buffer.From(raw).ReadUInt32LE().(int64)
		isM2 := uint32(magic) == constants.MagicMD20 || uint32(magic) == constants.MagicMD21
		ext := ".wmo"
		if isM2 {
			ext = ".m2"
		}
		fileName = fmt.Sprintf("unknown/%d%s", opts.FileDataID, ext)
	}
	if !strings.HasSuffix(strings.ToLower(fileName), ".m2") {
		return ConvertResult{}, fmt.Errorf("file %d is not an M2 model", opts.FileDataID)
	}

	getFile := func(_ context.Context, id uint32) ([]byte, error) {
		return src.GetRawFile(ctx, int(id))
	}
	loader := m2.NewLoader(buffer.From(raw), getFile)
	if err := loader.Load(ctx); err != nil {
		return ConvertResult{}, err
	}
	skin, err := loader.GetSkin(ctx, 0)
	if err != nil {
		return ConvertResult{}, err
	}
	if name, err := src.GetFileName(ctx, int(skin.FileDataID)); err == nil && name != "" {
		skin.FileName = name
	}

	variantTextures := opts.VariantTextures
	selectedSkinName := opts.SkinName
	if selectedSkinName != "" && len(variantTextures) == 0 {
		skins, _ := src.GetModelSkins(ctx, opts.FileDataID)
		for _, s := range skins {
			if s.ID == selectedSkinName {
				variantTextures = s.Textures
				break
			}
		}
	}

	exportRoot := cfg.ExportAssetDir
	exportPath := opts.ExportPathOverride
	if exportPath == "" {
		exportPath = resolveExportPath(exportRoot, fileName, selectedSkinName)
	}
	outDir := filepath.Dir(exportPath)

	geosetMask := opts.GeosetMask
	if geosetMask == nil && opts.GeosetMaskBuilder != nil {
		geosetMask = opts.GeosetMaskBuilder(skin)
	}
	if geosetMask == nil && selectedSkinName != "" {
		skins, _ := src.GetModelSkins(ctx, opts.FileDataID)
		var selected *ModelSkin
		for i := range skins {
			if skins[i].ID == selectedSkinName {
				selected = &skins[i]
				break
			}
		}
		geosetMask = BuildGeosetMaskForSkin(skin, selected)
	}

	dataTexSet := map[int]struct{}{}
	for k := range opts.DataTextures {
		dataTexSet[k] = struct{}{}
	}

	getRaw := func(c context.Context, id int) ([]byte, error) { return src.GetRawFile(c, id) }
	getName := func(c context.Context, id int) (string, error) { return src.GetFileName(c, id) }
	resolved, err := ResolveTextures(ctx, loader, variantTextures, opts.DataTextures, outDir, exportRoot, getRaw, getName)
	if err != nil {
		return ConvertResult{}, err
	}

	animFile, err := BuildBonesData(ctx, loader, opts.ExcludeAnimIDs, opts.FileDataID, buildKeyFromSource(ctx, src))
	if err != nil {
		return ConvertResult{}, err
	}

	metaObj, err := BuildMetadataObject(ctx, loader, skin, opts.FileDataID, fileName, geosetMask, resolved.ValidTextures, dataTexSet, getName)
	if err != nil {
		return ConvertResult{}, err
	}
	if norm := NormalizeJSONValues(metaObj); norm != nil {
		if normMap, ok := norm.(map[string]any); ok {
			metaObj = normMap
		}
	}
	meta := bundlemeta.NewFile(writers.ReplaceExtension(exportPath, ".json"), cfg, animFile)
	meta.LoadFromData(metaObj)

	meshes := BuildMeshes(loader, skin, geosetMask, resolved.ValidTextures, dataTexSet)
	var mtlLib string
	if len(resolved.MtlMaterials) > 0 {
		mtlLib = filepath.Base(writers.ReplaceExtension(exportPath, ".mtl"))
	}
	objResult := BuildObjResult(loader, meshes, filepath.Base(stripModelExt(exportPath)), mtlLib)

	var mtlMaterials []mtl.Material
	for _, mat := range resolved.MtlMaterials {
		mtlMaterials = append(mtlMaterials, mtl.Material{Name: mat.Name, MapKd: mat.MapKd})
	}

	assembled := assemble.AssembleWowModel(assemble.Inputs{
		ObjFilePath: exportPath,
		Obj:         objResult,
		Mtl:         struct{ Materials []mtl.Material }{Materials: mtlMaterials},
		Animation:   animFile,
		Metadata:    meta,
	}, cfg)

	return ConvertResult{MDL: assembled.MDL, TexturePaths: assembled.TexturePaths}, nil
}

func buildKeyFromSource(ctx context.Context, src FileSource) string {
	if src == nil {
		return ""
	}
	key, err := src.GetBuildKey(ctx)
	if err != nil {
		return ""
	}
	return key
}

// BuildGeosetMaskForSkin builds a geoset mask for the selected skin.
func BuildGeosetMaskForSkin(skin *m2.Skin, selected *ModelSkin) []m2export.GeosetMaskEntry {
	extraSet := map[int]struct{}{}
	if selected != nil {
		for _, g := range selected.ExtraGeosets {
			extraSet[g] = struct{}{}
		}
	}
	mask := make([]m2export.GeosetMaskEntry, len(skin.SubMeshes))
	for i, mesh := range skin.SubMeshes {
		id := int(mesh.SubmeshID)
		mask[i] = m2export.GeosetMaskEntry{ID: id, Checked: true}
		if selected != nil && len(selected.ExtraGeosets) > 0 {
			if id > 0 && id < 900 {
				mask[i].Checked = false
			}
			if _, ok := extraSet[id]; ok {
				mask[i].Checked = true
			}
		} else {
			idStr := fmt.Sprintf("%d", id)
			mask[i].Checked = strings.HasSuffix(idStr, "0") || strings.HasSuffix(idStr, "01")
		}
	}
	return mask
}

func resolveExportPath(exportRoot, fileName, skinName string) string {
	if skinName == "" {
		return virtualExportPath(exportRoot, fileName)
	}
	base := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	dir := filepath.Dir(fileName)
	var skinned string
	if strings.HasPrefix(skinName, base) {
		skinned = filepath.Join(dir, skinName+filepath.Ext(fileName))
	} else {
		skinned = filepath.Join(dir, base+"_"+skinName+filepath.Ext(fileName))
	}
	return virtualExportPath(exportRoot, skinned)
}

func stripModelExt(p string) string {
	for _, ext := range []string{".m2", ".wmo", ".mdl", ".mdx"} {
		if strings.HasSuffix(strings.ToLower(p), ext) {
			return strings.TrimSuffix(p, ext)
		}
	}
	return p
}

// NewAnimationFile creates a bones animation file placeholder.
func NewAnimationFile(path string, cfg config.Config) *bundleanim.File {
	return bundleanim.NewFile(path, cfg)
}

// ConvertM2CollisionToMdl converts M2 collision geometry to MDL.
func ConvertM2CollisionToMdl(ctx context.Context, cfg config.Config, src FileSource, fileDataID int) (ConvertResult, error) {
	raw, err := src.GetRawFile(ctx, fileDataID)
	if err != nil {
		return ConvertResult{}, err
	}
	fileName, err := src.GetFileName(ctx, fileDataID)
	if err != nil || fileName == "" {
		fileName = fmt.Sprintf("unknown/%d.m2", fileDataID)
	}
	if !strings.HasSuffix(strings.ToLower(fileName), ".m2") {
		return ConvertResult{}, fmt.Errorf("file %d is not an M2 model", fileDataID)
	}

	getFile := func(_ context.Context, id uint32) ([]byte, error) {
		return src.GetRawFile(ctx, int(id))
	}
	loader := m2.NewLoader(buffer.From(raw), getFile)
	if err := loader.Load(ctx); err != nil {
		return ConvertResult{}, err
	}

	exportPath := resolveExportPath(cfg.ExportAssetDir, fileName, "")
	physPath := writers.ReplaceExtension(exportPath, ".phys.m2")
	animFile := bundleanim.NewFile(writers.ReplaceExtension(physPath, "_bones.json"), cfg)
	meta := bundlemeta.NewFile(writers.ReplaceExtension(physPath, ".json"), cfg, animFile)
	objResult := BuildCollisionObjResult(loader, "Mesh")
	assembled := assemble.AssembleWowModel(assemble.Inputs{
		ObjFilePath: physPath,
		Obj:         objResult,
		Mtl:         struct{ Materials []mtl.Material }{Materials: nil},
		Animation:   animFile,
		Metadata:    meta,
	}, cfg)
	return ConvertResult{MDL: assembled.MDL, TexturePaths: assembled.TexturePaths}, nil
}

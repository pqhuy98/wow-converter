package common

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	stdmath "math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel"
	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	directwmo "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/wmo"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

var adtTileRe = regexp.MustCompile(`(?i)adt_(\d+)_(\d+)$`)

// AbsoluteExtents is min/max world bounds.
type AbsoluteExtents struct {
	Min, Max math.Vector3
}

// AssetManager tracks converted models and textures for export.
type AssetManager struct {
	config            config.Config
	wowClient         client.Client
	models            map[string]*Model
	textures          map[string]struct{}
	texturesOverwrite map[string]struct{}
	blpTextures       map[string][]byte
}

// NewAssetManager creates an asset manager.
func NewAssetManager(cfg config.Config, wowClient client.Client) *AssetManager {
	return &AssetManager{
		config:            cfg,
		wowClient:         wowClient,
		models:            map[string]*Model{},
		textures:          map[string]struct{}{},
		texturesOverwrite: map[string]struct{}{},
		blpTextures:       map[string][]byte{},
	}
}

// Models returns the model cache.
func (a *AssetManager) Models() map[string]*Model { return a.models }

// Parse resolves and caches a model by path.
func (a *AssetManager) Parse(objectPath string, noCache bool) (*Model, error) {
	return a.ResolveModel(objectPath, 0, WowObjectM2, noCache)
}

// ResolveModel loads ADT terrain from OBJ or M2/WMO from CASC.
func (a *AssetManager) ResolveModel(objectPath string, fileDataID int, typ WowObjectType, noCache bool) (*Model, error) {
	if m, ok := a.models[objectPath]; ok && !noCache {
		return m, nil
	}

	if typ == WowObjectADT {
		objPath := filepath.Join(a.config.ExportAssetDir, NormalizeLocalModelRef(objectPath)+".obj")
		result, err := wowmodel.ConvertAdtTerrainObjToMdl(objPath, a.config)
		if err != nil {
			return nil, err
		}
		model := &Model{RelativePath: result.MDL.Model.Name, MDL: result.MDL, TexturePaths: result.TexturePaths}
		for _, p := range result.TexturePaths {
			a.textures[normalizeRel(p)] = struct{}{}
		}
		if !noCache {
			a.models[objectPath] = model
		}
		return model, nil
	}

	if fileDataID <= 0 {
		return nil, fmt.Errorf("model not found: %s (missing fileDataID for %s)", objectPath, typ)
	}
	ext := ".m2"
	if typ == WowObjectWMO {
		ext = ".wmo"
	}
	skinName := ""
	model, err := a.ParseDirect(context.Background(), fileDataID, skinName, CachePathForLocalRef(a.config.ExportAssetDir, objectPath, ext))
	if err != nil {
		return nil, err
	}
	if !noCache {
		a.models[objectPath] = model
	}
	return model, nil
}

// ParseDirect converts M2/WMO directly from CASC.
func (a *AssetManager) ParseDirect(ctx context.Context, fileDataID int, skinName, exportPathOverride string) (*Model, error) {
	return a.ParseDirectOptions(ctx, DirectParseOptions{
		FileDataID: fileDataID, SkinName: skinName, ExportPathOverride: exportPathOverride,
	})
}

// DirectParseOptions configures direct M2 conversion.
type DirectParseOptions struct {
	FileDataID         int
	SkinName           string
	GeosetMask         []m2export.GeosetMaskEntry
	GeosetMaskBuilder  func(*m2.Skin) []m2export.GeosetMaskEntry
	DataTextures       map[int]directm2.DirectDataTexture
	ExcludeAnimIDs     map[int]struct{}
	ExportPathOverride string
}

// ParseDirectOptions converts M2/WMO with full direct pipeline options.
func (a *AssetManager) ParseDirectOptions(ctx context.Context, opts DirectParseOptions) (*Model, error) {
	src := characterCascSource{client: a.wowClient}
	raw, err := src.GetRawFile(ctx, opts.FileDataID)
	if err != nil {
		return nil, err
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

	var result directm2.ConvertResult
	if strings.HasSuffix(strings.ToLower(fileName), ".m2") {
		result, err = directm2.ConvertM2ToMdl(ctx, a.config, src, directm2.ConvertOptions{
			FileDataID:         opts.FileDataID,
			SkinName:           opts.SkinName,
			GeosetMask:         opts.GeosetMask,
			GeosetMaskBuilder:  opts.GeosetMaskBuilder,
			DataTextures:       opts.DataTextures,
			ExcludeAnimIDs:     opts.ExcludeAnimIDs,
			ExportPathOverride: opts.ExportPathOverride,
		})
	} else {
		result, err = directwmo.ConvertWmoToMdl(ctx, a.config, src, directwmo.ConvertOptions{
			FileDataID:         opts.FileDataID,
			FileName:           fileName,
			Raw:                raw,
			ExportPathOverride: opts.ExportPathOverride,
		})
	}
	if err != nil {
		return nil, err
	}
	m := result.MDL
	model := &Model{RelativePath: m.Model.Name, MDL: m}
	for p := range result.TexturePaths {
		a.textures[normalizeRel(p)] = struct{}{}
		model.TexturePaths = append(model.TexturePaths, normalizeRel(p))
	}
	return model, nil
}

type characterCascSource struct {
	client client.Client
}

func (s characterCascSource) GetRawFile(ctx context.Context, fileDataID int) ([]byte, error) {
	return s.client.DownloadCascFile(ctx, fileDataID)
}

func (s characterCascSource) GetFileName(ctx context.Context, fileDataID int) (string, error) {
	entry, err := s.client.GetFileByID(ctx, fileDataID)
	if err != nil {
		return "", err
	}
	return entry.FileName, nil
}

func (s characterCascSource) GetModelSkins(ctx context.Context, fileDataID int) ([]directm2.ModelSkin, error) {
	skins, err := s.client.GetModelSkins(ctx, fileDataID)
	if err != nil {
		return nil, err
	}
	out := make([]directm2.ModelSkin, len(skins))
	for i, sk := range skins {
		out[i] = directm2.ModelSkin{ID: sk.ID, ExtraGeosets: sk.ExtraGeosets, Textures: sk.Textures}
	}
	return out, nil
}

func (s characterCascSource) GetBuildKey(ctx context.Context) (string, error) {
	if s.client == nil {
		return "", nil
	}
	info, err := s.client.GetCASCInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.BuildKey, nil
}

// RegisterBLPTexture registers raw BLP bytes for export.
func (a *AssetManager) RegisterBLPTexture(relPath string, raw []byte) {
	rel := normalizeRel(relPath)
	a.textures[rel] = struct{}{}
	a.blpTextures[rel] = raw
}

func (a *AssetManager) AddPngTexture(texturePath string, overwrite bool) {
	rel := normalizeRel(texturePath)
	a.textures[rel] = struct{}{}
	if overwrite {
		a.texturesOverwrite[rel] = struct{}{}
	}
}

// ReleaseGeneratedTextureSources drops in-memory PNG registry entries for this
// export's texture paths after BLP encoding completes.
func (a *AssetManager) ReleaseGeneratedTextureSources() {
	paths := make([]string, 0, len(a.textures))
	for rel := range a.textures {
		paths = append(paths, rel)
	}
	texturesource.ReleaseGeneratedPNG(paths)
}

// ExportModels writes MDL/MDX files to assetPath.
func (a *AssetManager) ExportModels(assetPath string) error {
	log.Printf("Exporting models to %s ...", assetPath)
	start := time.Now()
	var writeCountAtomic atomic.Int32
	a.SmoothAdtTileBorders()
	tasks := make([]func() error, 0, len(a.models))
	for relPath, model := range a.models {
		relPath := relPath
		model := model
		tasks = append(tasks, func() error {
			ext := ".mdl"
			if a.config.MDX {
				ext = ".mdx"
			}
			full := filepath.Join(assetPath, a.config.AssetPrefix, relPath+ext)
			if !a.config.OverrideModels && ExportAssetExists(full) {
				return nil
			}
			mdlModel := model.MDL
			if mdlModel.Model.BoundsRadius > a.config.InfiniteExtentBoundRadiusThreshold {
				mdlModel.Modify.SetLargeBounds()
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if a.config.MDX {
				data, err := mdlModel.ToMdx()
				if err != nil {
					return err
				}
				if err := os.WriteFile(full, data, 0o644); err != nil {
					return err
				}
			} else if err := os.WriteFile(full, []byte(mdlModel.ToMdl()), 0o644); err != nil {
				return err
			}
			writeCountAtomic.Add(1)
			return nil
		})
	}
	if err := WorkerPool(config.MaxConcurrency(), tasks); err != nil {
		return err
	}
	writeCount := int(writeCountAtomic.Load())
	durationS := time.Since(start).Seconds()
	if durationS <= 0 {
		durationS = 0.01
	}
	log.Printf("Models export took %s (%s models/s)",
		ansi.Yellowf("%.2f", durationS)+" s",
		ansi.Grayf("%.2f", float64(writeCount)/durationS))
	return nil
}

type blpConvertItem struct {
	pngPath  string
	pngData  []byte
	rawBLP   []byte
	resizeTo *blp.Size
	outPath  string
}

type texturePrepResult struct {
	item       *blpConvertItem
	exported   string // non-empty when already written / skipped conversion
	countsWork bool
}

// ExportTextures writes BLP textures.
func (a *AssetManager) ExportTextures(assetPath string) ([]string, error) {
	log.Printf("Exporting textures to %s ...", assetPath)
	_ = os.MkdirAll(assetPath, 0o755)

	relPaths := make([]string, 0, len(a.textures))
	for rel := range a.textures {
		relPaths = append(relPaths, rel)
	}

	results := make([]texturePrepResult, len(relPaths))
	prepTasks := make([]func() error, len(relPaths))
	for i, rel := range relPaths {
		i, rel := i, rel
		prepTasks[i] = func() error {
			results[i] = a.prepTextureForExport(rel, assetPath)
			return nil
		}
	}
	if err := WorkerPool(config.MaxConcurrency(), prepTasks); err != nil {
		return nil, err
	}

	var exported []string
	var toProcess []blpConvertItem
	writeCount := 0
	for _, r := range results {
		if r.exported != "" {
			exported = append(exported, r.exported)
		}
		if r.countsWork {
			writeCount++
		}
		if r.item != nil {
			toProcess = append(toProcess, *r.item)
		}
	}
	totalTextures := len(a.textures)

	if len(toProcess) > 0 {
		workers := blp.GetWorkerPoolSize()
		if workers <= 0 {
			workers = config.MaxConcurrency()
		}
		if workers > len(toProcess) {
			workers = len(toProcess)
		}
		log.Printf("Converting %s textures to BLPs (%s concurrent threads)",
			ansi.Yellowf("%d", len(toProcess)),
			ansi.Yellowf("%d", workers))
		blpStart := time.Now()
		blp.EnsureWorkerPool(0)

		converted := make([]string, len(toProcess))
		convTasks := make([]func() error, len(toProcess))
		for i, item := range toProcess {
			i, item := i, item
			convTasks[i] = func() error {
				if err := os.MkdirAll(filepath.Dir(item.outPath), 0o755); err != nil {
					return err
				}
				switch {
				case len(item.rawBLP) > 0:
					if err := blp.SubmitBlpTask(blp.TaskInput{Kind: "blp2", Data: item.rawBLP, ResizeTo: item.resizeTo}, item.outPath); err != nil {
						return err
					}
				case len(item.pngData) > 0:
					if err := blp.SubmitBlpTask(blp.TaskInput{Kind: "png", Data: item.pngData, ResizeTo: item.resizeTo}, item.outPath); err != nil {
						return err
					}
				case item.pngPath != "":
					data, err := os.ReadFile(item.pngPath)
					if err != nil {
						return nil
					}
					if err := blp.SubmitBlpTask(blp.TaskInput{Kind: "png", Data: data, ResizeTo: item.resizeTo}, item.outPath); err != nil {
						return err
					}
				default:
					return nil
				}
				converted[i] = item.outPath
				return nil
			}
		}
		if err := WorkerPool(workers, convTasks); err != nil {
			return exported, err
		}
		for _, p := range converted {
			if p != "" {
				exported = append(exported, p)
			}
		}

		blpDurationS := time.Since(blpStart).Seconds()
		if blpDurationS <= 0 {
			blpDurationS = 0.01
		}
		log.Printf("Texture BLP conversion took %s (%s textures/s)",
			ansi.Yellowf("%.2f", blpDurationS)+" s",
			ansi.Grayf("%.2f", float64(len(toProcess))/blpDurationS))
	}

	skipped := totalTextures - writeCount
	log.Printf("Wrote %s, skipped %s textures. Total: %s",
		ansi.Yellowf("%d", writeCount),
		ansi.Grayf("%d", skipped),
		ansi.Yellowf("%d", len(exported)))
	a.ReleaseGeneratedTextureSources()
	return exported, nil
}

func (a *AssetManager) prepTextureForExport(rel, assetPath string) texturePrepResult {
	fromPath := filepath.Join(a.config.ExportAssetDir, rel)
	source, hasSource := texturesource.Get(rel)
	if !hasSource && !ExportAssetExists(fromPath) {
		log.Printf("Skipping texture not found %s", fromPath)
		return texturePrepResult{}
	}

	maxSize := a.config.MaxTextureSize
	if maxSize <= 0 {
		maxSize = stdmath.MaxInt32
	}
	width, height := readTextureDimensions(source, hasSource, fromPath, a)
	if width == 0 && height == 0 && hasSource {
		log.Printf("Failed to read texture metadata, proceeding without resize: %s", fromPath)
	}
	scale := stdmath.Min(1, float64(maxSize)/stdmath.Max(float64(width), float64(height)))
	targetW := int(stdmath.Round(float64(width) * scale))
	targetH := int(stdmath.Round(float64(height) * scale))

	blpRel := strings.ReplaceAll(rel, ".png", ".blp")
	outPath := filepath.Join(assetPath, a.config.AssetPrefix, blpRel)
	if ExportAssetExists(outPath) && !a.config.OverrideTextures {
		if _, ok := a.texturesOverwrite[rel]; !ok {
			if size := blp.ReadBlpSizeSync(outPath); size != nil && int(size.Width) == targetW && int(size.Height) == targetH {
				return texturePrepResult{exported: outPath, countsWork: true}
			}
		}
	}

	var resizeTo *blp.Size
	if a.config.MaxTextureSize > 0 && (width > targetW || height > targetH) {
		resizeTo = &blp.Size{Width: targetW, Height: targetH}
	}

	item := blpConvertItem{outPath: outPath, resizeTo: resizeTo}
	switch {
	case hasSource && source.Kind == texturesource.KindBLP && a.wowClient != nil:
		raw, err := a.wowClient.DownloadCascFile(context.Background(), source.FileDataID)
		if err != nil {
			return texturePrepResult{}
		}
		item.rawBLP = raw
	case hasSource && source.Kind == texturesource.KindPNG:
		item.pngData = append([]byte(nil), source.PNG...)
	default:
		item.pngPath = fromPath
	}
	return texturePrepResult{item: &item, countsWork: true}
}

func readTextureDimensions(source texturesource.Source, hasSource bool, fromPath string, a *AssetManager) (int, int) {
	if hasSource && source.Kind == texturesource.KindBLP && a.wowClient != nil {
		raw, err := a.wowClient.DownloadCascFile(context.Background(), source.FileDataID)
		if err == nil && len(raw) >= 20 {
			b := buffer.From(raw)
			b.Seek(12)
			w := int(b.ReadUInt32LE().(int64))
			b.Seek(16)
			h := int(b.ReadUInt32LE().(int64))
			return w, h
		}
	}
	if hasSource && source.Kind == texturesource.KindPNG {
		cfg, _, err := image.DecodeConfig(strings.NewReader(string(source.PNG)))
		if err == nil {
			return cfg.Width, cfg.Height
		}
	}
	f, err := os.Open(fromPath)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// PurgeTextures keeps only used texture paths.
func (a *AssetManager) PurgeTextures(used []string) {
	keep := map[string]struct{}{}
	for _, p := range used {
		p = strings.TrimSuffix(strings.TrimSuffix(p, ".blp"), ".png")
		p = strings.TrimPrefix(p, a.config.AssetPrefix+"/")
		p = strings.TrimPrefix(p, a.config.AssetPrefix+"\\")
		p = strings.ReplaceAll(p, "\\", "/")
		keep[p] = struct{}{}
	}
	for p := range a.textures {
		base := strings.TrimSuffix(strings.TrimSuffix(p, ".blp"), ".png")
		if _, ok := keep[base]; !ok {
			delete(a.textures, p)
		}
	}
}

// SmoothAdtTileBorders harmonizes normals across adjacent ADT tiles.
func (a *AssetManager) SmoothAdtTileBorders() {
	tiles := map[string]*Model{}
	for _, model := range a.models {
		base := filepath.Base(model.MDL.Model.Name)
		if m := adtTileRe.FindStringSubmatch(base); len(m) == 3 {
			key := m[1] + "_" + m[2]
			tiles[key] = model
		}
	}
	get := func(x, y int) *Model {
		return tiles[fmt.Sprintf("%d_%d", x, y)]
	}
	const posEps = 1e-2
	const borderEps = 500
	q := func(x float64) float64 { return stdmath.Round(x/posEps) * posEps }

	averageBorder := func(aModel, bModel *Model, axis byte) {
		aMinX, aMaxX, aMinZ, aMaxZ := geosetBounds(aModel)
		bMinX, _, bMinZ, _ := geosetBounds(bModel)
		var aSel, bSel func(*components.GeosetVertex) bool
		var keyFrom func(*components.GeosetVertex) float64
		if axis == 'x' {
			borderX := (aMaxX + bMinX) * 0.5
			aSel = func(v *components.GeosetVertex) bool { return stdmath.Abs(v.Position[0]-borderX) <= borderEps }
			bSel = func(v *components.GeosetVertex) bool { return stdmath.Abs(v.Position[0]-borderX) <= borderEps }
			keyFrom = func(v *components.GeosetVertex) float64 { return q(v.Position[2]) }
		} else {
			borderZ := (aMaxZ + bMinZ) * 0.5
			aSel = func(v *components.GeosetVertex) bool { return stdmath.Abs(v.Position[2]-borderZ) <= borderEps }
			bSel = func(v *components.GeosetVertex) bool { return stdmath.Abs(v.Position[2]-borderZ) <= borderEps }
			keyFrom = func(v *components.GeosetVertex) float64 { return q(v.Position[0]) }
		}
		aMap := map[float64][]*components.GeosetVertex{}
		bMap := map[float64][]*components.GeosetVertex{}
		for _, g := range aModel.MDL.Geosets {
			for _, v := range g.Vertices {
				if aSel(v) {
					k := keyFrom(v)
					aMap[k] = append(aMap[k], v)
				}
			}
		}
		for _, g := range bModel.MDL.Geosets {
			for _, v := range g.Vertices {
				if bSel(v) {
					k := keyFrom(v)
					bMap[k] = append(bMap[k], v)
				}
			}
		}
		for k, aVerts := range aMap {
			bVerts := bMap[k]
			if len(bVerts) == 0 {
				continue
			}
			var nx, ny, nz float64
			add := func(v *components.GeosetVertex) {
				nx += v.Normal[0]
				ny += v.Normal[1]
				nz += v.Normal[2]
			}
			for _, v := range aVerts {
				add(v)
			}
			for _, v := range bVerts {
				add(v)
			}
			lenN := stdmath.Sqrt(nx*nx + ny*ny + nz*nz)
			if lenN == 0 {
				lenN = 1
			}
			avg := math.Vector3{nx / lenN, ny / lenN, nz / lenN}
			set := func(v *components.GeosetVertex) { v.Normal = avg }
			for _, v := range aVerts {
				set(v)
			}
			for _, v := range bVerts {
				set(v)
			}
		}
		_, _, _, _ = aMinX, aMaxX, aMinZ, aMaxZ
	}

	for key, model := range tiles {
		parts := strings.Split(key, "_")
		if len(parts) != 2 {
			continue
		}
		var sx, sy int
		fmt.Sscanf(parts[0], "%d", &sx)
		fmt.Sscanf(parts[1], "%d", &sy)
		if east := get(sx+1, sy); east != nil {
			averageBorder(model, east, 'x')
		}
		if north := get(sx, sy+1); north != nil {
			averageBorder(model, north, 'z')
		}
	}
}

func geosetBounds(m *Model) (minX, maxX, minZ, maxZ float64) {
	minX, minZ = stdmath.MaxFloat64, stdmath.MaxFloat64
	maxX, maxZ = -stdmath.MaxFloat64, -stdmath.MaxFloat64
	for _, g := range m.MDL.Geosets {
		for _, v := range g.Vertices {
			if v.Position[0] < minX {
				minX = v.Position[0]
			}
			if v.Position[0] > maxX {
				maxX = v.Position[0]
			}
			if v.Position[2] < minZ {
				minZ = v.Position[2]
			}
			if v.Position[2] > maxZ {
				maxZ = v.Position[2]
			}
		}
	}
	return minX, maxX, minZ, maxZ
}

func normalizeRel(p string) string {
	return strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
}

// ComputeAbsoluteMinMaxExtents computes world bounds from roots using model vertices.
func ComputeAbsoluteMinMaxExtents(roots []*WowObject) AbsoluteExtents {
	ext := AbsoluteExtents{Min: math.V3All(stdmath.Inf(1)), Max: math.V3All(-stdmath.Inf(1))}

	isEmpty := func(obj *WowObject) bool {
		if obj.Model == nil || obj.Model.MDL == nil {
			return true
		}
		for _, geoset := range obj.Model.MDL.Geosets {
			if len(geoset.Vertices) > 0 {
				return false
			}
		}
		return true
	}

	for _, obj := range roots {
		nodes := []*WowObject{obj}
		basePos := math.Vector3{}
		baseRot := math.EulerRotation{}
		if isEmpty(obj) {
			nodes = obj.Children
			basePos = obj.Position
			baseRot = obj.Rotation
		}
		for _, node := range nodes {
			position := math.V3Sum(basePos, math.V3Rotate(node.Position, baseRot))
			rotation := math.CalculateChildAbsoluteEulerRotation(baseRot, node.Rotation)
			if node.Model == nil || node.Model.MDL == nil {
				continue
			}
			for _, geoset := range node.Model.MDL.Geosets {
				for _, v := range geoset.Vertices {
					rotated := math.V3Rotate(v.Position, rotation)
					positionV := math.V3Sum(position, rotated)
					ext.Min = math.V3Min(ext.Min, positionV)
					ext.Max = math.V3Max(ext.Max, positionV)
				}
			}
		}
	}
	return ext
}

package directwmo

import (
	"context"
	"fmt"
	"image"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/assemble"
	bundleanim "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	bundlemeta "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/metadata"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/mtl"
	objpkg "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/obj"
	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

// ConvertOptions configures WMO -> MDL conversion.
type ConvertOptions struct {
	FileDataID         int
	FileName           string
	Raw                []byte
	ExportPathOverride string
}

type textureMapEntry struct {
	matPathRelative string
	matPath         string
	matName         string
}

func virtualExportPath(exportRoot, file string) string {
	return filepath.Clean(filepath.Join(exportRoot, strings.ReplaceAll(file, " ", "")))
}

// wmoDiffuseFileID is the texture file data ID bound as the WC3 diffuse.
// Shader 23 ignores texture1 only when that slot is filled; an empty texture1
// must not also skip texture2.
func wmoDiffuseFileID(material wmo.Material) uint32 {
	skipTexture1 := material.Shader == 23 && material.Texture1 != 0
	for _, id := range exportTextureSlots(material) {
		if id == 0 {
			continue
		}
		if skipTexture1 {
			skipTexture1 = false
			continue
		}
		return id
	}
	return 0
}

func exportTextureSlots(material wmo.Material) []uint32 {
	slots := []uint32{material.Texture1, material.Texture2, material.Texture3}
	if material.Shader == 22 || material.Shader == 23 {
		slots = append(slots, material.Flags3, material.Color3)
		count := 1
		if material.Shader == 23 {
			count = 4
		}
		slots = append(slots, material.RuntimeData[:min(count, len(material.RuntimeData))]...)
	}
	return slots
}

func metaTextureSlots(material wmo.Material) []uint32 {
	slots := []uint32{material.Texture1, material.Texture2, material.Texture3}
	if material.Shader == 22 || material.Shader == 23 {
		slots = append(slots, material.Color3, material.Flags3)
		count := 1
		if material.Shader == 23 {
			count = 4
		}
		slots = append(slots, material.RuntimeData[:min(count, len(material.RuntimeData))]...)
	}
	return slots
}

func resolveWmoTextures(
	ctx context.Context,
	root *wmo.Loader,
	outDir, exportRoot string,
	getRaw func(context.Context, int) ([]byte, error),
	getName func(context.Context, int) (string, error),
) (map[int]textureMapEntry, map[int]string, []mtl.Material, error) {
	textureMap := map[int]textureMapEntry{}
	materialMap := map[int]string{}
	var mtlMaterials []mtl.Material

	isClassic := root.TextureNames != nil
	materials := root.Materials

	for i, material := range materials {
		// Shader 23 keeps the albedo in texture2. texture1 is often 0; skipping the
		// first non-zero slot then binds texture3 (a mask) and the surface goes magenta or black.
		diffuseID := wmoDiffuseFileID(material)
		for _, materialTexture := range exportTextureSlots(material) {
			if materialTexture == 0 {
				continue
			}
			var fileDataID int
			var fileName string
			if isClassic {
				fileName = root.TextureNames[int(materialTexture)]
				if id, ok := archivecasc.GetByFilename(fileName); ok {
					fileDataID = id
				}
				fileName = strings.ReplaceAll(fileName, " ", "")
			} else {
				fileDataID = int(materialTexture)
			}
			if fileDataID == 0 {
				continue
			}
			texFile := fmt.Sprintf("%d.png", fileDataID)
			texPath := filepath.Join(outDir, texFile)
			matName := fmt.Sprintf("mat_%d", fileDataID)

			if fileName == "" {
				if n, err := getName(ctx, fileDataID); err == nil {
					fileName = n
				}
			}
			if fileName != "" {
				matName = "mat_" + strings.TrimSuffix(strings.ToLower(filepath.Base(fileName)), ".blp")
				matName = strings.ReplaceAll(matName, " ", "")
			}

			if fileName != "" {
				fileName = writers.ReplaceExtension(fileName, ".png")
			} else {
				fileName = filepath.Join("unknown", texFile)
			}
			texPath = virtualExportPath(exportRoot, fileName)
			texFile = relPath(outDir, texPath)

			if _, err := getRaw(ctx, fileDataID); err != nil {
				log.Printf("Failed to resolve texture %d for WMO: %v", fileDataID, err)
				continue
			}
			relTex := relPath(exportRoot, texPath)
			opaque := material.BlendMode == 0
			if prev, ok := texturesource.Get(relTex); ok && !prev.Opaque {
				opaque = false
			}
			texturesource.Register(relTex, texturesource.Source{
				Kind: texturesource.KindBLP, FileDataID: fileDataID, Opaque: opaque,
			})

			mtlMaterials = append(mtlMaterials, mtl.Material{Name: matName, MapKd: texFile})
			textureMap[fileDataID] = textureMapEntry{matPathRelative: texFile, matPath: texPath, matName: matName}
			if _, ok := materialMap[i]; !ok && materialTexture == diffuseID {
				materialMap[i] = matName
			}
		}
	}
	return textureMap, materialMap, mtlMaterials, nil
}

func loadGroups(ctx context.Context, root *wmo.Loader, fileName string, getRaw func(context.Context, int) ([]byte, error)) ([]*wmo.Loader, error) {
	groups := make([]*wmo.Loader, root.GroupCount)
	for i := 0; i < int(root.GroupCount); i++ {
		var raw []byte
		var err error
		if root.GroupIDs != nil {
			raw, err = getRaw(ctx, int(root.GroupIDs[i]))
		} else {
			groupName := strings.Replace(fileName, ".wmo", fmt.Sprintf("_%03d.wmo", i), 1)
			id, ok := archivecasc.GetByFilename(groupName)
			if !ok || id == 0 {
				return nil, fmt.Errorf("unable to resolve WMO group file: %s", groupName)
			}
			raw, err = getRaw(ctx, id)
		}
		if err != nil {
			return nil, err
		}
		group := wmo.NewLoader(buffer.From(raw), 0, "", false)
		if err := group.Load(); err != nil {
			return nil, err
		}
		groups[i] = group
		root.Groups[i] = group
	}
	return groups, nil
}

func buildWmoObjResult(root *wmo.Loader, allGroups []*wmo.Loader, materialMap map[int]string, modelName, mtlLib string) objpkg.Result {
	type groupRef struct {
		group    *wmo.Loader
		indOfs   int
		indCount int
	}
	var groups []groupRef
	nInd := 0
	maxLayerCount := 0
	for _, group := range allGroups {
		if len(group.RenderBatches) == 0 {
			continue
		}
		indCount := len(group.Vertices) / 3
		nInd += indCount
		if len(group.UVs) > maxLayerCount {
			maxLayerCount = len(group.UVs)
		}
		groups = append(groups, groupRef{group: group, indCount: indCount})
	}

	vertsArray := make([]float64, nInd*3)
	normalsArray := make([]float64, nInd*3)
	uvArrays := make([][]float32, maxLayerCount)
	for i := range uvArrays {
		uvArrays[i] = make([]float32, nInd*2)
	}

	var meshes []directm2.ObjMesh
	indOfs := 0
	for _, ref := range groups {
		group := ref.group
		indCount := ref.indCount
		vertOfs := indOfs * 3
		for i, n := 0, len(group.Vertices); i < n; i++ {
			vertsArray[vertOfs+i] = float64(group.Vertices[i])
		}
		for i, n := 0, len(group.Normals); i < n; i++ {
			normalsArray[vertOfs+i] = float64(group.Normals[i])
		}
		uvsOfs := indOfs * 2
		uvCount := indCount * 2
		for layer := 0; layer < maxLayerCount; layer++ {
			var uv []float32
			if layer < len(group.UVs) {
				uv = group.UVs[layer]
			}
			for j := 0; j < uvCount; j++ {
				if uv != nil && j < len(uv) {
					uvArrays[layer][uvsOfs+j] = uv[j]
				}
			}
		}

		groupName := ""
		if root.GroupNames != nil && group.NameOfs != 0 {
			groupName = root.GroupNames[int(group.NameOfs)]
		}
		for bI, batch := range group.RenderBatches {
			indices := make([]int, batch.NumFaces)
			for i := 0; i < int(batch.NumFaces); i++ {
				indices[i] = int(group.Indices[batch.FirstFace+uint32(i)]) + indOfs
			}
			matID := int(batch.MaterialID)
			if batch.Flags&2 == 2 && len(batch.PossibleBox2) > 2 {
				matID = int(batch.PossibleBox2[2])
			}
			matName := materialMap[matID]
			meshes = append(meshes, directm2.ObjMesh{
				Name: groupName + fmt.Sprintf("%d", bI), Triangles: indices, MatName: matName,
			})
		}
		indOfs += indCount
	}

	if maxLayerCount > 2 {
		uvArrays = uvArrays[:2]
	}
	return directm2.BuildRawObjResult(vertsArray, normalsArray, uvArrays, meshes, modelName, mtlLib)
}

func buildWmoMetadata(root *wmo.Loader, textureMap map[int]textureMapEntry) bundlemeta.Data {
	var textures []bundlemeta.Texture
	textureCache := map[int]struct{}{}
	for _, material := range root.Materials {
		for _, materialTexture := range metaTextureSlots(material) {
			texID := int(materialTexture)
			if texID == 0 {
				continue
			}
			if _, found := textureCache[texID]; found {
				continue
			}
			textureCache[texID] = struct{}{}
			entry := textureMap[texID]
			textures = append(textures, bundlemeta.Texture{FileDataID: texID, FileNameExternal: entry.matPathRelative, MtlName: entry.matName})
		}
	}
	return bundlemeta.Data{FileType: "wmo", Textures: textures, WMOMaterials: root.Materials}
}

// ConvertWmoToMdl converts a WMO root file to MDL via the direct pipeline.
func ConvertWmoToMdl(ctx context.Context, cfg config.Config, src directm2.FileSource, opts ConvertOptions) (directm2.ConvertResult, error) {
	exportRoot := cfg.ExportAssetDir
	raw := opts.Raw
	var err error
	if raw == nil {
		raw, err = src.GetRawFile(ctx, opts.FileDataID)
		if err != nil {
			return directm2.ConvertResult{}, err
		}
	}
	listfileName := opts.FileName
	if listfileName == "" {
		listfileName, _ = src.GetFileName(ctx, opts.FileDataID)
	}
	fileName := listfileName
	if fileName == "" {
		fileName = fmt.Sprintf("unknown/%d.wmo", opts.FileDataID)
	}

	exportPath := opts.ExportPathOverride
	if exportPath == "" {
		exportPath = virtualExportPath(exportRoot, fileName)
	}
	outDir := filepath.Dir(exportPath)

	root := wmo.NewLoader(buffer.From(raw), opts.FileDataID, fileName, false)
	if err := root.Load(); err != nil {
		return directm2.ConvertResult{}, err
	}

	getRaw := func(c context.Context, id int) ([]byte, error) { return src.GetRawFile(c, id) }
	getName := func(c context.Context, id int) (string, error) { return src.GetFileName(c, id) }

	allGroups, err := loadGroups(ctx, root, fileName, getRaw)
	if err != nil {
		return directm2.ConvertResult{}, err
	}

	textureMap, materialMap, mtlMaterials, err := resolveWmoTextures(ctx, root, outDir, exportRoot, getRaw, getName)
	if err != nil {
		return directm2.ConvertResult{}, err
	}

	animFile := bundleanim.NewFile(writers.ReplaceExtension(exportPath, "_bones.json"), cfg)
	metaObj := buildWmoMetadata(root, textureMap)
	meta := bundlemeta.NewFile(writers.ReplaceExtension(exportPath, ".json"), cfg, animFile)
	meta.LoadFromData(metaObj)

	var mtlLib string
	if len(mtlMaterials) > 0 {
		mtlLib = filepath.Base(writers.ReplaceExtension(exportPath, ".mtl"))
	}
	modelName := strings.TrimSuffix(filepath.Base(exportPath), filepath.Ext(exportPath))
	objResult := buildWmoObjResult(root, allGroups, materialMap, modelName, mtlLib)

	assembled := assemble.AssembleWowModel(assemble.Inputs{
		ObjFilePath: exportPath,
		Obj:         objResult,
		Mtl:         struct{ Materials []mtl.Material }{Materials: mtlMaterials},
		Animation:   animFile,
		Metadata:    meta,
	}, cfg)

	result := directm2.ConvertResult{MDL: assembled.MDL, TexturePaths: assembled.TexturePaths, BakeStem: directm2.BakeStemFromListfile(fileName)}
	if cfg.TextureBaking.Enabled {
		if err := bakeWmoMaterials(ctx, cfg, src, root, allGroups, &result); err != nil {
			return directm2.ConvertResult{}, err
		}
	}
	return result, nil
}

func bakeWmoMaterials(ctx context.Context, cfg config.Config, src directm2.FileSource, root *wmo.Loader, groups []*wmo.Loader, result *directm2.ConvertResult) error {
	return bakeWmoMaterialsWithWorkers(ctx, cfg, src, root, groups, result, 2)
}

type wmoBakeTask struct {
	geosetIndex       int
	beforeGeosetCount int
	additionalGeosets int
	baseTextureCount  int
	baseMaterialCount int
	baseAnimCount     int
	baseGlobalCount   int
	g                 *components.Geoset
	group             *wmo.Loader
	batch             wmo.RenderBatch
	material          wmo.Material
	images            [9]*image.NRGBA
	speed             [4]float32
}

type wmoBakeTaskResult struct {
	result directm2.ConvertResult
	err    error
}

// bakeWmoMaterialsWithWorkers keeps texture resolution serial, then bakes
// independent render batches against private result graphs. Results are
// applied in source order so IDs, texture paths, and emitted geometry remain
// deterministic. workerLimit exists for equivalence and race tests.
func bakeWmoMaterialsWithWorkers(ctx context.Context, cfg config.Config, src directm2.FileSource, root *wmo.Loader, groups []*wmo.Loader, result *directm2.ConvertResult, workerLimit int) error {
	decoded := map[int]*image.NRGBA{}
	originals := append(result.MDL.Geosets[:0:0], result.MDL.Geosets...)
	tasks := make([]wmoBakeTask, 0, len(originals))
	gi := 0
	for _, group := range groups {
		for _, batch := range group.RenderBatches {
			if batch.NumFaces < 3 {
				continue
			}
			if gi >= len(originals) {
				return fmt.Errorf("WMO batch geometry is missing")
			}
			g := originals[gi]
			gi++
			matID := int(batch.MaterialID)
			if batch.Flags&2 != 0 && len(batch.PossibleBox2) > 2 {
				matID = int(batch.PossibleBox2[2])
			}
			if matID >= len(root.Materials) {
				return fmt.Errorf("WMO material %d out of range", matID)
			}
			material := root.Materials[matID]
			count, err := directm2.WMOShaderSamplerCount(material.Shader)
			if err != nil {
				return err
			}
			if g.Material == nil || len(g.Material.Layers) == 0 {
				continue
			}
			var speed [4]float32
			if matID < len(root.MaterialUVSpeed) {
				speed = root.MaterialUVSpeed[matID]
			}
			if count == 1 && len(group.VertexColours) == 0 && speed == [4]float32{} && material.BlendMode <= 3 {
				g.Material.Layers[0].Unshaded = material.Flags&1 != 0
				continue
			}
			var images [9]*image.NRGBA
			slots := metaTextureSlots(material)
			for sampler := range count {
				if sampler >= len(slots) || slots[sampler] == 0 {
					continue
				}
				id := int(slots[sampler])
				if root.TextureNames != nil {
					var found bool
					id, found = archivecasc.GetByFilename(root.TextureNames[id])
					if !found {
						continue
					}
				}
				img := decoded[id]
				if img == nil {
					raw, err := src.GetRawFile(ctx, id)
					if err != nil {
						return fmt.Errorf("WMO texture %d: %w", id, err)
					}
					texture, err := blp.NewBLPImage(buffer.From(raw))
					if err != nil {
						return err
					}
					pixels, err := texture.ToUInt8Array(0, 15)
					if err != nil {
						return err
					}
					img = image.NewNRGBA(image.Rect(0, 0, int(texture.Width), int(texture.Height)))
					copy(img.Pix, pixels)
					decoded[id] = img
				}
				images[sampler] = img
			}
			additional, err := directm2.WMOAdditionalGeosetCount(cfg, material.Shader, material.BlendMode, images[0] != nil)
			if err != nil {
				return err
			}
			tasks = append(tasks, wmoBakeTask{
				geosetIndex:       gi - 1,
				beforeGeosetCount: len(originals) + totalAdditionalGeosets(tasks),
				additionalGeosets: additional,
				baseTextureCount:  len(result.MDL.Textures),
				baseMaterialCount: len(result.MDL.Materials),
				baseAnimCount:     len(result.MDL.TextureAnims),
				baseGlobalCount:   len(result.MDL.GlobalSequences),
				g:                 g,
				group:             group,
				batch:             batch,
				material:          material,
				images:            images,
				speed:             speed,
			})
		}
	}
	if len(tasks) < 4 || workerLimit < 2 {
		for _, task := range tasks {
			if err := directm2.BakeWMOMaterial(ctx, cfg, result, task.g, task.group, task.batch, task.material, task.images, task.speed); err != nil {
				return err
			}
		}
		result.MDL.Sync()
		return nil
	}
	if workerLimit > 2 {
		workerLimit = 2
	}
	results := make([]wmoBakeTaskResult, len(tasks))
	baseMaterialCount := len(result.MDL.Materials)
	var next int
	var nextMu sync.Mutex
	var errorMu sync.Mutex
	var firstErr error
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	var workers sync.WaitGroup
	for range min(workerLimit, len(tasks)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				if workerCtx.Err() != nil {
					return
				}
				nextMu.Lock()
				index := next
				next++
				nextMu.Unlock()
				if index >= len(tasks) {
					return
				}
				task := tasks[index]
				workerResult := cloneWmoBakeResult(*result, task.geosetIndex, task.g, task.beforeGeosetCount)
				err := directm2.BakeWMOMaterial(workerCtx, cfg, &workerResult, workerResult.MDL.Geosets[task.geosetIndex], task.group, task.batch, task.material, task.images, task.speed)
				results[index] = wmoBakeTaskResult{result: workerResult, err: err}
				if err != nil {
					errorMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errorMu.Unlock()
					cancelWorkers()
					return
				}
			}
		}()
	}
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	if firstErr != nil {
		return firstErr
	}
	for i := range results {
		if got, want := len(results[i].result.MDL.Geosets), tasks[i].beforeGeosetCount+tasks[i].additionalGeosets; got != want {
			return fmt.Errorf("WMO batch %d produced %d geosets after preflight expected %d", i, got-tasks[i].beforeGeosetCount, tasks[i].additionalGeosets)
		}
	}
	for i, task := range tasks {
		if err := mergeWmoBakeResult(result, results[i].result, task); err != nil {
			return err
		}
	}
	rebaseWmoTextureAnimations(result.MDL, baseMaterialCount)
	result.MDL.Sync()
	return nil
}

func totalAdditionalGeosets(tasks []wmoBakeTask) int {
	total := 0
	for _, task := range tasks {
		total += task.additionalGeosets
	}
	return total
}

func cloneWmoBakeResult(base directm2.ConvertResult, geosetIndex int, geoset *components.Geoset, geosetCount int) directm2.ConvertResult {
	model := *base.MDL
	model.Geosets = append([]*components.Geoset(nil), base.MDL.Geosets...)
	model.Geosets[geosetIndex] = cloneWmoBakeGeoset(geoset)
	if geosetCount > len(model.Geosets) {
		model.Geosets = append(model.Geosets, make([]*components.Geoset, geosetCount-len(model.Geosets))...)
	}
	model.Geosets = model.Geosets[:len(model.Geosets):len(model.Geosets)]
	model.Textures = append([]*components.Texture(nil), base.MDL.Textures...)
	model.Textures = model.Textures[:len(model.Textures):len(model.Textures)]
	model.Materials = append([]*components.Material(nil), base.MDL.Materials...)
	model.Materials = model.Materials[:len(model.Materials):len(model.Materials)]
	model.TextureAnims = append([]components.TextureAnim(nil), base.MDL.TextureAnims...)
	model.TextureAnims = model.TextureAnims[:len(model.TextureAnims):len(model.TextureAnims)]
	model.GlobalSequences = append([]*components.GlobalSequence(nil), base.MDL.GlobalSequences...)
	model.GlobalSequences = model.GlobalSequences[:len(model.GlobalSequences):len(model.GlobalSequences)]
	// Match the serial model's virtual number of already emitted geosets for
	// bakeM2Geoset's shared per-model texture budget.
	model.Geosets = model.Geosets[:geosetCount]
	texturePaths := make(map[string]struct{}, len(base.TexturePaths))
	for path := range base.TexturePaths {
		texturePaths[path] = struct{}{}
	}
	return directm2.ConvertResult{MDL: &model, TexturePaths: texturePaths, BakeStem: base.BakeStem}
}

func cloneWmoBakeGeoset(source *components.Geoset) *components.Geoset {
	copyG := *source
	copyG.Vertices = nil
	copyG.Faces = append([]components.Face(nil), source.Faces...)
	vertices := make(map[*components.GeosetVertex]*components.GeosetVertex, len(source.Vertices))
	for _, vertex := range source.Vertices {
		cloned := *vertex
		vertices[vertex] = &cloned
		copyG.Vertices = append(copyG.Vertices, &cloned)
	}
	for fi := range copyG.Faces {
		for vi, vertex := range copyG.Faces[fi].Vertices {
			copyG.Faces[fi].Vertices[vi] = vertices[vertex]
		}
	}
	if source.Material != nil {
		material := *source.Material
		material.Layers = append([]components.Layer(nil), source.Material.Layers...)
		copyG.Material = &material
	}
	return &copyG
}

func mergeWmoBakeResult(destination *directm2.ConvertResult, worker directm2.ConvertResult, task wmoBakeTask) error {
	base := task.beforeGeosetCount
	if len(destination.MDL.Geosets) != base {
		return fmt.Errorf("WMO batch merge order mismatch: model has %d geosets, want %d", len(destination.MDL.Geosets), base)
	}
	originalGeoset := destination.MDL.Geosets[task.geosetIndex]
	animationRefs := collectWmoWorkerTextureAnimRefs(worker, task)
	destination.MDL.Geosets[task.geosetIndex] = worker.MDL.Geosets[task.geosetIndex]
	destination.MDL.Geosets = append(destination.MDL.Geosets, worker.MDL.Geosets[base:]...)
	for i := range destination.MDL.GeosetAnims {
		if destination.MDL.GeosetAnims[i].Geoset == originalGeoset {
			destination.MDL.GeosetAnims[i].Geoset = destination.MDL.Geosets[task.geosetIndex]
		}
	}
	destination.MDL.Textures = append(destination.MDL.Textures, worker.MDL.Textures[task.baseTextureCount:]...)
	destination.MDL.Materials = append(destination.MDL.Materials, worker.MDL.Materials[task.baseMaterialCount:]...)
	baseAnimCount, baseGlobalCount := task.baseAnimCount, task.baseGlobalCount
	animOffset := len(destination.MDL.TextureAnims)
	newAnims := worker.MDL.TextureAnims[baseAnimCount:]
	for i := range newAnims {
		newAnims[i].ID = animOffset + i
	}
	destination.MDL.TextureAnims = append(destination.MDL.TextureAnims, newAnims...)
	for animation, localID := range animationRefs {
		if localID >= baseAnimCount && localID < len(worker.MDL.TextureAnims) {
			animation.ID = animOffset + localID - baseAnimCount
		}
	}
	for _, sequence := range worker.MDL.GlobalSequences[baseGlobalCount:] {
		sequence.ID = len(destination.MDL.GlobalSequences)
		if sequence.HasRawID {
			sequence.RawID = sequence.ID
		}
		destination.MDL.GlobalSequences = append(destination.MDL.GlobalSequences, sequence)
	}
	for path := range worker.TexturePaths {
		destination.TexturePaths[path] = struct{}{}
	}
	return nil
}

func collectWmoWorkerTextureAnimRefs(worker directm2.ConvertResult, task wmoBakeTask) map[*components.TextureAnim]int {
	refs := make(map[*components.TextureAnim]int)
	collectMaterial := func(material *components.Material) {
		if material == nil {
			return
		}
		for li := range material.Layers {
			animation := material.Layers[li].TVertexAnim
			if animation != nil {
				if _, seen := refs[animation]; !seen {
					refs[animation] = animation.ID
				}
			}
		}
	}
	for _, material := range worker.MDL.Materials[task.baseMaterialCount:] {
		collectMaterial(material)
	}
	if task.geosetIndex >= 0 && task.geosetIndex < len(worker.MDL.Geosets) {
		collectMaterial(worker.MDL.Geosets[task.geosetIndex].Material)
	}
	for _, geoset := range worker.MDL.Geosets[task.beforeGeosetCount:] {
		if geoset != nil {
			collectMaterial(geoset.Material)
		}
	}
	return refs
}

func rebaseWmoTextureAnimations(model *mdl.MDL, firstGeneratedMaterial int) {
	for _, material := range model.Materials[firstGeneratedMaterial:] {
		if material == nil {
			continue
		}
		for li := range material.Layers {
			layer := &material.Layers[li]
			if layer.TVertexAnim != nil && layer.TVertexAnim.ID >= 0 && layer.TVertexAnim.ID < len(model.TextureAnims) {
				layer.TVertexAnim = &model.TextureAnims[layer.TVertexAnim.ID]
			}
		}
	}
}

func relPath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

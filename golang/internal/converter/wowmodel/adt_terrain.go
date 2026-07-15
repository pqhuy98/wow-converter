package wowmodel

import (
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/converter/convertlog"
	bundleanim "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	bundlemeta "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/metadata"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/mtl"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/obj"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/assemble"
	"github.com/pqhuy98/wow-converter/internal/config"
)

// ConvertAdtTerrainContentToMdl assembles MDL from in-memory OBJ/MTL text.
func ConvertAdtTerrainContentToMdl(objFilePath, objText, mtlText string, cfg config.Config) (ConvertResult, error) {
	convertlog.Loading(cfg, objFilePath)
	objRes := obj.Parse(objText)
	mtlMaterials := mtl.Parse(mtlText)

	animPath := strings.TrimSuffix(objFilePath, filepath.Ext(objFilePath)) + "_bones.json"
	animFile := bundleanim.NewFile(animPath, cfg)
	_ = animFile.Parse()

	metaPath := strings.TrimSuffix(objFilePath, filepath.Ext(objFilePath)) + ".json"
	metaFile := bundlemeta.NewFile(metaPath, cfg, animFile)
	_ = metaFile.Parse()

	inputs := assemble.Inputs{
		ObjFilePath: objFilePath,
		Obj:         objRes,
		Animation:   animFile,
		Metadata:    metaFile,
	}
	inputs.Mtl.Materials = mtlMaterials

	result := assemble.AssembleWowModel(inputs, cfg)
	textures := make([]string, 0, len(result.TexturePaths))
	for p := range result.TexturePaths {
		textures = append(textures, p)
	}
	return ConvertResult{MDL: result.MDL, TexturePaths: textures}, nil
}

// ConvertAdtTerrainObjToMdl parses ADT terrain OBJ/MTL and assembles MDL.
func ConvertAdtTerrainObjToMdl(objFilePath string, cfg config.Config) (ConvertResult, error) {
	convertlog.Loading(cfg, objFilePath)
	objRes, err := obj.ParseFile(objFilePath)
	if err != nil {
		return ConvertResult{}, err
	}

	mtlPath := strings.TrimSuffix(objFilePath, filepath.Ext(objFilePath)) + ".mtl"
	convertlog.Loading(cfg, mtlPath)
	mtlMaterials, _ := mtl.ParseFile(mtlPath)

	animPath := strings.TrimSuffix(objFilePath, filepath.Ext(objFilePath)) + "_bones.json"
	animFile := bundleanim.NewFile(animPath, cfg)
	_ = animFile.Parse()

	metaPath := strings.TrimSuffix(objFilePath, filepath.Ext(objFilePath)) + ".json"
	metaFile := bundlemeta.NewFile(metaPath, cfg, animFile)
	_ = metaFile.Parse()

	inputs := assemble.Inputs{
		ObjFilePath: objFilePath,
		Obj:         objRes,
		Animation:   animFile,
		Metadata:    metaFile,
	}
	inputs.Mtl.Materials = mtlMaterials

	result := assemble.AssembleWowModel(inputs, cfg)
	textures := make([]string, 0, len(result.TexturePaths))
	for p := range result.TexturePaths {
		textures = append(textures, p)
	}
	return ConvertResult{MDL: result.MDL, TexturePaths: textures}, nil
}

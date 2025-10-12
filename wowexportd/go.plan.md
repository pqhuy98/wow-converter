### Project Goal

Build a pure Go server (wowexportd) that reimplements all non‑UI logic from wow.export with wire‑compatible REST APIs and identical outputs, supporting both Retail and Classic. The Go implementation must significantly improve performance and scalability via parallel I/O/compute and robust caching, while remaining fully compatible with existing wow.export tooling and clients (request/response shapes, IDs, status codes, and export directory layout unchanged).

### wowexportd Go plan (DDD)

Matches `wow.export` structure and file names for direct cross-reference.

### Packages (Go file → wow.export path)
- `internal/core/`
  - `core.go` → `wow.export/src/js/core.js`
  - `config.go` → `wow.export/src/js/config.js`
  - `log.go` → `wow.export/src/js/log.js`
  - `generics.go` → `wow.export/src/js/generics.js`
- `internal/casc/`
  - `encoding_root.go` → parses encoding and root (logs match JS)
  - `casc-source.go` → `wow.export/src/js/casc/casc-source.js`
  - `casc-source-local.go` → `wow.export/src/js/casc/casc-source-local.js`
  - `casc-source-remote.go` → `wow.export/src/js/casc/casc-source-remote.js`
  - `blte-reader.go` → `wow.export/src/js/casc/blte-reader.js`
  - `cdn-resolver.go` → `wow.export/src/js/casc/cdn-resolver.js`
  - `cdn-config.go` → `wow.export/src/js/casc/cdn-config.js`
  - `version-config.go` → `wow.export/src/js/casc/version-config.js`
  - `install-manifest.go` → `wow.export/src/js/casc/install-manifest.js`
  - `build-cache.go` → `wow.export/src/js/casc/build-cache.js`
  - `content-flags.go` → `wow.export/src/js/casc/content-flags.js`
  - `locale-flags.go` → `wow.export/src/js/casc/locale-flags.js`
  - `realmlist.go` → `wow.export/src/js/casc/realmlist.js`
  - `salsa20.go` → `wow.export/src/js/casc/salsa20.js`
  - `tact-keys.go` → `wow.export/src/js/casc/tact-keys.js`
  - `jenkins96.go` → `wow.export/src/js/casc/jenkins96.js`
  - `listfile.go` → `wow.export/src/js/casc/listfile.js`
  - `export-helper.go` → `wow.export/src/js/casc/export-helper.js`
- `internal/db/`
  - `WDCReader.go` → `wow.export/src/js/db/WDCReader.js`
  - `CompressionType.go` → `wow.export/src/js/db/CompressionType.js`
  - `FieldType.go` → `wow.export/src/js/db/FieldType.js`
  - `DBDParser.go` → `wow.export/src/js/db/DBDParser.js`
  - `caches/DBCreatures.go` → `wow.export/src/js/db/caches/DBCreatures.js`
  - `caches/DBItemDisplays.go` → `wow.export/src/js/db/caches/DBItemDisplays.js`
  - `caches/DBModelFileData.go` → `wow.export/src/js/db/caches/DBModelFileData.js`
  - `caches/DBTextureFileData.go` → `wow.export/src/js/db/caches/DBTextureFileData.js`
  - `caches/init-cache.go` → `wow.export/src/js/db/caches/init-cache.js`
- `internal/formats/`
  - `Texture.go` → `wow.export/src/js/3D/Texture.js`
  - `Skin.go` → `wow.export/src/js/3D/Skin.js`
  - `AnimMapper.go` → `wow.export/src/js/3D/AnimMapper.js`
  - `BoneMapper.go` → `wow.export/src/js/3D/BoneMapper.js`
  - `GeosetMapper.go` → `wow.export/src/js/3D/GeosetMapper.js`
  - `ShaderMapper.go` → `wow.export/src/js/3D/ShaderMapper.js`
- `internal/loaders/`
  - `ADTLoader.go` → `wow.export/src/js/3D/loaders/ADTLoader.js`
  - `ANIMLoader.go` → `wow.export/src/js/3D/loaders/ANIMLoader.js`
  - `BONELoader.go` → `wow.export/src/js/3D/loaders/BONELoader.js`
  - `LoaderGenerics.go` → `wow.export/src/js/3D/loaders/LoaderGenerics.js`
  - `M2Generics.go` → `wow.export/src/js/3D/loaders/M2Generics.js`
  - `M2Loader.go` → `wow.export/src/js/3D/loaders/M2Loader.js`
  - `M3Loader.go` → `wow.export/src/js/3D/loaders/M3Loader.js`
  - `SKELLoader.go` → `wow.export/src/js/3D/loaders/SKELLoader.js`
  - `WDTLoader.go` → `wow.export/src/js/3D/loaders/WDTLoader.js`
  - `WMOLoader.go` → `wow.export/src/js/3D/loaders/WMOLoader.js`
- `internal/exporters/`
  - `ADTExporter.go` → `wow.export/src/js/3D/exporters/ADTExporter.js`
  - `M2Exporter.go` → `wow.export/src/js/3D/exporters/M2Exporter.js`
  - `M3Exporter.go` → `wow.export/src/js/3D/exporters/M3Exporter.js`
  - `WMOExporter.go` → `wow.export/src/js/3D/exporters/WMOExporter.js`
- `internal/writers/`
  - `CSVWriter.go` → `wow.export/src/js/3D/writers/CSVWriter.js`
  - `GLTFWriter.go` → `wow.export/src/js/3D/writers/GLTFWriter.js`
  - `JSONWriter.go` → `wow.export/src/js/3D/writers/JSONWriter.js`
  - `MTLWriter.go` → `wow.export/src/js/3D/writers/MTLWriter.js`
  - `OBJWriter.go` → `wow.export/src/js/3D/writers/OBJWriter.js`
- `internal/renderers/`
  - `CharMaterialRenderer.go` → `wow.export/src/js/3D/renderers/CharMaterialRenderer.js`
  - `M2Renderer.go` → `wow.export/src/js/3D/renderers/M2Renderer.js`
  - `M3Renderer.go` → `wow.export/src/js/3D/renderers/M3Renderer.js`
  - `RenderCache.go` → `wow.export/src/js/3D/renderers/RenderCache.js`
  - `WMORenderer.go` → `wow.export/src/js/3D/renderers/WMORenderer.js`
- `internal/ui/`
  - `adt-exporter.go` → `wow.export/src/js/3D/utils/map-export-utils.js`
  - `model-exporter.go` → `wow.export/src/js/ui/tab-models.js`
  - `character-exporter.go` → `wow.export/src/js/ui/headless-character.js`
  - `texture-exporter.go` → `wow.export/src/js/ui/texture-exporter.js`
- `internal/server/`
  - `rest-server.go` → `wow.export/src/js/rest/rest-server.js`
  - `router.go` → `wow.export/src/js/rest/rest-server.js`
  - `json.go` → `wow.export/src/js/rest/rest-server.js`
  - `cache.go` → `wow.export/src/js/rest/rest-server.js`
  - `download.go` → `wow.export/src/js/rest/rest-server.js`
- `internal/resthandlers/`
  - per-endpoint files → `wow.export/src/js/rest/rest-server.js`
- `internal/fileio/`
  - `file-writer.go` → `wow.export/src/js/file-writer.js`
  - `png-writer.go` → `wow.export/src/js/png-writer.js`
  - `tiled-png-writer.go` → `wow.export/src/js/tiled-png-writer.js`
- `internal/cache/`
  - `response_cache.go` → `wow.export/src/js/rest/rest-server.js`
- `internal/httpx/`
  - `client.go` → n/a (uses Go http client)
- `internal/utils/`
  - `buffer.go` → `wow.export/src/js/buffer.js`
  - `constants.go` → `wow.export/src/js/constants.js`
  - `multimap.go` → `wow.export/src/js/MultiMap.js`
  - `crc32.go` → `wow.export/src/js/crc32.js`

### REST endpoints
- GET
  - `/rest/getCascInfo` → `resthandlers/get_casc_info.go`
  - `/rest/getConfig` → `resthandlers/get_config.go`
  - `/rest/searchFiles` → `resthandlers/search_files.go`
  - `/rest/getFileById` → `resthandlers/get_file_by_id.go`
  - `/rest/getFileByName` → `resthandlers/get_file_by_name.go`
  - `/rest/getModelSkins` → `resthandlers/get_model_skins.go`
  - `/rest/download` → `resthandlers/download.go`
  - `/rest/getMapList` → `resthandlers/get_map_list.go`
- POST
  - `/rest/loadCascLocal` → `resthandlers/load_casc_local.go`
  - `/rest/loadCascRemote` → `resthandlers/load_casc_remote.go`
  - `/rest/loadCascBuild` → `resthandlers/load_casc_build.go`
  - `/rest/setConfig` → `resthandlers/set_config.go`
  - `/rest/exportModels` → `resthandlers/export_models.go`
  - `/rest/exportTextures` → `resthandlers/export_textures.go`
  - `/rest/exportCharacter` → `resthandlers/export_character.go`
  - `/rest/exportADT` → `resthandlers/export_adt.go`

### Tree
```
wowexportd/
  cmd/wowexportd/main.go
  internal/
    server/
    resthandlers/
    core/
    casc/
    db/
    formats/
    loaders/
    exporters/
    writers/
    renderers/
    ui/
    fileio/
    cache/
    httpx/
    utils/
```

### Exclusions (GUI-only in wow.export)
- `wow/ItemSlot.js` → not ported.
- `icon-render.js` → not ported.



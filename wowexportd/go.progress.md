## Progress Summary

Date: [auto]

### Implemented
- Server scaffolding
  - cmd/wowexportd/main.go (port 17753, env override WOWEXPORT_REST_PORT)
  - internal/server
    - rest-server.go (http server, timeouts, logging middleware)
    - router.go (registered REST endpoints)
    - middleware.go (request logging)
    - config.go (in-memory config, defaults including exportDirectory and listfile URLs)

- REST endpoints
  - GET: /rest/getConfig, /rest/download, /rest/getCascInfo, /rest/searchFiles, /rest/getFileById, /rest/getFileByName
  - POST: /rest/setConfig, /rest/loadCascLocal, /rest/loadCascRemote, /rest/loadCascBuild
  - JSON shapes and status codes mirror wow.export
  - getCascInfo parity: returns full selected build object and buildConfig map to match wow.export

- Listfile
  - internal/casc/listfile_store.go (ID↔name maps, regex/substring filtering)
  - Router bootstrap downloads community listfile if cache missing and loads it

- CASC foundations (pure Go, real logic)
  - Hash/crypto: jenkins96.go, salsa20.go
  - BLTE parsing/decrypt: blte_reader.go (MD5 verification, Salsa20 decrypt, zlib compression)
  - HTTP utilities: httpx.go (GET and ranged GET)
  - CDN/version parsing: cdn_config.go, version_config.go, cdn_resolver.go
  - Constants: constants.go (paths, patch endpoints, cache layout)
  - TACT keys store: tact_keys.go (API hooks; external load TODO)
  - Build cache: build_cache.go (per-build dir, sha1 integrity, manifest lastAccess)
- CASC sources:
  - Local: casc_source_local.go (parse .build.info, journal indexes; load BuildConfig with CDN fallback; load encoding+root; cache BUILD_ENCODING/BUILD_ROOT; BLTE read with fallback to CDN)
  - Remote: casc_source_remote.go (discover versions, resolve best CDN host, fetch CDN+Build configs; parallel archive index load (50 workers); cache indices; partial+direct fetch helpers)

- Active build tracking
  - Local/Remote now track selected build index; GetBuildKey/GetBuildName reflect the active build
  - /rest/getCascInfo uses the active build and parsed buildConfig for wire-compat

- Logging
  - Verbose logs mirroring wow.export across CASC init/load (indexes, configs, encoding, root, archives)

 - TACT keys
  - internal/casc/tact_keys.go (lazy load from disk JSON/text at PathTACTKeys or TACT_KEYS_PATH; optional fetch from TACT_KEYS_URL with cache; case-normalized lookups; supports 16/32-byte keys; ReloadTACTKeys provided for runtime refresh)
  - BLTE decryption uses GetTACTKey; encrypted blocks now decrypt when key is available

- CDN resilience
  - Remote ranks CDN hosts by ping and cycles fallbacks (getWithFallback/getRangeWithFallback)
  - Applied to archive indices, encoding/file downloads; resilient to host failures

- Listfile filtering
  - After /rest/loadCascBuild activation, applies root-filtered listfile (ApplyRootFilter) to mirror listfile.applyPreload behavior

- Listfile unknown ingestion (partial)
  - internal/casc/listfile.go: LoadIDTable, LoadUnknownTextures, LoadUnknownModels
  - internal/server/router.go: placeholder comment for wiring real DB readers; no stubs retained

- Texture export (PNG parity)
  - internal/fileio/png_writer.go (adaptive filters, zlib deflate, RGBA 8-bit)
  - internal/formats/blp.go (BLP type 1: palette, DXT1/3/5, BGRA)
  - internal/formats/texture.go (ExportTexturePNG from CASC via EKey)
  - internal/resthandlers/export_textures.go (/rest/exportTextures manifest + layout)
  - router: wired POST /rest/exportTextures

### Pending / Next
1) Listfile integration polish: replace stubbed DB cache getters with real readers and preload unknowns (improves name↔ID lookups for the name-based endpoint).
2) Response caching: wire ResponseCache (10s TTL) for heavy REST responses and future export endpoints.
3) Export endpoints (after textures): `/rest/exportModels`, `/rest/exportCharacter`, `/rest/exportADT`.
4) Map list endpoint: `/rest/getMapList` via DB2 reader, filtered by listfile presence.

### Implementation parity requirements
- The Go implementation must exactly match wow.export JS: algorithms, class/function interfaces, naming, logging text, and outputs.
- Mirror file/module structure and variable names where practical; maintain wire API shapes and status codes.

### Notes
- Wire-compat maintained for implemented endpoints (IDs, status codes)
- /rest/getCascInfo now mirrors wow.export shape (build + buildConfig)
- Listfile default URLs added; server auto-fetches at startup if cache missing
- All code is pure Go; no mocks used in implemented parts

 - Process requirement
  - After each implemented change, immediately update go.progress.md with a brief entry and adjust Pending/Next.
  - Never leave stubbed code in the tree; always implement real functionality before marking an item done. If temporary placeholders are unavoidable, remove them in the same session and document the replacement.


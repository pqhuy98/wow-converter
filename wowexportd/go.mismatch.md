### wowexportd vs wow.export mismatches (current vs expected)

Scope: Implemented Go files as of current progress. For each item: Current (Go) vs Expected (JS wow.export). Paths use `wowexportd/...` and `wow.export/src/js/...`.

## REST API and routing

- Missing endpoint: GET `/rest/getModelSkins`
  - Current (Go): Not registered in `wowexportd/internal/server/router.go`.
  - Expected (JS): Implemented in `wow.export/src/js/rest/rest-server.js` and returns `{ id: 'MODEL_SKINS', fileDataID, skins }`.

- Missing endpoint: GET `/rest/getMapList`
  - Current (Go): Not registered yet in `router.go`.
  - Expected (JS): Implemented in `rest-server.js` via DB2 read of `Map.db2`, filtered by listfile.

- Server port default
  - Current (Go): Default advertised as 17753 (`wowexportd/internal/server/rest-server.go` via `New()`; main sets env WOWEXPORT_REST_PORT per progress notes).
  - Expected (JS): 17752 by default (`wow.export/src/js/rest/rest-server.js`). Not wire shape, but worth noting for compatibility.

## Handlers: CASC info and lifecycle

- GET `/rest/getCascInfo` "type" field
  - Current (Go): Returns `type: "GoCASC"` in `wowexportd/internal/resthandlers/casc.go`.
  - Expected (JS): Returns constructor name of the CASC source (`CASCLocal` or `CASCRemote`) in `rest-server.js`.

- GET `/rest/getCascInfo` caching
  - Current (Go): Response cached for 10s using `ResponseCache` (`wowexportd/internal/server/cache.go`), keyed by buildKey.
  - Expected (JS): No caching applied for this endpoint.

- POST `/rest/loadCascLocal` and `/rest/loadCascRemote`: active CASC guard
  - Current (Go): `LoadCascLocal`/`LoadCascRemote` in `wowexportd/internal/resthandlers/casc.go` do not reject when an active CASC is already set.
  - Expected (JS): If `core.view.casc` exists, respond `409 { id: 'ERR_CASC_ACTIVE' }` (`rest-server.js`).

- POST `/rest/loadCascBuild`: side-effects and listfile application
  - Current (Go): Activates pending source and calls `list.ApplyRootFilter()` with `GetValidRootEntries()`; returns `getCascInfo` (`resthandlers/casc.go`).
  - Expected (JS): Applies preloaded listfile via `listfile.applyPreload(rootEntries)` and clears some caches (characters, ADT exporter) before returning `getCascInfo` (`rest-server.js`). UI/cache side-effects are not strictly required server-side, but listfile application should mirror `applyPreload` semantics.

- JSON body error handling (all POST handlers)
  - Current (Go): Invalid/malformed JSON generally returns `400 { id: 'ERR_INVALID_PARAMETERS', required: ... }` (e.g., `resthandlers/config.go`, `resthandlers/casc.go`).
  - Expected (JS): Malformed JSON triggers an exception in `readJSONBody`, which returns `500 { id: 'ERR_INTERNAL', message: 'ERR_INVALID_JSON' }` (`rest-server.js`).

## Handlers: Listfile endpoints

- GET `/rest/getFileById`: listfile loaded guard and parameter semantics
  - Current (Go): No `list.IsLoaded()` check; treats `fileDataID == 0` as invalid and returns 400 (`resthandlers/casc.go`).
  - Expected (JS): Returns `409 { id: 'ERR_LISTFILE_NOT_LOADED' }` if listfile not loaded; accepts any finite number (including 0), returning `LISTFILE_RESULT` with empty `fileName` when unknown (`rest-server.js`).

- GET `/rest/getFileByName`: listfile loaded guard
  - Current (Go): No `list.IsLoaded()` check; returns `LISTFILE_RESULT` regardless (`resthandlers/casc.go`).
  - Expected (JS): Returns `409 { id: 'ERR_LISTFILE_NOT_LOADED' }` if listfile not loaded (`rest-server.js`).

- GET `/rest/searchFiles`: response caching and ordering
  - Current (Go): Caches responses for 10s (`resthandlers/listfile.go`). Results are sorted by `fileDataID` ascending in `internal/casc/listfile.go`.
  - Expected (JS): No caching for this endpoint; result ordering follows insertion order of `Map` iteration (not guaranteed by ID sort) in `casc/listfile.js`.

## Handler: Download

- GET `/rest/download`: stream error handling
  - Current (Go): Uses `http.ServeFile`; on read errors, response may not be converted to JSON error (`resthandlers/download.go`).
  - Expected (JS): On stream error, responds `500 { id: 'ERR_INTERNAL', message: 'Failed to read file' }` (`rest-server.js`).

## Listfile implementation

- MDL/MDX fallback in filename lookup
  - Current (Go): `GetByFilename` lowercases and normalizes slashes; no MDL/MDX→M2 fallback (`wowexportd/internal/casc/listfile.go`).
  - Expected (JS): If `.mdl`/`.mdx` not found, also tries `.m2` (`wow.export/src/js/casc/listfile.js`).

- Preload logic, TTL and fallback URL
  - Current (Go): Router downloads to cache once if missing and loads `listfile.txt`; no TTL/refresh, minimal fallback handling (`wowexportd/internal/server/router.go`).
  - Expected (JS): `listfile.preload()` supports TTL via `listfileCacheRefresh`, fallback URL with `%s` placeholder trimming, and logs; stores to `constants.CACHE.DIR_LISTFILE` (`casc/listfile.js`).

- Filtered results sorting
  - Current (Go): `GetFilteredEntries` sorts by `fileDataID` ascending.
  - Expected (JS): Returns in iteration order; UI-level sorting is applied elsewhere; server-side REST returns whatever the module emits.

## CASC: Local and Remote

- Local remote-fallback product selection
  - Current (Go): In `Local.GetDataFile`, when initializing `remote`, it matches product using `l.builds[0].Product` (first build) instead of the active/selected build’s product (`wowexportd/internal/casc/casc_source_local.go`).
  - Expected (JS): Matches the selected local build product (`this.build.Product`) when initializing remote fallback (`casc/casc-source-local.js`).

- Build config exposure in `getCascInfo`
  - Current (Go): Returns `build: GetSelectedBuild()` with fields `{ Product, Branch, Version, BuildKey, CDNKey }` for local; remote includes `{ Product, Region, BuildConfig, CDNConfig, VersionsName }`.
  - Expected (JS): Returns `casc.build` as-is; ensure field names and presence mirror JS objects (case-sensitive keys). Double-check that JSON field names align with JS.

## BLTE reader behavior

- Block hash (MD5) mismatch handling
  - Current (Go): On per-block MD5 mismatch, `processNext` returns false and processing stops silently; output may be truncated without an explicit error (`wowexportd/internal/casc/blte_reader.go`).
  - Expected (JS): Throws a `BLTEIntegrityError` when a block hash does not match (`wow.export/src/js/casc/blte-reader.js`).

- Missing encryption key behavior (Salsa20)
  - Current (Go): If key is missing, `decryptBlock` returns `nil`; for non-partial readers this results in silently skipping data; for partial, advances write index with zeros (`blte_reader.go`).
  - Expected (JS): Throws `EncryptionError` when key is missing unless `partialDecrypt` is true, in which case zeros are written (`blte-reader.js`).

- Unsupported block flag 0x46 (recursive frame)
  - Current (Go): Falls through default handling (`writeBuffer`), effectively copying raw data (`blte_reader.go`).
  - Expected (JS): Explicitly throws `No frame decoder implemented!` (`blte-reader.js`).

- Zlib decompression failure
  - Current (Go): On zlib error, falls back to copying raw compressed data into output (`blte_reader.go`).
  - Expected (JS): Decompression is expected; failure should not silently produce invalid output (JS resizes buffer and processes; errors bubble).

## Response caching policy

- Search and info endpoints caching
  - Current (Go): Adds a 10s TTL cache to `/rest/searchFiles` and `/rest/getCascInfo` (`wowexportd/internal/server/cache.go`, used in handlers).
  - Expected (JS): Response caching is only applied to export endpoints (`exportModels`, `exportTextures`, `exportCharacter`, `exportADT`) with body+buildKey keyed entries (`rest-server.js`).

---

Actionable suggestions (high-level):

- Align REST routes with JS by adding `getModelSkins` and (when ready) `getMapList`.
- Update `/rest/getCascInfo` to emit `type: 'CASCLocal'|'CASCRemote'` and consider removing caching for parity.
- Enforce `ERR_CASC_ACTIVE` when Active CASC exists in `loadCascLocal/Remote`.
- Add `list.IsLoaded()` guard and 409 error to `getFileById`/`getFileByName`; allow `fileDataID === 0`.
- Harmonize invalid JSON handling to surface `ERR_INVALID_JSON` semantics or match JS’s 500 behavior.
- Improve `download` to return JSON error on stream failure.
- Implement MDL/MDX→M2 fallback in listfile filename lookup.
- In `Local.GetDataFile`, initialize remote fallback using the currently selected build’s product, not the first build.
- Make BLTE reader strict: throw on MD5 mismatch, on unknown block types, and on missing keys when not partial; avoid silently returning truncated/invalid data.
- Revisit caching policy to match JS (cache only heavy export endpoints, keyed by buildKey+stable body).



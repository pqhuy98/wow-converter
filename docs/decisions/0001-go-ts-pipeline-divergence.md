# ADR 0001: Go vs TypeScript — intentional pipeline divergence

**Status:** accepted  
**Date:** 2026-07-11

## Context

wow-converter maintains two implementations: TypeScript (Bun dev server) and Go (production / desktop). Both share the web UI and broad architecture, but they are **not required to be feature-identical**.

Recent Go work added a **diskless in-memory map-generate path** and related memory/perf tooling. Porting all of it to TS would add significant complexity for little benefit: TS dev mode already talks to wow-data-server over HTTP, and Node cannot hold CASC in-process the way bundled Go does.

## Decision

**Keep TS on the simpler disk-based map pipeline.** Implement advanced pipeline and memory features in **Go only**, for performance and bundled-desktop use.

Do **not** treat missing TS parity as a bug unless a TS dev workflow explicitly breaks.

## Go-only (intentional)

### Direct in-memory map generate

| Area | Go | TS |
|------|----|----|
| Load ADT tiles without disk export phase | `LoadADTTilesForConversion`, `TileRegistry` | Still `exportADT` → disk → convert |
| In-memory snapshot types | `ConversionOutput`, `PlacementRow`, `BakedTexture` | — |
| REST / client | `POST /rest/exportADTForConversion`, `Client.ExportADTForConversion` | — |
| Job result export type | `ADT_DIRECT` | Disk `exportADT` results |

### ADT/WMO export `conv != nil` path

- OBJ/MTL text and terrain PNGs kept in memory (no `.cache` artifacts)
- Placements → in-memory rows (not CSV)
- WMO interiors via `CollectDoodadPlacements`
- Conversion-mode bake cache batching (`BeginConversionExport` / `EndConversionExport`)

### Converter in-memory resolution

- `ConvertAdtTerrainContentToMdl` from OBJ/MTL strings
- `AssetManager` / `WowObjectManager` read terrain + placements from `TileRegistry`
- `TrimAfterParse`, `RegisterTerrainTextures(For)`, `Release`

### Memory / RSS controls (Go runtime)

- `ReleaseAdtExportBatchMemory` after tile load
- `AssetManager.ReleaseAfterExport`, `texturesource.ReleasePaths`
- End-of-job `GC` + `FreeOSMemory` in map generate
- Bundled `InProcessClient` direct `ExportForConversion` (no JSON round-trip)

### Tooling & observability

- `dev` / `WOW_CONVERTER_BUNDLED` single-process Go dev
- `GET /api/debugMemory` (converter, dev-only)
- Dev-only gating for debug memory routes (`config.IsDev()`)
- `maps.go` index rebuild concurrency fix (Go-specific race)

## Other intentional one-sided features

### TS-only remote asset caching

The TS converter supports a wow-data-server on another host. `WowDataClient`
automatically mirrors exported artifacts into the local `.cache` directory and
proxies raw CASC files needed by converter-side readers.

Go does not mirror remote assets. Its supported production path is bundled or
local, where conversion reads CASC in-process or uses the in-memory ADT path.
Do not add remote download caching to Go unless remote Go conversion becomes a
supported deployment mode.

### Go-only bulk map CLI

`golang/cmd/convert/main.go` is the standalone bulk map conversion entry point.
It remains Go-only because scripted, production, and memory-sensitive bulk
exports use the Go pipeline. If TS tooling needs bulk conversion later, it
should call the converter API rather than duplicate the conversion pipeline.

### TS wrapper-library role

Go is the default server and owns heavy export/conversion operations. TS
converter libraries remain supported for ergonomic automation: consumers can
call the Go API, then perform MDL transformations with TS syntax. This is not a
reason to retain duplicate TS server-side conversion pipelines indefinitely.

## TS alignment (partial, intentional)

These were aligned where cheap and user-visible:

- `mapsIncludeLiquid` / `mapsIncludeFoliage`: `false` during map generate
- `stepsPerTile`: `1` for simplified progress
- UI null-safe `failed` handling

TS **still** runs the full disk export loop (`exportADT` + `finalizeExportProgress`).

## Consequences

- **Go** (`npm run dev`, desktop, VPS): default for maps, model exports, and bundled CASC.
- **TS** (`npm run dev:ts`): compatibility runtime for TS wrapper development; its map path still uses disk ADT export via wow-data-server on `:17753`.
- A Go job timeout fails only that job. It must not reset the shared in-process wow-data-server or discard bundled CASC state.
- Before porting a Go feature to TS, ask: does TS dev actually need it, or is Go-only acceptable per this ADR?
- New decisions that affect cross-language expectations belong in `docs/decisions/` as additional ADR files.

## References

- Go entry: `golang/internal/server/api/maps_generate.go`, `golang/internal/converter/mapexporter/tile_loader.go`
- TS entry: `src/server/controllers/maps-generate.ts`
- TS remote caching: `src/lib/wow-data-client/wow-data-client.ts`, `src/lib/wow/archive/client/raw-client.ts`
- Go bulk CLI: `golang/cmd/convert/main.go`
- Bundled mode: `golang/cmd/wow-converter/main.go`, `package.json` `dev`

# ADR 0001: Go and TypeScript may have different conversion pipelines

**Status:** accepted
**Date:** 2026-07-11

## Context

Go owns production model/map conversion and the bundled desktop app. TypeScript retains a compatibility server and automation libraries. Both use the shared web UI, but matching internal implementations would duplicate heavy conversion and memory-management work.

## Decision

Develop production conversion, in-memory map loading and runtime memory controls in Go. Missing TypeScript parity is a bug only when a supported compatibility workflow requires it. New TypeScript automation should call the converter API where possible rather than introduce another conversion engine.

The Go map path loads ADT geometry, placements and liquid metadata through `LoadADTTilesForConversion` and `TileRegistry`. The TypeScript map controller uses the data server's disk export path. Bundled Go uses `InProcessClient`; its optional Unix socket serves tools and integration tests. A job cancellation or timeout must not reset shared CASC state.

TypeScript can mirror remote data-server assets into its local cache. Go's production path uses bundled/local data; remote download-cache parity is not required. The standalone bulk map CLI remains Go-only.

## Current entry points

- Default app: `bun run dev`, `golang/cmd/wow-converter/main.go`.
- Compatibility runtime: `bun run dev:ts`, `src/server/controllers/maps-generate.ts`.
- Go map loading: `golang/internal/converter/mapexporter/tile_loader.go`.
- Go bulk CLI: `golang/cmd/convert/main.go`.
- TypeScript remote access: `src/lib/wow-data-client/wow-data-client.ts` and `src/lib/wow/archive/client/raw-client.ts`.

Module ownership and verification boundaries are in [architecture](../architecture.md). Future changes to cross-language expectations belong in a separate decision.

# wow-converter (Go)

Go ports of:

- `src/wow-data-server/rest-server.ts` → `cmd/wow-data-server`
- `src/server/start.ts` and controllers → `cmd/wow-converter` + `internal/server/api`

## Build

```bash
cd golang
go build ./...
```

## Run wow-data-server only

```bash
go run ./cmd/wow-data-server
```

Listens on `http://127.0.0.1:17753` by default.

## Run wow-converter API (+ UI)

**Dev mode** (HTTP client to wow-data-server on `:17753`):

```bash
go run ./cmd/wow-converter
```

**Bundled mode** (in-process wow runtime via `InProcessClient`, with a unix socket for external tools):

```bash
WOW_CONVERTER_BUNDLED=1 go run ./cmd/wow-converter
# or
go run ./cmd/wow-converter -bundled
```

Bundled mode sets `WOW_DATA_TRANSPORT=socket` and listens on `.cache/wow-data-server.sock` (override with `WOW_DATA_SERVER_SOCKET`).

**Production build** (from repo root):

```bash
bun run build
```

Output directory: `dist-go/` (Go binary, `webui/out`, `bin/`, `resources/` including `template-empty.w3x`).

| Path | Contents |
|------|----------|
| `dist-go/wow-converter.exe` | Desktop app — double-click or run directly |
| `dist-go/webui/out/` | Static UI |
| `dist-go/bin/` | BLP encoder, upscayl, AzerothCore SQLite |
| `dist-go/resources/` | Icon frame assets and WC3 map template (`template-empty.w3x`) |

Bundled mode is auto-detected when `webui/out` sits beside the exe (same as the Bun desktop build). Configure WoW via the setup page in the UI; no `.env` file required.

Run: `.\dist-go\wow-converter.exe`

### Root development scripts

`bun run dev` is the default single-process Go development mode
(`WOW_CONVERTER_BUNDLED=1` + Air), with wow-data-server in-process and no
separate `:17753` listener. The Next.js UI remains on `:3000`; converter API
and UI proxy are on `:3001`.

Use `bun run dev:ts` only for legacy TS wrapper compatibility work. To debug
the Go data server independently, run `go run ./cmd/wow-data-server` from
`golang/` alongside the converter.

An export job timeout fails that job only. It must not restart or reset the
in-process wow-data-server because doing so would discard the bundled CASC
runtime used by other jobs.

## wow-data-server REST routes

| Method | Path | Response ID (success) |
|--------|------|------------------------|
| GET | `/rest/getCascInfo` | `CASC_INFO` |
| GET | `/rest/getConfig` | `CONFIG_FULL` / `CONFIG_SINGLE` |
| GET | `/rest/searchFiles` | `LISTFILE_SEARCH_RESULT` |
| GET | `/rest/getFileById` | `LISTFILE_RESULT` |
| GET | `/rest/getFileByName` | `LISTFILE_RESULT` |
| GET | `/rest/getModelSkins` | `MODEL_SKINS` |
| GET | `/rest/initModelCaches` | `MODEL_CACHES_READY` |
| GET | `/rest/cascFile` | (binary) |
| GET | `/rest/download` | (file stream) |
| GET | `/rest/debugMemory` | `DEBUG_MEMORY` — process heap, CASC/listfile/index sizes, DB caches, export caches |
| GET | `/rest/getMapList` | `MAP_LIST` |
| GET | `/rest/exportProgress` | `EXPORT_PROGRESS` |
| POST | `/rest/loadCascLocal` | `CASC_INSTALL_BUILDS` |
| POST | `/rest/loadCascRemote` | `CASC_INSTALL_BUILDS` |
| POST | `/rest/loadCascBuild` | `CASC_INFO` |
| POST | `/rest/unloadCasc` | `CASC_UNLOADED` |
| POST | `/rest/softRestart` | `SOFT_RESTART_DONE` |
| POST | `/rest/setConfig` | `CONFIG_SET_DONE` |
| POST | `/rest/charMeta` | `CHAR_META` |
| POST | `/rest/exportADT` | `EXPORT_RESULT` |
| POST | `/rest/finalizeExportProgress` | `EXPORT_PROGRESS` |

## wow-converter API routes (`/api/*`)

Mirrors `src/server/start.ts`:

| Method | Path | Controller |
|--------|------|------------|
| GET | `/api/get-config` | get-config |
| GET | `/api/browse` | browse |
| GET | `/api/browse/model-skins` | browse |
| POST | `/api/download` | download |
| GET/POST | `/api/wow-config/*` | wow-config |
| GET | `/api/maps` | maps |
| GET | `/api/maps/:map/wdt-mask` | maps |
| GET | `/api/maps/:map/minimap/:x/:y` | maps |
| POST | `/api/maps/:map/generate-wc3` | maps-generate |
| GET | `/api/maps/generate-wc3/status/:jobId` | maps-generate |
| GET | `/api/maps/generate-wc3/active` | maps-generate |
| GET/POST | `/api/export/character/*` | export-character |
| GET/POST | `/api/texture/*` | export-texture |
| GET | `/api/assets/*`, `/api/browse-assets/*` | export-character (static) |

All converter API routes are wired to the Go implementation (character export, textures, minimap, WC3 map generate).

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3001` | wow-converter HTTP port |
| `NODE_ENV` | (unset) | Set to `development` for dev UI proxy + CORS |
| `IS_SHARED_HOSTING` | `false` | Shared hosting behavior |
| `WOW_CONVERTER_BUNDLED` | (unset) | `1`/`true` enables in-process wow runtime |
| `WOW_DATA_SERVER_PORT` | `17753` | wow-data-server TCP port |
| `WOW_DATA_SERVER_URL` | `http://127.0.0.1:<port>` | HTTP client base URL (dev mode) |
| `WOW_DATA_TRANSPORT` | (unset) | Set to `socket` for unix socket transport |
| `WOW_DATA_SERVER_SOCKET` | `.cache/wow-data-server.sock` | Unix socket path when using socket transport |
| `WOW_EXPORT_DIR` | `.cache/wow-export` | Export directory |
| `CASC_LOCAL_WOW` | (unset) | Auto-load local CASC on wow-data-server start |
| `CASC_LOCAL_PRODUCT` | `wow` | Product for local install |
| `CASC_REMOTE_REGION` | (unset) | Auto-load remote CASC |
| `CASC_REMOTE_PRODUCT` | `wow` | Product for remote CASC |

## Project layout

```
golang/
├── cmd/
│   ├── wow-data-server/main.go
│   └── wow-converter/main.go
├── internal/
│   ├── server/
│   │   ├── api/          # chi router + /api handlers
│   │   ├── rest/         # wow-data-server REST
│   │   └── util/         # job queue
│   └── wow/
│       ├── bootstrap/
│       ├── client/       # HTTP + InProcess clients
│       ├── wowconfig/
│       └── ...
└── README.md
```

## Tests

Unit (no WoW install):

```bash
go test ./...
```

`//go:build integration_tests` files are compiled only with `-tags integration_tests`, so `go test ./...` never sees them. From the repo root, `bun run test:golang-integration` uses an already-running data server (`WOW_DATA_SERVER_SOCKET`, `WOW_DATA_SERVER_URL`, detected development sockets, or `:17753`). Baking API tests also need the converter (`WOW_CONVERTER_URL`, default `http://127.0.0.1:3001`). This suite does not start either server. `bun run verify` builds and boots `dist-go` after unit tests, then runs integration, API, and snapshot checks against it.

Use [architecture and verification](../docs/architecture.md) to choose focused checks. Map regression is currently disabled in `verify`; its success does not certify map output or behavior in Warcraft III.

Module benchmarks in `test/bench` use the same tag:

```bash
go test -tags integration_tests -bench . -benchmem ./test/bench/...
```

## Go client

```go
import "github.com/pqhuy98/wow-converter/internal/wow/client"

// Dev: talk to wow-data-server over HTTP
c := client.NewHTTPClient("")

// Bundled: call REST handlers in-process
c := client.NewInProcessClient(handler)
```

`client.Client` mirrors TypeScript `WowDataClient`.

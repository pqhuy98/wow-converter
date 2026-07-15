# wow-converter (Go)

Go ports of:

- `src/wow-data-server/rest-server.ts` → `cmd/wow-data-server`
- `src/server/start.ts` and controllers → `cmd/wow-converter` + `internal/server/api`

## Build

```bash
cd golang
go mod tidy
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

**Bundled mode** (in-process wow runtime via `InProcessClient`, unix socket transport):

```bash
WOW_CONVERTER_BUNDLED=1 go run ./cmd/wow-converter
# or
go run ./cmd/wow-converter -bundled
```

Bundled mode sets `WOW_DATA_TRANSPORT=socket` and listens on `.cache/wow-data-server.sock` (override with `WOW_DATA_SERVER_SOCKET`).

**Production build** (from repo root):

```bash
npm run build:go-app
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

### Suggested root `package.json` scripts

Add alongside existing Bun scripts:

```json
{
  "scripts": {
    "dev:go-data-server": "cd golang && go run ./cmd/wow-data-server",
    "dev:go-converter": "cd golang && cross-env NODE_ENV=development go tool air",
    "dev:go": "npm-run-all kill:dev-ports --parallel dev:go-data-server dev:go-converter dev:webui",
    "dev:goapp-converter": "cd golang && cross-env NODE_ENV=development WOW_CONVERTER_BUNDLED=1 go tool air",
    "dev:goapp": "npm-run-all kill:dev-ports --parallel dev:goapp-converter dev:webui"
  }
}
```

`dev:go` mirrors `dev` but swaps Bun server/data-server for Go binaries. Keep `dev:webui` for Next.js hot reload in development. Only `wow-converter` hot-reloads (via [Air](https://github.com/air-verse/air), pinned in `go.mod` as `go tool air`); `wow-data-server` runs once with plain `go run`.

`dev:goapp` runs a single bundled process (`WOW_CONVERTER_BUNDLED=1` + Air) with wow-data-server in-process — no separate `:17753` server. Same UI on `:3000`, converter API on `:3001`.

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

### Unit tests (default)

```bash
cd golang
go test ./...
```

Integration-tagged tests are excluded by default.

### Integration tests

Requires a running wow-data-server (TypeScript or Go) with CASC loaded on port `17753`:

```bash
# Terminal 1 — start server with local WoW install
CASC_LOCAL_WOW="D:/Programs/Blizzard Games/World of Warcraft" go run ./cmd/wow-data-server

# Terminal 2 — run integration suite
cd golang
go test -tags integration ./test/integration/...
```

| Variable | Default | Description |
|----------|---------|-------------|
| `WOW_DATA_SERVER_URL` | `http://127.0.0.1:17753` | Server under test |
| `WOW_TS_REFERENCE_URL` | (unset) | Optional TS server for live Go-vs-TS parity |

Integration tests **skip** (not fail) when the server is unreachable or CASC is not loaded.

- `test/integration/casc_parity_test.go` — `GET /rest/cascFile` magic checks (M2 MD20/MD21, BLP1/BLP2, DB2 WDC) plus optional golden/TS byte parity
- `test/integration/adt_parity_test.go` — `POST /rest/exportADT` for northrend tile `21_27` vs TS reference or golden manifest

### Benchmarks

```bash
go test -bench=. -benchmem ./test/bench/...
```

Skips when wow-data-server is unavailable.

### Snapshot harness

Port of `tests/compare-snapshots.ts`:

```bash
go run ./test/verify/compare.go snapshot <dir> --out snapshot.json
go run ./test/verify/compare.go compare snapshot.json <dir> --tolerance '\.png$' --max-delta 2
go run ./test/verify/compare.go diff <dirA> <dirB> --tolerance '\.png$'
```

Golden placeholders live under `test/golden/`. Populate after capturing a known-good TS export:

```bash
go run ./test/verify/compare.go snapshot .cache/wow-export/maps/northrend \
  --out test/golden/adt/northrend_21_27/manifest.json
```

Mount MDL parity (TS vs Go, requires two wow-data-server instances):

```bash
npm run parity:mount-mdl-loop
go run ./test/cmd/test-export -mount -format mdl -limit 1 -offset 0
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

# WebUI Map & Terrain Viewer Implementation Plan

## Overview

Add a new web UI experience to browse WoW maps via a canvas minimap grid and view combined terrain for selected ADT tiles. The UI should be fast, concurrent-user friendly, and use wow-converter’s existing architecture: webui (React/Next) calls our Express REST API, which in turn talks to wow.export REST. Terrain viewing assembles selected ADT tiles into a single scene with correctly placed meshes and higher-resolution per-tile textures (configurable 512/1024/2k/4k). Static/API responses must include HTTP cache headers.

## Current State Analysis

- WebUI (React/Next) exists with components like `browse-model` and a robust `ModelViewerUi` powered by `@pqhuy98/mdx-m3-viewer`, streaming assets via `/api/assets` or `/api/browse-assets`.
- Server has controllers: `browse`, `download`, `export-character` (with static file serving and cache headers), and `get-config`.
- Server already integrates with wow.export REST via `WowExportRestClient` for listfiles, exports, downloads, and caching.
- No existing minimap or terrain pages. No WDT/ADT-specific endpoints yet.

### Key Discoveries
- Caching patterns exist:
  - Static assets served with `express.static(..., { maxAge, setHeaders })` and `Cache-Control` set (see `export-character.ts`).
  - List searches use `Cache-Control: public, max-age=60` (see `browse.ts`).
- Model viewer fetch path solver uses `/api/assets` and `/api/browse-assets` and records loaded files for download.
- wow.export REST provides file search, export, and file download. It does not expose minimap tiles or ADT/WDT directly; we will orchestrate from our server.

## Desired End State

- New WebUI route `Maps` with a high-performance canvas minimap viewer:
  - Pan, zoom (cursor-anchored), selection (Shift+click/drag), Ctrl+A select masked tiles.
  - Hover shows world coords and tile indices.
  - Tiles loaded from server endpoint that returns decoded/processed images (minimap 512 tiles).
  - WDT-driven mask to know available tiles.
- “View Terrain” action assembles selected tiles into one composite terrain model:
  - Server-side converts selected ADT tiles into MDX via wow.export REST + wow-converter pipeline (OBJ used only as an internal intermediary for conversion); outputs per-tile textures up to the chosen size.
  - UI opens a viewer showing all tiles positioned correctly side-by-side.
- API endpoints provide HTTP caching (ETag/Last-Modified or Cache-Control) to scale with concurrent users.

Verification:
- User can browse maps, select tiles, and open the 3D terrain model with correct alignment and high-res textures.
- Viewer loads fast and is responsive with dozens of tiles.

## What We’re NOT Doing
- No export/bake to OBJ for maps beyond what’s needed to render in viewer.
- No in-viewer editing or saving terrain; viewing only.
- No custom shader pipeline beyond mdx-m3-viewer defaults.

## Implementation Approach

- Minimap: client-side canvas like wow.export’s `map-viewer`, but in React. Server provides:
  - List of maps, WDT-derived tile mask, and tile image loader returning PNGs.
- Terrain: server endpoint that, given map and selected tiles plus texture size, orchestrates wow.export exports for ADT tiles, merges outputs into a single export set (shared textures), and returns MDX-only assets with correct placement; or alternatively, returns a list of per-tile MDX model paths plus a manifest with placement transforms for the viewer to load multiple tiles.
- Prefer manifest approach for simplicity and reuse of viewer: client loads N MDX tile models and places them based on server-provided transforms (position, rotation, scale); textures are shared and cached.
- Add HTTP caching: `Cache-Control` on lists, masks, and minimap tiles. For tiles, include ETag via stable keys.

## Phase 1: Server APIs for Maps/Minimap

### Overview
Provide endpoints to list maps, expose WDT tile masks, and stream minimap tiles as PNGs with cache headers.

### Changes Required

1. Controller: `src/server/controllers/maps.ts`
- Endpoints:
  - `GET /api/maps` → list available maps (via wow.export listfile). [x]
  - `GET /api/maps` → list available maps with human-readable names from DB2 via wow.export REST (`/rest/getMapList`); fall back to directory names if DB2 unavailable. [x]
  - `GET /api/maps/:map/wdt-mask` → 64x64 boolean matrix using ADT presence. [x]
  - `GET /api/maps/:map/minimap/:x/:y?size=512` → returns PNG via wow.export texture export. [x]
- Caching:
  - `Cache-Control: public, max-age=3600` for maps and masks; tiles include `ETag` based on `{map,x,y,buildKey}`. [x]
  - In development mode, do not set Cache-Control headers for algorithmic JSON endpoints; still use in-memory caches to avoid recomputation. [x]

2. Wire controller
- Import and call `ControllerMaps(router)` in `src/server/index.ts` before static routes. [x]

## Phase 2: WebUI Minimap Page

### Overview
Implement React page `webui/app/maps/page.tsx` and components for canvas minimap with pan/zoom/selection and “View Terrain”.

### Changes Required

1. Components
- `webui/components/map-viewer/minimap-canvas.tsx` [x]
  - Canvas draw loop with tile cache; queue loads via `fetch(/api/maps/:map/minimap/:x/:y?size=...)`. [x]
  - Interactions: pan, zoom (cursor-anchored), Shift+click add/remove, Shift+drag rectangle select, Ctrl+A select masked. [x]
  - Hover shows world coordinates and `x,y`. [x]
- `webui/components/map-viewer/index.tsx` [x]
  - Wraps `minimap-canvas`, fetches maps (`/api/maps`), tile mask, and passes loader. [x]
  - Contains controls for texture size selection and the “View Terrain” button. [x]

2. Page
- `webui/app/maps/page.tsx` [x]
  - Renders the Map Viewer component. [x]

3. HTTP usage
- Use existing `fetch` patterns from webui (as in browse/recents). No new client libraries.

## Phase 3: Terrain Compose API

### Overview
Endpoint that takes selected tiles and returns a manifest describing models and their world placement, plus preferred texture size.

### Changes Required

1. Controller: `src/server/controllers/terrain.ts`
- `POST /api/terrain/compose`
  - Body: `{ map: string, tiles: { x: number, y: number }[], textureSize: '512'|'1024'|'2048'|'4096' }`
  - Behavior:
    - Extend wow.export REST to support tile export options similarly to its GUI (Export WMO, Export WMO Sets, Export M2, Export Foliage). For this feature, keep all those options OFF; only ADT tiles are exported.
    - For each selected tile, derive ADT path `world/maps/:map/:map_XX_YY.adt` and call wow.export REST to export that tile to OBJ+PNG in `outputDirBrowse` (internal intermediary only).
    - Use `WowObjectManager.readTerrainsDoodads(patterns, filter)` with a filter that includes only ADT files (exclude WMO/M2) to load the tiles, then generate batch MDX models from the tiles' OBJ. Never serve OBJ to the viewer.
    - Return a manifest:
      ```json
      {
        "versionId": "...",
        "models": [
          { "path": ".../map_XX_YY.mdx", "position": [worldX, worldY, worldZ], "rotation": [rx, ry, rz], "scale": s, "texturePath": ".../t_...png" }
        ],
        "tileSize": 533.333,
        "mapSize": 64
      }
      ```
    - Set `Cache-Control` to a short TTL (e.g., 60s) and include ETag keyed by `{buildKey,map,tiles,textureSize}`.
  - Note: Viewer must load MDX only. OBJ is used only as an internal step before conversion to MDX via wow-converter.

2. Static files
- Reuse `/api/browse-assets` for served outputs, already cached. Respect shared hosting settings.


## Phase 4: WebUI Terrain Viewer Integration

### Overview
On “View Terrain”, call compose API, then open a viewer loading all returned models positioned correctly.

### Changes Required

1. Manifest consumption
- Extend `ModelViewerUi` with optional prop `instances?: { path: string; position: [number,number,number]; rotation?: [number,number,number]; scale?: number }[]` to load multiple files and place instances with translations/rotations/scales. Alternatively, create a thin `TerrainViewer` wrapper component which loads each model via `ModelViewer` and creates dummy parent nodes for placement.
- Add a new page `webui/app/terrain/page.tsx` supporting query params: `map`, `tiles`, `texSize` or accept a POSTed manifest stored in memory keyed by `versionId`.

2. UX wiring
- In minimap, after compose success, navigate to `/terrain?version=<id>` and fetch the manifest via `/api/terrain/manifest/:id` (optional helper endpoint), or pass models in router state.

## Phase 5: Performance & Caching

- Server-side
  - Add `Cache-Control: public, max-age=3600` for `/api/maps` and `/api/maps/:map/wdt-mask`.
  - For `/api/maps/:map/minimap/:x/:y`, compute `ETag` from `{buildKey,map,x,y,baseTileHash}` and respond `304` if matches.
  - Dedupe tile loads per key to avoid thundering herd; keep an in-memory LRU for decoded base tiles to resize quickly.
- Client-side
  - Tile request queue with Manhattan-distance from cursor prioritization.
  - Request cancellation or drop logic when zoom quickly changes.

## Testing Strategy

### Unit Tests
- Mask computation: given mocked listfile, ensure correct 64x64 mask.
- Tile loader: BLP→PNG conversion path returns expected sizes.
- Compose API: manifest contains expected models and transforms for a sample 2×2 selection.

### Manual Testing Steps by Human
1. Open `/maps`. Pan/zoom around a large map; verify performance and hover info.
2. Shift+drag to select a rectangle; Ctrl+A selects all masked tiles.
3. Click “View Terrain” with texture size 4k; verify combined scene appears and tiles align.
4. Switch texture size to 512; verify faster compose and lower texture memory.
5. Validate HTTP caching: repeat tile loads return 304; network waterfall is minimal.

## Performance Considerations
- Decode/caching: keep base 512 minimap tiles cached in-process; rescale on the fly.
- Concurrency: avoid global locks; use per-tile pending promises and a queue.
- Memory: cap tile cache size with LRU eviction.
- Large selections: stream exports sequentially with limited concurrency (e.g., 2–4) to keep memory bounded.

## References
- Related research: `thoughts/shared/research/map-viewer-webui.md`
- Existing patterns:
  - `src/server/controllers/export-character.ts` (static file caching, queue)
  - `src/server/controllers/browse.ts` (Cache-Control usage)
  - `webui/components/common/model-viewer.tsx` (viewer integration)

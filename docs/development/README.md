# Development

Run commands from the repository root. Install Go, Bun and Git, then:

```powershell
bun install
bun run dev
```

The default process runs the bundled Go converter/data reader and a Next.js development UI. The UI is on `http://127.0.0.1:3000`; the API is on `http://127.0.0.1:3001`. If WoW data is not configured, open `/setup`. Go and UI sources hot-reload; the in-process data reader restarts with Go. Starting development first stops processes on ports 3000, 3001 and 17753.

`bun run build` creates the Windows desktop bundle in `dist-go`; `bun run build:linux` builds its Linux counterpart. The bundled application serves both UI and API on port 3001. `bun run dev:ts` is the compatibility TypeScript runtime; [ADR 0001](../decisions/0001-go-ts-pipeline-divergence.md) defines its scope.

Choose focused checks from [verification](verification.md). `bun run verify` builds and boots a release bundle, then runs its broader checks against that bundle; it is not required for a documentation-only edit.

## Cache controls

The setup page has two separate buttons under **Clear cache**, each showing its current size. The displayed GB value is bytes divided by 1024³.

| Button | Directory cleared | Loaded client |
|---|---|---|
| Clear CASC cache | All contents of `.cache` | Unloaded first |
| Clear browse assets | All contents of `exported-assets-browse` | Retained |

Browse assets include model previews and map preview caches, which rebuild when needed. These controls do not clear character outputs in `exported-assets` or generated maps in `maps`.

The API is `GET /api/wow-config/cache-size` and `POST /api/wow-config/clear-cache` with `{"kind":"casc"}` or `{"kind":"browse"}`. Ownership is `golang/internal/workspace/clear_cache.go` and `golang/internal/wow/wowconfig/service.go`. [Current UI capture](../screenshots/ui/cache-clear-buttons.png).

## Bundled gameplay data

Map source gameplay data comes from `bin/azerothcore-world.sqlite`; a live SQL server is not needed at runtime. Refresh it through:

```powershell
bun run generate:acore-sqlite
```

The generator reads `ACORE_SOURCE_DATABASE_URL`. Its table list is `scripts/acore-sqlite-tables.ts`. To refresh selected tables from AzerothCore base SQL dumps while retaining the rest:

```powershell
bun scripts/generate-azerothcore-sqlite.ts --sql-dir=<db_world-directory> --tables=gameobject,gameobject_template,gameobject_addon,game_event_gameobject,game_event_creature
```

Rerun `bunx prisma generate` when changing the Prisma schema. Check the generated table schema together with converter readers when expanding bundled data.

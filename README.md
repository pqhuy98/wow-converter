# Huy's Wow-Converter

Export models of World of Warcraft NPCs straight to Warcraft III's MDX/MDL models with just a few clicks.

Demo video: https://youtu.be/FLgBKT7c2TI?si=wdPLJ6pVaEVFE5iS

HiveWorkshop thread: https://www.hiveworkshop.com/threads/wow-converter-export-wow-npcs-to-warcraft-3.363033/

![screenshot 1](https://github.com/pqhuy98/wow-converter/blob/main/docs/screenshots/elwin-forest-1.jpg?raw=true)

![screenshot 2](https://github.com/pqhuy98/wow-converter/blob/main/docs/screenshots/icecrown.jpg?raw=true)


---

## Quick Start (No Install Required)

1. Visit **https://wow.quangdel.com/** – the converter UI runs entirely in your browser, nothing to install.
2. Provide your desired NPC character and optionally attached items by giving Wowhead URLs. See prefilled example URLs.
3. Adjust any options you like (animations, size, optimisations, etc.).
4. Click the button **Export Character**.
5. Wait until the export finishes, a download button will appear to download a **ZIP** containing the model (MDX/MDL) and all required BLP textures.
6. Import the files into your map with World Editor, or open it with WC3 modeling tools like Retera Model Studios.

---

## Using the Windows Binaries

Prefer working offline, fast, without waiting queue, and no constant ZIP download and extraction? Follow these simple steps to run the app locally.

### 1. Get the tools

Download the latest release ZIP here: https://github.com/pqhuy98/wow-converter/releases and extract it. You will see the main executables among many other files:

| File | Purpose |
|------|---------|
| `wow-converter.exe` | Native WoW reader + web UI + MDX/MDL conversion (single process) |

### 2. Start **wow-converter**

1. Run `wow-converter.exe`. A command line window opens and loads WoW data from your local install or Blizzard CDN (configure via `.env` or the web UI `/setup` wizard).
2. Wait until you see:
  ```
  ✅ WoW data ready: ...
  Serving UI web interface at http://127.0.0.1:3001
  ```

### 3. Export your model

1. Open **http://127.0.0.1:3001/** in your browser.
2. Use the app similarly to **https://wow.quangdel.com/**
3. All exported assets will be stored in the `exported-assets` directory inside the folder where you extracted the app. This is better than hosted version because you won't need to download or extract any ZIP file.

---

## Building From Source (Optional)
This section is for experienced programmers who want to build the app from source code. Requires [**Go**](https://go.dev/), [**Bun**](https://bun.com/), and [**Git**](https://git-scm.com/downloads).

Clone this repository:
```
git clone https://github.com/pqhuy98/wow-converter
cd wow-converter
bun install
bun run build   # outputs the bundled Go app into `dist-go`
```

For development, run the bundled Go server and Next.js UI:
```
bun run dev
```

The legacy TS server remains available as `bun run dev:ts` for wrapper-library compatibility work.

### Parity checks

Run the full Valiance Keep TS-versus-Go map comparison:

```bash
bun run parity:map
```

The command writes both outputs to `.parity-artifacts/map-output/{ts,go}`. Inspect
a failed map comparison semantically with:

```bash
go -C golang run ./test/cmd/compare-map ../.parity-artifacts/map-output/ts ../.parity-artifacts/map-output/go
```

For MDL parity, start the two data servers in one terminal, then run the loop in another:

```bash
bun scripts/start-parity-servers.ts
bun run parity:mdl
```

Map exports use a bundled SQLite copy of AzerothCore world data (`bin/azerothcore-world.sqlite`) — no live MySQL/PostgreSQL is required at runtime. To refresh that file from your AzerothCore world database:

```
bun run generate:acore-sqlite   # reads ACORE_SOURCE_DATABASE_URL, writes bin/azerothcore-world.sqlite
```

Table list for the export lives in `scripts/acore-sqlite-tables.ts`. After adding tables there, rerun the command above and `bunx prisma generate` if the Prisma schema changed.

The release binary embeds wow-data-server and talks to it over a local unix socket (not exposed on a second port). Only the web UI/API listens on `:3001`.


---

## Credits

- Built by me - *Warcraft Sandbox* (<https://www.youtube.com/@wc3-sandbox>).<br>
- WoW data reading originally inspired by **Kruithne**'s wow.export: https://github.com/Kruithne/wow.export
- Exported assets are from World of Warcraft, Blizzard Entertainment.
- https://github.com/flowtsohg/mdx-m3-viewer
- https://github.com/Deamon87/WebWowViewerCpp
- https://github.com/ChiefOfGxBxL/WC3MapTranslator
- https://github.com/4eb0da/war3-model
- https://github.com/ilimei/vscode-plugin-blp-preview
- https://github.com/azerothcore/azerothcore-wotlk
- Retera Model Studios and Twilac's Retera Model Studios
- Built with NodeJS, NextJS, v0.dev, Cursor AI, OpenAI LLMs.

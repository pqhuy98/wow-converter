---
name: shot-export-wowhead
description: Screenshot the Wowhead model-viewer canvas for a wowhead.com NPC, item, or object page. Use when the user pastes a Wowhead URL, asks how a model looks on Wowhead, or wants that canvas compared with the wow-converter export.
---

# Shot export (Wowhead)

Save the model canvas with the same Go capture code as the report server. The page, toolbar, and cookie banner stay outside the PNG. Use installed Chrome, Edge, or Chromium.

From the repository root:

```
go -C golang run ./cmd/shot-wowhead <url> --seq Stand --view front
```

Flags: `--seq Stand` (default), `--view front` (omit for all six; comma-separated views also work), `--variant 0`, `--out dir`. A second positional argument also sets the output directory. Relative output directories are relative to the repository root; default is `tmp/shots`. The command adds `#modelviewer` when needed and prints one labeled 1440×900 PNG path per selected view, named `<slug>-<sequence>-<view>.png`. Read the PNGs. Blank shots fail.

Use WoW animation names such as `Stand`, `Attack1H`, or `Death`. Prefixes select the first match in file order; `--variant` selects a later match. WC3 names differ: use the export's `reportMetadata.models[].sequences[]` (`wowName`/`wowVariant` versus `name`). Runtime table: `golang/internal/converter/wowmodel/animation/` (generated from `src/lib/converter/wow-model/animation/animation-mapper.ts`).

For comparison, follow shot-export-wow-converter to export fresh assets, then capture both sources together (prefer `shot-converter --wowhead` so the converter owns the pair):

```
go -C golang run ./cmd/shot-wowhead <url> --seq Stand --model <asset-path> --converter-seq "Stand 1" --view front --out tmp/compare-shots
```

The local converter must already be running; `--base` or `WOW_CONVERTER_URL` selects it. Only the requested views are captured. The report server's side refinement and source-to-converter camera transfer are shared, including the fixed Wowhead top/bottom cameras. Paired filenames end in `-wowhead.png` or `-converter.png`. Read both images.

Standalone shots keep the existing model-bounds framing. Pass `--aim-sheet <1920x800 converter sheet>` to aim side cameras at that sheet's silhouettes. `--cameras dest.json` writes the per-view eye/target/up and model transform map the converter reads with the same flag. Pass `--model-scale <reportMetadata.models[].modelScale>` for resized exports; it records exported units per source unit (default 56). Never calibrate this scale from mesh height: equipment changes bounds independently. Otherwise, if the output folder already contains `<slug>-<sequence>-<nn>-<view>-converter.png`, its silhouette aim is reused for that side view. Use an empty output folder to preserve source framing. `--sheet dest.png` writes an unlabeled 1920×800 2×3 sheet. Views are front, left, back, right, top, bottom. The pose is frozen at the animation midpoint; background is rgb(38,38,38), FOV 45 degrees, and lighting follows the converter's world-up curve. Captures wait for model assets and use isolated temporary browser profiles.

`.cursor/skills/shot-export-wowhead/snapshot.ts` is a compatibility wrapper around `go run`; no screenshot executable needs rebuilding.

# Standalone camera and sheet options

- `--aim-sheet <1920x800 converter sheet>` aims side cameras at that sheet's silhouettes.
- `--cameras dest.json` writes per-view eye/target/up and model transforms; the converter reads this map with its own `--cameras` flag.
- `--model-scale <reportMetadata.models[].modelScale>` records exported units per source unit (default 56). Use fresh export metadata; equipment changes mesh bounds independently, so never infer scale from mesh height.
- If the output folder contains `<slug>-<sequence>-<nn>-<view>-converter.png`, its silhouette aim is reused for that side view. Use an empty output folder to retain standalone source framing.
- `--sheet dest.png` writes an unlabeled 1920×800 2×3 sheet: front, left, back / right, top, bottom.

For paired source/export captures use the converter shot skill, which owns the shared camera-transfer workflow.

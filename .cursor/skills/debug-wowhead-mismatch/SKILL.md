---
name: debug-wowhead-mismatch
description: Diagnose and fix a reported visual mismatch between a Wowhead model and its wow-converter export.
---

# Debug a Wowhead mismatch

Fix the owning Go layer and trace affected character/map callers. Update TS when the supported compatibility workflow is affected; Go-only features need no TS counterpart (ADR 0001 in `docs/decisions/`).

Use [shot-export-wow-converter](../shot-export-wow-converter/SKILL.md) for a fresh export and paired captures. Choose a view that shows the reported difference and read both PNGs before editing. A blank or incomplete Wowhead frame is failed capture evidence; recapture it before diagnosing conversion.

Classify the difference, then read only the relevant detail:

- Missing geometry: [geosets](references/geosets.md).
- Wrong color, black, or magenta surface: [materials and textures](references/materials.md).
- Wrong pose/timing: verify the source animation pair from export metadata using the shot skill. Compare the same mapped sequence at its midpoint before changing animation mapping or timing.

Keep temporary logs, scripts, and captures in gitignored `tmp/`. After the fix, re-export and capture the same views, inspect both sources, and remove temporary source logs. Leave the shot CLIs unchanged.

For shared conversion changes, run the affected snapshot cases and broaden before claiming other catalog sheets remain valid. Follow [snapshot-tests](../../rules/snapshot-tests.mdc) for catalog structure, commands, and review. Once the user confirms a new model's paired shots match, add its catalog case and inspect the expected sheet. Rewrite existing approved sheets only when explicitly requested.

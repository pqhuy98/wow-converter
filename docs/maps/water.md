# Native water export

Water export is automatic and independent of NPC export and **Place path blockers**. It keeps the selected scale/clamp. Owner: `golang/internal/converter/mapexporter/terrain/terrain_water.go`.

Masked ADT MH2O/legacy MCLQ and WMO MLIQ cells travel through typed snapshots/object geometry. ADT liquid heights are absolute; WMO heights are local and receive the instance transform once. Reading liquid chunks does not require Fresh export. Client `LiquidType.db2` classifies water/ocean/magma/slime, with known legacy-ID fallback; unknown types are preserved in diagnostics.

## Native representation

Water uses native WC3 corners on a 128-unit grid, without liquid surface doodads. Any positive-area visible overlap enables all four corners of a quad. Clipped source triangles and edge extrema catch narrow wet slivers; higher ground hides extended water without lowering shores. Fully covered upper floors stay dry. Origin seams smaller than 0.0625 WC3 units are joined. Source masks still control liquid movement flags.

Stacked levels and slopes that do not fit the native grid count as approximations. Height overflow, magma/slime and unknown types are unsupported and reported; no model patch or scale change is introduced. Ordinary liquids use the map's WC3 water appearance, so WoW materials, waterfalls and overlapping surfaces are not fully reproduced. Source depth also participates in the navigation ground search.

## Output

| File | Contents |
|---|---|
| `war3map.w3e` | Native water corners; v12 presence bit `0x100`, low 14 height bits independent of boundary marker `0x4000` |
| `war3map.wpm` | Swimming `64` and no-building `8`; depth over 51.2 WC3 units also blocks walking `2`; liquid adds no no-flying flag |
| `terrain-water.json` | Source layers/cells/types, native coverage, approximations/unsupported reasons, height range and pathing counts |
| `terrain-water.png` | Light blue shallow, dark blue deep, orange non-water liquid pathing |

For the generated Lordaeron tileset, visible water Z is `(raw-8192)/4-89.6`, with 0.25-unit precision and range −2137.6…1958.15. Structural flags merge when blockers are enabled. Liquid WPM remains present with NPC/blocker export off. Cached exports remove formerly generated `water/liquid-*` models/textures.

## Verification

```powershell
go -C golang test ./internal/wc3/translators ./internal/wow/formats/adt ./internal/wow/formats/wmo ./internal/converter/common ./internal/converter/mapexporter/...
```

Fixtures cover masks, independent v11/v12 terrain bytes, absolute/local transforms, slopes, snapshot ownership, bridges, depth, small pools, unsupported types, chunk seams and cached cleanup. [Navigation validation](terrain-navigation.md#diagnostics-and-validation) checks the union of water and saved structural footprints.

Native shorelines, walking/swimming/flight, bridge traversal and editor save stability still require a WC3 check. The depth threshold follows the renderer's native-water convention and remains subject to native readback.
